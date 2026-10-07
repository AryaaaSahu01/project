package journal

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

var ErrEmptyEntry = errors.New("journal entry cannot be empty")
var ErrEntryNotFound = errors.New("journal entry not found")

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{
		repository: repository,
	}
}

func (s *Service) Create(
	userID string,
	input CreateEntryInput,
) (Entry, error) {
	title := strings.TrimSpace(input.Title)
	body := strings.TrimSpace(input.Body)

	if title == "" && body == "" {
		return Entry{}, ErrEmptyEntry
	}

	id, err := newID()
	if err != nil {
		return Entry{}, err
	}
	now := time.Now().UTC()

	entry := Entry{
		ID:        id,
		UserID:    userID,
		Title:     title,
		Body:      body,
		CreatedAt: now,
		UpdatedAt: now,
	}

	return s.repository.Create(entry)
}

func (s *Service) List(userID string) ([]Entry, error) {
	return s.repository.ListByUser(userID)
}
func newID() (string, error) {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *Service) Get(
	userID string,
	entryID string,
) (Entry, error) {
	return s.repository.GetByID(userID, entryID)
}
