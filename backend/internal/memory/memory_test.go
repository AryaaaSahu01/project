package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"journalapp/internal/account"
)

// This opt-in suite creates and drops its own schema. It never changes public
// tables. Use a development/test database whose user may create schemas.
func TestPersistentMemoryLifecycle(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL lifecycle/isolation tests")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("chrono_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	admin.Close(ctx)
	defer func() {
		conn, err := pgx.Connect(ctx, dsn)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close(ctx)
		if _, err := conn.Exec(ctx, `DROP SCHEMA `+pgx.Identifier{schema}.Sanitize()+` CASCADE`); err != nil {
			t.Error(err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET search_path TO `+pgx.Identifier{schema}.Sanitize())
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var active string
	if err := pool.QueryRow(ctx, "SELECT current_schema()").Scan(&active); err != nil || active != schema {
		t.Fatalf("schema isolation: %q %v", active, err)
	}
	migration, err := os.ReadFile("../../migrations/000001_core.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	svc, err := account.NewService(account.NewPostgresStore(pool))
	if err != nil {
		t.Fatal(err)
	}
	a, err := svc.Register(ctx, account.RegisterInput{Email: "a@example.com", Password: "a-long-passphrase", DisplayName: "A"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.Register(ctx, account.RegisterInput{Email: "b@example.com", Password: "a-long-passphrase", DisplayName: "B"})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(pool, svc)
	input := CreateInput{ID: "820c402a-753d-4e23-b39c-10b5307a2d40", WorkspaceID: a.Account.Workspace.ID, Title: "Private title", Body: "private-needle: words for the future"}
	m, err := h.create(ctx, a.Account.User.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	again, err := h.create(ctx, a.Account.User.ID, input)
	if err != nil || again.ID != m.ID {
		t.Fatalf("retry must not duplicate: %+v %v", again, err)
	}
	// A fresh handler represents a new request/process using the same database.
	h = NewHandler(pool, svc)
	loaded, err := h.get(ctx, a.Account.User.ID, m.ID)
	if err != nil || loaded.Body == nil || *loaded.Body != input.Body {
		t.Fatalf("persistence: %+v %v", loaded, err)
	}
	for _, op := range []func() error{
		func() error { _, e := h.get(ctx, b.Account.User.ID, m.ID); return e },
		func() error { _, e := h.open(ctx, b.Account.User.ID, m.ID); return e },
		func() error { return h.remove(ctx, b.Account.User.ID, m.ID) },
	} {
		if err := op(); !errors.Is(err, ErrNotFound) {
			t.Fatalf("foreign access: %v", err)
		}
	}
	name := "Updated title"
	updated, err := h.update(ctx, a.Account.User.ID, m.ID, UpdateInput{Title: &name, ExpectedUpdatedAt: &m.UpdatedAt})
	if err != nil {
		t.Fatal(err)
	}
	stale := m.UpdatedAt.Add(-time.Second)
	staleTitle := "Stale overwrite"
	if _, err := h.update(ctx, a.Account.User.ID, m.ID, UpdateInput{Title: &staleTitle, ExpectedUpdatedAt: &stale}); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale write: %v", err)
	}
	if _, err := h.seal(ctx, a.Account.User.ID, m.ID, time.Now().Add(20*time.Second)); !errors.Is(err, ErrInput) {
		t.Fatalf("early seal: %v", err)
	}
	sealed, err := h.seal(ctx, a.Account.User.ID, m.ID, time.Now().Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if sealed.Title != nil || sealed.Body != nil || sealed.CanOpen {
		t.Fatalf("sealed content exposed: %+v", sealed)
	}
	data, _ := json.Marshal(sealed)
	if strings.Contains(string(data), "private-needle") || strings.Contains(string(data), "Updated title") {
		t.Fatal("sealed JSON exposed writing")
	}
	if _, err := h.open(ctx, a.Account.User.ID, m.ID); !errors.Is(err, ErrLocked) {
		t.Fatalf("early open: %v", err)
	}
	if _, err := h.update(ctx, a.Account.User.ID, m.ID, UpdateInput{Title: &name, ExpectedUpdatedAt: &updated.UpdatedAt}); !errors.Is(err, ErrConflict) {
		t.Fatalf("sealed edit: %v", err)
	}
	items, err := h.listFiltered(ctx, a.Account.User.ID, url.Values{"workspace_id": {input.WorkspaceID}, "q": {"private-needle"}})
	if err != nil || len(items) != 0 {
		t.Fatalf("sealed search oracle: %d %v", len(items), err)
	}
	// Move only this fixture's schedule into the past to test the real SQL gate
	// without making every test run wait three minutes.
	if _, err := pool.Exec(ctx, `UPDATE memories SET sealed_at=now()-interval '3 minutes',unlock_at=now()-interval '1 minute' WHERE id=$1::uuid`, m.ID); err != nil {
		t.Fatal(err)
	}
	ready, err := h.get(ctx, a.Account.User.ID, m.ID)
	if err != nil || !ready.CanOpen || ready.Body != nil {
		t.Fatalf("ready must still hide writing: %+v %v", ready, err)
	}
	opened, err := h.open(ctx, a.Account.User.ID, m.ID)
	if err != nil || opened.Status != "returned" || opened.Body == nil || *opened.Body != input.Body {
		t.Fatalf("open: %+v %v", opened, err)
	}
	if _, err := h.open(ctx, a.Account.User.ID, m.ID); err != nil {
		t.Fatalf("repeat open: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO reflections(workspace_id,memory_id,author_user_id,body) VALUES($1::uuid,$2::uuid,$3::uuid,'Keep the relationship intact')`, input.WorkspaceID, m.ID, a.Account.User.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.remove(ctx, a.Account.User.ID, m.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("must not orphan reflection writing: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM reflections WHERE memory_id=$1::uuid`, m.ID); err != nil {
		t.Fatal(err)
	}
	toDelete, err := h.create(ctx, a.Account.User.ID, CreateInput{WorkspaceID: input.WorkspaceID, Title: "Remove a sealed memory", Body: "Writing to delete"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.seal(ctx, a.Account.User.ID, toDelete.ID, time.Now().Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := h.remove(ctx, a.Account.User.ID, toDelete.ID); err != nil {
		t.Fatalf("delete sealed memory: %v", err)
	}
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	r := httptest.NewRequest("GET", "/api/v1/memories/"+m.ID, nil)
	r.Header.Set("Authorization", "Bearer "+b.AccessToken)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatalf("foreign HTTP read = %d", w.Code)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO memories(workspace_id,created_by,title,body) SELECT $1::uuid,$2::uuid,'Page '||n,'saved' FROM generate_series(1,105) n`, input.WorkspaceID, a.Account.User.ID); err != nil {
		t.Fatal(err)
	}
	first, err := h.listFiltered(ctx, a.Account.User.ID, url.Values{"workspace_id": {input.WorkspaceID}})
	if err != nil || len(first) != 100 {
		t.Fatalf("first page %d %v", len(first), err)
	}
	second, err := h.listFiltered(ctx, a.Account.User.ID, url.Values{"workspace_id": {input.WorkspaceID}, "before": {first[99].ID}})
	if err != nil || len(second) != 6 {
		t.Fatalf("second page %d %v", len(second), err)
	}
	seen := map[string]bool{}
	for _, item := range append(first, second...) {
		if seen[item.ID] {
			t.Fatal("duplicate across pages")
		}
		seen[item.ID] = true
	}
	if err := h.remove(ctx, a.Account.User.ID, m.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.get(ctx, a.Account.User.ID, m.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete: %v", err)
	}
}
