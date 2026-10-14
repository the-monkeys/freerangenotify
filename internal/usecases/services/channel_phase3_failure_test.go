package services

import (
	"context"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type phoneAuthRepository struct {
	auth.Repository
	current *auth.AdminUser
}

func (r *phoneAuthRepository) GetUserByID(context.Context, string) (*auth.AdminUser, error) {
	return r.current, nil
}
func (r *phoneAuthRepository) UpdateUser(context.Context, *auth.AdminUser) error { return nil }

type phonePendingStore struct {
	repository.OTPRepository
	pending *auth.PendingRegistration
}

func (r *phonePendingStore) StorePhoneOTP(_ context.Context, _ string, p *auth.PendingRegistration) error {
	copy := *p
	r.pending = &copy
	return nil
}
func (r *phonePendingStore) GetPhoneOTP(context.Context, string) (*auth.PendingRegistration, error) {
	return r.pending, nil
}
func (r *phonePendingStore) DeletePhoneOTP(context.Context, string) error {
	r.pending = nil
	return nil
}

func TestChannelPhoneOTPExpiredProofCannotBeVerified(t *testing.T) {
	store := &phonePendingStore{pending: &auth.PendingRegistration{Email: "+919876543210", OTPCode: "123456", CreatedAt: time.Now().Add(-11 * time.Minute)}}
	service := &authService{repo: &phoneAuthRepository{current: &auth.AdminUser{UserID: "u"}}, otpRepo: store, logger: zap.NewNop()}
	if err := service.VerifyPhoneOTP(context.Background(), "u", &auth.PhoneVerifyRequest{Phone: "+919876543210", OTPCode: "123456"}); err == nil {
		t.Fatal("expired OTP proof accepted despite ten-minute lifetime")
	}
}

func TestChannelPhoneOTPFailureIsNotSuccess(t *testing.T) {
	for _, key := range []string{"FREERANGE_PROVIDERS_TWILIO_ACCOUNT_SID", "FREERANGE_PROVIDERS_TWILIO_AUTH_TOKEN", "FREERANGE_PROVIDERS_TWILIO_FROM_NUMBER", "FREERANGE_PROVIDERS_WHATSAPP_ACCOUNT_SID", "FREERANGE_PROVIDERS_WHATSAPP_AUTH_TOKEN"} {
		t.Setenv(key, "")
	}
	service := &authService{repo: &phoneAuthRepository{current: &auth.AdminUser{UserID: "u"}}, otpRepo: &phonePendingStore{}, logger: zap.NewNop()}
	if err := service.SendPhoneOTP(context.Background(), "u", &auth.PhoneOTPRequest{Phone: "+919876543210"}); err == nil {
		t.Fatal("unconfigured sender claimed OTP success")
	}
}

func TestChannelDispatchRejectedRequestReleasesHold(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		fmt.Fprint(w, `{"code":21211,"message":"recipient invalid"}`)
	}))
	defer server.Close()
	s := &ChannelDispatchService{Connections: connectionTestService(), Client: &http.Client{Transport: dispatchTransport{server.URL}}}
	n := &notification.Notification{AppID: "a", Channel: notification.ChannelWhatsApp, Content: notification.Content{Data: map[string]interface{}{"content_sid": "HXfixture"}}}
	d, err := s.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+919876543210"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DispatchPrepared(context.Background(), d)
	if err == nil || !providers.IsDefinitiveDeliveryFailure(err) {
		t.Fatalf("known HTTP rejection treated as unknown paid outcome: %v", err)
	}
}

func TestChannelDispatchMalformedMetaCannotBypassWindow(t *testing.T) {
	c := connectionTestService()
	c.Apps.(*connectionApps).app.Settings.WhatsApp = &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "waba", MetaPhoneNumberID: "phone", MetaAccessToken: "fixture-token"}
	s := &ChannelDispatchService{Connections: c}
	n := &notification.Notification{AppID: "a", Channel: notification.ChannelWhatsApp, Content: notification.Content{Body: "text", Data: map[string]interface{}{"whatsapp_template": "malformed"}}}
	if _, err := s.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+919876543210"}); err == nil {
		t.Fatal("malformed template became unguarded free form")
	}
	n.Content.Data = map[string]interface{}{"whatsapp_rich": map[string]interface{}{"kind": "product", "template_name": "fake-template"}}
	if _, err := s.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+919876543210"}); err == nil {
		t.Fatal("interactive product borrowed template permission")
	}
}

func TestChannelDispatchLegacyMetaWithoutWABA(t *testing.T) {
	c := connectionTestService()
	c.Apps.(*connectionApps).app.Settings.WhatsApp = &application.WhatsAppAppConfig{Provider: "meta", MetaPhoneNumberID: "phone", MetaAccessToken: "fixture-token"}
	s := &ChannelDispatchService{Connections: c}
	n := &notification.Notification{AppID: "a", Channel: notification.ChannelWhatsApp, Content: notification.Content{Data: map[string]interface{}{"whatsapp_template": map[string]interface{}{"name": "hello", "language": "en_US"}}}}
	if _, err := s.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+919876543210"}); err != nil {
		t.Fatalf("valid legacy Meta sender blocked: %v", err)
	}
}
