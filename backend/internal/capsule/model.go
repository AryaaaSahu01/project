package capsule

import "time"

type Status string

const (
	StatusSealed    Status = "sealed"
	StatusReturned  Status = "returned"
	StatusDestroyed Status = "destroyed"
)

type Capsule struct {
	ID              string
	UserID          string
	SourceEntryID   string
	SealedAt        time.Time
	UnlockAt        time.Time
	Status          Status
	ParentCapsuleID string

	title string
	body  string
}

type OpenedCapsule struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Body     string    `json:"body"`
	SealedAt time.Time `json:"sealed_at"`
	UnlockAt time.Time `json:"unlock_at"`
	CanOpen  bool      `json:"can_open"`
}

type CapsuleMetadata struct {
	ID            string    `json:"id"`
	SourceEntryID string    `json:"source_entry_id"`
	Status        Status    `json:"status"`
	SealedAt      time.Time `json:"sealed_at"`
	UnlockAt      time.Time `json:"unlock_at"`
	CanOpen       bool      `json:"can_open"`
}
