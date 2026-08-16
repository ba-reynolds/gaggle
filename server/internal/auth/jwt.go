package auth

import (
	"fmt"
	"time"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/models"
	"github.com/ba-reynolds/gophersocial/pkg/config"
	"github.com/golang-jwt/jwt/v5"
)

type JWTAuthenticator struct {
	secret             string
	issuer             string
	accessTokenExpiry  time.Duration
	refreshTokenExpiry time.Duration
}

// TokenType defines the type of JWT token
type TokenType string

const (
	// AccessToken is a short-lived token used for API access
	AccessToken TokenType = "access"
	// RefreshToken is a long-lived token used to obtain new access tokens
	RefreshToken TokenType = "refresh"
)

func NewJWTAuthenticator(cfg config.AuthConfig) *JWTAuthenticator {
	return &JWTAuthenticator{
		secret:             cfg.JWTSecret,
		issuer:             cfg.JWTIssuer,
		accessTokenExpiry:  cfg.JWTAccessTokenExpirationTime,
		refreshTokenExpiry: cfg.JWTRefreshTokenExpirationTime,
	}
}

func (a *JWTAuthenticator) GenerateAccessToken(userID int) (*models.Token, error) {
	return a.generateToken(userID, AccessToken)
}

func (a *JWTAuthenticator) GenerateRefreshToken(userID int) (*models.Token, error) {
	return a.generateToken(userID, RefreshToken)
}

func (a *JWTAuthenticator) generateToken(userID int, tokenType TokenType) (*models.Token, error) {
	now := time.Now()
	var exp time.Time
	if tokenType == AccessToken {
		exp = now.Add(a.accessTokenExpiry)
	} else {
		exp = now.Add(a.refreshTokenExpiry)
	}

	claims := jwt.MapClaims{
		"sub": userID,
		"exp": exp.Unix(),
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"iss": a.issuer,
		"aud": a.issuer,
		"typ": string(tokenType),
	}

	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(a.secret))
	if err != nil {
		return nil, apperrors.InternalServerError(fmt.Errorf("failed to sign token: %w", err))
	}

	return &models.Token{
		TokenString: tokenString,
		IssuedAt:    now,
		ExpiresAt:   exp,
	}, nil
}

func (a *JWTAuthenticator) ValidateToken(tokenString string, tokenType TokenType) (*jwt.Token, error) {
	// `jwt.Parse` uses the "key function" (the one we pass in to the function)
	// to parse the token. `jwt.Parse` expects the key function to return the
	// private key that was used to sign the token.
	// Once it has the private key, it can verify that the jwt wasn't tampered
	// with, that the signing method is correct, and that the claims are valid.

	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		return []byte(a.secret), nil
	},
		jwt.WithExpirationRequired(),
		jwt.WithAudience(a.issuer),
		jwt.WithIssuer(a.issuer),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
	)

	if err != nil {
		return nil, apperrors.InvalidTokenError("token validation failed", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, apperrors.InvalidTokenError("invalid claims type", nil)
	}

	typ, ok := claims["typ"].(string)
	if !ok {
		return nil, apperrors.InvalidTokenError("token missing 'typ' claim", nil)
	}

	if TokenType(typ) != tokenType {
		return nil, apperrors.InvalidTokenError(fmt.Sprintf("token type mismatch: expected %s, got %s", tokenType, typ), nil)
	}

	return token, nil
}

// GetUserIDFromToken extracts the user ID from a validated token
func (a *JWTAuthenticator) GetUserIDFromToken(token *jwt.Token) (int, error) {
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, apperrors.InvalidTokenError("invalid token claims", nil)
	}

	// values are unmarshaled from JSON, so they are of type float64
	subFloat, ok := claims["sub"].(float64)
	if !ok {
		return 0, apperrors.InvalidTokenError("invalid user ID in token", nil)
	}

	return int(subFloat), nil
}
