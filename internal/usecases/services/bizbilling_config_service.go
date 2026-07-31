package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// BizConfigService provides business logic for billing configuration management.
type BizConfigService struct {
	repo   bizbilling.BizConfigRepository
	logger *zap.Logger
}

// NewBizConfigService creates a new BizConfigService.
func NewBizConfigService(repo bizbilling.BizConfigRepository, logger *zap.Logger) *BizConfigService {
	return &BizConfigService{
		repo:   repo,
		logger: logger,
	}
}

// GetConfig retrieves a config by type for an app.
// Returns default values if no config is stored yet.
func (s *BizConfigService) GetConfig(ctx context.Context, appID string, configType bizbilling.ConfigType) (*bizbilling.BizConfig, error) {
	if appID == "" {
		return nil, errors.BadRequest("app_id is required")
	}
	if !bizbilling.ValidConfigTypes[configType] {
		return nil, errors.BadRequest("invalid config_type")
	}

	cfg, err := s.repo.Get(ctx, appID, configType)
	if err != nil {
		return nil, err
	}

	// Return defaults if no config exists
	if cfg == nil {
		return &bizbilling.BizConfig{
			AppID:      appID,
			ConfigType: configType,
			Data:       s.defaultData(configType),
			UpdatedAt:  time.Time{},
		}, nil
	}

	return cfg, nil
}

// UpdateConfig updates a config for an app.
func (s *BizConfigService) UpdateConfig(ctx context.Context, appID string, configType bizbilling.ConfigType, data map[string]interface{}) (*bizbilling.BizConfig, error) {
	if appID == "" {
		return nil, errors.BadRequest("app_id is required")
	}
	if !bizbilling.ValidConfigTypes[configType] {
		return nil, errors.BadRequest("invalid config_type")
	}
	if data == nil {
		return nil, errors.BadRequest("data is required")
	}

	// Validate specific config types
	if err := s.validateConfigData(configType, data); err != nil {
		return nil, err
	}

	cfg := &bizbilling.BizConfig{
		AppID:      appID,
		ConfigType: configType,
		Data:       data,
		UpdatedAt:  time.Now().UTC(),
	}

	if err := s.repo.Upsert(ctx, cfg); err != nil {
		s.logger.Error("Failed to update biz config",
			zap.String("app_id", appID),
			zap.String("config_type", string(configType)),
			zap.Error(err))
		return nil, err
	}

	s.logger.Info("Biz config updated",
		zap.String("app_id", appID),
		zap.String("config_type", string(configType)))

	return cfg, nil
}

// ─── Typed Config Accessors ──────────────────────────────────────────────

