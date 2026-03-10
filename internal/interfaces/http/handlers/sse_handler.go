package handlers

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/sse"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/pkg/utils"
	"go.uber.org/zap"
)

const (
	// sseTokenPrefix is the Redis key prefix for SSE tokens.
	sseTokenPrefix = "sse_token:"
	// sseTokenTTL is the default expiry for SSE tokens (15 minutes).
	sseTokenTTL = 15 * time.Minute
)

// sseTokenData is stored in Redis and bound to a single user+app.
type sseTokenData struct {
	UserID string `json:"user_id"`
	AppID  string `json:"app_id"`
}

// SSEHandler handles Server-Sent Events connections and streaming.
type SSEHandler struct {
	broadcaster  *sse.Broadcaster
	appService   usecases.ApplicationService
	notifService notification.Service
	userRepo     user.Repository
	redisClient  *redis.Client
	logger       *zap.Logger
	hmacEnforced bool // Phase 1: when true, subscriberHash query param is required
}

// NewSSEHandler creates a new SSE handler.
func NewSSEHandler(
	broadcaster *sse.Broadcaster,
	appService usecases.ApplicationService,
	notifService notification.Service,
	userRepo user.Repository,
	redisClient *redis.Client,
	logger *zap.Logger,
) *SSEHandler {
	return &SSEHandler{
		broadcaster:  broadcaster,
		appService:   appService,
		notifService: notifService,
		userRepo:     userRepo,
		redisClient:  redisClient,
		logger:       logger,
	}
}

// SetHMACEnforced enables HMAC subscriber authentication for SSE connections.
// Uses setter injection to maintain backward compatibility.
func (h *SSEHandler) SetHMACEnforced(enforced bool) {
	h.hmacEnforced = enforced
}

// createTokenRequest is the JSON body for POST /v1/sse/tokens.
type createTokenRequest struct {
	UserID string `json:"user_id" validate:"required"` // external_id or internal UUID
}

// CreateToken issues a short-lived, user-scoped SSE token.
// The caller authenticates with the application API key (Authorization header),
// so the API key never needs to reach the browser.
//
// POST /v1/sse/tokens
// Authorization: Bearer frn_xxx
// { "user_id": "my_platform_username" }  ← external_id or internal UUID
//
// Response:
// { "sse_token": "sset_abc123...", "user_id": "<resolved internal UUID>", "expires_in": 900 }
func (h *SSEHandler) CreateToken(c *fiber.Ctx) error {
	if h.redisClient == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "SSE tokens require Redis",
		})
	}

	var req createTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.UserID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id is required"})
	}

	// The API key middleware already validated the key and set app in locals.
	appID, _ := c.Locals("app_id").(string)
	if appID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "application not authenticated"})
	}

	// ── Resolve external_id → internal UUID ──
	userID := req.UserID
	if _, err := uuid.Parse(userID); err != nil && h.userRepo != nil {
		u, lookupErr := h.userRepo.GetByExternalID(c.Context(), appID, userID)
		if lookupErr != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": fmt.Sprintf("user with external_id %q not found in this application", userID),
			})
		}
		h.logger.Info("SSE token: resolved external_id to internal user_id",
			zap.String("external_id", userID),
			zap.String("user_id", u.UserID))
		userID = u.UserID
	}

	// ── Generate cryptographically random token ──
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		h.logger.Error("Failed to generate SSE token", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "token generation failed"})
	}
	tokenValue := "sset_" + hex.EncodeToString(rawBytes)

	// ── Store in Redis with TTL ──
	tokenData := sseTokenData{UserID: userID, AppID: appID}
	payload, _ := json.Marshal(tokenData)

	redisKey := sseTokenPrefix + tokenValue
	if err := h.redisClient.Set(c.Context(), redisKey, payload, sseTokenTTL).Err(); err != nil {
		h.logger.Error("Failed to store SSE token in Redis", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "token storage failed"})
	}

	h.logger.Info("SSE token issued",
		zap.String("user_id", userID),
		zap.String("app_id", appID),
		zap.Duration("ttl", sseTokenTTL))

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"sse_token":  tokenValue,
		"user_id":    userID,
		"expires_in": int(sseTokenTTL.Seconds()),
	})
}

