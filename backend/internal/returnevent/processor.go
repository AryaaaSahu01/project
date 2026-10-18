package returnevent

import (
	"journalapp/internal/capsule"
	"strings"
	"time"
)

type Processor struct {
	capsules   *capsule.Service
	repository Repository
	now        func() time.Time
}

func NewProcessor(
	capsules *capsule.Service,
	repository Repository,
	now func() time.Time,
) *Processor {
	return &Processor{
		capsules:   capsules,
		repository: repository,
		now:        now,
	}
}

func (p *Processor) ProcessDue(
	userID string,
) (int, error) {
	userID = strings.TrimSpace(userID)

	due, err := p.capsules.ListDue(userID)
	if err != nil {
		return 0, err
	}

	created := 0

	for _, c := range due {
		event := Event{
			ID:         TypeCapsuleDue + ":" + c.ID,
			UserID:     userID,
			CapsuleID:  c.ID,
			Type:       TypeCapsuleDue,
			UnlockAt:   c.UnlockAt,
			RecordedAt: p.now().UTC(),
		}

		inserted, err := p.repository.CreateIfAbsent(event)

		if err != nil {
			return created, err
		}

		if inserted {
			created++
		}

	}

	return created, nil

}
