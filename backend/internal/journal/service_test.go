package journal

import (
	"testing"
)

func TestCreateEntry(t *testing.T) {
	repo := NewMemoryRepository()
	service := NewService(repo)

	input := CreateEntryInput{
		Title: "My Test entry",
		Body:  "Testing the Journal Service",
	}

	entry, err := service.Create("user-1", input)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Body != input.Body {
		t.Errorf("expected %q, got %q", input.Body, entry.Body)
	}

	if entry.ID == "" {
		t.Error("expected a generated entry ID")
	}
}

func TestCreateEmptyEntry(t *testing.T) {
	repo := NewMemoryRepository()
	service := NewService(repo)

	input := CreateEntryInput{
		Title: "",
		Body:  "",
	}

	_, err := service.Create("user-1", input)

	if err != ErrEmptyEntry {
		t.Errorf("expected ErrEmptyEntry, got %v", err)
	}
}

func TestGetEntry(t *testing.T) {
	repo := NewMemoryRepository()
	service := NewService(repo)

	input := CreateEntryInput{
		Title: "First Entry",
		Body:  "Testing retrieval",
	}

	created, err := service.Create("user-1", input)
	if err != nil {
		t.Fatal(err)
	}

	found, err := service.Get("user-1", created.ID)
	if err != nil {
		t.Fatal(err)
	}

	if found.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, found.ID)
	}
	if found.Body != input.Body {
		t.Errorf("expected Body %q, got %q", input.Body, found.Body)
	}

	_, err = service.Get("user-2", created.ID)
	if err != ErrEntryNotFound {
		t.Errorf("expected ErrEntryNotFound, got %v", err)
	}

}

func TestUpdateEntry(t *testing.T) {
	repo := NewMemoryRepository()
	service := NewService(repo)

	created, err := service.Create("user-1", CreateEntryInput{
		Title: "Old Title",
		Body:  "Original body",
	})
	if err != nil {
		t.Fatal(err)
	}

	newTitle := "Updated title"

	updated, err := service.Update(
		"user-1",
		created.ID,
		UpdateEntryInput{Title: &newTitle},
	)

	if err != nil {
		t.Fatal(err)
	}

	if updated.Title != newTitle {
		t.Errorf("title was not updated")
	}

	if updated.Body != created.Body {
		t.Errorf("body should remain unchanged")
	}

	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("creation timestamp should not change")
	}
}

func TestDeleteEntry(t *testing.T) {
	repo := NewMemoryRepository()
	service := NewService(repo)

	created, err := service.Create("user-1", CreateEntryInput{
		Title: "Delete me",
		Body:  "Temproary Entry",
	})

	if err != nil {
		t.Fatal(err)
	}

	err = service.Delete("user-1", created.ID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Get("user-1", created.ID)
	if err != ErrEntryNotFound {
		t.Errorf("expected entry to be deleted, got %v", err)
	}

	err = service.Delete("user-1", created.ID)
	if err != ErrEntryNotFound {
		t.Errorf("expected ErrEntryNotFound on second delete")
	}
}
