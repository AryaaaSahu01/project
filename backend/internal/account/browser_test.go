package account

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type browserTestStore struct {
	account    Account
	hash       string
	sessions   map[[32]byte]time.Time
	revokeFail bool
}

func (s *browserTestStore) Register(_ context.Context, email, hash, name string, token [32]byte, expires time.Time) (Account, error) {
	if s.hash != "" {
		return Account{}, ErrEmailTaken
	}
	s.hash = hash
	s.account = Account{User: User{ID: "test-user", Email: email, CreatedAt: time.Now()}, Profile: Profile{UserID: "test-user", DisplayName: name}}
	s.sessions[token] = expires
	return s.account, nil
}
func (s *browserTestStore) PasswordRecord(_ context.Context, email string) (string, string, error) {
	if email != s.account.User.Email {
		return "", "", ErrNotFound
	}
	return s.account.User.ID, s.hash, nil
}
func (s *browserTestStore) CreateSession(_ context.Context, _ string, token [32]byte, expires time.Time) error {
	s.sessions[token] = expires
	return nil
}
func (s *browserTestStore) GetAccountByTokenHash(_ context.Context, token [32]byte) (Account, error) {
	if !s.sessions[token].After(time.Now()) {
		return Account{}, ErrNotFound
	}
	return s.account, nil
}
func (s *browserTestStore) UpdateProfile(_ context.Context, _ string, input UpdateProfileInput) (Profile, error) {
	if input.DisplayName != nil {
		s.account.Profile.DisplayName = *input.DisplayName
	}
	return s.account.Profile, nil
}
func (s *browserTestStore) ListWorkspaces(context.Context, string) ([]Workspace, error) {
	return []Workspace{}, nil
}
func (s *browserTestStore) RevokeSession(_ context.Context, token [32]byte) error {
	if s.revokeFail {
		return errors.New("database unavailable")
	}
	if _, ok := s.sessions[token]; !ok {
		return ErrUnauthorized
	}
	delete(s.sessions, token)
	return nil
}

func TestBrowserOriginConfiguration(t *testing.T) {
	for _, origin := range []string{"https://journal.test/path", "https://user@journal.test", "http://journal.test", "https://journal.test?x=1", "null"} {
		if _, err := WithBrowserSessions(http.NewServeMux(), nil, origin); err == nil {
			t.Errorf("accepted invalid origin %q", origin)
		}
	}
	for _, origin := range []string{"", "http://localhost:5173", "http://127.0.0.1:5173", "https://journal.test"} {
		if _, err := WithBrowserSessions(http.NewServeMux(), nil, origin); err != nil {
			t.Errorf("rejected origin %q: %v", origin, err)
		}
	}
}

func TestBrowserSessions(t *testing.T) {
	store := &browserTestStore{sessions: make(map[[32]byte]time.Time)}
	service, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	NewHandler(service).RegisterRoutes(mux)
	handler, err := WithBrowserSessions(mux, service, "https://journal.test")
	if err != nil {
		t.Fatal(err)
	}
	do := func(method, path, body, origin string, cookie *http.Cookie, bearer string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://journal.test"+path, strings.NewReader(body))
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	wantStatus := func(w *httptest.ResponseRecorder, expected int) {
		t.Helper()
		if w.Code != expected {
			t.Fatalf("status = %d, want %d: %s", w.Code, expected, w.Body.String())
		}
	}
	credentials := `{"email":"writer@example.com","password":"a-long-passphrase","display_name":"Writer"}`
	for _, origin := range []string{"", "https://other.test", "https://journal.test.evil.test", "null"} {
		wantStatus(do("POST", "/api/v1/browser/auth/register", credentials, origin, nil, ""), 403)
	}
	guest := do("GET", "/api/v1/browser/session", "", "", nil, "")
	wantStatus(guest, 200)
	if strings.TrimSpace(guest.Body.String()) != `{"account":null}` {
		t.Fatal("guest session must be explicit")
	}
	registered := do("POST", "/api/v1/browser/auth/register", credentials, "https://journal.test", nil, "")
	wantStatus(registered, 201)
	cookies := registered.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("expected one session cookie")
	}
	cookie := cookies[0]
	if cookie.Name != "__Host-chrono_session" || !cookie.HttpOnly || !cookie.Secure ||
		cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Domain != "" || cookie.MaxAge != 604800 {
		t.Fatalf("incorrect cookie attributes: %s", cookie.Name)
	}
	if strings.Contains(registered.Body.String(), "access_token") || strings.Contains(registered.Body.String(), cookie.Value) {
		t.Fatal("browser JSON leaked a token")
	}
	if registered.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("account responses must not be cached")
	}
	current := do("GET", "/api/v1/browser/session", "", "", cookie, "")
	wantStatus(current, 200)
	if !strings.Contains(current.Body.String(), "writer@example.com") {
		t.Fatal("cookie did not authenticate")
	}
	wantStatus(do("GET", "/api/v1/me", "", "", cookie, ""), 200)
	for _, origin := range []string{"", "https://other.test"} {
		wantStatus(do("PATCH", "/api/v1/me", `{"display_name":"Changed"}`, origin, cookie, ""), 403)
	}
	wantStatus(do("PATCH", "/api/v1/me", `{"display_name":"Changed"}`, "https://journal.test", cookie, ""), 200)
	// An explicit invalid Bearer header must not fall back to an ambient cookie.
	wantStatus(do("GET", "/api/v1/me", "", "", cookie, "invalid"), 401)
	loginBody := `{"email":"writer@example.com","password":"a-long-passphrase"}`
	legacy := do("POST", "/api/v1/auth/login", loginBody, "", nil, "")
	wantStatus(legacy, 200)
	var session Session
	if err := json.Unmarshal(legacy.Body.Bytes(), &session); err != nil || session.AccessToken == "" {
		t.Fatal("legacy Bearer contract changed")
	}
	wantStatus(do("PATCH", "/api/v1/me", `{"display_name":"Bearer writer"}`, "", cookie, session.AccessToken), 200)
	wantStatus(do("POST", "/api/v1/browser/auth/login", `{"email":"writer@example.com","password":"wrong"}`, "https://journal.test", nil, ""), 401)
	wantStatus(do("POST", "/api/v1/browser/auth/register", credentials, "https://journal.test", nil, ""), 409)
	store.revokeFail = true
	failedLogout := do("POST", "/api/v1/browser/auth/logout", "", "https://journal.test", cookie, "")
	wantStatus(failedLogout, 500)
	if len(failedLogout.Result().Cookies()) != 0 {
		t.Fatal("failed logout must not pretend to revoke the session")
	}
	store.revokeFail = false
	wantStatus(do("POST", "/api/v1/browser/auth/logout", "", "https://other.test", cookie, ""), 403)
	logout := do("POST", "/api/v1/browser/auth/logout", "", "https://journal.test", cookie, "")
	wantStatus(logout, 204)
	if logout.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("logout must expire the cookie")
	}
	wantStatus(do("GET", "/api/v1/me", "", "", cookie, ""), 401)
	wantStatus(do("POST", "/api/v1/browser/auth/logout", "", "https://journal.test", cookie, ""), 204)
	// The independent Bearer session remains valid.
	wantStatus(do("GET", "/api/v1/me", "", "", nil, session.AccessToken), 200)
}
