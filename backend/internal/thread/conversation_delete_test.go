package thread

import (
	"errors"
	"journalapp/internal/capsule"
	"testing"
	"time"
)

func TestDeleteConversationWithDescendants(t *testing.T) {
	threads, capsules, now, rootID := newFixture(t)

	*now = now.Add(2 * time.Hour)

	if _, err := capsules.Open("user-1", rootID); err != nil {
		t.Fatal(err)
	}

	_, err := threads.Create(
		"user-1",
		rootID,
		CreateReflectionInput{Body: "A draft"},
	)

	if err != nil {
		t.Fatal(err)
	}

	first, err := threads.Create(
		"user-1",
		rootID,
		CreateReflectionInput{Body: "First sealed reflection"},
	)

	if err != nil {
		t.Fatal(err)
	}

	child, err := threads.SealReflection(
		"user-1",
		rootID,
		first.ID,
		now.Add(time.Hour),
	)

	if err != nil {
		t.Fatal(err)
	}

	*now = now.Add(2 * time.Hour)

	if _, err := capsules.Open("user-1", child.ID); err != nil {
		t.Fatal(err)
	}
	second, err := threads.Create(
		"user-1",
		child.ID,
		CreateReflectionInput{Body: "Second generation"},
	)

	if err != nil {
		t.Fatal(err)
	}

	grandchild, err := threads.SealReflection(
		"user-1",
		child.ID,
		second.ID,
		now.Add(24*time.Hour),
	)

	if err != nil {
		t.Fatal(err)
	}

	unrelated, err := capsules.Seal(
		"user-1",
		capsule.SealInput{
			SourceEntryID: "unrelated-entry",
			Body:          "Keep this memory",
			UnlockAt:      now.Add(time.Hour),
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	if err := threads.Deleteconversation("user-1", rootID); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{
		rootID,
		child.ID,
		grandchild.ID,
	} {
		_, err := capsules.GetMetadata("user-1", id)
		if !errors.Is(err, capsule.ErrCapsuleNotFound) {
			t.Fatalf("capsule %s still exists: %v", id, err)
		}

		reflections, err := threads.repository.ListByCapsule(
			"user-1", id,
		)

		if err != nil {
			t.Fatal(err)
		}

		if len(reflections) != 0 {
			t.Fatalf("orphaned reflections remain for %s", id)
		}
	}

	if _, err := capsules.GetMetadata(
		"user-1", unrelated.ID,
	); err != nil {
		t.Fatal("unrelated capsule was deleted:", err)
	}
}

func TestDeleteConversationRejectsNonRootAndOtherUser(t *testing.T) {
	threads, capsules, now, rootID := newFixture(t)

	*now = now.Add(2 * time.Hour)

	if _, err := capsules.Open("user-1", rootID); err != nil {
		t.Fatal(err)
	}

	reflection, err := threads.Create(
		"user-1",
		rootID,
		CreateReflectionInput{Body: "Preserve this writing"},
	)

	if err != nil {
		t.Fatal(err)
	}

	child, err := threads.SealReflection(
		"user-1",
		rootID,
		reflection.ID,
		now.Add(24*time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}

	err = threads.Deleteconversation("user-2", rootID)
	if !errors.Is(err, capsule.ErrCapsuleNotFound) {
		t.Fatalf("expected ownership rejection, got %v", err)
	}

	err = threads.Deleteconversation("user-1", child.ID)
	if !errors.Is(err, capsule.ErrCapsuleNotRoot) {
		t.Fatalf("expected non-root rejection, got %v", err)
	}

	for _, id := range []string{rootID, child.ID} {
		if _, err := capsules.GetMetadata("user-1", id); err != nil {
			t.Fatalf("rejected deletion changed capsule %s: %v", id, err)
		}
	}
}
