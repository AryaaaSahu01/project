package sealing

import (
	"errors"
	"journalapp/internal/capsule"
	"journalapp/internal/journal"
	"testing"
	"time"
)

func TestSealEntry(t *testing.T) {
	current := time.Date(
		2026, 10, 9, 0, 0, 0, 0, time.UTC,
	)

	journalRepo := journal.NewMemoryRepository()
	journals := journal.NewService(journalRepo)

	capsuleRepo := capsule.NewMemoryRepository()
	capsules := capsule.NewService(
		capsuleRepo,
		func() time.Time { return current },
	)

	service := NewService(journals, capsules)

	entry, err := journals.Create("user-1", journal.CreateEntryInput{
		Title: "Dear Future me",
		Body:  "Remember this day.",
	})
	if err != nil {
		t.Fatal(err)
	}

	metadata, err := service.SealEntry(
		"user-1",
		entry.ID,
		current.Add(24*time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}

	if metadata.SourceEntryID != entry.ID {
		t.Error("incorrect source entry")
	}

	_, err = journals.Get("user-1", entry.ID)
	if !errors.Is(err, journal.ErrEntryNotFound) {
		t.Errorf("original entry should be gone: %v", err)
	}
	_, err = capsules.Open("user-1", metadata.ID)
	if !errors.Is(err, capsule.ErrCapsuleLocked) {
		t.Errorf("capsule should be locked: %v", err)
	}

	current = current.Add(25 * time.Hour)

	opened, err := capsules.Open("user-1", metadata.ID)
	if err != nil {
		t.Fatal(err)
	}

	if opened.Body != entry.Body {
		t.Error("original writing was not preserved")
	}
}
