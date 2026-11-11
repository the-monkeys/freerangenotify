package bizbilling

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type CommandKind string

const (
	CommandOpeningBalance    CommandKind = "opening_balance"
	CommandBalanceAdjustment CommandKind = "balance_adjustment"
)

// Command is a closed set of financial intents. Persist typed inputs; never
// store a generic patch or executable workflow received from an API client.
type Command struct {
	ExpenseConvert     *ExpenseConvert           `json:"expense_convert,omitempty"`
	InvoiceReminder    *InvoiceReminder          `json:"invoice_reminder,omitempty"`
	BalanceConsume     *BalanceConsume           `json:"balance_consume,omitempty"`
	EstimateConvert    *EstimateConvert          `json:"estimate_convert,omitempty"`
	BalanceSet         *BalanceSet               `json:"balance_set,omitempty"`
	OfflineRefund      *RefundReservationRequest `json:"offline_refund,omitempty"`
	RefundRelease      *RefundRelease            `json:"refund_release,omitempty"`
	CouponApply        *CouponApply              `json:"coupon_apply,omitempty"`
	CreditSourceCreate *CreditSourceCreate       `json:"credit_source_create,omitempty"`
	CreditSourceChange *CreditSourceChange       `json:"credit_source_change,omitempty"`
	CreditReversal     *CreditReversal           `json:"credit_reversal,omitempty"`
	InvoiceDraft       *InvoiceDraft             `json:"invoice_draft,omitempty"`
	InvoiceChange      *InvoiceChange            `json:"invoice_change,omitempty"`
	Kind               CommandKind               `json:"kind"`
	Scope              Scope                     `json:"scope"`
	IdempotencyKey     string                    `json:"idempotency_key,omitempty"`
	ActorID            string                    `json:"actor_id,omitempty"`
	OpeningBalance     *OpeningBalance           `json:"opening_balance,omitempty"`
	BalanceAdjustment  *BalanceAdjustment        `json:"balance_adjustment,omitempty"`
	ManualPayment      *ManualPayment            `json:"manual_payment,omitempty"`
	BalanceApplication *BalanceApplication       `json:"balance_application,omitempty"`
	RefundReservation  *RefundReservationRequest `json:"refund_reservation,omitempty"`
	RefundSettlement   *RefundSettlement         `json:"refund_settlement,omitempty"`
}

var currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)

