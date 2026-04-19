package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	pkgerrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// WhatsAppAdminHandler handles WhatsApp Embedded Signup and connection management.
type WhatsAppAdminHandler struct {
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
		"connected":          wa.ConnectionStatus == "connected",
		"provider":           "meta",
		"connection_status":  wa.ConnectionStatus,
		"connected_at":       wa.ConnectedAt,
		"phone_number_id":    wa.MetaPhoneNumberID,
		"waba_id":            wa.MetaWABAID,
		"display_phone":      wa.DisplayPhoneNumber,
		"quality_rating":     wa.QualityRating,
		"business_id":        wa.MetaBusinessID,
	}

	return c.JSON(fiber.Map{"success": true, "data": result})
}

// connectRequest is the payload from the frontend after Embedded Signup OAuth completes.
type connectRequest struct {
	Code  string `json:"code" validate:"required"`
	AppID string `json:"app_id" validate:"required"` // FRN app to associate
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
		h.logger.Error("Failed to exchange OAuth code for token", zap.Error(err))
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Failed to exchange code with Meta: "+err.Error())
	}

	// Step 2: Get shared WABA ID from the debug token / business integration response
	wabaID, bizID, err := h.getSharedWABAInfo(accessToken)
	if err != nil {
		h.logger.Error("Failed to get shared WABA info", zap.Error(err))
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Failed to retrieve WABA info: "+err.Error())
	}

	// Step 3: Get phone number(s) registered under this WABA
	phoneInfo, err := h.getPhoneNumbers(wabaID, accessToken)
	if err != nil {
		h.logger.Error("Failed to get phone numbers", zap.String("waba_id", wabaID), zap.Error(err))
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Failed to retrieve phone numbers: "+err.Error())
	}

	// Step 4: Store credentials in app settings
	waCfg := app.Settings.WhatsApp
	if waCfg == nil {
		waCfg = &application.WhatsAppAppConfig{}
	}
	waCfg.Provider = "meta"
	waCfg.MetaAccessToken = accessToken
	waCfg.MetaWABAID = wabaID
	waCfg.MetaBusinessID = bizID
	waCfg.ConnectionStatus = "connected"
	waCfg.ConnectedAt = time.Now().UTC().Format(time.RFC3339)

	if phoneInfo != nil {
		waCfg.MetaPhoneNumberID = phoneInfo.ID
		waCfg.DisplayPhoneNumber = phoneInfo.DisplayPhone
		waCfg.QualityRating = phoneInfo.QualityRating
	}

	app.Settings.WhatsApp = waCfg
	app.UpdatedAt = time.Now().UTC()

	if err := h.appRepo.Update(c.Context(), app); err != nil {
		h.logger.Error("Failed to save WhatsApp connection", zap.String("app_id", req.AppID), zap.Error(err))
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Failed to save connection")
	}

	h.logger.Info("WhatsApp Embedded Signup connected",
		zap.String("app_id", req.AppID),
		zap.String("waba_id", wabaID),
		zap.String("phone_number_id", waCfg.MetaPhoneNumberID))

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"waba_id":          wabaID,
			"phone_number_id":  waCfg.MetaPhoneNumberID,
			"display_phone":    waCfg.DisplayPhoneNumber,
			"connection_status": "connected",
		},
	})
}

// Disconnect handles POST /v1/admin/whatsapp/:app_id/disconnect
func (h *WhatsAppAdminHandler) Disconnect(c *fiber.Ctx) error {
	app, err := h.authorize(c)
	if err != nil {
		return err
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
		h.logger.Error("Failed to subscribe WABA webhooks", zap.Error(err))
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Failed to subscribe webhooks: "+err.Error())
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		h.logger.Error("Meta webhook subscription failed",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)))
		return pkgerrors.New(pkgerrors.ErrCodeInternal,
			fmt.Sprintf("Meta returned %d: %s", resp.StatusCode, string(body)))
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
	phoneInfo, err := h.getPhoneNumbers(req.WABAID, req.AccessToken)
	if err != nil {
		h.logger.Error("Failed to validate token by fetching phone numbers", zap.Error(err))
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Failed to validate access token with Meta: "+err.Error())
	}

	// If a specific phone number ID was provided, try to match it
	if req.PhoneNumberID != "" && phoneInfo != nil && phoneInfo.ID != req.PhoneNumberID {
		allPhones, _ := h.getPhoneNumbers(req.WABAID, req.AccessToken)
		if allPhones != nil {
			phoneInfo = allPhones
		}
	}

	waCfg := app.Settings.WhatsApp
	if waCfg == nil {
		waCfg = &application.WhatsAppAppConfig{}
	}
	waCfg.Provider = "meta"
	waCfg.MetaAccessToken = req.AccessToken
	waCfg.MetaWABAID = req.WABAID
	waCfg.ConnectionStatus = "connected"
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
	app.UpdatedAt = time.Now().UTC()

	if err := h.appRepo.Update(c.Context(), app); err != nil {
		h.logger.Error("Failed to save manual WhatsApp connection", zap.String("app_id", req.AppID), zap.Error(err))
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Failed to save connection")
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
			"connection_status": "connected",
		},
	})
}

// --- OAuth helpers ---

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
				Scope    string   `json:"scope"`
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

func (h *WhatsAppAdminHandler) getPhoneNumbers(wabaID, accessToken string) (*phoneNumberInfo, error) {
	apiURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/phone_numbers?access_token=%s",
		h.metaAPIVersion, wabaID, accessToken)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("phone numbers request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Meta returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID                 string `json:"id"`
			DisplayPhoneNumber string `json:"display_phone_number"`
			QualityRating      string `json:"quality_rating"`
			VerifiedName       string `json:"verified_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse phone numbers: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, nil
	}

	phone := result.Data[0]
	return &phoneNumberInfo{
		ID:            phone.ID,
		DisplayPhone:  phone.DisplayPhoneNumber,
		QualityRating: phone.QualityRating,
	}, nil
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

	if app.AdminUserID == userID {
		return app, auth.RoleOwner, nil
	}

	if h.membershipRepo != nil {
		membership, mErr := h.membershipRepo.GetByAppAndUser(c.Context(), appID, userID)
		if mErr == nil && membership != nil && (membership.Role == auth.RoleAdmin || membership.Role == auth.RoleOwner) {
			return app, membership.Role, nil
		}
	}

	return nil, "", pkgerrors.Forbidden("You do not have access to manage WhatsApp for this application")
}

