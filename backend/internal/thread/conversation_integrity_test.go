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

type mutationBeforeDeleteRepo struct {
	capsule.Repository
	beforeDelete func()
}

func (r *mutationBeforeDeleteRepo) DeleteTree(
	userID string,
	rootID string,
	expectedIDs []string,
) error {
	if r.beforeDelete != nil {
		hook := r.beforeDelete
		r.beforeDelete = nil
		hook()
	}

	return r.Repository.DeleteTree(
		userID,
		rootID,
		expectedIDs,
	)

}

func TestDeleteConversationRejectsLateChild(t *testing.T) {
	now := time.Date(
		2026, 10, 18, 0, 0, 0, 0, time.UTC,
	)

	repo := &mutationBeforeDeleteRepo{
		Repository: capsule.NewMemoryRepository(),
	}

	capsules := capsule.NewService(
		repo,
		func() time.Time { return now },
	)

	threads := NewService(
		NewMemoryRepository(),
		capsules,
		func() time.Time { return now },
	)

	root, err := capsules.Seal(
		"user-1",
		capsule.SealInput{
			SourceEntryID: "entry-1",
			Title:         "Original",
			Body:          "Protect this memory",
			UnlockAt:      now.Add(time.Hour),
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	now = now.Add(2 * time.Hour)

	if _, err := capsules.Open("user-1", root.ID); err != nil {
		t.Fatal(err)
	}

	draft, err := threads.Create(
		"user-1",
		root.ID,
		CreateReflectionInput{
			Body: "Do not delete this draft",
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	var lateChild capsule.Capsule
	var injectionErr error

	repo.beforeDelete = func() {
		lateChild, injectionErr = capsules.Seal(
			"user-1",
			capsule.SealInput{
				SourceReflectionID: "late-reflection",
				ParentCapsuleID:    root.ID,
				Body:               "Unexpected child",
				UnlockAt:           now.Add(time.Hour),
			},
		)
	}

	err = threads.DeleteConversation("user-1", root.ID)

	if injectionErr != nil {
		t.Fatalf("failed to inject child: %v", injectionErr)
	}

	if lateChild.ID == "" {
		t.Fatalf("mutation hook was not executed")
	}

	if !errors.Is(err, capsule.ErrCapsuleTreeChanged) {
		t.Fatalf(
			"expected stale-tree rejection, got %v",
			err,
		)
	}

	for _, id := range []string{root.ID, lateChild.ID} {
		if _, err := capsules.GetMetadata("user-1", id); err != nil {
			t.Fatalf(
				"rejected deletion lost capsule %s: %v",
				id, err,
			)
		}
	}

	saved, err := threads.repository.GetByID(
		"user-1",
		draft.ID,
	)
	if err != nil {
		t.Fatalf("draft disappeared: %v", err)
	}

	if saved.Body != draft.Body {
		t.Fatal("draft changed during rejected deletion")
	}
}
