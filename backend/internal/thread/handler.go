package thread

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"journalapp/internal/capsule"
	"journalapp/internal/httpapi"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(
		"POST /api/v1/capsules/{id}/reflections",
		h.createReflection,
	)

	mux.HandleFunc(
		"GET /api/v1/capsules/{id}/reflections",
		h.listReflections,
	)

	mux.HandleFunc(
		"POST /api/v1/capsules/{id}/reflections/{reflectionID}/seal",
		h.sealReflection,
	)
}

func (h *Handler) createReflection(
	w http.ResponseWriter,
	r *http.Request,
) {
	var input CreateReflectionInput

	status, err := httpapi.DecodeJSON(
		w,
		r,
		&input,
		httpapi.MaxReflectionBytes,
	)

	if err != nil {
		writeJSON(w, status, map[string]string{
			"error": err.Error(),
		})

		return
	}

	userID := "local-user"
	capsuleID := r.PathValue("id")

	reflection, err := h.service.Create(
		userID,
		capsuleID,
		input,
	)

	switch {
	case errors.Is(err, capsule.ErrCapsuleNotFound):
		writeJSON(w, http.StatusNotFound,
			map[string]string{
				"error": "capsule not found",
			})
	case errors.Is(err, ErrCapsuleNotReturned):
		writeJSON(w, http.StatusConflict,
			map[string]string{
				"error": "capsule must be opened before reflecting",
			})

	case errors.Is(err, ErrEmptyReflection),
		errors.Is(err, ErrReflectionTooLong):
		writeJSON(w, http.StatusBadRequest,
			map[string]string{
				"error": err.Error(),
			})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{
				"error": "internal server error",
			})

	default:
		writeJSON(w, http.StatusCreated, reflection)
	}

}

func (h *Handler) listReflections(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID := "local-user"
	capsuleID := r.PathValue("id")

	reflections, err := h.service.List(
		userID,
		capsuleID,
	)

	switch {
	case errors.Is(err, capsule.ErrCapsuleNotFound):
		writeJSON(w, http.StatusNotFound,
			map[string]string{
				"error": "capsule not found",
			})
	case errors.Is(err, ErrCapsuleNotReturned):
		writeJSON(w, http.StatusConflict,
			map[string]string{
				"error": "capsule must be opened before reflecting",
			})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{
				"error": "internal server error",
			})
	default:
		writeJSON(w, http.StatusOK, reflections)
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

type SealReflectionRequest struct {
	UnlockAt time.Time `json:"unlock_at"`
}

func (h *Handler) sealReflection(
	w http.ResponseWriter,
	r *http.Request,
) {
	var input SealReflectionRequest

	status, err := httpapi.DecodeJSON(
		w,
		r,
		&input,
		httpapi.MaxSealBytes,
	)

	if err != nil {
		writeJSON(w, status, map[string]string{
			"error": err.Error(),
		})
		return
	}

	userID := "local-user"
	parentID := r.PathValue("id")
	reflectionID := r.PathValue("reflectionID")

	child, err := h.service.SealReflection(
		userID,
		parentID,
		reflectionID,
		input.UnlockAt,
	)

	switch {
	case errors.Is(err, capsule.ErrCapsuleNotFound),
		errors.Is(err, ErrReflectionNotFound):
		writeJSON(w, http.StatusNotFound,
			map[string]string{
				"error": "capsule or reflection not found",
			})
	case errors.Is(err, ErrCapsuleNotReturned),
		errors.Is(err, ErrReflectionAlreadySealed),
		errors.Is(err, capsule.ErrCapsuleParentNotReturned):
		writeJSON(w, http.StatusConflict,
			map[string]string{
				"error": err.Error(),
			})
	case errors.Is(err, capsule.ErrInvalidUnlockAt):
		writeJSON(w, http.StatusBadRequest,
			map[string]string{
				"error": err.Error(),
			})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError,
			map[string]string{
				"error": "internal server error",
			})
	default:
		writeJSON(w, http.StatusCreated, child)
	}
}
