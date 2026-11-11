package bizbilling

// CreditApplication records exactly which source funded an invoice. Restoring
// an application must restore those sources, never mint unrelated cash credit.
type CreditApplication struct {
	ID                 string           `json:"id"`
	Scope              Scope            `json:"scope"`
	InvoiceID          string           `json:"invoice_id"`
	SourceType         string           `json:"source_type"`
	SourceID           string           `json:"source_id,omitempty"`
	AmountPaisa        int64            `json:"amount_paisa"`
	NonRefundablePaisa int64            `json:"non_refundable_paisa,omitempty"`
	LotDebits          []CreditLotDebit `json:"lot_debits,omitempty"`
	State              string           `json:"state"`
	LastSequence       int64            `json:"billing_sequence"`
}
type CreditLotDebit struct {
	LotID       string `json:"lot_id"`
	AmountPaisa int64  `json:"amount_paisa"`
}

func (a CreditApplication) FinancialScope() Scope    { return a.Scope }
func (a CreditApplication) FinancialSequence() int64 { return a.LastSequence }
func (a CreditApplication) FinancialID() string      { return a.ID }
