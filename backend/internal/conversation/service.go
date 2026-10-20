package conversation

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"journalapp/internal/memory"
)

var (
	ErrInvalid             = errors.New("invalid conversation input")
	ErrNotFound            = errors.New("memory or reflection not found")
	ErrConflict            = errors.New("invalid memory or reflection state")
	ErrHistoryTooLarge     = errors.New("conversation history limit exceeded")
	ErrHistoryInconsistent = errors.New("inconsistent conversation history")
	uuidPattern            = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

const (
	MaxReflectionRunes    = 20000
	MaxHistoryMemories    = 256
	MaxHistoryDepth       = 32
	MaxHistoryReflections = 1024
)

type Reflection struct {
	ID                  string    `json:"id"`
	WorkspaceID         string    `json:"workspace_id"`
	MemoryID            string    `json:"memory_id"`
	AuthorUserID        string    `json:"author_user_id"`
	Body                string    `json:"body"`
	Status              string    `json:"status"`
	SealedChildMemoryID *string   `json:"sealed_child_memory_id,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}

type CreateReflectionInput struct {
	Body string `json:"body"`
}

type SealReflectionInput struct {
	UnlockAt time.Time `json:"unlock_at"`
}

type ChildMetadata struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Status      string    `json:"status"`
	SealedAt    time.Time `json:"sealed_at"`
	UnlockAt    time.Time `json:"unlock_at"`
	CanOpen     bool      `json:"can_open"`
}

type HistoryNode struct {
	Memory      memory.Memory       `json:"memory"`
	Reflections []HistoryReflection `json:"reflections"`
}

type HistoryReflection struct {
	Reflection
	Child *HistoryNode `json:"child,omitempty"`
}

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

func validUUID(id string) bool { return uuidPattern.MatchString(id) }

const reflectionColumns = `
    r.id::text, r.workspace_id::text, r.memory_id::text,
    r.author_user_id::text, r.body, r.status,
    r.sealed_child_memory_id::text, r.created_at
`

type scanner interface{ Scan(dest ...any) error }

func scanReflection(row scanner) (Reflection, error) {
	var r Reflection
	err := row.Scan(&r.ID, &r.WorkspaceID, &r.MemoryID, &r.AuthorUserID,
		&r.Body, &r.Status, &r.SealedChildMemoryID, &r.CreatedAt)
	if err != nil {
		return Reflection{}, err
	}
	// Sealed reflection text lives only inside the child memory.
	if r.Status == "sealed" {
		r.Body = ""
	}
	return r, nil
}

// A reflection can be created only on a returned memory and by an editor/owner.
func (s *Service) Create(ctx context.Context, userID, parentID string, in CreateReflectionInput) (Reflection, error) {
	body := strings.TrimSpace(in.Body)
	if !validUUID(parentID) || body == "" || !utf8.ValidString(body) ||
		utf8.RuneCountInString(body) > MaxReflectionRunes || len(body) > 128<<10 {
		return Reflection{}, ErrInvalid
	}
	const query = `
        INSERT INTO reflections (workspace_id, memory_id, author_user_id, body)
        SELECT m.workspace_id, m.id, $1::uuid, $3::text
        FROM memories m
        JOIN workspace_members wm ON wm.workspace_id = m.workspace_id
        WHERE m.id = $2::uuid AND m.status = 'returned'
          AND wm.user_id = $1::uuid AND wm.role IN ('owner','editor')
        RETURNING id::text, workspace_id::text, memory_id::text,
          author_user_id::text, body, status, sealed_child_memory_id::text, created_at
    `
	r, err := scanReflection(s.db.QueryRow(ctx, query, userID, parentID, body))
	if errors.Is(err, pgx.ErrNoRows) {
		return Reflection{}, ErrConflict
	}
	return r, err
}

// list returns ErrNotFound for inaccessible memory IDs, without leaking their existence.
func (s *Service) List(ctx context.Context, userID, parentID string) ([]Reflection, error) {
	if !validUUID(parentID) {
		return nil, ErrInvalid
	}
	var status string
	err := s.db.QueryRow(ctx, `
        SELECT m.status FROM memories m
        JOIN workspace_members wm ON wm.workspace_id=m.workspace_id
        WHERE m.id=$2::uuid AND wm.user_id=$1::uuid
    `, userID, parentID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if status != "returned" {
		return nil, ErrConflict
	}

	rows, err := s.db.Query(ctx, `
        SELECT `+reflectionColumns+`
        FROM reflections r
        JOIN memories m ON m.id=r.memory_id AND m.workspace_id=r.workspace_id
        JOIN workspace_members wm ON wm.workspace_id=r.workspace_id
        WHERE r.memory_id=$1::uuid AND m.status='returned' AND wm.user_id=$2::uuid
        ORDER BY r.created_at, r.id
        LIMIT 1025
    `, parentID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Reflection, 0)
	for rows.Next() {
		r, err := scanReflection(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, r)
		if len(items) > MaxHistoryReflections {
			return nil, ErrHistoryTooLarge
		}
	}
	return items, rows.Err()
}

// Seal is atomic: child creation and reflection status update commit together.
// Row-locking the reflection serializes two concurrent seal attempts.
func (s *Service) Seal(ctx context.Context, userID, parentID, reflectionID string, unlockAt time.Time) (ChildMetadata, error) {
	if !validUUID(parentID) || !validUUID(reflectionID) ||
		!unlockAt.After(time.Now().UTC().Add(time.Minute)) {
		return ChildMetadata{}, ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ChildMetadata{}, err
	}
	defer tx.Rollback(ctx)

	// Lock workspace first to coordinate future conversation-tree deletion.
	// Authorization is checked in the same statement.
	var workspaceID string
	err = tx.QueryRow(ctx, `
        SELECT w.id::text FROM workspaces w
        JOIN workspace_members wm ON wm.workspace_id=w.id
        JOIN memories m ON m.workspace_id=w.id
        WHERE m.id=$2::uuid AND m.status='returned'
          AND wm.user_id=$1::uuid AND wm.role IN ('owner','editor')
        FOR UPDATE OF w
    `, userID, parentID).Scan(&workspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ChildMetadata{}, ErrConflict
	}
	if err != nil {
		return ChildMetadata{}, err
	}

	var body, status string
	err = tx.QueryRow(ctx, `
        SELECT body, status FROM reflections
        WHERE id=$1::uuid AND memory_id=$2::uuid AND workspace_id=$3::uuid
        FOR UPDATE
    `, reflectionID, parentID, workspaceID).Scan(&body, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ChildMetadata{}, ErrNotFound
	}
	if err != nil {
		return ChildMetadata{}, err
	}
	if status != "draft" || body == "" {
		return ChildMetadata{}, ErrConflict
	}

	child := ChildMetadata{WorkspaceID: workspaceID, Status: "sealed", UnlockAt: unlockAt.UTC(), CanOpen: false}
	err = tx.QueryRow(ctx, `
        INSERT INTO memories
            (workspace_id, created_by, title, body, status, sealed_at, unlock_at)
        SELECT $1::uuid, $2::uuid, 'Then & Now', $3::text, 'sealed', now(), $4::timestamptz
        WHERE $4::timestamptz > now()
        RETURNING id::text, sealed_at
    `, workspaceID, userID, body, child.UnlockAt).Scan(&child.ID, &child.SealedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ChildMetadata{}, ErrInvalid
	}
	if err != nil {
		return ChildMetadata{}, err
	}

	tag, err := tx.Exec(ctx, `
        UPDATE reflections
        SET status='sealed', body='', sealed_child_memory_id=$2::uuid
        WHERE id=$1::uuid AND status='draft' AND sealed_child_memory_id IS NULL
    `, reflectionID, child.ID)
	if err != nil {
		return ChildMetadata{}, err
	}
	if tag.RowsAffected() != 1 {
		return ChildMetadata{}, ErrConflict
	}

	if err := tx.Commit(ctx); err != nil {
		return ChildMetadata{}, err
	}
	return child, nil
}

const memoryColumns = `
    m.id::text, m.workspace_id::text, m.created_by::text,
    m.title, m.body, m.status, m.sealed_at, m.unlock_at,
    m.created_at, m.updated_at
`

func scanMemory(row scanner) (memory.Memory, error) {
	var m memory.Memory
	var title, body string
	err := row.Scan(&m.ID, &m.WorkspaceID, &m.CreatedBy,
		&title, &body, &m.Status, &m.SealedAt, &m.UnlockAt,
		&m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return memory.Memory{}, err
	}
	if m.Status != "sealed" {
		m.Title, m.Body = &title, &body
	}
	m.CanOpen = m.Status == "sealed" && m.UnlockAt != nil && !time.Now().UTC().Before(*m.UnlockAt)
	return m, nil
}

// History builds a consistent, read-only snapshot with cycle and size guards.
func (s *Service) History(ctx context.Context, userID, id string) (HistoryNode, error) {
	if !validUUID(id) {
		return HistoryNode{}, ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return HistoryNode{}, err
	}
	defer tx.Rollback(ctx)
	state := &historyState{visited: make(map[string]bool)}
	node, err := s.walk(ctx, tx, state, userID, id, "", 0)
	if err != nil {
		return HistoryNode{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return HistoryNode{}, err
	}
	return node, nil
}

type historyState struct {
	visited     map[string]bool
	count       int
	reflections int
}

func (s *Service) walk(ctx context.Context, tx pgx.Tx, state *historyState, userID, id, expectedWorkspace string, depth int) (HistoryNode, error) {
	if state.visited[id] {
		return HistoryNode{}, ErrHistoryInconsistent
	}
	if state.count >= MaxHistoryMemories || depth >= MaxHistoryDepth {
		return HistoryNode{}, ErrHistoryTooLarge
	}
	state.visited[id] = true
	state.count++
	m, err := scanMemory(tx.QueryRow(ctx, `
        SELECT `+memoryColumns+`
        FROM memories m JOIN workspace_members wm ON wm.workspace_id=m.workspace_id
        WHERE m.id=$2::uuid AND wm.user_id=$1::uuid
    `, userID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return HistoryNode{}, ErrNotFound
	}
	if err != nil {
		return HistoryNode{}, err
	}
	if expectedWorkspace != "" && m.WorkspaceID != expectedWorkspace {
		return HistoryNode{}, ErrHistoryInconsistent
	}
	node := HistoryNode{Memory: m, Reflections: make([]HistoryReflection, 0)}
	if m.Status != "returned" {
		return node, nil
	}

	rows, err := tx.Query(ctx, `
        SELECT `+reflectionColumns+`
        FROM reflections r WHERE r.workspace_id=$1::uuid AND r.memory_id=$2::uuid
        ORDER BY r.created_at, r.id LIMIT 1025
    `, m.WorkspaceID, m.ID)
	if err != nil {
		return HistoryNode{}, err
	}
	// pgx cannot run nested queries while a result set remains open.
	refs := make([]Reflection, 0)
	for rows.Next() {
		ref, scanErr := scanReflection(rows)
		if scanErr != nil {
			rows.Close()
			return HistoryNode{}, scanErr
		}
		refs = append(refs, ref)
		if state.reflections+len(refs) > MaxHistoryReflections {
			rows.Close()
			return HistoryNode{}, ErrHistoryTooLarge
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return HistoryNode{}, err
	}
	state.reflections += len(refs)

	for _, ref := range refs {
		item := HistoryReflection{Reflection: ref}
		switch ref.Status {
		case "draft":
			if ref.SealedChildMemoryID != nil {
				return HistoryNode{}, ErrHistoryInconsistent
			}
		case "sealed":
			if ref.SealedChildMemoryID == nil {
				return HistoryNode{}, ErrHistoryInconsistent
			}
			child, err := s.walk(ctx, tx, state, userID, *ref.SealedChildMemoryID, m.WorkspaceID, depth+1)
			if err != nil {
				return HistoryNode{}, err
			}
			item.Child = &child
		default:
			return HistoryNode{}, ErrHistoryInconsistent
		}
		node.Reflections = append(node.Reflections, item)
	}
	return node, nil
}
