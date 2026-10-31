package media

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"journalapp/internal/account"
	"journalapp/internal/memory"
	"journalapp/internal/platform/objectstore"
)

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/euoAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func TestRangeBoundaries(t *testing.T) {
	for _, tc := range []struct {
		value          string
		offset, length int64
		bad            bool
	}{
		{"", 0, 10, false}, {"bytes=0-0", 0, 1, false}, {"bytes=3-", 3, 7, false}, {"bytes=-3", 7, 3, false},
		{"bytes=2-100", 2, 8, false}, {"bytes=-30", 0, 10, false}, {"bytes=10-", 0, 0, true},
		{"bytes=3-2", 0, 0, true}, {"bytes=0-1,3-4", 0, 0, true}, {"bytes=-0", 0, 0, true}, {"items=0-3", 0, 0, true},
	} {
		o, n, _, err := parseRange(tc.value, 10)
		if (err != nil) != tc.bad || !tc.bad && (o != tc.offset || n != tc.length) {
			t.Errorf("%q -> %d %d %v", tc.value, o, n, err)
		}
	}
}
func TestContainerValidation(t *testing.T) {
	for _, tc := range []struct {
		data  []byte
		kind  string
		valid bool
	}{
		{tinyPNG(t), "image/png", true}, {tinyPNG(t), "image/jpeg", false},
		{[]byte("<script>alert(1)</script>"), "image/png", false}, {nil, "audio/webm", false},
		{[]byte("GIF89a"), "image/webp", false},
	} {
		f, err := os.CreateTemp(t.TempDir(), "media-")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(tc.data); err != nil {
			t.Fatal(err)
		}
		err = inspectFile(f, tc.kind)
		f.Close()
		if (err == nil) != tc.valid {
			t.Errorf("%s valid=%v err=%v", tc.kind, tc.valid, err)
		}
	}
	if safeName("../../private\\note\n.wav") != "note.wav" {
		t.Fatal("filename sanitization failed")
	}
}

