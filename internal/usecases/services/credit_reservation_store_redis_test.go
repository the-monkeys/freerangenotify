package services

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
)

func reservationRedisFixture(t *testing.T) (*redis.Client, *redisReservationStore) {
	t.Helper()
	addr := os.Getenv("FRN_CREDIT_REDIS_ADDR")
	if os.Getenv("FRN_CREDIT_INTEGRATION") != "1" {
		t.Skip("disposable Redis fixture disabled")
	}
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Fatal("reservation fixture requires explicit localhost Redis")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client, newRedisReservationStore(client)
}

func TestRedisPendingReservationSurvivesUntilSettlement(t *testing.T) {
	client, store := reservationRedisFixture(t)
	ctx := context.Background()
	res := &billing.CreditReservation{ID: "retention-test-" + uuid.NewString(), Status: billing.CreditReservationReserved, ExpiresAt: time.Now().Add(-time.Hour)}
	t.Cleanup(func() { store.Delete(ctx, res.ID) })
	if err := store.Save(ctx, res); err != nil {
		t.Fatal(err)
	}
	if ttl := client.TTL(ctx, creditReservationKey(res.ID)).Val(); ttl != -1 {
		t.Fatalf("pending reservation can disappear with its hold still active: TTL=%v", ttl)
	}
	res.Status = billing.CreditReservationReleased
	if err := store.Save(ctx, res); err != nil {
		t.Fatal(err)
	}
	if ttl := client.TTL(ctx, creditReservationKey(res.ID)).Val(); ttl <= 0 || ttl > 24*time.Hour {
		t.Fatalf("terminal receipt retention=%v", ttl)
	}
	if _, err := client.ZScore(ctx, creditReservationExpiryZSet, res.ID).Result(); err != redis.Nil {
		t.Fatalf("settled reservation remains in reaper index: %v", err)
	}
}

func TestRedisMissingReservationIsReportedWithoutStarvingValidHolds(t *testing.T) {
	client, store := reservationRedisFixture(t)
	ctx := context.Background()
	missing := "missing-test-" + uuid.NewString()
	valid := &billing.CreditReservation{ID: "valid-test-" + uuid.NewString(), Status: billing.CreditReservationReserved, ExpiresAt: time.Now().Add(-time.Hour)}
	t.Cleanup(func() { store.Delete(ctx, missing); store.Delete(ctx, valid.ID) })
	if err := store.Save(ctx, valid); err != nil {
		t.Fatal(err)
	}
	if err := client.ZAdd(ctx, creditReservationExpiryZSet, &redis.Z{Member: missing, Score: float64(time.Now().Add(-time.Hour).Unix())}).Err(); err != nil {
		t.Fatal(err)
	}
	res, err := store.ListExpiredWithError(ctx, time.Now())
	if err == nil || !strings.Contains(err.Error(), missing) {
		t.Fatalf("missing hold record silently ignored: %v", err)
	}
	found := false
	for _, item := range res {
		if item.ID == valid.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("missing record prevented recovery of an independent valid hold")
	}
}
