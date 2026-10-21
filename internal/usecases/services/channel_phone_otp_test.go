package services

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func phoneFixture(t *testing.T) (*PhoneOTPDispatcher, *atomic.Int32) {
	t.Helper()
	if os.Getenv("FRN_PHONE_OTP_INTEGRATION") != "1" {
		t.Skip("requires disposable OTP Redis fixture")
	}
	if os.Getenv("FRN_PHONE_OTP_REDIS_ADDR") != "127.0.0.1:16388" {
		t.Fatal("only designated disposable Redis permitted")
	}
	rc := redis.NewClient(&redis.Options{Addr: "127.0.0.1:16388"})
	t.Cleanup(func() { rc.Close() })
	calls := new(atomic.Int32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		r.ParseForm()
		if !strings.HasPrefix(r.Form.Get("To"), "+91") || !strings.Contains(r.Form.Get("Body"), "verification code") {
			t.Error("plus-phone or OTP body changed")
		}
		fmt.Fprint(w, `{"sid":"SMfixture","status":"queued"}`)
	}))
	t.Cleanup(server.Close)
	snap := &channelconnection.Snapshot{AppID: "__platform_auth", EnvironmentID: fmt.Sprintf("fixture-%s-%d", t.Name(), time.Now().UnixNano()), Channel: notification.ChannelSMS, Provider: "twilio", CredentialSource: "system", AccountID: "fixture-account", SenderID: "+919000000001", Credentials: map[string]string{"auth_token": "fixture-token"}}
	d := &PhoneOTPDispatcher{Dispatch: &ChannelDispatchService{Client: &http.Client{Transport: dispatchTransport{server.URL}}, Logger: zap.NewNop()}, Redis: rc, Pending: repository.NewOTPRepository(rc), Snapshot: snap, Policy: PhoneOTPPolicy{AllowedPrefixes: []string{"+91"}, DailyBudgetMinor: 1000, CostMinorPerSegment: 1, Currency: "INR"}}
	return d, calls
}

func TestChannelPhoneOTPDispatch(t *testing.T) {
	d, calls := phoneFixture(t)
	ctx := WithPhoneOTPClientIP(context.Background(), "127.0.0.1")
	core, logs := observer.New(zap.DebugLevel)
	d.Dispatch.Logger = zap.New(core)
	if err := d.DispatchPhoneVerificationOTP(ctx, "fixture-user", "+919876543210", "123456"); err != nil {
		t.Fatal(err)
	}
	if err := d.DispatchPhoneVerificationOTP(ctx, "fixture-user", "+919876543210", "123456"); err != nil || calls.Load() != 1 {
		t.Fatalf("replay sent again: %d %v", calls.Load(), err)
	}
	pending, err := d.Pending.GetPhoneOTP(ctx, "fixture-user")
	if err != nil || pending == nil || pending.OTPCode != "123456" {
		t.Fatal("accepted OTP missing verification proof")
	}
	for _, entry := range logs.All() {
		if strings.Contains(fmt.Sprint(entry.ContextMap()), "123456") || strings.Contains(entry.Message, "123456") {
			t.Fatal("OTP logged")
		}
	}
	if err := d.DispatchPhoneVerificationOTP(ctx, "other-user", "+447700900123", "234567"); err == nil || calls.Load() != 1 {
		t.Fatal("disallowed destination dispatched")
	}
}

