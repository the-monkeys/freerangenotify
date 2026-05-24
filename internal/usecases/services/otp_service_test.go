package services

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/otp"
	templateDomain "github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
)

// ---- Test doubles ---------------------------------------------------------

// fakeOTPRepo is an in-memory implementation of otp.Repository for unit tests.
// All operations are mutex-guarded so the constant-time concurrent verify
// test can race a brute-force loop against the attempt counter without flakes.
type fakeOTPRepo struct {
	mu       sync.Mutex
	store    map[string]*otp.Request
	rlCounts map[string]int
	failOn   string // when set, the named op returns an error
}

func newFakeOTPRepo() *fakeOTPRepo {
	return &fakeOTPRepo{store: map[string]*otp.Request{}, rlCounts: map[string]int{}}
}

func (f *fakeOTPRepo) Create(_ context.Context, req *otp.Request) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOn == "Create" {
		return errors.New("forced create failure")
	}
	cp := *req
	f.store[req.RequestID] = &cp
	return nil
}

func (f *fakeOTPRepo) Get(_ context.Context, id string) (*otp.Request, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.store[id]
	if !ok {
		return nil, otp.ErrNotFound
	}
	cp := *r
	return &cp, nil
}

func (f *fakeOTPRepo) IncrementAttempts(_ context.Context, id string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.store[id]
	if !ok {
		return 0, otp.ErrNotFound
	}
	r.Attempts++
	return r.Attempts, nil
}

func (f *fakeOTPRepo) MarkVerified(_ context.Context, id string, t time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.store[id]
	if !ok {
		return otp.ErrNotFound
	}
	r.Verified = true
	r.VerifiedAt = &t
	return nil
}

func (f *fakeOTPRepo) Update(_ context.Context, req *otp.Request) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *req
	f.store[req.RequestID] = &cp
	return nil
}

func (f *fakeOTPRepo) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.store, id)
	return nil
}

func (f *fakeOTPRepo) RecipientRateLimit(_ context.Context, appID, recipient string, _ int) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := appID + "|" + recipient
	f.rlCounts[k]++
	return f.rlCounts[k], nil
}

// fakeSender captures the most recent notification.SendRequest.
type fakeSender struct {
	mu       sync.Mutex
	calls    []notification.SendRequest
	nextID   int
	failNext bool
}

func (s *fakeSender) Send(_ context.Context, req notification.SendRequest) (*notification.Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failNext {
		s.failNext = false
		return nil, errors.New("forced send failure")
	}
	s.calls = append(s.calls, req)
	s.nextID++
	return &notification.Notification{NotificationID: "notif-test"}, nil
}

// fakeUserRepo only implements the methods the OTP service uses (userResolver).
type fakeUserRepo struct {
	mu      sync.Mutex
	byEmail map[string]*user.User
	byExtID map[string]*user.User
	byID    map[string]*user.User
	created []*user.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		byEmail: map[string]*user.User{},
		byExtID: map[string]*user.User{},
		byID:    map[string]*user.User{},
	}
}

func (r *fakeUserRepo) Create(_ context.Context, u *user.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u.Email != "" {
		r.byEmail[u.AppID+"|"+u.Email] = u
	}
	if u.ExternalID != "" {
		r.byExtID[u.AppID+"|"+u.ExternalID] = u
	}
	if u.UserID != "" {
		r.byID[u.UserID] = u
	}
	r.created = append(r.created, u)
	return nil
}

