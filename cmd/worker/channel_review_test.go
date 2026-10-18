package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
	"testing"
	"time"
)

type staleReviewRepo struct {
	*stubNotifRepo
	statusWrites int
	claimed      bool
}

func (r *staleReviewRepo) GetByID(ctx context.Context, id string) (*notification.Notification, error) {
	n, _ := r.stubNotifRepo.GetByID(ctx, id)
	b, _ := json.Marshal(n)
	copy := new(notification.Notification)
	json.Unmarshal(b, copy)
	n.Status = notification.StatusSent // another worker wins after this stale read
	return copy, nil
}
func (r *staleReviewRepo) ClaimNotificationProcessing(context.Context, *notification.Notification) (bool, error) {
	r.claimed = true
	return false, nil
}
func (r *staleReviewRepo) UpdateStatus(ctx context.Context, id string, state notification.Status) error {
	r.statusWrites++
	r.notifications[id].Status = state
	return nil
}
func TestPhase3ReviewStaleWorkerCannotChangeWinner(t *testing.T) {
	repo := &staleReviewRepo{stubNotifRepo: newStubNotifRepo()}
	repo.notifications["n"] = &notification.Notification{NotificationID: "n", AppID: "app-1", UserID: "user-1", Channel: notification.ChannelSMS, Status: notification.StatusQueued}
	p := newProcessorForTest(repo.stubNotifRepo, nil, map[string]*user.User{"user-1": makeUser()}, nil, nil, nil)
	p.notifRepo = repo
	p.processNotification(context.Background(), makeQueueItem("n"), zap.NewNop())
	if !repo.claimed || repo.statusWrites != 0 || repo.notifications["n"].Status != notification.StatusSent {
		t.Fatalf("stale worker rewrote winner: claim=%v writes=%d status=%s", repo.claimed, repo.statusWrites, repo.notifications["n"].Status)
	}
}
func TestPhase3ReviewRecurringEnvironment(t *testing.T) {
	repo := newStubNotifRepo()
	p := newProcessorForTest(repo, nil, nil, nil, nil, nil)
	n := &notification.Notification{NotificationID: "n", AppID: "app-1", EnvironmentID: "staging", Channel: notification.ChannelSMS, Recurrence: &notification.Recurrence{CronExpression: "@hourly"}, CreatedAt: time.Now()}
	p.handleRecurrence(context.Background(), n)
	if len(repo.created) != 1 || repo.created[0].EnvironmentID != "staging" {
		t.Fatal("recurrence lost its environment")
	}
}

type deadlineReviewRepo struct{ *stubNotifRepo }

func (r *deadlineReviewRepo) ClaimNotificationProcessing(ctx context.Context, n *notification.Notification) (bool, error) {
	if e := ctx.Err(); e != nil {
		return false, e
	}
	return true, nil
}
func TestPhase3ReviewExpiredMessageWithProcessingClaim(t *testing.T) {
	repo := &deadlineReviewRepo{newStubNotifRepo()}
	n := &notification.Notification{NotificationID: "expired", AppID: "app-1", Channel: notification.ChannelSMS, Status: notification.StatusQueued, Metadata: map[string]interface{}{"otp_expires_at": time.Now().Add(-time.Minute).Format(time.RFC3339Nano)}}
	repo.notifications[n.NotificationID] = n
	p := newProcessorForTest(repo.stubNotifRepo, nil, nil, nil, nil, nil)
	p.notifRepo = repo
	p.processNotification(context.Background(), makeQueueItem(n.NotificationID), zap.NewNop())
	if n.Status != notification.StatusFailed || n.Metadata["failure_code"] != "notification_expired" {
		t.Fatal("expired message stuck before processing claim")
	}
}

type reviewBalance struct {
	deliveryTestBalanceRepo
	reserves int
}