func TestChannelPhoneOTPDispatchBudgets(t *testing.T) {
	t.Run("user", func(t *testing.T) {
		d, calls := phoneFixture(t)
		ctx := WithPhoneOTPClientIP(context.Background(), "127.0.0.1")
		for i := 0; i < 4; i++ {
			err := d.DispatchPhoneVerificationOTP(ctx, "budget-user", fmt.Sprintf("+9198765432%02d", i), fmt.Sprintf("%06d", 123450+i))
			if (i < 3) != (err == nil) {
				t.Fatalf("user budget attempt %d err=%v", i, err)
			}
		}
		if calls.Load() != 3 {
			t.Fatal("user budget bypassed")
		}
	})
	t.Run("destination", func(t *testing.T) {
		d, calls := phoneFixture(t)
		ctx := WithPhoneOTPClientIP(context.Background(), "127.0.0.1")
		for i := 0; i < 4; i++ {
			err := d.DispatchPhoneVerificationOTP(ctx, fmt.Sprintf("user-%d", i), "+919876543210", fmt.Sprintf("%06d", 223450+i))
			if (i < 3) != (err == nil) {
				t.Fatalf("destination budget %d %v", i, err)
			}
		}
		if calls.Load() != 3 {
			t.Fatal("destination budget bypassed")
		}
	})
	t.Run("ip", func(t *testing.T) {
		d, calls := phoneFixture(t)
		ctx := WithPhoneOTPClientIP(context.Background(), "127.0.0.1")
		for i := 0; i < 11; i++ {
			err := d.DispatchPhoneVerificationOTP(ctx, fmt.Sprintf("user-%d", i), fmt.Sprintf("+9198765432%02d", i), fmt.Sprintf("%06d", 323450+i))
			if (i < 10) != (err == nil) {
				t.Fatalf("IP budget %d %v", i, err)
			}
		}
		if calls.Load() != 10 {
			t.Fatal("IP budget bypassed")
		}
	})
	t.Run("spend", func(t *testing.T) {
		d, calls := phoneFixture(t)
		d.Policy.DailyBudgetMinor = 1
		ctx := WithPhoneOTPClientIP(context.Background(), "127.0.0.1")
		if err := d.DispatchPhoneVerificationOTP(ctx, "first", "+919876543210", "123456"); err != nil {
			t.Fatal(err)
		}
		if err := d.DispatchPhoneVerificationOTP(ctx, "second", "+919876543211", "234567"); err == nil || calls.Load() != 1 {
			t.Fatal("spend budget bypassed")
		}
	})
	t.Run("redis failure", func(t *testing.T) {
		d, calls := phoneFixture(t)
		d.Redis = redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1})
		defer d.Redis.Close()
		if err := d.DispatchPhoneVerificationOTP(WithPhoneOTPClientIP(context.Background(), "127.0.0.1"), "user", "+919876543210", "123456"); err == nil || calls.Load() != 0 {
			t.Fatal("budget store outage dispatched paid OTP")
		}
	})
}

func TestChannelPhoneOTPDispatchAtomicVerification(t *testing.T) {
	d, _ := phoneFixture(t)
	ctx := context.Background()
	verifier, ok := d.Pending.(interface {
		VerifyPhoneOTPProof(context.Context, string, string, string) (*auth.PendingRegistration, error)
	})
	if !ok {
		t.Fatal("atomic phone OTP verification unavailable")
	}
	uid := fmt.Sprintf("verification-%d", time.Now().UnixNano())
	phone := "+919876543210"
	if err := d.Pending.StorePhoneOTP(ctx, uid, &auth.PendingRegistration{Email: phone, OTPCode: "123456", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := verifier.VerifyPhoneOTPProof(ctx, uid, phone, "000000"); err == nil {
				t.Error("wrong proof accepted")
			}
		}()
	}
	wg.Wait()
	if _, err := verifier.VerifyPhoneOTPProof(ctx, uid, phone, "123456"); err == nil {
		t.Fatal("concurrent guesses bypassed five-attempt maximum")
	}
	d.Pending.StorePhoneOTP(ctx, uid, &auth.PendingRegistration{Email: phone, OTPCode: "234567", CreatedAt: time.Now().UTC()})
	if _, err := verifier.VerifyPhoneOTPProof(ctx, uid, phone, "234567"); err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.VerifyPhoneOTPProof(ctx, uid, phone, "234567"); err == nil {
		t.Fatal("verification proof replayed")
	}
}
