package returnevent

import (
	"encoding/json"
	"errors"
	"journalapp/internal/capsule"
	"net/http"
)

type Handler struct {
	processor *Processor
}

func NewHandler(processor *Processor) *Handler {
	return &Handler{
		processor: processor,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(
		"POST /api/v1/dev/return-events/process-due",
		h.processDue,
	)

	mux.HandleFunc(
		"GET /api/v1/return-events/eligible",
		h.listEligible,
	)
}

func (h *Handler) processDue(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID := "local-user"

	created, err := h.processor.ProcessDue(userID)

	switch {
	case errors.Is(err, capsule.ErrInvalidUser):
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": err.Error()})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "internal server error"})
	default:
		writeJSON(w, http.StatusOK,
			map[string]int{"created": created})
	}
}

func (h *Handler) listEligible(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID := "local-user"

	events, err := h.processor.ListEligible(userID)

	switch {
	case errors.Is(err, capsule.ErrInvalidUser):
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": err.Error()})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "internal server error"})
	default:
		writeJSON(w, http.StatusOK, events)
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
