package sharedworkspace

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid  = errors.New("invalid workspace input")
	ErrNotFound = errors.New("workspace or invitation not found")
	ErrConflict = errors.New("workspace membership conflict")
	ErrLimit    = errors.New("workspace member or invitation limit")
	uuid        = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

const (
	maxMembers = 20
	maxInvites = 20
)

type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	Role string `json:"role"`
}

type Invitation struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspace_id"`
	InviteeEmail string    `json:"invitee_email"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
	Token        string    `json:"token,omitempty"` // returned only by CreateInvitation
}

type Member struct {
	UserID   string    `json:"user_id"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

func cleanEmail(input string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(input))
	if len(email) < 3 || len(email) > 254 {
		return "", ErrInvalid
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || strings.ContainsAny(email, " \r\n\t") {
		return "", ErrInvalid
	}
	return email, nil
}

func (s *Service) Create(ctx context.Context, ownerID, name string) (Workspace, error) {
	name = strings.TrimSpace(name)
	if !utf8.ValidString(name) || name == "" || utf8.RuneCountInString(name) > 120 {
		return Workspace{}, ErrInvalid
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Workspace{}, err
	}
	defer tx.Rollback(ctx)
	w := Workspace{Name: name, Kind: "shared", Role: "owner"}
	err = tx.QueryRow(ctx, `INSERT INTO workspaces (kind,name,owner_user_id)
		VALUES ('shared',$1,$2::uuid) RETURNING id::text`, name, ownerID).Scan(&w.ID)
	if err != nil {
		return Workspace{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO workspace_members (workspace_id,user_id,role)
		VALUES ($1::uuid,$2::uuid,'owner')`, w.ID, ownerID)
	if err != nil {
		return Workspace{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Workspace{}, err
	}
	return w, nil
}

func lockOwner(ctx context.Context, tx pgx.Tx, ownerID, workspaceID string) error {
	if !uuid.MatchString(workspaceID) {
		return ErrInvalid
	}
	var id string
	err := tx.QueryRow(ctx, `SELECT w.id::text FROM workspaces w
		JOIN workspace_members wm ON wm.workspace_id=w.id
		WHERE w.id=$1::uuid AND w.kind='shared' AND w.owner_user_id=$2::uuid
		AND wm.user_id=$2::uuid AND wm.role='owner'
		FOR UPDATE OF w`, workspaceID, ownerID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Service) CreateInvitation(ctx context.Context, ownerID, workspaceID, email, role string) (Invitation, error) {
	if !uuid.MatchString(workspaceID) || (role != "viewer" && role != "editor") {
		return Invitation{}, ErrInvalid
	}
	email, err := cleanEmail(email)
	if err != nil {
		return Invitation{}, err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Invitation{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockOwner(ctx, tx, ownerID, workspaceID); err != nil {
		return Invitation{}, err
	}
	var memberCount, inviteCount int
	err = tx.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM workspace_members WHERE workspace_id=$1::uuid),
		(SELECT count(*) FROM workspace_invitations
		 WHERE workspace_id=$1::uuid AND status='pending' AND expires_at>now())`,
		workspaceID).Scan(&memberCount, &inviteCount)
	if err != nil {
		return Invitation{}, err
	}
	if memberCount >= maxMembers || inviteCount >= maxInvites {
		return Invitation{}, ErrLimit
	}
	var exists bool
	err = tx.QueryRow(ctx, `SELECT
		EXISTS(SELECT 1 FROM users u JOIN workspace_members wm ON wm.user_id=u.id
		       WHERE wm.workspace_id=$1::uuid AND u.email=$2)
		OR EXISTS(SELECT 1 FROM workspace_invitations
		       WHERE workspace_id=$1::uuid AND invitee_email=$2
		       AND status='pending' AND expires_at>now())`,
		workspaceID, email).Scan(&exists)
	if err != nil {
		return Invitation{}, err
	}
	if exists {
		return Invitation{}, ErrConflict
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return Invitation{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	hash := sha256.Sum256(raw[:])
	invite := Invitation{WorkspaceID: workspaceID, InviteeEmail: email, Role: role,
		Status: "pending", Token: token}
	err = tx.QueryRow(ctx, `INSERT INTO workspace_invitations
		(workspace_id,invited_by,invitee_email,role,token_hash,expires_at)
		VALUES ($1::uuid,$2::uuid,$3,$4,$5,now()+interval '7 days')
		RETURNING id::text,expires_at,created_at`,
		workspaceID, ownerID, email, role, hash[:]).
		Scan(&invite.ID, &invite.ExpiresAt, &invite.CreatedAt)
	if err != nil {
		return Invitation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Invitation{}, err
	}
	return invite, nil
}

func (s *Service) Accept(ctx context.Context, userID, email, token string) (Workspace, error) {
	if len(token) != 43 {
		return Workspace{}, ErrNotFound
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 {
		return Workspace{}, ErrNotFound
	}
	hash := sha256.Sum256(raw)
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Workspace{}, err
	}
	defer tx.Rollback(ctx)
	var workspaceID string
	err = tx.QueryRow(ctx, `SELECT workspace_id::text FROM workspace_invitations
		WHERE token_hash=$1`, hash[:]).Scan(&workspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Workspace{}, ErrNotFound
	}
	if err != nil {
		return Workspace{}, err
	}
	var w Workspace
	err = tx.QueryRow(ctx, `SELECT id::text,name,kind FROM workspaces
		WHERE id=$1::uuid AND kind='shared' FOR UPDATE`, workspaceID).
		Scan(&w.ID, &w.Name, &w.Kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return Workspace{}, ErrNotFound
	}
	if err != nil {
		return Workspace{}, err
	}
	var inviteEmail, status string
	var expires time.Time
	err = tx.QueryRow(ctx, `SELECT invitee_email,role,status,expires_at
		FROM workspace_invitations WHERE token_hash=$1 AND workspace_id=$2::uuid
		FOR UPDATE`, hash[:], workspaceID).Scan(&inviteEmail, &w.Role, &status, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return Workspace{}, ErrNotFound
	}
	if err != nil {
		return Workspace{}, err
	}
	if inviteEmail != email || status != "pending" || !expires.After(time.Now().UTC()) {
		return Workspace{}, ErrNotFound
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM workspace_members
		WHERE workspace_id=$1::uuid`, workspaceID).Scan(&count); err != nil {
		return Workspace{}, err
	}
	if count >= maxMembers {
		return Workspace{}, ErrLimit
	}
	tag, err := tx.Exec(ctx, `INSERT INTO workspace_members (workspace_id,user_id,role)
		VALUES ($1::uuid,$2::uuid,$3)
		ON CONFLICT DO NOTHING`, workspaceID, userID, w.Role)
	if err != nil {
		return Workspace{}, err
	}
	if tag.RowsAffected() != 1 {
		return Workspace{}, ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE workspace_invitations
		SET status='accepted',accepted_at=now()
		WHERE token_hash=$1 AND status='pending'`, hash[:])
	if err != nil {
		return Workspace{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Workspace{}, err
	}
	return w, nil
}

func (s *Service) ListMembers(ctx context.Context, ownerID, workspaceID string) ([]Member, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := lockOwner(ctx, tx, ownerID, workspaceID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT wm.user_id::text,u.email,wm.role,wm.joined_at
		FROM workspace_members wm JOIN users u ON u.id=wm.user_id
		WHERE wm.workspace_id=$1::uuid ORDER BY wm.joined_at,wm.user_id
		LIMIT 101`, workspaceID)
	if err != nil {
		return nil, err
	}
	result := make([]Member, 0)
	for rows.Next() {
		var item Member
		if err := rows.Scan(&item.UserID, &item.Email, &item.Role, &item.JoinedAt); err != nil {
			rows.Close()
			return nil, err
		}
		result = append(result, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(result) > 100 {
		return nil, ErrLimit
	}
	return result, tx.Commit(ctx)
}

func (s *Service) ListInvitations(ctx context.Context, ownerID, workspaceID string) ([]Invitation, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := lockOwner(ctx, tx, ownerID, workspaceID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id::text,workspace_id::text,invitee_email,
		role,status,expires_at,created_at FROM workspace_invitations
		WHERE workspace_id=$1::uuid AND status='pending' AND expires_at>now()
		ORDER BY created_at,id LIMIT 21`, workspaceID)
	if err != nil {
		return nil, err
	}
	result := make([]Invitation, 0)
	for rows.Next() {
		var item Invitation
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.InviteeEmail,
			&item.Role, &item.Status, &item.ExpiresAt, &item.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		result = append(result, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(result) > maxInvites {
		return nil, ErrLimit
	}
	return result, tx.Commit(ctx)
}

func (s *Service) RevokeInvitation(ctx context.Context, ownerID, workspaceID, invitationID string) error {
	if !uuid.MatchString(invitationID) {
		return ErrInvalid
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockOwner(ctx, tx, ownerID, workspaceID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE workspace_invitations SET status='revoked'
		WHERE id=$1::uuid AND workspace_id=$2::uuid AND status='pending'`,
		invitationID, workspaceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

func (s *Service) RemoveMember(ctx context.Context, ownerID, workspaceID, memberID string) error {
	if !uuid.MatchString(memberID) || strings.EqualFold(memberID, ownerID) {
		return ErrInvalid
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockOwner(ctx, tx, ownerID, workspaceID); err != nil {
		return err
	}
	var email string
	err = tx.QueryRow(ctx, `SELECT u.email FROM workspace_members wm
		JOIN users u ON u.id=wm.user_id
		WHERE wm.workspace_id=$1::uuid AND wm.user_id=$2::uuid
		AND wm.role<>'owner'`, workspaceID, memberID).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE workspace_invitations SET status='revoked'
		WHERE workspace_id=$1::uuid AND invitee_email=$2 AND status='pending'`,
		workspaceID, email)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM workspace_members
		WHERE workspace_id=$1::uuid AND user_id=$2::uuid AND role<>'owner'`,
		workspaceID, memberID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return tx.Commit(ctx)
}
