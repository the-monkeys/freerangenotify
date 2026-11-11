package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"time"
)

func (r *redisOTPRepository) DeletePhoneOTPIfCodeMatches(ctx context.Context, uid, code string) error {
	return r.client.Eval(ctx, `local raw=redis.call('GET',KEYS[1]);if not raw then return 0 end;local p=cjson.decode(raw);if p.otp_code==ARGV[1] then redis.call('DEL',KEYS[1]) end;return 1`, []string{r.phoneKey(uid)}, code).Err()
}

// Read the timestamp in Go (including legacy timezone offsets); the Lua CAS
// binds the decision to that exact proof, preserves TTL, and consumes success.
func (r *redisOTPRepository) VerifyPhoneOTPProof(ctx context.Context, uid, phone, code string) (*auth.PendingRegistration, error) {
	raw, err := r.client.Get(ctx, r.phoneKey(uid)).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("No pending phone verification found")
	}
	if err != nil {
		return nil, fmt.Errorf("Phone verification storage unavailable")
	}
	var pending auth.PendingRegistration
	if json.Unmarshal(raw, &pending) != nil {
		return nil, fmt.Errorf("Phone verification proof invalid")
	}
	expired := 0
	if pending.CreatedAt.IsZero() || !time.Now().Before(pending.CreatedAt.Add(10*time.Minute)) {
		expired = 1
	}
	const verify = `local raw=redis.call('GET',KEYS[1]);if not raw then return 0 end;local p=cjson.decode(raw);
 if p.created_at~=ARGV[3] then return -4 end;
 if ARGV[4]=='1' then redis.call('DEL',KEYS[1]);return -3 end;
 local attempts=tonumber(p.attempts or 0);if attempts<0 or attempts>=5 then redis.call('DEL',KEYS[1]);return -2 end;
 if p.email~=ARGV[1] then return -1 end;
 if p.otp_code~=ARGV[2] then p.attempts=attempts+1;if p.attempts>=5 then redis.call('DEL',KEYS[1]);else local ttl=redis.call('PTTL',KEYS[1]);if ttl>0 then redis.call('SET',KEYS[1],cjson.encode(p),'PX',ttl) end end;return -1 end;
 redis.call('DEL',KEYS[1]);return 1;`
	state, err := r.client.Eval(ctx, verify, []string{r.phoneKey(uid)}, phone, code, pending.CreatedAt.Format(time.RFC3339Nano), expired).Int()
	if err != nil {
		return nil, fmt.Errorf("Phone verification storage unavailable")
	}
	if state != 1 {
		return nil, fmt.Errorf("Verification code invalid, expired or attempt limit reached")
	}
	return &pending, nil
}
