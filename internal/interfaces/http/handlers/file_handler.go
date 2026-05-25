package handlers

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	domainfile "github.com/the-monkeys/freerangenotify/internal/domain/file"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/filestore"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
)

// FileHandler exposes the /v1/files API: multipart upload, metadata get,
// listing, deletion, authenticated streaming download, and signed-URL minting
// for off-platform consumers. The signed-URL download path is registered as a
// public route and verifies the signature itself.
type FileHandler struct {
	svc       *services.FileService
	signer    *filestore.Signer
	publicURL string
	logger    *zap.Logger
}

// NewFileHandler wires the handler. publicURL is the externally reachable
// origin used to build signed download URLs; when empty, the signed-URL
// endpoint returns a relative path that the caller must join themselves.
func NewFileHandler(svc *services.FileService, signer *filestore.Signer, publicURL string, logger *zap.Logger) *FileHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &FileHandler{
		svc:       svc,
		signer:    signer,
		publicURL: strings.TrimRight(publicURL, "/"),
		logger:    logger,
	}
}

// Upload accepts a multipart form with a single "file" field and persists it
// to the tenant's namespace. Returns 201 + FileResponse.
//
//	POST /v1/files
//	Content-Type: multipart/form-data
//	field: file=<binary>
func (h *FileHandler) Upload(c *fiber.Ctx) error {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "missing app context")
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "missing or invalid 'file' field in multipart form")
	}

	f, err := fileHeader.Open()
	if err != nil {
		h.logger.Error("file upload: cannot open multipart file", zap.Error(err))
		return fiber.NewError(fiber.StatusBadRequest, "cannot read uploaded file")
	}
	defer f.Close()

	mime := fileHeader.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}

	obj, err := h.svc.Upload(c.UserContext(), services.UploadInput{
		AppID:        appID,
		Name:         fileHeader.Filename,
		MIMEType:     mime,
		DeclaredSize: fileHeader.Size,
		Reader:       f,
	})
	if err != nil {
		return h.mapServiceError(c, err, "upload")
	}

	return c.Status(fiber.StatusCreated).JSON(dto.NewFileResponse(obj))
}

// Get returns the file metadata.
//
//	GET /v1/files/:id
func (h *FileHandler) Get(c *fiber.Ctx) error {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "missing app context")
	}
	id := c.Params("id")
	obj, err := h.svc.Get(c.UserContext(), appID, id)
	if err != nil {
		return h.mapServiceError(c, err, "get")
	}
	return c.JSON(dto.NewFileResponse(obj))
}

// List returns the tenant's files newest first. limit defaults to 50, capped
// at 200 by the repository; offset defaults to 0.
//
//	GET /v1/files?limit=&offset=
func (h *FileHandler) List(c *fiber.Ctx) error {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "missing app context")
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	files, total, err := h.svc.List(c.UserContext(), appID, limit, offset)
	if err != nil {
		return h.mapServiceError(c, err, "list")
	}
	out := make([]dto.FileResponse, 0, len(files))
	for _, f := range files {
		out = append(out, dto.NewFileResponse(f))
	}
	return c.JSON(dto.FileListResponse{Files: out, Total: total})
}

// Delete removes the file's bytes and metadata.
//
//	DELETE /v1/files/:id
func (h *FileHandler) Delete(c *fiber.Ctx) error {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "missing app context")
	}
	id := c.Params("id")
	if err := h.svc.Delete(c.UserContext(), appID, id); err != nil {
		return h.mapServiceError(c, err, "delete")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// Content streams the file's bytes to an authenticated caller. The response
// sets Content-Type, Content-Length, Content-Disposition (attachment) and
// X-Content-Type-Options: nosniff.
//
//	GET /v1/files/:id/content
func (h *FileHandler) Content(c *fiber.Ctx) error {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "missing app context")
	}
	return h.streamContent(c, appID, c.Params("id"))
}

// DownloadURL mints a short-lived signed URL for off-platform consumers. The
// URL targets the public /v1/files/download/:id endpoint and embeds (exp, sig)
// as query parameters. Requires API-key auth to mint.
//
//	GET /v1/files/:id/download-url
func (h *FileHandler) DownloadURL(c *fiber.Ctx) error {
	if h.signer == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "signed downloads are disabled (no signing key configured)")
	}
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "missing app context")
	}
	id := c.Params("id")
	// Existence + tenancy check before issuing a URL.
	if _, err := h.svc.Get(c.UserContext(), appID, id); err != nil {
		return h.mapServiceError(c, err, "download-url")
	}
	exp, sig := h.signer.Sign(appID, id)
	relPath := fmt.Sprintf("/v1/files/download/%s?app_id=%s&exp=%d&sig=%s", id, appID, exp, sig)
	url := relPath
	if h.publicURL != "" {
		url = h.publicURL + relPath
	}
	return c.JSON(dto.SignedURLResponse{
		URL:       url,
		ExpiresAt: time.Unix(exp, 0).UTC(),
	})
}

