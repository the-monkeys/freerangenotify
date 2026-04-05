package middleware

import (
	"context"
	"net"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/audit"
	"go.uber.org/zap"
)

// AuditMiddleware logs state-changing HTTP requests (POST, PUT, PATCH, DELETE)
// as audit log entries. GET, HEAD, and OPTIONS requests are skipped.
// Recording is fire-and-forget — failures are logged but never block the response.
func AuditMiddleware(auditService audit.Service, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		method := c.Method()

		// Skip read-only methods
		if method == fiber.MethodGet || method == fiber.MethodHead || method == fiber.MethodOptions {
			return c.Next()
		}

		// Execute the handler first
		err := c.Next()

		// Only audit successful mutations (2xx responses)
		status := c.Response().StatusCode()
		if status < 200 || status >= 300 {
			return err
		}

		// Extract context values set by auth middleware
		appID, _ := c.Locals("app_id").(string)
		actorID := appID // Default: the authenticated app is the actor
		actorType := "api_key"

		// If JWT auth set a user_id, prefer that
		if uid, ok := c.Locals("user_id").(string); ok && uid != "" {
			actorID = uid
			actorType = "user"
		}

		action, resource := parseRoute(method, c.Path())

		entry := &audit.AuditLog{
			AppID:      appID,
			ActorID:    actorID,
			ActorType:  actorType,
			Action:     action,
			Resource:   resource,
			ResourceID: extractResourceID(c.Path()),
			IPAddress:  clientIP(c),
			UserAgent:  c.Get("User-Agent"),
		}

		// Fire-and-forget: record asynchronously so the response is not delayed.
		// IMPORTANT: Do NOT pass c.Context() into the goroutine — Fiber/fasthttp
		// recycles the RequestCtx after the handler returns. By the time the
		// goroutine executes, the context is already pooled and its internals
		// are nil, causing a nil-pointer panic in the Elasticsearch HTTP transport.
		// Use context.Background() instead — audit logs don't need request-scoped values.
		go func() {
			if recordErr := auditService.Record(context.Background(), entry); recordErr != nil {
				logger.Warn("Failed to record audit log",
					zap.String("action", action),
					zap.String("resource", resource),
					zap.Error(recordErr))
			}
		}()

		return err
	}
}

// parseRoute derives a human-readable action and resource from the HTTP method and path.
func parseRoute(method, path string) (action, resource string) {
	switch method {
	case fiber.MethodPost:
		action = "create"
	case fiber.MethodPut, fiber.MethodPatch:
		action = "update"
	case fiber.MethodDelete:
		action = "delete"
	default:
		action = strings.ToLower(method)
	}

	// Extract resource from path.
	// Paths like /v1/notifications/:id → resource = "notification"
	// Paths like /v1/templates → resource = "template"
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// Skip version prefix (e.g., "v1")
	if len(parts) > 1 && strings.HasPrefix(parts[0], "v") {
		parts = parts[1:]
	}

	if len(parts) > 0 {
		resource = strings.TrimSuffix(parts[0], "s") // pluralize → singular
	} else {
		resource = "unknown"
	}

	// Refine action for special sub-paths (e.g., /v1/notifications/:id/send → action = "send")
	if len(parts) >= 3 {
		sub := parts[len(parts)-1]
		// If the last segment is not a UUID/ID, treat it as a sub-action
		if len(sub) < 30 && !strings.Contains(sub, "-") {
			action = sub
		}
	}

	return action, resource
}

// extractResourceID returns the first UUID-like segment from the path.
func extractResourceID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for _, p := range parts {
		// UUID check (simple heuristic: 36 chars with dashes)
		if len(p) == 36 && strings.Count(p, "-") == 4 {
			return p
		}
	}
	return ""
}

// clientIP returns the best-effort client IP.
// If the direct peer is a private/loopback address (typical when behind nginx),
// it will prefer the first X-Forwarded-For / X-Real-IP value. Otherwise it
// returns the peer IP to avoid spoofing.
func clientIP(c *fiber.Ctx) string {
	peer := c.IP()
	if peer == "" {
		return ""
	}

	if !isPrivateOrLoopback(net.ParseIP(peer)) {
		return peer
	}

	xff := c.Get(fiber.HeaderXForwardedFor)
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			if ip := strings.TrimSpace(parts[0]); ip != "" && net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	if rip := c.Get("X-Real-IP"); rip != "" && net.ParseIP(strings.TrimSpace(rip)) != nil {
		return strings.TrimSpace(rip)
	}
	return peer
}

func isPrivateOrLoopback(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	// IPv4 private ranges
	privateRanges := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	for _, cidr := range privateRanges {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			return true
		}
	}
	// IPv6 ULA fc00::/7
	_, ula, _ := net.ParseCIDR("fc00::/7")
	return ula.Contains(ip)
}