func (r *reviewBalance) ReserveCredits(ctx context.Context, tenant string, n int64) (*billing.CreditBalance, error) {
	r.reserves++
	return r.deliveryTestBalanceRepo.ReserveCredits(ctx, tenant, n)
}
func TestPhase3ReviewDuplicateCannotAdmitCompatibilityHold(t *testing.T) {
	repo := &staleReviewRepo{stubNotifRepo: newStubNotifRepo()}
	repo.notifications["n"] = &notification.Notification{NotificationID: "n", AppID: "app-1", UserID: "user-1", Channel: notification.ChannelSMS, Status: notification.StatusQueued}
	p := newProcessorForTest(repo.stubNotifRepo, nil, map[string]*user.User{"user-1": makeUser()}, nil, nil, nil)
	p.notifRepo = repo
	balance := &reviewBalance{deliveryTestBalanceRepo: deliveryTestBalanceRepo{balance: &billing.CreditBalance{ID: "sub", TenantID: "app-1", CreditsTotal: 100, CreditsRemaining: 100}}}
	p.creditService = services.NewCreditService(balance, &deliveryTestLedger{}, &deliveryTestSubscriptionRepo{sub: &license.Subscription{Plan: "standard", Metadata: map[string]interface{}{"billing_model": "credits"}}}, nil, nil, nil, nil, zap.NewNop(), true)
	p.processNotification(context.Background(), makeQueueItem("n"), zap.NewNop())
	if balance.reserves != 0 {
		t.Fatalf("losing worker admitted %d unique compatibility holds", balance.reserves)
	}
}
func TestPhase3ReviewReusesPreUpgradeReservation(t *testing.T) {
	repo := newStubNotifRepo()
	n := &notification.Notification{NotificationID: "n", AppID: "app-1", UserID: "user-1", Channel: notification.ChannelSMS, Status: notification.StatusQueued, Content: notification.Content{Body: "hello"}}
	repo.notifications["n"] = n
	p := newProcessorForTest(repo, nil, map[string]*user.User{"user-1": makeUser()}, nil, nil, nil)
	balance := &reviewBalance{deliveryTestBalanceRepo: deliveryTestBalanceRepo{balance: &billing.CreditBalance{ID: "sub", TenantID: "app-1", CreditsTotal: 100, CreditsRemaining: 100}}}
	p.creditService = services.NewCreditService(balance, &deliveryTestLedger{}, &deliveryTestSubscriptionRepo{sub: &license.Subscription{Plan: "standard", Metadata: map[string]interface{}{"billing_model": "credits"}}}, nil, nil, nil, nil, zap.NewNop(), true)
	old, err := p.creditService.ReserveForNotification(context.Background(), "app-1", "app-1", "n", "sms")
	if err != nil {
		t.Fatal(err)
	}
	n.Metadata = map[string]interface{}{"reservation_id": old.ID}
	p.processNotification(context.Background(), makeQueueItem("n"), zap.NewNop())
	if balance.reserves != 1 || n.Metadata["reservation_id"] != old.ID {
		t.Fatalf("old admission replaced: holds=%d old=%s now=%v", balance.reserves, old.ID, n.Metadata["reservation_id"])
	}
}

type reviewInbox struct {
	whatsapp.Repository
	fail bool
	rows map[string]*whatsapp.InboundMessage
}

func (r *reviewInbox) StoreMessage(ctx context.Context, m *whatsapp.InboundMessage) error {
	if r.fail {
		return fmt.Errorf("fixture store unavailable")
	}
	copy := *m
	r.rows[m.ID] = &copy
	return nil
}
func TestPhase3ReviewInboxRepairAndFailures(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		t.Run(fmt.Sprint(terminal), func(t *testing.T) {
			repo := newStubNotifRepo()
			n := &notification.Notification{NotificationID: "n", AppID: "app-1", UserID: "missing", Channel: notification.ChannelWhatsApp, Status: notification.StatusQueued, Metadata: map[string]interface{}{channelconnection.DeliveryMetadataKey: channelconnection.Snapshot{AppID: "app-1", Provider: "twilio", AccountID: "account", SenderID: "sender"}, "channel_destination": "+919876543210"}}
			if terminal {
				n.Status = notification.StatusSent
			}
			repo.notifications["n"] = n
			p := newProcessorForTest(repo, nil, nil, nil, nil, nil)
			p.config.MaxRetries = 0
			q := &accountingContextQueue{}
			p.queue = q
			inbox := &reviewInbox{fail: terminal, rows: map[string]*whatsapp.InboundMessage{}}
			p.channelInbox = inbox
			p.processNotification(context.Background(), makeQueueItem("n"), zap.NewNop())
			if terminal {
				if q.ackCalls != 0 {
					t.Error("failed inbox write acknowledged before repair")
				}
				inbox.fail = false
				p.processNotification(context.Background(), makeQueueItem("n"), zap.NewNop())
			}
			m := inbox.rows["outbound:n"]
			want := "failed"
			if terminal {
				want = "sent"
			}
			if m == nil || m.DeliveryState != want {
				t.Fatalf("history failed to recover %s: %+v", want, m)
			}
		})
	}
}

func TestPhase3ReviewMissingBoundReservationDoesNotAdmit(t *testing.T) {
	repo := newStubNotifRepo()
	n := &notification.Notification{NotificationID: "n", AppID: "app-1", UserID: "user-1", Channel: notification.ChannelSMS, Status: notification.StatusQueued, Metadata: map[string]interface{}{"reservation_id": "missing"}}
	repo.notifications["n"] = n
	p := newProcessorForTest(repo, nil, map[string]*user.User{"user-1": makeUser()}, nil, nil, nil)
	balance := &reviewBalance{deliveryTestBalanceRepo: deliveryTestBalanceRepo{balance: &billing.CreditBalance{ID: "sub", TenantID: "app-1", CreditsTotal: 100, CreditsRemaining: 100}}}
	p.creditService = services.NewCreditService(balance, &deliveryTestLedger{}, &deliveryTestSubscriptionRepo{sub: &license.Subscription{Plan: "standard", Metadata: map[string]interface{}{"billing_model": "credits"}}}, nil, nil, nil, nil, zap.NewNop(), true)
	p.processNotification(context.Background(), makeQueueItem("n"), zap.NewNop())
	if balance.reserves != 0 || n.Status != notification.StatusFailed {
		t.Fatal("missing financial admission silently replaced")
	}
}

