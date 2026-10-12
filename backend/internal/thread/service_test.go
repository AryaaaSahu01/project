package thread

import (
	"errors"
	"journalapp/internal/capsule"
	"testing"
	"time"
)

func newFixture(
	t *testing.T,
) (*Service, *capsule.Service, *time.Time, string) {
	t.Helper()

	now := time.Date(
		2026, 10, 12, 0, 0, 0, 0, time.UTC,
	)
	capsules := capsule.NewService(
		capsule.NewMemoryRepository(),
		func() time.Time { return now },
	)

	threads := NewService(
		NewMemoryRepository(),
		capsules,
		func() time.Time { return now },
	)

	c, err := capsules.Seal("user-1", capsule.SealInput{
		SourceEntryID: "entry-1",
		Title:         "Dear future me",
		Body:          "Remember this moment.",
		UnlockAt:      now.Add(time.Hour),
	})

	if err != nil {
		t.Fatal(err)
	}

	return threads, capsules, &now, c.ID
}

func TestReflectionRequiresOpenedCapsule(t *testing.T) {
	threads, capsules, now, capsuleID := newFixture(t)

	input := CreateReflectionInput{
		Body: "Life has changed.",
	}

	_, err := threads.Create("user-1", capsuleID, input)

	if !errors.Is(err, ErrCapsuleNotReturned) {
		t.Fatalf("expected not returned, got %v", err)
	}

	*now = now.Add(2 * time.Hour)

	_, err = threads.Create("user-1", capsuleID, input)

	if !errors.Is(err, ErrCapsuleNotReturned) {
		t.Fatalf("unopened capsule accepted reflection: %v ", err)
	}

	_, err = capsules.Open("user-1", capsuleID)
	if err != nil {
		t.Fatal(err)
	}

	reflection, err := threads.Create(
		"user-1",
		capsuleID,
		input,
	)

	if err != nil {
		t.Fatal(err)
	}

	if reflection.Body != input.Body {
		t.Fatal("reflection content mismatch")
	}

}

func TestReflectionOwnership(t *testing.T) {
	threads, capsules, now, capsuleID := newFixture(t)

	*now = now.Add(2 * time.Hour)

	if _, err := capsules.Open("user-1", capsuleID); err != nil {
		t.Fatal(err)
	}

	_, err := threads.Create(
		"user-2",
		capsuleID,
		CreateReflectionInput{Body: "Unauthorized"},
	)
	if !errors.Is(err, capsule.ErrCapsuleNotFound) {
		t.Fatalf("expected capsule not found, got %v", err)
	}

	_, err = threads.List("user-2", capsuleID)

	if !errors.Is(err, capsule.ErrCapsuleNotFound) {
		t.Fatalf("expected capsule not foudn, got %v", err)
	}

}

func TestReflectionChronologicalOrder(t *testing.T) {
	threads, capsules, now, capsuleID := newFixture(t)

	*now = now.Add(2 * time.Hour)

	if _, err := capsules.Open("user-1", capsuleID); err != nil {
		t.Fatal(err)
	}

	_, err := threads.Create(
		"user-1",
		capsuleID,
		CreateReflectionInput{Body: "First reflection"},
	)

	if err != nil {
		t.Fatal(err)
	}

	*now = now.Add(time.Minute)

	_, err = threads.Create(
		"user-1",
		capsuleID,
		CreateReflectionInput{Body: "Second reflection"},
	)

	if err != nil {
		t.Fatal(err)
	}

	list, err := threads.List("user-1", capsuleID)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 2 {
		t.Fatalf("expected 2 reflection, got %d", len(list))
	}

	if list[0].Body != "First reflection" ||
		list[1].Body != "Second reflection" {
		t.Fatal("reflections are not chronological")
	}

}

func TestEmptyReflection(t *testing.T) {
	threads, capsules, now, capsuleID := newFixture(t)

	*now = now.Add(2 * time.Hour)

	if _, err := capsules.Open("user-1", capsuleID); err != nil {
		t.Fatal(err)
	}

	_, err := threads.Create(
		"user-1",
		capsuleID,
		CreateReflectionInput{Body: "  "},
	)

	if !errors.Is(err, ErrEmptyReflection) {
		t.Fatalf("expected empty reflection error, got %v", err)
	}
}
