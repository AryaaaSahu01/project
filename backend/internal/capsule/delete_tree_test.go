package capsule

import (
	"errors"
	"testing"
	"time"
)

func TestDeleteTreeRejectsStaleIDs(t *testing.T) {
	now := time.Date(
		2026, 10, 17, 0, 0, 0, 0, time.UTC,
	)

	service := NewService(
		NewMemoryRepository(),
		func() time.Time { return now },
	)

	root, err := service.Seal("user-1", SealInput{
		SourceEntryID: "entry-1",
		Body:          "Root memory",
		UnlockAt:      now.Add(time.Hour),
	})

	if err != nil {
		t.Fatal(err)
	}

	now = now.Add(2 * time.Hour)

	if _, err := service.Open("user-1", root.ID); err != nil {
		t.Fatal(err)
	}

	child, err := service.Seal("user-1", SealInput{
		SourceReflectionID: "reflection-1",
		ParentCapsuleID:    root.ID,
		Body:               "Child memory",
		UnlockAt:           now.Add(24 * time.Hour),
	})

	if err != nil {
		t.Fatal(err)
	}

	err = service.DeleteTree(
		"user-1",
		root.ID,
		[]string{root.ID, "incorrect-child-id"},
	)

	if !errors.Is(err, ErrCapsuleTreeChanged) {
		t.Fatalf("expected stale-tree conflict, got %v", err)
	}

	for _, id := range []string{root.ID, child.ID} {
		if _, err := service.GetMetadata("user-1", id); err != nil {
			t.Fatalf("failed deletion removed capsule %s: %v", id, err)
		}
	}

	err = service.DeleteTree(
		"user-1",
		root.ID,
		[]string{root.ID, child.ID},
	)

	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{root.ID, child.ID} {
		_, err := service.GetMetadata("user-1", id)
		if !errors.Is(err, ErrCapsuleNotFound) {
			t.Fatalf("capsule %s survived deletion: %v", id, err)
		}
	}
}
