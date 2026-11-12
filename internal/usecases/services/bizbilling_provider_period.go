package services

import (
	"context"
	"sort"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func (s *BizRecurringService) SyncProviderSubscription(ctx context.Context, app, subscription string) (int, error) {
	sub, err := s.lifecycle.GetSubscription(ctx, app, subscription)
	if err != nil {
		return 0, err
	}
	if err = s.effects.Refresh(ctx); err != nil {
		return 0, err
	}
	query := bizbillingrepo.NewQuery("").Term("scope.app_id", app).Terms("kind", []string{"subscription_authorize"}).Term("subscription_id", sub.ID)
	count := 0
	var after string
	for {
		body := query.Body("id", 100, 0)
		if after != "" {
			body["search_after"] = []any{after}
		}
		effects, _, err := s.effects.Find(ctx, body)
		if err != nil {
			return count, err
		}
		if len(effects) == 0 {
			return count, nil
		}
		for _, effect := range effects {
			after = effect.ID
			if effect.ProviderID == "" {
				continue
			}
			processed, err := s.syncProviderEffect(ctx, sub, effect)
			count += processed
			if err != nil {
				return count, err
			}
		}
	}
}

func (s *BizRecurringService) syncProviderEffect(ctx context.Context, sub *bizbilling.BizSubscription, effect *bizbilling.GatewayEffect) (int, error) {
	if effect.Scope != sub.FinancialScope() || effect.SubscriptionID != sub.ID || effect.RecurringIntent == nil || effect.ProviderID == "" {
		return 0, bizbilling.ErrInvalidGatewayProof
	}
	gateway, err := s.gateway(ctx, effect.Gateway)
	if err != nil {
		return 0, err
	}
	reader, ok := gateway.(bizbilling.RecurringInvoiceGateway)
	if !ok {
		return 0, bizbilling.ErrGatewayPending
	}
	remote, err := gateway.FetchSubscription(ctx, effect.ProviderID)
	if err != nil {
		return 0, err
	}
	if !recurringSubscriptionMatches(remote, *effect.RecurringIntent) {
		return 0, bizbilling.ErrInvalidGatewayProof
	}
	// Invoice ownership is tied to this retained merchant revision and the
	// independently fetched subscription, including retired authorizations.
	count := 0
	for offset := 0; ; offset += 100 {
		if ctx.Err() != nil {
			return count, ctx.Err()
		}
		rows, err := reader.ListRecurringInvoices(ctx, remote.ID, offset, 100)
		if err != nil {
			return count, err
		}
		if len(rows) > 100 {
			return count, bizbilling.ErrInvalidGatewayProof
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].Start.Before(rows[j].Start) })
		for _, invoice := range rows {
			if invoice.Start.IsZero() && invoice.End.IsZero() {
				continue
			} // authorization fee, not a cycle
			if invoice.Status == "draft" || invoice.Status == "cancelled" || invoice.Status == "expired" {
				continue
			}
			created, _, err := s.mirrorProviderInvoice(ctx, sub, effect, invoice, gateway)
			if err != nil {
				return count, err
			}
			if created {
				count++
			}
		}
		if len(rows) < 100 {
			return count, nil
		}
	}
}

