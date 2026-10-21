package media

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"journalapp/internal/account"
	"journalapp/internal/httpapi"
	"journalapp/internal/platform/objectstore"
)

const (
	maxImageBytes     int64 = 10 << 20
	maxWorkspaceBytes int64 = 100 << 20
	maxWorkspaceItems       = 20
	maxMemoryItems          = 10
)

var uuid = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type Handler struct {
	db       *pgxpool.Pool
	accounts *account.Service
	objects  *objectstore.R2
}

func NewHandler(db *pgxpool.Pool, accounts *account.Service, objects *objectstore.R2) *Handler {
	return &Handler{db: db, accounts: accounts, objects: objects}
}

type intentInput struct {
	ContentType   string `json:"content_type"`
	DeclaredBytes int64  `json:"declared_bytes"`
}

type asset struct {
	ID            string `json:"id"`
	ContentType   string `json:"content_type"`
	VerifiedBytes int64  `json:"verified_bytes"`
	Position      int    `json:"position"`
}

func respond(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Cache-Control", "no-store")
	if value != nil {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(code)
	if value != nil {
		_ = json.NewEncoder(w).Encode(value)
	}
}

func fail(w http.ResponseWriter, code int, message string) {
	respond(w, code, map[string]string{"error": message})
}

func serverError(w http.ResponseWriter, err error) {
	// The server log retains the cause; never expose object keys or SDK errors.
	fmt.Fprintf(os.Stderr, "media request failed: %v\n", err)
	fail(w, http.StatusInternalServerError, "media request failed")
}

func (h *Handler) secure(next func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(r.Header.Get("Authorization")) > 128 || len(parts) != 2 ||
			!strings.EqualFold(parts[0], "Bearer") {
			fail(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		me, err := h.accounts.Authenticate(ctx, parts[1])
		if errors.Is(err, account.ErrUnauthorized) {
			fail(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}
		next(w, r, me.User.ID)
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/memories/{id}/media/intents", h.secure(h.intent))
	mux.HandleFunc("POST /api/v1/memories/{id}/media/{assetID}/finalize", h.secure(h.finalize))
	mux.HandleFunc("GET /api/v1/memories/{id}/media", h.secure(h.list))
	mux.HandleFunc("GET /api/v1/memories/{id}/media/{assetID}/download", h.secure(h.download))
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func (h *Handler) intent(w http.ResponseWriter, r *http.Request, userID string) {
	memoryID := r.PathValue("id")
	if !uuid.MatchString(memoryID) {
		fail(w, 400, "invalid memory ID")
		return
	}
	var in intentInput
	code, err := httpapi.DecodeJSON(w, r, &in, 4096)
	if err != nil {
		fail(w, code, err.Error())
		return
	}
	if (in.ContentType != "image/jpeg" && in.ContentType != "image/png") ||
		in.DeclaredBytes <= 0 || in.DeclaredBytes > maxImageBytes {
		fail(w, 400, "use image/jpeg or image/png with 1 to 10485760 bytes")
		return
	}
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var workspaceID string
	err = tx.QueryRow(r.Context(), `
		SELECT w.id::text FROM memories m
		JOIN workspaces w ON w.id=m.workspace_id
		JOIN workspace_members wm ON wm.workspace_id=w.id
		WHERE m.id=$2::uuid AND m.status='draft'
		  AND w.kind='personal' AND w.owner_user_id=$1::uuid
		  AND wm.user_id=$1::uuid AND wm.role='owner'
		FOR UPDATE OF w, m`, userID, memoryID).Scan(&workspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "draft memory not found")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	var count, reserved int64
	err = tx.QueryRow(r.Context(), `
		SELECT count(*), COALESCE(sum(CASE WHEN status='ready' THEN verified_bytes
		  ELSE declared_bytes END), 0)
		FROM media_assets WHERE workspace_id=$1::uuid
		  AND (status='ready' OR (status='pending' AND created_at>now()-interval '15 minutes'))`,
		workspaceID).Scan(&count, &reserved)
	if err != nil {
		serverError(w, err)
		return
	}
	if count >= maxWorkspaceItems || reserved+in.DeclaredBytes > maxWorkspaceBytes {
		fail(w, 429, "image quota reached")
		return
	}
	id, err := newID()
	if err != nil {
		serverError(w, err)
		return
	}
	key := "pending/" + workspaceID + "/" + id
	_, err = tx.Exec(r.Context(), `
		INSERT INTO media_assets
		(id, workspace_id, uploaded_by, object_key, kind, content_type, declared_bytes)
		VALUES ($1::uuid,$2::uuid,$3::uuid,$4,'image',$5,$6)`,
		id, workspaceID, userID, key, in.ContentType, in.DeclaredBytes)
	if err != nil {
		serverError(w, err)
		return
	}
	url, signed, err := h.objects.PresignUpload(r.Context(), key, in.ContentType)
	if err != nil {
		serverError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		serverError(w, err)
		return
	}
	// Content-Type must exactly match the signed value during the client's PUT.
	headers := map[string]string{"Content-Type": in.ContentType}
	for name, values := range signed {
		if len(values) > 0 && !strings.EqualFold(name, "Host") {
			headers[name] = values[0]
		}
	}
	respond(w, 201, map[string]any{
		"asset_id": id, "upload_url": url, "upload_headers": headers,
		"expires_at": time.Now().UTC().Add(10 * time.Minute),
	})
}

// ownedMemory never discloses metadata for another user's workspace.
func (h *Handler) ownedMemory(ctx context.Context, userID, memoryID string) (string, string, error) {
	var workspaceID, status string
	err := h.db.QueryRow(ctx, `
		SELECT m.workspace_id::text, m.status FROM memories m
		JOIN workspaces w ON w.id=m.workspace_id
		JOIN workspace_members wm ON wm.workspace_id=w.id
		WHERE m.id=$2::uuid AND w.kind='personal'
		  AND w.owner_user_id=$1::uuid AND wm.user_id=$1::uuid AND wm.role='owner'`,
		userID, memoryID).Scan(&workspaceID, &status)
	return workspaceID, status, err
}

func memoryAccess(w http.ResponseWriter, err error, status string) bool {
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "memory not found")
		return false
	}
	if err != nil {
		serverError(w, err)
		return false
	}
	if status == "sealed" {
		fail(w, 423, "open the memory before viewing media")
		return false
	}
	return true
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request, userID string) {
	memoryID := r.PathValue("id")
	if !uuid.MatchString(memoryID) {
		fail(w, 400, "invalid memory ID")
		return
	}
	_, status, err := h.ownedMemory(r.Context(), userID, memoryID)
	if !memoryAccess(w, err, status) {
		return
	}
	rows, err := h.db.Query(r.Context(), `
		SELECT a.id::text, a.content_type, a.verified_bytes, ma.position
		FROM memory_assets ma JOIN media_assets a
		  ON a.id=ma.asset_id AND a.workspace_id=ma.workspace_id
		WHERE ma.memory_id=$1::uuid AND a.status='ready'
		ORDER BY ma.position, a.id LIMIT 10`, memoryID)
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()
	items := make([]asset, 0)
	for rows.Next() {
		var a asset
		if err := rows.Scan(&a.ID, &a.ContentType, &a.VerifiedBytes, &a.Position); err != nil {
			serverError(w, err)
			return
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		serverError(w, err)
		return
	}
	respond(w, 200, items)
}

func (h *Handler) download(w http.ResponseWriter, r *http.Request, userID string) {
	memoryID, assetID := r.PathValue("id"), r.PathValue("assetID")
	if !uuid.MatchString(memoryID) || !uuid.MatchString(assetID) {
		fail(w, 400, "invalid ID")
		return
	}
	_, status, err := h.ownedMemory(r.Context(), userID, memoryID)
	if !memoryAccess(w, err, status) {
		return
	}
	var key string
	err = h.db.QueryRow(r.Context(), `
		SELECT a.object_key FROM memory_assets ma
		JOIN media_assets a ON a.id=ma.asset_id AND a.workspace_id=ma.workspace_id
		WHERE ma.memory_id=$1::uuid AND ma.asset_id=$2::uuid AND a.status='ready'`,
		memoryID, assetID).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "image not found")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	url, err := h.objects.PresignDownload(r.Context(), key)
	if err != nil {
		serverError(w, err)
		return
	}
	respond(w, 200, map[string]any{
		"download_url": url, "expires_at": time.Now().UTC().Add(time.Minute),
	})
}
