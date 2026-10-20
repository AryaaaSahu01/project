package account

import (
	"context"
	"encoding/json"
	"errors"
	"journalapp/internal/httpapi"
	"net/http"
	"strings"
	"time"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/register", h.register)
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.HandleFunc("POST /api/v1/auth/logout", h.logout)
	mux.HandleFunc("GET /api/v1/me", h.me)
	mux.HandleFunc("PATCH /api/v1/me", h.updateProfile)
	mux.HandleFunc("GET /api/v1/me/workspaces", h.workspaces)
}

func writeResponse(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Cache-Control", "no-store")
	if body == nil {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json ; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func publicError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid account details"})
	case errors.Is(err, ErrEmailTaken):
		writeResponse(w, http.StatusConflict, map[string]string{"error": "email already registered"})
	case errors.Is(err, ErrInvalidCredential):
		writeResponse(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
	case errors.Is(err, ErrUnauthorized), errors.Is(err, ErrNotFound):
		writeResponse(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	default:
		writeResponse(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}

func decodeRequest(w http.ResponseWriter, r *http.Request, body any) bool {
	status, err := httpapi.DecodeJSON(w, r, body, 4<<10)
	if err != nil {
		writeResponse(w, status, map[string]string{"error": err.Error()})
		return false
	}
	return true
}

func requestContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 10*time.Second)
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var input RegisterInput
	if !decodeRequest(w, r, &input) {
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	session, err := h.service.Register(ctx, input)
	if err != nil {
		publicError(w, err)
		return
	}
	writeResponse(w, http.StatusCreated, session)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var input LoginInput
	if !decodeRequest(w, r, &input) {
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	session, err := h.service.Login(ctx, input)
	if err != nil {
		publicError(w, err)
		return
	}
	writeResponse(w, http.StatusOK, session)
}

func bearerToken(r *http.Request) (string, error) {
	value := r.Header.Get("Authorization")
	if len(value) > 128 {
		return "", ErrUnauthorized
	}

	parts := strings.Fields(value)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", ErrUnauthorized
	}
	return parts[1], nil
}

func (h *Handler) authenticated(r *http.Request) (context.Context, context.CancelFunc, string, Account, error) {
	ctx, cancel := requestContext(r)
	token, err := bearerToken(r)
	if err != nil {
		return ctx, cancel, "", Account{}, err
	}
	account, err := h.service.Authenticate(ctx, token)
	return ctx, cancel, token, account, err
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	_, cancel, _, a, err := h.authenticated(r)
	defer cancel()
	if err != nil {
		publicError(w, err)
		return
	}

	writeResponse(w, http.StatusOK, a)
}

func (h *Handler) updateProfile(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, _, a, err := h.authenticated(r)
	defer cancel()
	if err != nil {
		publicError(w, err)
		return
	}
	var input UpdateProfileInput
	if !decodeRequest(w, r, &input) {
		return
	}
	profile, err := h.service.UpdateProfile(ctx, a.User.ID, input)
	if err != nil {
		publicError(w, err)
		return
	}
	writeResponse(w, http.StatusOK, profile)
}

func (h *Handler) workspaces(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, _, a, err := h.authenticated(r)
	defer cancel()
	if err != nil {
		publicError(w, err)
		return
	}

	workspaces, err := h.service.Workspaces(ctx, a.User.ID)
	if err != nil {
		publicError(w, err)
		return
	}
	writeResponse(w, http.StatusOK, workspaces)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, token, _, err := h.authenticated(r)
	defer cancel()
	if err != nil {
		publicError(w, err)
		return
	}
	if err := h.service.Logout(ctx, token); err != nil {
		publicError(w, err)
		return
	}
	writeResponse(w, http.StatusNoContent, nil)
}