// decodeConfig loads a config (with defaults) and decodes its Data map into
// the typed struct `out`.
func (s *BizConfigService) decodeConfig(ctx context.Context, appID string, configType bizbilling.ConfigType, out interface{}) error {
	cfg, err := s.GetConfig(ctx, appID, configType)
	if err != nil {
		return err
	}
	b, err := json.Marshal(cfg.Data)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// TaxConfig returns the app's typed tax configuration.
func (s *BizConfigService) TaxConfig(ctx context.Context, appID string) (bizbilling.TaxConfig, error) {
	var tc bizbilling.TaxConfig
	err := s.decodeConfig(ctx, appID, bizbilling.ConfigTypeTax, &tc)
	return tc, err
}

// DunningConfig returns the app's typed dunning configuration.
func (s *BizConfigService) DunningConfig(ctx context.Context, appID string) (bizbilling.DunningConfig, error) {
	var dc bizbilling.DunningConfig
	err := s.decodeConfig(ctx, appID, bizbilling.ConfigTypeDunning, &dc)
	return dc, err
}

// LateFeeConfig returns the app's typed late fee configuration.
func (s *BizConfigService) LateFeeConfig(ctx context.Context, appID string) (bizbilling.LateFeeConfig, error) {
	var lf bizbilling.LateFeeConfig
	err := s.decodeConfig(ctx, appID, bizbilling.ConfigTypeLateFees, &lf)
	return lf, err
}

// PaymentTermsConfig returns the app's typed payment terms configuration.
func (s *BizConfigService) PaymentTermsConfig(ctx context.Context, appID string) (bizbilling.PaymentTermsConfig, error) {
	var pt bizbilling.PaymentTermsConfig
	err := s.decodeConfig(ctx, appID, bizbilling.ConfigTypePaymentTerms, &pt)
	return pt, err
}

// BrandingConfig returns the app's typed branding configuration.
func (s *BizConfigService) BrandingConfig(ctx context.Context, appID string) (bizbilling.BrandingConfig, error) {
	var bc bizbilling.BrandingConfig
	err := s.decodeConfig(ctx, appID, bizbilling.ConfigTypeBranding, &bc)
	return bc, err
}

// ─── Sequential Document Numbering ───────────────────────────────────────

// NumberKind identifies which counter NextNumber advances.
type NumberKind string

const (
	NumberKindInvoice    NumberKind = "invoice"
	NumberKindEstimate   NumberKind = "estimate"
	NumberKindCreditNote NumberKind = "credit_note"
	NumberKindContract   NumberKind = "contract"
)

// NextNumber atomically-ish advances the per-app counter for the given
// document kind and returns a formatted number like "INV-000042".
// Counter state lives in the numbering config document.
func (s *BizConfigService) NextNumber(ctx context.Context, appID string, kind NumberKind) (string, error) {
	var nc bizbilling.NumberingConfig
	if err := s.decodeConfig(ctx, appID, bizbilling.ConfigTypeNumbering, &nc); err != nil {
		return "", err
	}

	var prefix string
	var next int64
	switch kind {
	case NumberKindInvoice:
		prefix, next = orDefault(nc.InvoicePrefix, "INV"), orDefaultN(nc.NextInvoiceNumber)
		nc.NextInvoiceNumber = next + 1
	case NumberKindEstimate:
		prefix, next = orDefault(nc.EstimatePrefix, "EST"), orDefaultN(nc.NextEstimateNumber)
		nc.NextEstimateNumber = next + 1
	case NumberKindCreditNote:
		prefix, next = orDefault(nc.CreditNotePrefix, "CN"), orDefaultN(nc.NextCreditNoteNumber)
		nc.NextCreditNoteNumber = next + 1
	case NumberKindContract:
		prefix, next = orDefault(nc.ContractPrefix, "CTR"), orDefaultN(nc.NextContractNumber)
		nc.NextContractNumber = next + 1
	default:
		return "", errors.BadRequest("invalid number kind")
	}

	data := map[string]interface{}{}
	b, _ := json.Marshal(nc)
	if err := json.Unmarshal(b, &data); err != nil {
		return "", err
	}
	if err := s.repo.Upsert(ctx, &bizbilling.BizConfig{
		AppID:      appID,
		ConfigType: bizbilling.ConfigTypeNumbering,
		Data:       data,
		UpdatedAt:  time.Now().UTC(),
	}); err != nil {
		return "", err
	}

	return FormatDocNumber(prefix, next), nil
}

// FormatDocNumber renders a document number: PREFIX-000042.
func FormatDocNumber(prefix string, n int64) string {
	return fmt.Sprintf("%s-%06d", prefix, n)
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func orDefaultN(v int64) int64 {
	if v <= 0 {
		return 1
	}
	return v
}

// validateConfigData performs type-specific validation.
func (s *BizConfigService) validateConfigData(configType bizbilling.ConfigType, data map[string]interface{}) error {
	switch configType {
	case bizbilling.ConfigTypeTax:
		return s.validateTaxConfig(data)
	case bizbilling.ConfigTypeLateFees:
		return s.validateLateFeeConfig(data)
	default:
		return nil // Other config types have no strict validation
	}
}

func (s *BizConfigService) validateTaxConfig(data map[string]interface{}) error {
	// Marshal → unmarshal to validate structure
	b, err := json.Marshal(data)
	if err != nil {
		return errors.BadRequest("invalid tax config data")
	}
	var tc bizbilling.TaxConfig
	if err := json.Unmarshal(b, &tc); err != nil {
		return errors.BadRequest("invalid tax config structure")
	}
	if tc.DefaultTaxRate < 0 || tc.DefaultTaxRate > 28 {
		return errors.BadRequest("default_tax_rate must be between 0 and 28")
	}
	return nil
}

func (s *BizConfigService) validateLateFeeConfig(data map[string]interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return errors.BadRequest("invalid late fee config data")
	}
	var lf bizbilling.LateFeeConfig
	if err := json.Unmarshal(b, &lf); err != nil {
		return errors.BadRequest("invalid late fee config structure")
	}
	if lf.Enabled {
		if lf.Type != "percentage" && lf.Type != "fixed" {
			return errors.BadRequest("late fee type must be 'percentage' or 'fixed'")
		}
		if lf.Value <= 0 {
			return errors.BadRequest("late fee value must be greater than 0")
		}
	}
	return nil
}

// defaultData returns sensible defaults for each config type.
func (s *BizConfigService) defaultData(configType bizbilling.ConfigType) map[string]interface{} {
	switch configType {
	case bizbilling.ConfigTypeTax:
		return map[string]interface{}{
			"default_tax_rate":  18,
			"tax_inclusive":     false,
			"e_invoice_enabled": false,
		}
	case bizbilling.ConfigTypeDunning:
		return map[string]interface{}{
			"enabled":         true,
			"auto_action":     "pause",
			"auto_action_day": 10,
			"steps": []map[string]interface{}{
				{"day_offset": 1, "channel": "email", "template": "biz_reminder_gentle"},
				{"day_offset": 3, "channel": "whatsapp", "template": "biz_reminder_whatsapp"},
				{"day_offset": 7, "channel": "sms", "template": "biz_reminder_final"},
			},
		}
	case bizbilling.ConfigTypeNumbering:
		return map[string]interface{}{
			"invoice_prefix":          "INV",
			"estimate_prefix":         "EST",
			"credit_note_prefix":      "CN",
			"contract_prefix":         "CTR",
			"next_invoice_number":     1,
			"next_estimate_number":    1,
			"next_credit_note_number": 1,
			"next_contract_number":    1,
		}
	case bizbilling.ConfigTypePaymentTerms:
		return map[string]interface{}{
			"default_terms": "due_on_receipt",
		}
	case bizbilling.ConfigTypeLateFees:
		return map[string]interface{}{
			"enabled":    false,
			"type":       "percentage",
			"value":      200, // 2% (in basis points)
			"grace_days": 3,
		}
	case bizbilling.ConfigTypeBranding:
		return map[string]interface{}{
			"primary_color": "#4F46E5",
		}
	case bizbilling.ConfigTypeExpenseCategories:
		return map[string]interface{}{
			"categories": []string{
				"Travel", "Software", "Marketing", "Office Supplies",
				"Rent", "Utilities", "Professional Services", "Other",
			},
		}
	default:
		return map[string]interface{}{}
	}
}
