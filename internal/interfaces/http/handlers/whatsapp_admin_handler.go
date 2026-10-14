package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"errors"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/tenant"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	pkgerrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// WhatsAppAdminHandler handles WhatsApp Embedded Signup and connection management.
type WhatsAppAdminHandler struct {
	Connections    *services.ChannelConnectionService
	tenantService  tenant.Service
	appService     usecases.ApplicationService
	membershipRepo auth.MembershipRepository
	appRepo        application.Repository
	metaAppID      string // Our FRN Facebook App ID
	metaAppSecret  string // Our FRN Facebook App Secret
	metaAPIVersion string // Graph API version
	webhookURL     string // Our public webhook URL for Meta to call
	logger         *zap.Logger
}

// NewWhatsAppAdminHandler creates a new WhatsApp admin handler.
func NewWhatsAppAdminHandler(
	appService usecases.ApplicationService,
	membershipRepo auth.MembershipRepository,
	appRepo application.Repository,
	metaAppID, metaAppSecret, metaAPIVersion, webhookURL string,
	logger *zap.Logger,
) *WhatsAppAdminHandler {
	if metaAPIVersion == "" {
		metaAPIVersion = "v23.0"
	}
	return &WhatsAppAdminHandler{
		appService:     appService,
		membershipRepo: membershipRepo,
		appRepo:        appRepo,
		metaAppID:      metaAppID,
		metaAppSecret:  metaAppSecret,
		metaAPIVersion: metaAPIVersion,
		webhookURL:     webhookURL,
		logger:         logger,
	}
}

// GetStatus handles GET /v1/admin/whatsapp/:app_id/status
func (h *WhatsAppAdminHandler) GetStatus(c *fiber.Ctx) error {
	app, err := h.authorize(c)
	if err != nil {
		return err
	}

	wa := app.Settings.WhatsApp
	if h.Connections != nil {
		row, e := h.Connections.Repo.Get(c.Context(), channelconnection.ID(app.AppID, "default", notification.ChannelWhatsApp))
		if e == nil {
			return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"connected": row.Readiness == "ready", "provider": row.Provider, "connection_status": row.Readiness, "revision": row.Revision, "waba_id": row.AccountID, "phone_number_id": row.SenderID}})
		}
		if !errors.Is(e, channelconnection.ErrNotFound) {
			return pkgerrors.New(pkgerrors.ErrCodeInternal, "Connection unavailable")
		}
	}
	if wa == nil || wa.Provider != "meta" {
		prov := "twilio"
		if wa != nil && wa.Provider != "" {
			prov = wa.Provider
		}
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"connected": false,
				"provider":  prov,
				"message":   "WhatsApp Meta not configured for this app",
			},
		})
	}

	result := fiber.Map{
		"connected":         wa.ConnectionStatus == "connected",
		"provider":          "meta",
		"connection_status": wa.ConnectionStatus,
		"connected_at":      wa.ConnectedAt,
		"phone_number_id":   wa.MetaPhoneNumberID,
		"waba_id":           wa.MetaWABAID,
		"display_phone":     wa.DisplayPhoneNumber,
		"quality_rating":    wa.QualityRating,
		"business_id":       wa.MetaBusinessID,
	}

	return c.JSON(fiber.Map{"success": true, "data": result})
}

// connectRequest is the payload from the frontend after Embedded Signup OAuth completes.
type connectRequest struct {
	PhoneNumberID string `json:"phone_number_id"`
	Revision      *int64 `json:"revision"`
	Code          string `json:"code" validate:"required"`
	AppID         string `json:"app_id" validate:"required"` // FRN app to associate
}

