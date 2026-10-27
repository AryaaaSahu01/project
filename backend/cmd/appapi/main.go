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

	// R2 is optional in local development. If configured, require all four values.
	fields := []string{"R2_ACCOUNT_ID", "R2_BUCKET", "R2_ACCESS_KEY_ID", "R2_SECRET_ACCESS_KEY"}
	configured := 0
	for _, name := range fields {
		if os.Getenv(name) != "" {
			configured++
		}
	}
	if configured != 0 && configured != len(fields) {
		log.Fatal("all four R2_* variables are required to enable media")
	}
	if configured == len(fields) {
		objects, err := objectstore.NewR2(context.Background(), objectstore.R2Config{
			AccountID: os.Getenv("R2_ACCOUNT_ID"), Bucket: os.Getenv("R2_BUCKET"),
			AccessKeyID: os.Getenv("R2_ACCESS_KEY_ID"), SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
		})
		if err != nil {
			log.Fatal("unable to configure media: ", err)
		}
		media.NewHandler(db, accountService, objects).RegisterRoutes(mux)
		log.Print("Private image media routes enabled")
	}
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
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10,
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
