package license

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
)

type selfHostedClaims struct {
	Plan string `json:"plan,omitempty"`
	Mode string `json:"mode,omitempty"`
	jwt.RegisteredClaims
}

// SelfHostedChecker validates locally attached signed license keys.
type SelfHostedChecker struct {
	opts   SelfHostedOptions
	remote remoteVerifier

	mu                sync.RWMutex
	cache             *decisionCacheEntry
	lastRemoteCheckAt time.Time
}

func NewSelfHostedChecker(opts SelfHostedOptions) (Checker, error) {
	if opts.CacheTTL <= 0 {
		opts.CacheTTL = 5 * time.Minute
	}
	if opts.VerifyInterval <= 0 {
		opts.VerifyInterval = 5 * time.Minute
	}
	if opts.GraceWindow < 0 {
		opts.GraceWindow = 0
	}
	if opts.FailMode == "" {
		opts.FailMode = FailModeClosed
	}

	checker := &SelfHostedChecker{opts: opts}
	checker.remote = newHTTPRemoteVerifier(opts.LicenseServerURL, 10*time.Second)
	return checker, nil
}

func (s *SelfHostedChecker) Enabled() bool { return true }

func (s *SelfHostedChecker) Mode() Mode { return ModeSelfHosted }

func (s *SelfHostedChecker) Check(ctx context.Context, _ *application.Application) (Decision, error) {
	now := time.Now().UTC()

	if cached, ok := s.getCache(); ok && now.Sub(cached.fetchedAt) <= s.opts.CacheTTL {
		if s.shouldRunRemoteVerification(now) {
			// Skip cache to enforce periodic remote verification cadence.
		} else {
			d := cached.decision
			d.Source = "cache"
			d.CheckedAt = now
			return d, nil
		}
	}

	decision, err := s.evaluate(ctx, now)
	if err != nil {
		if s.opts.FailMode == FailModeOpen {
			decision = Decision{Allowed: true, Mode: ModeSelfHosted, State: StateGrace, Reason: "license_validation_error", Source: "fail_open", CheckedAt: now}
		} else {
			decision = Decision{Allowed: false, Mode: ModeSelfHosted, State: StateInvalid, Reason: "license_validation_error", Source: "fail_closed", CheckedAt: now}
		}
	}

	s.setCache(decision, now)
	return decision, nil
}

func (s *SelfHostedChecker) evaluate(ctx context.Context, now time.Time) (Decision, error) {
	if s.opts.LicenseKey == "" {
		return Decision{Allowed: false, Mode: ModeSelfHosted, State: StateUnlicensed, Reason: "license_required", Source: "local", CheckedAt: now}, nil
	}

	if s.opts.PublicKeyPEM == "" && s.remote == nil {
		return Decision{}, fmt.Errorf("self-hosted license validation requires public_key_pem or license_server_url")
	}

	decision := Decision{Allowed: true, Mode: ModeSelfHosted, State: StateActive, Reason: "remote_validation_required", Source: "remote", CheckedAt: now}
	if s.opts.PublicKeyPEM != "" {
		localDecision, err := s.evaluateLocal(now)
		if err != nil {
			return Decision{}, err
		}
		if !localDecision.Allowed {
			return localDecision, nil
		}
		decision = localDecision
	}

	if s.remote == nil {
		return decision, nil
	}

	if !s.shouldRunRemoteVerification(now) {
		if decision.Source == "local" {
			decision.Source = "local+remote_cached"
		}
		return decision, nil
	}

	result, remoteErr := s.remote.Verify(ctx, s.opts.LicenseKey)
	s.markRemoteVerification(now)
	if remoteErr != nil {
		if s.opts.FailMode == FailModeOpen {
			return Decision{Allowed: true, Mode: ModeSelfHosted, State: StateGrace, Reason: "remote_verification_unavailable", Source: "fail_open_remote", CheckedAt: now, ValidUntil: decision.ValidUntil}, nil
		}
		return Decision{Allowed: false, Mode: ModeSelfHosted, State: StateInvalid, Reason: "remote_verification_unavailable", Source: "remote", CheckedAt: now, ValidUntil: decision.ValidUntil}, nil
	}

	if !result.Allowed {
		reason := strings.TrimSpace(result.Reason)
		if reason == "" {
			reason = "remote_rejected"
		}
		return Decision{Allowed: false, Mode: ModeSelfHosted, State: StateInvalid, Reason: reason, Source: "remote", CheckedAt: now, ValidUntil: result.ValidUntil}, nil
	}

	if result.ValidUntil != nil {
		decision.ValidUntil = result.ValidUntil
	}
	if decision.Source == "local" {
		decision.Source = "local+remote"
	} else {
		decision.Source = "remote"
	}
	decision.Reason = "license_active"
	decision.CheckedAt = now
	return decision, nil
}

