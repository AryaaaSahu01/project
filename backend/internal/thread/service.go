package thread

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"journalapp/internal/capsule"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrCapsuleNotReturned = errors.New(
		"capsule must be opened before reflecting",
	)

	ErrEmptyReflection = errors.New(
		"reflection cannot be empty",
	)

	ErrReflectionTooLong = errors.New(
		"reflection exceeds maximum length",
	)
)

const MaxReflectionRunes = 20000

type Service struct {
	repository Repository
	capsules   *capsule.Service
	now        func() time.Time
}

func NewService(
	repository Repository,
	capsules *capsule.Service,
	now func() time.Time,

) *Service {
	return &Service{
		repository: repository,
		capsules:   capsules,
		now:        now,
	}
}

func (s *Service) requireReturned(
	userID string,
	capsuleID string,
) error {
	metadata, err := s.capsules.GetMetadata(
		userID,
		capsuleID,
	)
	if err != nil {
		return err
	}

	if metadata.Status != capsule.StatusReturned {
		return ErrCapsuleNotReturned
	}
	return nil

}

func (s *Service) Create(
	userID string,
	capsuleID string,
	input CreateReflectionInput,
) (Reflection, error) {
	userID = strings.TrimSpace(userID)
	capsuleID = strings.TrimSpace(capsuleID)

	if err := s.requireReturned(userID, capsuleID); err != nil {
		return Reflection{}, err
	}

	body := strings.TrimSpace(input.Body)

	if body == "" {
		return Reflection{}, ErrEmptyReflection
	}

	if utf8.RuneCountInString(body) > MaxReflectionRunes {
		return Reflection{}, ErrReflectionTooLong
	}

	bytes := make([]byte, 16)

	if _, err := rand.Read(bytes); err != nil {
		return Reflection{}, err
	}

	reflection := Reflection{
		ID:        hex.EncodeToString(bytes),
		UserID:    userID,
		CapsuleID: capsuleID,
		Body:      body,
		CreatedAt: s.now().UTC(),
	}

	return s.repository.Create(reflection)
}

func (s *Service) List(
	userID string,
	capsuleID string,
) ([]Reflection, error) {
	userID = strings.TrimSpace(userID)
	capsuleID = strings.TrimSpace(capsuleID)

	if err := s.requireReturned(userID, capsuleID); err != nil {
		return nil, err
	}

	reflections, err := s.repository.ListByCapsule(
		userID,
		capsuleID,
	)

	if err != nil {
		return nil, err
	}

	sort.Slice(reflections, func(i, j int) bool {
		if reflections[i].CreatedAt.Equal(
			reflections[j].CreatedAt,
		) {
			return reflections[i].ID < reflections[j].ID
		}

		return reflections[i].CreatedAt.Before(
			reflections[j].CreatedAt,
		)
	})

	return reflections, nil
}
