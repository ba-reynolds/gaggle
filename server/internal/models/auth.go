package models

import (
	"time"

	"github.com/google/uuid"
)

// Refers to either an access or refresh token
// Used as a struct through the app for convenience
// Never really sent to the client
type Token struct {
	TokenString string
	IssuedAt    time.Time
	ExpiresAt   time.Time
}

type RefreshToken struct {
	RefreshTokenID uuid.UUID  `json:"refresh_token_id"`
	UserID         int        `json:"user_id"`
	TokenHash      string     `json:"token_hash"`
	IssuedAt       time.Time  `json:"issued_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	Revoked        bool       `json:"revoked"`
	RevokedAt      *time.Time `json:"revoked_at"`
	UserAgent      string     `json:"user_agent"`
	IPAddress      string     `json:"ip_address"`
}

type LoginRequest struct {
	Identifier string `json:"identifier" validate:"required"`
	Password   string `json:"password"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
}

type RefreshTokenResponse struct {
	AccessToken string `json:"access_token"`
}

type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=16"`
	Email    string `json:"email" validate:"required,email,max=96"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type RegisterResponse struct {
	User        *User  `json:"user"`
	AccessToken string `json:"access_token"`
}
