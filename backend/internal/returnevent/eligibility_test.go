package returnevent

import (
	"encoding/json"
	"errors"
	"journalapp/internal/capsule"
	"strings"
	"testing"
	"time"
)

func TestListEligibleFiltersOpenedAndDeleted(t *testing.T) {
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

	seal := func(userID, sourceID, body string) capsule.Capsule {
		t.Helper()

		c, err := capsules.Seal(userID, capsule.SealInput{
			SourceEntryID: sourceID,
			Body:          body,
			UnlockAt:      now.Add(time.Hour),
		})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}

	keep := seal("user-1", "entry-1", "SECRET-KEEP")
	opened := seal("user-1", "entry-2", "SECRET-OPENED")
	deleted := seal("user-1", "entry-3", "SECRET-DELETED")
	other := seal("user-2", "entry-4", "SECRET-OTHER")

	now = now.Add(2 * time.Hour)

	count, err := processor.ProcessDue("user-1")
	if err != nil || count != 3 {
		t.Fatalf("expected 3 created events: %d, %v", count, err)
	}

	count, err = processor.ProcessDue("user-2")
	if err != nil || count != 1 {
		t.Fatalf("expected other user's event: %d, %v", count, err)
	}

	eligible, err := processor.ListEligible("user-1")

	if err != nil || len(eligible) != 3 {
		t.Fatalf("expected 3 eligible events: %v", err)
	}

	if _, err := capsules.Open("user-1", opened.ID); err != nil {
		t.Fatal(err)
	}

	if err := capsules.Destroy("user-1", deleted.ID); err != nil {
		t.Fatal(err)
	}

	eligible, err = processor.ListEligible("user-1")
	if err != nil {
		t.Fatal(err)
	}

	if len(eligible) != 1 || eligible[0].CapsuleID != keep.ID {
		t.Fatal("opened or deleted capsule remained eligible")
	}

	//historical records are still present
	stored, err := repo.ListByUser("user-1")
	if err != nil || len(stored) != 3 {
		t.Fatal("event history was unexpectedly modified")
	}

	encoded, err := json.Marshal(eligible)

	if err != nil {
		t.Fatal(err)
	}

	for _, secret := range []string{
		"SECRET-KEEP",
		"SECRET-OPENED",
		"SECRET-DELETED",
		"SECRET-OTHER",
	} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("private content leaked: %s", secret)
		}
	}

	metadata, err := capsules.GetMetadata("user-1", keep.ID)

	if err != nil {
		t.Fatal(err)
	}

	if metadata.Status != capsule.StatusSealed {
		t.Fatal("eligibility check opened the capsule")
	}

	otherEligible, err := processor.ListEligible("user-2")

	if err != nil || len(otherEligible) != 1 ||
		otherEligible[0].CapsuleID != other.ID {
		t.Fatal("cross-user eligibility isolation failed")
	}
}

func TestListEligibleHandlesInvalidUserAndMissingCapsule(t *testing.T) {
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

	_, err := processor.ListEligible("  ")
	if !errors.Is(err, capsule.ErrInvalidUser) {
		t.Fatalf("expected invalid user, got %v", err)
	}

	_, err = repo.CreateIfAbsent(Event{
		ID:         "capsule_due:missing",
		UserID:     "user-1",
		CapsuleID:  "missing",
		Type:       TypeCapsuleDue,
		UnlockAt:   now,
		RecordedAt: now,
	})

	if err != nil {
		t.Fatal(err)
	}

	eligible, err := processor.ListEligible("user-1")
	if err != nil || len(eligible) != 0 {
		t.Fatal("missing capsule remained eligible")
	}
}