// Connect handles POST /v1/admin/whatsapp/connect
// Exchanges the OAuth code for a token, fetches WABA + phone info, and stores credentials.
func (h *WhatsAppAdminHandler) Connect(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return pkgerrors.Unauthorized("User not authenticated")
	}

	var req connectRequest
	if err := c.BodyParser(&req); err != nil {
		return pkgerrors.BadRequest("Invalid request body")
	}
	if req.Code == "" || req.AppID == "" {
		return pkgerrors.BadRequest("code and app_id are required")
	}

	app, _, err := h.authorizeByID(c, req.AppID, userID)
	if err != nil {
		return err
	}

	if h.metaAppID == "" || h.metaAppSecret == "" {
		return pkgerrors.New(pkgerrors.ErrCodeInternal,
			"Meta App credentials not configured. Set FREERANGE_PROVIDERS_META_WHATSAPP_META_APP_ID and FREERANGE_PROVIDERS_META_WHATSAPP_META_APP_SECRET.")
	}

	// Step 1: Exchange code for access token
	accessToken, err := h.exchangeCodeForToken(req.Code)
	if err != nil {
		h.logger.Error("Failed to exchange OAuth code for token", zap.String("error_category", "meta_request_failed"))
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Failed to exchange code with Meta")
	}

	// Step 2: Get shared WABA ID from the debug token / business integration response
	wabaID, bizID, err := h.getSharedWABAInfo(accessToken)
	if err != nil {
		h.logger.Error("Failed to get shared WABA info", zap.String("error_category", "meta_request_failed"))
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Failed to retrieve WABA info")
	}

	// Step 3: Get phone number(s) registered under this WABA
	phoneInfo, err := h.getPhoneNumbers(wabaID, accessToken, req.PhoneNumberID)
	if err != nil {
		h.logger.Error("Failed to get phone numbers", zap.String("waba_id", wabaID), zap.String("error_category", "meta_request_failed"))
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Failed to retrieve phone numbers")
	}

	// Step 4: Store credentials in app settings
	waCfg := app.Settings.WhatsApp
	if waCfg == nil {
		waCfg = &application.WhatsAppAppConfig{}
	}
	waCfg.Provider = "meta"
	waCfg.MetaAccessToken = accessToken
	waCfg.MetaAppSecret = h.metaAppSecret
	waCfg.MetaWABAID = wabaID
	waCfg.MetaBusinessID = bizID
	waCfg.ConnectionStatus = "pending"
	waCfg.ConnectedAt = time.Now().UTC().Format(time.RFC3339)

	if phoneInfo != nil {
		waCfg.MetaPhoneNumberID = phoneInfo.ID
		waCfg.DisplayPhoneNumber = phoneInfo.DisplayPhone
		waCfg.QualityRating = phoneInfo.QualityRating
	}

	app.Settings.WhatsApp = waCfg

	if err := h.saveConnection(c, app, waCfg, req.Revision); err != nil {
		h.logger.Error("Failed to save WhatsApp connection", zap.String("app_id", req.AppID), zap.String("error_category", "meta_request_failed"))
		return connectionHTTPError(err)
	}

	h.logger.Info("WhatsApp Embedded Signup connected",
		zap.String("app_id", req.AppID),
		zap.String("waba_id", wabaID),
		zap.String("phone_number_id", waCfg.MetaPhoneNumberID))

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"waba_id":           wabaID,
			"phone_number_id":   waCfg.MetaPhoneNumberID,
			"display_phone":     waCfg.DisplayPhoneNumber,
			"connection_status": "pending",
		},
	})
}

// Disconnect handles POST /v1/admin/whatsapp/:app_id/disconnect
func (h *WhatsAppAdminHandler) Disconnect(c *fiber.Ctx) error {
	app, err := h.authorize(c)
	if err != nil {
		return err
	}
	if h.Connections != nil {
		row, e := h.Connections.Repo.Get(c.Context(), channelconnection.ID(app.AppID, "default", notification.ChannelWhatsApp))
		if e == nil {
			var body struct {
				Revision *int64 `json:"revision"`
			}
			_ = c.BodyParser(&body)
			if body.Revision == nil || *body.Revision != row.Revision {
				return connectionHTTPError(channelconnection.ErrConflict)
			}
			if _, e = h.Connections.UpdateConnection(c.Context(), row.ID, *body.Revision, channelconnection.Patch{Disconnect: true}); e != nil {
				return connectionHTTPError(e)
			}
			return c.JSON(fiber.Map{"success": true, "message": "WhatsApp disconnected"})
		}
		if !errors.Is(e, channelconnection.ErrNotFound) {
			return pkgerrors.New(pkgerrors.ErrCodeInternal, "Connection unavailable")
		}
	}

	if app.Settings.WhatsApp == nil {
		return pkgerrors.BadRequest("WhatsApp is not configured for this app")
	}

	app.Settings.WhatsApp.ConnectionStatus = "disconnected"
	app.Settings.WhatsApp.MetaAccessToken = ""
	app.UpdatedAt = time.Now().UTC()

	if err := h.appRepo.Update(c.Context(), app); err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Failed to update app")
	}

	h.logger.Info("WhatsApp disconnected", zap.String("app_id", app.AppID))

	return c.JSON(fiber.Map{"success": true, "message": "WhatsApp disconnected"})
}

