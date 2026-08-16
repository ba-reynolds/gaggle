package util

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// PaginationCursor represents a cursor for pagination
type PaginationCursor struct {
	ID        interface{} `json:"id"`
	Timestamp string      `json:"timestamp,omitempty"`
	Order     string      `json:"order,omitempty"` // "asc" or "desc"
}

// EncodeCursor encodes a cursor to a base64 string
func EncodeCursor(cursor PaginationCursor) (string, error) {
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cursor: %w", err)
	}
	return base64.URLEncoding.EncodeToString(data), nil
}

// DecodeCursor decodes a base64 string to a cursor
func DecodeCursor(cursorStr string) (*PaginationCursor, error) {
	if cursorStr == "" {
		return nil, nil
	}

	data, err := base64.URLEncoding.DecodeString(cursorStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cursor: %w", err)
	}

	var cursor PaginationCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cursor: %w", err)
	}

	return &cursor, nil
}

// CreateIDCursor creates a simple cursor with just an ID
func CreateIDCursor(id interface{}) (*PaginationCursor, error) {
	return &PaginationCursor{
		ID:    id,
		Order: "desc", // Default order
	}, nil
}

// CreateTimestampCursor creates a cursor with ID and timestamp
func CreateTimestampCursor(id interface{}, timestamp string) (*PaginationCursor, error) {
	return &PaginationCursor{
		ID:        id,
		Timestamp: timestamp,
		Order:     "desc", // Default order
	}, nil
}

// ParsePaginationParams parses pagination parameters from query string
func ParsePaginationParams(limitStr, cursorStr string, defaultLimit, maxLimit int) (limit int, cursor string, err error) {
	// Parse limit
	if limitStr == "" {
		limit = defaultLimit
	} else {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit <= 0 {
			limit = defaultLimit
		} else if parsedLimit > maxLimit {
			limit = maxLimit
		} else {
			limit = parsedLimit
		}
	}

	// Parse cursor
	cursor = strings.TrimSpace(cursorStr)

	return limit, cursor, nil
}

// ValidateAndNormalizeLimit validates and normalizes pagination limits
func ValidateAndNormalizeLimit(limit, defaultLimit, maxLimit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}
