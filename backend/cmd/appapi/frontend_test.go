package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFrontendFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>Chrono Journal</h1>"), 0600); err != nil {
		t.Fatal(err)
	}
	h, err := frontendHandler(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path   string
		status int
	}{{"/memories/123", 200}, {"/api/v1/missing", 404}, {"/assets/missing.js", 404}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", test.path, nil))
		if w.Code != test.status {
			t.Errorf("%s = %d", test.path, w.Code)
		}
		if test.status == 200 && w.Header().Get("Cache-Control") != "no-store" {
			t.Error("SPA shell must not be cached")
		}
	}
}
