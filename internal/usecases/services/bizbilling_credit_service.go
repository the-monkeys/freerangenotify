package services

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// BizCreditService manages credit notes, retainers, user credit balances,
// and customer account statements.
type BizCreditService struct {
	stores    *bizbillingrepo.Stores
	configSvc *BizConfigService
	users     user.Repository
	notifier  *BizNotifier
	logger    *zap.Logger
}

// NewBizCreditService creates a new BizCreditService.
func NewBizCreditService(stores *bizbillingrepo.Stores, configSvc *BizConfigService, users user.Repository, notifier *BizNotifier, logger *zap.Logger) *BizCreditService {
	return &BizCreditService{stores: stores, configSvc: configSvc, users: users, notifier: notifier, logger: logger}
}

// ─── Credit Notes ────────────────────────────────────────────────────────

// CreateCreditNote issues a credit note for a user.
func (s *BizCreditService) CreateCreditNote(ctx context.Context, appID string, req *bizbilling.CreateCreditNoteRequest) (*bizbilling.CreditNote, error) {
	if req.AmountPaisa <= 0 {
		return nil, errors.BadRequest("amount_paisa must be greater than 0")
	}

	number, err := s.configSvc.NextNumber(ctx, appID, NumberKindCreditNote)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	cn := &bizbilling.CreditNote{
		ID:               uuid.New().String(),
		AppID:            appID,
		UserID:           req.UserID,
		InvoiceID:        req.InvoiceID,
		CreditNoteNumber: number,
		AmountPaisa:      req.AmountPaisa,
		BalancePaisa:     req.AmountPaisa,
		Reason:           req.Reason,
		Status:           bizbilling.CreditNoteStatusOpen,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.stores.CreditNotes.Create(ctx, cn.ID, cn); err != nil {
		return nil, err
	}

	s.notifier.Send(ctx, appID, req.UserID, "biz_credit_note_issued", "", map[string]interface{}{
		"credit_note_number": number,
		"amount":             FormatPaisa(req.AmountPaisa, "INR"),
	})
	return cn, nil
}

// GetCreditNote fetches a credit note scoped to the app.
func (s *BizCreditService) GetCreditNote(ctx context.Context, appID, id string) (*bizbilling.CreditNote, error) {
	cn, err := s.stores.CreditNotes.Get(ctx, id)
	if err != nil || cn.AppID != appID {
		return nil, errors.NotFound("credit_note", id)
	}
	return cn, nil
}

// ListCreditNotes lists credit notes for an app.
func (s *BizCreditService) ListCreditNotes(ctx context.Context, filter bizbilling.CreditNoteFilter) ([]*bizbilling.CreditNote, int64, error) {
	body := bizbillingrepo.NewQuery(filter.AppID).
		Term("user_id", filter.UserID).
		Term("status", string(filter.Status)).
		Body("created_at", filter.Limit, filter.Offset)
	return s.stores.CreditNotes.Find(ctx, body)
}

// ConsumeCreditNote draws down up to `maxPaisa` from an open credit note and
// records the invoice it was applied to. Returns the amount consumed.
func (s *BizCreditService) ConsumeCreditNote(ctx context.Context, appID, creditNoteID, invoiceID string, maxPaisa int64) (int64, error) {
	var consumed int64
	_, err := s.stores.CreditNotes.Update(ctx, creditNoteID, func(cn *bizbilling.CreditNote) error {
		if cn.AppID != appID {
			return errors.NotFound("credit_note", creditNoteID)
		}
		if cn.Status != bizbilling.CreditNoteStatusOpen || cn.BalancePaisa <= 0 {
			return errors.BadRequest("credit note has no available balance")
		}
		consumed = cn.BalancePaisa
		if consumed > maxPaisa {
			consumed = maxPaisa
		}
		cn.BalancePaisa -= consumed
		cn.AppliedInvoices = append(cn.AppliedInvoices, invoiceID)
		if cn.BalancePaisa == 0 {
			cn.Status = bizbilling.CreditNoteStatusApplied
		}
		cn.UpdatedAt = time.Now().UTC()
		return nil
	})
	return consumed, err
}

// RefundCreditNote marks the remaining balance of a credit note as refunded.
func (s *BizCreditService) RefundCreditNote(ctx context.Context, appID, id string) (*bizbilling.CreditNote, error) {
	cn, err := s.stores.CreditNotes.Update(ctx, id, func(cn *bizbilling.CreditNote) error {
		if cn.AppID != appID {
			return errors.NotFound("credit_note", id)
		}
		if cn.Status != bizbilling.CreditNoteStatusOpen || cn.BalancePaisa <= 0 {
			return errors.BadRequest("credit note has no refundable balance")
		}
		cn.Status = bizbilling.CreditNoteStatusRefunded
		cn.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.notifier.Send(ctx, appID, cn.UserID, "biz_refund_processed", "", map[string]interface{}{
		"amount": FormatPaisa(cn.BalancePaisa, "INR"),
	})
	return cn, nil
}

// ─── Retainers ───────────────────────────────────────────────────────────

// CreateRetainer creates an advance-payment retainer.
func (s *BizCreditService) CreateRetainer(ctx context.Context, appID string, req *bizbilling.CreateRetainerRequest) (*bizbilling.Retainer, error) {
	if req.AmountPaisa <= 0 {
		return nil, errors.BadRequest("amount_paisa must be greater than 0")
	}

	now := time.Now().UTC()
	ret := &bizbilling.Retainer{
		ID:           uuid.New().String(),
		AppID:        appID,
		UserID:       req.UserID,
		AmountPaisa:  req.AmountPaisa,
		BalancePaisa: req.AmountPaisa,
		Status:       bizbilling.RetainerStatusOpen,
		Notes:        req.Notes,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.stores.Retainers.Create(ctx, ret.ID, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// GetRetainer fetches a retainer scoped to the app.
func (s *BizCreditService) GetRetainer(ctx context.Context, appID, id string) (*bizbilling.Retainer, error) {
	ret, err := s.stores.Retainers.Get(ctx, id)
	if err != nil || ret.AppID != appID {
		return nil, errors.NotFound("retainer", id)
	}
	return ret, nil
}

// ListRetainers lists retainers for an app.
func (s *BizCreditService) ListRetainers(ctx context.Context, filter bizbilling.RetainerFilter) ([]*bizbilling.Retainer, int64, error) {
	body := bizbillingrepo.NewQuery(filter.AppID).
		Term("user_id", filter.UserID).
		Term("status", string(filter.Status)).
		Body("created_at", filter.Limit, filter.Offset)
	return s.stores.Retainers.Find(ctx, body)
}

// MarkRetainerPaid transitions an open retainer to paid (funds received).
func (s *BizCreditService) MarkRetainerPaid(ctx context.Context, appID, id string) (*bizbilling.Retainer, error) {
	return s.stores.Retainers.Update(ctx, id, func(ret *bizbilling.Retainer) error {
		if ret.AppID != appID {
			return errors.NotFound("retainer", id)
		}
		if ret.Status != bizbilling.RetainerStatusOpen {
			return errors.BadRequest("only open retainers can be marked paid")
		}
		ret.Status = bizbilling.RetainerStatusPaid
		ret.UpdatedAt = time.Now().UTC()
		return nil
	})
}

// ConsumeRetainer draws down up to maxPaisa from a paid retainer.
func (s *BizCreditService) ConsumeRetainer(ctx context.Context, appID, retainerID, invoiceID string, maxPaisa int64) (int64, error) {
	var consumed int64
	_, err := s.stores.Retainers.Update(ctx, retainerID, func(ret *bizbilling.Retainer) error {
		if ret.AppID != appID {
			return errors.NotFound("retainer", retainerID)
		}
		if ret.Status != bizbilling.RetainerStatusPaid || ret.BalancePaisa <= 0 {
			return errors.BadRequest("retainer has no available balance")
		}
		consumed = ret.BalancePaisa
		if consumed > maxPaisa {
			consumed = maxPaisa
		}
		ret.BalancePaisa -= consumed
		ret.AppliedInvoices = append(ret.AppliedInvoices, invoiceID)
		if ret.BalancePaisa == 0 {
			ret.Status = bizbilling.RetainerStatusApplied
		}
		ret.UpdatedAt = time.Now().UTC()
		return nil
	})
	return consumed, err
}

// ─── User Credit Balance ─────────────────────────────────────────────────

// AdjustBalance adds (or subtracts) credit on the user's stored balance.
func (s *BizCreditService) AdjustBalance(ctx context.Context, appID, userID string, deltaPaisa int64) (int64, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return 0, errors.NotFound("user", userID)
	}
	if u.AppID != appID {
		return 0, errors.NotFound("user", userID)
	}

	u.BalancePaisa += deltaPaisa
	if u.BalancePaisa < 0 {
		return 0, errors.BadRequest("balance cannot go negative")
	}
	if err := s.users.Update(ctx, u); err != nil {
		return 0, err
	}
	return u.BalancePaisa, nil
}

// ConsumeBalance draws down up to maxPaisa from the user's credit balance.
func (s *BizCreditService) ConsumeBalance(ctx context.Context, appID, userID string, maxPaisa int64) (int64, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil || u.AppID != appID {
		return 0, errors.NotFound("user", userID)
	}
	if u.BalancePaisa <= 0 || maxPaisa <= 0 {
		return 0, nil
	}

	consumed := u.BalancePaisa
	if consumed > maxPaisa {
		consumed = maxPaisa
	}
	u.BalancePaisa -= consumed
	if err := s.users.Update(ctx, u); err != nil {
		return 0, err
	}
	return consumed, nil
}

// ─── Customer Statement ──────────────────────────────────────────────────

// Statement builds an account statement of all billing transactions for a user.
func (s *BizCreditService) Statement(ctx context.Context, appID, userID string) (*bizbilling.Statement, error) {
	stmt := &bizbilling.Statement{UserID: userID, GeneratedAt: time.Now().UTC()}

	invoices, _, err := s.stores.Invoices.Find(ctx,
		bizbillingrepo.NewQuery(appID).Term("user_id", userID).Body("created_at", 100, 0))
	if err != nil {
		return nil, err
	}
	for _, inv := range invoices {
		if inv.Status == bizbilling.InvoiceStatusDraft || inv.Status == bizbilling.InvoiceStatusVoid {
			continue
		}
		stmt.Entries = append(stmt.Entries, bizbilling.StatementEntry{
			Date:        inv.CreatedAt,
			Type:        "invoice",
			Reference:   inv.InvoiceNumber,
			Description: string(inv.Status),
			DebitPaisa:  inv.TotalPaisa,
		})
		stmt.TotalBilled += inv.TotalPaisa
	}

	payments, _, err := s.stores.Payments.Find(ctx,
		bizbillingrepo.NewQuery(appID).Term("user_id", userID).Body("created_at", 100, 0))
	if err != nil {
		return nil, err
	}
	for _, p := range payments {
		if p.Status != bizbilling.PaymentStatusSuccess && p.Status != bizbilling.PaymentStatusRefunded {
			continue
		}
		stmt.Entries = append(stmt.Entries, bizbilling.StatementEntry{
			Date:        p.CreatedAt,
			Type:        "payment",
			Reference:   p.ID,
			Description: p.Method,
			CreditPaisa: p.AmountPaisa - p.RefundedPaisa,
		})
		stmt.TotalPaid += p.AmountPaisa - p.RefundedPaisa
	}

	creditNotes, _, err := s.stores.CreditNotes.Find(ctx,
		bizbillingrepo.NewQuery(appID).Term("user_id", userID).Body("created_at", 100, 0))
	if err != nil {
		return nil, err
	}
	for _, cn := range creditNotes {
		if cn.Status == bizbilling.CreditNoteStatusVoid {
			continue
		}
		stmt.Entries = append(stmt.Entries, bizbilling.StatementEntry{
			Date:        cn.CreatedAt,
			Type:        "credit_note",
			Reference:   cn.CreditNoteNumber,
			Description: cn.Reason,
			CreditPaisa: cn.AmountPaisa,
		})
	}

	sort.Slice(stmt.Entries, func(i, j int) bool {
		return stmt.Entries[i].Date.After(stmt.Entries[j].Date)
	})

	stmt.BalanceOwed = stmt.TotalBilled - stmt.TotalPaid
	if u, err := s.users.GetByID(ctx, userID); err == nil && u.AppID == appID {
		stmt.CreditPaisa = u.BalancePaisa
	}
	return stmt, nil
}

// FormatPaisa renders a paisa amount as a human-readable currency string.
func FormatPaisa(paisa int64, currency string) string {
	if currency == "" {
		currency = "INR"
	}
	whole := paisa / 100
	frac := paisa % 100
	if frac < 0 {
		frac = -frac
	}
	return currency + " " + formatThousands(whole) + "." + twoDigits(frac)
}

func formatThousands(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	digits := []byte{}
	for i := 0; ; i++ {
		if i > 0 && i%3 == 0 {
			digits = append([]byte{','}, digits...)
		}
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
		if n == 0 {
			break
		}
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

func twoDigits(n int64) string {
	return string([]byte{byte('0' + n/10), byte('0' + n%10)})
}
