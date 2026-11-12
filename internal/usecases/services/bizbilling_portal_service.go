package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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
		Token:     "pt2_" + base64.RawURLEncoding.EncodeToString(raw),
		Version:   2,
		AppID:     appID,
		UserID:    userID,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
	}
	token.ID = portalTokenDigest(token.Token)
	stored := *token
	stored.Token = ""
	if err := s.tokens.Create(ctx, token.ID, &stored); err != nil {
		return nil, err
	}
	return token, nil
}

// Resolve validates a token and returns it. Expired or unknown tokens fail.
func (s *BizPortalService) Resolve(ctx context.Context, token string) (*bizbilling.PortalToken, error) {
	if token == "" || len(token) > 256 || strings.ContainsAny(token, "/\\?#%\r\n\t ") {
		return nil, errors.Unauthorized("portal token required")
	}
	id := portalTokenDigest(token)
	pt, err := s.tokens.Get(ctx, id)
	if err != nil && !strings.HasPrefix(token, "pt2_") {
		// Historical links are read only at their original key and expiry.
		// Never migrate them by minting a replacement credential on access.
		id = token
		pt, err = s.tokens.Get(ctx, id)
		if err == nil && (pt.Version != 0 || pt.Token != token) {
			return nil, errors.Unauthorized("invalid portal token")
		}
	}
	if err != nil {
		return nil, errors.Unauthorized("invalid portal token")
	}
	if pt.AppID == "" || pt.UserID == "" || pt.RevokedAt != nil || !time.Now().UTC().Before(pt.ExpiresAt) || (pt.Version == 2 && pt.ID != id) {
		return nil, errors.Unauthorized("portal token expired")
	}
	pt.ID = id
	pt.Token = token // compatibility return only; never written back
	return pt, nil
}

func portalTokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "v2_" + hex.EncodeToString(sum[:])
}

func (s *BizPortalService) ResolveScope(ctx context.Context, token string) (bizbilling.PortalScope, error) {
	pt, err := s.Resolve(ctx, token)
	if err != nil {
		return bizbilling.PortalScope{}, err
	}
	return bizbilling.PortalScope{AppID: pt.AppID, UserID: pt.UserID, TokenID: pt.ID, ExpiresAt: pt.ExpiresAt}, nil
}

func (s *BizPortalService) Revoke(ctx context.Context, appID, tokenID string) error {
	if appID == "" || tokenID == "" || len(tokenID) > 256 {
		return errors.NotFound("portal link", "")
	}
	_, err := s.tokens.Update(ctx, tokenID, func(token *bizbilling.PortalToken) error {
		if token.AppID != appID {
			return errors.NotFound("portal link", "")
		}
		if token.RevokedAt == nil {
			now := time.Now().UTC()
			token.RevokedAt = &now
		}
		return nil
	})
	return err
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
