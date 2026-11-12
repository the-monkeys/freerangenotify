package services

import (
	"bytes"
	"context"
	stderrors "errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"go.uber.org/zap"
)

func TestBizManualInvoiceEndToEnd(t *testing.T) {
	lifecycle, _, _, _, scope := financialFacadesFixture(t)
	ctx := context.Background()
	invoices := NewBizInvoiceService(lifecycle)
	request := bizbilling.CreateInvoiceRequest{UserID: scope.UserID, Currency: "INR", PaymentTerms: "net_30", LineItems: []bizbilling.InvoiceLineItem{{Description: "Consultation", Quantity: 1, UnitPrice: 1000}}}
	draft, err := lifecycle.CreateOneTimeInvoice(WithBizOperationKey(ctx, "draft"), scope.AppID, &request)
	require.NoError(t, err)
	issued, err := invoices.Issue(ctx, scope.AppID, draft.ID, "issue")
	require.NoError(t, err)
	require.Equal(t, int64(1180), issued.TotalPaisa)
	require.Equal(t, bizbilling.InvoiceStatusOpen, issued.Status)
	require.Empty(t, issued.GatewayOrderID, "manual billing must not require a gateway")
	first, err := lifecycle.RecordPayment(WithBizOperationKey(ctx, "cash"), scope.AppID, draft.ID, &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: 300, Reference: "CASH-REGISTER-1"})
	require.NoError(t, err)
	require.Equal(t, "CASH-REGISTER-1", first.Reference)
	partial, err := lifecycle.GetInvoice(ctx, scope.AppID, draft.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.InvoiceStatusPartiallyPaid, partial.Status)
	require.Equal(t, int64(880), partial.AmountDuePaisa)
	_, err = lifecycle.RecordPayment(WithBizOperationKey(ctx, "upi"), scope.AppID, draft.ID, &bizbilling.RecordPaymentRequest{Method: "upi", AmountPaisa: 880, Reference: "UPI-STAFF-CONFIRMED"})
	require.NoError(t, err)
	document, err := invoices.Document(ctx, scope.AppID, scope.UserID, draft.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1180), document.AmountPaidPaisa)
	require.Zero(t, document.AmountDuePaisa)
	require.Equal(t, bizbilling.InvoiceStatusPaid, document.Status)
	require.Len(t, document.Payments, 2)
	_, err = invoices.Document(ctx, scope.AppID, uuid.NewString(), draft.ID)
	require.Error(t, err)
	replayed, err := invoices.Issue(ctx, scope.AppID, draft.ID, "issue")
	require.NoError(t, err)
	require.Equal(t, issued.InvoiceNumber, replayed.InvoiceNumber)
}

func TestBizIssuedSnapshotSurvivesProfileEdit(t *testing.T) {
	lifecycle, _, _, _, scope := financialFacadesFixture(t)
	ctx := context.Background()
	_, err := lifecycle.configSvc.UpdateConfig(ctx, scope.AppID, bizbilling.ConfigTypeBranding, map[string]any{"company_name": "Original Clinic"})
	require.NoError(t, err)
	profile, err := lifecycle.users.GetByID(ctx, scope.UserID)
	require.NoError(t, err)
	profile.FullName = "Original Customer"
	profile.BillingAddress = &user.BillingAddress{Line1: "Original Street", State: "KA", Country: "IN"}
	require.NoError(t, lifecycle.users.Update(ctx, profile))
	invoices := NewBizInvoiceService(lifecycle)
	draft, err := lifecycle.CreateOneTimeInvoice(WithBizOperationKey(ctx, "draft"), scope.AppID, &bizbilling.CreateInvoiceRequest{UserID: scope.UserID, Currency: "INR", LineItems: []bizbilling.InvoiceLineItem{{Description: "Original Service", Quantity: 1, UnitPrice: 1000}}})
	require.NoError(t, err)
	_, err = invoices.Issue(ctx, scope.AppID, draft.ID, "issue")
	require.NoError(t, err)
	_, err = lifecycle.configSvc.UpdateConfig(ctx, scope.AppID, bizbilling.ConfigTypeBranding, map[string]any{"company_name": "Changed Clinic"})
	require.NoError(t, err)
	profile.FullName = "Changed Customer"
	profile.BillingAddress.Line1 = "Changed Street"
	require.NoError(t, lifecycle.users.Update(ctx, profile))
	document, err := invoices.Document(ctx, scope.AppID, scope.UserID, draft.ID)
	require.NoError(t, err)
	require.Equal(t, "Original Clinic", document.Issuer.Name)
	require.Equal(t, "Original Customer", document.Customer.Name)
	require.Equal(t, "Original Street", document.Customer.Address.Line1)
	require.Equal(t, "Original Service", document.LineItems[0].Description)
	require.Equal(t, int64(1180), document.TotalPaisa)
}