// PublicDownload serves a file to an unauthenticated caller carrying a valid
// signed URL. This route MUST be registered without API-key middleware.
//
//	GET /v1/files/download/:id?app_id=&exp=&sig=
func (h *FileHandler) PublicDownload(c *fiber.Ctx) error {
	if h.signer == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "signed downloads are disabled")
	}
	id := c.Params("id")
	appID := c.Query("app_id")
	expStr := c.Query("exp")
	sig := c.Query("sig")
	if appID == "" || expStr == "" || sig == "" {
		return fiber.NewError(fiber.StatusBadRequest, "missing required query parameters: app_id, exp, sig")
	}
	if err := h.signer.VerifyQuery(appID, id, expStr, sig); err != nil {
		switch {
		case errors.Is(err, filestore.ErrSignatureExpired):
			return fiber.NewError(fiber.StatusGone, "download link has expired")
		case errors.Is(err, filestore.ErrSignatureMissing),
			errors.Is(err, filestore.ErrSignatureMalformed):
			return fiber.NewError(fiber.StatusBadRequest, "invalid signature")
		default:
			// Treat verification failures as 404 to avoid leaking existence.
			return fiber.NewError(fiber.StatusNotFound, "file not found")
		}
	}
	return h.streamContent(c, appID, id)
}

// streamContent is the shared body-streaming path for authenticated and
// signed-URL downloads.
func (h *FileHandler) streamContent(c *fiber.Ctx, appID, id string) error {
	obj, rc, err := h.svc.OpenContent(c.UserContext(), appID, id)
	if err != nil {
		return h.mapServiceError(c, err, "content")
	}
	c.Set(fiber.HeaderContentType, obj.MIMEType)
	c.Set(fiber.HeaderContentLength, strconv.FormatInt(obj.Size, 10))
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set(fiber.HeaderContentDisposition,
		fmt.Sprintf(`attachment; filename="%s"`, sanitizeFilename(obj.Name)))
	return c.SendStream(streamCloser{rc}, int(obj.Size))
}

// mapServiceError translates domain/service errors to HTTP responses without
// leaking implementation details.
func (h *FileHandler) mapServiceError(c *fiber.Ctx, err error, op string) error {
	switch {
	case errors.Is(err, domainfile.ErrFileNotFound):
		return fiber.NewError(fiber.StatusNotFound, "file not found")
	case errors.Is(err, domainfile.ErrFileExpired):
		return fiber.NewError(fiber.StatusGone, "file has expired")
	case errors.Is(err, domainfile.ErrUnsupportedMIMEType):
		return fiber.NewError(fiber.StatusUnsupportedMediaType, "mime type is not allowed")
	case errors.Is(err, domainfile.ErrFileTooLarge):
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, "file exceeds maximum allowed size")
	case errors.Is(err, domainfile.ErrInvalidFileName),
		errors.Is(err, domainfile.ErrInvalidMIMEType),
		errors.Is(err, domainfile.ErrInvalidSize),
		errors.Is(err, domainfile.ErrInvalidAppID),
		errors.Is(err, domainfile.ErrInvalidFileID):
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	h.logger.Error("file handler: unexpected error",
		zap.String("op", op), zap.Error(err))
	return fiber.NewError(fiber.StatusInternalServerError, "internal error")
}

// sanitizeFilename strips characters that would break Content-Disposition.
// It does NOT attempt full RFC 5987 encoding — names with non-ASCII codepoints
// are best-effort.
func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, `"`, "")
	name = strings.ReplaceAll(name, "\n", "")
	name = strings.ReplaceAll(name, "\r", "")
	name = strings.ReplaceAll(name, "\\", "")
	if name == "" {
		return "download"
	}
	return name
}

// streamCloser adapts an io.ReadCloser to io.Reader for Fiber's SendStream,
// while ensuring Close is invoked when the response is finalized.
type streamCloser struct{ rc io.ReadCloser }

func (s streamCloser) Read(p []byte) (int, error) {
	n, err := s.rc.Read(p)
	if err != nil {
		_ = s.rc.Close()
	}
	return n, err
}
