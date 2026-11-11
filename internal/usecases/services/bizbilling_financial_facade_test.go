package services

import (
	"context"
	"github.com/google/uuid"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

func financialFacadesFixture(t *testing.T, faults ...*lostBillingResponse) (*BizLifecycleService, *BizCreditService, *BizOperationService, *elasticsearch.Client, bizbilling.Scope) {
	t.Helper()
	ops, es, scope := financialFixture(t, faults...)
	ctx := context.Background()
	templates := &database.IndexTemplates{}
	ensureBillingIndex(t, es, "users", templates.GetUsersTemplate())
	ensureBillingIndex(t, es, "frn_biz_configs", templates.GetBizConfigsTemplate())
	users := repository.NewUserRepository(es, zap.NewNop())
	require.NoError(t, users.Create(ctx, &user.User{UserID: scope.UserID, AppID: scope.AppID, Email: "facade-test@example.invalid"}))
	state := bizbilling.DefaultLifecycleState(scope.AppID)
	state.LifecycleVersion = 2
	state.Status = "active"
	state.PrimaryCurrency = scope.Currency
	gate := NewBizLifecycleGate(&gateMemoryStore{states: map[string]bizbilling.LifecycleState{scope.AppID: state}})
	ops.SetLifecycleGate(gate)
	credit := NewBizCreditService(ops.stores, ops.config, users, nil, zap.NewNop())
	credit.SetOperationService(ops)
	credit.SetLifecycleGate(gate)
	lifecycle := &BizLifecycleService{repo: bizbillingrepo.NewLifecycleRepo(es, zap.NewNop()), stores: ops.stores, configSvc: ops.config, creditSvc: credit, users: users, logger: zap.NewNop()}
	lifecycle.SetOperationService(ops)
	lifecycle.SetLifecycleGate(gate)
	return lifecycle, credit, ops, es, scope
}

func TestBizOfflineRefundFacadePreservesPaidHistory(t *testing.T) {
	lifecycle, _, ops, es, scope := financialFacadesFixture(t)
	ctx := context.Background()
	inv := seedFinancialInvoice(t, es, scope, 1000)
	payment, err := lifecycle.RecordPayment(ctx, scope.AppID, inv.ID, &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: 1200})
	require.NoError(t, err)
	refundCtx := WithBizOperationKey(ctx, "offline-refund")
	first, err := lifecycle.RefundPayment(refundCtx, scope.AppID, payment.ID, &bizbilling.RefundPaymentRequest{Reason: "customer return"})
	require.NoError(t, err)
	replay, err := lifecycle.RefundPayment(refundCtx, scope.AppID, payment.ID, &bizbilling.RefundPaymentRequest{Reason: "customer return"})
	require.NoError(t, err)
	require.Equal(t, first, replay)
	require.Greater(t, first.LastSequence, payment.LastSequence)
	require.Equal(t, int64(1200), first.RefundedPaisa)
	require.Zero(t, first.RefundReservedPaisa)
	invoice, err := lifecycle.GetInvoice(ctx, scope.AppID, inv.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.InvoiceStatusPaid, invoice.Status)
	require.Zero(t, invoice.AmountDuePaisa)
	require.Equal(t, int64(1200), invoice.CashRefundedPaisa)
	balance, err := ops.Balance(ctx, scope)
	require.NoError(t, err)
	require.Zero(t, balance.AvailablePaisa)
	require.Zero(t, balance.ReservedPaisa)
}

func TestBizExplicitUserBalanceUsesJournal(t *testing.T) {
	_, credit, ops, _, scope := financialFacadesFixture(t)
	ctx := context.Background()
	profile := &user.User{UserID: scope.UserID, AppID: scope.AppID}
	keyCtx := WithBizOperationKey(ctx, "profile-target")
	first, err := credit.SetUserBalance(keyCtx, scope.AppID, profile, 100, "profile balance set", "original-request")
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, int64(100), first.BalancePaisa)
	require.Positive(t, first.Sequence)
	_, err = credit.AdjustBalance(ctx, scope.AppID, scope.UserID, 20)
	require.NoError(t, err)
	replay, err := credit.SetUserBalance(keyCtx, scope.AppID, profile, 100, "profile balance set", "original-request")
	require.NoError(t, err)
	require.Equal(t, first, replay)
	balance, err := ops.Balance(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, int64(120), balance.AvailablePaisa)
	_, err = credit.SetUserBalance(keyCtx, scope.AppID, profile, 100, "profile balance set", "changed-request")
	require.ErrorIs(t, err, bizbilling.ErrIdempotencyConflict)
	_, err = credit.SetUserBalance(ctx, scope.AppID, profile, 0, "profile balance set", "zero")
	require.NoError(t, err)
	balance, err = ops.Balance(ctx, scope)
	require.NoError(t, err)
	require.Zero(t, balance.AvailablePaisa)
	require.Zero(t, balance.NonRefundablePaisa)
}