func (s *BizRecurringService) mirrorProviderInvoice(ctx context.Context, sub *bizbilling.BizSubscription, effect *bizbilling.GatewayEffect, invoice bizbilling.GatewayRecurringInvoice, gateway bizbilling.RecurringGateway) (bool, bizbilling.OperationResult, error) {
	var result bizbilling.OperationResult
	intent := effect.RecurringIntent
	if intent == nil || invoice.ID == "" || invoice.SubscriptionID != effect.ProviderID || invoice.Gateway != effect.Gateway || invoice.Currency != intent.Currency || invoice.AmountPaisa != intent.AmountPaisa || invoice.OrderID == "" || invoice.Start.IsZero() || !invoice.End.After(invoice.Start) || invoice.PaidPaisa < 0 || invoice.PaidPaisa > invoice.AmountPaisa || invoice.DuePaisa != invoice.AmountPaisa-invoice.PaidPaisa {
		return false, result, bizbilling.ErrInvalidGatewayProof
	}
	if invoice.Status != "issued" && invoice.Status != "partially_paid" && invoice.Status != "paid" {
		return false, result, bizbilling.ErrInvalidGatewayProof
	}
	accountProof, err := s.merchants.connections.AccountProof(ctx, effect.Gateway)
	if err != nil {
		return false, result, err
	}
	if accountProof.AppID != sub.AppID {
		return false, result, bizbilling.ErrInvalidGatewayProof
	}
	id := bizbillingrepo.PeriodID(sub.AppID, sub.ID, invoice.Start)
	_, err = s.lifecycle.stores.Periods.Get(ctx, id)
	created := apperrors.IsNotFound(err)
	if err != nil && !created {
		return false, result, err
	}
	// Mutable settlement fields cannot change the identity of a period intent.
	frozen := invoice
	frozen.PaymentID, frozen.Status, frozen.PaidPaisa, frozen.DuePaisa = "", "", 0, frozen.AmountPaisa
	proof := &bizbilling.VerifiedProviderPeriod{EffectID: effect.ID, Invoice: frozen}
	key := gatewayEffectID("period_provider", id, invoice.ID)
	result, err = s.lifecycle.operations.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandSubscriptionPeriod, Scope: effect.Scope, IdempotencyKey: key, SubscriptionPeriod: &bizbilling.SubscriptionPeriod{SubscriptionID: sub.ID, Start: invoice.Start.UTC(), End: invoice.End.UTC(), ProviderInvoiceID: invoice.ID, ProviderProof: proof}})
	if err != nil {
		return false, result, err
	}
	if invoice.PaymentID == "" {
		if invoice.PaidPaisa != 0 {
			return created, result, bizbilling.ErrInvalidGatewayProof
		}
		return created, result, nil
	}
	period, err := s.lifecycle.stores.Periods.Get(ctx, id)
	if err != nil {
		return created, result, err
	}
	payment, err := gateway.FetchPayment(ctx, invoice.PaymentID)
	if err != nil {
		return created, result, err
	}
	if payment.ID != invoice.PaymentID || payment.OrderID != invoice.OrderID || payment.Gateway != effect.Gateway || payment.Currency != invoice.Currency {
		return created, result, bizbilling.ErrInvalidGatewayProof
	}
	if !payment.Captured || payment.Status != "captured" {
		return created, result, bizbilling.ErrGatewayPending
	}
	if payment.AmountPaisa != invoice.AmountPaisa || invoice.PaidPaisa != payment.AmountPaisa || invoice.DuePaisa != 0 {
		return created, result, bizbilling.ErrInvalidGatewayProof
	}
	key = gatewayEffectID("capture", sub.AppID, effect.Gateway.ConnectionID, effect.Gateway.AccountID, string(effect.Gateway.Mode), payment.ID)
	result, err = s.lifecycle.operations.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandGatewayCapture, Scope: effect.Scope, IdempotencyKey: key, ManualPayment: &bizbilling.ManualPayment{InvoiceID: period.InvoiceID, AmountPaisa: payment.AmountPaisa, Method: "gateway", GatewayPaymentID: payment.ID, VerifiedCapture: true, GatewayBinding: &effect.Gateway}})
	return created, result, err
}

// Resolve invoice-only callbacks through the connection which verified the
// callback; never through the business's current/default payment account.
func (s *BizRecurringService) callbackSubscription(ctx context.Context, receipt *bizbilling.GatewayEventReceipt) (string, error) {
	if receipt.Data.SubscriptionID != "" {
		return receipt.Data.SubscriptionID, nil
	}
	if receipt.Data.InvoiceID == "" {
		return "", bizbilling.ErrGatewayPending
	}
	gateway, err := s.gateway(ctx, receipt.Gateway)
	if err != nil {
		return "", err
	}
	reader, ok := gateway.(bizbilling.RecurringInvoiceGateway)
	if !ok {
		return "", bizbilling.ErrGatewayPending
	}
	invoice, err := reader.FetchRecurringInvoice(ctx, receipt.Data.InvoiceID)
	if err != nil {
		return "", err
	}
	if !sameMerchantIdentity(invoice.Gateway, receipt.Gateway) {
		return "", bizbilling.ErrInvalidGatewayProof
	}
	return invoice.SubscriptionID, nil
}

func (s *BizRecurringService) reconcileProviderCallback(ctx context.Context, sub *bizbilling.BizSubscription, effect *bizbilling.GatewayEffect, gateway bizbilling.RecurringGateway, receipt *bizbilling.GatewayEventReceipt) (bizbilling.OperationResult, error) {
	reader, ok := gateway.(bizbilling.RecurringInvoiceGateway)
	if !ok {
		return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
	}
	if receipt.Data.InvoiceID != "" {
		invoice, err := reader.FetchRecurringInvoice(ctx, receipt.Data.InvoiceID)
		if err != nil {
			return bizbilling.OperationResult{}, err
		}
		if receipt.Data.PaymentID != "" && invoice.PaymentID != receipt.Data.PaymentID {
			return bizbilling.OperationResult{}, bizbilling.ErrInvalidGatewayProof
		}
		_, result, err := s.mirrorProviderInvoice(ctx, sub, effect, invoice, gateway)
		return result, err
	}
	if receipt.Data.PaymentID == "" {
		return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
	}
	for offset := 0; ; offset += 100 {
		rows, err := reader.ListRecurringInvoices(ctx, effect.ProviderID, offset, 100)
		if err != nil {
			return bizbilling.OperationResult{}, err
		}
		if len(rows) > 100 {
			return bizbilling.OperationResult{}, bizbilling.ErrInvalidGatewayProof
		}
		for _, invoice := range rows {
			if invoice.PaymentID == receipt.Data.PaymentID && !invoice.Start.IsZero() {
				_, result, err := s.mirrorProviderInvoice(ctx, sub, effect, invoice, gateway)
				return result, err
			}
		}
		if len(rows) < 100 {
			return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
		}
	}
}