// Opt-in against development PostgreSQL only; each run uses its own schema and
// temporary private file directory, and never changes public application data.
func TestPrivateMediaLifecycle(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL for media integration tests")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("media_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	admin.Close(ctx)
	defer func() {
		c, err := pgx.Connect(ctx, dsn)
		if err != nil {
			t.Error(err)
			return
		}
		defer c.Close(ctx)
		_, err = c.Exec(ctx, `DROP SCHEMA `+pgx.Identifier{schema}.Sanitize()+` CASCADE`)
		if err != nil {
			t.Error(err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		_, err := c.Exec(ctx, `SET search_path TO `+pgx.Identifier{schema}.Sanitize())
		return err
	}
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, path := range []string{"../../migrations/000001_core.up.sql", "../../migrations/000002_media.up.sql", "../../migrations/000002_media.up.sql"} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, string(b)); err != nil {
			t.Fatal(err)
		}
	}
	accounts, err := account.NewService(account.NewPostgresStore(db))
	if err != nil {
		t.Fatal(err)
	}
	owner, err := accounts.Register(ctx, account.RegisterInput{Email: "media-owner@example.com", Password: "long-test-passphrase", DisplayName: "Owner"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := accounts.Register(ctx, account.RegisterInput{Email: "media-other@example.com", Password: "long-test-passphrase", DisplayName: "Other"})
	if err != nil {
		t.Fatal(err)
	}
	objects, err := objectstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	NewHandler(db, accounts, objects).RegisterRoutes(mux)
	memory.NewHandler(db, accounts).RegisterRoutes(mux)
	request := func(method, path, token string, data []byte, headers map[string]string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/v1"+path, bytes.NewReader(data))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	want := func(w *httptest.ResponseRecorder, code int) {
		t.Helper()
		if w.Code != code {
			t.Fatalf("status=%d want=%d body=%s", w.Code, code, w.Body.String())
		}
	}
	create := func() string {
		t.Helper()
		b, _ := json.Marshal(map[string]string{"workspace_id": owner.Account.Workspace.ID, "title": "Media test", "body": "private writing"})
		w := request("POST", "/memories", owner.AccessToken, b, map[string]string{"Content-Type": "application/json"})
		want(w, 201)
		var m memory.Memory
		if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		return m.ID
	}
	id := create()
	path := "/memories/" + id + "/media"
	png := tinyPNG(t)
	headers := map[string]string{"Content-Type": "image/png", "X-Upload-Name": "Picture.png", "X-Upload-Size": strconv.Itoa(len(png)), "X-Upload-ID": "720c402a-753d-4e23-b39c-10b5307a2d40"}
	want(request("POST", path, "", png, headers), 401)
	want(request("POST", path, other.AccessToken, png, headers), 404)
	w := request("POST", path, owner.AccessToken, png, headers)
	want(w, 201)
	var a asset
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	want(request("POST", path, owner.AccessToken, png, headers), 200) // retry does not duplicate
	w = request("GET", path, owner.AccessToken, nil, nil)
	want(w, 200)
	var list []asset
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list) != 1 {
		t.Fatalf("retry duplicated attachment: %s", w.Body.String())
	}
	content := path + "/" + a.ID + "/content"
	want(request("GET", content, other.AccessToken, nil, nil), 404)
	w = request("GET", content, owner.AccessToken, nil, map[string]string{"Range": "bytes=1-8"})
	want(w, 206)
	if !bytes.Equal(w.Body.Bytes(), png[1:9]) || w.Header().Get("Content-Range") != fmt.Sprintf("bytes 1-8/%d", len(png)) {
		t.Fatal("incorrect byte range")
	}
	if w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("private response may be cached")
	}
	want(request("GET", content, owner.AccessToken, nil, map[string]string{"Range": "bytes=999999-"}), 416)
	want(request("HEAD", content, owner.AccessToken, nil, nil), 200)
	// Invalid content never becomes readable, and quota size checks happen first.
	bad := map[string]string{"Content-Type": "image/png", "X-Upload-Size": "12", "X-Upload-Name": "bad.png"}
	want(request("POST", path, owner.AccessToken, []byte("not an image"), bad), 422)
	bad["X-Upload-Size"] = "999999999"
	want(request("POST", path, owner.AccessToken, nil, bad), 413)
	// An upload in flight blocks both sealing and whole-memory deletion.
	pending := "730c402a-753d-4e23-b39c-10b5307a2d40"
	_, err = db.Exec(ctx, `INSERT INTO media_assets(id,workspace_id,uploaded_by,object_key,kind,content_type,declared_bytes,memory_id)
        VALUES($1::uuid,$2::uuid,$3::uuid,'assets/pending/test','audio','audio/webm',10,$4::uuid)`, pending, owner.Account.Workspace.ID, owner.Account.User.ID, id)
	if err != nil {
		t.Fatal(err)
	}
	seal, _ := json.Marshal(map[string]string{"unlock_at": time.Now().Add(3 * time.Minute).UTC().Format(time.RFC3339Nano)})
	want(request("POST", "/memories/"+id+"/seal", owner.AccessToken, seal, map[string]string{"Content-Type": "application/json"}), 409)
	want(request("DELETE", "/memories/"+id, owner.AccessToken, nil, nil), 409)
	if _, err := db.Exec(ctx, `UPDATE media_assets SET status='rejected' WHERE id=$1::uuid`, pending); err != nil {
		t.Fatal(err)
	}
	want(request("POST", "/memories/"+id+"/seal", owner.AccessToken, seal, map[string]string{"Content-Type": "application/json"}), 200)
	want(request("GET", path, owner.AccessToken, nil, nil), 423)
	want(request("GET", content, owner.AccessToken, nil, nil), 423)
	want(request("POST", "/memories/"+id+"/open", owner.AccessToken, nil, nil), 423)
	want(request("DELETE", path+"/"+a.ID, owner.AccessToken, nil, nil), 409)
	if _, err := db.Exec(ctx, `UPDATE memories SET sealed_at=now()-interval '3 minutes',unlock_at=now()-interval '1 minute' WHERE id=$1::uuid`, id); err != nil {
		t.Fatal(err)
	}
	want(request("GET", content, owner.AccessToken, nil, nil), 423) // due is not opened
	want(request("POST", "/memories/"+id+"/open", owner.AccessToken, nil, nil), 200)
	w = request("GET", content, owner.AccessToken, nil, nil)
	want(w, 200)
	if !bytes.Equal(w.Body.Bytes(), png) {
		t.Fatal("media changed on return")
	}
	want(request("DELETE", "/memories/"+id, owner.AccessToken, nil, nil), 204)
	want(request("GET", content, owner.AccessToken, nil, nil), 404)
	if _, err := CleanupExpired(ctx, db, objects); err != nil {
		t.Fatal(err)
	}
	if body, err := objects.ReadRange(ctx, "assets/"+owner.Account.Workspace.ID+"/"+a.ID, 0, 1); err == nil {
		body.Close()
		t.Fatal("deleted memory left its private file")
	}
	// Detaching from a saved draft also queues deletion, without deleting text.
	id = create()
	path = "/memories/" + id + "/media"
	delete(headers, "X-Upload-ID")
	w = request("POST", path, owner.AccessToken, png, headers)
	want(w, 201)
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	want(request("DELETE", path+"/"+a.ID, owner.AccessToken, nil, nil), 204)
	want(request("GET", "/memories/"+id, owner.AccessToken, nil, nil), 200)
	if _, err := CleanupExpired(ctx, db, objects); err != nil {
		t.Fatal(err)
	}
	w = request("GET", path, owner.AccessToken, nil, nil)
	want(w, 200)
	if strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatal("attachment still visible after removal")
	}
}
