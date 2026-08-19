package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/models"
	"github.com/ba-reynolds/gaggle/internal/service"
	"github.com/ba-reynolds/gaggle/internal/util"
	"github.com/google/uuid"
)

// MediaHandler handles HTTP requests for media operations
type MediaHandler struct {
	service *service.Service
	logger  *slog.Logger
}

func NewMediaHandler(service *service.Service, logger *slog.Logger) *MediaHandler {
	return &MediaHandler{
		service: service,
		logger:  logger,
	}
}

// UploadMedia godoc
//
//	@Summary		Upload media
//	@Description	Upload media
//	@Tags			media
//	@Accept			multipart/form-data
//	@Param			media	formData	file	true	"Media"
//	@Success		201		{object}	models.Envelope{data=models.MediaUploadResponse,error=nil}
//	@Failure		400		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/media/upload [post]
func (h *MediaHandler) UploadMedia(w http.ResponseWriter, r *http.Request) {
	// limit total request size (e.g. 32 MB)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		// Log HTTP layer errors - these are request parsing concerns
		h.logger.Error("failed to parse multipart form",
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
		)
		util.RespondWithAppError(w, apperrors.BadRequestError("request too large", err))
		return
	}

	files := r.MultipartForm.File["media"]
	if len(files) == 0 {
		// Log validation errors - these are HTTP layer concerns
		h.logger.Warn("no files uploaded",
			"path", r.URL.Path,
		)
		util.RespondWithAppError(w, apperrors.BadRequestError("no files uploaded", nil))
		return
	}
	if len(files) > 4 {
		// Log validation errors - these are HTTP layer concerns
		h.logger.Warn("too many files uploaded",
			"count", len(files),
			"path", r.URL.Path,
		)
		util.RespondWithAppError(w, apperrors.BadRequestError("too many files uploaded", nil))
		return
	}

	// Log the upload request for debugging
	h.logger.Debug("upload media request",
		"fileCount", len(files),
		"path", r.URL.Path,
	)

	ctx := r.Context()
	var uuids []uuid.UUID

	// Process each file
	for _, fh := range files {
		f, errOpen := fh.Open()
		if errOpen != nil {
			// Log file handling errors - these are HTTP layer concerns
			h.logger.Error("failed to open uploaded file",
				"filename", fh.Filename,
				"contentType", fh.Header.Get("Content-Type"),
				"error", errOpen,
			)
			util.RespondWithAppError(w, apperrors.InternalServerError(errOpen))
			return
		}

		mediaUUID := uuid.New()

		// Create media object
		media := &models.Media{
			UUID:     mediaUUID,
			MimeType: fh.Header.Get("Content-Type"),
			Filename: fh.Filename,
		}

		// Use the service to save the media
		if err := h.service.Media.Create(ctx, media, f); err != nil {
			f.Close()
			// Don't log service errors - they're already logged at appropriate layer
			// Just handle HTTP response mapping
			if appErr, ok := err.(*apperrors.AppError); ok {
				util.RespondWithAppError(w, appErr)
				return
			}
			util.RespondWithAppError(w, apperrors.InternalServerError(err))
			return
		}
		f.Close()

		// Record successful writes
		uuids = append(uuids, mediaUUID)
	}

	if err := util.RespondWithJson(w, http.StatusCreated, models.MediaUploadResponse{UUIDs: uuids}); err != nil {
		// Log HTTP response errors - these are HTTP layer concerns
		h.logger.Error("failed to write HTTP response",
			"error", err,
			"status", http.StatusCreated,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
}

// GetMediaByID godoc
//
//	@Summary		Get media by ID
//	@Description	Returns the actual media file by UUID
//	@Tags			media
//	@Produce		octet-stream
//	@Param			uuid	path	string	true	"Media UUID"
//	@Success		200		{file}	binary
//	@Failure		400		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		404		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Failure		500		{object}	models.Envelope{data=nil,error=apperrors.AppError}
//	@Security		ApiKeyAuth
//	@Router			/media/{uuid} [get]
func (h *MediaHandler) GetMediaByID(w http.ResponseWriter, r *http.Request) {
	// Parse UUID from URL
	mediaUUID, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		// Log parameter parsing errors - these are HTTP layer concerns
		h.logger.Warn("invalid media UUID parameter",
			"uuid", r.PathValue("uuid"),
			"error", err,
		)
		util.RespondWithAppError(w, apperrors.BadRequestError("invalid media UUID", err))
		return
	}

	// Log the get media request for debugging
	h.logger.Debug("get media request",
		"mediaUUID", mediaUUID,
		"path", r.URL.Path,
	)

	// Get media metadata from database
	ctx := r.Context()
	media, err := h.service.Media.GetByID(ctx, mediaUUID)
	if err != nil {
		// Don't log service errors - they're already logged at appropriate layer
		// Just handle HTTP response mapping
		if appErr, ok := err.(*apperrors.AppError); ok {
			util.RespondWithAppError(w, appErr)
			return
		}
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}

	// Get file path
	mediaPath := filepath.Join(h.service.Config.MediaDir, mediaUUID.String())

	// Open the file
	file, err := os.Open(mediaPath)
	if err != nil {
		// Log file system errors - these are HTTP layer concerns
		h.logger.Error("failed to open media file",
			"mediaUUID", mediaUUID,
			"path", mediaPath,
			"error", err,
		)
		util.RespondWithAppError(w, apperrors.InternalServerError(err))
		return
	}
	defer file.Close()

	// Set content type header based on the stored mime type
	w.Header().Set("Content-Type", media.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%s", media.Filename))

	// Stream file to response
	http.ServeContent(w, r, media.Filename, media.CreatedAt, file)
}