func (s Scope) Validate() error {
	if strings.TrimSpace(s.AppID) == "" || strings.TrimSpace(s.UserID) == "" || len(s.AppID) > 200 || len(s.UserID) > 200 || !currencyCode.MatchString(s.Currency) {
		return fmt.Errorf("invalid billing scope")
	}
	return nil
}
func (c Command) Validate() error {
	if err := c.Scope.Validate(); err != nil {
		return err
	}
	if len(c.IdempotencyKey) > 200 || len(c.ActorID) > 200 {
		return ErrInvalidMoney
	}
	payloads := 0
	for _, present := range []bool{c.OpeningBalance != nil, c.BalanceAdjustment != nil, c.ManualPayment != nil, c.BalanceApplication != nil, c.RefundReservation != nil, c.RefundSettlement != nil, c.InvoiceDraft != nil, c.InvoiceChange != nil, c.CreditSourceCreate != nil, c.CreditSourceChange != nil, c.CreditReversal != nil, c.CouponApply != nil, c.RefundRelease != nil, c.OfflineRefund != nil, c.BalanceSet != nil, c.EstimateConvert != nil, c.BalanceConsume != nil, c.ExpenseConvert != nil, c.InvoiceReminder != nil} {
		if present {
			payloads++
		}
	}
	if payloads != 1 {
		return ErrInvalidMoney
	}
	switch c.Kind {
	case CommandExpenseConvert:
		if c.ExpenseConvert == nil || len(c.ExpenseConvert.ExpenseIDs) == 0 {
			return ErrInvalidMoney
		}
	case CommandInvoiceReminder:
		if c.InvoiceReminder == nil || c.InvoiceReminder.InvoiceID == "" || c.InvoiceReminder.DaysOverdue < 0 || len(c.InvoiceReminder.Steps) == 0 {
			return ErrInvalidMoney
		}

	case CommandBalanceConsume:
		if c.BalanceConsume == nil || c.BalanceConsume.MaxPaisa < 0 {
			return ErrInvalidMoney
		}
	case CommandEstimateConvert:
		if c.EstimateConvert == nil || c.EstimateConvert.EstimateID == "" {
			return ErrInvalidMoney
		}
	case CommandBalanceSet:
		if c.BalanceSet == nil || c.BalanceSet.TargetPaisa < 0 || strings.TrimSpace(c.BalanceSet.Reason) == "" || len(c.BalanceSet.RequestFingerprint) > 128 {
			return ErrInvalidMoney
		}
	case CommandOfflineRefund:
		if c.OfflineRefund == nil || c.OfflineRefund.PaymentID == "" || c.OfflineRefund.AmountPaisa < 0 || strings.TrimSpace(c.OfflineRefund.Reason) == "" {
			return ErrInvalidMoney
		}
	case CommandRefundRelease:
		if c.RefundRelease == nil || c.RefundRelease.ReservationID == "" || strings.TrimSpace(c.RefundRelease.Reason) == "" || !c.RefundRelease.DefinitiveFailure {
			return ErrInvalidMoney
		}
	case CommandCouponApply:
		if c.CouponApply == nil || c.CouponApply.InvoiceID == "" || strings.TrimSpace(c.CouponApply.Code) == "" {
			return ErrInvalidMoney
		}
	case CommandCreditSourceCreate:
		p := c.CreditSourceCreate
		if p == nil || p.AmountPaisa <= 0 || (p.Type != "credit_note" && p.Type != "retainer") {
			return ErrInvalidMoney
		}
	case CommandCreditSourceChange:
		p := c.CreditSourceChange
		if p == nil || p.SourceID == "" || p.MaxPaisa < 0 || (p.Type != "credit_note" && p.Type != "retainer") {
			return ErrInvalidMoney
		}
		switch p.Action {
		case "apply":
			if p.InvoiceID == "" {
				return ErrInvalidMoney
			}
		case "paid":
			if p.Type != "retainer" || p.InvoiceID != "" {
				return ErrInvalidMoney
			}
		case "refund":
			if p.Type != "credit_note" || p.InvoiceID != "" {
				return ErrInvalidMoney
			}
		default:
			return ErrInvalidMoney
		}
	case CommandCreditReversal:
		if c.CreditReversal == nil || c.CreditReversal.ApplicationID == "" || strings.TrimSpace(c.CreditReversal.Reason) == "" {
			return ErrInvalidMoney
		}

	case CommandInvoiceDraft:
		if c.InvoiceDraft == nil || c.InvoiceDraft.Request.UserID != c.Scope.UserID || c.InvoiceDraft.Request.Currency != c.Scope.Currency || len(c.InvoiceDraft.Request.LineItems) == 0 {
			return ErrInvalidMoney
		}
	case CommandInvoiceChange:
		if c.InvoiceChange == nil || c.InvoiceChange.InvoiceID == "" {
			return ErrInvalidMoney
		}
		switch c.InvoiceChange.Action {
		case "amend":
			if c.InvoiceChange.Update == nil {
				return ErrInvalidMoney
			}
		case "issue", "void", "write_off", "late_fee", "overdue":
			if c.InvoiceChange.Update != nil {
				return ErrInvalidMoney
			}
		default:
			return ErrInvalidMoney
		}

	case CommandOpeningBalance:
		if c.OpeningBalance == nil || c.BalanceAdjustment != nil || c.OpeningBalance.AvailablePaisa < 0 || (c.OpeningBalance.UnresolvedLegacy && c.OpeningBalance.AvailablePaisa != 0) {
			return ErrInvalidMoney
		}
	case CommandBalanceAdjustment:
		if c.BalanceAdjustment == nil || c.OpeningBalance != nil || strings.TrimSpace(c.BalanceAdjustment.Reason) == "" || len(c.BalanceAdjustment.Reason) > 1000 {
			return ErrInvalidMoney
		}
	case CommandManualPayment:
		p := c.ManualPayment
		if p == nil || p.InvoiceID == "" || p.AmountPaisa < 0 || p.TDSPaisa < 0 || p.Method == "" {
			return ErrInvalidMoney
		}
	case CommandBalanceApplication:
		p := c.BalanceApplication
		if p == nil || p.InvoiceID == "" || p.MaxPaisa < 0 {
			return ErrInvalidMoney
		}
	case CommandRefundReservation:
		p := c.RefundReservation
		if p == nil || p.PaymentID == "" || p.AmountPaisa < 0 || strings.TrimSpace(p.Reason) == "" {
			return ErrInvalidMoney
		}
	case CommandRefundSettlement:
		p := c.RefundSettlement
		if p == nil || p.ReservationID == "" || strings.TrimSpace(p.ConfirmationID) == "" {
			return ErrInvalidMoney
		}
	default:
		return ErrInvalidMoney
	}
	return nil
}

