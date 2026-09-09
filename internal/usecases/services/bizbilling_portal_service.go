package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// DefaultPortalTokenTTL is how long a customer portal link stays valid.
const DefaultPortalTokenTTL = 30 * 24 * time.Hour

// BizPortalService manages customer self-service portal tokens.
// Tokens are opaque, random, stored server-side, and time-limited — the
// portal middleware resolves them to (app_id, user_id).
type BizPortalService struct {
	tokens        *bizbillingrepo.Store[bizbilling.PortalToken]
	portalBaseURL string
	logger        *zap.Logger
}

// NewBizPortalService creates a new BizPortalService.
func NewBizPortalService(stores *bizbillingrepo.Stores, portalBaseURL string, logger *zap.Logger) *BizPortalService {
	return &BizPortalService{
		tokens:        stores.PortalTokens,
		portalBaseURL: strings.TrimRight(portalBaseURL, "/"),
		logger:        logger,
	}
}

// CreateToken mints a portal token for a user. ttl <= 0 uses the default.
func (s *BizPortalService) CreateToken(ctx context.Context, appID, userID string, ttl time.Duration) (*bizbilling.PortalToken, error) {
	if appID == "" || userID == "" {
		return nil, errors.BadRequest("app_id and user_id are required")
	}
	if ttl <= 0 {
		ttl = DefaultPortalTokenTTL
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("bizbilling: generate portal token: %w", err)
	}

	now := time.Now().UTC()
	token := &bizbilling.PortalToken{
		Token:     base64.RawURLEncoding.EncodeToString(raw),
		AppID:     appID,
		UserID:    userID,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
	}
	if err := s.tokens.Create(ctx, token.Token, token); err != nil {
		return nil, err
	}
	return token, nil
}

// Resolve validates a token and returns it. Expired or unknown tokens fail.
func (s *BizPortalService) Resolve(ctx context.Context, token string) (*bizbilling.PortalToken, error) {
	if token == "" {
		return nil, errors.Unauthorized("portal token required")
	}
	pt, err := s.tokens.Get(ctx, token)
	if err != nil {
		return nil, errors.Unauthorized("invalid portal token")
	}
	if time.Now().UTC().After(pt.ExpiresAt) {
		return nil, errors.Unauthorized("portal token expired")
	}
	return pt, nil
}

// PortalLink returns a full portal URL for the token, or just the token when
// no portal base URL is configured.
func (s *BizPortalService) PortalLink(token string) string {
	if s.portalBaseURL == "" {
		return ""
	}
	return s.portalBaseURL + "/" + token
}

// PaymentLinkFor mints a fresh portal token for the invoice's user and
// returns a portal URL. Returns "" when no portal base URL is configured.
func (s *BizPortalService) PaymentLinkFor(ctx context.Context, appID, userID, invoiceID string) string {
	if s.portalBaseURL == "" {
		return ""
	}
	token, err := s.CreateToken(ctx, appID, userID, 0)
	if err != nil {
		s.logger.Warn("bizbilling: could not mint payment link token",
			zap.String("app_id", appID), zap.String("invoice_id", invoiceID), zap.Error(err))
		return ""
	}
	return fmt.Sprintf("%s/%s/invoices/%s", s.portalBaseURL, token.Token, invoiceID)
}
