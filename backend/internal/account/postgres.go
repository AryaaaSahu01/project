package account

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (p *PostgresStore) Register(ctx context.Context, email, passwordHash, displayName string, tokenHash [32]byte, expires time.Time) (Account, error) {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Account{}, err
	}
	defer tx.Rollback(ctx)

	var a Account
	err = tx.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1,$2)
		RETURNING id::text, email, created_at`, email, passwordHash).Scan(&a.User.ID, &a.User.Email, &a.User.CreatedAt)
	if err != nil {
		var dbError *pgconn.PgError
		if errors.As(err, &dbError) && dbError.Code == "23505" {
			return Account{}, ErrEmailTaken
		}
		return Account{}, err
	}

	err = tx.QueryRow(ctx, `INSERT INTO user_profiles (user_id, display_name)
		VALUES ($1::uuid,$2) RETURNING user_id::text, display_name, bio, timezone, updated_at`, a.User.ID, displayName).
		Scan(&a.Profile.UserID, &a.Profile.DisplayName, &a.Profile.Bio, &a.Profile.Timezone, &a.Profile.UpdatedAt)
	if err != nil {
		return Account{}, err
	}

	w := &Workspace{Kind: "personal", Name: "Personal memories", Role: "owner"}
	err = tx.QueryRow(ctx, `INSERT INTO workspaces (kind, name, owner_user_id)
		VALUES ($1,$2,$3::uuid) RETURNING id::text`, w.Kind, w.Name, a.User.ID).Scan(&w.ID)
	if err != nil {
		return Account{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1::uuid,$2::uuid,'owner')`, w.ID, a.User.ID)
	if err != nil {
		return Account{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1::uuid,$2,$3)`, a.User.ID, tokenHash[:], expires)
	if err != nil {
		return Account{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Account{}, err
	}
	a.Workspace = w
	return a, nil
}

func (p *PostgresStore) PasswordRecord(ctx context.Context, email string) (string, string, error) {
	var id, hash string
	err := p.pool.QueryRow(ctx, `SELECT id::text, password_hash FROM users
		WHERE email=$1 AND disabled_at IS NULL`, email).Scan(&id, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrNotFound
	}
	return id, hash, err
}

func (p *PostgresStore) CreateSession(ctx context.Context, userID string, tokenHash [32]byte, expires time.Time) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1::uuid,$2,$3)`, userID, tokenHash[:], expires)
	return err
}

func (p *PostgresStore) GetAccountByTokenHash(ctx context.Context, tokenHash [32]byte) (Account, error) {
	var a Account
	err := p.pool.QueryRow(ctx, `SELECT u.id::text, u.email, u.created_at,
		p.user_id::text, p.display_name, p.bio, p.timezone, p.updated_at
		FROM sessions s JOIN users u ON u.id=s.user_id
		JOIN user_profiles p ON p.user_id=u.id
		WHERE s.token_hash=$1 AND s.revoked_at IS NULL
		AND s.expires_at>now() AND u.disabled_at IS NULL`, tokenHash[:]).Scan(
		&a.User.ID, &a.User.Email, &a.User.CreatedAt,
		&a.Profile.UserID, &a.Profile.DisplayName, &a.Profile.Bio, &a.Profile.Timezone, &a.Profile.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	return a, err
}

func (p *PostgresStore) UpdateProfile(ctx context.Context, userID string, input UpdateProfileInput) (Profile, error) {
	var profile Profile
	err := p.pool.QueryRow(ctx, `UPDATE user_profiles SET
		display_name=COALESCE($2::text, display_name),
		bio=COALESCE($3::text, bio),
		timezone=COALESCE($4::text, timezone),
		updated_at=now()
		WHERE user_id=$1::uuid
		RETURNING user_id::text, display_name, bio, timezone, updated_at`,
		userID, input.DisplayName, input.Bio, input.Timezone).Scan(
		&profile.UserID, &profile.DisplayName, &profile.Bio, &profile.Timezone, &profile.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	return profile, err
}

func (p *PostgresStore) ListWorkspaces(ctx context.Context, userID string) ([]Workspace, error) {
	rows, err := p.pool.Query(ctx, `SELECT w.id::text, w.name, w.kind, m.role
		FROM workspace_members m JOIN workspaces w ON w.id=m.workspace_id
		WHERE m.user_id=$1::uuid ORDER BY w.created_at, w.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	workspaces := make([]Workspace, 0)
	for rows.Next() {
		var w Workspace
		if err := rows.Scan(&w.ID, &w.Name, &w.Kind, &w.Role); err != nil {
			return nil, err
		}
		workspaces = append(workspaces, w)
	}
	return workspaces, rows.Err()
}

func (p *PostgresStore) RevokeSession(ctx context.Context, tokenHash [32]byte) error {
	result, err := p.pool.Exec(ctx, `UPDATE sessions SET revoked_at=now()
		WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>now()`, tokenHash[:])
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrUnauthorized
	}
	return nil
}
