package journal

import (
	"encoding/json"
	"errors"
	"journalapp/internal/httpapi"
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
	mux.HandleFunc("GET /api/v1/entries/{id}", h.getEntry)
	mux.HandleFunc("PATCH /api/v1/entries/{id}", h.UpdateEntry)
	mux.HandleFunc("DELETE /api/v1/entries/{id}", h.deleteEntry)
}

func (h *Handler) createEntry(
	w http.ResponseWriter,
	r *http.Request,
) {
	var input CreateEntryInput

	status, err := httpapi.DecodeJSON(
		w,
		r,
		&input,
		httpapi.MaxEntryBytes,
	)

	if err != nil {
		writeJSON(
			w,
			status,
			map[string]string{
				"error": err.Error(),
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

func (h *Handler) getEntry(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID := "local-user"

	entryID := r.PathValue("id")

	entry, err := h.service.Get(userID, entryID)

	if errors.Is(err, ErrEntryNotFound) {
		writeJSON(
			w,
			http.StatusNotFound,
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
		http.StatusOK,
		entry,
	)
}

func (h *Handler) UpdateEntry(
	w http.ResponseWriter,
	r *http.Request,
) {
	var input UpdateEntryInput

	status, err := httpapi.DecodeJSON(
		w,
		r,
		&input,
		httpapi.MaxEntryBytes,
	)
	if err != nil {
		writeJSON(w, status,
			map[string]string{"error": err.Error()})
		return
	}

	entryID := r.PathValue("id")
	userID := "local-user"

	entry, err := h.service.Update(userID, entryID, input)

	switch {
	case errors.Is(err, ErrEntryNotFound):
		writeJSON(w, http.StatusNotFound,
			map[string]string{"error": err.Error()})

	case errors.Is(err, ErrEmptyEntry),
		errors.Is(err, ErrNoFieldsToUpdate):
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": err.Error()})

	case err != nil:
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "internal server error"})
	default:
		writeJSON(w, http.StatusOK, entry)

	}
}

func (h *Handler) deleteEntry(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID := "local-user"
	entryID := r.PathValue("id")

	err := h.service.Delete(userID, entryID)

	if errors.Is(err, ErrEntryNotFound) {
		writeJSON(
			w,
			http.StatusNotFound,
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

	w.WriteHeader(http.StatusNoContent)
}
