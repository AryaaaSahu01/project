package main

import (
	"context"
	"errors"
	"journalapp/internal/account"
	"journalapp/internal/conversation"
	"journalapp/internal/dueevent"
	"journalapp/internal/media"
	"journalapp/internal/memory"
	"journalapp/internal/memorylink"
	"journalapp/internal/platform/objectstore"
	"journalapp/internal/platform/postgres"
	"journalapp/internal/sharedworkspace"
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
	connectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := postgres.Open(connectCtx, os.Getenv("DATABASE_URL"), int32(maxConns))
	cancel()
	if err != nil {
		log.Fatal("PostgreSQL unavailable: ", err)
	}
	defer db.Close()
	accountService, err := account.NewService(account.NewPostgresStore(db))
	if err != nil {
		log.Fatal("unable to initialize account service: ", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if err := db.Ping(ctx); err != nil {
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"status":"unavailable"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})
	account.NewHandler(accountService).RegisterRoutes(mux)
	memory.NewHandler(db, accountService).RegisterRoutes(mux)
	conversation.NewHandler(conversation.NewService(db), accountService).RegisterRoutes(mux)
	dueevent.NewHandler(dueevent.NewService(db), accountService).RegisterRoutes(mux)
	memorylink.NewHandler(memorylink.NewService(db), accountService).RegisterRoutes(mux)
	sharedworkspace.NewHandler(sharedworkspace.NewService(db), accountService).RegisterRoutes(mux)

	//media files stay outside postgresql, its metadata and authorization remain there
	// local directory is never mounted as a public file server and shouldn't be tried
	objects, driver, err := objectstore.FromEnv(context.Background())
	if err != nil {
		log.Fatal("unable to configure attachments: ", err)
	}

	if _, err := db.Exec(context.Background(), `SELECT original_name, memory_id, source FROM media_assets LIMIT 0`); err != nil {
		log.Fatal("apply backend/migrations/000002_media.up.sql before starting this API")
	}

	media.NewHandler(db, accountService, objects).RegisterRoutes(mux)
	cleanupCtx, stopCleanup := context.WithCancel(context.Background())
	cleanupDone := make(chan struct{})
	go func() { defer close(cleanupDone); media.RunCleanup(cleanupCtx, db, objects) }()
	defer func() { stopCleanup(); <-cleanupDone }()
	log.Printf("Private attachments enabled (storage: %s)", driver)

	webHandler, err := account.WithBrowserSessions(mux, accountService, os.Getenv("APP_ORIGIN"))
	if err != nil {
		log.Fatal("unable to configure browser sessions: ", err)
	}
	if dist := os.Getenv("FRONTEND_DIST"); dist != "" {
		frontend, err := frontendHandler(dist)
		if err != nil {
			log.Fatal("unable to serve frontend: ", err)
		}
		mux.Handle("/", frontend)
	}
	address := os.Getenv("API_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	server := &http.Server{
		Addr: address, Handler: webHandler,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Minute,
		WriteTimeout: 5 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10,
	}
	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	shutdownDone := make(chan struct{})
	go func() {
		<-stopCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
		close(shutdownDone)
	}()
	log.Printf("Unified Journal API listening at http://%s", server.Addr)
	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	<-shutdownDone
}
