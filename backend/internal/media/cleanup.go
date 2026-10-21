package media

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"journalapp/internal/platform/objectstore"
)

func CleanupExpired(ctx context.Context, db *pgxpool.Pool, objects *objectstore.R2) (int, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `
		SELECT id::text, workspace_id::text, object_key
		FROM media_assets
		WHERE status IN ('pending','rejected')
		  AND created_at < now()-interval '15 minutes'
		ORDER BY created_at, id LIMIT 50 FOR UPDATE SKIP LOCKED`)
	if err != nil {
		return 0, err
	}
	type stale struct{ id, workspaceID, key string }
	batch := make([]stale, 0)
	for rows.Next() {
		var item stale
		if err := rows.Scan(&item.id, &item.workspaceID, &item.key); err != nil {
			rows.Close()
			return 0, err
		}
		batch = append(batch, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, item := range batch {
		// A rejected upload might also have left a copied destination.
		for _, key := range []string{
			item.key, "assets/" + item.workspaceID + "/" + item.id,
		} {
			deleteCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := objects.Delete(deleteCtx, key)
			cancel()
			if err != nil {
				return 0, err // rollback; retry on the next worker pass.
			}
		}
		_, err := tx.Exec(ctx, `UPDATE media_assets SET status='deleted'
			WHERE id=$1::uuid AND status IN ('pending','rejected')`, item.id)
		if err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(batch), nil
}
