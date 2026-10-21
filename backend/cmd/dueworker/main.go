package main

import (
	"context"
	"errors"
	"journalapp/internal/dueevent"
	"journalapp/internal/media"
	"journalapp/internal/platform/objectstore"
	"journalapp/internal/platform/postgres"
	"log"
	"os"
	"os/signal"
	"strconv"
	"time"
)

func main() {
	maxConns := int32(2)
	if raw := os.Getenv("DB_MAX_CONNS"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || n < 1 || n > 100 {
			log.Fatal("DB_MAX_CONNS must be 1..100")
		}
		maxConns = int32(n)
	}

	root, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	connectCtx, cancel := context.WithTimeout(root, 10*time.Second)
	db, err := postgres.Open(connectCtx, os.Getenv("DATABASE_URL"), maxConns)
	cancel()
	if err != nil {
		log.Fatal("PostgreSQL unavailable: ", err)
	}
	defer db.Close()

	service := dueevent.NewService(db)
	var objects *objectstore.R2
	r2Names := []string{"R2_ACCOUNT_ID", "R2_BUCKET", "R2_ACCESS_KEY_ID", "R2_SECRET_ACCESS_KEY"}
	r2Count := 0
	for _, name := range r2Names {
		if os.Getenv(name) != "" {
			r2Count++
		}
	}
	if r2Count != 0 && r2Count != len(r2Names) {
		log.Fatal("all four R2_* variables are required for media cleanup")
	}
	if r2Count == len(r2Names) {
		objects, err = objectstore.NewR2(root, objectstore.R2Config{
			AccountID:       os.Getenv("R2_ACCOUNT_ID"),
			Bucket:          os.Getenv("R2_BUCKET"),
			AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
		})
		if err != nil {
			log.Fatal("unable to configure media cleanup: ", err)
		}
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	log.Print("Durable due-event worker started (every 60 seconds)")

	for {
		ctx, cancel := context.WithTimeout(root, 25*time.Second)
		created, cancelled, err := service.ProcessDue(ctx)
		cancel()
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("event processing failed: %v", err)
		} else if err == nil && (created > 0 || cancelled > 0) {
			log.Printf("events: created=%d cancelled=%d", created, cancelled)
		}
		if objects != nil {
			cleanupCtx, cleanupCancel := context.WithTimeout(root, 35*time.Second)
			removed, cleanupErr := media.CleanupExpired(cleanupCtx, db, objects)
			cleanupCancel()
			if cleanupErr != nil && !errors.Is(cleanupErr, context.Canceled) {
				log.Printf("media cleanup failed: %v", cleanupErr)
			} else if removed > 0 {
				log.Printf("media: expired intents cleaned=%d", removed)
			}
		}
		select {
		case <-root.Done():
			return
		case <-ticker.C:
		}
	}
}