// SubscribeWebhooks handles POST /v1/admin/whatsapp/:app_id/subscribe-webhooks
// Subscribes the app's WABA to our webhook endpoint in Meta.
func (h *WhatsAppAdminHandler) SubscribeWebhooks(c *fiber.Ctx) error {
	app, err := h.authorize(c)
	if err != nil {
		return err
	}

	wa := app.Settings.WhatsApp
	if h.Connections != nil {
		resolved, _, e := h.Connections.WhatsAppConfig(c.Context(), app.AppID, "default")
		if e != nil {
			return pkgerrors.BadRequest("Selected connection unavailable")
		}
		wa = resolved
	}
	if wa == nil || wa.Provider != "meta" || wa.MetaAccessToken == "" {
		return pkgerrors.BadRequest("WhatsApp Meta is not connected for this app")
	}

	if h.webhookURL == "" {
		return pkgerrors.New(pkgerrors.ErrCodeInternal,
			"Webhook URL not configured. Set server.public_url in config.")
	}

	apiURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/subscribed_apps",
		h.metaAPIVersion, wa.MetaWABAID)

	reqBody := url.Values{}
	reqBody.Set("access_token", wa.MetaAccessToken)

	resp, err := http.Post(apiURL, "application/x-www-form-urlencoded", strings.NewReader(reqBody.Encode()))
	if err != nil {
		h.logger.Error("Failed to subscribe WABA webhooks", zap.String("error_category", "meta_request_failed"))
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Failed to subscribe webhooks")
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		h.logger.Error("Meta webhook subscription failed",
			zap.Int("status", resp.StatusCode),
		)
		return pkgerrors.New(pkgerrors.ErrCodeInternal,
			fmt.Sprintf("Meta webhook subscription returned %d", resp.StatusCode))
	}

	h.logger.Info("WABA webhook subscription successful",
		zap.String("app_id", app.AppID),
		zap.String("waba_id", wa.MetaWABAID))

	return c.JSON(fiber.Map{"success": true, "message": "Webhook subscription active"})
}

