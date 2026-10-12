package thread

import "time"

type Reflection struct {
	ID        string    `json:"id"`
	UserID    string    `json:"-"`
	CapsuleID string    `json:"capsule_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateReflectionInput struct {
	Body string `json:"body"`
}
