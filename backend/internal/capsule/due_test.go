package capsule

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestListDueFiltersAndPreservesPrivacy(t *testing.T) {
	now := time.Date(
		2026, 10, 18, 0, 0, 0, 0, time.UTC,
	)

	service := NewService(
		NewMemoryRepository(),
		func() time.Time { return now },
	)

	seal := func(
		userID, sourceID, body string,
		dealy time.Duration,
	) Capsule {
		t.Helper()

		c, err := service.Seal(userID, SealInput{
			SourceEntryID: sourceID,
			Body:          body,
			UnlockAt:      now.Add(dealy),
		})

		if err != nil {
			t.Fatal(err)
		}

		return c
	}

	first := seal(
		"user-1", "entry-1",
		"PRIVATE-FIRST", time.Hour,
	)

	second := seal(
		"user-1", "entry-2",
		"PRIVATE-SECOND", 2*time.Hour,
	)

	future := seal(
		"user-1", "entry-3",
		"PRIVATE-FUTURE", 3*time.Hour,
	)

	opened := seal(
		"user-1", "entry-4",
		"Previously opened", 30*time.Minute,
	)

	other := seal(
		"user-2", "entry-5",
		"OTHER-USER-SECRET", time.Hour,
	)

	due, err := service.ListDue("user-1")
	if err != nil || len(due) != 0 {
		t.Fatalf("expected no due capsules: %v", err)
	}

	now = now.Add(2 * time.Hour)

	if _, err := service.Open("user-1", opened.ID); err != nil {
		t.Fatal(err)
	}

	due, err = service.ListDue("user-1")

	if err != nil {
		t.Fatal(err)
	}

	if len(due) != 2 {
		t.Fatalf("expected 2 due capsules, got %d", len(due))
	}

	if due[0].ID != first.ID || due[1].ID != second.ID {
		t.Fatal("due capsules are incorrectly ordered")
	}

	for _, c := range due {
		if c.Status != StatusSealed || !c.CanOpen {
			t.Fatal("due detection changed capsule state")
		}
	}

	encoded, err := json.Marshal(due)
	if err != nil {
		t.Fatal(err)
	}

	for _, secret := range []string{
		"PRIVATE-FIRST",
		"PRIVATE-SECOND",
		"PRIVATE-FUTURE",
		"OTHER-USER-SECRET",
	} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("private writing leaked: %s", secret)
		}
	}

	metadata, err := service.GetMetadata("user-1", first.ID)
	if err != nil {
		t.Fatal(err)
	}

	if metadata.Status != StatusSealed {
		t.Fatal("due detection opened a capsule")
	}

	if _, err := service.Open("user-1", future.ID); !errors.Is(
		err, ErrCapsuleLocked,
	) {
		t.Fatalf("expected locked capsule, got %v", err)
	}

	otherDue, err := service.ListDue("user-2")

	if err != nil {
		t.Fatal(err)
	}

	if len(otherDue) != 1 || otherDue[0].ID != other.ID {
		t.Fatal("cross-user due-capsule filtering failed")
	}

	now = now.Add(2 * time.Hour)

	due, err = service.ListDue("user-1")
	if err != nil || len(due) != 3 {
		t.Fatalf("expected 3 due sealed capsules: %v", err)
	}
}

func TestListDueUnlockBoundaryAndInvalidUser(t *testing.T) {
	now := time.Date(
		2026, 10, 18, 0, 0, 0, 0, time.UTC,
	)

	service := NewService(
		NewMemoryRepository(),
		func() time.Time { return now },
	)

	c, err := service.Seal("user-1", SealInput{
		SourceEntryID: "entry-boundary",
		Body:          "Time boundary test",
		UnlockAt:      now.Add(time.Hour),
	})

	if err != nil {
		t.Fatal(err)
	}

	now = now.Add(time.Hour - time.Nanosecond)

	due, err := service.ListDue("user-1")
	if err != nil || len(due) != 0 {
		t.Fatal("capsule became due too early")
	}

	now = now.Add(time.Nanosecond)

	due, err = service.ListDue("user-1")
	if err != nil {
		t.Fatal(err)
	}

	if len(due) != 1 || due[0].ID != c.ID {
		t.Fatal("capsule was not due at its unlock timestamp")
	}

	_, err = service.ListDue("  ")
	if !errors.Is(err, ErrInvalidUser) {
		t.Fatalf("expected invalid user error, got %v", err)
	}
}
