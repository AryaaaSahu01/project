package capsule

import "errors"

var ErrCapsuleNotFound = errors.New("capsule not found")
var ErrCapsuleAlreadyExists = errors.New("capsule already exists")
var ErrCapsuleParentNotReturned = errors.New("parent capsule must be returned")
var ErrCapsuleHasChildren = errors.New("capsule has child capsule")
var ErrCapsuleLinkedToReflection = errors.New("capsule is linked to a sealed reflection")
var ErrCapsuleNotRoot = errors.New(
	"only a root capsule can be deleted as a conversation",
)
var ErrCapsuleTreeChanged = errors.New("capsule tree changed during deletion")

type Repository interface {
	Create(c Capsule) (Capsule, error)
	GetByID(userID string, capsuleID string) (Capsule, error)
	Update(c Capsule) (Capsule, error)
	Delete(userID string, capsuleID string) error
	ListByUser(userID string) ([]Capsule, error)
	DeleteTree(userID string, rootID string, expectedIDs []string) error
}