func TestBizCustomerMirrorOnlyUsesPrimaryCurrency(t *testing.T) {
	_, credit, ops, _, scope := financialFacadesFixture(t)
	ctx := context.Background()
	scope.Currency = "USD"
	_, err := ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandBalanceSet, Scope: scope, BalanceSet: &bizbilling.BalanceSet{TargetPaisa: 100, Reason: "secondary credit"}})
	require.NoError(t, err)
	err = credit.VerifyUserBalanceReceipt(ctx, &user.User{UserID: scope.UserID, AppID: scope.AppID, BalancePaisa: 100, BizBalanceSequence: 1, BizBalanceCurrency: "USD"})
	require.ErrorIs(t, err, bizbilling.ErrCurrencyMismatch)
}
func TestBizCouponDefinitionsCannotBypassCapacity(t *testing.T) {
	_, _, ops, es, scope := financialFacadesFixture(t)
	ctx := context.Background()
	ensureBillingIndex(t, es, bizbillingrepo.CouponIndex, (&database.IndexTemplates{}).GetBizCouponsTemplate())
	coupon := &bizbilling.Coupon{ID: "capacity-" + scope.UserID, AppID: scope.AppID, Code: scope.UserID, Active: true, CurrentUses: 1, MaxUses: 2, DiscountType: bizbilling.DiscountTypeFixed, DiscountValue: 100}
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.Coupon](es, bizbillingrepo.CouponIndex, zap.NewNop()).Create(ctx, coupon.ID, coupon))
	service := NewBizCouponService(ops.stores, zap.NewNop())
	require.Error(t, service.Delete(ctx, scope.AppID, coupon.ID), "used coupon cannot be deleted underneath its redemption receipts")
	_, err := ops.stores.Coupons.Update(ctx, coupon.ID, func(row *bizbilling.Coupon) error { row.CurrentUses = 0; return nil })
	require.ErrorIs(t, err, bizbilling.ErrReconciliationRequired)
}

func TestBizCustomerOpeningCreditAndConditionalProfile(t *testing.T) {
	lifecycle, credit, ops, es, scope := financialFacadesFixture(t)
	ctx := context.Background()
	profiles := repository.NewUserRepository(es, zap.NewNop()).(*repository.UserRepository)
	profiles.SetBillingWriteGuard(func(ctx context.Context, appID string) (bool, func(), error) {
		_, done, err := BeginBizMutation(ctx, lifecycle.gate, appID)
		return true, func() { require.NoError(t, done()) }, err
	})
	profiles.SetBillingBalanceAuthorizer(credit.VerifyUserBalanceReceipt)
	profile := &user.User{UserID: uuid.NewString(), AppID: scope.AppID, Email: "opening@example.invalid"}
	receipt, err := credit.SetUserBalance(WithBizOperationKey(ctx, "opening"), scope.AppID, profile, 500, "customer opening balance", "profile-request")
	require.NoError(t, err)
	profile.BalancePaisa = receipt.BalancePaisa
	profile.BizBalanceSequence = receipt.Sequence
	profile.BizBalanceCurrency = receipt.Currency
	require.NoError(t, profiles.Create(ctx, profile))
	stored, err := profiles.GetByID(ctx, profile.UserID)
	require.NoError(t, err)
	require.Equal(t, int64(500), stored.BalancePaisa)
	require.Equal(t, int64(1), stored.BizBalanceSequence)
	stale := *stored
	stale.Email = "updated@example.invalid"
	stale.BalancePaisa = 999999
	stale.BizBalanceSequence = 99
	stale.BizBalanceCurrency = "USD"
	require.NoError(t, profiles.Update(ctx, &stale))
	stored, err = profiles.GetByID(ctx, profile.UserID)
	require.NoError(t, err)
	require.Equal(t, int64(500), stored.BalancePaisa)
	require.Equal(t, int64(1), stored.BizBalanceSequence)
	require.Equal(t, "INR", stored.BizBalanceCurrency)
	require.Error(t, profiles.Create(ctx, profile), "same id must never replace an existing profile")
	stored, err = profiles.GetByID(ctx, profile.UserID)
	require.NoError(t, err)
	require.Equal(t, "updated@example.invalid", stored.Email)
	actual, err := ops.Balance(ctx, bizbilling.Scope{AppID: scope.AppID, UserID: profile.UserID, Currency: scope.Currency})
	require.NoError(t, err)
	require.Equal(t, int64(500), actual.AvailablePaisa)
}

func TestBizCreditSourceRequiresOwnedCustomer(t *testing.T) {
	ops, es, scope := financialFixture(t)
	ctx := context.Background()
	templates := &database.IndexTemplates{}
	ensureBillingIndex(t, es, "users", templates.GetUsersTemplate())
	ensureBillingIndex(t, es, bizbillingrepo.RetainerIndex, templates.GetBizRetainersTemplate())
	require.NoError(t, repository.NewUserRepository(es, zap.NewNop()).Create(ctx, &user.User{UserID: scope.UserID, AppID: "another-app", Email: "other@example.invalid"}))
	_, err := ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandCreditSourceCreate, Scope: scope, CreditSourceCreate: &bizbilling.CreditSourceCreate{Type: "retainer", AmountPaisa: 100}})
	require.True(t, apperrors.IsNotFound(err), "a customer in another app cannot own this business credit: %v", err)
}
