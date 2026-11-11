package bizbilling

// CustomerBalance is scoped by business, customer, and currency. LastSequence
// changes in the same conditional write as the amounts, preventing double apply.
type CustomerBalance struct {
	Scope              Scope `json:"scope"`
	AvailablePaisa     int64 `json:"available_paisa"`
	ReservedPaisa      int64 `json:"reserved_paisa"`
	LastSequence       int64 `json:"last_sequence"`
	UnresolvedLegacy   bool  `json:"unresolved_legacy"`
	NonRefundablePaisa int64 `json:"non_refundable_paisa,omitempty"`
}

// CreditLot retains the cash source of excess credit. Refunds may consume only
// unspent/unreserved excess; returns are separate from credit spent on invoices.
type CreditLot struct {
	ID              string `json:"id"`
	Scope           Scope  `json:"scope"`
	SourcePaymentID string `json:"source_payment_id"`
	Currency        string `json:"currency"`
	OriginalPaisa   int64  `json:"original_paisa"`
	AvailablePaisa  int64  `json:"available_paisa"`
	ReservedPaisa   int64  `json:"reserved_paisa"`
	RefundedPaisa   int64  `json:"refunded_paisa"`
	LastSequence    int64  `json:"billing_sequence"`
}

func (l CreditLot) FinancialScope() Scope    { return l.Scope }
func (l CreditLot) FinancialSequence() int64 { return l.LastSequence }
func (l CreditLot) FinancialID() string      { return l.ID }

type OpeningBalance struct {
	AvailablePaisa   int64 `json:"available_paisa"`
	UnresolvedLegacy bool  `json:"unresolved_legacy"`
}

// UserBalanceMirror updates only the compatible primary-currency scalar on an
// existing subscriber. The customer/currency balance remains authoritative.
type UserBalanceMirror struct {
	UserID       string `json:"user_id"`
	AppID        string `json:"app_id"`
	BalancePaisa int64  `json:"balance_paisa"`
	Currency     string `json:"biz_balance_currency,omitempty"`
	LastSequence int64  `json:"biz_balance_sequence,omitempty"`
}

type BalanceAdjustment struct {
	DeltaPaisa int64  `json:"delta_paisa"`
	Reason     string `json:"reason"`
}
