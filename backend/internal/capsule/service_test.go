package capsule

import (
	"errors"
	"testing"
	"time"
)

func TestSealCapsule(t *testing.T) {
	fixedTime := time.Date(
		2026, 10, 9, 0, 0, 0, 0, time.UTC,
	)

	repo := NewMemoryRepository()

	service := NewService(repo, func() time.Time {
		return fixedTime
	})

	input := SealInput{
		SourceEntryID: "entry-123",
		Title:         "Dear future me",
		Body:          "I hope you are doing well",
		UnlockAt:      fixedTime.Add(24 * time.Hour),
	}

	c, err := service.Seal("user-1", input)

	if err != nil {
		t.Fatalf("unexpected error : %v", err)

	}

	if c.ID == "" {
		t.Error("expected a generate capsule ID")
	}

	if c.Status != StatusSealed {
		t.Errorf("expected sealed, got %s", c.Status)
	}

	if !c.UnlockAt.Equal(input.UnlockAt) {
		t.Errorf("incorrect unlock date")
	}

	if c.title != input.Title || c.body != input.Body {
		t.Errorf("snapshot content does not match")
	}

	saved, err := repo.GetByID("user-1", c.ID)

	if err != nil {
		t.Fatal(err)
	}

	if saved.ID != c.ID {
		t.Error("capsule was not stored correctly")
	}
}

func TestSealRejectsPastDate(t *testing.T) {
	fixedTime := time.Date(
		2026, 10, 9, 0, 0, 0, 0, time.UTC,
	)

	repo := NewMemoryRepository()
	service := NewService(repo, func() time.Time {
		return fixedTime
	})

	input := SealInput{
		SourceEntryID: "entry-123",
		Title:         "Past Capsule",
		Body:          "This should fail.",
		UnlockAt:      fixedTime.Add(-24 * time.Hour),
	}
	_, err := service.Seal("user-1", input)

	if !errors.Is(err, ErrInvalidUnlockAt) {
		t.Errorf("expected ErrInvalidUnlockAt, got %v", err)
	}
}