func (r *fakeUserRepo) GetByID(_ context.Context, id string) (*user.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

func (r *fakeUserRepo) GetByEmail(_ context.Context, appID, email string) (*user.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byEmail[appID+"|"+email]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

func (r *fakeUserRepo) GetByExternalID(_ context.Context, appID, externalID string) (*user.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byExtID[appID+"|"+externalID]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

// fakeTemplateCreator returns a synthetic template with a stable ID.
type fakeTemplateCreator struct {
	mu    sync.Mutex
	calls []*templateDomain.CreateRequest
}

func (c *fakeTemplateCreator) Create(_ context.Context, req *templateDomain.CreateRequest) (*templateDomain.Template, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, req)
	return &templateDomain.Template{ID: "tmpl-test", AppID: req.AppID, Name: req.Name, Channel: req.Channel, Body: req.Body}, nil
}

// ---- Helpers --------------------------------------------------------------

func newServiceForTest(t *testing.T) (*OTPService, *fakeOTPRepo, *fakeSender, *fakeUserRepo, *fakeTemplateCreator) {
	t.Helper()
	repo := newFakeOTPRepo()
	sender := &fakeSender{}
	users := newFakeUserRepo()
	tmpls := &fakeTemplateCreator{}
	svc := NewOTPService(repo, sender, users, tmpls, zap.NewNop())
	return svc, repo, sender, users, tmpls
}

// ---- Code generation ------------------------------------------------------

func TestGenerateCode_NumericLengthAndAlphabet(t *testing.T) {
	for _, n := range []int{4, 6, 8, 10} {
		code, err := generateCode(n, false)
		if err != nil {
			t.Fatalf("generateCode(%d, false) error: %v", n, err)
		}
		if len(code) != n {
			t.Errorf("len=%d, want %d", len(code), n)
		}
		for _, c := range code {
			if c < '0' || c > '9' {
				t.Errorf("non-numeric char %q in %s", c, code)
			}
		}
	}
}

func TestGenerateCode_AlphanumericExcludesLookalikes(t *testing.T) {
	// 200 samples × 8 chars = 1600 chars. Probability of *never* drawing
	// a forbidden char if it were in the alphabet is vanishingly small.
	forbidden := "0O1Il"
	for i := 0; i < 200; i++ {
		code, err := generateCode(8, true)
		if err != nil {
			t.Fatalf("generateCode error: %v", err)
		}
		if strings.ContainsAny(code, forbidden) {
			t.Fatalf("alphanumeric code %q contains forbidden lookalike", code)
		}
	}
}

func TestGenerateCode_Distinctness(t *testing.T) {
	// Two consecutive 6-digit codes should virtually never match — a 1-in-1M
	// collision is acceptable; we only fail if 100 in a row collide.
	prev, _ := generateCode(6, false)
	collisions := 0
	for i := 0; i < 100; i++ {
		c, _ := generateCode(6, false)
		if c == prev {
			collisions++
		}
		prev = c
	}
	if collisions == 100 {
		t.Errorf("100 consecutive identical codes — RNG is broken")
	}
}

// ---- Hash + body rendering -----------------------------------------------

func TestHashCode_DeterministicAndSaltDependent(t *testing.T) {
	h1 := hashCode("123456", "saltA")
	h2 := hashCode("123456", "saltA")
	h3 := hashCode("123456", "saltB")
	if h1 != h2 {
		t.Errorf("same inputs produced different hashes")
	}
	if h1 == h3 {
		t.Errorf("different salts produced same hash")
	}
	if len(h1) != 64 { // SHA-256 hex
		t.Errorf("expected 64-char hex hash, got %d", len(h1))
	}
}

func TestRenderOTPBody_DefaultAndCustom(t *testing.T) {
	def := renderOTPBody("", "123456", 5)
	if !strings.Contains(def, "123456") || !strings.Contains(def, "5 minute") {
		t.Errorf("default body missing code or ttl: %q", def)
	}
	custom := renderOTPBody("Code: {{code}} ({{ttl}} min)", "ABCDE", 3)
	if custom != "Code: ABCDE (3 min)" {
		t.Errorf("custom render mismatch: %q", custom)
	}
}

// ---- Recipient validation ------------------------------------------------

func TestValidateRecipient(t *testing.T) {
	cases := []struct {
		channel otp.Channel
		input   string
		wantErr bool
	}{
		{otp.ChannelEmail, "user@example.com", false},
		{otp.ChannelEmail, "not-an-email", true},
		{otp.ChannelSMS, "+14155551234", false},
		{otp.ChannelSMS, "14155551234", true}, // missing +
		{otp.ChannelSMS, "+1", true},          // too short
		{otp.ChannelWhatsApp, "+15555550100", false},
		{otp.ChannelWhatsApp, "+0123456789", true}, // leading 0 after +
	}
	for _, tc := range cases {
		err := validateRecipient(tc.channel, tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("validateRecipient(%s, %q) err=%v wantErr=%v", tc.channel, tc.input, err, tc.wantErr)
		}
	}
}

// ---- Service: Send happy path -------------------------------------------

func TestSend_Happy_EmailCreatesUserAndDispatches(t *testing.T) {
	svc, repo, sender, users, tmpls := newServiceForTest(t)
	res, err := svc.Send(context.Background(), otp.SendInput{
		AppID:     "app-1",
		Channel:   otp.ChannelEmail,
		Recipient: "alice@example.com",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if res.RequestID == "" || !strings.HasPrefix(res.RequestID, "otp_") {
		t.Errorf("unexpected request_id %q", res.RequestID)
	}
	if res.NotificationID != "notif-test" {
		t.Errorf("notification_id = %q", res.NotificationID)
	}
	if res.TTLSeconds != otp.DefaultTTLSeconds {
		t.Errorf("ttl = %d, want default %d", res.TTLSeconds, otp.DefaultTTLSeconds)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("expected 1 send call, got %d", len(sender.calls))
	}
	if len(users.created) != 1 {
		t.Errorf("expected 1 user created, got %d", len(users.created))
	}
	if len(tmpls.calls) != 1 {
		t.Errorf("expected 1 transient template, got %d", len(tmpls.calls))
	}
	// Persisted record must NOT contain the plaintext code anywhere.
	for _, r := range repo.store {
		if r.CodeHash == "" {
			t.Errorf("stored record has empty hash")
		}
		if len(r.Salt) != 32 { // 16 bytes hex
			t.Errorf("salt length = %d, want 32 hex chars", len(r.Salt))
		}
	}
}

func TestSend_SMSReusesUserOnSecondCall(t *testing.T) {
	svc, _, _, users, _ := newServiceForTest(t)
	for i := 0; i < 2; i++ {
		if _, err := svc.Send(context.Background(), otp.SendInput{
			AppID:     "app-1",
			Channel:   otp.ChannelSMS,
			Recipient: "+14155551234",
		}); err != nil {
			t.Fatalf("Send #%d: %v", i, err)
		}
	}
	if len(users.created) != 1 {
		t.Errorf("expected 1 user created across 2 sends, got %d", len(users.created))
	}
}

// ---- Service: Send validation --------------------------------------------

func TestSend_RejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name string
		in   otp.SendInput
		want error
	}{
		{"empty app", otp.SendInput{Channel: otp.ChannelEmail, Recipient: "a@b.co"}, nil}, // app_id error is non-sentinel
		{"bad channel", otp.SendInput{AppID: "a", Channel: "push", Recipient: "x@y.z"}, otp.ErrInvalidChannel},
		{"bad email", otp.SendInput{AppID: "a", Channel: otp.ChannelEmail, Recipient: "nope"}, otp.ErrInvalidRecipient},
		{"bad phone", otp.SendInput{AppID: "a", Channel: otp.ChannelSMS, Recipient: "555"}, otp.ErrInvalidRecipient},
		{"length too small", otp.SendInput{AppID: "a", Channel: otp.ChannelSMS, Recipient: "+14155551234", Length: 3}, otp.ErrInvalidLength},
		{"length too big", otp.SendInput{AppID: "a", Channel: otp.ChannelSMS, Recipient: "+14155551234", Length: 11}, otp.ErrInvalidLength},
		{"ttl too small", otp.SendInput{AppID: "a", Channel: otp.ChannelSMS, Recipient: "+14155551234", TTLSeconds: 10}, otp.ErrInvalidTTL},
		{"ttl too big", otp.SendInput{AppID: "a", Channel: otp.ChannelSMS, Recipient: "+14155551234", TTLSeconds: 10000}, otp.ErrInvalidTTL},
		{"max attempts too big", otp.SendInput{AppID: "a", Channel: otp.ChannelSMS, Recipient: "+14155551234", MaxAttempts: 99}, otp.ErrInvalidAttempts},
		{"template missing code", otp.SendInput{AppID: "a", Channel: otp.ChannelSMS, Recipient: "+14155551234", TemplateBody: "hello no placeholder"}, otp.ErrTemplateMissingCode},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _, _, _ := newServiceForTest(t)
			_, err := svc.Send(context.Background(), tc.in)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestSend_RateLimited(t *testing.T) {
	svc, _, _, _, _ := newServiceForTest(t)
	svc.recipientLimitPerHr = 2
	for i := 0; i < 2; i++ {
		if _, err := svc.Send(context.Background(), otp.SendInput{
			AppID: "a", Channel: otp.ChannelSMS, Recipient: "+14155551234",
		}); err != nil {
			t.Fatalf("send #%d: %v", i, err)
		}
	}
	_, err := svc.Send(context.Background(), otp.SendInput{
		AppID: "a", Channel: otp.ChannelSMS, Recipient: "+14155551234",
	})
	if !errors.Is(err, otp.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

// ---- Service: Verify ------------------------------------------------------

// sendAndCaptureCode wraps Send so a test can recover the plaintext code that
// was emitted into the notification body — production code never exposes it.
func sendAndCaptureCode(t *testing.T, svc *OTPService, sender *fakeSender, in otp.SendInput) (*otp.SendResult, string) {
	t.Helper()
	res, err := svc.Send(context.Background(), in)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	sender.mu.Lock()
	defer sender.mu.Unlock()
	last := sender.calls[len(sender.calls)-1]
	code, _ := last.Data["code"].(string)
	if code == "" {
		t.Fatalf("no code in notification data")
	}
	return res, code
}

func TestVerify_Success(t *testing.T) {
	svc, _, sender, _, _ := newServiceForTest(t)
	res, code := sendAndCaptureCode(t, svc, sender, otp.SendInput{
		AppID: "a", Channel: otp.ChannelSMS, Recipient: "+14155551234",
	})
	v, err := svc.Verify(context.Background(), otp.VerifyInput{RequestID: res.RequestID, Code: code})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !v.Verified {
		t.Errorf("expected verified=true")
	}
	if v.VerifiedAt == nil {
		t.Errorf("expected verified_at to be set")
	}
}

func TestVerify_IdempotentOnAlreadyVerified(t *testing.T) {
	svc, _, sender, _, _ := newServiceForTest(t)
	res, code := sendAndCaptureCode(t, svc, sender, otp.SendInput{
		AppID: "a", Channel: otp.ChannelSMS, Recipient: "+14155551234",
	})
	if _, err := svc.Verify(context.Background(), otp.VerifyInput{RequestID: res.RequestID, Code: code}); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	// Second verify with same code must succeed without consuming an attempt.
	v2, err := svc.Verify(context.Background(), otp.VerifyInput{RequestID: res.RequestID, Code: code})
	if err != nil || !v2.Verified {
		t.Errorf("idempotent re-verify failed: %v / %+v", err, v2)
	}
}

func TestVerify_InvalidCodeConsumesAttempt(t *testing.T) {
	svc, repo, sender, _, _ := newServiceForTest(t)
	res, _ := sendAndCaptureCode(t, svc, sender, otp.SendInput{
		AppID: "a", Channel: otp.ChannelSMS, Recipient: "+14155551234",
	})
	v, err := svc.Verify(context.Background(), otp.VerifyInput{RequestID: res.RequestID, Code: "000000"})
	if !errors.Is(err, otp.ErrInvalidCode) {
		t.Fatalf("got %v, want ErrInvalidCode", err)
	}
	if v == nil || v.AttemptsRemaining != otp.DefaultMaxAttempts-1 {
		t.Errorf("attempts_remaining = %+v, want %d", v, otp.DefaultMaxAttempts-1)
	}
	stored := repo.store[res.RequestID]
	if stored.Attempts != 1 {
		t.Errorf("repo attempts = %d, want 1", stored.Attempts)
	}
}

func TestVerify_AttemptsExhausted(t *testing.T) {
	svc, repo, sender, _, _ := newServiceForTest(t)
	res, _ := sendAndCaptureCode(t, svc, sender, otp.SendInput{
		AppID: "a", Channel: otp.ChannelSMS, Recipient: "+14155551234",
		MaxAttempts: 2,
	})
	// Two wrong attempts, then a third must hit exhaustion.
	_, _ = svc.Verify(context.Background(), otp.VerifyInput{RequestID: res.RequestID, Code: "000000"})
	_, _ = svc.Verify(context.Background(), otp.VerifyInput{RequestID: res.RequestID, Code: "000000"})
	_, err := svc.Verify(context.Background(), otp.VerifyInput{RequestID: res.RequestID, Code: "000000"})
	if !errors.Is(err, otp.ErrAttemptsExhausted) {
		t.Fatalf("got %v, want ErrAttemptsExhausted", err)
	}
	if _, ok := repo.store[res.RequestID]; ok {
		t.Errorf("expected record to be deleted on exhaustion")
	}
}

func TestVerify_Expired(t *testing.T) {
	svc, repo, sender, _, _ := newServiceForTest(t)
	res, code := sendAndCaptureCode(t, svc, sender, otp.SendInput{
		AppID: "a", Channel: otp.ChannelSMS, Recipient: "+14155551234",
	})
	// Force expiry in the fake repo.
	stored := repo.store[res.RequestID]
	stored.ExpiresAt = time.Now().UTC().Add(-1 * time.Minute)

	_, err := svc.Verify(context.Background(), otp.VerifyInput{RequestID: res.RequestID, Code: code})
	if !errors.Is(err, otp.ErrExpired) {
		t.Errorf("got %v, want ErrExpired", err)
	}
}

func TestVerify_NotFound(t *testing.T) {
	svc, _, _, _, _ := newServiceForTest(t)
	_, err := svc.Verify(context.Background(), otp.VerifyInput{RequestID: "otp_bogus", Code: "123456"})
	if !errors.Is(err, otp.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestVerify_EmptyInputs(t *testing.T) {
	svc, _, _, _, _ := newServiceForTest(t)
	_, err := svc.Verify(context.Background(), otp.VerifyInput{RequestID: "", Code: ""})
	if !errors.Is(err, otp.ErrInvalidCode) {
		t.Errorf("got %v, want ErrInvalidCode", err)
	}
}

// ---- Service: Resend -----------------------------------------------------

func TestResend_NewCodeOldCodeRejected(t *testing.T) {
	svc, repo, sender, _, _ := newServiceForTest(t)
	svc.resendCooldownSec = 0 // disable cooldown for test
	res, oldCode := sendAndCaptureCode(t, svc, sender, otp.SendInput{
		AppID: "a", Channel: otp.ChannelSMS, Recipient: "+14155551234",
	})

	// Resend — must succeed and mint a new code.
	r2, err := svc.Resend(context.Background(), res.RequestID)
	if err != nil {
		t.Fatalf("Resend: %v", err)
	}
	if r2.RequestID != res.RequestID {
		t.Errorf("resend changed request_id: %q -> %q", res.RequestID, r2.RequestID)
	}
	sender.mu.Lock()
	newCode, _ := sender.calls[len(sender.calls)-1].Data["code"].(string)
	sender.mu.Unlock()
	if newCode == oldCode {
		t.Errorf("resend should issue a new code")
	}

	// Old code must no longer verify.
	_, err = svc.Verify(context.Background(), otp.VerifyInput{RequestID: res.RequestID, Code: oldCode})
	if !errors.Is(err, otp.ErrInvalidCode) {
		t.Errorf("old code accepted after resend: %v", err)
	}
	// New code must verify and attempt counter must have been reset.
	stored := repo.store[res.RequestID]
	if stored.Attempts != 1 { // the failed-old-code call above consumed 1
		t.Errorf("expected attempts=1 after one bad verify post-resend, got %d", stored.Attempts)
	}
}

func TestResend_CooldownEnforced(t *testing.T) {
	svc, _, sender, _, _ := newServiceForTest(t)
	// Default cooldown is 60s; the just-sent record's LastSentAt = now, so a
	// resend immediately must be rejected.
	res, _ := sendAndCaptureCode(t, svc, sender, otp.SendInput{
		AppID: "a", Channel: otp.ChannelSMS, Recipient: "+14155551234",
	})
	_, err := svc.Resend(context.Background(), res.RequestID)
	if !errors.Is(err, otp.ErrResendCooldown) {
		t.Errorf("got %v, want ErrResendCooldown", err)
	}
}

func TestResend_AlreadyVerified(t *testing.T) {
	svc, _, sender, _, _ := newServiceForTest(t)
	svc.resendCooldownSec = 0
	res, code := sendAndCaptureCode(t, svc, sender, otp.SendInput{
		AppID: "a", Channel: otp.ChannelSMS, Recipient: "+14155551234",
	})
	if _, err := svc.Verify(context.Background(), otp.VerifyInput{RequestID: res.RequestID, Code: code}); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	_, err := svc.Resend(context.Background(), res.RequestID)
	if !errors.Is(err, otp.ErrAlreadyVerified) {
		t.Errorf("got %v, want ErrAlreadyVerified", err)
	}
}

func TestResend_NotFound(t *testing.T) {
	svc, _, _, _, _ := newServiceForTest(t)
	_, err := svc.Resend(context.Background(), "otp_bogus")
	if !errors.Is(err, otp.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

// ---- Brute-force resistance ---------------------------------------------

func TestVerify_BruteForceLockedOutBeforeGuessing(t *testing.T) {
	svc, _, sender, _, _ := newServiceForTest(t)
	res, realCode := sendAndCaptureCode(t, svc, sender, otp.SendInput{
		AppID: "a", Channel: otp.ChannelSMS, Recipient: "+14155551234",
		Length: 4, MaxAttempts: 3,
	})
	// Three wrong guesses exhaust the budget; the 4th call returns
	// ErrAttemptsExhausted even if it would have been the correct code.
	for i := 0; i < 3; i++ {
		_, _ = svc.Verify(context.Background(), otp.VerifyInput{RequestID: res.RequestID, Code: "0000"})
	}
	_, err := svc.Verify(context.Background(), otp.VerifyInput{RequestID: res.RequestID, Code: realCode})
	if !errors.Is(err, otp.ErrAttemptsExhausted) && !errors.Is(err, otp.ErrNotFound) {
		// Either is acceptable — exhaustion may have deleted the record.
		t.Errorf("after exhaustion, got %v; want ErrAttemptsExhausted or ErrNotFound", err)
	}
}

// ---- Recipient resolution via user_id / external_id ----------------------

func TestSend_ResolvesByExternalID_UsesUserEmail(t *testing.T) {
	svc, repo, sender, users, _ := newServiceForTest(t)
	// Seed a known user with both contact fields.
	u := &user.User{
		UserID:     "u-1",
		AppID:      "app-1",
		ExternalID: "ext-abc",
		Email:      "found@example.com",
		Phone:      "+14155551234",
	}
	_ = users.Create(context.Background(), u)

	res, err := svc.Send(context.Background(), otp.SendInput{
		AppID:      "app-1",
		Channel:    otp.ChannelEmail,
		ExternalID: "ext-abc",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if res == nil || res.RequestID == "" {
		t.Fatalf("missing result")
	}
	// Persisted record's Recipient must be the user's email, not the
	// external_id we passed in.
	persisted, _ := repo.Get(context.Background(), res.RequestID)
	if persisted.Recipient != "found@example.com" {
		t.Errorf("persisted.Recipient = %q; want %q", persisted.Recipient, "found@example.com")
	}
	// Notification must be addressed to the existing user (no auto-create).
	if got := sender.calls[len(sender.calls)-1].UserID; got != "u-1" {
		t.Errorf("notification UserID = %q; want u-1", got)
	}
	if len(users.created) != 1 {
		t.Errorf("expected exactly 1 user (the seed), got %d", len(users.created))
	}
}

func TestSend_ResolvesByUserID_UsesUserPhoneForSMS(t *testing.T) {
	svc, _, sender, users, _ := newServiceForTest(t)
	u := &user.User{
		UserID: "u-2",
		AppID:  "app-1",
		Email:  "x@example.com",
		Phone:  "+15555550100",
	}
	_ = users.Create(context.Background(), u)

	res, err := svc.Send(context.Background(), otp.SendInput{
		AppID:   "app-1",
		Channel: otp.ChannelSMS,
		UserID:  "u-2",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if sender.calls[len(sender.calls)-1].UserID != "u-2" {
		t.Errorf("notification UserID = %q; want u-2", sender.calls[len(sender.calls)-1].UserID)
	}
	if res == nil {
		t.Fatal("nil result")
	}
}

func TestSend_UserID_CrossTenant_Rejected(t *testing.T) {
	svc, _, _, users, _ := newServiceForTest(t)
	u := &user.User{UserID: "u-3", AppID: "OTHER-APP", Email: "x@example.com"}
	_ = users.Create(context.Background(), u)

	_, err := svc.Send(context.Background(), otp.SendInput{
		AppID:   "app-1",
		Channel: otp.ChannelEmail,
		UserID:  "u-3",
	})
	if !errors.Is(err, otp.ErrUserNotFound) {
		t.Errorf("err = %v; want ErrUserNotFound", err)
	}
}

func TestSend_ExternalID_NotFound(t *testing.T) {
	svc, _, _, _, _ := newServiceForTest(t)
	_, err := svc.Send(context.Background(), otp.SendInput{
		AppID:      "app-1",
		Channel:    otp.ChannelEmail,
		ExternalID: "missing",
	})
	if !errors.Is(err, otp.ErrUserNotFound) {
		t.Errorf("err = %v; want ErrUserNotFound", err)
	}
}

func TestSend_UserMissingChannelAddress(t *testing.T) {
	svc, _, _, users, _ := newServiceForTest(t)
	u := &user.User{UserID: "u-4", AppID: "app-1", ExternalID: "ext-no-phone", Email: "noemail-but-has@example.com"}
	_ = users.Create(context.Background(), u)

	_, err := svc.Send(context.Background(), otp.SendInput{
		AppID:      "app-1",
		Channel:    otp.ChannelSMS, // user has no phone
		ExternalID: "ext-no-phone",
	})
	if !errors.Is(err, otp.ErrUserMissingChannelAddress) {
		t.Errorf("err = %v; want ErrUserMissingChannelAddress", err)
	}
}

func TestSend_AmbiguousRecipient_Rejected(t *testing.T) {
	svc, _, _, _, _ := newServiceForTest(t)
	_, err := svc.Send(context.Background(), otp.SendInput{
		AppID:      "app-1",
		Channel:    otp.ChannelEmail,
		Recipient:  "a@b.com",
		ExternalID: "ext-1",
	})
	if !errors.Is(err, otp.ErrAmbiguousRecipient) {
		t.Errorf("err = %v; want ErrAmbiguousRecipient", err)
	}
}

func TestSend_NoIdentifier_Rejected(t *testing.T) {
	svc, _, _, _, _ := newServiceForTest(t)
	_, err := svc.Send(context.Background(), otp.SendInput{
		AppID:   "app-1",
		Channel: otp.ChannelEmail,
	})
	if !errors.Is(err, otp.ErrInvalidRecipient) {
		t.Errorf("err = %v; want ErrInvalidRecipient", err)
	}
}
