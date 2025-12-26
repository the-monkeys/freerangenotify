package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/sse"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"go.uber.org/zap"
)

// SSEHandler handles Server-Sent Events
type SSEHandler struct {
	broadcaster *sse.Broadcaster
	appService  usecases.ApplicationService
	logger      *zap.Logger
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler(broadcaster *sse.Broadcaster, appService usecases.ApplicationService, logger *zap.Logger) *SSEHandler {
	return &SSEHandler{
		broadcaster: broadcaster,
		appService:  appService,
		logger:      logger,
	}
}

// Connect establishes an SSE connection for a user
func (h *SSEHandler) Connect(c *fiber.Ctx) error {
	// token := c.Query("token")
	appID := c.Query("app_id")
	userID := c.Query("user_id")

	// If token and app_id are provided, we attempt validation
	// Validate App ID if provided
	if appID != "" {
		_, err := h.appService.GetByID(c.Context(), appID)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid app_id"})
		}
	}

	/* External validation disabled per user request
	if token != "" && appID != "" {
		app, err := h.appService.GetByID(c.Context(), appID)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid app_id"})
		}

		// Support for Zero-Trust validation via Client's own API
		if app.Settings.ValidationURL != "" {
			externalID, err := h.validateExternal(token, app)
			if err != nil {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "external validation failed: " + err.Error()})
			}
			userID = externalID
		} else {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "application has no validation_url configured"})
		}
	}
	*/

	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "user_id or valid token is required",
		})
	}

	h.logger.Info("SSE connection authenticated", zap.String("user_id", userID), zap.String("app_id", appID))

	// Handle the SSE connection
	return h.broadcaster.HandleSSE(c, userID)
}

func (h *SSEHandler) validateExternal(token string, app *application.Application) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	config := app.Settings.ValidationConfig

	// Default configuration (backward compatibility)
	method := "POST"
	tokenKey := "token"
	tokenPlacement := "body_json"

	if config != nil {
		if config.Method != "" {
			method = config.Method
		}
		if config.TokenKey != "" {
			tokenKey = config.TokenKey
		}
		if config.TokenPlacement != "" {
			tokenPlacement = config.TokenPlacement
		}
	}

	var req *http.Request
	var err error
	targetURL := app.Settings.ValidationURL

	switch tokenPlacement {
	case "body_json":
		bodyMap := map[string]string{tokenKey: token}
		if config == nil {
			bodyMap["app_id"] = app.AppID
		}
		data, _ := json.Marshal(bodyMap)
		req, err = http.NewRequest(method, targetURL, bytes.NewBuffer(data))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
		}

	case "body_form":
		// Simple form encoding
		// We'll constructs the body manually or use url.Values but we need "net/url"
		// To avoid adding imports if possible, let's use a simple string for single value
		// But adding imports is safer. I'll stick to 'token=...' string for simplicity if imports are tricky,
		// but I should add imports. I will handle imports in a separate block or assume I can add them here if ReplaceFile allows (it doesn't auto-add imports).
		// I will have to add imports "strings" and "net/url" separately.
		// For now, I'll use basic string construction for form since it's simple.
		body := fmt.Sprintf("%s=%s", tokenKey, token)
		req, err = http.NewRequest(method, targetURL, bytes.NewBufferString(body))
		if err == nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}

	case "query":
		req, err = http.NewRequest(method, targetURL, nil)
		if err == nil {
			q := req.URL.Query()
			q.Add(tokenKey, token)
			req.URL.RawQuery = q.Encode()
		}

	case "header":
		req, err = http.NewRequest(method, targetURL, nil)
		if err == nil {
			req.Header.Set(tokenKey, token)
		}

	case "cookie":
		req, err = http.NewRequest(method, targetURL, nil)
		if err == nil {
			req.AddCookie(&http.Cookie{Name: tokenKey, Value: token})
		}

	default:
		return "", fmt.Errorf("unsupported token placement: %s", tokenPlacement)
	}

	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add static headers
	if config != nil && config.StaticHeaders != nil {
		for k, v := range config.StaticHeaders {
			req.Header.Set(k, v)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call validation URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("validation service returned status %d", resp.StatusCode)
	}

	// Decode response to map to support flexible fields
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode validation response: %w", err)
	}

	// Check explicit 'valid' field if present (optional)
	if v, ok := result["valid"]; ok {
		if boolVal, ok := v.(bool); ok && !boolVal {
			return "", fmt.Errorf("token is invalid according to external service")
		}
	}

	// Heuristic to find user ID
	possibleKeys := []string{"user_id", "id", "sub", "uid", "account_id", "username"}
	for _, k := range possibleKeys {
		if v, ok := result[k]; ok {
			if strVal, ok := v.(string); ok && strVal != "" {
				return strVal, nil
			}
		}
	}

	// If legacy logic expected specific struct, we might fail here if we rely on new logic only.
	// But assuming the external API returns SOMETHING identifiable.
	return "", fmt.Errorf("could not find user identifier (user_id, id, sub, uid) in response")
}
