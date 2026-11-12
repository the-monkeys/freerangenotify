package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

// BizInvoiceService shares the lifecycle's authorization and journal. It does
// not introduce a second invoice store or payment implementation.
type BizInvoiceService struct{ lifecycle *BizLifecycleService }

func NewBizInvoiceService(lifecycle *BizLifecycleService) *BizInvoiceService {
	return &BizInvoiceService{lifecycle: lifecycle}
}

func (s *BizInvoiceService) Issue(ctx context.Context, appID, invoiceID, key string) (*bizbilling.BizInvoice, error) {
	return s.lifecycle.IssueInvoice(WithBizOperationKey(ctx, key), appID, invoiceID)
}

func (s *BizInvoiceService) Preview(ctx context.Context, appID string, req bizbilling.CreateInvoiceRequest) (bizbilling.InvoicePreview, error) {
	var result bizbilling.InvoicePreview
	scope, _, err := financialScopeFor(ctx, s.lifecycle.gate, appID, req.UserID, req.Currency)
	if err != nil {
		return result, err
	}
	if scope.Currency == "" {
		scope.Currency = s.lifecycle.defaultCurrency
		if scope.Currency == "" {
			scope.Currency = "INR"
		}
	}
	if err = scope.Validate(); err != nil {
		return result, apperrors.BadRequest("valid app, customer and currency are required")
	}
	if req.GatewayMode != "" && !req.GatewayMode.Valid() {
		return result, bizbilling.ErrModeMismatch
	}
	profile, err := s.lifecycle.users.GetByID(ctx, req.UserID)
	if err != nil {
		return result, err
	}
	if profile.AppID != appID {
		return result, apperrors.NotFound("customer", req.UserID)
	}
	terms, _, err := validatedInvoiceTerms(ctx, s.lifecycle.configSvc, appID, req.PaymentTerms, time.Now().UTC())
	if err != nil {
		return result, err
	}
	tax, err := s.lifecycle.configSvc.TaxConfig(ctx, appID)
	if err != nil {
		return result, err
	}
	items := append([]bizbilling.InvoiceLineItem(nil), req.LineItems...)
	state := ""
	if profile.BillingAddress != nil {
		state = profile.BillingAddress.State
	}
	overrides, _ := invoiceTaxOverrides(items)
	totals, err := bizbilling.ComputeInvoiceTotalsChecked(items, tax, state, overrides)
	if err != nil {
		return result, err
	}
	result = bizbilling.InvoicePreview{Currency: scope.Currency, LineItems: items, PaymentTerms: terms, SubtotalPaisa: totals.SubtotalPaisa, DiscountPaisa: totals.DiscountPaisa, TaxAmountPaisa: totals.TaxPaisa, TaxBreakdown: totals.Breakdown, TotalPaisa: totals.TotalPaisa}
	return result, nil
}

func validatedInvoiceTerms(ctx context.Context, config *BizConfigService, appID, terms string, from time.Time) (string, time.Time, error) {
	cfg, err := config.PaymentTermsConfig(ctx, appID)
	if err != nil {
		return "", time.Time{}, err
	}
	if terms == "" {
		terms = cfg.DefaultTerms
	}
	if terms == "" {
		terms = "due_on_receipt"
	}
	switch terms {
	case "due_on_receipt", "net_15", "net15", "net_30", "net30", "net_60", "net60":
		return terms, dueDateFor(terms, from), nil
	case "custom":
		if cfg.CustomDays >= 0 && cfg.CustomDays <= 3650 {
			return terms, from.AddDate(0, 0, cfg.CustomDays), nil
		}
	}
	return "", time.Time{}, apperrors.BadRequest("unsupported invoice payment terms")
}

func invoiceIssueSnapshot(ctx context.Context, config *BizConfigService, profile *user.User, inv *bizbilling.BizInvoice, issuedAt time.Time) (*bizbilling.InvoiceSnapshot, error) {
	if profile == nil || profile.AppID != inv.AppID || profile.UserID != inv.UserID {
		return nil, apperrors.NotFound("customer", inv.UserID)
	}
	branding, err := config.BrandingConfig(ctx, inv.AppID)
	if err != nil {
		return nil, err
	}
	tax, err := config.TaxConfig(ctx, inv.AppID)
	if err != nil {
		return nil, err
	}
	party := bizbilling.BillingParty{Name: profile.FullName, Email: profile.Email, Phone: profile.Phone, GSTIN: profile.GSTIN}
	if address := profile.BillingAddress; address != nil {
		party.Address = bizbilling.BillingAddress{Line1: address.Line1, Line2: address.Line2, City: address.City, State: address.State, Pincode: address.Pincode, Country: address.Country}
	}
	snapshot := &bizbilling.InvoiceSnapshot{Version: 1, InvoiceID: inv.ID, InvoiceNumber: inv.InvoiceNumber, Issuer: bizbilling.BillingParty{Name: branding.CompanyName, Email: branding.Email, Phone: branding.Phone, Address: branding.Address, GSTIN: tax.BusinessGSTIN}, Customer: party, LineItems: inv.LineItems, Currency: inv.Currency, SubtotalPaisa: inv.SubtotalPaisa, DiscountPaisa: inv.DiscountPaisa, TaxAmountPaisa: inv.TaxAmountPaisa, TaxBreakdown: inv.TaxBreakdown, TotalPaisa: inv.TotalPaisa, IssuedAt: issuedAt, DueDate: inv.DueDate, PaymentTerms: inv.PaymentTerms, Notes: inv.Notes, FooterText: branding.FooterText, TermsText: branding.TermsText}
	// Deep-copy slice elements and optional pointers; future draft/profile edits
	// must not share mutable memory with the frozen event.
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	var frozen bizbilling.InvoiceSnapshot
	if err = json.Unmarshal(encoded, &frozen); err != nil {
		return nil, err
	}
	return &frozen, nil
}

