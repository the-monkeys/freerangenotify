package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"mime"
	"strings"
)

func UploadTransportLimit(maxFileBytes int64) int {
	if maxFileBytes <= 0 || maxFileBytes > services.DefaultMaxFileBytes {
		maxFileBytes = services.DefaultMaxFileBytes
	}
	limit := maxFileBytes + (1 << 20)
	if limit < 16<<20 {
		limit = 16 << 20
	}
	return int(limit)
}

// IsFileUploadRequest identifies only the supported multipart upload route.
// The server defers that route's ordinary body guard until API authentication.
func IsFileUploadRequest(c *fiber.Ctx) bool {
	path := strings.TrimSuffix(c.Path(), "/")
	kind, _, err := mime.ParseMediaType(c.Get(fiber.HeaderContentType))
	return err == nil && kind == "multipart/form-data" && c.Method() == fiber.MethodPost && path == "/v1/files"
}

func NonUploadBodyLimit(limit int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		appID, _ := c.Locals("app_id").(string)
		if appID != "" && IsFileUploadRequest(c) {
			return c.Next()
		}
		if len(c.Body()) > limit {
			return fiber.ErrRequestEntityTooLarge
		}
		return c.Next()
	}
}
