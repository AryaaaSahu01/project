package capsule

import "sync"

type MemoryRepository struct {
	mu       sync.RWMutex
	capsules map[string]Capsule
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		capsules: make(map[string]Capsule),
	}
}

func (r *MemoryRepository) Create(c Capsule) (Capsule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.capsules[c.ID]; exists {
		return Capsule{}, ErrCapsuleAlreadyExists
	}

	if c.ParentCapsuleID != "" {
		parent, exists := r.capsules[c.ParentCapsuleID]

		if !exists || parent.UserID != c.UserID {
			return Capsule{}, ErrCapsuleNotFound
		}

		if parent.Status != StatusReturned {
			return Capsule{}, ErrCapsuleParentNotReturned
		}
	}
	r.capsules[c.ID] = c
	return c, nil
}

func (r *MemoryRepository) GetByID(
	userID string,
	capsuleID string,
) (Capsule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, exists := r.capsules[capsuleID]

	if !exists || c.UserID != userID {
		return Capsule{}, ErrCapsuleNotFound
	}

	return c, nil
}

func (r *MemoryRepository) Update(c Capsule) (Capsule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.capsules[c.ID]

	if !exists || existing.UserID != c.UserID {
		return Capsule{}, ErrCapsuleNotFound
	}

	r.capsules[c.ID] = c
	return c, nil
}

func (r *MemoryRepository) Delete(
	userID string,
	capsuleID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, exists := r.capsules[capsuleID]

	if !exists || c.UserID != userID {
		return ErrCapsuleNotFound
	}
	for _, child := range r.capsules {
		if child.ParentCapsuleID == capsuleID {
			return ErrCapsuleHasChildren
		}
	}
	delete(r.capsules, capsuleID)
	return nil

}

func (r *MemoryRepository) ListByUser(
	userID string,
) ([]Capsule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	capsules := make([]Capsule, 0)

	for _, c := range r.capsules {
		if c.UserID == userID {
			capsules = append(capsules, c)
		}
	}
	return capsules, nil
}
