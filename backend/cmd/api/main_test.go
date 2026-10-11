package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func performRequest(
	handler http.Handler,
	method string,
	path string,
	body string,
) *httptest.ResponseRecorder {
	req := httptest.NewRequest(
		method,
		path,
		strings.NewReader(body),
	)

	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	return recorder
}

func requireStatus(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	expected int,
) {
	t.Helper()

	if recorder.Code != expected {
		t.Fatalf(
			"expected HTTP %d, got %d: %s",
			expected,
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func requireID(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
) string {
	t.Helper()
	var response struct {
		ID string `json:"id"`
	}

	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	); err != nil {
		t.Fatal(err)
	}

	if response.ID == "" {
		t.Fatal("response did not contain an ID")
	}

	return response.ID
}

func TestCapsuleHTTPFlow(t *testing.T) {
	current := time.Date(
		2026, 10, 11, 0, 0, 0, 0, time.UTC,
	)

	router := newMux(func() time.Time {
		return current
	})

	title := "Dear Future me"
	body := "My Private Message."
	// checking the health endpoints
	res := performRequest(router, "GET", "/health", "")
	requireStatus(t, res, http.StatusOK)
	//creating a journal entry
	entryJSON := `{"title":"Dear Future me","body":"My Private Message."}`

	res = performRequest(
		router,
		"POST",
		"/api/v1/entries",
		entryJSON,
	)
	requireStatus(t, res, http.StatusCreated)

	entryID := requireID(t, res)
	//3. seal the entry for one hour
	unlockAt := current.Add(time.Hour)

	sealJSON := `{"unlock_at":"` +
		unlockAt.Format(time.RFC3339Nano) + `"}`

	res = performRequest(
		router,
		"POST",
		"/api/v1/entries/"+entryID+"/seal",
		sealJSON,
	)
	requireStatus(t, res, http.StatusCreated)

	capsuleID := requireID(t, res)

	//4. the original journal entry must be unavailable.

	res = performRequest(
		router,
		"GET",
		"/api/v1/entries/"+entryID,
		"",
	)
	requireStatus(t, res, http.StatusNotFound)

	//5. metadata must not expose private writing.
	res = performRequest(
		router,
		"GET",
		"/api/v1/capsules/"+capsuleID,
		"",
	)
	requireStatus(t, res, http.StatusOK)

	for _, secret := range []string{title, body, `"title"`, `"body"`} {
		if strings.Contains(res.Body.String(), secret) {
			t.Fatalf("metadata exposed private content: %s", secret)
		}

	}
	//6. Listing must not expose private writing.

	res = performRequest(
		router,
		"GET",
		"/api/v1/capsules",
		"",
	)
	requireStatus(t, res, http.StatusOK)
	if strings.Contains(res.Body.String(), body) {
		t.Fatal("capsule list exposed private writing")
	}

	// 7. opening before the unlock date must fail.

	res = performRequest(
		router,
		"POST",
		"/api/v1/capsules/"+capsuleID+"/open",
		"",
	)
	requireStatus(t, res, http.StatusLocked)

	if strings.Contains(res.Body.String(), body) {
		t.Fatal("locked capsule exposed its contents")
	}

	//8. move the fake clock two hours forward.
	current = current.Add(2 * time.Hour)

	//9. opening must now succeed.
	res = performRequest(
		router,
		"POST",
		"/api/v1/capsules/"+capsuleID+"/open",
		"",
	)

	requireStatus(t, res, http.StatusOK)

	if strings.Contains(res.Body.String(), `"can_open"`) {
		t.Fatal("opened capsule response has an unnecessary can_open field")
	}

	var opened struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}

	if err := json.Unmarshal(res.Body.Bytes(), &opened); err != nil {
		t.Fatal(err)
	}

	if opened.Title != title || opened.Body != body {
		t.Fatalf(
			"expected title=%q body=%q, got title=%q body=%q",
			title, body, opened.Title, opened.Body,
		)
	}

	//10. metadata should now show returned status.

	res = performRequest(
		router,
		"GET",
		"/api/v1/capsules/"+capsuleID,
		"",
	)

	requireStatus(t, res, http.StatusOK)

	var metadata struct {
		Status  string `json:"status"`
		CanOpen bool   `json:"can_open"`
	}

	if err := json.Unmarshal(res.Body.Bytes(), &metadata); err != nil {
		t.Fatal(err)
	}

	if metadata.Status != "returned" || !metadata.CanOpen {
		t.Fatal("capsule did not transition to returned")
	}

	//11. Destroy the capsule.
	res = performRequest(
		router,
		"DELETE",
		"/api/v1/capsules/"+capsuleID,
		"",
	)

	requireStatus(t, res, http.StatusNoContent)

	//12. Destroyed capsule must no longer exist.
	res = performRequest(
		router,
		"GET",
		"/api/v1/capsules/"+capsuleID,
		"",
	)
	requireStatus(t, res, http.StatusNotFound)

	//13. destroyed capsule must not reopen.

	res = performRequest(
		router,
		"POST",
		"/api/v1/capsules/"+capsuleID+"/open",
		"",
	)
	requireStatus(t, res, http.StatusNotFound)

	//14. second deletion must return not found.

	res = performRequest(
		router,
		"DELETE",
		"/api/v1/capsules/"+capsuleID,
		"",
	)
	requireStatus(t, res, http.StatusNotFound)

}

func TestInvalidSealKeepsJournalHTTP(t *testing.T) {
	current := time.Date(
		2026, 10, 11, 0, 0, 0, 0, time.UTC,
	)

	router := newMux(func() time.Time {
		return current
	})

	res := performRequest(
		router,
		"POST",
		"/api/v1/entries",
		`{"title":"Important entry", "body":"Keep this safe."}`,
	)
	requireStatus(t, res, http.StatusCreated)

	entryID := requireID(t, res)

	pastDate := current.Add(-time.Hour)

	sealJSON := `{"unlock_at":"` +
		pastDate.Format(time.RFC3339Nano) + `"}`

	res = performRequest(
		router,
		"POST",
		"/api/v1/entries/"+entryID+"/seal",
		sealJSON,
	)
	requireStatus(t, res, http.StatusBadRequest)

	res = performRequest(
		router,
		"GET",
		"/api/v1/entries/"+entryID,
		"",
	)

	requireStatus(t, res, http.StatusOK)

	if !strings.Contains(res.Body.String(), "Keep this safe.") {
		t.Fatal("original journal content was lost")
	}

	//Failed sealing must not create a capsule.
	res = performRequest(
		router,
		"GET",
		"/api/v1/capsules",
		"",
	)
	requireStatus(t, res, http.StatusOK)

	if strings.TrimSpace(res.Body.String()) != "[]" {
		t.Fatalf("expected empty capsule list, got %s", res.Body.String())
	}

}

func TestMissingCapsuleHTTP(t *testing.T) {
	router := newMux(time.Now)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "metadata",
			method: "GET",
			path:   "/api/v1/capsules/missing",
		},
		{
			name:   "open",
			method: "POST",
			path:   "/api/v1/capsules/missing/open",
		},
		{
			name:   "delete",
			method: "DELETE",
			path:   "/api/v1/capsules/missing",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := performRequest(
				router,
				tt.method,
				tt.path,
				"",
			)
			requireStatus(t, res, http.StatusNotFound)
		})
	}
}
func TestJournalHTTPRejectsBadJSON(t *testing.T) {
	router := newMux(time.Now)

	tests := []struct {
		name   string
		body   string
		status int
	}{
		{
			name:   "unknown field",
			body:   `{"title":"Hello","unexpected":true}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "multiple JSON documents",
			body:   `{"title":"One"}{"title":"Two"}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "malformed JSON",
			body:   `{"title":`,
			status: http.StatusBadRequest,
		},
		{
			name: "oversized request",
			body: `{"title":"` +
				strings.Repeat("x", 1<<20) + `"}`,
			status: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := performRequest(
				router,
				"POST",
				"/api/v1/entries",
				tt.body,
			)

			requireStatus(t, res, tt.status)
		})
	}
}

