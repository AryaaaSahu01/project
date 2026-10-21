// Package memorylink stores independent relationships between memories.
// Reflection-child lineage is a different concept and is never modified here.
package memorylink

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid  = errors.New("invalid link input")
	ErrNotFound = errors.New("memory or link not found")
	ErrConflict = errors.New("link already exists")
	ErrTooMany  = errors.New("memory link limit reached")
	uuid        = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

const maxLinksPerMemory = 100

type Link struct {
	ID            string    `json:"id"`
	FromMemoryID  string    `json:"from_memory_id"`
	ToMemoryID    string    `json:"to_memory_id"`
	Relation      string    `json:"relation"`
	CreatedAt     time.Time `json:"created_at"`
	OtherMemoryID string    `json:"other_memory_id,omitempty"`
	Direction     string    `json:"direction,omitempty"`
}

type NearbyMemory struct {
	MemoryID string `json:"memory_id"`
	Distance int    `json:"distance"`
}

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

func validRelation(v string) bool {
	switch v {
	case "related", "continues", "same_event", "inspired_by":
		return true
	default:
		return false
	}
}

func (s *Service) ownedWorkspace(ctx context.Context, userID, memoryID string) (string, error) {
	if !uuid.MatchString(memoryID) {
		return "", ErrInvalid
	}
	var workspaceID string
	err := s.db.QueryRow(ctx, `
		SELECT m.workspace_id::text FROM memories m
		JOIN workspaces w ON w.id=m.workspace_id
		JOIN workspace_members wm ON wm.workspace_id=w.id
		WHERE m.id=$2::uuid AND w.kind='personal'
		  AND w.owner_user_id=$1::uuid
		  AND wm.user_id=$1::uuid AND wm.role='owner'`,
		userID, memoryID).Scan(&workspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return workspaceID, err
}

func (s *Service) Create(ctx context.Context, userID, fromID, toID, relation string) (Link, error) {
	if !uuid.MatchString(fromID) || !uuid.MatchString(toID) ||
		strings.EqualFold(fromID, toID) || !validRelation(relation) {
		return Link{}, ErrInvalid
	}
	fromID, toID = strings.ToLower(fromID), strings.ToLower(toID)
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Link{}, err
	}
	defer tx.Rollback(ctx)
	// Lock both endpoints in ID order. A concurrent link insertion or draft
	// deletion touching either memory must wait until this transaction commits.
	rows, err := tx.Query(ctx, `
		SELECT m.id::text, m.workspace_id::text FROM memories m
		JOIN workspaces w ON w.id=m.workspace_id
		JOIN workspace_members wm ON wm.workspace_id=w.id
		WHERE m.id IN ($2::uuid,$3::uuid)
		  AND w.kind='personal' AND w.owner_user_id=$1::uuid
		  AND wm.user_id=$1::uuid AND wm.role='owner'
		ORDER BY m.id FOR UPDATE OF m`, userID, fromID, toID)
	if err != nil {
		return Link{}, err
	}
	workspaceID := ""
	found := 0
	for rows.Next() {
		var id, currentWorkspace string
		if err := rows.Scan(&id, &currentWorkspace); err != nil {
			rows.Close()
			return Link{}, err
		}
		if workspaceID != "" && workspaceID != currentWorkspace {
			rows.Close()
			return Link{}, ErrNotFound
		}
		workspaceID = currentWorkspace
		found++
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Link{}, err
	}
	if found != 2 {
		return Link{}, ErrNotFound
	}
	var fromCount, toCount int
	err = tx.QueryRow(ctx, `
		SELECT
		  count(*) FILTER (WHERE from_memory_id=$2::uuid OR to_memory_id=$2::uuid),
		  count(*) FILTER (WHERE from_memory_id=$3::uuid OR to_memory_id=$3::uuid)
		FROM memory_links WHERE workspace_id=$1::uuid`,
		workspaceID, fromID, toID).Scan(&fromCount, &toCount)
	if err != nil {
		return Link{}, err
	}
	if fromCount >= maxLinksPerMemory || toCount >= maxLinksPerMemory {
		return Link{}, ErrTooMany
	}
	var link Link
	err = tx.QueryRow(ctx, `
		INSERT INTO memory_links
		  (workspace_id, from_memory_id, to_memory_id, relation, created_by)
		VALUES ($1::uuid,$2::uuid,$3::uuid,$4,$5::uuid)
		RETURNING id::text, from_memory_id::text, to_memory_id::text,
		  relation, created_at`, workspaceID, fromID, toID, relation, userID).
		Scan(&link.ID, &link.FromMemoryID, &link.ToMemoryID, &link.Relation, &link.CreatedAt)
	if err != nil {
		var dbError *pgconn.PgError
		if errors.As(err, &dbError) && dbError.Code == "23505" {
			return Link{}, ErrConflict
		}
		return Link{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Link{}, err
	}
	return link, nil
}

func (s *Service) List(ctx context.Context, userID, memoryID string) ([]Link, error) {
	workspaceID, err := s.ownedWorkspace(ctx, userID, memoryID)
	if err != nil {
		return nil, err
	}
	memoryID = strings.ToLower(memoryID)
	rows, err := s.db.Query(ctx, `
		SELECT id::text, from_memory_id::text, to_memory_id::text,
		  relation, created_at
		FROM memory_links WHERE workspace_id=$1::uuid
		  AND (from_memory_id=$2::uuid OR to_memory_id=$2::uuid)
		ORDER BY created_at, id LIMIT 101`, workspaceID, memoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Link, 0)
	for rows.Next() {
		var link Link
		if err := rows.Scan(&link.ID, &link.FromMemoryID, &link.ToMemoryID,
			&link.Relation, &link.CreatedAt); err != nil {
			return nil, err
		}
		if link.FromMemoryID == memoryID {
			link.OtherMemoryID, link.Direction = link.ToMemoryID, "outgoing"
		} else {
			link.OtherMemoryID, link.Direction = link.FromMemoryID, "incoming"
		}
		result = append(result, link)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result) > maxLinksPerMemory {
		return nil, ErrTooMany
	}
	return result, nil
}

func (s *Service) Delete(ctx context.Context, userID, memoryID, linkID string) error {
	if !uuid.MatchString(linkID) {
		return ErrInvalid
	}
	workspaceID, err := s.ownedWorkspace(ctx, userID, memoryID)
	if err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx, `
		DELETE FROM memory_links WHERE id=$1::uuid
		  AND workspace_id=$2::uuid
		  AND (from_memory_id=$3::uuid OR to_memory_id=$3::uuid)`,
		linkID, workspaceID, memoryID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

// Traverse performs a bounded breadth-first walk. Every edge remains inside
// the owner's personal workspace; sealed titles and bodies are never queried.
func (s *Service) Traverse(ctx context.Context, userID, rootID string, depth int) ([]NearbyMemory, error) {
	if depth < 1 || depth > 3 {
		return nil, ErrInvalid
	}
	workspaceID, err := s.ownedWorkspace(ctx, userID, rootID)
	if err != nil {
		return nil, err
	}
	rootID = strings.ToLower(rootID)
	seen := map[string]bool{rootID: true}
	frontier := []string{rootID}
	result := make([]NearbyMemory, 0)
	for distance := 1; distance <= depth && len(frontier) > 0; distance++ {
		rows, err := s.db.Query(ctx, `
			SELECT from_memory_id::text, to_memory_id::text
			FROM memory_links WHERE workspace_id=$1::uuid
			  AND (from_memory_id IN (
		    SELECT value::uuid FROM unnest($2::text[]) AS member(value))
		    OR to_memory_id IN (
		    SELECT value::uuid FROM unnest($2::text[]) AS member(value)))
			ORDER BY id LIMIT 10001`, workspaceID, frontier)
		if err != nil {
			return nil, err
		}
		frontierSet := make(map[string]bool, len(frontier))
		for _, id := range frontier {
			frontierSet[id] = true
		}
		next := make([]string, 0)
		edges := 0
		for rows.Next() {
			var fromID, toID string
			if err := rows.Scan(&fromID, &toID); err != nil {
				rows.Close()
				return nil, err
			}
			edges++
			if edges > 10000 {
				rows.Close()
				return nil, ErrTooMany
			}
			other := fromID
			if frontierSet[fromID] {
				other = toID
			}
			if !seen[other] {
				seen[other] = true
				result = append(result, NearbyMemory{MemoryID: other, Distance: distance})
				next = append(next, other)
				if len(result) > maxLinksPerMemory {
					rows.Close()
					return nil, ErrTooMany
				}
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		frontier = next
	}
	return result, nil
}