func (s *SelfHostedChecker) evaluateLocal(now time.Time) (Decision, error) {
	if s.opts.PublicKeyPEM == "" {
		return Decision{}, fmt.Errorf("self-hosted license validation requires public_key_pem")
	}

	claims := &selfHostedClaims{}
	key, err := parseRSAPublicKeyFromPEM(s.opts.PublicKeyPEM)
	if err != nil {
		return Decision{}, fmt.Errorf("parse public key: %w", err)
	}

	token, err := jwt.ParseWithClaims(s.opts.LicenseKey, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return key, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			validUntil := time.Time{}
			if claims.ExpiresAt != nil {
				validUntil = claims.ExpiresAt.Time.UTC()
			}

			if !validUntil.IsZero() && s.opts.FailMode == FailModeOpen && now.Before(validUntil.Add(s.opts.GraceWindow)) {
				return Decision{Allowed: true, Mode: ModeSelfHosted, State: StateGrace, Reason: "license_expired_grace", Source: "local", CheckedAt: now, ValidUntil: &validUntil}, nil
			}

			if !validUntil.IsZero() {
				return Decision{Allowed: false, Mode: ModeSelfHosted, State: StateExpired, Reason: "license_expired", Source: "local", CheckedAt: now, ValidUntil: &validUntil}, nil
			}

			return Decision{Allowed: false, Mode: ModeSelfHosted, State: StateExpired, Reason: "license_expired", Source: "local", CheckedAt: now}, nil
		}

		return Decision{Allowed: false, Mode: ModeSelfHosted, State: StateInvalid, Reason: "invalid_license_signature", Source: "local", CheckedAt: now}, nil
	}

	if token == nil || !token.Valid {
		return Decision{Allowed: false, Mode: ModeSelfHosted, State: StateInvalid, Reason: "invalid_license_signature", Source: "local", CheckedAt: now}, nil
	}

	if claims.ExpiresAt == nil {
		return Decision{Allowed: false, Mode: ModeSelfHosted, State: StateInvalid, Reason: "license_missing_expiry", Source: "local", CheckedAt: now}, nil
	}

	if claims.Mode != "" && claims.Mode != string(ModeSelfHosted) {
		return Decision{Allowed: false, Mode: ModeSelfHosted, State: StateInvalid, Reason: "license_mode_mismatch", Source: "local", CheckedAt: now}, nil
	}

	validUntil := claims.ExpiresAt.Time.UTC()
	if now.After(validUntil) {
		if s.opts.FailMode == FailModeOpen && now.Before(validUntil.Add(s.opts.GraceWindow)) {
			return Decision{Allowed: true, Mode: ModeSelfHosted, State: StateGrace, Reason: "license_expired_grace", Source: "local", CheckedAt: now, ValidUntil: &validUntil}, nil
		}
		return Decision{Allowed: false, Mode: ModeSelfHosted, State: StateExpired, Reason: "license_expired", Source: "local", CheckedAt: now, ValidUntil: &validUntil}, nil
	}

	if claims.NotBefore != nil && now.Before(claims.NotBefore.Time.UTC()) {
		return Decision{Allowed: false, Mode: ModeSelfHosted, State: StateInvalid, Reason: "license_not_yet_valid", Source: "local", CheckedAt: now, ValidUntil: &validUntil}, nil
	}

	return Decision{Allowed: true, Mode: ModeSelfHosted, State: StateActive, Reason: "license_active", Source: "local", CheckedAt: now, ValidUntil: &validUntil}, nil
}

func (s *SelfHostedChecker) shouldRunRemoteVerification(now time.Time) bool {
	if s.remote == nil {
		return false
	}
	if s.opts.VerifyInterval <= 0 {
		return true
	}

	s.mu.RLock()
	last := s.lastRemoteCheckAt
	s.mu.RUnlock()

	if last.IsZero() {
		return true
	}
	return now.Sub(last) >= s.opts.VerifyInterval
}

func (s *SelfHostedChecker) markRemoteVerification(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastRemoteCheckAt = now
}

func parseRSAPublicKeyFromPEM(pemValue string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemValue))
	if block == nil {
		return nil, fmt.Errorf("invalid PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not RSA")
	}
	return rsaPub, nil
}

func (s *SelfHostedChecker) getCache() (decisionCacheEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cache == nil {
		return decisionCacheEntry{}, false
	}
	return *s.cache, true
}

func (s *SelfHostedChecker) setCache(decision Decision, fetchedAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = &decisionCacheEntry{decision: decision, fetchedAt: fetchedAt}
}

// SetLicenseKey updates the in-memory license key for runtime validation.
// Persisting the key across restarts must be handled by config patching.
func (s *SelfHostedChecker) SetLicenseKey(licenseKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opts.LicenseKey = strings.TrimSpace(licenseKey)
	s.cache = nil
	s.lastRemoteCheckAt = time.Time{}
}

// ClearCache forces the next Check call to re-evaluate the license.
func (s *SelfHostedChecker) ClearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = nil
	s.lastRemoteCheckAt = time.Time{}
}
