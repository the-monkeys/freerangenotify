package handlers

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/tenant"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	pkgerrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"strings"
	"time"
)

// ChannelConnectionUpdateRequest preserves the flat connection patch wire format.
type ChannelConnectionUpdateRequest struct {
	channelconnection.Patch
	Revision *int64 `json:"revision" validate:"required,min=0" minimum:"0"`
}

type ChannelConnectionHandler struct {
	Service        *services.ChannelConnectionService
	Validator      *services.ConnectionValidator
	Members        auth.MembershipRepository
	Tenants        tenant.Service
	SenderBindings whatsapp.SenderBindingStore
}

func (h *ChannelConnectionHandler) scope(c *fiber.Ctx, manage bool) (*application.Application, string, notification.Channel, error) {
	user, _ := c.Locals("user_id").(string)
	if user == "" {
		return nil, "", "", pkgerrors.Unauthorized("Authentication required")
	}
	app, err := h.Service.Apps.GetByID(c.Context(), c.Params("id"))
	if err != nil || app == nil {
		return nil, "", "", pkgerrors.New(pkgerrors.ErrCodeInternal, "Application unavailable")
	}
	if _, err = channelAccess(c.Context(), app, user, h.Members, h.Tenants, manage); err != nil {
		return nil, "", "", err
	}
	env := c.Query("environment_id", "default")
	ch := notification.Channel(c.Params("channel"))
	if ch != "whatsapp" && ch != "sms" {
		return nil, "", "", pkgerrors.BadRequest("Unsupported channel")
	}
	if env != "default" {
		if h.Service.Environments == nil {
			return nil, "", "", pkgerrors.BadRequest("Environment unavailable")
		}
		e, err := h.Service.Environments.GetByID(c.Context(), env)
		if err != nil || e == nil || e.AppID != app.AppID {
			return nil, "", "", pkgerrors.Forbidden("Environment does not belong to application")
		}
	}
	return app, env, ch, nil
}
func (h *ChannelConnectionHandler) view(c *channelconnection.Connection, inherited bool, revision int64) fiber.Map {
	configured := map[string]bool{}
	for field := range c.Secrets {
		configured[field] = true
	}
	return fiber.Map{"id": c.ID, "app_id": c.AppID, "environment_id": c.EnvironmentID, "channel": c.Channel, "provider": c.Provider, "credential_source": c.CredentialSource, "account_id": c.AccountID, "sender_id": c.SenderID, "revision": revision, "effective_revision": c.Revision, "readiness": c.Readiness, "facts": c.Facts, "secret_configured": configured, "inherited": inherited, "writes_enabled": h.Service.WritesEnabled, "meta_enabled": h.Service.MetaEnabled == nil || *h.Service.MetaEnabled, "exotel": c.Exotel, "exotel_enabled": h.Service.ExotelEnabled}
}

// @Summary Get
// @Description Requires an account JWT and permission for the selected application or tenant.
// @Tags ChannelConnection
// @Produce json
// @Param id path string true "id"
// @Param channel path string true "channel"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/apps/{id}/channel-connections/{channel} [get]
// @Security BearerAuth
func (h *ChannelConnectionHandler) Get(c *fiber.Ctx) error {
	app, env, ch, err := h.scope(c, false)
	if err != nil {
		return err
	}
	id := channelconnection.ID(app.AppID, env, ch)
	row, err := h.Service.Repo.Get(c.Context(), id)
	if err == nil {
		return c.JSON(fiber.Map{"success": true, "data": h.view(row, false, row.Revision)})
	}
	if !errors.Is(err, channelconnection.ErrNotFound) {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Connection unavailable")
	}
	snap, err := h.Service.ResolveDeliveryConnection(c.Context(), app.AppID, env, ch)
	row = &channelconnection.Connection{ID: id, AppID: app.AppID, EnvironmentID: env, Channel: ch, Readiness: "unconfigured", CredentialSource: "byoc", Secrets: map[string]channelconnection.EncryptedSecret{}}
	if err == nil {
		row.Exotel = snap.Exotel
		row.Provider = snap.Provider
		row.AccountID = snap.AccountID
		row.SenderID = snap.SenderID
		row.Readiness = snap.Readiness
		row.CredentialSource = snap.CredentialSource
		row.Revision = snap.Revision
		for field, value := range snap.Credentials {
			if value != "" {
				row.Secrets[field] = channelconnection.EncryptedSecret{}
			}
		}
	} else if errors.Is(err, channelconnection.ErrDisconnected) {
		row.Readiness = "disconnected"
		if cfg := app.Settings.WhatsApp; ch == "whatsapp" && cfg != nil {
			row.Provider = cfg.Provider
			row.AccountID = cfg.MetaWABAID
			row.SenderID = cfg.MetaPhoneNumberID
			if cfg.Provider != "meta" {
				row.Provider = "twilio"
				row.AccountID = cfg.AccountSID
				row.SenderID = cfg.FromNumber
			}
		}
	} else if !errors.Is(err, channelconnection.ErrUnconfigured) {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Effective connection unavailable")
	}
	return c.JSON(fiber.Map{"success": true, "data": h.view(row, env != "default", 0)})
}
func connectionHTTPError(err error) error {
	var appErr *pkgerrors.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, channelconnection.ErrConflict) {
		return fiber.NewError(409, "Connection changed; reload before retrying")
	}
	if errors.Is(err, channelconnection.ErrWritesDisabled) {
		return fiber.NewError(503, err.Error())
	}
	return pkgerrors.BadRequest("Connection update rejected; check account, sender and encryption configuration")
}

