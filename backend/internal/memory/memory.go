package memory

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"journalapp/internal/account"
	"journalapp/internal/httpapi"
)

var (
	ErrInput    = errors.New("invalid memory input")
	ErrNotFound = errors.New("memory not found")
	ErrLocked   = errors.New("memory is still sealed")
	ErrConflict = errors.New("memory state conflict")

	uuidPattern = regexp.MustCompile(
		`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`,
	)
)

type Memory struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	CreatedBy   string     `json:"created_by"`
	Title       *string    `json:"title,omitempty"`
	Body        *string    `json:"body,omitempty"`
	Status      string     `json:"status"`
	SealedAt    *time.Time `json:"sealed_at,omitempty"`
	UnlockAt    *time.Time `json:"unlock_at,omitempty"`
	CanOpen     bool       `json:"can_open"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type CreateInput struct {
	ID          string `json:"id,omitempty"`
	WorkspaceID string `json:"workspace_id"`
	Title       string `json:"title"`
	Body        string `json:"body"`
}

type UpdateInput struct {
	Title             *string    `json:"title,omitempty"`
	Body              *string    `json:"body,omitempty"`
	ExpectedUpdatedAt *time.Time `json:"expected_updated_at,omitempty"`
}

type SealInput struct {
	UnlockAt time.Time `json:"unlock_at"`
}

type Handler struct {
	db       *pgxpool.Pool
	accounts *account.Service
}

func NewHandler(db *pgxpool.Pool, accounts *account.Service) *Handler {
	return &Handler{db: db, accounts: accounts}
}

const selectColumns = `
	m.id::text, m.workspace_id::text, m.created_by::text,
	m.title, m.body, m.status, m.sealed_at, m.unlock_at,
	m.created_at, m.updated_at,
	(m.status = 'sealed' AND m.unlock_at <= now())
