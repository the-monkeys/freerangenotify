package handlers

import (
	"bufio"
	"context"
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

// Connect establishes an SSE connection for a user.
//
// Query params:
//
//	user_id (required) — the internal UUID or external_id of the user.
//	token   (optional) — API key (frn_xxx) for authorization. Scopes the
//	                      connection to the token's app.
//	app_id  (optional) — explicit app ID. When used with token, must match.
//
// Minimal integration (no auth):
//
//	GET /v1/sse?user_id=user002
//
// Authorized integration:
//
//	GET /v1/sse?user_id=user002&token=frn_xxx
//
// External ID:
//
//	GET /v1/sse?user_id=my_platform_username&token=frn_xxx
func (h *SSEHandler) Connect(c *fiber.Ctx) error {
	userID := c.Query("user_id")
	token := c.Query("token")
	appID := c.Query("app_id")

	// ── 1. user_id is always required ──
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "user_id query parameter is required",
		})
	}

	// ── 2. Authorize via API key token (optional) ──
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

	// ── 2b. Resolve user_id: if not a UUID, try external_id lookup ──
	if _, err := uuid.Parse(userID); err != nil && appID != "" && h.userRepo != nil {
		u, lookupErr := h.userRepo.GetByExternalID(c.Context(), appID, userID)
		if lookupErr == nil {
			h.logger.Info("SSE: resolved external_id to internal user_id",
				zap.String("external_id", userID),
				zap.String("user_id", u.UserID))
			userID = u.UserID
		}
		// If lookup fails, keep the original value — backward compatible with
		// clients that use non-UUID custom user_ids as ES document IDs.
	}

	// ── 2c. Phase 1: HMAC subscriber authentication (feature-gated) ──
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
