package models

import "github.com/ba-reynolds/vitrilium/internal/apperrors"

// EnvelopeError is now replaced by AppError for consistency
// type EnvelopeError struct {
// 	ErrorType string `json:"error_type"`
// 	Details   string `json:"details,omitempty"`
// }

type Envelope struct {
	Data  any                 `json:"data"`
	Error *apperrors.AppError `json:"error"`
}
