package media

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"journalapp/internal/account"
	"journalapp/internal/platform/objectstore"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db       *pgxpool.Pool
	accounts *account.Service
	objects  objectstore.Store
	uploads  chan struct{}
}

func NewHandler(db *pgxpool.Pool, accounts *account.Service, objects objectstore.Store) *Handler {
	return &Handler{db: db, accounts: accounts, objects: objects, uploads: make(chan struct{}, 2)}
}

type asset struct {
	ID            string `json:"id"`
	ContentType   string `json:"content_type"`
	VerifiedBytes int64  `json:"verified_bytes"`
	Position      int    `json:"position"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	Source        string `json:"source"`
}

const assetColumns = `a.id::text,a.content_type,a.verified_bytes,ma.position,a.original_name,a.kind,a.source`

func readAsset(row interface{ Scan(...any) error }) (asset, error) {
	var a asset
	err := row.Scan(&a.ID, &a.ContentType, &a.VerifiedBytes, &a.Position, &a.Name, &a.Kind, &a.Source)
	return a, err
}
func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if value != nil {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(status)
	if value != nil {
		_ = json.NewEncoder(w).Encode(value)
	}
}
func fail(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}
func serverError(w http.ResponseWriter, err error) {
	log.Printf("media request failed: %v", err)
	fail(w, 503, "Your attachment could not be reached. Please try again.")
}
func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
func (h *Handler) secure(next func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Minute)
		defer cancel()
		r = r.WithContext(ctx)
		fields := strings.Fields(r.Header.Get("Authorization"))
		if len(r.Header.Get("Authorization")) > 128 || len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
			fail(w, 401, "Sign in to use attachments.")
			return
		}
		me, err := h.accounts.Authenticate(ctx, fields[1])
		if errors.Is(err, account.ErrUnauthorized) {
			fail(w, 401, "Your sign-in has expired. Sign in again.")
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
	mux.HandleFunc("GET /api/v1/media/config", h.secure(func(w http.ResponseWriter, r *http.Request, _ string) {
		respond(w, 200, map[string]any{"formats": rules, "max_memory_items": maxMemoryItems, "max_workspace_bytes": maxWorkspaceBytes, "max_workspace_items": maxWorkspaceItems})
	}))
	mux.HandleFunc("POST /api/v1/memories/{id}/media", h.secure(h.upload))
	mux.HandleFunc("GET /api/v1/memories/{id}/media", h.secure(h.list))
	mux.HandleFunc("DELETE /api/v1/memories/{id}/media/{assetID}", h.secure(h.remove))
	mux.HandleFunc("GET /api/v1/memories/{id}/media/{assetID}/content", h.secure(h.content))
	mux.HandleFunc("HEAD /api/v1/memories/{id}/media/{assetID}/content", h.secure(h.content))
	// Retire the prototype's public signed-link flow. No new bearer URLs are issued.
	retired := h.secure(func(w http.ResponseWriter, r *http.Request, _ string) {
		fail(w, 410, "Use the private attachment upload and content endpoints.")
	})
	mux.HandleFunc("POST /api/v1/memories/{id}/media/intents", retired)
	mux.HandleFunc("POST /api/v1/memories/{id}/media/{assetID}/finalize", retired)
	mux.HandleFunc("GET /api/v1/memories/{id}/media/{assetID}/download", retired)
}
func validID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, c := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}
func (h *Handler) ownedMemory(ctx context.Context, userID, memoryID string) (string, string, error) {
	var workspaceID, status string
	err := h.db.QueryRow(ctx, `SELECT m.workspace_id::text,m.status FROM memories m
        JOIN workspaces w ON w.id=m.workspace_id JOIN workspace_members wm ON wm.workspace_id=w.id
        WHERE m.id=$2::uuid AND w.kind='personal' AND w.owner_user_id=$1::uuid
        AND wm.user_id=$1::uuid AND wm.role='owner'`, userID, memoryID).Scan(&workspaceID, &status)
	return workspaceID, status, err
}
func memoryAccess(w http.ResponseWriter, err error, status string) bool {
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "Memory not found.")
		return false
	}
	if err != nil {
		serverError(w, err)
		return false
	}
	if status == "sealed" {
		fail(w, 423, "Open the capsule before viewing attachments.")
		return false
	}
	return true
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request, userID string) {
	id := r.PathValue("id")
	if !validID(id) {
		fail(w, 400, "Invalid memory.")
		return
	}
	_, status, err := h.ownedMemory(r.Context(), userID, id)
	if !memoryAccess(w, err, status) {
		return
	}
	rows, err := h.db.Query(r.Context(), `SELECT `+assetColumns+` FROM memory_assets ma
        JOIN media_assets a ON a.id=ma.asset_id AND a.workspace_id=ma.workspace_id
        WHERE ma.memory_id=$1::uuid AND a.status='ready' ORDER BY ma.position,a.created_at,a.id LIMIT 10`, id)
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()
	items := make([]asset, 0)
	for rows.Next() {
		a, err := readAsset(rows)
		if err != nil {
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
func (h *Handler) content(w http.ResponseWriter, r *http.Request, userID string) {
	id, assetID := r.PathValue("id"), r.PathValue("assetID")
	if !validID(id) || !validID(assetID) {
		fail(w, 400, "Invalid attachment.")
		return
	}
	// One query authorizes the memory, attachment and current sealed state.
	var key, contentType, status, name string
	var size int64
	err := h.db.QueryRow(r.Context(), `SELECT a.object_key,a.content_type,a.verified_bytes,m.status,a.original_name
        FROM memories m JOIN workspaces w ON w.id=m.workspace_id
        JOIN workspace_members wm ON wm.workspace_id=w.id
        JOIN memory_assets ma ON ma.memory_id=m.id AND ma.workspace_id=m.workspace_id
        JOIN media_assets a ON a.id=ma.asset_id AND a.workspace_id=ma.workspace_id
        WHERE m.id=$2::uuid AND a.id=$3::uuid AND a.status='ready'
        AND w.kind='personal' AND w.owner_user_id=$1::uuid AND wm.user_id=$1::uuid AND wm.role='owner'`, userID, id, assetID).
		Scan(&key, &contentType, &size, &status, &name)
	if !memoryAccess(w, err, status) {
		return
	}
	offset, length, partial, err := parseRange(r.Header.Get("Range"), size)
	if err != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
		fail(w, 416, "Requested range is unavailable.")
		return
	}
	var bodyErr error
	if r.Method != http.MethodHead {
		body, err := h.objects.ReadRange(r.Context(), key, offset, length)
		if err != nil {
			serverError(w, err)
			return
		}
		defer body.Close()
		setContentHeaders(w, contentType, name, size, offset, length, partial)
		if partial {
			w.WriteHeader(http.StatusPartialContent)
		}
		_, bodyErr = copyBytes(w, body, length)
	} else {
		setContentHeaders(w, contentType, name, size, offset, length, partial)
		if partial {
			w.WriteHeader(http.StatusPartialContent)
		}
	}
	if bodyErr != nil {
		log.Printf("attachment stream interrupted: %v", bodyErr)
	}
}
func setContentHeaders(w http.ResponseWriter, contentType, name string, size, offset, length int64, partial bool) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": safeName(name)}))
	if partial {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, offset+length-1, size))
	}
}

// A single RFC byte range covers normal audio/video seeking. Reject multi-range
// requests rather than constructing multipart responses or unbounded allocations.
func parseRange(value string, size int64) (int64, int64, bool, error) {
	bad := errors.New("invalid range")
	if size <= 0 {
		return 0, 0, false, bad
	}
	if value == "" {
		return 0, size, false, nil
	}
	if !strings.HasPrefix(value, "bytes=") || strings.Contains(value, ",") {
		return 0, 0, false, bad
	}
	a, b, ok := strings.Cut(strings.TrimPrefix(value, "bytes="), "-")
	if !ok {
		return 0, 0, false, bad
	}
	if a == "" {
		n, err := strconv.ParseInt(b, 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, false, bad
		}
		if n > size {
			n = size
		}
		return size - n, n, true, nil
	}
	start, err := strconv.ParseInt(a, 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, false, bad
	}
	end := size - 1
	if b != "" {
		end, err = strconv.ParseInt(b, 10, 64)
		if err != nil || end < start {
			return 0, 0, false, bad
		}
		if end >= size {
			end = size - 1
		}
	}
	return start, end - start + 1, true, nil
}