`

type scanner interface {
	Scan(dest ...any) error
}

func scanMemory(row scanner) (Memory, error) {
	var m Memory
	var title, body string

	err := row.Scan(
		&m.ID, &m.WorkspaceID, &m.CreatedBy,
		&title, &body, &m.Status, &m.SealedAt, &m.UnlockAt,
		&m.CreatedAt, &m.UpdatedAt, &m.CanOpen,
	)
	if err != nil {
		return Memory{}, err
	}

	// Never serialize sealed writing before an authorized Open.
	if m.Status != "sealed" {
		m.Title = &title
		m.Body = &body
	}

	return m, nil
}

func validID(id string) bool {
	return uuidPattern.MatchString(id)
}

func validWriting(title, body string) bool {
	return utf8.ValidString(title) &&
		utf8.ValidString(body) &&
		utf8.RuneCountInString(title) <= 500 &&
		len(body) <= 1<<20 &&
		(strings.TrimSpace(title) != "" ||
			strings.TrimSpace(body) != "")
}

func (h *Handler) create(
	ctx context.Context, userID string, in CreateInput,
) (Memory, error) {
	if !validID(in.WorkspaceID) || (in.ID != "" && !validID(in.ID)) ||
		!validWriting(in.Title, in.Body) {
		return Memory{}, ErrInput
	}

	// Only workspace owners/editors may create memories.
	const query = `
		INSERT INTO memories AS m
			(id, workspace_id, created_by, title, body)
		SELECT COALESCE(NULLIF($5, '')::uuid, gen_random_uuid()), $1::uuid, $2::uuid, $3, $4
		WHERE EXISTS (
			SELECT 1 FROM workspace_members wm
			WHERE wm.workspace_id = $1::uuid
				AND wm.user_id = $2::uuid
				AND wm.role IN ('owner', 'editor')
		)
		ON CONFLICT (id) DO UPDATE SET id = m.id
		WHERE m.workspace_id = EXCLUDED.workspace_id AND m.created_by = EXCLUDED.created_by
		  AND m.title = EXCLUDED.title AND m.body = EXCLUDED.body AND m.status = 'draft'
		RETURNING id::text
	`

	var id string
	err := h.db.QueryRow(
		ctx, query, in.WorkspaceID, userID,
		in.Title, in.Body, in.ID,
	).Scan(&id)

	if errors.Is(err, pgx.ErrNoRows) {
		return Memory{}, ErrNotFound
	}
	if err != nil {
		return Memory{}, err
	}

	return h.get(ctx, userID, id)
}

func (h *Handler) get(
	ctx context.Context, userID, id string,
) (Memory, error) {
	if !validID(id) {
		return Memory{}, ErrInput
	}

	query := `
		SELECT ` + selectColumns + `
		FROM memories m
		JOIN workspace_members wm
			ON wm.workspace_id = m.workspace_id
		WHERE m.id = $2::uuid
			AND wm.user_id = $1::uuid
	`

	m, err := scanMemory(
		h.db.QueryRow(ctx, query, userID, id),
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Memory{}, ErrNotFound
	}

	return m, err
}

func (h *Handler) update(
	ctx context.Context, userID, id string, in UpdateInput,
) (Memory, error) {
	if !validID(id) || (in.Title == nil && in.Body == nil) {
		return Memory{}, ErrInput
	}

	if in.Title != nil &&
		(!utf8.ValidString(*in.Title) ||
			utf8.RuneCountInString(*in.Title) > 500) {
		return Memory{}, ErrInput
	}
	if in.Body != nil &&
		(!utf8.ValidString(*in.Body) ||
			len(*in.Body) > 1<<20) {
		return Memory{}, ErrInput
	}

	query := `
		UPDATE memories AS m SET
			title = COALESCE($3::text, m.title),
			body = COALESCE($4::text, m.body),
			updated_at = now()
		WHERE m.id = $2::uuid
			AND m.status = 'draft'
			AND ($5::timestamptz IS NULL OR m.updated_at = $5
			  OR (m.title = COALESCE($3::text,m.title) AND m.body = COALESCE($4::text,m.body)))
			AND (
				btrim(COALESCE($3::text, m.title)) <> ''
				OR btrim(COALESCE($4::text, m.body)) <> ''
			)
			AND EXISTS (
				SELECT 1 FROM workspace_members wm
				WHERE wm.workspace_id = m.workspace_id
					AND wm.user_id = $1::uuid
					AND wm.role IN ('owner', 'editor')
			)
		RETURNING ` + selectColumns

	m, err := scanMemory(
		h.db.QueryRow(ctx, query, userID, id,
			in.Title, in.Body, in.ExpectedUpdatedAt),
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Memory{}, ErrConflict
	}
	return m, err
}

func (h *Handler) seal(
	ctx context.Context, userID, id string, unlockAt time.Time,
) (Memory, error) {
	if !validID(id) ||
		!unlockAt.After(time.Now().UTC().Add(time.Minute)) {
		return Memory{}, ErrInput
	}

	query := `
		UPDATE memories AS m SET
			status = 'sealed',
			sealed_at = now(),
			unlock_at = $3,
			updated_at = now()
		WHERE m.id = $2::uuid
			AND m.status = 'draft'
			AND $3 > now()
			AND EXISTS (
				SELECT 1 FROM workspace_members wm
				WHERE wm.workspace_id = m.workspace_id
					AND wm.user_id = $1::uuid
					AND wm.role IN ('owner', 'editor')
			)
		RETURNING ` + selectColumns

	m, err := scanMemory(
		h.db.QueryRow(ctx, query, userID, id, unlockAt.UTC()),
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Memory{}, ErrConflict
	}
	return m, err
}

func (h *Handler) open(
	ctx context.Context, userID, id string,
) (Memory, error) {
	if !validID(id) {
		return Memory{}, ErrInput
	}

	query := `
		UPDATE memories AS m SET
			status = 'returned',
			updated_at = now()
		WHERE m.id = $2::uuid
			AND m.status = 'sealed'
			AND m.unlock_at <= now()
			AND EXISTS (
				SELECT 1 FROM workspace_members wm
				WHERE wm.workspace_id = m.workspace_id
					AND wm.user_id = $1::uuid
			)
		RETURNING ` + selectColumns

	m, err := scanMemory(
		h.db.QueryRow(ctx, query, userID, id),
	)
	if err == nil {
		return m, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Memory{}, err
	}

	// Distinguish locked, returned and inaccessible memories.
	current, err := h.get(ctx, userID, id)
	if err != nil {
		return Memory{}, err
	}

	switch current.Status {
	case "sealed":
		return Memory{}, ErrLocked
	case "returned":
		return current, nil
	default:
		return Memory{}, ErrConflict
	}
}

func (h *Handler) remove(
	ctx context.Context, userID, id string,
) error {
	if !validID(id) {
		return ErrInput
	}
	tx, err := h.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Link creation locks both endpoint memories. This lock makes the link
	// check and deletion atomic with respect to concurrent link creation.
	var status string
	err = tx.QueryRow(ctx, `
		SELECT m.status FROM memories m
		JOIN workspace_members wm ON wm.workspace_id=m.workspace_id
		WHERE m.id=$2::uuid AND wm.user_id=$1::uuid
		  AND wm.role IN ('owner','editor') FOR UPDATE OF m`,
		userID, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	var attachedOrLinked bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM memory_assets WHERE memory_id=$1::uuid)
		    OR EXISTS(SELECT 1 FROM memory_links
		      WHERE from_memory_id=$1::uuid OR to_memory_id=$1::uuid)
		    OR EXISTS(SELECT 1 FROM reflections
		      WHERE memory_id=$1::uuid OR sealed_child_memory_id=$1::uuid)`, id).
		Scan(&attachedOrLinked)
	if err != nil {
		return err
	}
	if attachedOrLinked {
		return ErrConflict
	}
	tag, err := tx.Exec(ctx, `DELETE FROM memories WHERE id=$1::uuid`, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrConflict
		}
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return tx.Commit(ctx)
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Cache-Control", "no-store")
	if value == nil {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func respondError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInput):
		respond(w, 400, map[string]string{
			"error": "invalid memory input",
		})
	case errors.Is(err, ErrNotFound):
		respond(w, 404, map[string]string{
			"error": "memory not found",
		})
	case errors.Is(err, ErrLocked):
		respond(w, 423, map[string]string{
			"error": "memory is still sealed",
		})
	case errors.Is(err, ErrConflict):
		respond(w, 409, map[string]string{
			"error": "memory state conflict",
		})
	default:
		respond(w, 500, map[string]string{
			"error": "internal server error",
		})
	}
}

