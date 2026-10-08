package journal

import "testing"

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
