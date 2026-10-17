package thread

import (
	"errors"
	"journalapp/internal/capsule"
	"sync"
	"testing"
	"time"
)

func TestConcurrentDeleteConversationAndCreate(t *testing.T) {
	for attempt := 0; attempt < 50; attempt++ {
		threads, capsules, now, rootID := newFixture(t)

		*now = now.Add(2 * time.Hour)

		if _, err := capsules.Open("user-1", rootID); err != nil {
			t.Fatal(err)
		}

		start := make(chan struct{})

		var wg sync.WaitGroup
		var createErr, deleteErr error

		wg.Add(2)

		go func() {
			defer wg.Done()
			<-start

			_, createErr = threads.Create(
				"user-1",
				rootID,
				CreateReflectionInput{Body: "Racing draft"},
			)
		}()

		go func() {
			defer wg.Done()
			<-start
			deleteErr = threads.DeleteConversation(
				"user-1",
				rootID,
			)
		}()

		close(start)
		wg.Wait()

		if deleteErr != nil {
			t.Fatalf("attempt %d: deletion failed: %v",
				attempt, deleteErr)
		}

		if createErr != nil &&
			!errors.Is(createErr, capsule.ErrCapsuleNotFound) {
			t.Fatalf("attempt %d: unexpected created error: %v",
				attempt, createErr)
		}

		remaining, err := capsules.List("user-1")
		if err != nil || len(remaining) != 0 {
			t.Fatalf("attempt %d: capsules remain: %v",
				attempt, err)
		}

		reflections, err := threads.repository.ListByCapsule(
			"user-1", rootID,
		)

		if err != nil || len(reflections) != 0 {
			t.Fatalf("attempt %d: orphan reflections remain",
				attempt)
		}

	}
}

func TestConcurrentDeleteConversationAndReseal(t *testing.T) {
	for attempt := 0; attempt < 50; attempt++ {
		threads, capsules, now, rootID := newFixture(t)

		*now = now.Add(2 * time.Hour)

		if _, err := capsules.Open("user-1", rootID); err != nil {
			t.Fatal(err)
		}

		reflection, err := threads.Create(
			"user-1",
			rootID,
			CreateReflectionInput{Body: "Future reflection"},
		)

		if err != nil {
			t.Fatal(err)
		}

		start := make(chan struct{})
		var wg sync.WaitGroup
		var child capsule.CapsuleMetadata
		var sealErr, deleteErr error

		wg.Add(2)

		go func() {
			defer wg.Done()
			<-start

			child, sealErr = threads.SealReflection(
				"user-1",
				rootID,
				reflection.ID,
				now.Add(24*time.Hour),
			)
		}()

		go func() {
			defer wg.Done()
			<-start

			deleteErr = threads.DeleteConversation(
				"user-1",
				rootID,
			)
		}()

		close(start)
		wg.Wait()

		if deleteErr != nil {
			t.Fatalf("attempt %d: deletion failed: %v",
				attempt, deleteErr)
		}

		if sealErr != nil &&
			!errors.Is(sealErr, capsule.ErrCapsuleNotFound) {
			t.Fatalf("attempt %d: unexpected seal error: %v",
				attempt, sealErr)
		}

		remaining, err := capsules.List("user-1")
		if err != nil || len(remaining) != 0 {
			t.Fatalf("attempt %d: orphan capsules remain",
				attempt)
		}

		reflections, err := threads.repository.ListByCapsule(
			"user-1", rootID,
		)

		if err != nil || len(reflections) != 0 {
			t.Fatalf("attempt %d: orphan reflections remain",
				attempt)
		}

		if sealErr == nil && child.ID == "" {
			t.Fatal("succesful reseal returned no child ID")
		}

	}
}

type failingDeleteTreeRepo struct {
	capsule.Repository
	failure error
}

func (r *failingDeleteTreeRepo) DeleteTree(
	userID string,
	rootID string,
	expectedIDs []string,
) error {
	return r.failure
}

func TestDeleteConversationStorageFailurePreservesData(t *testing.T) {
	now := time.Date(
		2026, 10, 17, 0, 0, 0, 0, time.UTC,
	)

	injectedErr := errors.New("simulated storage failure")

	repository := &failingDeleteTreeRepo{
		Repository: capsule.NewMemoryRepository(),
		failure:    injectedErr,
	}

	capsules := capsule.NewService(
		repository,
		func() time.Time { return now },
	)

	threads := NewService(
		NewMemoryRepository(),
		capsules,
		func() time.Time { return now },
	)

	root, err := capsules.Seal("user-1", capsule.SealInput{
		SourceEntryID: "entry-1",
		Body:          "Preserve this memory",
		UnlockAt:      now.Add(time.Hour),
	})

	if err != nil {
		t.Fatal(err)
	}

	now = now.Add(2 * time.Hour)

	if _, err := capsules.Open("user-1", root.ID); err != nil {
		t.Fatal(err)
	}

	reflection, err := threads.Create(
		"user-1",
		root.ID,
		CreateReflectionInput{Body: "Preserve this draft"},
	)

	if err != nil {
		t.Fatal(err)
	}

	err = threads.DeleteConversation("user-1", root.ID)

	if !errors.Is(err, injectedErr) {
		t.Fatalf("expected injected failure, got %v", err)
	}

	if _, err := capsules.GetMetadata(
		"user-1", root.ID,
	); err != nil {
		t.Fatalf("capsule was lost: %v", err)
	}

	saved, err := threads.repository.GetByID(
		"user-1", reflection.ID,
	)

	if err != nil {
		t.Fatalf("reflection was lost: %v", err)
	}

	if saved.Body != reflection.Body {
		t.Fatal("reflection content changed after failed deletion")
	}
}
