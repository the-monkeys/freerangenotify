package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Allowed MIME types for media upload (Twilio-compatible).
var allowedMediaTypes = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/gif":       ".gif",
	"image/webp":      ".webp",
	"application/pdf": ".pdf",
	"video/mp4":       ".mp4",
}

const (
	maxMediaSize = 16 << 20 // 16 MB — Twilio WhatsApp limit
	uploadDir    = "./uploads/media"
)

// MediaHandler handles file upload and serving for WhatsApp media.
type MediaHandler struct {
	publicURL string
	logger    *zap.Logger
}

// NewMediaHandler creates a new MediaHandler.
func NewMediaHandler(publicURL string, logger *zap.Logger) *MediaHandler {
	// Ensure upload directory exists
	if err := os.MkdirAll(uploadDir, 0o750); err != nil {
		logger.Error("Failed to create upload directory", zap.Error(err))
	}
	return &MediaHandler{
		publicURL: strings.TrimRight(publicURL, "/"),
		logger:    logger,
	}
}

// Upload handles POST /v1/media/upload (multipart/form-data with "file" field).
func (h *MediaHandler) Upload(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "missing or invalid 'file' field in multipart form",
		})
	}

	// Validate size
	if file.Size > maxMediaSize {
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
			"error": fmt.Sprintf("file too large: %d bytes (max %d bytes)", file.Size, maxMediaSize),
		})
	}

	// Validate MIME type
	contentType := file.Header.Get("Content-Type")
	ext, ok := allowedMediaTypes[contentType]
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":         fmt.Sprintf("unsupported file type: %s", contentType),
			"allowed_types": []string{"image/jpeg", "image/png", "image/gif", "image/webp", "application/pdf", "video/mp4"},
		})
	}

	// Generate unique filename: timestamp-uuid.ext
	filename := fmt.Sprintf("%d-%s%s", time.Now().UnixMilli(), uuid.New().String()[:8], ext)
	savePath := filepath.Join(uploadDir, filename)

	if err := c.SaveFile(file, savePath); err != nil {
		h.logger.Error("Failed to save uploaded file", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to save file",
		})
	}

	publicURL := fmt.Sprintf("%s/media/%s", h.publicURL, filename)

	h.logger.Info("Media uploaded",
		zap.String("filename", filename),
		zap.String("content_type", contentType),
		zap.Int64("size", file.Size),
		zap.String("url", publicURL),
	)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"url":          publicURL,
		"filename":     filename,
		"content_type": contentType,
		"size":         file.Size,
	})
}