func TestBizPreviewEqualsIssue(t *testing.T) {
	lifecycle, _, _, _, scope := financialFacadesFixture(t)
	ctx := context.Background()
	invoices := NewBizInvoiceService(lifecycle)
	request := bizbilling.CreateInvoiceRequest{UserID: scope.UserID, Currency: "INR", LineItems: []bizbilling.InvoiceLineItem{{Description: "Service", Quantity: 2, UnitPrice: 1000, AmountPaisa: 999999}}}
	preview, err := invoices.Preview(ctx, scope.AppID, request)
	require.NoError(t, err)
	require.Equal(t, int64(2360), preview.TotalPaisa)
	require.Equal(t, int64(2000), preview.LineItems[0].AmountPaisa)
	draft, err := lifecycle.CreateOneTimeInvoice(WithBizOperationKey(ctx, "draft"), scope.AppID, &request)
	require.NoError(t, err)
	require.Equal(t, "INV-000001", draft.InvoiceNumber, "preview cannot allocate a legal invoice number")
	issued, err := invoices.Issue(ctx, scope.AppID, draft.ID, "issue")
	require.NoError(t, err)
	require.Equal(t, preview.TotalPaisa, issued.TotalPaisa)
}

func TestBizDraftAndReferencedEntityValidation(t *testing.T) {
	lifecycle, _, _, es, scope := financialFacadesFixture(t)
	ctx := context.Background()
	foreign := uuid.NewString()
	require.NoError(t, bizbillingrepo.NewStore[user.User](es, "users", zap.NewNop()).Create(ctx, foreign, &user.User{UserID: foreign, AppID: "other-app", Email: "other@example.invalid"}))
	for _, request := range []bizbilling.CreateInvoiceRequest{
		{UserID: foreign, Currency: "INR", LineItems: []bizbilling.InvoiceLineItem{{Description: "Service", Quantity: 1, UnitPrice: 1000}}},
		{UserID: scope.UserID, Currency: "INR", PaymentTerms: "net_bogus", LineItems: []bizbilling.InvoiceLineItem{{Description: "Service", Quantity: 1, UnitPrice: 1000}}},
	} {
		_, err := lifecycle.CreateOneTimeInvoice(ctx, scope.AppID, &request)
		require.Error(t, err)
	}
}

func TestBizIssuedLegacyDocumentDoesNotInventIdentity(t *testing.T) {
	lifecycle, _, _, es, scope := financialFacadesFixture(t)
	invoice := seedFinancialInvoice(t, es, scope, 1000)
	_, err := NewBizInvoiceService(lifecycle).Document(context.Background(), scope.AppID, scope.UserID, invoice.ID)
	require.ErrorIs(t, err, bizbilling.ErrLegacyReviewRequired)
}

func TestBizIssuedSnapshotRecoversAfterCommittedResponseLoss(t *testing.T) {
	fault := &lostBillingResponse{match: func(r *http.Request, body []byte) bool {
		return r.Method == http.MethodPut && strings.Contains(r.URL.Path, "frn_biz_invoices/") && bytes.Contains(body, []byte(`"issued_snapshot"`))
	}}
	lifecycle, _, ops, es, scope := financialFacadesFixture(t, fault)
	ctx := context.Background()
	draft, err := lifecycle.CreateOneTimeInvoice(WithBizOperationKey(ctx, "draft"), scope.AppID, &bizbilling.CreateInvoiceRequest{UserID: scope.UserID, Currency: scope.Currency, LineItems: []bizbilling.InvoiceLineItem{{Description: "Original Service", Quantity: 1, UnitPrice: 1000}}})
	require.NoError(t, err)
	fault.armed.Store(true)
	service := NewBizInvoiceService(lifecycle)
	_, interrupted := service.Issue(ctx, scope.AppID, draft.ID, "issue-once")
	var pending *bizbilling.OperationFailure
	require.True(t, stderrors.As(interrupted, &pending), "an interrupted projection must return its recoverable operation")
	require.False(t, fault.armed.Load(), "test must interrupt the persisted snapshot response")
	_, err = lifecycle.configSvc.UpdateConfig(ctx, scope.AppID, bizbilling.ConfigTypeBranding, map[string]any{"company_name": "Edited after issue"})
	require.NoError(t, err)
	// Simulate the crashed owner's lease expiring, as the other durable recovery
	// fixtures do. A live owner's lease must never be stolen by an immediate retry.
	expireOperationLease(t, es, scope)
	_, err = ops.Resume(ctx, scope.AppID, pending.Result.OperationID)
	require.NoError(t, err)
	issued, err := service.Issue(ctx, scope.AppID, draft.ID, "issue-once")
	require.NoError(t, err)
	require.NotNil(t, issued.IssuedSnapshot)
	require.NotEqual(t, "Edited after issue", issued.IssuedSnapshot.Issuer.Name)
	head, err := ops.streams.Get(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, int64(2), head.CommittedSequence, "draft and issue commit exactly once")
}