// @Summary Update
// @Description Requires an account JWT and permission for the selected application or tenant.
// @Tags ChannelConnection
// @Produce json
// @Param id path string true "id"
// @Param channel path string true "channel"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/apps/{id}/channel-connections/{channel} [put]
// @Security BearerAuth
// @Accept json
// @Param body body ChannelConnectionUpdateRequest true "Flat connection patch; revision is required and nonnegative"
// @Failure 409 {object} map[string]interface{} "Revision conflict"
func (h *ChannelConnectionHandler) Update(c *fiber.Ctx) error {
	app, env, ch, err := h.scope(c, true)
	if err != nil {
		return err
	}
	var body ChannelConnectionUpdateRequest
	if c.BodyParser(&body) != nil || body.Revision == nil || *body.Revision < 0 {
		return pkgerrors.BadRequest("Connection revision required")
	}
	body.AppID = app.AppID
	body.EnvironmentID = env
	body.Channel = ch
	id := channelconnection.ID(app.AppID, env, ch)
	existing, err := h.Service.Repo.Get(c.Context(), id)
	if err == nil && (existing.AppID != app.AppID || existing.EnvironmentID != env || existing.Channel != ch) {
		return pkgerrors.Forbidden("Connection scope mismatch")
	}
	if err != nil && !errors.Is(err, channelconnection.ErrNotFound) {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Connection unavailable")
	}
	row, err := h.Service.UpdateConnection(c.Context(), id, *body.Revision, body.Patch)
	if err != nil {
		return connectionHTTPError(err)
	}
	return c.JSON(fiber.Map{"success": true, "data": h.view(row, false, row.Revision)})
}

// @Summary Validate
// @Description Requires an account JWT and permission for the selected application or tenant.
// @Tags ChannelConnection
// @Produce json
// @Param id path string true "id"
// @Param channel path string true "channel"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/apps/{id}/channel-connections/{channel}/validate [post]
// @Security BearerAuth
func (h *ChannelConnectionHandler) Validate(c *fiber.Ctx) error {
	app, env, ch, err := h.scope(c, true)
	if err != nil {
		return err
	}
	if !h.Service.WritesEnabled {
		return connectionHTTPError(channelconnection.ErrWritesDisabled)
	}
	var body struct {
		Revision *int64 `json:"revision"`
	}
	if c.BodyParser(&body) != nil || body.Revision == nil {
		return pkgerrors.BadRequest("Connection revision required")
	}
	id := channelconnection.ID(app.AppID, env, ch)
	row, err := h.Service.Repo.Get(c.Context(), id)
	if err != nil {
		return pkgerrors.BadRequest("Save this environment's connection before validation")
	}
	if row.Revision != *body.Revision {
		return connectionHTTPError(channelconnection.ErrConflict)
	}
	if row.Readiness == "disconnected" {
		return pkgerrors.BadRequest("Connection disconnected")
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	snap, err := h.Service.ResolveRevision(ctx, id, row.Revision)
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Connection credentials unavailable")
	}
	if h.Validator == nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Connection validator unavailable")
	}
	state, facts, validationErr := h.Validator.Validate(ctx, snap)
	if validationErr != nil {
		state = "invalid"
		if facts == nil {
			facts = map[string]string{}
		}
		facts["validation_error"] = "Provider validation unavailable; check credentials and retry"
	}
	if state == "ready" && ch == "whatsapp" && h.SenderBindings != nil {
		sender := strings.TrimPrefix(row.SenderID, "whatsapp:")
		binding := &whatsapp.InboundSenderBinding{ID: fmt.Sprintf("%x", sha256.Sum256([]byte(row.Provider+"\x00"+row.AccountID+"\x00"+sender))), AppID: app.AppID, EnvironmentID: env, Provider: row.Provider, AccountID: row.AccountID, SenderID: sender}
		if h.SenderBindings.ClaimSender(ctx, binding) != nil {
			return fiber.NewError(409, "Sender is already assigned to another application or environment")
		}
	}
	row.Readiness = state
	row.Facts = facts
	row.Revision++
	row.UpdatedAt = time.Now().UTC()
	if err = h.Service.Repo.Save(ctx, row, *body.Revision); err != nil {
		return connectionHTTPError(err)
	}
	return c.JSON(fiber.Map{"success": true, "data": h.view(row, false, row.Revision)})
}
