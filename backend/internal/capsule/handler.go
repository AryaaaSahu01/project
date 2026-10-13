package capsule

import (
	"encoding/json"
	"errors"
	"net/http"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/capsules", h.listCapsules)
	mux.HandleFunc("GET /api/v1/capsules/{id}", h.getMetadata)
	mux.HandleFunc("POST /api/v1/capsules/{id}/open", h.openCapsule)
	mux.HandleFunc("DELETE /api/v1/capsules/{id}", h.deleteCapsule)
}

func (h *Handler) listCapsules(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID := "local-user"

	capsules, err := h.service.List(userID)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, capsules)
}

func (h *Handler) getMetadata(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID := "local-user"
	capsuleID := r.PathValue("id")

	metadata, err := h.service.GetMetadata(userID, capsuleID)

	if errors.Is(err, ErrCapsuleNotFound) {
		writeJSON(w, http.StatusNotFound,
			map[string]string{"error": "capsule not found"})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, metadata)
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

func (h *Handler) openCapsule(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID := "local-user"
	capsuleID := r.PathValue("id")

	opened, err := h.service.Open(userID, capsuleID)

	switch {
	case errors.Is(err, ErrCapsuleNotFound):
		writeJSON(w, http.StatusNotFound,
			map[string]string{"error": "capsule not found"})

	case errors.Is(err, ErrCapsuleLocked):
		writeJSON(w, http.StatusLocked,
			map[string]string{"error": "capsule is still sealed"})

	case err != nil:
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{"error": "internal server error"})

	default:
		writeJSON(w, http.StatusOK, opened)

	}
}

func (h *Handler) deleteCapsule(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID := "local-user"
	capsuleID := r.PathValue("id")

	err := h.service.Destroy(userID, capsuleID)

	switch {

	case errors.Is(err, ErrCapsuleNotFound):
		writeJSON(
			w,
			http.StatusNotFound,
			map[string]string{
				"error": "capsule not found",
			},
		)
	case errors.Is(err, ErrCapsuleHasChildren):
		writeJSON(
			w,
			http.StatusConflict,
			map[string]string{
				"error": "capsule has child capsules",
			},
		)
	case err != nil:
		writeJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{
				"error": "internal server error",
			},
		)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
