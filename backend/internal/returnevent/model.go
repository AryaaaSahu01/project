package returnevent

import "time"

const TypeCapsuleDue = "capsule_due"

type Event struct {
	ID         string    `json:"id"`
	UserID     string    `json:"-"`
	CapsuleID  string    `json:"capsule_id"`
	Type       string    `json:"type"`
	UnlockAt   time.Time `json:"unlock_at"`
	RecordedAt time.Time `json:"recorded_at"`
}
