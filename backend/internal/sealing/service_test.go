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

func TestSealInvalidDataKeepsJournal(t *testing.T) {
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
		Title: "Important Entry",
		Body:  "This writing must not disappear",
	})

	if err != nil {
		t.Fatal(err)
	}

	_, err = service.SealEntry(
		"user-1",
		entry.ID,
		current.Add(-time.Hour),
	)

	if !errors.Is(err, capsule.ErrInvalidUnlockAt) {
		t.Fatalf("expected invalid date, got %v", err)
	}

	saved, err := journals.Get("user-1", entry.ID)

	if err != nil {
		t.Fatalf("journal entry was lost: %v", err)
	}

	if saved.Body != entry.Body {
		t.Error("original journal content changed")
	}

	list, err := capsules.List("user-1")

	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 0 {
		t.Error("failed sealing should not create a capsule")
	}
}

func TestSealEntryOwnership(t *testing.T) {
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
		Title: "private writing",
		Body:  "only my account should access this",
	})

	if err != nil {
		t.Fatal(err)
	}

	_, err = service.SealEntry(
		"user-2",
		entry.ID,
		current.Add(24*time.Hour),
	)

	if !errors.Is(err, journal.ErrEntryNotFound) {
		t.Fatalf("expected entry not found, got %v", err)
	}

	_, err = journals.Get("user-1", entry.ID)
	if err != nil {
		t.Errorf("owner should retain the entry: %v", err)
	}

	list, err := capsules.List("user-2")

	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 0 {
		t.Error("unauthorized sealing created a capsule")
	}

}
