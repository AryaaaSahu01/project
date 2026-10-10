package sealing

import (
	"encoding/json"
	"errors"
	"journalapp/internal/capsule"
	"journalapp/internal/journal"
	"net/http"
	"time"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(
		"POST /api/v1/entries/{id}/seal",
		h.sealEntry,
	)
}

type SealRequest struct {
	UnlockAt time.Time `json:"unlock_at"`
}

func (h *Handler) sealEntry(
	w http.ResponseWriter,
	r *http.Request,
) {
	var input SealRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": "invalid request body"})
		return
	}

	userID := "local-user"
	entryID := r.PathValue("id")

	metadata, err := h.service.SealEntry(
		userID,
		entryID,
		input.UnlockAt,
	)

	switch {
	case errors.Is(err, journal.ErrEntryNotFound):
		writeJSON(w, http.StatusNotFound,
			map[string]string{"error": "journal entry not found"})

	case errors.Is(err, capsule.ErrInvalidUnlockAt):
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": err.Error()})

	case err != nil:
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "internal server error"})

	default:
		writeJSON(w, http.StatusCreated, metadata)

	}

}

func writeJSON(
	w http.ResponseWriter,
	status int,
	data any,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}
