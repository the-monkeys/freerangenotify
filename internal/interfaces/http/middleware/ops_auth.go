package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// OpsAuth validates machine-to-machine ops authorization header.
// Expected format: Authorization: Bearer ops:<secret>
func OpsAuth(expectedSecret string, tolerance time.Duration, logger *zap.Logger) fiber.Handler {
	expected := strings.TrimSpace(expectedSecret)
	if tolerance <= 0 {
		tolerance = 5 * time.Minute
	}

	var nonces sync.Map

	return func(c *fiber.Ctx) error {
		if expected == "" {
			logger.Error("OpsAuth misconfigured: empty ops secret")
			return errors.Unauthorized("Ops authentication is not configured")
		}

		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return errors.Unauthorized("Missing Authorization header")
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			return errors.Unauthorized("Invalid Authorization header format")
		}

		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if !strings.HasPrefix(token, "ops:") {
			return errors.Unauthorized("Invalid ops token format")
		}

		provided := strings.TrimSpace(strings.TrimPrefix(token, "ops:"))
		if provided == "" {
			return errors.Unauthorized("Invalid ops token")
		}

		if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			logger.Warn("OpsAuth denied request",
				zap.String("ip", c.IP()),
				zap.String("path", c.Path()),
				zap.String("method", c.Method()),
			)
			return errors.Unauthorized("Invalid ops token")
		}

		timestampRaw := strings.TrimSpace(c.Get("X-Ops-Timestamp"))
		nonce := strings.TrimSpace(c.Get("X-Ops-Nonce"))
		sig := strings.TrimSpace(c.Get("X-Ops-Signature"))
		if timestampRaw == "" || nonce == "" || sig == "" {
			return errors.Unauthorized("Missing ops replay-protection headers")
		}

		ts, err := strconv.ParseInt(timestampRaw, 10, 64)
		if err != nil {
			return errors.Unauthorized("Invalid X-Ops-Timestamp")
		}
		now := time.Now().UTC().Unix()
		if absInt64(now-ts) > int64(tolerance.Seconds()) {
			return errors.Unauthorized("Stale ops request")
		}

		if expRaw, found := nonces.Load(nonce); found {
			if exp, ok := expRaw.(int64); ok && exp > now {
				return errors.Unauthorized("Replay detected")
			}
		}

		reqURI := c.OriginalURL()
		expectedSig := signOpsMessage(expected, c.Method(), reqURI, timestampRaw, nonce)
		if subtle.ConstantTimeCompare([]byte(strings.ToLower(sig)), []byte(strings.ToLower(expectedSig))) != 1 {
			return errors.Unauthorized("Invalid ops signature")
		}

		nonces.Store(nonce, now+int64(tolerance.Seconds()))
		nonces.Range(func(key, value interface{}) bool {
			exp, ok := value.(int64)
			if ok && exp <= now {
				nonces.Delete(key)
			}
			return true
		})

		c.Locals("auth_plane", "ops")
		return c.Next()
	}
}

func signOpsMessage(secret, method, requestURI, ts, nonce string) string {
	message := fmt.Sprintf("%s\n%s\n%s\n%s", strings.ToUpper(strings.TrimSpace(method)), strings.TrimSpace(requestURI), strings.TrimSpace(ts), strings.TrimSpace(nonce))
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	_, _ = mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
