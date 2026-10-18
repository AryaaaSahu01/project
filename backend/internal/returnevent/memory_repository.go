package returnevent

import (
	"sort"
	"sync"
)

type Repository interface {
	CreateIfAbsent(event Event) (bool, error)
	ListByUser(userID string) ([]Event, error)
}

type eventKey struct {
	UserID    string
	CapsuleID string
}

type MemoryRepository struct {
	mu     sync.RWMutex
	events map[eventKey]Event
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		events: make(map[eventKey]Event),
	}
}

func (r *MemoryRepository) CreateIfAbsent(
	event Event,
) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := eventKey{
		UserID:    event.UserID,
		CapsuleID: event.CapsuleID,
	}

	if _, exists := r.events[key]; exists {
		return false, nil
	}

	r.events[key] = event

	return true, nil
}

func (r *MemoryRepository) ListByUser(
	userID string,
) ([]Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]Event, 0)

	for _, event := range r.events {
		if event.UserID == userID {
			result = append(result, event)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].UnlockAt.Equal(result[j].UnlockAt) {
			return result[i].ID < result[j].ID
		}

		return result[i].UnlockAt.Before(result[j].UnlockAt)
	})

	return result, nil
}
