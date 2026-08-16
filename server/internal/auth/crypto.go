package auth

import (
	"crypto/sha256"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	// convert from array to slice
	return hex.EncodeToString(hash[:])
}

func HashPassword(password string) (string, error) {
	// Bcrypt randomly generates a salt, the salt is later stored along with
	// hash all in one string, meaning that you don't need to separately manage
	// it
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}
