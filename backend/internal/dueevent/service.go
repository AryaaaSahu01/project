// Package dueevent implements persistent, idempotent return events for
// personal-workspace memories. Shared-workspace per-recipient notifications
// will need a separate delivery model.
package dueevent

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const KindCapsuleDue = "capsule_due"

var (
	ErrInvalidID = errors.New("invalid event ID")
	ErrNotFound  = errors.New("event not found or not eligible")
	uuidPattern  = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

type Event struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	MemoryID    string    `json:"memory_id"`
	Kind        string    `json:"kind"`
	Status      string    `json:"status"`
	UnlockAt    time.Time `json:"unlock_at"`
	RecordedAt  time.Time `json:"recorded_at"`
}

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func validUUID(s string) bool { return uuidPattern.MatchString(s) }

// ProcessDue reconciles stale events and creates notifications for due sealed
// memories in personal workspaces. The unique (memory_id, kind) constraint
// plus ON CONFLICT DO NOTHING makes overlapping workers idempotent.
// Work is bounded per invocation; rerun periodically to drain a backlog.
func (s *Service) ProcessDue(ctx context.Context) (created, cancelled int64, err error) {
	// A returned memory is no longer waiting to be unlocked. Do not display a
	// stale pending event after the user has already opened the memory.
	cancelTag, err := s.db.Exec(ctx, `
		UPDATE return_events AS e
		SET status = 'cancelled'
		FROM memories AS m
		WHERE e.memory_id = m.id
		  AND e.workspace_id = m.workspace_id
		  AND e.status = 'pending'
		  AND m.status <> 'sealed'
	`)
	if err != nil {
		return 0, 0, err
	}
	cancelled = cancelTag.RowsAffected()

	// NOT EXISTS avoids starvation once there are more than 500 due records.
	// SKIP LOCKED avoids waiting on a concurrently sealed/opened/deleted memory.
	const insert = `
		WITH batch AS (
			SELECT m.id, m.workspace_id
			FROM memories AS m
			JOIN workspaces AS w ON w.id = m.workspace_id
			WHERE m.status = 'sealed'
			  AND m.unlock_at <= now()
			  AND w.kind = 'personal'
			  AND NOT EXISTS (
				SELECT 1 FROM return_events AS e
				WHERE e.memory_id = m.id
				  AND e.kind = 'capsule_due'
			  )
			ORDER BY m.unlock_at, m.id
			LIMIT 500
			FOR UPDATE OF m SKIP LOCKED
		)
		INSERT INTO return_events (workspace_id, memory_id, kind, status)
		SELECT b.workspace_id, b.id, 'capsule_due', 'pending'
		FROM batch AS b
		WHERE true
		ON CONFLICT (memory_id, kind) DO NOTHING
	`
	insertTag, err := s.db.Exec(ctx, insert)
	if err != nil {
		return 0, cancelled, err
	}
	return insertTag.RowsAffected(), cancelled, nil
}

// ListEligible deliberately returns metadata only, never sealed title/body.
// This version is personal-workspace-only: events are not per-recipient yet.
func (s *Service) ListEligible(ctx context.Context, userID string) ([]Event, error) {
	const query = `
		SELECT e.id::text, e.workspace_id::text, e.memory_id::text,
		       e.kind, e.status, m.unlock_at, e.recorded_at
		FROM return_events AS e
		JOIN memories AS m
		  ON m.id = e.memory_id AND m.workspace_id = e.workspace_id
		JOIN workspaces AS w ON w.id = m.workspace_id
		JOIN workspace_members AS wm ON wm.workspace_id = w.id
		WHERE wm.user_id = $1::uuid
		  AND wm.role = 'owner'
		  AND w.owner_user_id = $1::uuid
		  AND w.kind = 'personal'
		  AND e.kind = 'capsule_due'
		  AND e.status = 'pending'
		  AND m.status = 'sealed'
		  AND m.unlock_at <= now()
		ORDER BY m.unlock_at, e.id
		LIMIT 100
	`
	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Event, 0)
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.WorkspaceID, &e.MemoryID,
			&e.Kind, &e.Status, &e.UnlockAt, &e.RecordedAt); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

// Acknowledge is idempotent while the event remains eligible. Delivery means
// the user acknowledged the notification, not that the memory was opened.
func (s *Service) Acknowledge(ctx context.Context, userID, eventID string) error {
	if !validUUID(eventID) {
		return ErrInvalidID
	}
	const query = `
		UPDATE return_events AS e
		SET status = 'delivered', delivered_at = COALESCE(e.delivered_at, now())
		FROM memories AS m
		JOIN workspaces AS w ON w.id = m.workspace_id
		JOIN workspace_members AS wm ON wm.workspace_id = w.id
		WHERE e.id = $2::uuid
		  AND e.memory_id = m.id AND e.workspace_id = m.workspace_id
		  AND e.kind = 'capsule_due'
		  AND e.status IN ('pending','delivered')
		  AND m.status = 'sealed' AND m.unlock_at <= now()
		  AND w.kind = 'personal'
		  AND w.owner_user_id = $1::uuid
		  AND wm.user_id = $1::uuid AND wm.role = 'owner'
		RETURNING e.id::text
	`
	var id string
	err := s.db.QueryRow(ctx, query, userID, eventID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
