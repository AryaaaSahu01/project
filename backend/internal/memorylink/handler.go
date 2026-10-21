package memorylink

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"journalapp/internal/account"
	"journalapp/internal/httpapi"
)

type Handler struct {
	svc      *Service
	accounts *account.Service
}

func NewHandler(svc *Service, accounts *account.Service) *Handler {
	return &Handler{svc: svc, accounts: accounts}
}

func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Cache-Control", "no-store")
	if value != nil {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(status)
	if value != nil {
		_ = json.NewEncoder(w).Encode(value)
	}
}

func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalid):
		write(w, 400, map[string]string{"error": "invalid link input"})
	case errors.Is(err, ErrNotFound):
		write(w, 404, map[string]string{"error": "memory or link not found"})
	case errors.Is(err, ErrConflict):
		write(w, 409, map[string]string{"error": "link already exists"})
	case errors.Is(err, ErrTooMany):
		write(w, 422, map[string]string{"error": "memory link limit reached"})
	default:
		write(w, 500, map[string]string{"error": "internal server error"})
	}
}

func (h *Handler) secure(next func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		authorization := r.Header.Get("Authorization")
		parts := strings.Fields(authorization)
		if len(authorization) > 128 || len(parts) != 2 ||
			!strings.EqualFold(parts[0], "Bearer") {
			write(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		me, err := h.accounts.Authenticate(ctx, parts[1])
		if errors.Is(err, account.ErrUnauthorized) {
			write(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		if err != nil {
			writeErr(w, err)
			return
		}
		next(w, r, me.User.ID)
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/memories/{id}/links", h.secure(h.create))
	mux.HandleFunc("GET /api/v1/memories/{id}/links", h.secure(h.list))
	mux.HandleFunc("GET /api/v1/memories/{id}/links/traverse", h.secure(h.traverse))
	mux.HandleFunc("DELETE /api/v1/memories/{id}/links/{linkID}", h.secure(h.remove))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request, userID string) {
	var input struct {
		ToMemoryID string `json:"to_memory_id"`
		Relation   string `json:"relation"`
	}
	code, err := httpapi.DecodeJSON(w, r, &input, 4096)
	if err != nil {
		write(w, code, map[string]string{"error": err.Error()})
		return
	}
	link, err := h.svc.Create(r.Context(), userID, r.PathValue("id"), input.ToMemoryID, input.Relation)
	if err != nil {
		writeErr(w, err)
		return
	}
	write(w, 201, link)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request, userID string) {
	links, err := h.svc.List(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	write(w, 200, links)
}

func (h *Handler) traverse(w http.ResponseWriter, r *http.Request, userID string) {
	depth := 2
	if raw := r.URL.Query().Get("depth"); raw != "" {
		var err error
		depth, err = strconv.Atoi(raw)
		if err != nil {
			writeErr(w, ErrInvalid)
			return
		}
	}
	items, err := h.svc.Traverse(r.Context(), userID, r.PathValue("id"), depth)
	if err != nil {
		writeErr(w, err)
		return
	}
	write(w, 200, items)
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request, userID string) {
	if err := h.svc.Delete(r.Context(), userID, r.PathValue("id"), r.PathValue("linkID")); err != nil {
		writeErr(w, err)
		return
	}
	write(w, 204, nil)
}
