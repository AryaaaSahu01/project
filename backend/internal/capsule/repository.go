package capsule

import "errors"

var ErrCapsuleNotFound = errors.New("capsule not found")
var ErrCapsuleAlreadyExists = errors.New("capsule already exists")
var ErrCapsuleParentNotReturned = errors.New("parent capsule must be returned")
var ErrCapsuleHasChildren = errors.New("capsule has child capsule")
var ErrCapsuleLinkedToReflection = errors.New("capsule is linked to a sealed reflection")

type Repository interface {
	Create(c Capsule) (Capsule, error)
	GetByID(userID string, capsuleID string) (Capsule, error)
	Update(c Capsule) (Capsule, error)
	Delete(userID string, capsuleID string) error
	ListByUser(userID string) ([]Capsule, error)
}
