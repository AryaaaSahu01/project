package journal

import (
	"encoding/json"
	"errors"
	"net/http"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/entries", h.createEntry)
	mux.HandleFunc("GET /api/v1/entries", h.listEntries)
}

func (h *Handler) createEntry(
	w http.ResponseWriter,
	r *http.Request,
) {
	var input CreateEntryInput

	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid request body",
			},
		)
		return
	}
	userID := "local-user"

	entry, err := h.service.Create(userID, input)

	if errors.Is(err, ErrEmptyEntry) {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": err.Error(),
			},
		)
		return
	}
	if err != nil {
		writeJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{
				"error": "internal server error",
			},
		)
		return
	}
	writeJSON(
		w,
		http.StatusCreated,
		entry,
	)
}

func (h *Handler) listEntries(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID := "local-user"

	entries, err := h.service.List(userID)

	if err != nil {
		writeJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{
				"error": "internal server error",
			},
		)
		return
	}
	writeJSON(
		w,
		http.StatusOK,
		entries,
	)
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	data any,
) {

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(status)

	json.NewEncoder(w).Encode(data)
}
