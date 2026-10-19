package returnevent

import (
	"encoding/json"
	"errors"
	"journalapp/internal/capsule"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type failingEventRepository struct {
	Repository
	readErr  error
	writeErr error
}

func (r *failingEventRepository) ListByUser(
	userID string,
) ([]Event, error) {
	if r.readErr != nil {
		return nil, r.readErr
	}

	return (r.Repository.ListByUser(userID))
}

func (r *failingEventRepository) CreateIfAbsent(
	event Event,
) (bool, error) {
	if r.writeErr != nil {
		return false, r.writeErr
	}
	return r.Repository.CreateIfAbsent(event)
}

func requireSafeInternalError(
	t *testing.T,
	mux http.Handler,
	method string,
	path string,
	secret string,
) {
	t.Helper()

	request := httptest.NewRequest(method, path, nil)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf(
			"expected 500, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	if !strings.Contains(
		recorder.Header().Get("Content-Type"),
		"application/json",
	) {
		t.Fatal("error response must use JSON")
	}

	var response struct {
		Error string `json:"error"`
	}

	if err := json.Unmarshal(
		recorder.Body.Bytes(), &response,
	); err != nil {
		t.Fatal(err)
	}

	if response.Error != "internal server error" {
		t.Fatalf(
			"unexpected public error: %q",
			response.Error,
		)
	}

	if strings.Contains(recorder.Body.String(), secret) {
		t.Fatal("internal information leaked in response")
	}
}

func TestEligibleHTTPReposiotryReadFailure(t *testing.T) {
	now := time.Date(
		2026, 10, 19, 0, 0, 0, 0, time.UTC,
	)

	capsules := capsule.NewService(
		capsule.NewMemoryRepository(),
		func() time.Time { return now },
	)

	secret := "SECRET-INTERNAL-STORAGE-DETAIL"

	repository := &failingEventRepository{
		Repository: NewMemoryRepository(),
		readErr:    errors.New(secret),
	}

	processor := NewProcessor(
		capsules,
		repository,
		func() time.Time { return now },
	)

	mux := http.NewServeMux()
	NewHandler(processor).RegisterRoutes(mux)

	requireSafeInternalError(
		t,
		mux,
		http.MethodGet,
		"/api/v1/return-events/eligible",
		secret,
	)
}

func TestProcessDueHTTPRepositoryWriteFailure(t *testing.T) {
	now := time.Date(
		2026, 10, 19, 0, 0, 0, 0, time.UTC,
	)

	capsules := capsule.NewService(
		capsule.NewMemoryRepository(),
		func() time.Time { return now },
	)

	c, err := capsules.Seal(
		"local-user",
		capsule.SealInput{
			SourceEntryID: "entry-1",
			Body:          "PRIVATE-CAPSULE-CONTENT",
			UnlockAt:      now.Add(time.Hour),
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	now = now.Add(2 * time.Hour)

	secret := "SECRET-EVENT-WRITE-FAILURE"
	baseRepository := NewMemoryRepository()

	repository := &failingEventRepository{
		Repository: baseRepository,
		writeErr:   errors.New(secret),
	}

	processor := NewProcessor(
		capsules,
		repository,
		func() time.Time { return now },
	)

	mux := http.NewServeMux()
	NewHandler(processor).RegisterDevRoutes(mux)

	requireSafeInternalError(
		t,
		mux,
		http.MethodPost,
		"/api/v1/dev/return-events/process-due",
		secret,
	)

	events, err := baseRepository.ListByUser("local-user")
	if err != nil {
		t.Fatal(err)
	}

	if len(events) != 0 {
		t.Fatal("failed write created an event")
	}

	metadata, err := capsules.GetMetadata(
		"local-user",
		c.ID,
	)

	if err != nil {
		t.Fatal(err)
	}

	if metadata.Status != capsule.StatusSealed {
		t.Fatal("failed processing changed capsule status")
	}
}