type OperationState string

const (
	OperationAccepted  OperationState = "accepted"
	OperationPrepared  OperationState = "prepared"
	OperationCompleted OperationState = "completed"
	OperationRejected  OperationState = "rejected"
)

type ResourceRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}
type Operation struct {
	SchemaVersion int            `json:"schema_version"`
	ID            string         `json:"id"`
	AppID         string         `json:"app_id"`
	Scope         Scope          `json:"scope"`
	IntentHash    string         `json:"intent_hash"`
	Command       Command        `json:"command"`
	State         OperationState `json:"state"`
	Event         *JournalEvent  `json:"event,omitempty"`
	Resources     []ResourceRef  `json:"resources,omitempty"`
	ErrorCode     string         `json:"error_code,omitempty"`
	HeadReleased  bool           `json:"head_released"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}
type OperationResult struct {
	OperationID string         `json:"operation_id"`
	State       OperationState `json:"state"`
	Resources   []ResourceRef  `json:"resources,omitempty"`
	ErrorCode   string         `json:"error_code,omitempty"`
}

// OperationFailure preserves a durable reference when a synchronous request is
// uncertain. Its public text never includes storage URLs or provider payloads.
type OperationFailure struct {
	Result OperationResult
	Cause  error
}

func (e *OperationFailure) Error() string { return "billing operation requires review or retry" }
func (e *OperationFailure) Unwrap() error { return e.Cause }

func (o Operation) Result() OperationResult {
	return OperationResult{OperationID: o.ID, State: o.State, Resources: o.Resources, ErrorCode: o.ErrorCode}
}

type Stream struct {
	ID                string    `json:"id"`
	Scope             Scope     `json:"scope"`
	CommittedSequence int64     `json:"committed_sequence"`
	ActiveOperationID string    `json:"active_operation_id"`
	Owner             string    `json:"owner"`
	Generation        int64     `json:"generation"`
	LeaseUntil        time.Time `json:"lease_until"`
}
type StreamClaim struct {
	Scope              Scope
	Sequence           int64
	OperationID, Owner string
	Generation         int64
}

// JournalEvent is frozen before append. Absolute typed projections and their
// sequence checkpoints make recovery independent of later configuration edits.
type JournalEvent struct {
	ExpensesAfter          []Expense            `json:"expenses_after,omitempty"`
	BalanceConsumedPaisa   int64                `json:"balance_consumed_paisa,omitempty"`
	EstimateAfter          *Estimate            `json:"estimate_after,omitempty"`
	CouponRedemptionAfter  *CouponRedemption    `json:"coupon_redemption_after,omitempty"`
	CreditNoteAfter        *CreditNote          `json:"credit_note_after,omitempty"`
	RetainerAfter          *Retainer            `json:"retainer_after,omitempty"`
	CreditApplicationAfter *CreditApplication   `json:"credit_application_after,omitempty"`
	UserBalanceAfter       *UserBalanceMirror   `json:"user_balance_after,omitempty"`
	SchemaVersion          int                  `json:"schema_version"`
	ID                     string               `json:"id"`
	AppID                  string               `json:"app_id"`
	OperationID            string               `json:"operation_id"`
	Scope                  Scope                `json:"scope"`
	Sequence               int64                `json:"sequence"`
	Kind                   CommandKind          `json:"kind"`
	BalanceAfter           *CustomerBalance     `json:"balance_after,omitempty"`
	InvoiceAfter           *BizInvoice          `json:"invoice_after,omitempty"`
	PaymentAfter           *BizPayment          `json:"payment_after,omitempty"`
	CreditLotsAfter        []CreditLot          `json:"credit_lots_after,omitempty"`
	RefundReservationAfter *RefundReservation   `json:"refund_reservation_after,omitempty"`
	Resources              []ResourceRef        `json:"resources,omitempty"`
	Notifications          []NotificationIntent `json:"notifications,omitempty"`
	CreatedAt              time.Time            `json:"created_at"`
}
