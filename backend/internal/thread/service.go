package thread

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"journalapp/internal/capsule"
	"sort"
	"strings"
	"sync"
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
	ErrReflectionAlreadySealed = errors.New(
		"reflection is already sealed",
	)
)

const MaxReflectionRunes = 20000

type Service struct {
	mu         sync.Mutex
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

	s.mu.Lock()
	defer s.mu.Unlock()

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
		Status:    StatusDraft,
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

func (s *Service) SealReflection(
	userID string,
	parentCapsuleID string,
	reflectionID string,
	unlockAt time.Time,
) (capsule.CapsuleMetadata, error) {

	s.mu.Lock()
	defer s.mu.Unlock()

	userID = strings.TrimSpace(userID)

	parentCapsuleID = strings.TrimSpace(parentCapsuleID)

	reflectionID = strings.TrimSpace(reflectionID)

	if err := s.requireReturned(
		userID,
		parentCapsuleID,
	); err != nil {
		return capsule.CapsuleMetadata{}, err
	}

	reflection, err := s.repository.GetByID(
		userID,
		reflectionID,
	)

	if err != nil {
		return capsule.CapsuleMetadata{}, err
	}

	if reflection.CapsuleID != parentCapsuleID {
		return capsule.CapsuleMetadata{},
			ErrReflectionNotFound
	}

	if reflection.Status == StatusSealed {
		return capsule.CapsuleMetadata{},
			ErrReflectionAlreadySealed
	}

	if reflection.Status != StatusDraft {
		return capsule.CapsuleMetadata{},
			ErrReflectionNotFound
	}
	child, err := s.capsules.Seal(
		userID,
		capsule.SealInput{
			SourceReflectionID: reflection.ID,
			ParentCapsuleID:    parentCapsuleID,
			Title:              "Then & Now",
			Body:               reflection.Body,
			UnlockAt:           unlockAt,
		},
	)

	if err != nil {
		return capsule.CapsuleMetadata{}, err
	}

	reflection.Status = StatusSealed
	reflection.Body = ""
	reflection.SealedCapsuleID = child.ID

	_, err = s.repository.Update(reflection)

	if err != nil {
		rollbackErr := s.capsules.Destroy(
			userID,
			child.ID,
		)

		return capsule.CapsuleMetadata{},
			errors.Join(err, rollbackErr)
	}

	return s.capsules.GetMetadata(
		userID,
		child.ID,
	)

}

func (s *Service) DeleteByCapsule(
	userID string,
	capsuleID string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	userID = strings.TrimSpace(userID)
	capsuleID = strings.TrimSpace(capsuleID)

	metadata, err := s.capsules.GetMetadata(
		userID,
		capsuleID,
	)

	if err != nil {
		return err
	}

	if metadata.SourceReflectionID != "" {
		return capsule.ErrCapsuleLinkedToReflection
	}

	if err := s.capsules.Destroy(
		userID,
		capsuleID,
	); err != nil {
		return err
	}

	s.repository.DeleteByCapsule(
		userID,
		capsuleID,
	)

	return nil
}

func (s *Service) DeleteCapsule(
	userID string,
	caspuleID string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	userID = strings.TrimSpace(userID)
	capsuleID = strings.TrimSpace(capsuleID)

	metadata, err := s.capsules.GetMetadata(
		userID,
		capsuleID,
	)

	if err != nil {
		return err
	}

	if metadata.SourceReflectionID != "" {
		return capsule.ErrCapsuleLinkedToReflection
	}

	if err := s.capsules.Destroy(
		userID,
		capsuleID,
	); err != nil {
		return err
	}

	s.repository.DeleteByCapsule(
		userID,
		capsuleID,
	)
	return nil
}
