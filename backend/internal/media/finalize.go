package media

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

func copyBytes(w io.Writer, r io.Reader, n int64) (int64, error) { return io.CopyN(w, r, n) }

func (h *Handler) upload(w http.ResponseWriter, r *http.Request, userID string) {
	id := r.PathValue("id")
	if !validID(id) {
		fail(w, 400, "Invalid memory.")
		return
	}
	contentType := canonicalType(r.Header.Get("Content-Type"))
	rule, ok := rules[contentType]
	if !ok {
		fail(w, 415, "Choose a supported picture, audio or video file.")
		return
	}
	declared, err := strconv.ParseInt(r.Header.Get("X-Upload-Size"), 10, 64)
	if err != nil || declared <= 0 || declared > rule.MaxBytes {
		fail(w, 413, "This file exceeds the size limit for its media type.")
		return
	}
	if r.ContentLength >= 0 && r.ContentLength != declared {
		fail(w, 400, "Upload size does not match the file.")
		return
	}
	rawName, err := url.PathUnescape(r.Header.Get("X-Upload-Name"))
	if err != nil || len(rawName) > 2048 {
		fail(w, 400, "Invalid filename.")
		return
	}
	name := safeName(rawName)
	source := r.Header.Get("X-Media-Source")
	if source == "" {
		source = "upload"
	}
	if source != "upload" && source != "voice" || source == "voice" && rule.Kind != "audio" {
		fail(w, 400, "Invalid media source.")
		return
	}
	assetID := r.Header.Get("X-Upload-ID")
	if assetID == "" {
		assetID, err = newID()
		if err != nil {
			serverError(w, err)
			return
		}
	}
	if !validID(assetID) {
		fail(w, 400, "Invalid upload ID.")
		return
	}
	select {
	case h.uploads <- struct{}{}:
		defer func() { <-h.uploads }()
	default:
		fail(w, 429, "Two files are uploading. Wait a moment and try again.")
		return
	}

	// Short reservation transaction. Lock workspace before memory consistently;
	// this serializes quota decisions without holding a connection during transfer.
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var workspaceID, status string
	err = tx.QueryRow(r.Context(), `SELECT w.id::text FROM workspaces w JOIN memories m ON m.workspace_id=w.id
        JOIN workspace_members wm ON wm.workspace_id=w.id WHERE m.id=$2::uuid AND w.kind='personal'
        AND w.owner_user_id=$1::uuid AND wm.user_id=$1::uuid AND wm.role='owner' FOR UPDATE OF w`, userID, id).Scan(&workspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "Memory not found.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	err = tx.QueryRow(r.Context(), `SELECT status FROM memories WHERE id=$1::uuid FOR UPDATE`, id).Scan(&status)
	if err != nil {
		serverError(w, err)
		return
	}
	if status != "draft" {
		fail(w, 409, "Only saved, unsealed memories accept attachments.")
		return
	}
	// A lost response can be retried with the same ID without creating a second attachment.
	a, err := readAsset(tx.QueryRow(r.Context(), `SELECT `+assetColumns+` FROM media_assets a JOIN memory_assets ma ON ma.asset_id=a.id
        WHERE a.id=$1::uuid AND ma.memory_id=$2::uuid AND a.uploaded_by=$3::uuid AND a.status='ready'
        AND a.content_type=$4 AND a.declared_bytes=$5 AND a.original_name=$6 AND a.source=$7`, assetID, id, userID, contentType, declared, name, source))
	if err == nil {
		respond(w, 200, a)
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		serverError(w, err)
		return
	}
	var count, total, memoryCount int64
	err = tx.QueryRow(r.Context(), `SELECT count(*),COALESCE(sum(CASE WHEN status='ready' THEN verified_bytes ELSE declared_bytes END),0)
        FROM media_assets WHERE workspace_id=$1::uuid AND (status='ready' OR status='pending')`, workspaceID).Scan(&count, &total)
	if err != nil {
		serverError(w, err)
		return
	}
	err = tx.QueryRow(r.Context(), `SELECT (SELECT count(*) FROM memory_assets WHERE memory_id=$1::uuid)+
        (SELECT count(*) FROM media_assets WHERE memory_id=$1::uuid AND status='pending')`, id).Scan(&memoryCount)
	if err != nil {
		serverError(w, err)
		return
	}
	if count >= maxWorkspaceItems || total+declared > maxWorkspaceBytes || memoryCount >= maxMemoryItems {
		fail(w, 429, "Attachment limit reached. Remove an attachment or use a smaller file.")
		return
	}
	key := "assets/" + workspaceID + "/" + assetID
	tag, err := tx.Exec(r.Context(), `INSERT INTO media_assets(id,workspace_id,uploaded_by,object_key,kind,content_type,declared_bytes,memory_id,original_name,source)
        VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6,$7,$8::uuid,$9,$10) ON CONFLICT(id) DO NOTHING`, assetID, workspaceID, userID, key, rule.Kind, contentType, declared, id, name, source)
	if err != nil {
		serverError(w, err)
		return
	}
	if tag.RowsAffected() != 1 {
		fail(w, 409, "This upload is already in progress. Refresh attachments before retrying.")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		serverError(w, err)
		return
	}
	done := false
	defer func() {
		if !done {
			h.reject(assetID)
		}
	}()

	// A bounded private temporary file avoids buffering videos in RAM and lets
	// us inspect bytes before anything becomes readable from object storage.
	f, err := os.CreateTemp("", "chrono-upload-")
	if err != nil {
		serverError(w, err)
		return
	}
	defer os.Remove(f.Name())
	defer f.Close()
	r.Body = http.MaxBytesReader(w, r.Body, rule.MaxBytes)
	n, err := io.Copy(f, r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			fail(w, 413, "File is too large.")
		} else {
			fail(w, 400, "Upload was interrupted. Your saved memory is unchanged.")
		}
		return
	}
	if n != declared {
		fail(w, 422, "File size changed during upload. Choose the file again.")
		return
	}
	if err := inspectFile(f, contentType); err != nil {
		fail(w, 422, "The file contents do not match a supported media format.")
		return
	}
	if err := h.objects.Put(r.Context(), key, contentType, n, f); err != nil {
		serverError(w, err)
		return
	}

	tx, err = h.db.Begin(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	err = tx.QueryRow(r.Context(), `SELECT status FROM memories WHERE id=$1::uuid FOR UPDATE`, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && status != "draft" {
		fail(w, 409, "Memory changed before upload finished. Refresh your journal.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	var uploadState string
	err = tx.QueryRow(r.Context(), `SELECT status FROM media_assets WHERE id=$1::uuid FOR UPDATE`, assetID).Scan(&uploadState)
	if err != nil {
		serverError(w, err)
		return
	}
	if uploadState != "pending" {
		fail(w, 409, "Upload expired. Please choose the file again.")
		return
	}
	var position int
	err = tx.QueryRow(r.Context(), `SELECT COALESCE(max(position)+1,0) FROM memory_assets WHERE memory_id=$1::uuid`, id).Scan(&position)
	if err != nil {
		serverError(w, err)
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE media_assets SET status='ready',verified_bytes=$2,verified_at=now() WHERE id=$1::uuid`, assetID, n)
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO memory_assets(workspace_id,memory_id,asset_id,position) VALUES($1::uuid,$2::uuid,$3::uuid,$4)`, workspaceID, id, assetID, position)
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		serverError(w, err)
		return
	}
	done = true
	respond(w, 201, asset{ID: assetID, ContentType: contentType, VerifiedBytes: n, Position: position, Name: name, Kind: rule.Kind, Source: source})
}
func (h *Handler) reject(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Never undo a ready attachment if COMMIT succeeded but its acknowledgement
	// was lost. Otherwise retain its key in the durable cleanup queue.
	_, _ = h.db.Exec(ctx, `WITH rejected AS (UPDATE media_assets SET status='rejected' WHERE id=$1::uuid AND status='pending' RETURNING object_key)
        INSERT INTO media_deletions(object_key) SELECT object_key FROM rejected ON CONFLICT DO NOTHING`, id)
}
func (h *Handler) remove(w http.ResponseWriter, r *http.Request, userID string) {
	id, assetID := r.PathValue("id"), r.PathValue("assetID")
	if !validID(id) || !validID(assetID) {
		fail(w, 400, "Invalid attachment.")
		return
	}
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var status string
	err = tx.QueryRow(r.Context(), `SELECT m.status FROM memories m JOIN workspaces w ON w.id=m.workspace_id
        JOIN workspace_members wm ON wm.workspace_id=w.id WHERE m.id=$2::uuid AND w.kind='personal'
        AND w.owner_user_id=$1::uuid AND wm.user_id=$1::uuid AND wm.role='owner' FOR UPDATE OF m`, userID, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "Memory not found.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if status != "draft" {
		fail(w, 409, "Attachments are fixed once a memory is sealed.")
		return
	}
	tag, err := tx.Exec(r.Context(), `DELETE FROM memory_assets WHERE memory_id=$1::uuid AND asset_id=$2::uuid`, id, assetID)
	if err != nil {
		serverError(w, err)
		return
	}
	if tag.RowsAffected() != 1 {
		fail(w, 404, "Attachment not found.")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		serverError(w, err)
		return
	}
	respond(w, 204, nil)
}
