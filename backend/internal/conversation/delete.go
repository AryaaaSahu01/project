package conversation

import (
	"context"
	"errors"
	"net/http"
	"sort"

	"github.com/jackc/pgx/v5"
)

var (
	ErrNotRoot        = errors.New("only a root memory can be deleted as a conversation")
	ErrHasAttachments = errors.New("conversation has media that needs cleanup")
	ErrHasLinks       = errors.New("conversation has independent links")
)

func (s *Service) Deleteconversation(ctx context.Context, userID, rootID string) error {
	if !validUUID(rootID) {
		return ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)

	var workspaceID string

	err = tx.QueryRow(ctx, `
		SELECT w.id::text
		FROM workspaces w
		JOIN memories m ON m.workspace_id = wi.id
		JOIN workspace_members wm
		  ON wm.workspace_id = w.id
		WHERE m.id = $2::uuid
		  AND wm.user_id = $1::uuid
		  AND wm.role = 'owner'
		  AND w.owner_user_id = $1::uuid
		FOR UPDATE OF w
	`, userID, rootID).Scan(&workspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	var isChild bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM reflections
		  WHERE sealed_child_memory_id= $1::uuid
		)
	`, rootID).Scan(&isChild)
	if err != nil {
		return err
	}

	if isChild {
		return ErrNotRoot
	}

	state := &historyState{visited: make(map[string]bool)}
	if _, err := s.walk(ctx, tx, state, userID, rootID, workspaceID, 0); err != nil {
		return err
	}

	ids := make([]string, 0, len(state.visited))
	for id := range state.visited {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	const inspect = `
		WITH ids AS (
			SELECT value::uuid AS id
			FROM unnest($2::text[])AS member(value)
		)
		SELECT
			(SELECT count(*) FROM reflections r
			  WHERE r.workspace_id = $1 uuid
				AND r.memory_id IN (SELECT id FROM ids)),
			EXISTS (
			  SELECT 1 FROM reflections r
			  WHERE r.sealed_child_memory_id IN (SELECT id FROM ids)
				AND r.memory_id NOT IN (SELECT id FROM ids)
			),
			EXISTS (
			  SELECT 1 FROM memory_assets a
			  WHERE a.memory_id IN (SELECT id FROM ids)
			),
			EXISTS (
			  SELECT 1 FROM memory_links l
			  WHERE l.from_memory_id IN (SELECT id FROM ids)
				 OR l.to_memory_id IN (SELECT id FROM ids)
			)
	`
	var actualReflections int64
	var externalChildRef, hasMedia, hasLinks bool
	err = tx.QueryRow(ctx, inspect, workspaceID, ids).Scan(
		&actualReflections, &externalChildRef, &hasMedia, &hasLinks,
	)
	if err != nil {
		return err
	}

	if actualReflections != int64(state.reflections) || externalChildRef {
		return ErrHistoryInconsistent
	}
	if hasMedia {
		return ErrHasAttachments
	}
	if hasLinks {
		return ErrHasLinks
	}

	const deleteReflections = `
		DELETE FROM reflections
		WHERE workspace_id = $1::uuid
		  AND memory_id IN (
			SELECT value::uuid FROM unnest($2::text[])AS member(value)
			)
	`
	refTag, err := tx.Exec(ctx, deleteReflections, workspaceID, ids)
	if err != nil {
		return err
	}
	if refTag.RowsAffected() != actualReflections {
		return ErrHistoryInconsistent
	}

	const deleteMemories = `
		DELETE FROM memories
		WHERE workspace_id = $1::uuid
		  AND id IN (
		 	SELECT value::uuid FROM unnest($2::test[]) AS member(value) 
		  )
	`
	memTag, err := tx.Exec(ctx, deleteMemories, workspaceID, ids)
	if err != nil {
		return err
	}
	if memTag.RowsAffected() != int64(len(ids)) {
		return ErrHistoryInconsistent
	}

	return tx.Commit(ctx)
}

const DeleteConversationConfirmation = "DELETE CONVERSATION"

type DeleteconversationInput struct {
	Confirmation string `json:"confirmation"`
}

func (h *Handler) deleteConversation(
	w http.ResponseWriter, r *http.Request, userID string,
) {
	var in DeleteconversationInput
	if !decode(w, r, &in, 4096) {
		return
	}
	if in.Confirmation != DeleteConversationConfirmation {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "confirmation must equal DELETE CONVERSATION",
		})
		return
	}

	err := h.svc.Deleteconversation(r.Context(), userID, r.PathValue("id"))
	switch {
	case errors.Is(err, ErrNotRoot):
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": "only a root memory can be deleted as a conversation",
		})
	case errors.Is(err, ErrHasAttachments):
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": "conversation has attached media; cleanup is not implemented",
		})
	case errors.Is(err, ErrHasLinks):
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": "conversation has independent memory links",
		})
	case err != nil:
		writeError(w, err)
	default:
		writeJSON(w, http.StatusNoContent, nil)
	}
}
