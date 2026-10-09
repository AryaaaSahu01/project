package sealing

import (
	"errors"
	"journalapp/internal/capsule"
	"journalapp/internal/journal"
	"time"
)

type Service struct {
	journals *journal.Service
	capsules *capsule.Service
}

func NewService(
	journals *journal.Service,
	capsules *capsule.Service,
) *Service {
	return &Service{
		journals: journals,
		capsules: capsules,
	}
}

func (s *Service) SealEntry(
	userID string,
	entryID string,
	unlockAt time.Time,
) (capsule.CapsuleMetadata, error) {
	entry, err := s.journals.Get(userID, entryID)
	if err != nil {
		return capsule.CapsuleMetadata{}, err
	}

	c, err := s.capsules.Seal(userID, capsule.SealInput{
		SourceEntryID: entry.ID,
		Title:         entry.Title,
		Body:          entry.Body,
		UnlockAt:      unlockAt,
	})

	if err != nil {
		return capsule.CapsuleMetadata{}, err
	}

	if err := s.journals.Delete(userID, entryID); err != nil {
		rollbackErr := s.capsules.Destroy(userID, c.ID)
		return capsule.CapsuleMetadata{}, errors.Join(err, rollbackErr)
	}

	return capsule.CapsuleMetadata{
		ID:            c.ID,
		SourceEntryID: c.SourceEntryID,
		Status:        c.Status,
		SealedAt:      c.SealedAt,
		UnlockAt:      c.UnlockAt,
		CanOpen:       false,
	}, nil

}
