package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"journalapp/internal/capsule"
	"journalapp/internal/journal"
	"journalapp/internal/returnevent"
	"journalapp/internal/sealing"
	"journalapp/internal/thread"
)

func newMux(clock func() time.Time) *http.ServeMux {
	return newMuxWithDevRoutes(clock, false)
}

func newMuxWithDevRoutes(
	clock func() time.Time,
	enableDevRoutes bool,
) *http.ServeMux {
	journalRepo := journal.NewMemoryRepository()
	journalService := journal.NewService(journalRepo)

	capsuleRepo := capsule.NewMemoryRepository()
	capsuleService := capsule.NewService(capsuleRepo, clock)
	returnEventRepo := returnevent.NewMemoryRepository()

	returnEventProcessor := returnevent.NewProcessor(
		capsuleService,
		returnEventRepo,
		clock,
	)

	threadRepo := thread.NewMemoryRepository()

	threadService := thread.NewService(
		threadRepo,
		capsuleService,
		clock,
	)

	sealingService := sealing.NewService(
		journalService,
		capsuleService,
	)

	journalHandler := journal.NewHandler(journalService)
	capsuleHandler := capsule.NewHandler(
		capsuleService,
		threadService,
	)
	sealingHandler := sealing.NewHandler(sealingService)
	threadHandler := thread.NewHandler(threadService)
	returnEventHandler := returnevent.NewHandler(
		returnEventProcessor,
	)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	journalHandler.RegisterRoutes(mux)
	capsuleHandler.RegisterRoutes(mux)
	sealingHandler.RegisterRoutes(mux)
	threadHandler.RegisterRoutes(mux)
	returnEventHandler.RegisterRoutes(mux)

	if enableDevRoutes {
		returnEventHandler.RegisterDevRoutes(mux)
	}

	return mux
}

func main() {
	server := &http.Server{
		Addr: "127.0.0.1:8080",
		Handler: newMuxWithDevRoutes(
			time.Now,
			os.Getenv("JOURNAL_ENV") == "development" &&
				os.Getenv("JOURNAL_ENABLE_DEV_PROCESSING") == "1",
		),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
	)

	defer stop()

	shutdownDone := make(chan struct{})

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown error: %v", err)
		}

		close(shutdownDone)
	}()

	log.Printf(
		"Journal API listening on http://%s",
		server.Addr,
	)

	err := server.ListenAndServe()

	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}

	<-shutdownDone

}
