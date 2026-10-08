package capsule

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

var ErrEmptySnapshot = errors.New("capsule content cannot be empty")
var ErrInvalidUnlockAt = errors.New("unlock date must be in the future")
var ErrInvalidSourceEntry = errors.New("source entry ID is required")
var ErrInvalidUser = errors.New("user ID is required")
var ErrCapsuleLocked = errors.New("Capsule is still sealed")

type SealInput struct {
	SourceEntryID string
	Title         string
	Body          string
	UnlockAt      time.Time
}

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(
	repository Repository,
	now func() time.Time,
) *Service {
	return &Service{
		repository: repository,
		now:        now,
	}
}

func (s *Service) Seal(
	userID string,
	input SealInput,
) (Capsule, error) {

	userID = strings.TrimSpace(userID)
	sourceID := strings.TrimSpace(input.SourceEntryID)

	if userID == "" {
		return Capsule{}, ErrInvalidUser
	}

	if sourceID == "" {
		return Capsule{}, ErrInvalidSourceEntry
	}

	title := strings.TrimSpace(input.Title)
	body := strings.TrimSpace(input.Body)

	if title == "" && body == "" {
		return Capsule{}, ErrEmptySnapshot
	}

	now := s.now().UTC()
	unlockAt := input.UnlockAt.UTC()

	if !unlockAt.After(now) {
		return Capsule{}, ErrInvalidUnlockAt
	}

	idBytes := make([]byte, 16)

	if _, err := rand.Read(idBytes); err != nil {
		return Capsule{}, err
	}

	c := Capsule{
		ID:            hex.EncodeToString(idBytes),
		UserID:        userID,
		SourceEntryID: sourceID,
		SealedAt:      now,
		UnlockAt:      unlockAt,
		Status:        StatusSealed,
		title:         title,
		body:          body,
	}

	return s.repository.Create(c)
}

func (s *Service) Open(
	userID string,
	capsuleID string,
) (OpenedCapsule, error) {

	c, err := s.repository.GetByID(userID, capsuleID)
	if err != nil {
		return OpenedCapsule{}, err
	}

	if c.Status != StatusSealed && c.Status != StatusReturned {
		return OpenedCapsule{}, ErrCapsuleNotFound
	}

	if s.now().UTC().Before(c.UnlockAt) {
		return OpenedCapsule{}, ErrCapsuleLocked
	}

	if c.Status == StatusSealed {
		c.Status = StatusReturned

		c, err = s.repository.Update(c)
		if err != nil {
			return OpenedCapsule{}, err
		}
	}
	return OpenedCapsule{
		ID:       c.ID,
		Title:    c.title,
		Body:     c.body,
		SealedAt: c.SealedAt,
		UnlockAt: c.UnlockAt,
	}, nil
}
