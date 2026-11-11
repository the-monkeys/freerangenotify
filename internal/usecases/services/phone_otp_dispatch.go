package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"net"
	"regexp"
	"strings"
	"time"
)

type phoneOTPIPKey struct{}

func WithPhoneOTPClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, phoneOTPIPKey{}, ip)
}

type PhoneOTPPolicy struct {
	AllowedPrefixes     []string
	DailyBudgetMinor    int64
	CostMinorPerSegment int64
	Currency            string
}
type PhoneOTPDispatcher struct {
	Dispatch *ChannelDispatchService
	Redis    *redis.Client
	Pending  repository.OTPRepository
	Snapshot *channelconnection.Snapshot
	Policy   PhoneOTPPolicy
}

func (d *PhoneOTPDispatcher) DispatchPhoneVerificationOTP(ctx context.Context, uid, phone, code string) error {
	unavailable := func() error {
		return apperrors.New(apperrors.ErrCodeServiceUnavailable, "Phone verification messaging is not configured")
	}
	if d.Dispatch == nil || d.Redis == nil || d.Pending == nil || d.Snapshot == nil || d.Policy.DailyBudgetMinor <= 0 || d.Policy.DailyBudgetMinor > 9007199254740991 || d.Policy.CostMinorPerSegment <= 0 || len(d.Policy.Currency) != 3 || len(d.Policy.AllowedPrefixes) == 0 {
		return unavailable()
	}
	snap := *d.Snapshot
	if snap.AppID != "__platform_auth" || snap.Channel != notification.ChannelSMS || snap.CredentialSource != "system" || snap.AccountID == "" || snap.SenderID == "" || snap.Provider != "twilio" && snap.Provider != "vonage" || snap.Provider == "twilio" && snap.Credentials["auth_token"] == "" || snap.Provider == "vonage" && snap.Credentials["api_secret"] == "" {
		return unavailable()
	}
	if uid == "" || !regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`).MatchString(phone) || !regexp.MustCompile(`^[0-9]{6}$`).MatchString(code) {
		return apperrors.BadRequest("Valid phone and verification code required")
	}
	allowed := false
	for _, prefix := range d.Policy.AllowedPrefixes {
		if regexp.MustCompile(`^\+[1-9][0-9]{0,5}$`).MatchString(prefix) && strings.HasPrefix(phone, prefix) {
			allowed = true
		}
	}
	if !allowed {
		return apperrors.BadRequest("Phone verification destination is not enabled")
	}
	ip, _ := ctx.Value(phoneOTPIPKey{}).(string)
	if net.ParseIP(ip) == nil {
		return apperrors.BadRequest("Phone verification client address required")
	}
	body := fmt.Sprintf("Your FreeRange Notify verification code is: %s. It expires in 10 minutes.", code)
	_, units, err := billing.SMSUnits(body)
	if err != nil || d.Policy.CostMinorPerSegment > d.Policy.DailyBudgetMinor/units {
		return apperrors.New(apperrors.ErrCodeRateLimitExceeded, "Phone verification spend budget exceeded")
	}
	charge := d.Policy.CostMinorPerSegment * units
	digest := func(v string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(v))) }
	scope := digest(snap.EnvironmentID)
	root := "frn:auth:phone-otp:" + scope + ":"
	id := digest(uid + "\x00" + phone + "\x00" + code)
	now := time.Now().UTC()
	day := now.Format("20060102")
	until := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC).Unix()
	keys := []string{root + "user:" + digest(uid), root + "destination:" + digest(phone), root + "ip:" + digest(ip), root + "spend:" + day, root + "receipt:" + id, root + "user-lock:" + digest(uid)}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	const admit = `
 local state=redis.call('GET',KEYS[5]);if state and string.sub(state,1,8)=='accepted' then return 2 end;if state then return -2 end;
 if redis.call('EXISTS',KEYS[6])==1 then return 0 end;
 for i=1,3 do redis.call('ZREMRANGEBYSCORE',KEYS[i],'-inf',ARGV[3]);end;
 if redis.call('ZCARD',KEYS[1])>=3 or redis.call('ZCARD',KEYS[2])>=3 or redis.call('ZCARD',KEYS[3])>=10 then return 0 end;
 local spend=tonumber(redis.call('GET',KEYS[4]) or '0');if spend>tonumber(ARGV[6])-tonumber(ARGV[5]) then return 0 end;
 for i=1,3 do redis.call('ZADD',KEYS[i],ARGV[2],ARGV[1]);redis.call('EXPIRE',KEYS[i],600);end;
 redis.call('INCRBY',KEYS[4],ARGV[5]);redis.call('EXPIREAT',KEYS[4],ARGV[7]);
 redis.call('SET',KEYS[5],'dispatching','EX',600);redis.call('SET',KEYS[6],ARGV[1],'EX',30);return 1;`
	state, err := d.Redis.Eval(ctx, admit, keys, id, now.UnixMilli(), now.Add(-10*time.Minute).UnixMilli(), 600, charge, d.Policy.DailyBudgetMinor, until).Int()
	if err != nil {
		return apperrors.New(apperrors.ErrCodeServiceUnavailable, "Phone verification budget unavailable")
	}
	if state == 2 {
		return nil
	}
	if state != 1 {
		return apperrors.New(apperrors.ErrCodeRateLimitExceeded, "Phone verification dispatch limit reached or prior outcome unresolved")
	}
	finish := func(outcome string, refund bool) error {
		repairCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer stop()
		flag := 0
		if refund {
			flag = 1
		}
		return d.Redis.Eval(repairCtx, `if redis.call('GET',KEYS[5])=='dispatching' then redis.call('SET',KEYS[5],ARGV[2],'EX',600);if ARGV[3]=='1' then local n=tonumber(redis.call('GET',KEYS[4]) or '0');if n>=tonumber(ARGV[4]) then redis.call('DECRBY',KEYS[4],ARGV[4]) end end end;if redis.call('GET',KEYS[6])==ARGV[1] then redis.call('DEL',KEYS[6]) end;return 1`, keys, id, outcome, flag, charge).Err()
	}
	// Budget admission precedes replacing the active verification proof.
	pending := &auth.PendingRegistration{Email: phone, OTPCode: code, CreatedAt: now, Attempts: 0}
	if err := d.Pending.StorePhoneOTP(ctx, uid, pending); err != nil {
		_ = finish("failed", true)
		return apperrors.New(apperrors.ErrCodeServiceUnavailable, "Phone verification proof storage unavailable")
	}
	n := &notification.Notification{NotificationID: "phone-otp:" + id, AppID: "__platform_auth", Channel: notification.ChannelSMS, Content: notification.Content{Body: body}}
	result, err := d.Dispatch.DispatchPrepared(ctx, &channelconnection.PreparedDelivery{Snapshot: &snap, Notification: n, User: &user.User{UserID: uid, Phone: phone}, PayloadKind: "text"})
	if err != nil {
		definite := providers.IsDefinitiveDeliveryFailure(err)
		if definite {
			if cleaner, ok := d.Pending.(interface {
				DeletePhoneOTPIfCodeMatches(context.Context, string, string) error
			}); ok {
				cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				_ = cleaner.DeletePhoneOTPIfCodeMatches(cleanup, uid, code)
				stop()
			}
		}
		outcome := "unknown"
		if definite {
			outcome = "failed"
		}
		_ = finish(outcome, definite)
		return apperrors.New(apperrors.ErrCodeServiceUnavailable, "Phone verification dispatch failed; request a new code if it does not arrive")
	}
	if err := finish("accepted:"+result.ProviderMessageID, false); err != nil {
		return apperrors.New(apperrors.ErrCodeServiceUnavailable, "Phone verification outcome awaiting reconciliation")
	}
	return nil
}
