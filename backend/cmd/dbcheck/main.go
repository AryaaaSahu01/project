package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"journalapp/internal/platform/postgres"
)

func main() {
	maxConns := int64(10)
	if raw := os.Getenv("DB_MAX_CONNS"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			log.Fatal("DB_MAX_CONNS must be an integer")
		}
		maxConns = n
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := postgres.Open(ctx, os.Getenv("DATABASE_URL"), int32(maxConns))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	var userCount, workspaceCount, memoryCount int64
	// This also verifies the first migration was applied.
	err = db.QueryRow(ctx, `SELECT
        (SELECT count(*) FROM users),
        (SELECT count(*) FROM workspaces),
        (SELECT count(*) FROM memories)`).Scan(&userCount, &workspaceCount, &memoryCount)
	if err != nil {
		log.Fatal("schema check failed: apply migrations first")
	}
	log.Printf("PostgreSQL OK | users=%d workspaces=%d memories=%d",
		userCount, workspaceCount, memoryCount)
}
