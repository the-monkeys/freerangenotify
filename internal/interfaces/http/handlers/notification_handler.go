package handlers

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"go.uber.org/zap"
)

// NotificationHandler handles HTTP requests for notifications
type NotificationHandler struct {
	service notification.Service
	logger  *zap.Logger
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler(service notification.Service, logger *zap.Logger) *NotificationHandler {
	return &NotificationHandler{
		service: service,
		logger:  logger,
	}
}

// Send handles POST /v1/notifications
// @Summary Send a notification
// @Description Send a notification to a user
// @Tags notifications
// @Accept json
// @Produce json
// @Param request body dto.SendNotificationRequest true "Send notification request"
// @Success 202 {object} dto.NotificationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security ApiKeyAuth
// @Router /v1/notifications [post]
func (h *NotificationHandler) Send(c *fiber.Ctx) error {
	// Get app ID from context (set by API key middleware)
	appID := c.Locals("app_id").(string)

	// Debug: Log the raw request body
	body := c.Body()
	fmt.Printf("DEBUG: Raw request body: %s, app_id: %s\n", string(body), appID)
	h.logger.Debug("Raw request body", zap.String("body", string(body)), zap.String("app_id", appID))

	var req dto.SendNotificationRequest
	if err := c.BodyParser(&req); err != nil {
		fmt.Printf("DEBUG: BodyParser error: %v\n", err)
		h.logger.Error("Failed to parse notification request body",
			zap.Error(err),
			zap.String("body", string(body)),
			zap.String("app_id", appID))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	h.logger.Debug("Successfully parsed notification request",
		zap.String("user_id", req.UserID),
		zap.String("template_id", req.TemplateID),
		zap.String("channel", req.Channel),
		zap.String("priority", req.Priority),
		zap.Any("data", req.Data))

	// Convert to domain request
	sendReq := req.ToSendRequest(appID)

	// Send notification
	notif, err := h.service.Send(c.Context(), sendReq)
	if err != nil {
		h.logger.Error("Failed to send notification", zap.Error(err))

		// Check if it's a validation error
		if notification.IsValidationError(err) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Check for rate limit error
		if err == notification.ErrRateLimitExceeded {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Check for DND error
		if err == notification.ErrDNDEnabled {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Convert to response
	response := dto.FromNotification(notif)

	return c.Status(fiber.StatusAccepted).JSON(response)
}

// SendBulk handles POST /v1/notifications/bulk
// @Summary Send bulk notifications
// @Description Send notifications to multiple users
// @Tags notifications
// @Accept json
// @Produce json
// @Param request body dto.BulkSendNotificationRequest true "Bulk send request"
// @Success 202 {object} dto.BulkSendResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security ApiKeyAuth
// @Router /v1/notifications/bulk [post]
func (h *NotificationHandler) SendBulk(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)

	var req dto.BulkSendNotificationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Convert to domain request
	bulkReq := req.ToBulkSendRequest(appID)

	// Send bulk notifications
	notifications, err := h.service.SendBulk(c.Context(), bulkReq)
	if err != nil {
		h.logger.Error("Failed to send bulk notifications", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Convert to response
	var items []*dto.NotificationResponse
	for _, n := range notifications {
		items = append(items, dto.FromNotification(n))
	}

	response := dto.BulkSendResponse{
		Sent:  len(notifications),
		Total: len(req.UserIDs),
		Items: items,
	}

	return c.Status(fiber.StatusAccepted).JSON(response)
}

// SendBatch handles POST /v1/notifications/batch
// @Summary Send batch notifications
// @Description Send multiple distinct notifications
// @Tags notifications
// @Accept json
// @Produce json
// @Param request body dto.BatchSendNotificationRequest true "Batch send request"
// @Success 202 {object} dto.BulkSendResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security ApiKeyAuth
// @Router /v1/notifications/batch [post]
func (h *NotificationHandler) SendBatch(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)

	var req dto.BatchSendNotificationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Convert to domain request
	batchReq := req.ToBatchSendRequest(appID)

	// Send batch notifications
	notifications, err := h.service.SendBatch(c.Context(), batchReq)
	if err != nil {
		h.logger.Error("Failed to send batch notifications", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Convert to response
	var items []*dto.NotificationResponse
	for _, n := range notifications {
		items = append(items, dto.FromNotification(n))
	}

	response := dto.BulkSendResponse{
		Sent:  len(notifications),
		Total: len(req.Notifications),
		Items: items,
	}

	return c.Status(fiber.StatusAccepted).JSON(response)
}

// List handles GET /v1/notifications
// @Summary List notifications
// @Description Get a list of notifications with filtering
// @Tags notifications
// @Produce json
// @Param user_id query string false "User ID"
// @Param channel query string false "Channel (push, email, sms, webhook, in_app)"
// @Param status query string false "Status"
// @Param priority query string false "Priority"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} dto.NotificationListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security ApiKeyAuth
// @Router /v1/notifications [get]
func (h *NotificationHandler) List(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)

	// Build filter from query params
	filter := notification.DefaultFilter()
	filter.AppID = appID

	if userID := c.Query("user_id"); userID != "" {
		filter.UserID = userID
	}
	if channel := c.Query("channel"); channel != "" {
		filter.Channel = notification.Channel(channel)
	}
	if status := c.Query("status"); status != "" {
		filter.Status = notification.Status(status)
	}
	if priority := c.Query("priority"); priority != "" {
		filter.Priority = notification.Priority(priority)
	}

	filter.Page = c.QueryInt("page", 1)
	filter.PageSize = c.QueryInt("page_size", 20)

	// List notifications
	notifications, err := h.service.List(c.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to list notifications", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Convert to response
	var items []*dto.NotificationResponse
	for _, n := range notifications {
		items = append(items, dto.FromNotification(n))
	}

	response := dto.NotificationListResponse{
		Notifications: items,
		Total:         int64(len(items)), // TODO: Get actual count from repository
		Page:          filter.Page,
		PageSize:      filter.PageSize,
	}

	return c.JSON(response)
}

// Get handles GET /v1/notifications/:id
// @Summary Get notification by ID
// @Description Get details of a specific notification
// @Tags notifications
// @Produce json
// @Param id path string true "Notification ID"
// @Success 200 {object} dto.NotificationResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security ApiKeyAuth
// @Router /v1/notifications/{id} [get]
func (h *NotificationHandler) Get(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	notificationID := c.Params("id")

	notif, err := h.service.Get(c.Context(), notificationID, appID)
	if err != nil {
		h.logger.Error("Failed to get notification", zap.Error(err))
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "notification not found",
		})
	}

	response := dto.FromNotification(notif)
	return c.JSON(response)
}

// UpdateStatus handles PUT /v1/notifications/:id/status
// @Summary Update notification status
// @Description Update the status of a notification (admin/webhook only)
// @Tags notifications
// @Accept json
// @Produce json
// @Param id path string true "Notification ID"
// @Param request body dto.UpdateStatusRequest true "Update status request"
// @Success 200 {object} dto.NotificationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security ApiKeyAuth
// @Router /v1/notifications/{id}/status [put]
func (h *NotificationHandler) UpdateStatus(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	notificationID := c.Params("id")

	var req dto.UpdateStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	status := notification.Status(req.Status)

	err := h.service.UpdateStatus(c.Context(), notificationID, status, req.ErrorMessage)
	if err != nil {
		h.logger.Error("Failed to update notification status", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Get updated notification
	notif, err := h.service.Get(c.Context(), notificationID, appID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "notification not found",
		})
	}

	response := dto.FromNotification(notif)
	return c.JSON(response)
}

// Cancel handles DELETE /v1/notifications/:id
// @Summary Cancel a notification
// @Description Cancel a scheduled notification
// @Tags notifications
// @Produce json
// @Param id path string true "Notification ID"
// @Success 200 {object} fiber.Map
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security ApiKeyAuth
// @Router /v1/notifications/{id} [delete]
func (h *NotificationHandler) Cancel(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	notificationID := c.Params("id")

	err := h.service.Cancel(c.Context(), notificationID, appID)
	if err != nil {
		h.logger.Error("Failed to cancel notification", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "notification cancelled successfully",
	})
}

// CancelBatch handles DELETE /v1/notifications/batch
// @Summary Cancel batch notifications
// @Description Cancel multiple scheduled notifications
// @Tags notifications
// @Accept json
// @Produce json
// @Param request body dto.BatchCancelRequest true "Batch cancel request"
// @Success 200 {object} fiber.Map
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security ApiKeyAuth
// @Router /v1/notifications/batch [delete]
func (h *NotificationHandler) CancelBatch(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)

	var req dto.BatchCancelRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	err := h.service.CancelBatch(c.Context(), req.NotificationIDs, appID)
	if err != nil {
		h.logger.Error("Failed to cancel batch notifications", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "notifications cancelled successfully",
	})
}

// Retry handles POST /v1/notifications/:id/retry
// @Summary Retry a failed notification
// @Description Retry a failed notification
// @Tags notifications
// @Produce json
// @Param id path string true "Notification ID"
// @Success 200 {object} fiber.Map
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security ApiKeyAuth
// @Router /v1/notifications/{id}/retry [post]
func (h *NotificationHandler) Retry(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	notificationID := c.Params("id")

	err := h.service.Retry(c.Context(), notificationID, appID)
	if err != nil {
		h.logger.Error("Failed to retry notification", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "notification queued for retry",
	})
}

// GetUnreadCount handles GET /v1/notifications/unread/count
func (h *NotificationHandler) GetUnreadCount(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	userID := c.Query("user_id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id is required"})
	}

	count, err := h.service.GetUnreadCount(c.Context(), userID, appID)
	if err != nil {
		h.logger.Error("Failed to get unread count", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"count": count})
}

// ListUnread handles GET /v1/notifications/unread
func (h *NotificationHandler) ListUnread(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	userID := c.Query("user_id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id is required"})
	}

	notifications, err := h.service.ListUnread(c.Context(), userID, appID)
	if err != nil {
		h.logger.Error("Failed to list unread notifications", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	// Convert to DTO and render content for each notification
	var responses []*dto.NotificationResponse
	for _, n := range notifications {
		resp := dto.FromNotification(n)

		// Render the body template with data if available
		if n.Content.Body != "" && n.Content.Data != nil {
			rendered, err := renderTemplate(n.Content.Body, n.Content.Data)
			if err != nil {
				h.logger.Warn("Failed to render notification content",
					zap.String("notification_id", n.NotificationID),
					zap.Error(err))
				// Use the raw body if rendering fails
				resp.Content.Notification = n.Content.Body
			} else {
				resp.Content.Notification = rendered
			}
		} else {
			resp.Content.Notification = n.Content.Body
		}

		responses = append(responses, resp)
	}

	return c.JSON(fiber.Map{"data": responses})
}

// renderTemplate renders a Go template string with the provided data
func renderTemplate(templateStr string, data map[string]any) (string, error) {
	tmpl, err := template.New("notification").Parse(templateStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// MarkRead handles POST /v1/notifications/read
func (h *NotificationHandler) MarkRead(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)

	var req struct {
		UserID          string   `json:"user_id"`
		NotificationIDs []string `json:"notification_ids"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.UserID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id is required"})
	}

	err := h.service.MarkRead(c.Context(), req.NotificationIDs, appID, req.UserID)
	if err != nil {
		h.logger.Error("Failed to mark notifications as read", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "notifications marked as read"})
}
