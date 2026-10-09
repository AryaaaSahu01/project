package capsule

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
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

func (s *Service) GetMetadata(
	userID string,
	capsuleID string,
) (CapsuleMetadata, error) {
	c, err := s.repository.GetByID(userID, capsuleID)

	if err != nil {
		return CapsuleMetadata{}, err
	}

	if c.Status == StatusDestroyed {
		return CapsuleMetadata{}, ErrCapsuleNotFound
	}

	now := s.now().UTC()

	canOpen := !now.Before(c.UnlockAt) &&
		(c.Status == StatusSealed || c.Status == StatusReturned)

	return CapsuleMetadata{
		ID:            c.ID,
		SourceEntryID: c.SourceEntryID,
		Status:        c.Status,
		SealedAt:      c.SealedAt,
		UnlockAt:      c.UnlockAt,
		CanOpen:       canOpen,
	}, nil
}

func (s *Service) Destroy(
	userID string,
	capsuleID string,
) error {
	userID = strings.TrimSpace(userID)
	capsuleID = strings.TrimSpace(capsuleID)

	if userID == "" {
		return ErrInvalidUser
	}

	if capsuleID == "" {
		return ErrCapsuleNotFound
	}

	return s.repository.Delete(userID, capsuleID)
}

func (s *Service) List(
	userID string,
) ([]CapsuleMetadata, error) {

	userID = strings.TrimSpace(userID)

	if userID == "" {
		return nil, ErrInvalidUser
	}

	capsules, err := s.repository.ListByUser(userID)

	if err != nil {
		return nil, err
	}

	now := s.now().UTC()
	result := make([]CapsuleMetadata, 0, len(capsules))

	for _, c := range capsules {
		if c.Status == StatusDestroyed {
			continue
		}
		canOpen := !now.Before(c.UnlockAt) &&
			(c.Status == StatusSealed || c.Status == StatusReturned)

		result = append(result, CapsuleMetadata{
			ID:            c.ID,
			SourceEntryID: c.SourceEntryID,
			Status:        c.Status,
			SealedAt:      c.SealedAt,
			UnlockAt:      c.UnlockAt,
			CanOpen:       canOpen,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].UnlockAt.Equal(result[j].UnlockAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].UnlockAt.Before(result[j].UnlockAt)

	})
	return result, nil

}
