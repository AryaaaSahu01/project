package media

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

func (h *Handler) finalize(w http.ResponseWriter, r *http.Request, userID string) {
	memoryID, assetID := r.PathValue("id"), r.PathValue("assetID")
	if !uuid.MatchString(memoryID) || !uuid.MatchString(assetID) {
		fail(w, 400, "invalid ID")
		return
	}
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var workspaceID, memoryStatus string
	err = tx.QueryRow(r.Context(), `
		SELECT m.workspace_id::text, m.status FROM memories m
		JOIN workspaces w ON w.id=m.workspace_id
		JOIN workspace_members wm ON wm.workspace_id=w.id
		WHERE m.id=$2::uuid AND w.kind='personal'
		  AND w.owner_user_id=$1::uuid AND wm.user_id=$1::uuid AND wm.role='owner'
		FOR UPDATE OF m`, userID, memoryID).Scan(&workspaceID, &memoryStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "memory not found")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if memoryStatus != "draft" {
		fail(w, 409, "images can only be added to draft memories")
		return
	}
	var sourceKey, contentType, state string
	var declaredBytes int64
	var createdAt time.Time
	err = tx.QueryRow(r.Context(), `
		SELECT object_key, content_type, declared_bytes, status, created_at
		FROM media_assets WHERE id=$1::uuid AND workspace_id=$2::uuid
		  AND uploaded_by=$3::uuid AND kind='image' FOR UPDATE`,
		assetID, workspaceID, userID).Scan(&sourceKey, &contentType, &declaredBytes, &state, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "image upload not found")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if state == "ready" {
		var a asset
		err = tx.QueryRow(r.Context(), `
			SELECT a.id::text, a.content_type, a.verified_bytes, ma.position
			FROM media_assets a JOIN memory_assets ma
			  ON ma.asset_id=a.id AND ma.workspace_id=a.workspace_id
			WHERE a.id=$1::uuid AND ma.memory_id=$2::uuid`, assetID, memoryID).
			Scan(&a.ID, &a.ContentType, &a.VerifiedBytes, &a.Position)
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 409, "image already belongs to another memory")
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}
		respond(w, 200, a)
		return
	}
	if state != "pending" || createdAt.Before(time.Now().UTC().Add(-15*time.Minute)) {
		fail(w, 410, "upload intent expired; request another")
		return
	}
	var count int
	err = tx.QueryRow(r.Context(), `
		SELECT count(*) FROM memory_assets WHERE memory_id=$1::uuid`, memoryID).Scan(&count)
	if err != nil {
		serverError(w, err)
		return
	}
	if count >= maxMemoryItems {
		fail(w, 429, "memory image limit reached")
		return
	}
	// Reject obvious oversize or mislabeled uploads before copying them.
	sourceSize, sourceType, err := h.objects.Head(r.Context(), sourceKey)
	if err != nil {
		serverError(w, err) // Upload may still be in progress; retry finalize.
		return
	}
	sourceSniff, err := h.objects.DetectContentType(r.Context(), sourceKey)
	if err != nil {
		serverError(w, err)
		return
	}

	readyKey := "assets/" + workspaceID + "/" + assetID
	if sourceSize != declaredBytes || sourceType != contentType || sourceSniff != contentType {
		_, err = tx.Exec(r.Context(), `
			UPDATE media_assets SET status='rejected' WHERE id=$1::uuid`, assetID)
		if err != nil {
			serverError(w, err)
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			serverError(w, err)
			return
		}
		h.deleteBestEffort(sourceKey)
		h.deleteBestEffort(readyKey)
		fail(w, 422, "image bytes, type or size do not match intent")
		return
	}
	if err := h.objects.Copy(r.Context(), sourceKey, readyKey); err != nil {
		serverError(w, err) // e.g. the PUT has not completed; retry finalize.
		return
	}
	size, actualType, err := h.objects.Head(r.Context(), readyKey)
	if err != nil {
		serverError(w, err)
		return
	}
	sniffed, err := h.objects.DetectContentType(r.Context(), readyKey)
	if err != nil {
		serverError(w, err)
		return
	}
	if size != declaredBytes || actualType != contentType || sniffed != contentType {
		_, err = tx.Exec(r.Context(), `
			UPDATE media_assets SET status='rejected' WHERE id=$1::uuid`, assetID)
		if err != nil {
			serverError(w, err)
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			serverError(w, err)
			return
		}
		h.deleteBestEffort(sourceKey)
		h.deleteBestEffort(readyKey)
		fail(w, 422, "image bytes, type or size do not match intent")
		return
	}
	_, err = tx.Exec(r.Context(), `
		UPDATE media_assets SET object_key=$2, verified_bytes=$3,
		  status='ready', verified_at=now() WHERE id=$1::uuid`,
		assetID, readyKey, size)
	if err != nil {
		serverError(w, err)
		return
	}
	_, err = tx.Exec(r.Context(), `
		INSERT INTO memory_assets (workspace_id, memory_id, asset_id, position)
		VALUES ($1::uuid,$2::uuid,$3::uuid,$4)`, workspaceID, memoryID, assetID, count)
	if err != nil {
		serverError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		serverError(w, err)
		return
	}
	h.deleteBestEffort(sourceKey)
	respond(w, 200, asset{
		ID: assetID, ContentType: contentType,
		VerifiedBytes: size, Position: count,
	})
}

func (h *Handler) deleteBestEffort(key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.objects.Delete(ctx, key); err != nil {
		// A later cleanup worker must retry; never hide a committed success.
		log.Printf("media cleanup failed: %v", err)
	}
}
