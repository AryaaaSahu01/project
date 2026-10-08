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

func (r *MemoryRepository) GetByID(
	userID string,
	entryID string,
) (Entry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, entry := range r.entries {
		if entry.ID == entryID && entry.UserID == userID {
			return entry, nil
		}
	}
	return Entry{}, ErrEntryNotFound
}

func (r *MemoryRepository) Update(entry Entry) (Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.entries {
		if r.entries[i].ID == entry.ID &&
			r.entries[i].UserID == entry.UserID {
			r.entries[i] = entry
			return entry, nil
		}
	}
	return Entry{}, ErrEntryNotFound
}

func (r *MemoryRepository) Delete(
	userID string,
	entryID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, entry := range r.entries {
		if entry.ID == entryID && entry.UserID == userID {
			copy(r.entries[i:], r.entries[i+1:])

			last := len(r.entries) - 1
			r.entries[last] = Entry{}
			r.entries = r.entries[:last]

			return nil
		}
	}
	return ErrEntryNotFound
}
