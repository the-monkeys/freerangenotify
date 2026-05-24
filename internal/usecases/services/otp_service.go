package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/otp"
	templateDomain "github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
)

// notificationSender is the narrow slice of notification.Service that the
// OTP service depends on. Keeping it minimal lets tests substitute a stub
// without implementing every notification.Service method.
type notificationSender interface {
	Send(ctx context.Context, req notification.SendRequest) (*notification.Notification, error)
}

// userResolver is the narrow slice of user.Repository the OTP service uses
// to look up or auto-create recipient users.
type userResolver interface {
	Create(ctx context.Context, u *user.User) error
	GetByID(ctx context.Context, id string) (*user.User, error)
	GetByEmail(ctx context.Context, appID, email string) (*user.User, error)
	GetByExternalID(ctx context.Context, appID, externalID string) (*user.User, error)
}

// templateCreator is the narrow slice of usecases.TemplateService the OTP
// service uses to materialise a transient template per send.
type templateCreator interface {
	Create(ctx context.Context, req *templateDomain.CreateRequest) (*templateDomain.Template, error)
}

// OTPService is the public OTP-as-a-service implementation. It generates the
// verification code, persists a hashed copy in Redis, and dispatches the
// notification through the standard notification pipeline so that credit
// metering, audit logging, retries, and provider routing all work without
// duplication.
type OTPService struct {
	repo            otp.Repository
	notificationSvc notificationSender
	userRepo        userResolver
	templateService templateCreator
	logger          *zap.Logger

	resendCooldownSec   int
	recipientLimitPerHr int
}

// NewOTPService constructs a service. resendCooldownSec defaults to 60s and
// recipientLimitPerHr to 5 when given non-positive values.
func NewOTPService(
	repo otp.Repository,
	notificationSvc notificationSender,
	userRepo userResolver,
	templateService templateCreator,
	logger *zap.Logger,
) *OTPService {
	return &OTPService{
		repo:                repo,
		notificationSvc:     notificationSvc,
		userRepo:            userRepo,
		templateService:     templateService,
		logger:              logger,
		resendCooldownSec:   otp.DefaultResendCooldownS,
		recipientLimitPerHr: 5,
	}
}

// e164Re is a permissive E.164 validator: leading +, then 8-15 digits. Strict
// per-country validation is the caller's responsibility.
var e164Re = regexp.MustCompile(`^\+[1-9]\d{7,14}$`)

// Send creates a new OTP, dispatches it through the notification pipeline,
// and persists the hashed record.
func (s *OTPService) Send(ctx context.Context, in otp.SendInput) (*otp.SendResult, error) {
	if err := s.validateSendInput(&in); err != nil {
		return nil, err
	}

	// Resolve the recipient up-front so rate limiting, persistence, and the
	// notification dispatch all key off the SAME channel address. When the
	// caller passes user_id / external_id, the channel-appropriate field on
	// the user record (email or phone) becomes the canonical Recipient.
	resolvedUserID, resolvedAddress, err := s.resolveRecipient(ctx, &in)
	if err != nil {
		return nil, err
	}
	in.Recipient = resolvedAddress

	// Rate-limit per app+recipient (default 5 sends / hour). Hits before any
	// expensive work so abusive callers don't burn credits or fill Redis.
	if count, err := s.repo.RecipientRateLimit(ctx, in.AppID, in.Recipient, 3600); err != nil {
		s.logger.Warn("otp: rate-limit check failed (allowing)", zap.Error(err))
	} else if count > s.recipientLimitPerHr {
		return nil, otp.ErrRateLimited
	}

	// Generate the plaintext code, hash it, and forget the plaintext on
	// return (we render it once into the notification body below). The code
	// itself never lands in any log line.
	code, err := generateCode(in.Length, in.Alphanumeric)
	if err != nil {
		return nil, fmt.Errorf("otp: generate code: %w", err)
	}
	salt, err := randomHex(16)
	if err != nil {
		return nil, fmt.Errorf("otp: generate salt: %w", err)
	}
	codeHash := hashCode(code, salt)

	now := time.Now().UTC()
	ttlSec := in.TTLSeconds
	if ttlSec == 0 {
		ttlSec = otp.DefaultTTLSeconds
	}
	maxAttempts := in.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = otp.DefaultMaxAttempts
	}

	req := &otp.Request{
		RequestID:    "otp_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		AppID:        in.AppID,
		Channel:      in.Channel,
		Recipient:    in.Recipient,
		CodeHash:     codeHash,
		Salt:         salt,
		Length:       in.Length,
		Alphanumeric: in.Alphanumeric,
		Attempts:     0,
		MaxAttempts:  maxAttempts,
		ExpiresAt:    now.Add(time.Duration(ttlSec) * time.Second),
		LastSentAt:   now,
		CreatedAt:    now,
	}

	notif, err := s.dispatchNotification(ctx, req, resolvedUserID, code, in.TemplateBody, in.TemplateData)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, req); err != nil {
		// Code already left the building via the notification pipeline. Best
		// we can do is log and surface the error so the caller knows verify
		// will fail.
		s.logger.Error("otp: persist after dispatch failed",
			zap.String("request_id", req.RequestID),
			zap.String("notification_id", notif),
			zap.Error(err))
		return nil, fmt.Errorf("otp: persist: %w", err)
	}

	s.logger.Info("otp: sent",
		zap.String("request_id", req.RequestID),
		zap.String("notification_id", notif),
		zap.String("channel", string(req.Channel)),
		zap.Int("ttl_seconds", ttlSec),
		zap.Int("max_attempts", maxAttempts),
		// recipient is intentionally NOT logged at info level — it's PII.
	)

	return &otp.SendResult{
		RequestID:      req.RequestID,
		NotificationID: notif,
		Channel:        req.Channel,
		ExpiresAt:      req.ExpiresAt,
		TTLSeconds:     ttlSec,
		MaxAttempts:    maxAttempts,
	}, nil
}

