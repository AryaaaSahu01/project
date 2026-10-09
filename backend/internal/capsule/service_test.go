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

func TestOpenCapsule(t *testing.T) {
	current := time.Date(
		2026, 10, 9, 0, 0, 0, 0, time.UTC,
	)

	repo := NewMemoryRepository()

	service := NewService(repo, func() time.Time {
		return current
	})

	input := SealInput{
		SourceEntryID: "entry-123",
		Title:         "Dear Future me",
		Body:          "Remember this moment.",
		UnlockAt:      current.Add(24 * time.Hour),
	}

	c, err := service.Seal("user-1", input)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Open("user-1", c.ID)

	if !errors.Is(err, ErrCapsuleLocked) {
		t.Fatalf("expected locked error, got %v", err)
	}

	current = current.Add(25 * time.Hour)

	opened, err := service.Open("user-1", c.ID)

	if err != nil {
		t.Fatal(err)
	}

	if opened.Title != input.Title {
		t.Error("incorrect opened title")
	}

	if opened.Body != input.Body {
		t.Error("incorrect opened body")
	}

	saved, err := repo.GetByID("user-1", c.ID)
	if err != nil {
		t.Fatal(err)
	}

	if saved.Status != StatusReturned {
		t.Errorf("expected returned, got %s", saved.Status)
	}
}

func TestOpenAnotherUsersCapsule(t *testing.T) {
	current := time.Date(
		2026, 10, 9, 0, 0, 0, 0, time.UTC,
	)

	repo := NewMemoryRepository()

	service := NewService(repo, func() time.Time {
		return current
	})

	c, err := service.Seal("user-1", SealInput{
		SourceEntryID: "entry-123",
		Title:         "Private Capsule",
		UnlockAt:      current.Add(time.Hour),
	})

	if err != nil {
		t.Fatal(err)
	}

	current = current.Add(2 * time.Hour)

	_, err = service.Open("user-2", c.ID)

	if !errors.Is(err, ErrCapsuleNotFound) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestGetMetaData(t *testing.T) {
	current := time.Date(
		2026, 10, 9, 0, 0, 0, 0, time.UTC,
	)

	repo := NewMemoryRepository()

	service := NewService(repo, func() time.Time {
		return current
	})

	c, err := service.Seal("user-1", SealInput{
		SourceEntryID: "entry-123",
		Title:         "Private title",
		Body:          "Private content",
		UnlockAt:      current.Add(24 * time.Hour),
	})

	if err != nil {
		t.Fatal(err)
	}

	metadata, err := service.GetMetadata("user-1", c.ID)

	if err != nil {
		t.Fatal(err)
	}

	if metadata.CanOpen {
		t.Error("capsule should still be locked")
	}

	if metadata.Status != StatusSealed {
		t.Error("expected sealed status")
	}

	current = current.Add(25 * time.Hour)

	metadata, err = service.GetMetadata("user-1", c.ID)

	if err != nil {
		t.Fatal(err)
	}

	if !metadata.CanOpen {
		t.Error("capsule should now be available")
	}

	if metadata.Status != StatusSealed {
		t.Error("Metadata lookup should not change status")
	}
}

func TestGetMetadataOwnership(t *testing.T) {
	current := time.Date(
		2026, 10, 9, 0, 0, 0, 0, time.UTC,
	)

	repo := NewMemoryRepository()

	service := NewService(repo, func() time.Time {
		return current
	})

	c, err := service.Seal("user-1", SealInput{
		SourceEntryID: "entry-123",
		Title:         "Private Capsule",
		UnlockAt:      current.Add(24 * time.Hour),
	})

	if err != nil {
		t.Fatal(err)
	}

	_, err = service.GetMetadata("user-2", c.ID)

	if !errors.Is(err, ErrCapsuleNotFound) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestDestroyCapsuleOwnership(t *testing.T) {
	current := time.Date(
		2026, 10, 9, 0, 0, 0, 0, time.UTC,
	)

	repo := NewMemoryRepository()

	service := NewService(repo, func() time.Time {
		return current
	})

	c, err := service.Seal("user-1", SealInput{
		SourceEntryID: "entry-123",
		Title:         "Private capsule",
		UnlockAt:      current.Add(24 * time.Hour),
	})

	if err != nil {
		t.Fatal(err)
	}

	err = service.Destroy("user-2", c.ID)

	if !errors.Is(err, ErrCapsuleNotFound) {
		t.Errorf("Expected not found, got %v", err)
	}

	_, err = service.GetMetadata("user-1", c.ID)

	if err != nil {
		t.Errorf("owner's capsule should still exist: %v", err)
	}
}

func TestDestroyCapsule(t *testing.T) {
	current := time.Date(
		2026, 10, 9, 0, 0, 0, 0, time.UTC,
	)

	repo := NewMemoryRepository()

	service := NewService(repo, func() time.Time {
		return current
	})

	c, err := service.Seal("user-1", SealInput{
		SourceEntryID: "entry-123",
		Title:         "Destroy me",
		Body:          "this content should disappear.",
		UnlockAt:      current.Add(24 * time.Hour),
	})

	if err != nil {
		t.Fatal(err)
	}

	err = service.Destroy("user-1", c.ID)

	if err != nil {
		t.Fatal(err)
	}

	_, err = service.GetMetadata("user-1", c.ID)

	if !errors.Is(err, ErrCapsuleNotFound) {
		t.Errorf("expected not found, got %v", err)
	}

	_, err = service.Open("user-1", c.ID)

	if !errors.Is(err, ErrCapsuleNotFound) {
		t.Errorf("destroyed capsule should not open,got %v", err)
	}

	err = service.Destroy("user-1", c.ID)

	if !errors.Is(err, ErrCapsuleNotFound) {
		t.Errorf("second destroy should return not found, got %v", err)
	}
}

func TestListCapsules(t *testing.T) {
	current := time.Date(
		2026, 10, 9, 0, 0, 0, 0, time.UTC,
	)

	repo := NewMemoryRepository()

	service := NewService(repo, func() time.Time {
		return current
	})
	first, err := service.Seal("user-1", SealInput{
		SourceEntryID: "entry-1",
		Title:         "First Capsule",
		UnlockAt:      current.Add(24 * time.Hour),
	})

	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Seal("user-2", SealInput{
		SourceEntryID: "entry-2",
		Title:         "someone else's capsule",
		UnlockAt:      current.Add(48 * time.Hour),
	})

	if err != nil {
		t.Fatal(err)
	}

	capsules, err := service.List("user-1")

	if err != nil {
		t.Fatal(err)
	}

	if len(capsules) != 1 {
		t.Fatalf("expected 1 capsule, got %d", len(capsules))
	}

	if capsules[0].ID != first.ID {
		t.Errorf("returned the wrong capsule")
	}

	if capsules[0].CanOpen {
		t.Errorf("capsule should still be locked")
	}

	current = current.Add(25 * time.Hour)

	capsules, err = service.List("user-1")

	if err != nil {
		t.Fatal(err)
	}

	if !capsules[0].CanOpen {
		t.Error("capsule should now be available")
	}
}
