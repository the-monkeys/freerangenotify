package services

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

// Phase 3 supplies durable subscription-cycle transitions. Until then an
// upgraded app must never advance dates or mint partial proration children.
func (s *BizLifecycleService) requireSubscriptionLifecycle(ctx context.Context, appID string) error {
	if s.gate == nil {
		return nil
	}
	state, err := s.gate.State(ctx, appID)
	if err != nil {
		return err
	}
	if state.LifecycleVersion != 1 {
		return apperrors.Conflict("Subscription changes are not enabled for this billing lifecycle yet")
	}
	return nil
}

func (s *BizLifecycleService) financialInvoiceScope(ctx context.Context, appID, id string) (bizbilling.Scope, bool, error) {
	inv, err := s.getOwnedInvoice(ctx, appID, id)
	if err != nil {
		return bizbilling.Scope{}, false, err
	}
	scope, upgraded, err := financialScopeFor(ctx, s.gate, appID, inv.UserID, inv.Currency)
	if err == nil && upgraded && s.operations == nil {
		err = bizbilling.ErrReconciliationRequired
	}
	return scope, upgraded, err
}

func (s *BizLifecycleService) applyFinancialCommand(ctx context.Context, command bizbilling.Command) (*bizbilling.JournalEvent, error) {
	return applyRequestedFinancialCommand(ctx, s.operations, command)
}

func applyRequestedFinancialCommand(ctx context.Context, operations *BizOperationService, command bizbilling.Command) (*bizbilling.JournalEvent, error) {
	if operations == nil {
		return nil, bizbilling.ErrReconciliationRequired
	}
	command.IdempotencyKey, command.ActorID = bizOperationKey(ctx), bizOperationActor(ctx)
	result, err := operations.Apply(ctx, command)
	if err != nil {
		return nil, err
	}
	event, err := operations.completedFinancialEvent(ctx, command.Scope.AppID, result.OperationID)
	if err != nil {
		return nil, &bizbilling.OperationFailure{Result: result, Cause: err}
	}
	return event, nil
}

func (s *BizLifecycleService) tryFinancialInvoiceChange(ctx context.Context, appID, id, action string, update *bizbilling.UpdateInvoiceRequest) (*bizbilling.BizInvoice, bool, error) {
	scope, upgraded, err := s.financialInvoiceScope(ctx, appID, id)
	if err != nil || !upgraded {
		return nil, upgraded, err
	}
	event, err := s.applyFinancialCommand(ctx, bizbilling.Command{Kind: bizbilling.CommandInvoiceChange, Scope: scope, InvoiceChange: &bizbilling.InvoiceChange{InvoiceID: id, Action: action, Update: update}})
	if err != nil {
		return nil, true, err
	}
	return event.InvoiceAfter, true, nil
}

func (s *BizLifecycleService) tryFinancialDraft(ctx context.Context, appID string, input bizbilling.InvoiceDraft) (*bizbilling.BizInvoice, bool, error) {
	scope, upgraded, err := financialScopeFor(ctx, s.gate, appID, input.Request.UserID, input.Request.Currency)
	if err != nil || !upgraded {
		return nil, upgraded, err
	}
	input.Request.Currency = scope.Currency
	event, err := s.applyFinancialCommand(ctx, bizbilling.Command{Kind: bizbilling.CommandInvoiceDraft, Scope: scope, InvoiceDraft: &input})
	if err != nil {
		return nil, true, err
	}
	return event.InvoiceAfter, true, nil
}
