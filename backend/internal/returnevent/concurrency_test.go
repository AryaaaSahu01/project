package returnevent

import (
	"errors"
	"journalapp/internal/capsule"
	"sync"
	"testing"
	"time"
)

func TestProcessDueConcurrenceStateChanges(t *testing.T) {
	for _, action := range []string{"open", "delete"} {
		t.Run(action, func(t *testing.T) {
			for attempt := 0; attempt < 50; attempt++ {
				now := time.Date(
					2026, 10, 19, 0, 0, 0, 0, time.UTC,
				)

				capsules := capsule.NewService(
					capsule.NewMemoryRepository(),
					func() time.Time { return now },
				)

				c, err := capsules.Seal(
					"user-1",
					capsule.SealInput{
						SourceEntryID: "entry-1",
						Body:          "Private memory",
						UnlockAt:      now.Add(time.Hour),
					},
				)

				if err != nil {
					t.Fatal(err)
				}

				now = now.Add(2 * time.Hour)

				repo := NewMemoryRepository()
				processor := NewProcessor(
					capsules,
					repo,
					func() time.Time { return now },
				)

				start := make(chan struct{})

				var wg sync.WaitGroup
				var processingErr, mutationErr error

				wg.Add(2)

				go func() {
					defer wg.Done()
					<-start

					_, processingErr = processor.ProcessDue(
						"user-1",
					)
				}()

				go func() {
					defer wg.Done()
					<-start

					if action == "open" {
						_, mutationErr = capsules.Open(
							"user-1", c.ID,
						)
					} else {
						mutationErr = capsules.Destroy(
							"user-1", c.ID,
						)
					}
				}()

				close(start)
				wg.Wait()

				if processingErr != nil {
					t.Fatalf(
						"attempt %d: processing: %v",
						attempt, processingErr,
					)
				}

				if mutationErr != nil {
					t.Fatalf(
						"attempt %d: mutation: %v",
						attempt, mutationErr,
					)
				}

				stored, err := repo.ListByUser("user-1")
				if err != nil || len(stored) > 1 {
					t.Fatal("duplicate events or repository  error")
				}

				eligible, err := processor.ListEligible("user-1")
				if err != nil {
					t.Fatal(err)
				}

				if len(eligible) != 0 {
					t.Fatalf(
						"attempt %d: stale event is eligible",
						attempt,
					)
				}

				metadata, err := capsules.GetMetadata(
					"user-1", c.ID,
				)

				if action == "open" {
					if err != nil ||
						metadata.Status != capsule.StatusReturned {
						t.Fatal("capsule was not opened")
					}
				} else if !errors.Is(
					err, capsule.ErrCapsuleNotFound,
				) {
					t.Fatal("deleted capsule still exists")
				}
			}
		})

	}
}

type mutationOnInsertRepository struct {
	Repository
	beforeInsert func() error
}

func (r *mutationOnInsertRepository) CreateIfAbsent(
	event Event,
) (bool, error) {
	if r.beforeInsert != nil {
		hook := r.beforeInsert
		r.beforeInsert = nil

		if err := hook(); err != nil {
			return false, err
		}
	}

	return r.Repository.CreateIfAbsent(event)
}

func TestProcessDueStaleSnapshotAfterDeletion(t *testing.T) {
	now := time.Date(
		2026, 10, 19, 0, 0, 0, 0, time.UTC,
	)

	capsules := capsule.NewService(
		capsule.NewMemoryRepository(),
		func() time.Time { return now },
	)

	c, err := capsules.Seal("user-1", capsule.SealInput{
		SourceEntryID: "entry-1",
		Body:          "Delete before event insertion",
		UnlockAt:      now.Add(time.Hour),
	})

	if err != nil {
		t.Fatal(err)
	}

	now = now.Add(2 * time.Hour)

	baseRepo := NewMemoryRepository()

	repo := &mutationOnInsertRepository{
		Repository: baseRepo,
		beforeInsert: func() error {
			return capsules.Destroy("user-1", c.ID)
		},
	}

	processor := NewProcessor(
		capsules,
		repo,
		func() time.Time { return now },
	)

	created, err := processor.ProcessDue("user-1")
	if err != nil {
		t.Fatal(err)
	}

	if created != 1 {
		t.Fatalf(
			"expected one historical event, got %d",
			created,
		)
	}

	stored, err := baseRepo.ListByUser("user-1")
	if err != nil || len(stored) != 1 {
		t.Fatal("expected one retained event record")
	}

	eligible, err := processor.ListEligible("user-1")
	if err != nil {
		t.Fatal(err)
	}

	if len(eligible) != 0 {
		t.Fatal("event for deleted capsule remained eligible")
	}

	_, err = capsules.GetMetadata("user-1", c.ID)
	if !errors.Is(err, capsule.ErrCapsuleNotFound) {
		t.Fatal("test did not delete the capsule")
	}
}