func TestBizIssuedDocumentTracksCreditSourcesAndTerminalStates(t *testing.T) {
	for _, action := range []string{"zero", "credit_note", "retainer", "void", "write_off"} {
		t.Run(action, func(t *testing.T) {
			lifecycle, _, ops, es, scope := financialFacadesFixture(t)
			ctx := context.Background()
			templates := &database.IndexTemplates{}
			ensureBillingIndex(t, es, bizbillingrepo.CreditNoteIndex, templates.GetBizCreditNotesTemplate())
			ensureBillingIndex(t, es, bizbillingrepo.RetainerIndex, templates.GetBizRetainersTemplate())
			ensureBillingIndex(t, es, bizbillingrepo.CreditApplicationIndex, templates.GetBizCreditApplicationsTemplate())
			price := int64(1000)
			if action == "zero" {
				price = 0
			}
			draft, err := lifecycle.CreateOneTimeInvoice(WithBizOperationKey(ctx, "draft"), scope.AppID, &bizbilling.CreateInvoiceRequest{UserID: scope.UserID, Currency: scope.Currency, LineItems: []bizbilling.InvoiceLineItem{{Description: "Service", Quantity: 1, UnitPrice: price}}})
			require.NoError(t, err)
			service := NewBizInvoiceService(lifecycle)
			_, err = service.Issue(ctx, scope.AppID, draft.ID, "issue")
			require.NoError(t, err)
			switch action {
			case "credit_note", "retainer":
				created, err := ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandCreditSourceCreate, Scope: scope, IdempotencyKey: "credit-source", CreditSourceCreate: &bizbilling.CreditSourceCreate{Type: action, AmountPaisa: 1180, Reason: "customer credit"}})
				require.NoError(t, err)
				id := operationResource(t, created, action)
				if action == "retainer" {
					_, err = ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandCreditSourceChange, Scope: scope, IdempotencyKey: "retainer-paid", CreditSourceChange: &bizbilling.CreditSourceChange{Type: action, SourceID: id, Action: "paid"}})
					require.NoError(t, err)
				}
				_, err = ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandCreditSourceChange, Scope: scope, IdempotencyKey: "apply", CreditSourceChange: &bizbilling.CreditSourceChange{Type: action, SourceID: id, Action: "apply", InvoiceID: draft.ID}})
				require.NoError(t, err)
			case "void":
				_, err = lifecycle.VoidInvoice(WithBizOperationKey(ctx, "void"), scope.AppID, draft.ID)
				require.NoError(t, err)
			case "write_off":
				_, err = lifecycle.WriteOffInvoice(WithBizOperationKey(ctx, "write-off"), scope.AppID, draft.ID)
				require.NoError(t, err)
			}
			document, err := service.Document(ctx, scope.AppID, scope.UserID, draft.ID)
			require.NoError(t, err)
			require.Zero(t, document.AmountPaidPaisa, "credit, zero-value, void and forgiveness are not cash receipts")
			require.Zero(t, document.AmountDuePaisa)
			require.Equal(t, draft.TotalPaisa, document.TotalPaisa, "settlement never rewrites the issued total")
			if action == "credit_note" || action == "retainer" {
				require.Len(t, document.Credits, 1)
				require.Equal(t, action, document.Credits[0].SourceType)
				require.Equal(t, int64(1180), document.Credits[0].AmountPaisa)
			}
		})
	}
}

func TestBizIssuedDocumentIncludesRefundReference(t *testing.T) {
	lifecycle, _, _, _, scope := financialFacadesFixture(t)
	ctx := context.Background()
	draft, err := lifecycle.CreateOneTimeInvoice(WithBizOperationKey(ctx, "draft"), scope.AppID, &bizbilling.CreateInvoiceRequest{UserID: scope.UserID, Currency: scope.Currency, LineItems: []bizbilling.InvoiceLineItem{{Description: "Service", Quantity: 1, UnitPrice: 1000}}})
	require.NoError(t, err)
	service := NewBizInvoiceService(lifecycle)
	_, err = service.Issue(ctx, scope.AppID, draft.ID, "issue")
	require.NoError(t, err)
	payment, err := lifecycle.RecordPayment(WithBizOperationKey(ctx, "payment"), scope.AppID, draft.ID, &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: 1180, Reference: "STAFF-RECEIPT"})
	require.NoError(t, err)
	_, err = lifecycle.RefundPayment(WithBizOperationKey(ctx, "offline-refund"), scope.AppID, payment.ID, &bizbilling.RefundPaymentRequest{AmountPaisa: 100, Reason: "cash returned to customer"})
	require.NoError(t, err)
	document, err := service.Document(ctx, scope.AppID, scope.UserID, draft.ID)
	require.NoError(t, err)
	require.Len(t, document.Payments, 1)
	require.Len(t, document.Payments[0].Refunds, 1)
	require.Equal(t, int64(100), document.Payments[0].Refunds[0].AmountPaisa)
	require.NotEmpty(t, document.Payments[0].Refunds[0].Reference)
	require.Equal(t, bizbilling.InvoiceStatusPaid, document.Status)
	require.Zero(t, document.AmountDuePaisa)
}
