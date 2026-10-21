package dueevent

import (
	"context"
	"encoding/json"
	"errors"
	"journalapp/internal/account"
	"net/http"
	"strings"
	"time"
)

type Handler struct {
	service  *Service
	accounts *account.Service
}

func NewHandler(service *Service, accounts *account.Service) *Handler {
	return &Handler{service: service, accounts: accounts}
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Cache-Control", "no-store")
	if value == nil {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (h *Handler) authenticated(next func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		r = r.WithContext(ctx)

		auth := r.Header.Get("Authorization")
		if len(auth) > 128 {
			respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		parts := strings.Fields(auth)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		me, err := h.accounts.Authenticate(ctx, parts[1])
		if err != nil {
			if errors.Is(err, account.ErrUnauthorized) {
				respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			} else {
				respond(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			}
			return
		}
		next(w, r, me.User.ID)
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/return-events/eligible", h.authenticated(h.eligible))
	mux.HandleFunc("POST /api/v1/return-events/{id}/ack", h.authenticated(h.ack))
}

func (h *Handler) eligible(w http.ResponseWriter, r *http.Request, userID string) {
	events, err := h.service.ListEligible(r.Context(), userID)

	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	respond(w, http.StatusOK, events)
}

func (h *Handler) ack(w http.ResponseWriter, r *http.Request, userID string) {
	err := h.service.Acknowledge(r.Context(), userID, r.PathValue("id"))
	switch {
	case errors.Is(err, ErrInvalidID):
		respond(w, http.StatusBadRequest, map[string]string{"error": "invalid event ID"})
	case errors.Is(err, account.ErrNotFound):
		respond(w, http.StatusNotFound, map[string]string{"error": "event not found or no longer eligible"})
	case err != nil:
		respond(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	default:
		respond(w, http.StatusNoContent, nil)
	}
}
