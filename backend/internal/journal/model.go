package journal

import "time"

type Entry struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateEntryInput struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type UpdateEntryInput struct {
	Title *string `json:"title"`
	Body  *string `json:"body"`
}
