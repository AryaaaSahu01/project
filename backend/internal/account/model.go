package account

import (
	"errors"
	"time"
)

var (
	ErrInvalidInput      = errors.New("invalid account details")
	ErrEmailTaken        = errors.New("email already registered")
	ErrInvalidCredential = errors.New("invalid email or password")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrNotFound          = errors.New("account not found")
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Profile struct {
	UserID      string    `json:"user_id"`
	DisplayName string    `json:"display_name"`
	Bio         string    `json:"bio"`
	Timezone    string    `json:"timezone"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	Role string `json:"role"`
}

type Account struct {
	User      User       `json:"user"`
	Profile   Profile    `json:"profile"`
	Workspace *Workspace `json:"workspace,omitempty"`
}

type Session struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
	Account     Account   `json:"account"`
}

type RegisterInput struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UpdateProfileInput struct {
	DisplayName *string `json:"display_name,omitempty"`
	Bio         *string `json:"bio,omitempty"`
	Timezone    *string `json:"timezone,omitempty"`
}
