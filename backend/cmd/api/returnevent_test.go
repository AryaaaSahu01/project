package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

const processDuePath = "/api/v1/dev/return-events/process-due"
const eligibleEventsPath = "/api/v1/return-events/eligible"

func createReturnEventCapsule(
	t *testing.T,
	router http.Handler,
	unlockAt time.Time,
	body string,
) string {
	t.Helper()

	entryJSON, err := json.Marshal(map[string]string{
		"title": "return event test",
		"body":  body,
	})

	if err != nil {
		t.Fatal(err)
	}

	res := performRequest(
		router, "POST", "/api/v1/entries",
		string(entryJSON),
	)

	requireStatus(t, res, http.StatusCreated)
	entryID := requireID(t, res)

	sealJSON, err := json.Marshal(map[string]string{
		"unlock_at": unlockAt.Format(time.RFC3339Nano),
	})

	if err != nil {
		t.Fatal(err)
	}

	res = performRequest(
		router,
		"POST",
		"/api/v1/entries/"+entryID+"/seal",
		string(sealJSON),
	)

	requireStatus(t, res, http.StatusCreated)
	return requireID(t, res)

}

func TestReturnEventHTTPFlow(t *testing.T) {
	current := time.Date(
		2026, 10, 19, 0, 0, 0, 0, time.UTC,
	)

	router := newMux(func() time.Time {
		return current
	})

	firstID := createReturnEventCapsule(
		t, router,
		current.Add(time.Hour),
		"PRIVATE-FIRST",
	)

	secondID := createReturnEventCapsule(
		t, router,
		current.Add(4*time.Hour),
		"PRIVATE-SECOND",
	)

	res := performRequest(
		router, "GET", eligibleEventsPath, "",
	)

	requireStatus(t, res, http.StatusOK)

	var events []struct {
		CapsuleID string `json:"capsule_id"`
		Type      string `json:"type"`
	}

	if err := json.Unmarshal(res.Body.Bytes(), &events); err != nil {
		t.Fatal(err)
	}

	if len(events) != 0 {
		t.Fatal("events appeared before processing")
	}

	res = performRequest(
		router, "POST", processDuePath, "",
	)
	requireStatus(t, res, http.StatusOK)

	var result struct {
		Created int `json:"created"`
	}

	if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	if result.Created != 0 {
		t.Fatal("premature event was created")
	}

	current = current.Add(2 * time.Hour)

	res = performRequest(
		router, "POST", processDuePath, "",
	)

	requireStatus(t, res, http.StatusOK)

	if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	if result.Created != 1 {
		t.Fatalf("expected one event, got %d", result.Created)
	}

	res = performRequest(
		router, "POST", processDuePath, "",
	)

	requireStatus(t, res, http.StatusOK)

	if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	if result.Created != 0 {
		t.Fatal("duplicated event created")
	}

	res = performRequest(
		router, "GET", eligibleEventsPath, "",
	)

	requireStatus(t, res, http.StatusOK)

	if err := json.Unmarshal(res.Body.Bytes(), &events); err != nil {
		t.Fatal(err)
	}

	if len(events) != 1 ||
		events[0].CapsuleID != firstID ||
		events[0].Type != "capsule_due" {
		t.Fatal("incorrect eligible return events")
	}

	for _, secret := range []string{
		"PRIVATE-FIRST",
		"PRIVATE-SECOND",
		"local-user",
	} {
		if strings.Contains(res.Body.String(), secret) {
			t.Fatalf("response leaked information: %s", secret)
		}
	}

	res = performRequest(
		router,
		"POST",
		"/api/v1/capsules/"+firstID+"/open",
		"",
	)

	requireStatus(t, res, http.StatusOK)

	res = performRequest(
		router, "GET", eligibleEventsPath, "",
	)
	requireStatus(t, res, http.StatusOK)

	if err := json.Unmarshal(res.Body.Bytes(), &events); err != nil {
		t.Fatal(err)
	}

	if len(events) != 0 {
		t.Fatal("opened capsule still eligible")
	}

	current = current.Add(3 * time.Hour)

	res = performRequest(
		router, "POST", processDuePath, "",
	)
	requireStatus(t, res, http.StatusOK)

	if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	if result.Created != 1 {
		t.Fatal("second capsule event not created")
	}

	res = performRequest(
		router,
		"DELETE",
		"/api/v1/capsules/"+secondID,
		"",
	)

	requireStatus(t, res, http.StatusNoContent)

	res = performRequest(
		router, "GET", eligibleEventsPath, "",
	)
	requireStatus(t, res, http.StatusOK)

	if err := json.Unmarshal(res.Body.Bytes(), &events); err != nil {
		t.Fatal(err)
	}

	if len(events) != 0 {
		t.Fatal("deleted capsule remained eligible")
	}
}

func TestReturnEventHTTPMethodRestrictions(t *testing.T) {
	router := newMux(time.Now)

	res := performRequest(
		router, "GET", processDuePath, "",
	)

	requireStatus(t, res, http.StatusMethodNotAllowed)

	res = performRequest(
		router, "POST", eligibleEventsPath, "",
	)

	requireStatus(t, res, http.StatusMethodNotAllowed)
}
