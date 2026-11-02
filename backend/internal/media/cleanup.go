package media

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"journalapp/internal/platform/objectstore"
)

func CleanupExpired(ctx context.Context, db *pgxpool.Pool, objects *objectstore.Store) (int, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `WITH expired AS (
		UPDATE media_assets SET status='rejected'
		WHERE status='pending' AND created_at<now()-interval '15 minutes' RETURNING object_key)
		INSERT INTO media_deletions(object_key) SELECT object_key FROM expired ON CONFLICT DO NOTHING`)
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(ctx, `INSERT INTO media_deletions(object_key)
		SELECT object_key FROM media_assets WHERE status='rejected'
		UNION SELECT 'assets/' ||workspace_id::text|| '/' ||id::text FROM media_assets WHERE status='rejected'
		ON CONFLICT DO NOTHING`)
	if err != nil {
		return 0, err
	}

	rows, err := tx.Query(ctx, `
		SELECT object_key
		FROM media_deletions
		ORDER BY created_at, object_key	
		LIMIT 50 FOR UPDATE SKIP LOCKED`)
	if err != nil {
		return 0, err
	}

	keys := make([]string, 0)
	for row.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			rows.Close()
			return 0, err
		}
		keys = append(keys, key)
	}

	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, key := range keys {
		deleteCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := objects.Delete(deleteCtx, key)
		cancel()

		if err != nil {
			log.Printf("attachment cleanup will retry: %v", err)
			continue
		}

		if _, err := tx.Exec(ctx, `DELETE FROM media_deletions WHERE object_key=$1`, key); err != nil {
			return 0, err
		}

		if _, err := tx.Exec(ctx, `UPDATE media_assets SET status='deleted' WHERE object_key=$1 AND status='rejected'`, key); err != nil {
			return 0, err
		}

		count++
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return count, nil
}

func RunCleanup(ctx, context.Context, db *pgxpool.Pool, objects objectstore.Store) {
	timer := time.NewTicker(time.Minute)
	defer timer.Stop()

	for {
		pass, cancel := context.WithTimeout(ctx, 30*time.Second)
		_, err := CleanupExpired(pass, db, objects)
		cancel()

		if err != nil && ctx.Err() == nil {
			log.Printf("attachment cleanup failed: %v" ,err)
		}

		select {
		case <- ctx.Done()
			return 
		case <-timer.C:
		}
	}
}