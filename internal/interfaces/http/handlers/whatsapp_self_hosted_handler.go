package handlers

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	pkgerrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// WhatsAppSelfHostedHandler handles the self-hosted WhatsApp Web/multi-device operations.
type WhatsAppSelfHostedHandler struct {
	service        *services.WhatsAppSelfHostedService
	appService     usecases.ApplicationService
	membershipRepo auth.MembershipRepository
	appRepo        application.Repository
	logger         *zap.Logger
}

// NewWhatsAppSelfHostedHandler creates a new WhatsAppSelfHostedHandler.
func NewWhatsAppSelfHostedHandler(
	service *services.WhatsAppSelfHostedService,
	appService usecases.ApplicationService,
	membershipRepo auth.MembershipRepository,
	appRepo application.Repository,
	logger *zap.Logger,
) *WhatsAppSelfHostedHandler {
	return &WhatsAppSelfHostedHandler{
		service:        service,
		appService:     appService,
		membershipRepo: membershipRepo,
		appRepo:        appRepo,
		logger:         logger,
	}
}

// GetStatus handles GET /v1/admin/whatsapp/self-hosted/:app_id/status
func (h *WhatsAppSelfHostedHandler) GetStatus(c *fiber.Ctx) error {
	app, err := h.authorize(c)
	if err != nil {
		return err
	}

	status, err := h.service.GetStatus(app.AppID)
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "failed to fetch WhatsApp connection status: "+err.Error())
	}

	return c.JSON(fiber.Map{"success": true, "data": status})
}

// Connect handles GET /v1/admin/whatsapp/self-hosted/:app_id/connect (SSE stream)
func (h *WhatsAppSelfHostedHandler) Connect(c *fiber.Ctx) error {
	app, err := h.authorize(c)
	if err != nil {
		return err
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Access-Control-Allow-Origin", "*")
	c.Set("Access-Control-Allow-Headers", "Cache-Control")

	if c.Context().Conn() != nil {
		_ = c.Context().Conn().SetWriteDeadline(time.Time{})
	}

	h.logger.Info("Client requested self-hosted WhatsApp connect SSE stream", zap.String("app_id", app.AppID))

	ctxDone := c.Context().Done()
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		streamCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sseCh := make(chan string, 10)

		// Startwhatsmeow service connection in background
		go func() {
			err := h.service.Connect(streamCtx, app.AppID, sseCh)
			if err != nil {
				h.logger.Error("WhatsApp self-hosted connection failed", zap.String("app_id", app.AppID), zap.Error(err))
				sseCh <- "error: " + err.Error()
			}
		}()

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		// Stream loop
		for {
			select {
			case msg := <-sseCh:
				if msg == "success" || msg == "connected" {
					// Persist connection mode to Application config settings
					go h.updateAppProviderMode(app.AppID)
				}
				fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
				w.Flush()
			case <-ticker.C:
				fmt.Fprintf(w, ": keepalive\n\n")
				w.Flush()
			case <-ctxDone:
				h.logger.Info("Self-hosted WhatsApp SSE client disconnected", zap.String("app_id", app.AppID))
				return
			}
		}
	})

	return nil
}

// Disconnect handles POST /v1/admin/whatsapp/self-hosted/:app_id/disconnect
func (h *WhatsAppSelfHostedHandler) Disconnect(c *fiber.Ctx) error {
	app, err := h.authorize(c)
	if err != nil {
		return err
	}

	err = h.service.Disconnect(c.Context(), app.AppID)
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "failed to disconnect WhatsApp session: "+err.Error())
	}

	// Update app settings to default twilio or disconnected
	if app.Settings.WhatsApp != nil {
		app.Settings.WhatsApp.ConnectionStatus = "disconnected"
		app.Settings.WhatsApp.Provider = ""
		app.UpdatedAt = time.Now().UTC()
		_ = h.appRepo.Update(c.Context(), app)
	}

	return c.JSON(fiber.Map{"success": true, "message": "WhatsApp self-hosted session disconnected and session terminated"})
}

// updateAppProviderMode sets the active provider to "self_hosted" in ES app settings.
func (h *WhatsAppSelfHostedHandler) updateAppProviderMode(appID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	app, err := h.appService.GetByID(ctx, appID)
	if err != nil {
		return
	}

	waCfg := app.Settings.WhatsApp
	if waCfg == nil {
		waCfg = &application.WhatsAppAppConfig{}
	}
	waCfg.Provider = "self_hosted"
	waCfg.ConnectionStatus = "connected"
	waCfg.ConnectedAt = time.Now().UTC().Format(time.RFC3339)

	status, err := h.service.GetStatus(appID)
	if err == nil {
		if phone, ok := status["display_phone"].(string); ok {
			waCfg.DisplayPhoneNumber = phone
		}
	}

	app.Settings.WhatsApp = waCfg
	app.UpdatedAt = time.Now().UTC()

	_ = h.appRepo.Update(ctx, app)
}

// authorize verifies the request context is allowed to modify app settings.
func (h *WhatsAppSelfHostedHandler) authorize(c *fiber.Ctx) (*application.Application, error) {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return nil, pkgerrors.Unauthorized("User not authenticated")
	}

	appID := c.Params("app_id")
	if appID == "" {
		return nil, pkgerrors.BadRequest("app_id is required")
	}

	app, err := h.appService.GetByID(c.Context(), appID)
	if err != nil {
		return nil, err
	}

	if app.AdminUserID == userID {
		return app, nil
	}

	if h.membershipRepo != nil {
		membership, mErr := h.membershipRepo.GetByAppAndUser(c.Context(), appID, userID)
		if mErr == nil && membership != nil && (membership.Role == auth.RoleAdmin || membership.Role == auth.RoleOwner) {
			return app, nil
		}
	}

	return nil, pkgerrors.Forbidden("You do not have access to manage WhatsApp for this application")
}
