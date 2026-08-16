package models

import (
	"time"

	"github.com/google/uuid"
)

// Media represents a file uploaded by the user
// Its bytes are stored on disk under mediaDir with the UUID as filename
// and its metadata persisted in the media table.
type Media struct {
	UUID      uuid.UUID
	MimeType  string
	Filename  string
	CreatedAt time.Time
}

// MediaUploadResponse is the response from the media upload endpoint
type MediaUploadResponse struct {
	UUIDs []uuid.UUID `json:"uuids"`
}
