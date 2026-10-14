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

func TestReflectionHTTPFlow(t *testing.T) {
	current := time.Date(
		2026, 10, 12, 0, 0, 0, 0, time.UTC,
	)

	router := newMux(func() time.Time {
		return current
	})

	res := performRequest(
		router,
		"POST",
		"/api/v1/entries",
		`{"title":"Past me","body":"Remember today."}`,
	)

	requireStatus(t, res, http.StatusCreated)

	entryID := requireID(t, res)

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

	path := "/api/v1/capsules/" +
		capsuleID + "/reflections"

	res = performRequest(
		router,
		"POST",
		path,
		`{"body":"Too early"}`,
	)
	requireStatus(t, res, http.StatusConflict)

	current = current.Add(2 * time.Hour)

	res = performRequest(
		router,
		"POST",
		path,
		`{"body":"Still unopened"}`,
	)

	requireStatus(t, res, http.StatusConflict)

	res = performRequest(
		router,
		"POST",
		"/api/v1/capsules/"+capsuleID+"/open",
		"",
	)

	requireStatus(t, res, http.StatusOK)

	res = performRequest(
		router,
		"POST",
		path,
		`{"body":"First reflection"}`,
	)
	requireStatus(t, res, http.StatusCreated)

	firstID := requireID(t, res)

	current = current.Add(time.Minute)

	res = performRequest(
		router,
		"POST",
		path,
		`{"body":"Second reflection"}`,
	)

	requireStatus(t, res, http.StatusCreated)

	res = performRequest(
		router,
		"GET",
		path,
		"",
	)

	requireStatus(t, res, http.StatusOK)

	var reflections []struct {
		ID        string `json:"id"`
		CapsuleID string `json:"capsule_id"`
		Body      string `json:"body"`
	}

	if err := json.Unmarshal(
		res.Body.Bytes(),
		&reflections,
	); err != nil {
		t.Fatal(err)
	}

	if len(reflections) != 2 {
		t.Fatalf(
			"expected 2 reflections, got %d",
			len(reflections),
		)
	}

	if reflections[0].ID != firstID ||
		reflections[0].Body != "First reflection" ||
		reflections[1].Body != "Second reflection" {
		t.Fatal("reflection history is incorrect")
	}

	for _, reflection := range reflections {
		if reflection.CapsuleID != capsuleID {
			t.Fatal("reflections belongs to wrong capsule")
		}
	}

	res = performRequest(
		router,
		"GET",
		"/api/v1/capsules/"+capsuleID,
		"",
	)

	requireStatus(t, res, http.StatusOK)

	if strings.Contains(
		res.Body.String(),
		"First reflection",
	) {
		t.Fatal("capsule metadata exposed reflection content")
	}

	res = performRequest(
		router,
		"DELETE",
		"/api/v1/capsules/"+capsuleID,
		"",
	)

	requireStatus(t, res, http.StatusNoContent)

	res = performRequest(
		router,
		"GET",
		path,
		"",
	)
	requireStatus(t, res, http.StatusNotFound)
}

func newOpenendCapsuleRouter(
	t *testing.T,
) (http.Handler, string) {
	t.Helper()

	current := time.Date(
		2026, 10, 12, 0, 0, 0, 0, time.UTC,
	)

	router := newMux(func() time.Time {
		return current
	})

	res := performRequest(
		router,
		"POST",
		"/api/v1/entries",
		`{"title":"Memory", "body":"Original writing"}`,
	)
	requireStatus(t, res, http.StatusCreated)

	entryID := requireID(t, res)
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

	current = current.Add(2 * time.Hour)

	res = performRequest(
		router,
		"POST",
		"/api/v1/capsules/"+capsuleID+"/open",
		"",
	)
	requireStatus(t, res, http.StatusOK)

	return router, capsuleID
}

