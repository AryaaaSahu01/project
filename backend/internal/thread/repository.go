package thread

import "errors"

var ErrReflectionAlreadyExists = errors.New(
	"reflection already exists",
)

type Repository interface {
	Create(reflection Reflection) (Reflection, error)

	ListByCapsule(
		userID string,
		capsuleID string,
	) ([]Reflection, error)
}