func TestJournalHTTPRejectMissingContentType(t *testing.T) {
	router := newMux(time.Now)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/entries",
		strings.NewReader(`{"title":"Hello"}`),
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	requireStatus(
		t,
		recorder,
		http.StatusUnsupportedMediaType,
	)
}

func TestSealHTTPRejectUnknownFields(t *testing.T) {
	current := time.Date(
		2026, 10, 11, 0, 0, 0, 0, time.UTC,
	)

	router := newMux(func() time.Time {
		return current
	})

	res := performRequest(
		router,
		"POST",
		"/api/v1/entries",
		`{"title":"Important","body":"Do not lose me."}`,
	)

	requireStatus(t, res, http.StatusCreated)

	entryID := requireID(t, res)

	unlockAt := current.Add(24 * time.Hour)
	body := `{"unlock_at":"` +
		unlockAt.Format(time.RFC3339Nano) +
		`","unexpected":true}`
	res = performRequest(
		router,
		"POST",
		"/api/v1/entries/"+entryID+"/seal",
		body,
	)
	requireStatus(t, res, http.StatusBadRequest)

	res = performRequest(
		router,
		"GET",
		"/api/v1/entries/"+entryID,
		"",
	)

	requireStatus(t, res, http.StatusOK)

	if !strings.Contains(
		res.Body.String(),
		"Do not lose me.",
	) {
		t.Fatal("failed sealing lost the journal content")
	}
}
