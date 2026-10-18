package returnevent

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"journalapp/internal/capsule"
)

func TestProcessDueIdempotenceAndPrivacy(t *testing.T) {
	now := time.Date(
		2026, 10, 18, 0, 0, 0, 0, time.UTC,
	)
	capsules := capsule.NewService(
		capsule.NewMemoryRepository(),
		func() time.Time { return now },
	)

	repo := NewMemoryRepository()

	processor := NewProcessor(
		capsules,
		repo,
		func() time.Time { return now },
	)

	seal := func(
		userID, sourceID, body string,
		delay time.Duration,
	) capsule.Capsule {
		t.Helper()

		c, err := capsules.Seal(userID, capsule.SealInput{
			SourceEntryID: sourceID,
			Body:          body,
			UnlockAt:      now.Add(delay),
		})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}

	first := seal(
		"user-1", "entry-1", "SECRET-FRIST", time.Hour,
	)
	second := seal(
		"user-1", "entry-2", "SECRET-SECOND", 2*time.Hour,
	)
	future := seal(
		"user-1", "entry-3", "SECRET-FUTURE", 4*time.Hour,
	)
	opened := seal(
		"user-1", "entry-4", "SECRET-OPENED", 30*time.Minute,
	)
	other := seal(
		"user-2", "entry-5", "SECRET-OTHER", time.Hour,
	)

	count, err := processor.ProcessDue("user-1")
	if err != nil || count != 0 {
		t.Fatalf("expected no events before unlock: %d, %v", count, err)
	}

	now = now.Add(3 * time.Hour)

	if _, err := capsules.Open("user-1", opened.ID); err != nil {
		t.Fatal(err)
	}

	count, err = processor.ProcessDue("user-1")
	if err != nil || count != 2 {
		t.Fatalf("expected two new events: %d, %v", count, err)
	}

	for i := 0; i < 100; i++ {
		count, err = processor.ProcessDue("user-1")
		if err != nil || count != 0 {
			t.Fatalf("duplicate processing at attempt %d: %v", i, err)
		}
	}

	events, err := repo.ListByUser("user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 ||
		events[0].CapsuleID != first.ID ||
		events[1].CapsuleID != second.ID {
		t.Fatal("incorrect event count or ordering")
	}

	for _, event := range events {
		if event.Type != TypeCapsuleDue {
			t.Fatal("incorrect event type")
		}
	}

	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}

	for _, secret := range []string{
		"SECRET-FIRST",
		"SECRET-SECOND",
		"SECRET-FUTURE",
		"SECRET-OPENED",
		"SECRET-OTHER",
	} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("event leaked private writing: %s", secret)
		}
	}

	metadata, err := capsules.GetMetadata("user-1", first.ID)
	if err != nil {
		t.Fatal(err)
	}

	if metadata.Status != capsule.StatusSealed {
		t.Fatal("event processing opened a capsule")
	}

	count, err = processor.ProcessDue("user-2")
	if err != nil || count != 1 {
		t.Fatal("other user's due event was not created")
	}

	otherEvents, err := repo.ListByUser("user-2")
	if err != nil || len(otherEvents) != 1 ||
		otherEvents[0].CapsuleID != other.ID {
		t.Fatal("user isolation failed")
	}

	now = now.Add(2 * time.Hour)

	count, err = processor.ProcessDue("user-1")
	if err != nil || count != 1 {
		t.Fatal("future capsule did not create its event")
	}

	events, err = repo.ListByUser("user-1")
	if err != nil || len(events) != 3 ||
		events[2].CapsuleID != future.ID {
		t.Fatal("future event or fnal count incorrect")
	}
}

func TestProcessDueConcurrentIdempotent(t *testing.T) {
	now := time.Date(
		2026, 10, 18, 0, 0, 0, 0, time.UTC,
	)

	capsules := capsule.NewService(
		capsule.NewMemoryRepository(),
		func() time.Time { return now },
	)

	c, err := capsules.Seal("user-1", capsule.SealInput{
		SourceEntryID: "entry-1",
		Body:          "Keep this private",
		UnlockAt:      now.Add(time.Hour),
	})

	if err != nil {
		t.Fatal(err)
	}

	now = now.Add(2 * time.Hour)

	repo := NewMemoryRepository()
	processor := NewProcessor(
		capsules,
		repo,
		func() time.Time { return now },
	)

	const workers = 50

	type result struct {
		count int
		err   error
	}

	results := make(chan result, workers)

	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			count, err := processor.ProcessDue("user-1")
			results <- result{count: count, err: err}
		}()
	}

	wg.Wait()
	close(results)

	totalCreated := 0

	for r := range results {
		if r.err != nil {
			t.Fatal(r.err)
		}
		totalCreated += r.count
	}

	if totalCreated != 1 {
		t.Fatalf("expected one insertion, got %d", totalCreated)
	}

	events, err := repo.ListByUser("user-1")
	if err != nil || len(events) != 1 ||
		events[0].CapsuleID != c.ID {
		t.Fatal("concurrent processing created duplicated")
	}
}
func TestProcessDueInvalidUserAndDeletedCapsule(t *testing.T) {
	now := time.Date(
		2026, 10, 18, 0, 0, 0, 0, time.UTC,
	)

	capsules := capsule.NewService(
		capsule.NewMemoryRepository(),
		func() time.Time { return now },
	)

	repo := NewMemoryRepository()
	processor := NewProcessor(
		capsules,
		repo,
		func() time.Time { return now },
	)

	_, err := processor.ProcessDue("  ")
	if !errors.Is(err, capsule.ErrInvalidUser) {
		t.Fatalf("expected invalid user error, got %v", err)
	}

	c, err := capsules.Seal("user-1", capsule.SealInput{
		SourceEntryID: "entry-delete",
		Body:          "Deleted memory",
		UnlockAt:      now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	now = now.Add(2 * time.Hour)

	if err := capsules.Destroy("user-1", c.ID); err != nil {
		t.Fatal(err)
	}

	count, err := processor.ProcessDue("user-1")
	if err != nil || count != 0 {
		t.Fatal("deleted capsule generated a return event")
	}

	events, err := repo.ListByUser("user-1")
	if err != nil || len(events) != 0 {
		t.Fatal("deleted capsule left an unexpected event")
	}
}
