package thread

import "time"

type Status string

const (
	StatusDraft  Status = "draft"
	StatusSealed Status = "sealed"
)

type Reflection struct {
	ID              string    `json:"id"`
	UserID          string    `json:"-"`
	CapsuleID       string    `json:"capsule_id"`
	Body            string    `json:"body"`
	Status          Status    `json:"status"`
	SealedCapsuleID string    `json:"sealed_capsule_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type CreateReflectionInput struct {
	Body string `json:"body"`
}