// Verify checks a code against the stored hash with attempt counting and
// constant-time comparison. Idempotent: a second successful verify for an
// already-verified request returns success without re-incrementing attempts.
func (s *OTPService) Verify(ctx context.Context, in otp.VerifyInput) (*otp.VerifyResult, error) {
	requestID := strings.TrimSpace(in.RequestID)
	code := strings.TrimSpace(in.Code)
	if requestID == "" || code == "" {
		return nil, otp.ErrInvalidCode
	}

	req, err := s.repo.Get(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if time.Now().UTC().After(req.ExpiresAt) {
		return nil, otp.ErrExpired
	}

	if req.Verified {
		// Idempotent: return prior success without consuming an attempt.
		return &otp.VerifyResult{
			Verified:          true,
			AttemptsRemaining: req.MaxAttempts - req.Attempts,
			VerifiedAt:        req.VerifiedAt,
		}, nil
	}

	// Atomically increment first — this prevents brute-force races where
	// concurrent verifies would otherwise share a single attempt budget.
	attempts, err := s.repo.IncrementAttempts(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if attempts > req.MaxAttempts {
		// Burn the record on exhaustion so the same request_id cannot be
		// retried after recovering somehow.
		_ = s.repo.Delete(ctx, requestID)
		return nil, otp.ErrAttemptsExhausted
	}

	expectedHash, err := hex.DecodeString(req.CodeHash)
	if err != nil {
		return nil, fmt.Errorf("otp: corrupt hash: %w", err)
	}
	actualHash := sha256.Sum256([]byte(code + req.Salt))

	if subtle.ConstantTimeCompare(expectedHash, actualHash[:]) != 1 {
		return &otp.VerifyResult{
			Verified:          false,
			AttemptsRemaining: req.MaxAttempts - attempts,
		}, otp.ErrInvalidCode
	}

	now := time.Now().UTC()
	if err := s.repo.MarkVerified(ctx, requestID, now); err != nil {
		s.logger.Error("otp: mark verified failed", zap.String("request_id", requestID), zap.Error(err))
		return nil, fmt.Errorf("otp: persist verify: %w", err)
	}

	s.logger.Info("otp: verified",
		zap.String("request_id", requestID),
		zap.String("channel", string(req.Channel)),
	)

	return &otp.VerifyResult{
		Verified:          true,
		AttemptsRemaining: req.MaxAttempts - attempts,
		VerifiedAt:        &now,
	}, nil
}

// Resend dispatches the SAME code through a fresh notification. Re-issuing
// a new code on resend would invalidate any prior delivery and double-charge
// the customer; instead we reset the attempt counter and re-render.
func (s *OTPService) Resend(ctx context.Context, requestID string) (*otp.SendResult, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil, otp.ErrNotFound
	}
	req, err := s.repo.Get(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if req.Verified {
		return nil, otp.ErrAlreadyVerified
	}
	if time.Now().UTC().After(req.ExpiresAt) {
		return nil, otp.ErrExpired
	}
	cooldownEnd := req.LastSentAt.Add(time.Duration(s.resendCooldownSec) * time.Second)
	if time.Now().UTC().Before(cooldownEnd) {
		return nil, otp.ErrResendCooldown
	}

	// Resend uses the same code — but the plaintext was discarded after the
	// initial send. We cannot recover it; the only safe path is to abort.
	// To support true "resend same code" semantics we would need to keep the
	// plaintext (or a reversible encryption of it) until expiry, which
	// materially weakens the security posture. Instead resend mints a fresh
	// code while preserving the request_id — most providers (Twilio Verify,
	// Auth0, Firebase) work this way.
	code, err := generateCode(req.Length, req.Alphanumeric)
	if err != nil {
		return nil, fmt.Errorf("otp: generate code: %w", err)
	}
	salt, err := randomHex(16)
	if err != nil {
		return nil, fmt.Errorf("otp: generate salt: %w", err)
	}
	req.CodeHash = hashCode(code, salt)
	req.Salt = salt
	req.Attempts = 0
	req.LastSentAt = time.Now().UTC()

	notifID, err := s.dispatchNotification(ctx, req, "", code, "", nil)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, req); err != nil {
		s.logger.Error("otp: resend persist failed",
			zap.String("request_id", req.RequestID),
			zap.String("notification_id", notifID),
			zap.Error(err))
		return nil, fmt.Errorf("otp: persist: %w", err)
	}

	s.logger.Info("otp: resent",
		zap.String("request_id", req.RequestID),
		zap.String("notification_id", notifID),
		zap.String("channel", string(req.Channel)),
	)

	return &otp.SendResult{
		RequestID:      req.RequestID,
		NotificationID: notifID,
		Channel:        req.Channel,
		ExpiresAt:      req.ExpiresAt,
		TTLSeconds:     int(time.Until(req.ExpiresAt).Seconds()),
		MaxAttempts:    req.MaxAttempts,
	}, nil
}

// resolveRecipient turns the SendInput's identifier triple (recipient /
// user_id / external_id) into the concrete (userID, channelAddress) pair the
// rest of the pipeline needs. Exactly one of the three is expected to be
// populated — that contract is enforced in validateSendInput.
//
// When `recipient` is supplied, no lookup happens here; userID is left empty
// so dispatchNotification will fall through to its legacy auto-create path.
// When `user_id` or `external_id` is supplied, the user is loaded and the
// channel-appropriate field (Email for email; Phone for sms/whatsapp) is
// returned as the recipient.
func (s *OTPService) resolveRecipient(ctx context.Context, in *otp.SendInput) (userID, address string, err error) {
	switch {
	case in.UserID != "":
		u, lookupErr := s.userRepo.GetByID(ctx, in.UserID)
		if lookupErr != nil || u == nil {
			return "", "", otp.ErrUserNotFound
		}
		// Tenancy guard: a user_id must belong to the calling app. Without
		// this check, a leaked user_id from one tenant could be probed
		// across other tenants' API keys.
		if u.AppID != in.AppID {
			return "", "", otp.ErrUserNotFound
		}
		addr, addrErr := channelAddressForUser(u, in.Channel)
		if addrErr != nil {
			return "", "", addrErr
		}
		return u.UserID, addr, nil

	case in.ExternalID != "":
		u, lookupErr := s.userRepo.GetByExternalID(ctx, in.AppID, in.ExternalID)
		if lookupErr != nil || u == nil {
			return "", "", otp.ErrUserNotFound
		}
		addr, addrErr := channelAddressForUser(u, in.Channel)
		if addrErr != nil {
			return "", "", addrErr
		}
		return u.UserID, addr, nil

	default:
		// Legacy path: caller supplied a raw recipient. Defer user resolution
		// to dispatchNotification (which auto-creates when missing).
		return "", in.Recipient, nil
	}
}

// channelAddressForUser pulls the contact address from a user record that
// corresponds to the requested OTP channel.
func channelAddressForUser(u *user.User, channel otp.Channel) (string, error) {
	switch channel {
	case otp.ChannelEmail:
		if strings.TrimSpace(u.Email) == "" {
			return "", otp.ErrUserMissingChannelAddress
		}
		return u.Email, nil
	case otp.ChannelSMS, otp.ChannelWhatsApp:
		if strings.TrimSpace(u.Phone) == "" {
			return "", otp.ErrUserMissingChannelAddress
		}
		return u.Phone, nil
	}
	return "", otp.ErrInvalidChannel
}

// dispatchNotification resolves the recipient to an internal user (auto-
// creating an ephemeral record when needed), creates a transient template
// with the rendered OTP body, and delegates to notification.Service.Send.
// Returns the notification ID. Code plaintext stays scoped to this function.
//
// If preResolvedUserID is non-empty, the recipient-lookup step is skipped —
// this is the path taken when the caller supplied user_id / external_id and
// Send already resolved the user.
func (s *OTPService) dispatchNotification(
	ctx context.Context,
	req *otp.Request,
	preResolvedUserID string,
	code string,
	customTemplateBody string,
	templateData map[string]interface{},
) (string, error) {
	userID := preResolvedUserID
	if userID == "" {
		var err error
		userID, err = s.resolveOrCreateRecipientUser(ctx, req.AppID, req.Channel, req.Recipient)
		if err != nil {
			return "", err
		}
	}

	body := renderOTPBody(customTemplateBody, code, int(time.Until(req.ExpiresAt).Minutes()))
	subject := "Your verification code"

	channelString := channelToNotificationChannel(req.Channel)
	tmplName := fmt.Sprintf("_otp_%s", uuid.NewString()[:8])
	tmpl, err := s.templateService.Create(ctx, &templateDomain.CreateRequest{
		AppID:     req.AppID,
		Name:      tmplName,
		Channel:   channelString,
		Subject:   subject,
		Body:      body,
		Locale:    "en",
		CreatedBy: "system:otp",
	})
	if err != nil {
		return "", fmt.Errorf("otp: create transient template: %w", err)
	}

	data := map[string]interface{}{}
	for k, v := range templateData {
		data[k] = v
	}
	// `code` is intentionally exposed to the template renderer so BYO
	// templates that include {{code}} resolve correctly. Do not place the
	// raw code into structured fields like Notification.Content.Data that
	// might be persisted long-term beyond Body rendering.
	data["code"] = code

	sendReq := notification.SendRequest{
		AppID:      req.AppID,
		UserID:     userID,
		Channel:    notification.Channel(channelString),
		Priority:   notification.PriorityHigh, // OTPs are time-sensitive
		TemplateID: tmpl.ID,
		Data:       data,
		Metadata: map[string]interface{}{
			"otp_request_id": req.RequestID,
		},
	}
	notif, err := s.notificationSvc.Send(ctx, sendReq)
	if err != nil {
		return "", fmt.Errorf("otp: dispatch notification: %w", err)
	}
	return notif.NotificationID, nil
}

func (s *OTPService) resolveOrCreateRecipientUser(
	ctx context.Context,
	appID string,
	channel otp.Channel,
	recipient string,
) (string, error) {
	true_ := true
	prefs := user.Preferences{
		EmailEnabled:    &true_,
		SMSEnabled:      &true_,
		WhatsAppEnabled: &true_,
	}

	// Email path uses the email index for de-dup.
	if channel == otp.ChannelEmail {
		existing, err := s.userRepo.GetByEmail(ctx, appID, recipient)
		if err == nil && existing != nil {
			return existing.UserID, nil
		}
		now := time.Now().UTC()
		u := &user.User{
			UserID:      uuid.NewString(),
			AppID:       appID,
			Email:       recipient,
			Preferences: prefs,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := s.userRepo.Create(ctx, u); err != nil {
			return "", fmt.Errorf("otp: auto-create email user: %w", err)
		}
		return u.UserID, nil
	}

	// Phone path (SMS + WhatsApp) — use external_id == E.164 as a stable
	// de-dup key. The same user is reused across both channels.
	existing, err := s.userRepo.GetByExternalID(ctx, appID, recipient)
	if err == nil && existing != nil {
		return existing.UserID, nil
	}
	now := time.Now().UTC()
	u := &user.User{
		UserID:      uuid.NewString(),
		AppID:       appID,
		ExternalID:  recipient,
		Phone:       recipient,
		Preferences: prefs,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.userRepo.Create(ctx, u); err != nil {
		return "", fmt.Errorf("otp: auto-create phone user: %w", err)
	}
	return u.UserID, nil
}

func (s *OTPService) validateSendInput(in *otp.SendInput) error {
	in.AppID = strings.TrimSpace(in.AppID)
	in.Recipient = strings.TrimSpace(in.Recipient)
	in.UserID = strings.TrimSpace(in.UserID)
	in.ExternalID = strings.TrimSpace(in.ExternalID)
	if in.AppID == "" {
		return fmt.Errorf("otp: app_id required")
	}
	if !in.Channel.Valid() {
		return otp.ErrInvalidChannel
	}
	// Exactly one of recipient / user_id / external_id must be supplied.
	supplied := 0
	if in.Recipient != "" {
		supplied++
	}
	if in.UserID != "" {
		supplied++
	}
	if in.ExternalID != "" {
		supplied++
	}
	switch supplied {
	case 0:
		return otp.ErrInvalidRecipient
	case 1:
		// ok
	default:
		return otp.ErrAmbiguousRecipient
	}
	// Format validation only applies to the raw-recipient path; for the
	// user_id / external_id paths the address is read from a vetted user
	// record after lookup.
	if in.Recipient != "" {
		if err := validateRecipient(in.Channel, in.Recipient); err != nil {
			return err
		}
	}
	if in.Length == 0 {
		in.Length = otp.DefaultCodeLength
	}
	if in.Length < otp.MinCodeLength || in.Length > otp.MaxCodeLength {
		return otp.ErrInvalidLength
	}
	if in.TTLSeconds == 0 {
		in.TTLSeconds = otp.DefaultTTLSeconds
	}
	if in.TTLSeconds < 30 || in.TTLSeconds > otp.MaxTTLSeconds {
		return otp.ErrInvalidTTL
	}
	if in.MaxAttempts == 0 {
		in.MaxAttempts = otp.DefaultMaxAttempts
	}
	if in.MaxAttempts < otp.MinAttempts || in.MaxAttempts > otp.MaxAttemptsCap {
		return otp.ErrInvalidAttempts
	}
	if in.TemplateBody != "" && !strings.Contains(in.TemplateBody, "{{code}}") &&
		!strings.Contains(in.TemplateBody, "{{ code }}") &&
		!strings.Contains(in.TemplateBody, "{{.code}}") {
		return otp.ErrTemplateMissingCode
	}
	return nil
}

func validateRecipient(channel otp.Channel, recipient string) error {
	if recipient == "" {
		return otp.ErrInvalidRecipient
	}
	switch channel {
	case otp.ChannelEmail:
		if _, err := mail.ParseAddress(recipient); err != nil {
			return otp.ErrInvalidRecipient
		}
	case otp.ChannelSMS, otp.ChannelWhatsApp:
		if !e164Re.MatchString(recipient) {
			return otp.ErrInvalidRecipient
		}
	}
	return nil
}

// generateCode produces a cryptographically secure numeric or alphanumeric
// code of the given length using crypto/rand. The alphanumeric alphabet
// excludes look-alike characters (0/O, 1/I/l) to reduce user transcription
// errors.
func generateCode(length int, alphanumeric bool) (string, error) {
	const numeric = "0123456789"
	const alnum = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no 0,O,1,I,l
	alphabet := numeric
	if alphanumeric {
		alphabet = alnum
	}
	mod := big.NewInt(int64(len(alphabet)))
	out := make([]byte, length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, mod)
		if err != nil {
			return "", err
		}
		out[i] = alphabet[n.Int64()]
	}
	return string(out), nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashCode(code, salt string) string {
	h := sha256.Sum256([]byte(code + salt))
	return hex.EncodeToString(h[:])
}

func renderOTPBody(custom, code string, ttlMinutes int) string {
	if ttlMinutes < 1 {
		ttlMinutes = 1
	}
	if custom == "" {
		return fmt.Sprintf("Your verification code is: %s. It expires in %d minute(s). Do not share this code with anyone.", code, ttlMinutes)
	}
	body := custom
	for _, ph := range []string{"{{code}}", "{{ code }}", "{{.code}}"} {
		body = strings.ReplaceAll(body, ph, code)
	}
	for _, ph := range []string{"{{ttl}}", "{{ ttl }}", "{{.ttl}}"} {
		body = strings.ReplaceAll(body, ph, fmt.Sprintf("%d", ttlMinutes))
	}
	return body
}

func channelToNotificationChannel(c otp.Channel) string {
	switch c {
	case otp.ChannelSMS:
		return "sms"
	case otp.ChannelWhatsApp:
		return "whatsapp"
	case otp.ChannelEmail:
		return "email"
	}
	return ""
}

// Ensure interface compliance at compile time.
var _ otp.Service = (*OTPService)(nil)
