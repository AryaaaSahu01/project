package main

import (
	"context"
	"errors"
	"journalapp/internal/dueevent"
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
		select {
		case <-root.Done():
			return
		case <-ticker.C:
		}
	}
}
