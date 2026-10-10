package main

import (
	"log"
	"net/http"
	"time"

	"journalapp/internal/capsule"
	"journalapp/internal/journal"
	"journalapp/internal/sealing"
)

func main() {
	repository := journal.NewMemoryRepository()

	service := journal.NewService(repository)

	handler := journal.NewHandler(service)

	capsuleRepsitory := capsule.NewMemoryRepository()

	capsuleService := capsule.NewService(
		capsuleRepsitory,
		time.Now,
	)

	capsuleHandler := capsule.NewHandler(capsuleService)

	sealingService := sealing.NewService(
		service,
		capsuleService,
	)

	sealingHandler := sealing.NewHandler(sealingService)

	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /health",
		func(w http.ResponseWriter, r *http.Request) {

			w.Header().Set(
				"Content-Type",
				"application/json",
			)
			w.WriteHeader(http.StatusOK)

			w.Write([]byte(`{"status":"ok"}`))
		},
	)

	handler.RegisterRoutes(mux)
	sealingHandler.RegisterRoutes(mux)
	capsuleHandler.RegisterRoutes(mux)

	address := ":8080"

	log.Printf(
		"Journal API listening on http://localhost%s",
		address,
	)
	err := http.ListenAndServe(
		address,
		mux,
	)

	if err != nil {
		log.Fatal(err)
	}
}