func TestReflectionHTTPValidation(t *testing.T) {
	router, capsuleID := newOpenendCapsuleRouter(t)

	path := "/api/v1/capsules/" +
		capsuleID + "/reflections"

	tests := []struct {
		name   string
		body   string
		status int
	}{
		{
			name:   "empty reflection",
			body:   `{"body":"   "}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "missing body",
			body:   `{}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "unknown field",
			body:   `{"body":"Hello","unknown":"true}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "malformed JSON",
			body:   `{"body":}`,
			status: http.StatusBadRequest,
		},
		{
			name: "reflection too long",
			body: `{"body":"` +
				strings.Repeat("x", 20001) + `"}`,
			status: http.StatusBadRequest,
		},
		{
			name: "oversized request",
			body: `{"body":"` +
				strings.Repeat("x", 1<<17) + `"}`,
			status: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := performRequest(
				router,
				"POST",
				path,
				tt.body,
			)

			requireStatus(t, res, tt.status)
		})
	}

	req := httptest.NewRequest(
		http.MethodPost,
		path,
		strings.NewReader(`{"body":"Hello"}`),
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	requireStatus(
		t,
		recorder,
		http.StatusUnsupportedMediaType,
	)

	res := performRequest(router, "GET", path, "")
	requireStatus(t, res, http.StatusOK)

	if strings.TrimSpace(res.Body.String()) != "[]" {
		t.Fatalf(
			"invalid requests created reflections : %s",
			res.Body.String(),
		)
	}
}

func TestReflectionHTTPMissingCapsule(t *testing.T) {
	router := newMux(time.Now)

	path := "/api/v1/capsules/missing/reflections"

	res := performRequest(
		router,
		"GET",
		path,
		"",
	)
	requireStatus(t, res, http.StatusNotFound)

	if !strings.Contains(res.Body.String(), "capsule not found") {
		t.Fatalf(
			"expected capsule not found error, got %s",
			res.Body.String(),
		)
	}

	res = performRequest(
		router,
		"POST",
		path,
		`{"body":"This should fail"}`,
	)
	requireStatus(t, res, http.StatusNotFound)

	if !strings.Contains(res.Body.String(), "capsule not found") {
		t.Fatalf(
			"expected capsule not found error, got %s",
			res.Body.String(),
		)
	}
}

func TestResealReflectionHTTPFlow(t *testing.T) {
	current := time.Date(
		2026, 10, 13, 0, 0, 0, 0, time.UTC,
	)

	router := newMux(func() time.Time {
		return current
	})
	res := performRequest(
		router,
		"POST",
		"/api/v1/entries",
		`{"title":"Past me","body":"An old memory"}`,
	)

	requireStatus(t, res, http.StatusCreated)

	entryID := requireID(t, res)

	unlockAt := current.Add(time.Hour)

	sealBody := `{"unlock_at":"` +
		unlockAt.Format(time.RFC3339Nano) + `"}`

	res = performRequest(
		router,
		"POST",
		"/api/v1/entries/"+entryID+"/seal",
		sealBody,
	)

	requireStatus(t, res, http.StatusCreated)

	parentID := requireID(t, res)

	current = current.Add(2 * time.Hour)

	res = performRequest(
		router,
		"POST",
		"/api/v1/capsules/"+parentID+"/open",
		"",
	)

	requireStatus(t, res, http.StatusOK)

	reflectionText := "My life has changed."

	reflectionPath := "/api/v1/capsules/" +
		parentID + "/reflections"

	res = performRequest(
		router,
		"POST",
		reflectionPath,
		`{"body":"My life has changed."}`,
	)

	requireStatus(t, res, http.StatusCreated)

	reflectionID := requireID(t, res)

	resealPath := reflectionPath +
		"/" + reflectionID + "/seal"

	childUnlockAt := current.Add(24 * time.Hour)

	resealBody := `{"unlock_at":"` +
		childUnlockAt.Format(time.RFC3339Nano) + `"}`

	res = performRequest(
		router,
		"POST",
		resealPath,
		resealBody,
	)

	requireStatus(t, res, http.StatusCreated)

	var child struct {
		ID                 string `json:"id"`
		ParentCapsuleID    string `json:"parent_capsule_id"`
		SourceReflectionID string `json:"source_reflection_id"`
		Status             string `json:"status"`
	}

	if err := json.Unmarshal(
		res.Body.Bytes(),
		&child,
	); err != nil {
		t.Fatal(err)
	}

	if child.ID == "" ||
		child.ParentCapsuleID != parentID ||
		child.SourceReflectionID != reflectionID ||
		child.Status != "sealed" {
		t.Fatal("incorrect child capsule metadata")
	}

	res = performRequest(
		router,
		"GET",
		reflectionPath,
		"",
	)

	requireStatus(t, res, http.StatusOK)

	var reflections []struct {
		Body            string `json:"body"`
		Status          string `json:"status"`
		SealedCapsuleID string `json:"sealed_capsule_id"`
	}

	if err := json.Unmarshal(
		res.Body.Bytes(),
		&reflections,
	); err != nil {
		t.Fatal(err)
	}

	if len(reflections) != 1 {
		t.Fatalf("expected 1 reflection, got %d", len(reflections))
	}

	if reflections[0].Body != "" ||
		reflections[0].Status != "sealed" ||
		reflections[0].SealedCapsuleID != child.ID {
		t.Fatal("resealed reflection state is incorrect")
	}

	if strings.Contains(res.Body.String(), reflectionText) {
		t.Fatal("sealed reflection text leaked")
	}

	res = performRequest(
		router,
		"POST",
		resealPath,
		resealBody,
	)

	requireStatus(t, res, http.StatusConflict)

	res = performRequest(
		router,
		"POST",
		"/api/v1/capsules/"+child.ID+"/open",
		"",
	)

	requireStatus(t, res, http.StatusLocked)

	res = performRequest(
		router,
		"DELETE",
		"/api/v1/capsules/"+parentID,
		"",
	)
	requireStatus(t, res, http.StatusConflict)

	//a resealed child should not be deleted individually

	res = performRequest(
		router,
		"DELETE",
		"/api/v1/capsules/"+child.ID,
		"",
	)

	requireStatus(t, res, http.StatusConflict)

	if !strings.Contains(
		res.Body.String(),
		"linked child capsule",
	) {
		t.Fatalf(
			"expected linked-child conflict, got %s",
			res.Body.String(),
		)
	}
	res = performRequest(
		router,
		"GET",
		"/api/v1/capsules/"+child.ID,
		"",
	)

	current = current.Add(25 * time.Hour)

	res = performRequest(
		router,
		"POST",
		"/api/v1/capsules/"+child.ID+"/open",
		"",
	)

	requireStatus(t, res, http.StatusOK)

	var opened struct {
		Body string `json:"body"`
	}

	if err := json.Unmarshal(
		res.Body.Bytes(),
		&opened,
	); err != nil {
		t.Fatal(err)
	}

	if opened.Body != reflectionText {
		t.Fatal("child capsule lost the reflection text")
	}
}

func TestResealReflectionHTTPValidation(t *testing.T) {
	router, parentID := newOpenendCapsuleRouter(t)

	reflectionPath := "/api/v1/capsules/" +
		parentID + "/reflections"

	res := performRequest(
		router,
		"POST",
		reflectionPath,
		`{"body":"Keep this reflection safe."}`,
	)

	requireStatus(t, res, http.StatusCreated)

	reflectionID := requireID(t, res)

	path := reflectionPath +
		"/" + reflectionID + "/seal"

	tests := []struct {
		name   string
		body   string
		status int
	}{
		{
			name:   "missing unlock date",
			body:   `{}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "past unlock date",
			body:   `{"unlock_at":"2020-01-01T00:00:00Z"}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "invalid time",
			body:   `{"unlock_at":"tomorrow"}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "unknown field",
			body:   `{"unlock_at":"2099-01-01T00:00:00Z","extra":true}`,
			status: http.StatusBadRequest,
		},
		{
			name: "oversized request",
			body: `{"unlock_at":"2099-01-01T00:00:00Z","padding":"` +
				strings.Repeat("x", 4096) + `"}`,
			status: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := performRequest(
				router,
				"POST",
				path,
				tt.body,
			)

			requireStatus(t, res, tt.status)
		})
	}
	res = performRequest(
		router,
		"GET",
		reflectionPath,
		"",
	)
	requireStatus(t, res, http.StatusOK)

	if !strings.Contains(
		res.Body.String(),
		"Keep this reflection safe.",
	) {
		t.Fatal("invalid resealing lost the reflection")
	}
}

func TestResealReflectionHTTPNotFound(t *testing.T) {
	router, parentID := newOpenendCapsuleRouter(t)

	body := `{"unlock_at":"2099-01-01T00:00:00Z"}`

	tests := []struct {
		name string
		path string
	}{
		{
			name: "missing reflection",
			path: "/api/v1/capsules/" +
				parentID + "/reflections/missing/seal",
		},
		{
			name: "missing parent",
			path: "/api/v1/capsules/missing/reflections/" +
				"missing/seal",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := performRequest(
				router,
				"POST",
				tt.path,
				body,
			)
			requireStatus(t, res, http.StatusNotFound)

			if !strings.Contains(
				res.Body.String(),
				"capsule or reflection not found",
			) {
				t.Fatal("expected application not-found error")
			}
		})
	}
}
