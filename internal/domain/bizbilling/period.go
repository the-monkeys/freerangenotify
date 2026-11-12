package bizbilling

import "time"

const CommandSubscriptionPeriod CommandKind = "subscription_period"

type SubscriptionPeriod struct {
	ProviderProof     *VerifiedProviderPeriod `json:"provider_proof,omitempty"`
	SubscriptionID    string                  `json:"subscription_id"`
	Start             time.Time               `json:"start"`
	End               time.Time               `json:"end"`
	ProviderInvoiceID string                  `json:"provider_invoice_id,omitempty"`
}

type BillingPeriod struct {
	ID                string    `json:"id"`
	Scope             Scope     `json:"scope"`
	SubscriptionID    string    `json:"subscription_id"`
	Start             time.Time `json:"start"`
	End               time.Time `json:"end"`
	InvoiceID         string    `json:"invoice_id"`
	ProviderInvoiceID string    `json:"provider_invoice_id,omitempty"`
	LastSequence      int64     `json:"billing_sequence"`
}

func (p BillingPeriod) FinancialID() string      { return p.ID }
func (p BillingPeriod) FinancialScope() Scope    { return p.Scope }
func (p BillingPeriod) FinancialSequence() int64 { return p.LastSequence }
