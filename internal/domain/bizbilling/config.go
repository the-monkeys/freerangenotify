package bizbilling

import (
	"context"
	"time"
)

// ─── Config Type Constants ───────────────────────────────────────────────

// ConfigType represents the type of billing configuration.
type ConfigType string

const (
	ConfigTypeTax               ConfigType = "tax"
	ConfigTypeDunning           ConfigType = "dunning"
	ConfigTypeBranding          ConfigType = "branding"
	ConfigTypePaymentTerms      ConfigType = "payment_terms"
	ConfigTypeLateFees          ConfigType = "late_fees"
	ConfigTypeNumbering         ConfigType = "numbering"
	ConfigTypeExpenseCategories ConfigType = "expense_categories"
)

// ValidConfigTypes contains all valid config type values.
var ValidConfigTypes = map[ConfigType]bool{
	ConfigTypeTax:               true,
	ConfigTypeDunning:           true,
	ConfigTypeBranding:          true,
	ConfigTypePaymentTerms:      true,
	ConfigTypeLateFees:          true,
	ConfigTypeNumbering:         true,
	ConfigTypeExpenseCategories: true,
}

// ─── Config Model ────────────────────────────────────────────────────────

// BizConfig stores per-app billing module configuration.
// Uses a single index with config_type to differentiate between different
// config categories (tax, dunning, branding, etc.).
type BizConfig struct {
	AppID      string                 `json:"app_id" es:"app_id"`
	ConfigType ConfigType             `json:"config_type" es:"config_type"`
	Data       map[string]interface{} `json:"data" es:"data"`
	UpdatedAt  time.Time              `json:"updated_at" es:"updated_at"`
}

// ─── Typed Config Helpers ────────────────────────────────────────────────

// TaxConfig holds the tax configuration for an app.
type TaxConfig struct {
	BusinessGSTIN   string `json:"business_gstin,omitempty"`
	BusinessState   string `json:"business_state,omitempty"`   // State code for CGST/SGST vs IGST determination
	DefaultTaxRate  int    `json:"default_tax_rate,omitempty"` // Default: 18
	TaxInclusive    bool   `json:"tax_inclusive,omitempty"`
	EInvoiceEnabled bool   `json:"e_invoice_enabled,omitempty"`
}

// DunningConfig holds the dunning (payment recovery) configuration.
type DunningConfig struct {
	Enabled       bool          `json:"enabled"`
	Steps         []DunningStep `json:"steps,omitempty"`
	AutoAction    string        `json:"auto_action,omitempty"`     // "pause", "cancel", or "none"
	AutoActionDay int           `json:"auto_action_day,omitempty"` // Days overdue before AutoAction fires (default 10)
}

// DunningStep represents a single step in the dunning sequence.
type DunningStep struct {
	DayOffset int    `json:"day_offset"` // Days after invoice due date
	Channel   string `json:"channel"`    // "email", "whatsapp", "sms"
	Template  string `json:"template"`   // Template name to use
}

// NumberingConfig holds invoice/estimate numbering configuration.
type NumberingConfig struct {
	InvoicePrefix        string `json:"invoice_prefix,omitempty"`     // Default: "INV"
	EstimatePrefix       string `json:"estimate_prefix,omitempty"`    // Default: "EST"
	CreditNotePrefix     string `json:"credit_note_prefix,omitempty"` // Default: "CN"
	ContractPrefix       string `json:"contract_prefix,omitempty"`    // Default: "CTR"
	NextInvoiceNumber    int64  `json:"next_invoice_number,omitempty"`
	NextEstimateNumber   int64  `json:"next_estimate_number,omitempty"`
	NextCreditNoteNumber int64  `json:"next_credit_note_number,omitempty"`
	NextContractNumber   int64  `json:"next_contract_number,omitempty"`
}

// LateFeeConfig holds late fee configuration.
type LateFeeConfig struct {
	Enabled    bool   `json:"enabled"`
	Type       string `json:"type,omitempty"`         // "percentage" or "fixed"
	Value      int64  `json:"value,omitempty"`        // Percentage (e.g., 2 for 2%) or fixed amount in paisa
	GraceDays  int    `json:"grace_days,omitempty"`   // Days after due date before late fee kicks in
	MaxLateFee int64  `json:"max_late_fee,omitempty"` // Maximum late fee in paisa (cap)
}

// PaymentTermsConfig holds default payment terms.
type PaymentTermsConfig struct {
	DefaultTerms string `json:"default_terms,omitempty"` // "due_on_receipt", "net_15", "net_30", "net_60", "custom"
	CustomDays   int    `json:"custom_days,omitempty"`   // Days for custom terms
}

// BrandingConfig holds invoice/portal branding.
type BrandingConfig struct {
	LogoFileID   string `json:"logo_file_id,omitempty"`
	PrimaryColor string `json:"primary_color,omitempty"` // Hex color
	CompanyName  string `json:"company_name,omitempty"`
	FooterText   string `json:"footer_text,omitempty"`
	TermsText    string `json:"terms_text,omitempty"`
}

// ─── Repository Interface ────────────────────────────────────────────────

// BizConfigRepository defines persistence operations for billing configs.
type BizConfigRepository interface {
	// Get retrieves a specific config by app ID and config type.
	// Returns nil, nil if not found (use defaults).
	Get(ctx context.Context, appID string, configType ConfigType) (*BizConfig, error)

	// Upsert creates or updates a config. Uses the composite key (app_id + config_type).
	Upsert(ctx context.Context, config *BizConfig) error
}
