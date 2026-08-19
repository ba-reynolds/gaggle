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
	SessionID      uuid.UUID  `json:"session_id"`
	UserID         int        `json:"user_id"`
	TokenHash      string     `json:"token_hash"`
	IssuedAt       time.Time  `json:"issued_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	Revoked        bool       `json:"revoked"`
	RevokedAt      *time.Time `json:"revoked_at"`
	RevokedReason  *string    `json:"revoked_reason,omitempty"`
	UserAgent      string     `json:"user_agent"`
	IPAddress      string     `json:"ip_address"`
}

// Reasons recorded on revoked refresh tokens.
var (
	RevokedReasonRotated = "rotated" // replaced by a newer token in the same session family
	RevokedReasonLogout  = "logout"  // the user ended the session explicitly
	RevokedReasonTheft   = "theft"   // a rotated token was replayed; session is being killed
)

type LoginRequest struct {
	Identifier string `json:"identifier" validate:"required,min=3,max=96"`
	Password   string `json:"password" validate:"required,max=72"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
}

type RefreshTokenResponse struct {
	AccessToken string `json:"access_token"`
}

type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=16,regexp=^[a-zA-Z0-9_]+$"`
	Email    string `json:"email" validate:"required,email,max=96"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type RegisterResponse struct {
	User        *User  `json:"user"`
	AccessToken string `json:"access_token"`
}