func (s *BizInvoiceService) Document(ctx context.Context, appID, userID, invoiceID string) (bizbilling.BillingDocument, error) {
	var result bizbilling.BillingDocument
	lifecycle := s.lifecycle
	var revision *financialReportRevision
	if lifecycle.gate != nil {
		state, err := lifecycle.gate.State(ctx, appID)
		if err != nil {
			return result, err
		}
		if state.LifecycleVersion == 2 {
			r, err := beginFinancialReport(ctx, lifecycle.gate, appID)
			if err != nil {
				return result, err
			}
			revision = &r
		}
	}
	inv, err := lifecycle.GetInvoice(ctx, appID, invoiceID)
	if err != nil {
		return result, err
	}
	if userID != "" && inv.UserID != userID {
		return result, apperrors.NotFound("invoice", invoiceID)
	}
	if inv.IssuedSnapshot == nil {
		return result, bizbilling.ErrLegacyReviewRequired
	}
	result = bizbilling.BillingDocument{InvoiceSnapshot: *inv.IssuedSnapshot, Status: inv.Status, AmountPaidPaisa: inv.AmountPaidPaisa, AmountDuePaisa: inv.AmountDuePaisa, CreditsApplied: inv.CreditsApplied, TDSPaisa: inv.TDSPaisa, LateFeePaisa: inv.LateFeePaisa, ForgivenPaisa: inv.ForgivenPaisa, CashRefundedPaisa: inv.CashRefundedPaisa, Payments: []bizbilling.DocumentPayment{}, Credits: []bizbilling.DocumentCredit{}}
	scope := inv.FinancialScope()
	query := bizbillingrepo.NewQuery(appID).Term("user_id", inv.UserID).Term("invoice_ids", inv.ID)
	err = walkDocumentRows(ctx, lifecycle, lifecycle.stores.Payments, query, scope, func(payment *bizbilling.BizPayment) error {
		ref := bizbilling.DocumentPayment{ID: payment.ID, Method: payment.Method, Reference: payment.Reference, GatewayPaymentID: payment.GatewayPaymentID, AmountPaisa: payment.AmountPaisa, TDSPaisa: payment.TDSPaisa, RefundedPaisa: payment.RefundedPaisa, Status: payment.Status, CreatedAt: payment.CreatedAt, Refunds: []bizbilling.DocumentRefund{}}
		refundQuery := documentScopeQuery(scope).Term("payment_id", payment.ID)
		if err := walkDocumentRows(ctx, lifecycle, lifecycle.stores.RefundReservations, refundQuery, scope, func(refund *bizbilling.RefundReservation) error {
			ref.Refunds = append(ref.Refunds, bizbilling.DocumentRefund{ID: refund.ID, Reference: refund.ConfirmationID, AmountPaisa: refund.AmountPaisa, State: refund.State})
			return nil
		}); err != nil {
			return err
		}
		result.Payments = append(result.Payments, ref)
		return nil
	})
	if err != nil {
		return bizbilling.BillingDocument{}, err
	}
	err = walkDocumentRows(ctx, lifecycle, lifecycle.stores.CreditApplications, documentScopeQuery(scope).Term("invoice_id", inv.ID), scope, func(credit *bizbilling.CreditApplication) error {
		result.Credits = append(result.Credits, bizbilling.DocumentCredit{ID: credit.ID, SourceType: credit.SourceType, SourceID: credit.SourceID, AmountPaisa: credit.AmountPaisa, State: credit.State})
		return nil
	})
	if err != nil {
		return bizbilling.BillingDocument{}, err
	}
	if revision != nil {
		if err = revision.verify(ctx, lifecycle.gate); err != nil {
			return bizbilling.BillingDocument{}, err
		}
	}
	return result, nil
}

func documentScopeQuery(scope bizbilling.Scope) *bizbillingrepo.Query {
	return bizbillingrepo.NewQuery("").Term("scope.app_id", scope.AppID).Term("scope.user_id", scope.UserID).Term("scope.currency", scope.Currency)
}

// Unlike real-money reports, a test invoice document must include its own test
// receipts. Every page is scoped and checked; the surrounding app revision
// prevents a concurrent writer from producing a mixed settlement document.
func walkDocumentRows[T bizbilling.FinancialSnapshot](ctx context.Context, lifecycle *BizLifecycleService, store *bizbillingrepo.Store[T], query *bizbillingrepo.Query, scope bizbilling.Scope, visit func(*T) error) error {
	after := ""
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		body := query.Body("id", 100, 0)
		body["sort"] = []map[string]interface{}{{"id": "asc"}}
		if after != "" {
			body["search_after"] = []interface{}{after}
		}
		rows, _, err := store.Find(ctx, body)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			value := *row
			if value.FinancialID() == "" || value.FinancialID() <= after {
				return bizbilling.ErrReconciliationRequired
			}
			after = value.FinancialID()
			if value.FinancialScope().Mode != scope.Mode {
				continue
			}
			if err = financialScopeMatches(value.FinancialScope(), scope); err != nil {
				return err
			}
			if err = guardFinancialRead(ctx, lifecycle.gate, lifecycle.operations, scope, value.FinancialSequence()); err != nil {
				return err
			}
			if err = visit(row); err != nil {
				return err
			}
		}
	}
}
