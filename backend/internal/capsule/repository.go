package capsule

import "errors"

var ErrCapsuleNotFound = errors.New("capsule not found")
var ErrCapsuleAlreadyExists = errors.New("capsule already exists")

type Repository interface {
	Create(c Capsule) (Capsule, error)
	GetByID(userID string, capsuleID string) (Capsule, error)
	Update(c Capsule) (Capsule, error)
	Delete(userID string, capsuleID string) error
	ListByUser(userID string) ([]Capsule, error)
}
