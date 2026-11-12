package services

import (
	"context"
	"encoding/json"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
)

type financialWriteIdentity struct {
	GatewayMode     bizbilling.GatewayMode `json:"gateway_mode"`
	BillingMode     bizbilling.GatewayMode `json:"billing_mode"`
	BilledToUser    string                 `json:"billed_to_user"`
	AppID           string                 `json:"app_id"`
	UserID          string                 `json:"user_id"`
	Currency        string                 `json:"currency"`
	Scope           bizbilling.Scope       `json:"scope"`
	Sequence        int64                  `json:"billing_sequence"`
	BalanceSequence int64                  `json:"last_sequence"`
	MirrorSequence  int64                  `json:"biz_balance_sequence"`
	MirrorCurrency  string                 `json:"biz_balance_currency"`
}

func financialIdentity(raw json.RawMessage) (bizbilling.Scope, int64, error) {
	var value financialWriteIdentity
	if err := json.Unmarshal(raw, &value); err != nil {
		return bizbilling.Scope{}, 0, err
	}
	if value.UserID == "" {
		value.UserID = value.BilledToUser
	}
	scope := value.Scope
	if scope.AppID == "" {
		scope = bizbilling.Scope{AppID: value.AppID, UserID: value.UserID, Currency: value.Currency}
	}
	if scope.Currency == "" {
		scope.Currency = value.MirrorCurrency
	}
	if value.GatewayMode == bizbilling.GatewayTest || value.BillingMode == bizbilling.GatewayTest {
		scope.Mode = bizbilling.GatewayTest
	}
	return scope, max(value.Sequence, value.BalanceSequence, value.MirrorSequence), nil
}

func financialWriteGuard(gate *BizLifecycleGate, operations *BizOperationService) bizbillingrepo.FinancialWriteGuard {
	if gate == nil {
		return nil
	}
	return func(ctx context.Context, index string, before, after json.RawMessage) (func(), error) {
		noop := func() {}
		raw := after
		if len(raw) == 0 {
			raw = before
		}
		scope, sequence, err := financialIdentity(raw)
		if err != nil {
			return noop, err
		}
		if scope.AppID == "" {
			return noop, bizbilling.ErrReconciliationRequired
		}
		if len(before) > 0 && len(after) > 0 {
			prior, _, err := financialIdentity(before)
			if err != nil {
				return noop, err
			}
			if prior.AppID != scope.AppID || prior.Mode != scope.Mode || (index != bizbillingrepo.ExpenseIndex && prior.UserID != scope.UserID) || (prior.Currency != "" && prior.Currency != scope.Currency) {
				return noop, bizbilling.ErrReconciliationRequired
			}
		}
		ctx, done, err := BeginBizMutation(ctx, gate, scope.AppID)
		if err != nil {
			return noop, err
		}
		finish := func() {
			if operations != nil {
				releaseBizMutation(operations.logger, done)
			} else {
				releaseBizMutation(nil, done)
			}
		}
		state, err := gate.State(ctx, scope.AppID)
		if err != nil {
			finish()
			return noop, err
		}
		if state.LifecycleVersion == 1 && sequence == 0 {
			return finish, nil
		}
		if index == bizbillingrepo.RevenueScheduleIndex && state.LifecycleVersion == 2 && state.Status == "active" {
			if err := guardRevenueSchedule(before, after); err != nil {
				finish()
				return noop, err
			}
			return finish, nil
		}
		if index == bizbillingrepo.CouponIndex && state.LifecycleVersion == 2 && state.Status == "active" {
			if err := guardCouponDefinition(before, after); err != nil {
				finish()
				return noop, err
			}
			return finish, nil
		}
		if index == bizbillingrepo.ExpenseIndex && state.LifecycleVersion == 2 && state.Status == "active" && sequence == 0 {
			if err := guardExpenseDefinition(before, after); err != nil {
				finish()
				return noop, err
			}
			return finish, nil
		}
		if index == bizbillingrepo.EstimateIndex && state.LifecycleVersion == 2 && state.Status == "active" && sequence == 0 {
			if err := guardEstimateDefinition(before, after); err != nil {
				finish()
				return noop, err
			}
			return finish, nil
		}
		migration, migrationErr := gate.migrationAllowed(ctx, scope.AppID)
		if migrationErr != nil {
			finish()
			return noop, migrationErr
		}
		claim, trusted := bizbillingrepo.FinancialProjectionClaim(ctx)
		if (!migration && (state.LifecycleVersion != 2 || state.Status != "active")) || !trusted || operations == nil || len(after) == 0 || claim.Scope != scope || claim.Sequence != sequence {
			finish()
			return noop, bizbilling.ErrReconciliationRequired
		}
		if err = operations.streams.Check(ctx, claim); err != nil {
			finish()
			return noop, err
		}
		return finish, nil
	}
}

func guardCouponDefinition(before, after json.RawMessage) error {
	var previous, next bizbilling.Coupon
	if len(before) > 0 {
		if err := json.Unmarshal(before, &previous); err != nil {
			return err
		}
	}
	if len(after) == 0 {
		if previous.CurrentUses != 0 || previous.PendingCapacity != nil {
			return bizbilling.ErrReconciliationRequired
		}
		return nil
	}
	if err := json.Unmarshal(after, &next); err != nil {
		return err
	}
	if previous.CurrentUses != next.CurrentUses || !sameCouponCapacity(previous.PendingCapacity, next.PendingCapacity) {
		return bizbilling.ErrReconciliationRequired
	}
	if next.MaxUses < 0 || next.MaxUsesPerUser < 0 || (next.MaxUses > 0 && next.MaxUses < next.CurrentUses) {
		return bizbilling.ErrInvalidMoney
	}
	return nil
}
func sameCouponCapacity(a, b *bizbilling.CouponCapacityClaim) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func guardEstimateDefinition(before, after json.RawMessage) error {
	var previous, next bizbilling.Estimate
	if len(before) > 0 {
		if err := json.Unmarshal(before, &previous); err != nil {
			return err
		}
	}
	if previous.LastSequence != 0 || previous.Status == bizbilling.EstimateStatusAccepted || previous.Status == bizbilling.EstimateStatusConverted {
		return bizbilling.ErrReconciliationRequired
	}
	if len(after) == 0 {
		if previous.Status != bizbilling.EstimateStatusDraft {
			return bizbilling.ErrInvalidMoney
		}
		return nil
	}
	if err := json.Unmarshal(after, &next); err != nil {
		return err
	}
	if next.ConvertedInvoiceID != "" || next.Status == bizbilling.EstimateStatusConverted {
		return bizbilling.ErrReconciliationRequired
	}
	return nil
}
