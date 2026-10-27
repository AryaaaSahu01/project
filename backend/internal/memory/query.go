package memory

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (h *Handler) listFiltered(ctx context.Context, userID string, params url.Values) ([]Memory, error) {
	workspace, before := params.Get("workspace_id"), params.Get("before")
	state, q := params.Get("state"), strings.TrimSpace(params.Get("q"))
	if !validID(workspace) || (before != "" && !validID(before)) || len(q) > 500 {
		return nil, ErrInput
	}

	allowed := map[string]bool{"": true, "all": true, "draft": true, "sealed": true, "ready": true, "returned": true, "capsules": true}
	if !allowed[state] {
		return nil, ErrInput
	}
	comparison, order := "<", "DESC"
	if params.Get("sort") == "oldest" {
		comparison, order = ">", "ASC"
	}
	query := `SELECT ` + selectColumns + ` FROM memories m
	JOIN workspace_members wm ON wm.workspace_id = m.workspace_id
	WHERE wm.user_id = $1::uuid AND m.workspace_id = $2::uuid
	AND ($3 = '' OR (m.status <> 'sealed' AND position(lower($3) in lower(m.title || E'\n' || m.body)) > 0))
	AND ($4 IN ('', 'all') OR ($4 = 'draft' AND m.status = 'draft' )
		OR ($4 = 'sealed' AND m.status = 'sealed' AND m.unlock_at > now())
		OR ($4 = 'ready' AND m.status = 'sealed' AND m.unlock_at <= now())
		OR ($4 = 'returned' AND m.status = 'returned')
		OR ($4 = 'capsules' AND m.status IN ('sealed', 'returned')))
	AND ($5::uuid IS NULL OR (m.created_at, m.id) ` + comparison + `
		(SELECT c.created_at, c.id FROM memories c WHERE c.id=$5::uuid AND C.workspace_id=$2::uuid))
	ORDER BY m.created_at ` + order + `, m.id ` + order + ` LIMIT 100`
	var cursor any
	if before != "" {
		cursor = before
	}
	rows, err := h.db.Query(ctx, query, userID, workspace, q, state, cursor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Memory, 0)
	for rows.Next() {
		item, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (h *Handler) summaryHTTP(w http.ResponseWriter, r *http.Request, userID string) {
	workspace := r.URL.Query().Get("workspace_id")
	if !validID(workspace) {
		respondError(w, ErrInput)
		return
	}
	var draft, sealed, ready, returned int
	var serverTime time.Time
	err := h.db.QueryRow(r.Context(), `SELECT
		count(*) FILTER (WHERE m.status='draft'),
		count(*) FILTER (WHERE m.status='sealed' AND m.unlock_at > now()),
		count(*) FILTER (WHERE m.status='sealed' AND m.unlock_at <= now()),
		count(*) FILTER (WHERE m.status='returned'), now()
		FROM memories m JOIN workspace_members wm ON wm.workspace_id=m.workspace_id
		WHERE wm.user_id=$1::uuid AND m.workspace_id=$2::uuid`, userID, workspace).
		Scan(&draft, &sealed, &ready, &returned, &serverTime)
	if err != nil {
		respondError(w, err)
		return
	}
	respond(w, 200, map[string]any{"draft": draft, "sealed": sealed, "ready": ready, "returned": returned, "server_time": serverTime})
}