func decode(
	w http.ResponseWriter, r *http.Request,
	dst any, maxBytes int64,
) bool {
	status, err := httpapi.DecodeJSON(w, r, dst, maxBytes)
	if err != nil {
		respond(w, status, map[string]string{
			"error": err.Error(),
		})
		return false
	}
	return true
}

type operation func(http.ResponseWriter, *http.Request, string)

// Every memory endpoint resolves the user from a verified session.
// The client cannot choose its own user ID.
func (h *Handler) protected(next operation) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(
			r.Context(), 10*time.Second,
		)
		defer cancel()

		r = r.WithContext(ctx)

		auth := strings.Fields(r.Header.Get("Authorization"))
		if len(auth) != 2 ||
			!strings.EqualFold(auth[0], "Bearer") {
			respond(w, 401, map[string]string{
				"error": "unauthorized",
			})
			return
		}

		a, err := h.accounts.Authenticate(ctx, auth[1])
		if err != nil {
			if errors.Is(err, account.ErrUnauthorized) {
				respond(w, 401, map[string]string{
					"error": "unauthorized",
				})
			} else {
				respond(w, 500, map[string]string{
					"error": "internal server error",
				})
			}
			return
		}

		next(w, r, a.User.ID)
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/memories", h.protected(h.createHTTP))
	mux.HandleFunc("GET /api/v1/memories", h.protected(h.listHTTP))
	mux.HandleFunc("GET /api/v1/memories/summary", h.protected(h.summaryHTTP))
	mux.HandleFunc("GET /api/v1/memories/{id}", h.protected(h.getHTTP))
	mux.HandleFunc("PATCH /api/v1/memories/{id}", h.protected(h.updateHTTP))
	mux.HandleFunc("DELETE /api/v1/memories/{id}", h.protected(h.deleteHTTP))
	mux.HandleFunc("POST /api/v1/memories/{id}/seal", h.protected(h.sealHTTP))
	mux.HandleFunc("POST /api/v1/memories/{id}/open", h.protected(h.openHTTP))
}

func (h *Handler) createHTTP(w http.ResponseWriter, r *http.Request, userID string) {
	var input CreateInput
	if !decode(w, r, &input, httpapi.MaxEntryBytes) {
		return
	}

	m, err := h.create(r.Context(), userID, input)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusCreated, m)
}

func (h *Handler) listHTTP(w http.ResponseWriter, r *http.Request, userID string) {
	items, err := h.listFiltered(r.Context(), userID, r.URL.Query())
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, items)
}

func (h *Handler) getHTTP(w http.ResponseWriter, r *http.Request, userID string) {
	m, err := h.get(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, m)
}

func (h *Handler) updateHTTP(w http.ResponseWriter, r *http.Request, userID string) {
	var input UpdateInput
	if !decode(w, r, &input, httpapi.MaxEntryBytes) {
		return
	}

	m, err := h.update(
		r.Context(), userID, r.PathValue("id"), input,
	)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, m)
}

func (h *Handler) sealHTTP(w http.ResponseWriter, r *http.Request, userID string) {
	var input SealInput
	if !decode(w, r, &input, httpapi.MaxSealBytes) {
		return
	}

	m, err := h.seal(
		r.Context(), userID, r.PathValue("id"),
		input.UnlockAt,
	)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, m)
}

func (h *Handler) openHTTP(w http.ResponseWriter, r *http.Request, userID string) {
	m, err := h.open(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusOK, m)
}

func (h *Handler) deleteHTTP(w http.ResponseWriter, r *http.Request, userID string) {
	err := h.remove(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, http.StatusNoContent, nil)
}
