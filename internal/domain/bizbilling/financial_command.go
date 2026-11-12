package bizbilling

// FinancialOperationSchemaVersion changes when a known command gains required
// projections. Older binaries must refuse recovery, not commit only part of it.
const FinancialOperationSchemaVersion = 5

// Historical facts remain readable and recoverable without rewriting them.
// Schema 5 also fences recovery which would omit durable delivery progress.
func SupportsFinancialSchema(version int) bool {
	return version == 2 || version == 3 || version == 4 || version == FinancialOperationSchemaVersion
}

const (
	CommandGatewayCapture     CommandKind = "gateway_capture"
	CommandExpenseConvert     CommandKind = "expense_convert"
	CommandInvoiceReminder    CommandKind = "invoice_reminder"
	CommandBalanceConsume     CommandKind = "balance_consume"
	CommandEstimateConvert    CommandKind = "estimate_convert"
	CommandBalanceSet         CommandKind = "balance_set"
	CommandOfflineRefund      CommandKind = "offline_refund"
	CommandRefundRelease      CommandKind = "refund_release"
	CommandCouponApply        CommandKind = "coupon_apply"
	CommandCreditSourceCreate CommandKind = "credit_source_create"
	CommandCreditSourceChange CommandKind = "credit_source_change"
	CommandCreditReversal     CommandKind = "credit_reversal"
	CommandInvoiceDraft       CommandKind = "invoice_draft"
	CommandInvoiceChange      CommandKind = "invoice_change"
	CommandManualPayment      CommandKind = "manual_payment"
	CommandBalanceApplication CommandKind = "balance_application"
	CommandRefundReservation  CommandKind = "refund_reservation"
	CommandRefundSettlement   CommandKind = "refund_settlement"
)

type BalanceSet struct {
	Initialize         bool   `json:"initialize,omitempty"`
	TargetPaisa        int64  `json:"target_paisa"`
	Reason             string `json:"reason"`
	RequestFingerprint string `json:"request_fingerprint,omitempty"`
}

type RefundRelease struct {
	ReservationID     string `json:"reservation_id"`
	Reason            string `json:"reason"`
	DefinitiveFailure bool   `json:"definitive_failure"`
}

type CreditSourceCreate struct {
	Type        string `json:"type"`
	AmountPaisa int64  `json:"amount_paisa"`
	InvoiceID   string `json:"invoice_id,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

type CreditSourceChange struct {
	Type      string `json:"type"`
	SourceID  string `json:"source_id"`
	Action    string `json:"action"`
	InvoiceID string `json:"invoice_id,omitempty"`
	MaxPaisa  int64  `json:"max_paisa,omitempty"`
}

type CreditReversal struct {
	ApplicationID string `json:"application_id"`
	Reason        string `json:"reason"`
}

type InvoiceDraft struct {
	Request        CreateInvoiceRequest `json:"request"`
	SubscriptionID string               `json:"subscription_id,omitempty"`
	InvoiceType    InvoiceType          `json:"invoice_type,omitempty"`
	EstimateID     string               `json:"estimate_id,omitempty"`
}

// InvoiceChange is a closed set of invoice actions. It contains user intent,
// never precomputed totals or an arbitrary persisted-document patch.
type InvoiceChange struct {
	InvoiceID string                `json:"invoice_id"`
	Action    string                `json:"action"`
	Update    *UpdateInvoiceRequest `json:"update,omitempty"`
}

type ManualPayment struct {
	Reference        string      `json:"reference,omitempty"`
	GatewayBinding   *GatewayRef `json:"gateway_binding,omitempty"`
	InvoiceID        string      `json:"invoice_id"`
	AmountPaisa      int64       `json:"amount_paisa"`
	TDSPaisa         int64       `json:"tds_paisa,omitempty"`
	Method           string      `json:"method"`
	Notes            string      `json:"notes,omitempty"`
	GatewayPaymentID string      `json:"gateway_payment_id,omitempty"`
	VerifiedCapture  bool        `json:"verified_capture,omitempty"`
}
type BalanceApplication struct {
	InvoiceID string `json:"invoice_id"`
	MaxPaisa  int64  `json:"max_paisa,omitempty"`
}
type RefundReservationRequest struct {
	PaymentID   string `json:"payment_id"`
	AmountPaisa int64  `json:"amount_paisa,omitempty"`
	Reason      string `json:"reason"`
}
type RefundSettlement struct {
	ReservationID  string `json:"reservation_id"`
	ConfirmationID string `json:"confirmation_id"`
}

// FinancialSnapshot identifies a typed resource owned by one customer stream.
// This is a persistence contract, not an interpreter for arbitrary patches.
type FinancialSnapshot interface {
	FinancialScope() Scope
	FinancialSequence() int64
	FinancialID() string
}

type CouponApply struct {
	InvoiceID string `json:"invoice_id"`
	Code      string `json:"code"`
}

type EstimateConvert struct {
	EstimateID  string `json:"estimate_id"`
	LineIndexes []int  `json:"line_indexes,omitempty"`
}

type BalanceConsume struct {
	MaxPaisa int64 `json:"max_paisa"`
}

type ExpenseConvert struct {
	ExpenseIDs []string `json:"expense_ids"`
}
type InvoiceReminder struct {
	InvoiceID   string        `json:"invoice_id"`
	DaysOverdue int           `json:"days_overdue"`
	Steps       []DunningStep `json:"steps"`
}
