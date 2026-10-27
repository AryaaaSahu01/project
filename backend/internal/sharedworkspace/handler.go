package sharedworkspace

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

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Cache-Control", "no-store")
	if value != nil {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(status)
	if value != nil {
		_ = json.NewEncoder(w).Encode(value)
	}
}

func respondErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalid):
		respond(w, 400, map[string]string{"error": "invalid workspace input"})
	case errors.Is(err, ErrNotFound):
		respond(w, 404, map[string]string{"error": "workspace or invitation not found"})
	case errors.Is(err, ErrConflict):
		respond(w, 409, map[string]string{"error": "workspace membership conflict"})
	case errors.Is(err, ErrLimit):
		respond(w, 422, map[string]string{"error": "workspace limit reached"})
	default:
		respond(w, 500, map[string]string{"error": "internal server error"})
	}
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	status, err := httpapi.DecodeJSON(w, r, dst, 4096)
	if err != nil {
		respond(w, status, map[string]string{"error": err.Error()})
		return false
	}
	return true
}

type action func(http.ResponseWriter, *http.Request, account.User)

func (h *Handler) secure(next action) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		header := r.Header.Get("Authorization")
		parts := strings.Fields(header)
		if len(header) > 128 || len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			respond(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		me, err := h.accounts.Authenticate(ctx, parts[1])
		if errors.Is(err, account.ErrUnauthorized) {
			respond(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		if err != nil {
			respondErr(w, err)
			return
		}
		next(w, r, me.User)
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/workspaces", h.secure(h.create))
	mux.HandleFunc("GET /api/v1/workspaces/{id}/members", h.secure(h.members))
	mux.HandleFunc("DELETE /api/v1/workspaces/{id}/members/{userID}", h.secure(h.removeMember))
	mux.HandleFunc("POST /api/v1/workspaces/{id}/invitations", h.secure(h.invite))
	mux.HandleFunc("GET /api/v1/workspaces/{id}/invitations", h.secure(h.invitations))
	mux.HandleFunc("DELETE /api/v1/workspaces/{id}/invitations/{inviteID}", h.secure(h.revokeInvitation))
	mux.HandleFunc("POST /api/v1/invitations/accept", h.secure(h.accept))
	// Keep the original spelling as a compatibility alias.
	mux.HandleFunc("POST /api/v1/invitaions/accept", h.secure(h.accept))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request, user account.User) {
	var input struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &input) {
		return
	}
	value, err := h.svc.Create(r.Context(), user.ID, input.Name)
	if err != nil {
		respondErr(w, err)
		return
	}
	respond(w, 201, value)
}

func (h *Handler) invite(w http.ResponseWriter, r *http.Request, user account.User) {
	var input struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if !decode(w, r, &input) {
		return
	}
	value, err := h.svc.CreateInvitation(r.Context(), user.ID, r.PathValue("id"), input.Email, input.Role)
	if err != nil {
		respondErr(w, err)
		return
	}
	respond(w, 201, value)
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request, user account.User) {
	var input struct {
		Token string `json:"token"`
	}
	if !decode(w, r, &input) {
		return
	}
	value, err := h.svc.Accept(r.Context(), user.ID, user.Email, input.Token)
	if err != nil {
		respondErr(w, err)
		return
	}
	respond(w, 200, value)
}

func (h *Handler) members(w http.ResponseWriter, r *http.Request, user account.User) {
	value, err := h.svc.ListMembers(r.Context(), user.ID, r.PathValue("id"))
	if err != nil {
		respondErr(w, err)
		return
	}
	respond(w, 200, value)
}

func (h *Handler) invitations(w http.ResponseWriter, r *http.Request, user account.User) {
	value, err := h.svc.ListInvitations(r.Context(), user.ID, r.PathValue("id"))
	if err != nil {
		respondErr(w, err)
		return
	}
	respond(w, 200, value)
}

func (h *Handler) removeMember(w http.ResponseWriter, r *http.Request, user account.User) {
	if err := h.svc.RemoveMember(r.Context(), user.ID, r.PathValue("id"), r.PathValue("userID")); err != nil {
		respondErr(w, err)
		return
	}
	respond(w, 204, nil)
}

func (h *Handler) revokeInvitation(w http.ResponseWriter, r *http.Request, user account.User) {
	if err := h.svc.RevokeInvitation(r.Context(), user.ID, r.PathValue("id"), r.PathValue("inviteID")); err != nil {
		respondErr(w, err)
		return
	}
	respond(w, 204, nil)
}
