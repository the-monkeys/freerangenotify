package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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
// service previously used for templates. Retained for constructor compatibility.
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
	return s.sendOTP(ctx, in)
}

// Verify atomically consumes an attempt and records verification.
func (s *OTPService) Verify(ctx context.Context, in otp.VerifyInput) (*otp.VerifyResult, error) {
	return s.verifyOTP(ctx, in)
}

// Resend replaces the code only after a fenced delivery is accepted.
func (s *OTPService) Resend(ctx context.Context, requestID string) (*otp.SendResult, error) {
	return s.resendOTP(ctx, requestID)
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

// dispatchNotification resolves the recipient when necessary and sends rendered
// direct content without creating a permanent per-code template.
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

	data := map[string]interface{}{}
	for k, v := range templateData {
		data[k] = v
	}
	// `code` is intentionally exposed to the template renderer so BYO
	// templates that include {{code}} resolve correctly. Do not place the
	// raw code into structured fields like Notification.Content.Data that
	// might be persisted long-term beyond Body rendering.
	data["code"] = code
	body = renderOTPVariables(body, data).(string)
	for k, v := range data {
		if k != "code" {
			data[k] = renderOTPVariables(v, data)
		}
	}

	sendReq := notification.SendRequest{
		AppID:         req.AppID,
		EnvironmentID: req.EnvironmentID,
		UserID:        userID,
		Channel:       notification.Channel(channelString),
		Priority:      notification.PriorityHigh, // OTPs are time-sensitive
		Title:         subject,
		Body:          body,
		Data:          data,
		Metadata: map[string]interface{}{
			"otp_request_id":   req.RequestID,
			"otp_operation_id": req.Delivery.ID,
			"otp_expires_at":   req.ExpiresAt.UTC().Format(time.RFC3339Nano),
		},
	}
	notif, err := s.notificationSvc.Send(notification.WithDispatchIdentity(ctx, req.Delivery.NotificationID), sendReq)
	if err != nil {
		return "", fmt.Errorf("otp: dispatch notification: %w", err)
	}
	if notif == nil {
		return "", otp.ErrInvalidRecipient
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

func otpRequestInScope(ctx context.Context, req *otp.Request) bool {
	scope, scoped := otp.ScopeFromContext(ctx)
	if !scoped {
		return true
	} // Backward-compatible trusted internal service calls.
	return scope.AppID != "" && req.AppID == scope.AppID && (req.EnvironmentID == "" || req.EnvironmentID == scope.EnvironmentID)
}

// Render each string once so structured WhatsApp parameters remain typed.
func renderOTPVariables(value interface{}, data map[string]interface{}) interface{} {
	pairs := []string{}
	for k, v := range data {
		switch v.(type) {
		case string, float64, int, bool:
			for _, key := range []string{"{{" + k + "}}", "{{ " + k + " }}", "{{." + k + "}}"} {
				pairs = append(pairs, key, fmt.Sprint(v))
			}
		}
	}
	switch x := value.(type) {
	case string:
		return strings.NewReplacer(pairs...).Replace(x)
	case map[string]interface{}:
		out := map[string]interface{}{}
		for k, v := range x {
			out[k] = renderOTPVariables(v, data)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, v := range x {
			out[i] = renderOTPVariables(v, data)
		}
		return out
	default:
		return value
	}
}
