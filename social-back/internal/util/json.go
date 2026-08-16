package util

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/ba-reynolds/vitrilium/internal/apperrors"
	"github.com/ba-reynolds/vitrilium/internal/models"
	"github.com/go-playground/validator/v10"
)

// RespondWithJson sends a JSON response with the given status code and data,
// wrapped in a consistent envelope: {"data": ..., "error": null}.
// Every successful response shares this shape so the frontend has a single contract.
func RespondWithJson(w http.ResponseWriter, status int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	envelope := models.Envelope{
		Data:  data,
		Error: nil,
	}
	return json.NewEncoder(w).Encode(envelope)
}

// RespondWithAppError sends a JSON error response using the AppError structure
func RespondWithAppError(w http.ResponseWriter, appErr *apperrors.AppError) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.Status)

	envelope := models.Envelope{
		Data:  nil,
		Error: appErr,
	}

	return json.NewEncoder(w).Encode(envelope)
}

// NewEnvelope creates a new envelope with data
func NewEnvelope(data interface{}) models.Envelope {
	return models.Envelope{
		Data:  data,
		Error: nil,
	}
}

func ReadJSON(r *http.Request, data any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(data)
	if err != nil {
		slog.Error("failed to decode json payload", "error", err)
	}
	return err
}

var Validate *validator.Validate

// Special go function, executes by itself
func init() {
	Validate = validator.New(validator.WithRequiredStructEnabled())
}

var hexColorRegex = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// IsHexColor returns true if s is a valid hex color string (e.g., #1DA1F2)
func IsHexColor(s string) bool {
	return hexColorRegex.MatchString(s)
}
