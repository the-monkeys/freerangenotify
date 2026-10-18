package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
)

// DeliveryBudgetDeferredError means no provider request was made. Waiting must
// not consume a delivery retry or leave a wallet reservation behind.
type DeliveryBudgetDeferredError struct {
	Reason     string
	RetryAfter time.Duration
}

func (e *DeliveryBudgetDeferredError) Error() string { return e.Reason }

type ChannelDeliveryBudget struct {
	Redis *redis.Client
	Now   func() time.Time
}
type deliveryBudgetIdentity struct{}

func WithDeliveryBudgetIdentity(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, deliveryBudgetIdentity{}, id)
}

// A rolling account ceiling and sender messages/segments ceilings are checked
// in the same transaction as UTC daily units and estimated provider spend.
// Physical accounts/senders share limits across applications and environments.
const deliveryBudgetScript = `
local now, cutoff = tonumber(ARGV[1]), tonumber(ARGV[1])-60000
redis.call('ZREMRANGEBYSCORE',KEYS[1],'-inf',cutoff)
redis.call('ZREMRANGEBYSCORE',KEYS[2],'-inf',cutoff)
local member=ARGV[2]..':'..ARGV[3]
if redis.call('ZSCORE',KEYS[2],member) then return 0 end
local function waitfor(key)
 local first=redis.call('ZRANGE',key,0,0,'WITHSCORES')
 if #first>0 then return math.max(1,tonumber(first[2])+60001-now) end
 return 60001
end
if redis.call('ZCARD',KEYS[1])>=200 then return waitfor(KEYS[1]) end
if redis.call('ZCARD',KEYS[2])>=tonumber(ARGV[4]) then return waitfor(KEYS[2]) end
local units=0
for _,m in ipairs(redis.call('ZRANGE',KEYS[2],0,-1)) do units=units+tonumber(string.match(m,'^(%d+):')) end
if units+tonumber(ARGV[2])>tonumber(ARGV[5]) then return waitfor(KEYS[2]) end
local charged=redis.call('EXISTS',KEYS[4])==1
local dayunits=tonumber(redis.call('HGET',KEYS[3],'units') or '0')
local dayspend=tonumber(redis.call('HGET',KEYS[3],'spend') or '0')
local cost=tonumber(ARGV[2])*tonumber(ARGV[8])
if not charged and (dayunits+tonumber(ARGV[2])>tonumber(ARGV[6]) or dayspend+cost>tonumber(ARGV[7])) then return tonumber(ARGV[9]) end
redis.call('ZADD',KEYS[1],now,ARGV[3]); redis.call('PEXPIRE',KEYS[1],120000)
redis.call('ZADD',KEYS[2],now,member); redis.call('PEXPIRE',KEYS[2],120000)
if not charged then
 redis.call('HINCRBY',KEYS[3],'units',ARGV[2]); redis.call('HINCRBY',KEYS[3],'spend',cost)
 redis.call('EXPIRE',KEYS[3],172800); redis.call('SET',KEYS[4],'1','EX',172800)
end
return 0`

func (b *ChannelDeliveryBudget) AcquireDeliveryBudget(ctx context.Context, s *channelconnection.Snapshot, units int64) error {
	if s == nil || s.Provider != "exotel" {
		return nil
	}
	deferError := func(reason string, wait time.Duration) error { return &DeliveryBudgetDeferredError{reason, wait} }
	if b == nil || b.Redis == nil {
		return deferError("sender budget store unavailable", time.Minute)
	}
	if s.Exotel == nil || s.Exotel.Validate() != nil || units < 1 || units > s.Exotel.Limits.MaxSegments {
		return deferError("sender budget policy unavailable", time.Hour)
	}
	now := time.Now().UTC()
	if b.Now != nil {
		now = b.Now().UTC()
	}
	identity, _ := ctx.Value(deliveryBudgetIdentity{}).(string)
	if identity == "" {
		identity = uuid.NewString()
	}
	sender := fmt.Sprintf("%x", sha256.Sum256([]byte(s.SenderID)))
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(s.AppID+"\x00"+identity)))
	prefix := exotelAccountBudgetPrefix(s)
	day := now.Format("2006-01-02")
	keys := []string{prefix + "account", prefix + sender + ":minute", prefix + sender + ":" + day, prefix + sender + ":" + day + ":" + id}
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	l := s.Exotel.Limits
	wait, err := b.Redis.Eval(ctx, deliveryBudgetScript, keys, now.UnixMilli(), units, id, l.MessagesPerMinute, l.UnitsPerMinute, l.DailyUnits, l.DailySpendMinor, l.CostMinorPerUnit, midnight.Sub(now).Milliseconds()+1).Int64()
	if err != nil {
		return deferError("sender budget store unavailable", time.Minute)
	}
	if wait > 0 {
		return deferError("sender delivery capacity or daily budget reached", time.Duration(wait)*time.Millisecond)
	}
	return nil
}

// ExotelReadBudget is shared by onboarding and durable status recovery.
type ExotelReadBudget interface {
	AcquireProviderRead(context.Context, *channelconnection.Snapshot) error
}

func exotelAccountBudgetPrefix(s *channelconnection.Snapshot) string {
	account := fmt.Sprintf("%x", sha256.Sum256([]byte(s.Provider+"\x00"+s.Exotel.Region+"\x00"+s.AccountID)))
	return "frn:delivery:{" + account + "}:"
}
func (b *ChannelDeliveryBudget) AcquireProviderRead(ctx context.Context, s *channelconnection.Snapshot) error {
	if b == nil || b.Redis == nil || s == nil || s.Exotel == nil || s.Provider != "exotel" || s.AccountID == "" {
		return &DeliveryBudgetDeferredError{"provider request capacity unavailable", time.Minute}
	}
	now := time.Now().UTC()
	if b.Now != nil {
		now = b.Now().UTC()
	}
	const script = `local now=tonumber(ARGV[1]);redis.call('ZREMRANGEBYSCORE',KEYS[1],'-inf',now-60000);if redis.call('ZCARD',KEYS[1])>=200 then local first=redis.call('ZRANGE',KEYS[1],0,0,'WITHSCORES');return math.max(1,tonumber(first[2])+60001-now) end;redis.call('ZADD',KEYS[1],now,ARGV[2]);redis.call('PEXPIRE',KEYS[1],120000);return 0`
	wait, e := b.Redis.Eval(ctx, script, []string{exotelAccountBudgetPrefix(s) + "account"}, now.UnixMilli(), "read:"+uuid.NewString()).Int64()
	if e != nil {
		return &DeliveryBudgetDeferredError{"provider request capacity unavailable", time.Minute}
	}
	if wait > 0 {
		return &DeliveryBudgetDeferredError{"provider account request capacity reached", time.Duration(wait) * time.Millisecond}
	}
	return nil
}
func acquireExotelRead(ctx context.Context, b ExotelReadBudget, s *channelconnection.Snapshot) error {
	if b == nil {
		return &DeliveryBudgetDeferredError{"provider request capacity unavailable", time.Minute}
	}
	return b.AcquireProviderRead(ctx, s)
}
