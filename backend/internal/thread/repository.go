package thread

import "errors"

var (
	ErrReflectionAlreadyExists = errors.New(
		"reflection already exists",
	)

	ErrReflectionNotFound = errors.New(
		"reflection not found",
	)
)

type Repository interface {
	Create(reflection Reflection) (Reflection, error)

	GetByID(
		userID string,
		reflectionID string,
	) (Reflection, error)

	Update(reflection Reflection) (Reflection, error)

	ListByCapsule(
		userID string,
		capsuleID string,
	) ([]Reflection, error)
}
