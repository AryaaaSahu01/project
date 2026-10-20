package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"journalapp/internal/account"
	"journalapp/internal/httpapi"
	"net/http"
	"strings"
	"time"
)

type Handler struct {
	svc      *Service
	accounts *account.Service
}

func NewHandler(svc *Service, accounts *account.Service) *Handler {
	return &Handler{svc: svc, accounts: accounts}
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Cache-Control", "no-store")
	if payload == nil {
		w.WriteHeader(code)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalid):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid conversation input"})
	case errors.Is(err, ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "memory or reflection not found"})
	case errors.Is(err, ErrConflict):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "memory must be returned or reflection already sealed"})
	case errors.Is(err, ErrHistoryTooLarge):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "history retrieval limit exceeded"})
	case errors.Is(err, ErrHistoryInconsistent):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "conversation history inconsistent"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}

func decode(w http.ResponseWriter, r *http.Request, v any, limit int64) bool {
	status, err := httpapi.DecodeJSON(w, r, v, limit)
	if err != nil {
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return false
	}
	return true
}

type securedAction func(http.ResponseWriter, *http.Request, string)

func (h *Handler) secure(next securedAction) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		r = r.WithContext(ctx)

		authorization := r.Header.Get("Authorization")
		if len(authorization) > 128 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		parts := strings.Fields(authorization)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		me, err := h.accounts.Authenticate(ctx, parts[1])
		if err != nil {
			if errors.Is(err, account.ErrUnauthorized) {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			} else {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			}
			return
		}
		next(w, r, me.User.ID)
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/memories/{id}/reflections", h.secure(h.create))
	mux.HandleFunc("GET /api/v1/memories/{id}/reflections", h.secure(h.list))
	mux.HandleFunc("POST /api/v1/memories/{id}/reflections/{reflectionID}/seal", h.secure(h.seal))
	mux.HandleFunc("GET /api/v1/memories/{id}/history", h.secure(h.history))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request, userID string) {
	var in CreateReflectionInput
	if !decode(w, r, &in, httpapi.MaxReflectionBytes) {
		return
	}
	value, err := h.svc.Create(r.Context(), userID, r.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, value)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request, userID string) {
	value, err := h.svc.List(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (h *Handler) seal(w http.ResponseWriter, r *http.Request, userID string) {
	var in SealReflectionInput
	if !decode(w, r, &in, httpapi.MaxSealBytes) {
		return
	}
	value, err := h.svc.Seal(r.Context(), userID, r.PathValue("id"), r.PathValue("reflectionID"), in.UnlockAt)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request, userID string) {
	value, err := h.svc.History(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
