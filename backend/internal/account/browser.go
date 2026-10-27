package account

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func WithBrowserSessions(mux *http.ServeMux, service *Service, appOrigin string) (http.Handler, error) {
	if appOrigin == "" {
		appOrigin = "http://localhost:5173"
	}

	u, err := url.Parse(appOrigin)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" ||
		u.Fragment != "" || (u.Path != "" && u.Path != "/") ||
		(u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("APP_ORIGIN must be one http or https origin")
	}

	loopback := u.Hostname() == "localhost"
	if ip := net.ParseIP(u.Hostname()); ip != nil {
		loopback = ip.IsLoopback()
	}

	if u.Scheme == "http" && !loopback {
		return nil, errors.New("APP_ORIGIN  must use https outside localhost")
	}

	b := &browserHandler{
		accounts: NewHandler(service),
		origin:   u.Scheme + "://" + u.Host,
		secure:   u.Scheme == "https",
	}

	mux.HandleFunc("POST /api/v1/browser/auth/register", b.register)
	mux.HandleFunc("POST /api/v1/browser/auth/login", b.login)
	mux.HandleFunc("POST /api/v1/browser/auth/logout", b.logout)
	mux.HandleFunc("GET /api/v1/browser/session", b.session)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			mux.ServeHTTP(w, r)
			return
		}
		cookie, cookieErr := r.Cookie(b.cookieName())
		browserAuth := strings.HasPrefix(r.URL.Path, "/api/v1/browser/auth/")
		cookieAuth := cookieErr == nil && r.Header.Get("Authorization") == ""
		unsafe := r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions

		if unsafe && (browserAuth || cookieAuth) &&
			(r.Header.Get("Origin") != b.origin || r.Header.Get("Sec-Fetch-Site") == "cross-site") {
			writeResponse(w, http.StatusForbidden, map[string]string{"error": "request origin not allowed"})
			return
		}

		if cookieAuth {
			r = r.Clone(r.Context())
			r.Header.Set("Authorization", "Bearer "+cookie.Value)
		}
		mux.ServeHTTP(w, r)

	}), nil
}

type browserHandler struct {
	accounts *Handler
	origin   string
	secure   bool
}

func (b *browserHandler) cookieName() string {
	if b.secure {
		return "__Host-chrono_session"
	}
	return "chrono-session"
}

func (b *browserHandler) setCookie(w http.ResponseWriter, token string, expires time.Time) {
	maxAge := int(SessionLifetime.Seconds())
	if token == "" {
		maxAge = -1
		expires = time.Unix(1, 0)
	}

	http.SetCookie(w, &http.Cookie{
		Name: b.cookieName(), Value: token, Path: "/",
		HttpOnly: true, Secure: b.secure, SameSite: http.SameSiteStrictMode,
		MaxAge: maxAge, Expires: expires,
	})
}

func (b *browserHandler) register(w http.ResponseWriter, r *http.Request) {
	var input RegisterInput
	if !decodeRequest(w, r, &input) {
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	session, err := b.accounts.service.Register(ctx, input)
	if err != nil {
		publicError(w, err)
		return
	}

	b.setCookie(w, session.AccessToken, session.ExpiresAt)
	writeResponse(w, http.StatusCreated, map[string]any{"account": session.Account})
}

func (b *browserHandler) login(w http.ResponseWriter, r *http.Request) {
	var input LoginInput
	if !decodeRequest(w, r, &input) {
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()
	session, err := b.accounts.service.Login(ctx, input)
	if err != nil {
		publicError(w, err)
		return
	}

	b.setCookie(w, session.AccessToken, session.ExpiresAt)
	writeResponse(w, http.StatusOK, map[string]any{"account": session.Account})
}

func (b *browserHandler) session(w http.ResponseWriter, r *http.Request) {
	_, cancel, _, a, err := b.accounts.authenticated(r)
	defer cancel()
	if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrNotFound) {
		writeResponse(w, http.StatusOK, map[string]any{"account": nil})
		return
	}
	if err != nil {
		publicError(w, err)
		return
	}

	writeResponse(w, http.StatusOK, map[string]any{"account": a})
}

func (b *browserHandler) logout(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestContext(r)
	defer cancel()
	token, err := bearerToken(r)
	if err == nil {
		err = b.accounts.service.Logout(ctx, token)

	}
	if err != nil && !errors.Is(err, ErrUnauthorized) && !errors.Is(err, ErrNotFound) {
		publicError(w, err)
		return
	}
	b.setCookie(w, "", time.Time{})
	writeResponse(w, http.StatusNoContent, nil)
}