// ManualConnect handles POST /v1/admin/whatsapp/manual-connect
// Allows connecting a WABA using a pre-existing System User Access Token instead of Embedded Signup.
func (h *WhatsAppAdminHandler) ManualConnect(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return pkgerrors.Unauthorized("User not authenticated")
	}

	var req struct {
		Revision      *int64 `json:"revision"`
		AppSecret     string `json:"app_secret"`
		AppID         string `json:"app_id"`
		AccessToken   string `json:"access_token"`
		WABAID        string `json:"waba_id"`
		PhoneNumberID string `json:"phone_number_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return pkgerrors.BadRequest("Invalid request body")
	}
	if req.AppID == "" || req.AccessToken == "" || req.WABAID == "" {
		return pkgerrors.BadRequest("app_id, access_token, and waba_id are required")
	}

	app, _, err := h.authorizeByID(c, req.AppID, userID)
	if err != nil {
		return err
	}

	// Fetch phone numbers from the WABA to validate the token and get phone info
	phoneInfo, err := h.getPhoneNumbers(req.WABAID, req.AccessToken, req.PhoneNumberID)
	if err != nil {
		h.logger.Error("Failed to validate token by fetching phone numbers", zap.String("error_category", "meta_request_failed"))
		return pkgerrors.BadRequest("Select an owned phone number from a non-empty WABA")
	}

	waCfg := app.Settings.WhatsApp
	if waCfg == nil {
		waCfg = &application.WhatsAppAppConfig{}
	}
	waCfg.Provider = "meta"
	waCfg.MetaAccessToken = req.AccessToken
	waCfg.MetaWABAID = req.WABAID
	if req.AppSecret != "" {
		waCfg.MetaAppSecret = req.AppSecret
	}
	waCfg.ConnectionStatus = "pending"
	waCfg.ConnectedAt = time.Now().UTC().Format(time.RFC3339)

	if phoneInfo != nil {
		waCfg.MetaPhoneNumberID = phoneInfo.ID
		waCfg.DisplayPhoneNumber = phoneInfo.DisplayPhone
		waCfg.QualityRating = phoneInfo.QualityRating
	}
	if req.PhoneNumberID != "" {
		waCfg.MetaPhoneNumberID = req.PhoneNumberID
	}

	app.Settings.WhatsApp = waCfg

	if err := h.saveConnection(c, app, waCfg, req.Revision); err != nil {
		h.logger.Error("Failed to save manual WhatsApp connection", zap.String("app_id", req.AppID), zap.String("error_category", "meta_request_failed"))
		return connectionHTTPError(err)
	}

	h.logger.Info("WhatsApp manually connected",
		zap.String("app_id", req.AppID),
		zap.String("waba_id", req.WABAID),
		zap.String("phone_number_id", waCfg.MetaPhoneNumberID))

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"waba_id":           req.WABAID,
			"phone_number_id":   waCfg.MetaPhoneNumberID,
			"display_phone":     waCfg.DisplayPhoneNumber,
			"connection_status": "pending",
		},
	})
}

// --- OAuth helpers ---

func (h *WhatsAppAdminHandler) saveConnection(c *fiber.Ctx, app *application.Application, wa *application.WhatsAppAppConfig, expected *int64) error {
	if h.Connections != nil && h.Connections.WritesEnabled {
		revision := int64(0)
		id := channelconnection.ID(app.AppID, "default", notification.ChannelWhatsApp)
		old, err := h.Connections.Repo.Get(c.Context(), id)
		if err == nil {
			if expected == nil || *expected != old.Revision {
				return channelconnection.ErrConflict
			}
			revision = *expected
		} else if !errors.Is(err, channelconnection.ErrNotFound) {
			return err
		}
		provider, source := "meta", "byoc"
		_, err = h.Connections.UpdateConnection(c.Context(), id, revision, channelconnection.Patch{AppID: app.AppID, Channel: notification.ChannelWhatsApp, Provider: &provider, CredentialSource: &source, AccountID: &wa.MetaWABAID, SenderID: &wa.MetaPhoneNumberID, Secrets: map[string]string{"access_token": wa.MetaAccessToken, "app_secret": wa.MetaAppSecret}})
		return err
	}
	if h.Connections != nil {
		_, err := h.Connections.Repo.Get(c.Context(), channelconnection.ID(app.AppID, "default", notification.ChannelWhatsApp))
		if err == nil {
			return channelconnection.ErrWritesDisabled
		}
		if !errors.Is(err, channelconnection.ErrNotFound) {
			return err
		}
	}
	app.Settings.WhatsApp = wa
	if repo, ok := h.appRepo.(interface {
		UpdateSettingsIfRevision(context.Context, string, time.Time, application.Settings) error
	}); ok {
		return repo.UpdateSettingsIfRevision(c.Context(), app.AppID, app.UpdatedAt, app.Settings)
	}
	app.UpdatedAt = time.Now().UTC()
	return h.appRepo.Update(c.Context(), app)
}

func (h *WhatsAppAdminHandler) exchangeCodeForToken(code string) (string, error) {
	apiURL := fmt.Sprintf("https://graph.facebook.com/%s/oauth/access_token", h.metaAPIVersion)

	params := url.Values{}
	params.Set("client_id", h.metaAppID)
	params.Set("client_secret", h.metaAppSecret)
	params.Set("code", code)

	resp, err := http.Get(apiURL + "?" + params.Encode())
	if err != nil {
		return "", fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Meta returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("empty access_token in response")
	}

	return result.AccessToken, nil
}

// getSharedWABAInfo uses the debug_token endpoint and the business's shared
// WABA list to find the WABA ID granted during Embedded Signup.
func (h *WhatsAppAdminHandler) getSharedWABAInfo(accessToken string) (wabaID, businessID string, err error) {
	// First, debug the token to get the granular scopes and business ID
	debugURL := fmt.Sprintf("https://graph.facebook.com/%s/debug_token?input_token=%s&access_token=%s|%s",
		h.metaAPIVersion, accessToken, h.metaAppID, h.metaAppSecret)

	resp, err := http.Get(debugURL)
	if err != nil {
		return "", "", fmt.Errorf("debug_token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var debugResult struct {
		Data struct {
			GranularScopes []struct {
				Scope     string   `json:"scope"`
				TargetIDs []string `json:"target_ids"`
			} `json:"granular_scopes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &debugResult); err != nil {
		return "", "", fmt.Errorf("failed to parse debug_token: %w", err)
	}

	// Extract WABA ID from the whatsapp_business_messaging scope target
	for _, scope := range debugResult.Data.GranularScopes {
		if scope.Scope == "whatsapp_business_messaging" && len(scope.TargetIDs) > 0 {
			wabaID = scope.TargetIDs[0]
		}
		if scope.Scope == "whatsapp_business_management" && len(scope.TargetIDs) > 0 && wabaID == "" {
			wabaID = scope.TargetIDs[0]
		}
		if scope.Scope == "business_management" && len(scope.TargetIDs) > 0 {
			businessID = scope.TargetIDs[0]
		}
	}

	if wabaID == "" {
		return "", "", fmt.Errorf("no WABA ID found in token scopes — user may not have completed signup")
	}

	return wabaID, businessID, nil
}

