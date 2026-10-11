package main

import (
	"log"
	"net/http"
	"time"

	"journalapp/internal/capsule"
	"journalapp/internal/journal"
	"journalapp/internal/sealing"
)

func newMux(clock func() time.Time) *http.ServeMux {
	journalRepo := journal.NewMemoryRepository()
	journalService := journal.NewService(journalRepo)

	capsuleRepo := capsule.NewMemoryRepository()
	capsuleService := capsule.NewService(capsuleRepo, clock)

	sealingService := sealing.NewService(
		journalService,
		capsuleService,
	)

	journalHandler := journal.NewHandler(journalService)
	capsuleHandler := capsule.NewHandler(capsuleService)
	sealingHandler := sealing.NewHandler(sealingService)

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

	return mux
}

func main() {
	address := ":8080"

	log.Printf(
		"Journal API listening on http://localhost%s",
		address,
	)

	err := http.ListenAndServe(address, newMux(time.Now))

	if err != nil {
		log.Fatal(err)
	}
}
