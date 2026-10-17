package main

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

const deleteConfirmationJSON = `{"confirmation":"DELETE CONVERSATION"}`

func createChildForDeleteTest(
	t *testing.T,
	router http.Handler,
	parentID string,
) string {
	t.Helper()

	path := "/api/v1/capsules/" + parentID + "/reflections"

	res := performRequest(
		router,
		"POST",
		path,
		`{"body":"A Private child memory"}`,
	)

	requireStatus(t, res, http.StatusCreated)

	reflectionID := requireID(t, res)

	res = performRequest(
		router,
		"POST",
		path+"/"+reflectionID+"/seal",
		`{"unlock_at":"2099-01-01T00:00:00Z"}`,
	)

	requireStatus(t, res, http.StatusCreated)

	return requireID(t, res)

}

func TestDeleteConversationHTTPFlow(t *testing.T) {
	router, rootID := newOpenendCapsuleRouter(t)

	childID := createChildForDeleteTest(t, router, rootID)

	res := performRequest(
		router,
		"POST",
		"/api/v1/entries",
		`{"title":"Unrelated","body":"Keep this"}`,
	)

	requireStatus(t, res, http.StatusCreated)

	entryID := requireID(t, res)

	res = performRequest(
		router,
		"POST",
		"/api/v1/entries/"+entryID+"/seal",
		`{"unlock_at":"2099-01-01T00:00:00Z"}`,
	)

	requireStatus(t, res, http.StatusCreated)
	unrelatedID := requireID(t, res)

	deletePath := "/api/v1/capsules/" + rootID + "/history"

	res = performRequest(
		router, "DELETE", deletePath, deleteConfirmationJSON,
	)

	requireStatus(t, res, http.StatusNoContent)

	if res.Body.Len() != 0 {
		t.Fatal("204 response must have no body")
	}

	for _, id := range []string{rootID, childID} {
		res = performRequest(
			router,
			"GET",
			"/api/v1/capsules/"+id,
			"",
		)
		requireStatus(t, res, http.StatusNotFound)
	}

	res = performRequest(router, "GET", deletePath, "")
	requireStatus(t, res, http.StatusNotFound)

	//unrelated capsule must survive

	res = performRequest(
		router,
		"GET",
		"/api/v1/capsules/"+unrelatedID,
		"",
	)

	requireStatus(t, res, http.StatusOK)

	res = performRequest(
		router, "DELETE", deletePath, deleteConfirmationJSON,
	)

	requireStatus(t, res, http.StatusNotFound)

}

func TestDeleteConversationHTTPValidation(t *testing.T) {
	router, rootID := newOpenendCapsuleRouter(t)

	path := "/api/v1/capsules/" + rootID + "/history"

	tests := []struct {
		name   string
		body   string
		status int
	}{
		{
			name:   "missing confirmation",
			body:   `{}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "incorrect confirmation",
			body:   `{"confirmation":"DELETE"}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "malformed JSON",
			body:   `{"confirmation":`,
			status: http.StatusBadRequest,
		},
		{
			name: "unknown field",
			body: `{"confirmation":"DELETE CONVERSATION",` +
				`"unexpected":true}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "multiple JSON documents",
			body:   deleteConfirmationJSON + `{}`,
			status: http.StatusBadRequest,
		},
		{
			name: "oversized request",
			body: `{"confirmation":"DELETE CONVERSATION",` +
				`"padding":"` + strings.Repeat("x", 4096) + `"}`,
			status: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := performRequest(
				router, "DELETE", path, tt.body,
			)
			requireStatus(t, res, tt.status)
		})
	}

	res := performRequest(router, "DELETE", path, "")
	requireStatus(t, res, http.StatusUnsupportedMediaType)

	//every rejected request must preserve the capsule.
	res = performRequest(
		router,
		"GET",
		"/api/v1/capsules/"+rootID,
		"",
	)
	requireStatus(t, res, http.StatusOK)
}

func TestDeleteConversationHTTPNonRoot(t *testing.T) {
	router, rootID := newOpenendCapsuleRouter(t)
	childID := createChildForDeleteTest(t, router, rootID)

	res := performRequest(
		router,
		"DELETE",
		"/api/v1/capsules/"+childID+"/history",
		deleteConfirmationJSON,
	)

	requireStatus(t, res, http.StatusConflict)

	for _, id := range []string{rootID, childID} {
		res = performRequest(
			router,
			"GET",
			"/api/v1/capsules/"+id,
			"",
		)
		requireStatus(t, res, http.StatusOK)
	}

}

func TestDeleteConversationHTTPMissing(t *testing.T) {
	router := newMux(time.Now)

	res := performRequest(
		router,
		"DELETE",
		"/api/v1/capsules/missing/history",
		deleteConfirmationJSON,
	)

	requireStatus(t, res, http.StatusNotFound)

	if !strings.Contains(res.Body.String(), "capsule not found") {
		t.Fatal("expected application not-found response")
	}
}