type phoneNumberInfo struct {
	ID            string
	DisplayPhone  string
	QualityRating string
}

func (h *WhatsAppAdminHandler) getPhoneNumbers(wabaID, accessToken string, selected ...string) (*phoneNumberInfo, error) {
	phones, err := (&services.ConnectionValidator{GraphVersion: h.metaAPIVersion}).MetaPhones(context.Background(), wabaID, accessToken)
	if err != nil {
		return nil, err
	}
	wanted := ""
	if len(selected) > 0 {
		wanted = selected[0]
	}
	if len(phones) == 0 {
		return nil, fmt.Errorf("WABA has no phone numbers")
	}
	if wanted == "" && len(phones) != 1 {
		return nil, fmt.Errorf("select a phone number")
	}
	for _, phone := range phones {
		if wanted == "" || phone.ID == wanted {
			return &phoneNumberInfo{ID: phone.ID, DisplayPhone: phone.DisplayPhoneNumber, QualityRating: phone.QualityRating}, nil
		}
	}
	return nil, fmt.Errorf("selected phone does not belong to WABA")
}

// --- Authorization helpers ---

func (h *WhatsAppAdminHandler) authorize(c *fiber.Ctx) (*application.Application, error) {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return nil, pkgerrors.Unauthorized("User not authenticated")
	}

	appID := c.Params("app_id")
	if appID == "" {
		return nil, pkgerrors.BadRequest("app_id is required")
	}

	app, _, err := h.authorizeByID(c, appID, userID)
	return app, err
}

func (h *WhatsAppAdminHandler) authorizeByID(c *fiber.Ctx, appID, userID string) (*application.Application, auth.Role, error) {
	app, err := h.appService.GetByID(c.Context(), appID)
	if err != nil {
		return nil, "", err
	}

	role, accessErr := channelAccess(c.Context(), app, userID, h.membershipRepo, h.tenantService, true)
	return app, role, accessErr
}

func (h *WhatsAppAdminHandler) SetTenantService(service tenant.Service) { h.tenantService = service }