// Connect establishes an SSE connection for a user.
//
// Query params:
//
//	sse_token (recommended) — short-lived token from POST /v1/sse/tokens.
//	                          user_id is resolved from the token automatically.
//	user_id   (required if no sse_token) — the internal UUID or external_id.
//	token     (deprecated) — raw API key. Prefer sse_token instead.
//	app_id    (optional)   — explicit app ID.
//
// Recommended flow (secure — API key never reaches browser):
//
//	GET /v1/sse?sse_token=sset_abc123
//
// Legacy flow (still supported for backward compatibility):
//
//	GET /v1/sse?user_id=user002&token=frn_xxx
func (h *SSEHandler) Connect(c *fiber.Ctx) error {
	sseToken := c.Query("sse_token")
	userID := c.Query("user_id")
	token := c.Query("token")
	appID := c.Query("app_id")

	// ── 1. SSE Token auth (preferred, secure path) ──
	if sseToken != "" {
		if h.redisClient == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "SSE tokens require Redis",
			})
		}

		redisKey := sseTokenPrefix + sseToken
		val, err := h.redisClient.Get(c.Context(), redisKey).Result()
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired sse_token"})
		}

		var td sseTokenData
		if err := json.Unmarshal([]byte(val), &td); err != nil {
			h.logger.Error("Failed to unmarshal SSE token data", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "token data corrupt"})
		}

		// Override user_id and app_id from the token (authoritative).
		userID = td.UserID
		appID = td.AppID

		h.logger.Info("SSE connection via sse_token",
			zap.String("user_id", userID),
			zap.String("app_id", appID))

	} else {
		// ── 2. Legacy path: raw token / no auth ──
		if userID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "user_id or sse_token query parameter is required",
			})
		}

		if token != "" {
			app, err := h.appService.ValidateAPIKey(c.Context(), token)
			if err != nil {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
			}
			if appID != "" && app.AppID != appID {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "token does not match app_id"})
			}
			appID = app.AppID
		} else if appID != "" {
			if _, err := h.appService.GetByID(c.Context(), appID); err != nil {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid app_id"})
			}
		}

		// Resolve external_id → internal UUID
		if _, err := uuid.Parse(userID); err != nil && appID != "" && h.userRepo != nil {
			u, lookupErr := h.userRepo.GetByExternalID(c.Context(), appID, userID)
			if lookupErr == nil {
				h.logger.Info("SSE: resolved external_id to internal user_id",
					zap.String("external_id", userID),
					zap.String("user_id", u.UserID))
				userID = u.UserID
			}
		}

		// HMAC subscriber authentication (feature-gated, legacy path only)
		if h.hmacEnforced && token != "" {
			subscriberHash := c.Query("subscriber_hash")
			if subscriberHash == "" {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "subscriber_hash query parameter is required when HMAC enforcement is enabled",
				})
			}
			if !utils.ValidateSubscriberHash(userID, token, subscriberHash) {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "invalid subscriber_hash",
				})
			}
		}
	}

	h.logger.Info("SSE connection established",
		zap.String("user_id", userID),
		zap.String("app_id", appID))

	// ── 3. Flush any queued notifications for instant delivery ──
	if h.notifService != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := h.notifService.FlushQueued(ctx, userID); err != nil {
				h.logger.Error("Failed to flush queued notifications", zap.Error(err))
			}
		}()
	}

	// ── 4. Hand off to the broadcaster ──
	return h.broadcaster.HandleSSE(c, userID)
}

// CreateDashboardToken issues a short-lived SSE token for dashboard users (JWT auth).
// POST /v1/admin/sse/token
// Authorization: Bearer <jwt>
// Response: { "sse_token": "sset_...", "user_id": "...", "expires_in": 900 }
//
// The frontend uses this token to connect to GET /v1/sse?sse_token=sset_... for real-time
// dashboard notifications (e.g. org invites).
func (h *SSEHandler) CreateDashboardToken(c *fiber.Ctx) error {
	if h.redisClient == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "SSE tokens require Redis",
		})
	}

	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}

	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		h.logger.Error("Failed to generate dashboard SSE token", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "token generation failed"})
	}
	tokenValue := "sset_" + hex.EncodeToString(rawBytes)

	tokenData := sseTokenData{UserID: userID, AppID: "dashboard"}
	payload, _ := json.Marshal(tokenData)

	redisKey := sseTokenPrefix + tokenValue
	if err := h.redisClient.Set(c.Context(), redisKey, payload, sseTokenTTL).Err(); err != nil {
		h.logger.Error("Failed to store dashboard SSE token in Redis", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "token storage failed"})
	}

	h.logger.Info("Dashboard SSE token issued", zap.String("user_id", userID))

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"sse_token":  tokenValue,
		"user_id":    userID,
		"expires_in": int(sseTokenTTL.Seconds()),
	})
}

// AdminActivityFeed streams real-time notification status events to admin
// dashboards via SSE. Subscribes to "notification:activity" Redis pub/sub channel.
func (h *SSEHandler) AdminActivityFeed(c *fiber.Ctx) error {
	if h.redisClient == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "activity feed not available (no Redis connection)",
		})
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Access-Control-Allow-Origin", "*")
	c.Set("Access-Control-Allow-Headers", "Cache-Control")

	if c.Context().Conn() != nil {
		_ = c.Context().Conn().SetWriteDeadline(time.Time{})
	}

	ctxDone := c.Context().Done()
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		subCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pubsub := h.redisClient.Subscribe(subCtx, "notification:activity")
		defer pubsub.Close()

		ch := pubsub.Channel()

		h.logger.Info("Admin activity feed SSE client connected")

		// Named event so clients can use addEventListener("connected", ...)
		fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
		w.Flush()

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "event: activity\ndata: %s\n\n", msg.Payload)
				w.Flush()

			case <-ticker.C:
				fmt.Fprintf(w, ": keepalive\n\n")
				w.Flush()

			case <-ctxDone:
				h.logger.Info("Admin activity feed SSE client disconnected")
				return
			}
		}
	})

	return nil
}
