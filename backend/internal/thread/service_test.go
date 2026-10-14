package thread

import (
	"encoding/json"
	"errors"
	"journalapp/internal/capsule"
	"strings"
	"sync"
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

func TestSealReflectionCreatesChildCapsule(t *testing.T) {
	threads, capsules, now, parentID := newFixture(t)

	*now = now.Add(2 * time.Hour)

	_, err := capsules.Open("user-1", parentID)
	if err != nil {
		t.Fatal(err)
	}

	reflection, err := threads.Create(
		"user-1",
		parentID,
		CreateReflectionInput{
			Body: "Life has changed since I wrote that.",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	child, err := threads.SealReflection(
		"user-1",
		parentID,
		reflection.ID,
		now.Add(24*time.Hour),
	)

	if err != nil {
		t.Fatal(err)
	}

	if child.ParentCapsuleID != parentID {
		t.Fatal("child capsule has incorrect parent")
	}

	if child.SourceReflectionID != reflection.ID {
		t.Fatal("child cpasule has incorrect source reflection")
	}

	if child.Status != capsule.StatusSealed {
		t.Fatal("child capsule should be sealed")
	}

	list, err := threads.List("user-1", parentID)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 {
		t.Fatalf("expected 1 reflection, got %d", len(list))
	}

	if list[0].Status != StatusSealed {
		t.Fatal("reflection should be marked sealed")
	}

	if list[0].Body != "" {
		t.Fatal("sealed reflection still exposes its body")
	}

	if list[0].SealedCapsuleID != child.ID {
		t.Fatal("reflection does not link to child capsule")
	}

	encoded, err := json.Marshal(list)

	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(
		string(encoded),
		"Life has changed since I wrote that.",
	) {
		t.Fatal("sealed reflection leaked through JSON")
	}

	_, err = capsules.Open("user-1", child.ID)
	if !errors.Is(err, capsule.ErrCapsuleLocked) {
		t.Fatalf("expected locked child, got %v", err)
	}
	*now = now.Add(25 * time.Hour)

	opened, err := capsules.Open("user-1", child.ID)
	if err != nil {
		t.Fatal(err)
	}

	if opened.Body != reflection.Body {
		t.Fatal("child capsule did not preserve reflection text")
	}

}

func TestCannotResealReflectionTwice(t *testing.T) {
	threads, capsules, now, parentID := newFixture(t)

	*now = now.Add(2 * time.Hour)

	_, err := capsules.Open("user-1", parentID)

	if err != nil {
		t.Fatal(err)
	}

	reflection, err := threads.Create(
		"user-1",
		parentID,
		CreateReflectionInput{Body: "My reflection"},
	)

	if err != nil {
		t.Fatal(err)
	}
	unlockAt := now.Add(24 * time.Hour)

	_, err = threads.SealReflection(
		"user-2",
		parentID,
		reflection.ID,
		unlockAt,
	)

	if !errors.Is(err, capsule.ErrCapsuleNotFound) {
		t.Fatalf("expected access denial, got %v", err)
	}

	_, err = threads.SealReflection(
		"user-1",
		parentID,
		reflection.ID,
		unlockAt,
	)

	if err != nil {
		t.Fatal(err)
	}

	_, err = threads.SealReflection(
		"user-1",
		parentID,
		reflection.ID,
		unlockAt,
	)

	if !errors.Is(err, ErrReflectionAlreadySealed) {
		t.Fatalf("expected already-sealed error, got %v", err)
	}

	list, err := capsules.List("user-1")
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 2 {
		t.Fatalf("expected parent and child, got %d", len(list))
	}

	err = capsules.Destroy("user-1", parentID)

	if !errors.Is(err, capsule.ErrCapsuleHasChildren) {
		t.Fatalf("expected parent deletion conflict, got %v", err)
	}

}

func TestFailedResealKeepsReflection(t *testing.T) {
	threads, capsules, now, parentID := newFixture(t)

	*now = now.Add(2 * time.Hour)

	_, err := capsules.Open("user-1", parentID)
	if err != nil {
		t.Fatal(err)
	}

	reflection, err := threads.Create(
		"user-1",
		parentID,
		CreateReflectionInput{
			Body: "This writing must survive failure.",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = threads.SealReflection(
		"user-1",
		parentID,
		reflection.ID,
		now.Add(-time.Minute),
	)

	if !errors.Is(err, capsule.ErrInvalidUnlockAt) {
		t.Fatalf("expected invalid unlock date, got %v", err)
	}

	saved, err := threads.repository.GetByID(
		"user-1",
		reflection.ID,
	)
	if err != nil {
		t.Fatal(err)
	}

	if saved.Status != StatusDraft {
		t.Fatal("failed reseal changed reflection status")
	}

	if saved.Body != reflection.Body {
		t.Fatal("failed reseal lost refletion content")
	}

	if saved.SealedCapsuleID != "" {
		t.Fatal("failed reseal linked a child capsule")
	}

	capsuleList, err := capsules.List("user-1")
	if err != nil {
		t.Fatal(err)
	}

	if len(capsuleList) != 1 {
		t.Fatal("failed reseal unexpectedly created a child capsule")
	}
}

func TestDeleteCapsuleRemoveDraftReflections(t *testing.T) {
	threads, capsules, now, capsuleID := newFixture(t)

	*now = now.Add(2 * time.Hour)

	if _, err := capsules.Open(
		"user-1",
		capsuleID,
	); err != nil {
		t.Fatal(err)
	}

	for _, body := range []string{
		"First draft",
		"Second draft",
	} {
		_, err := threads.Create(
			"user-1",
			capsuleID,
			CreateReflectionInput{Body: body},
		)

		if err != nil {
			t.Fatal(err)
		}
	}

	err := threads.DeleteCapsule(
		"user-2",
		capsuleID,
	)

	if !errors.Is(err, capsule.ErrCapsuleNotFound) {
		t.Fatalf("expected ownership rejection, got %v", err)
	}

	err = threads.DeleteCapsule(
		"user-1",
		capsuleID,
	)

	if err != nil {
		t.Fatal(err)
	}

	_, err = capsules.GetMetadata(
		"user-1",
		capsuleID,
	)

	if !errors.Is(err, capsule.ErrCapsuleNotFound) {
		t.Fatal("deleted capsule is still accessible")
	}

	remaining, err := threads.repository.ListByCapsule(
		"user-1",
		capsuleID,
	)

	if err != nil {
		t.Fatal(err)
	}

	if len(remaining) != 0 {
		t.Fatalf(
			"expected no orphaned reflections, got %d",
			len(remaining),
		)
	}
}

func TestDeleteCapsulePreservesSealedChild(t *testing.T) {
	threads, capsules, now, parentID := newFixture(t)

	*now = now.Add(2 * time.Hour)

	if _, err := capsules.Open(
		"user-1",
		parentID,
	); err != nil {
		t.Fatal(err)
	}

	reflection, err := threads.Create(
		"user-1",
		parentID,
		CreateReflectionInput{
			Body: "My future self should read this.",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	child, err := threads.SealReflection(
		"user-1",
		parentID,
		reflection.ID,
		now.Add(24*time.Hour),
	)

	if err != nil {
		t.Fatal(err)
	}

	err = threads.DeleteCapsule(
		"user-1",
		parentID,
	)

	if !errors.Is(err, capusle.ErrCapsuleHasChildren) {
		t.Fatalf("expected parent deletion conflict, got %v", err)
	}

	err = threads.DeleteCapsule(
		"user-1",
		child.ID,
	)

	if !errors.Is(err, capsule.ErrCapsuleLinkedToReflection) {
		t.Fatalf("expected linked-child conflict, got %v", err)
	}

	metadata, err := capsules.GetMetadata(
		"user-1",
		child.ID,
	)

	if err != nil {
		t.Fatal(err)
	}

	if metadata.ParentCapsuleID != parentID {
		t.Fatal("child lost parent relationship")

	}

	list, err := threads.List("user-1", parentID)

	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 ||
		list[0].Status != StatusSealed ||
		list[0].SealedCapsuleID != child.ID ||
		list[0].Body != "" {
		t.Fatal("sealed reflection state was corrupted")
	}
}

func TestConcurrentCreateAndDeleteNoOrphans(t *testing.T) {
	threads, capsules, now, capsuleID := newFixture(t)

	*now = now.Add(2 * time.Hour)

	if _, err := capsules.Open(
		"user-1",
		capsuleID,
	); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	var createErr error
	var deleteErr error

	start := make(chan struct{})

	wg.Add(2)

	go func() {
		defer wg.Done()
		<-start

		_, createErr = threads.Create(
			"user-1",
			capsuleID,
			CreateReflectionInput{
				Body: "Concurrent reflection",
			},
		)
	}()

	go func() {
		defer wg.Done()

		<-start

		deleteErr = threads.DeleteCapsule(
			"user-1",
			capsuleID,
		)
	}()

	close(start)
	wg.Wait()

	if deleteErr != nil {
		t.Fatalf("capsule deletion failed: %v", deleteErr)
	}

	if createErr != nil &&
		!errors.Is(createErr, capsule.ErrCapsuleNotFound) {
		t.Fatalf("unexpected cretaion error :%v", createErr)
	}

	remaining, err := threads.repository.ListByCapsule(
		"user-1",
		capsuleID,
	)

	if err != nil {
		t.Fatal(err)
	}

	if len(remaining) != 0 {
		t.Fatal("concurrent operation left orphaned reflections")
	}

}