type lostAdmissionReviewRepo struct{ *stubNotifRepo }

func (r *lostAdmissionReviewRepo) ClaimNotificationProcessing(context.Context, *notification.Notification) (bool, error) {
	return true, nil
}
func (r *lostAdmissionReviewRepo) Update(ctx context.Context, n *notification.Notification) error {
	if n.Metadata["reservation_id"] != nil {
		return notification.ErrProcessingClaimLost
	}
	return r.stubNotifRepo.Update(ctx, n)
}
func TestPhase3ReviewLosingLeaseReleasesUnusedCompatibilityHold(t *testing.T) {
	repo := &lostAdmissionReviewRepo{newStubNotifRepo()}
	n := &notification.Notification{NotificationID: "n", AppID: "app-1", UserID: "user-1", Channel: notification.ChannelSMS, Status: notification.StatusQueued}
	repo.notifications["n"] = n
	p := newProcessorForTest(repo.stubNotifRepo, nil, map[string]*user.User{"user-1": makeUser()}, nil, nil, nil)
	p.notifRepo = repo
	balance := &reviewBalance{deliveryTestBalanceRepo: deliveryTestBalanceRepo{balance: &billing.CreditBalance{ID: "sub", TenantID: "app-1", CreditsTotal: 100, CreditsRemaining: 100}}}
	p.creditService = services.NewCreditService(balance, &deliveryTestLedger{}, &deliveryTestSubscriptionRepo{sub: &license.Subscription{Plan: "standard", Metadata: map[string]interface{}{"billing_model": "credits"}}}, nil, nil, nil, nil, zap.NewNop(), true)
	p.processNotification(context.Background(), makeQueueItem("n"), zap.NewNop())
	if balance.reserves != 1 || balance.releases != 1 || balance.commits != 0 {
		t.Fatalf("unused losing hold not compensated: reserve=%d release=%d commits=%d", balance.reserves, balance.releases, balance.commits)
	}
}

func TestPhase3ReviewLosingLeaseDoesNotReleaseReusedHold(t *testing.T) {
	repo := &lostAdmissionReviewRepo{newStubNotifRepo()}
	n := &notification.Notification{NotificationID: "n", AppID: "app-1", UserID: "user-1", Channel: notification.ChannelSMS, Status: notification.StatusQueued}
	repo.notifications["n"] = n
	p := newProcessorForTest(repo.stubNotifRepo, nil, map[string]*user.User{"user-1": makeUser()}, nil, nil, nil)
	p.notifRepo = repo
	balance := &reviewBalance{deliveryTestBalanceRepo: deliveryTestBalanceRepo{balance: &billing.CreditBalance{ID: "sub", TenantID: "app-1", CreditsTotal: 100, CreditsRemaining: 100}}}
	p.creditService = services.NewCreditService(balance, &deliveryTestLedger{}, &deliveryTestSubscriptionRepo{sub: &license.Subscription{Plan: "standard", Metadata: map[string]interface{}{"billing_model": "credits"}}}, nil, nil, nil, nil, zap.NewNop(), true)
	old, e := p.creditService.ReserveForNotification(context.Background(), "app-1", "app-1", "n", "sms")
	if e != nil {
		t.Fatal(e)
	}
	n.Metadata = map[string]interface{}{"reservation_id": old.ID}
	p.processNotification(context.Background(), makeQueueItem("n"), zap.NewNop())
	if balance.reserves != 1 || balance.releases != 0 || balance.commits != 0 {
		t.Fatal("losing worker released a shared prior hold")
	}
}

func TestExotelBudgetWaitPreservesRetryAndCreditState(t *testing.T) {
	repo := newStubNotifRepo()
	q := &stubQueue{}
	n := &notification.Notification{NotificationID: "budget-wait", AppID: "app-1", Channel: notification.ChannelSMS, Status: notification.StatusProcessing, RetryCount: 2}
	repo.notifications[n.NotificationID] = n
	p := newProcessorForTest(repo, q, nil, nil, nil, nil)
	p.deferForChannelBudget(context.Background(), n, makeQueueItem(n.NotificationID), &services.DeliveryBudgetDeferredError{Reason: "sender capacity reached", RetryAfter: time.Minute})
	if n.Status != notification.StatusQueued || n.RetryCount != 2 || len(q.scheduled) != 1 || n.Metadata["reservation_id"] != nil || n.ErrorMessage != "" {
		t.Fatal("capacity wait consumed retry or created financial admission")
	}
}
