package journal

import "sync"

type MemoryRepository struct {
	mu      sync.RWMutex
	entries []Entry
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		entries: make([]Entry, 0),
	}
}

func (r *MemoryRepository) Create(entry Entry) (Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = append(r.entries, entry)

	return entry, nil
}

func (r *MemoryRepository) ListByUser(userID string) ([]Entry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]Entry, 0)

	for _, entry := range r.entries {
		if entry.UserID == userID {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}
