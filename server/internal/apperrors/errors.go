package apperrors

import (
	"errors"
	"fmt"
	"net/http"
)

// Error codes
const (
	BadRequest      = "BAD_REQUEST"
	Unauthorized    = "UNAUTHORIZED"
	Forbidden       = "FORBIDDEN"
	NotFound        = "NOT_FOUND"
	AlreadyExists   = "ALREADY_EXISTS"
	Validation      = "VALIDATION"
	InternalServer  = "INTERNAL_SERVER"
	InvalidToken    = "INVALID_TOKEN"
	UsernameExists  = "USERNAME_EXISTS"
	EmailExists     = "EMAIL_EXISTS"
	TooManyRequests = "TOO_MANY_REQUESTS"
)

// AppError represents a standardized application error
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
	Status  int    `json:"-"`
	Err     error  `json:"-"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Err.Error())
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Err
}

// BadRequestError creates a new bad request error
func BadRequestError(message string, err error) *AppError {
	return &AppError{
		Code:    BadRequest,
		Message: message,
		Status:  http.StatusBadRequest,
		Err:     err,
	}
}

// UnauthorizedError creates a new unauthorized error
func UnauthorizedError(message string, err error) *AppError {
	return &AppError{
		Code:    Unauthorized,
		Message: message,
		Status:  http.StatusUnauthorized,
		Err:     err,
	}
}

// ForbiddenError creates a new forbidden error
func ForbiddenError(message string, err error) *AppError {
	return &AppError{
		Code:    Forbidden,
		Message: message,
		Status:  http.StatusForbidden,
		Err:     err,
	}
}

// NotFoundError creates a new not found error
func NotFoundError(message string, err error) *AppError {
	return &AppError{
		Code:    NotFound,
		Message: message,
		Status:  http.StatusNotFound,
		Err:     err,
	}
}

// AlreadyExistsError creates a new already exists error
func AlreadyExistsError(message string, err error) *AppError {
	return &AppError{
		Code:    AlreadyExists,
		Message: message,
		Status:  http.StatusConflict,
		Err:     err,
	}
}

// UsernameExistsError creates a new username already exists error
func UsernameExistsError(message string, err error) *AppError {
	return &AppError{
		Code:    UsernameExists,
		Message: message,
		Status:  http.StatusConflict,
		Err:     err,
	}
}

// EmailExistsError creates a new email already exists error
func EmailExistsError(message string, err error) *AppError {
	return &AppError{
		Code:    EmailExists,
		Message: message,
		Status:  http.StatusConflict,
		Err:     err,
	}
}

// ValidationError creates a new validation error
func ValidationError(message string, err error) *AppError {
	return &AppError{
		Code:    Validation,
		Message: message,
		Status:  http.StatusBadRequest,
		Err:     err,
	}
}

// InternalServerError creates a new internal server error
func InternalServerError(err error) *AppError {
	return &AppError{
		Code:    InternalServer,
		Message: "internal server error",
		Status:  http.StatusInternalServerError,
		Err:     err,
	}
}

// InvalidTokenError creates a new invalid token error
func InvalidTokenError(message string, err error) *AppError {
	return &AppError{
		Code:    InvalidToken,
		Message: message,
		Status:  http.StatusUnauthorized,
		Err:     err,
	}
}

// PayloadValidationError creates a new payload validation error
func PayloadValidationError(err error) *AppError {
	return &AppError{
		Code:    Validation,
		Message: "invalid request payload",
		Status:  http.StatusBadRequest,
		Err:     err,
	}
}

// TooManyRequestsError creates a new rate limit error
func TooManyRequestsError() *AppError {
	return &AppError{
		Code:    TooManyRequests,
		Message: "rate limit exceeded, please try again later",
		Status:  http.StatusTooManyRequests,
	}
}

// Is reports whether err is an AppError with the given code.
// Handlers/services use this instead of comparing error values (which fails
// for pointer-typed AppErrors constructed at different call sites).
func Is(err error, code string) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}
