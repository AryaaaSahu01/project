package thread

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"journalapp/internal/capsule"
)

func TestGetHistoryMultiGeneration(t *testing.T) {
	threads, capsules, now, rootID := newFixture(t)

	locked, err := threads.GetHistory("user-1", rootID)

	if err != nil {
		t.Fatal(err)
	}

	if locked.Capsule.Status != capsule.StatusSealed ||
		len(locked.Reflections) != 0 {
		t.Fatal("unopened capsule exposed history")
	}

	*now = now.Add(2 * time.Hour)

	if _, err := capsules.Open("user-1", rootID); err != nil {
		t.Fatal(err)
	}

	draft, err := threads.Create(
		"user-1",
		rootID,
		CreateReflectionInput{Body: "My visible draft"},
	)

	if err != nil {
		t.Fatal(err)
	}

	*now = now.Add(time.Minute)

	secretB := "Private writing for capsule B"

	reflectionB, err := threads.Create(
		"user-1",
		rootID,
		CreateReflectionInput{Body: secretB},
	)

	if err != nil {
		t.Fatal(err)
	}

	childB, err := threads.SealReflection(
		"user-1",
		rootID,
		reflectionB.ID,
		now.Add(time.Hour),
	)

	if err != nil {
		t.Fatal(err)
	}

	history, err := threads.GetHistory("user-1", rootID)

	if err != nil {
		t.Fatal(err)
	}

	if len(history.Reflections) != 2 {
		t.Fatalf(
			"expected 2 reflections, got %d",
			len(history.Reflections),
		)
	}

	if history.Reflections[0].ID != draft.ID ||
		history.Reflections[0].Body != draft.Body {
		t.Fatal("draft reflection is incorrect")
	}

	second := history.Reflections[1]

	if second.Status != StatusSealed ||
		second.Body != "" ||
		second.Child == nil ||
		second.Child.Capsule.ID != childB.ID {
		t.Fatal("sealed reflection or child link is incorrect")
	}

	if second.Child.Capsule.Status != capsule.StatusSealed ||
		len(second.Child.Reflections) != 0 {
		t.Fatal("sealed child exposed its history")
	}

	encoded, err := json.Marshal(history)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(encoded), secretB) {
		t.Fatal("sealed writing leaked into history JSON")
	}

	*now = now.Add(2 * time.Hour)

	if _, err := capsules.Open("user-1", childB.ID); err != nil {
		t.Fatal(err)
	}

	reflectionC, err := threads.Create(
		"user-1",
		childB.ID,
		CreateReflectionInput{
			Body: "Private writing for capsule C",
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	childC, err := threads.SealReflection(
		"user-1",
		childB.ID,
		reflectionC.ID,
		now.Add(24*time.Hour),
	)

	if err != nil {
		t.Fatal(err)
	}

	history, err = threads.GetHistory("user-1", rootID)
	if err != nil {
		t.Fatal(err)
	}

	b := history.Reflections[1].Child
	if b == nil || b.Capsule.ID != childB.ID ||
		b.Capsule.Status != capsule.StatusReturned {
		t.Fatal("second generation is incorrect")
	}

	if len(b.Reflections) != 1 {
		t.Fatal("expected one reflection in generation B")
	}

	c := b.Reflections[0].Child
	if c == nil || c.Capsule.ID != childC.ID ||
		c.Capsule.Status != capsule.StatusSealed {
		t.Fatal("third generation is incorrect")
	}

	encoded, err = json.Marshal(history)

	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(
		string(encoded),
		"Private writing for capsule C",
	) {
		t.Fatal("third-generation sealed content leaked")
	}

}

func TestGetHistoryOwnership(t *testing.T) {
	threads, _, _, capsuleID := newFixture(t)

	_, err := threads.GetHistory("user-2", capsuleID)

	if !errors.Is(err, capsule.ErrCapsuleNotFound) {
		t.Fatalf("expected access denial, got %v", err)
	}

	_, err = threads.GetHistory("user-1", "missing")

	if !errors.Is(err, capsule.ErrCapsuleNotFound) {
		t.Fatalf("expected missing capsule, got %v", err)
	}
}

func TestGetHistoryMultipleBranches(t *testing.T) {
	threads, capsules, now, rootID := newFixture(t)

	*now = now.Add(2 * time.Hour)

	if _, err := capsules.Open("user-1", rootID); err != nil {
		t.Fatal(err)
	}

	first, err := threads.Create(
		"user-1",
		rootID,
		CreateReflectionInput{Body: "First branch"},
	)
	if err != nil {
		t.Fatal(err)
	}

	firstChild, err := threads.SealReflection(
		"user-1",
		rootID,
		first.ID,
		now.Add(24*time.Hour),
	)

	if err != nil {
		t.Fatal(err)
	}

	*now = now.Add(time.Minute)

	second, err := threads.Create(
		"user-1",
		rootID,
		CreateReflectionInput{Body: "Second Branch"},
	)

	if err != nil {
		t.Fatal(err)
	}

	secondChild, err := threads.SealReflection(
		"user-1",
		rootID,
		second.ID,
		now.Add(24*time.Hour),
	)

	if err != nil {
		t.Fatal(err)
	}

	history, err := threads.GetHistory("user-1", rootID)
	if err != nil {
		t.Fatal(err)
	}

	if len(history.Reflections) != 2 {
		t.Fatal("expected two branches")
	}

	a := history.Reflections[0]
	b := history.Reflections[1]

	if a.ID != first.ID || b.ID != second.ID {
		t.Fatal("branches are not chronologically ordered")
	}

	if a.Child == nil || b.Child == nil ||
		a.Child.Capsule.ID != firstChild.ID ||
		b.Child.Capsule.ID != secondChild.ID {
		t.Fatal("branch links are incorrect")
	}
}

func TestGetHistoryRejectsBrokenLink(t *testing.T) {
	threads, capsules, now, rootID := newFixture(t)

	*now = now.Add(2 * time.Hour)

	if _, err := capsules.Open("user-1", rootID); err != nil {
		t.Fatal(err)
	}

	reflection, err := threads.Create(
		"user-1",
		rootID,
		CreateReflectionInput{Body: "Keep this safe"},
	)

	if err != nil {
		t.Fatal(err)
	}

	_, err = threads.SealReflection(
		"user-1",
		rootID,
		reflection.ID,
		now.Add(24*time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}

	saved, err := threads.repository.GetByID(
		"user-1",
		reflection.ID,
	)

	if err != nil {
		t.Fatal(err)
	}

	saved.SealedCapsuleID = "nonexistent-child"

	if _, err := threads.repository.Update(saved); err != nil {
		t.Fatal(err)
	}

	_, err = threads.GetHistory("user-1", rootID)

	if !errors.Is(err, ErrHistoryInconsistent) {
		t.Fatalf("expected broken-link error, got %v", err)
	}
}
