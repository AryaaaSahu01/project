package thread

import "sync"

type MemoryRepository struct {
	mu          sync.RWMutex
	reflections map[string]Reflection
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		reflections: make(map[string]Reflection),
	}
}

func (r *MemoryRepository) Create(
	reflection Reflection,
) (Reflection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.reflections[reflection.ID]; exists {
		return Reflection{}, ErrReflectionAlreadyExists
	}

	r.reflections[reflection.ID] = reflection
	return reflection, nil
}

func (r *MemoryRepository) ListByCapsule(
	userID string,
	capsuleID string,
) ([]Reflection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Reflection, 0)

	for _, reflection := range r.reflections {
		if reflection.UserID == userID &&
			reflection.CapsuleID == capsuleID {
			result = append(result, reflection)
		}
	}
	return result, nil
}

func (r *MemoryRepository) GetByID(
	userID string,
	reflectionID string,
) (Reflection, error) {

	r.mu.RLock()
	defer r.mu.RUnlock()

	reflection, exists := r.reflections[reflectionID]

	if !exists || reflection.UserID != userID {
		return Reflection{}, ErrReflectionNotFound
	}

	return reflection, nil
}

func (r *MemoryRepository) Update(
	reflection Reflection,
) (Reflection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.reflections[reflection.ID]

	if !exists ||
		existing.UserID != reflection.UserID ||
		existing.CapsuleID != reflection.CapsuleID {
		return Reflection{}, ErrReflectionNotFound
	}

	r.reflections[reflection.ID] = reflection

	return reflection, nil
}
