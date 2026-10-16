package thread

import (
	"errors"
	"journalapp/internal/capsule"
	"testing"
	"time"
)

func TestDeleteConversationRejectsUnlinkedChild(t *testing.T) {
	threads, capsules, now, rootID := newFixture(t)

	*now = now.Add(2 * time.Hour)

	if _, err := capsules.Open("user-1", rootID); err != nil {
		t.Fatal(err)
	}

	child, err := capsules.Seal(
		"user-1",
		capsule.SealInput{
			SourceReflectionID: "unlinked-reflection",
			ParentCapsuleID:    rootID,
			Body:               "Do not lose this",
			UnlockAt:           now.Add(24 * time.Hour),
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	err = threads.DeleteConversation("user-1", rootID)

	if !errors.Is(err, ErrHistoryInconsistent) {
		t.Fatalf("expected integrity error, got %v", err)
	}

	for _, id := range []string{rootID, child.ID} {
		if _, err := capsules.GetMetadata("user-1", id); err != nil {
			t.Fatalf("capsule %s was removed: %v", id, err)
		}
	}
}
