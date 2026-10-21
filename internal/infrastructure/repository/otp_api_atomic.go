package repository

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/otp"
)

func (r *redisOTPAPIRepository) CompareAndSwap(ctx context.Context, before, next *otp.Request) error {
	if before == nil || next == nil || before.RequestID == "" || before.RequestID != next.RequestID || before.AppID != next.AppID || before.EnvironmentID != next.EnvironmentID || !before.ExpiresAt.Equal(next.ExpiresAt) {
		return otp.ErrConflict
	}
	key := r.key(before.RequestID)
	for attempt := 0; attempt < 8; attempt++ {
		err := r.client.Watch(ctx, func(tx *redis.Tx) error {
			raw, err := tx.Get(ctx, key).Bytes()
			if err == redis.Nil {
				return otp.ErrNotFound
			}
			if err != nil {
				return err
			}
			var current otp.Request
			if err = json.Unmarshal(raw, &current); err != nil {
				return err
			}
			if current.Revision != before.Revision || current.AppID != before.AppID || current.EnvironmentID != before.EnvironmentID || !current.ExpiresAt.Equal(before.ExpiresAt) {
				return otp.ErrConflict
			}
			ttl := time.Until(current.ExpiresAt)
			if ttl <= 0 {
				return otp.ErrExpired
			}
			updated := *next
			updated.Revision = current.Revision + 1
			var doc map[string]json.RawMessage
			if err = json.Unmarshal(raw, &doc); err != nil {
				return err
			}
			// Preserve fields written by a newer compatible reader, while removing
			// known optional fields that this transition intentionally clears.
			typ := reflect.TypeOf(updated)
			for i := 0; i < typ.NumField(); i++ {
				name := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
				if name != "" && name != "-" {
					delete(doc, name)
				}
			}
			encoded, err := json.Marshal(updated)
			if err != nil {
				return err
			}
			var fields map[string]json.RawMessage
			if err = json.Unmarshal(encoded, &fields); err != nil {
				return err
			}
			for k, v := range fields {
				doc[k] = v
			}
			encoded, err = json.Marshal(doc)
			if err != nil {
				return err
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error { pipe.Set(ctx, key, encoded, ttl); return nil })
			return err
		}, key)
		if err == redis.TxFailedErr {
			continue
		}
		if err == nil {
			next.Revision = before.Revision + 1
		}
		return err
	}
	return otp.ErrConflict
}

var _ otp.AtomicRepository = (*redisOTPAPIRepository)(nil)
