package main

import (
	"context"
	"errors"
	"journalapp/internal/account"
	"journalapp/internal/memory"
	"journalapp/internal/platform/postgres"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"
)

func main() {
	maxConns := int64(10)

	if raw := os.Getenv("DB_MAX_CONNS"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			log.Fatal("DB_MAX_CONNS must be an integer")
		}
		maxConns = value
	}

	connectCtx, cancel := context.WithTimeout(
		context.Background(), 10*time.Second,
	)

	db, err := postgres.Open(
		connectCtx,
		os.Getenv("DATABASE_URL"),
		int32(maxConns),
	)
	cancel()

	if err != nil {
		log.Fatal("PostgreSQL unavailable: ", err)
	}

	defer db.Close()

	accountService, err := account.NewService(
		account.NewPostgresStore(db),
	)

	if err != nil {
		log.Fatal("unable to initialize account service: ", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(
		w http.ResponseWriter, r *http.Request,
	) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	account.NewHandler(accountService).RegisterRoutes(mux)
	memory.NewHandler(db, accountService).RegisterRoutes(mux)

	server := &http.Server{
		Addr:              "127.0.0.1:8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}

	stopCtx, stop := signal.NotifyContext(
		context.Background(), os.Interrupt,
	)

	defer stop()

	shutdownDone := make(chan struct{})

	go func() {
		<-stopCtx.Done()

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(), 10*time.Second,
		)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}

		close(shutdownDone)
	}()

	log.Printf(
		"Unified Journal API listening at http://%s",
		server.Addr,
	)

	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}

	<-shutdownDone
}
