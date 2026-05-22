package billing

import (
	"strings"

	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
)

const (
	BillingModelCredits = "credits"
	BillingModelLegacy  = "legacy"

	RateCardVersionLegacy = "legacy"
)

// LegacyRates returns the pre-credit per-channel quota rate card (main-branch semantics).
func LegacyRates() map[string]PlanTier {
	return legacyRatesWithQuotas(500, 50, 50, 1000)
}

func legacyRatesWithQuotas(emailQuota, whatsappQuota, smsQuota, pushQuota int64) map[string]PlanTier {
	return map[string]PlanTier{
		"free_trial": {
			Name:            "free_trial",
			MonthlyFeePaisa: 0,
			IncludedQuotas: map[string]int64{
				"email":    emailQuota,
				"whatsapp": whatsappQuota,
				"sms":      smsQuota,
				"push":     pushQuota,
			},
			OverageRates: map[string]int64{
				"email":    25,
				"whatsapp": 105,
				"sms":      40,
				"push":     5,
			},
			BYOCFees: map[string]int64{
				"email":    2,
				"whatsapp": 3,
				"sms":      3,
				"push":     0,
			},
			PlatformFees: map[string]int64{
				"push":  0,
				"inapp": 0,
				"sse":   0,
			},
		},
		"pro":     legacyPaidTier("pro", 149900, 7500, 750, 750, 15000),
		"growth":  legacyPaidTier("growth", 499900, 35000, 3000, 3000, 60000),
		"scale":   legacyPaidTier("scale", 1499900, 150000, 12000, 12000, 250000),
	}
}

func legacyPaidTier(name string, fee, email, whatsapp, sms, push int64) PlanTier {
	overage := map[string]int64{}
	byoc := map[string]int64{}
	switch name {
	case "pro":
		overage = map[string]int64{"email": 22, "whatsapp": 115, "sms": 38, "push": 0}
		byoc = map[string]int64{"email": 2, "whatsapp": 3, "sms": 3, "push": 0}
	case "growth":
		overage = map[string]int64{"email": 18, "whatsapp": 105, "sms": 34, "push": 0}
		byoc = map[string]int64{"email": 1, "whatsapp": 2, "sms": 2, "push": 0}
	case "scale":
		overage = map[string]int64{"email": 15, "whatsapp": 95, "sms": 30, "push": 0}
		byoc = map[string]int64{"email": 1, "whatsapp": 2, "sms": 2, "push": 0}
	}
	return PlanTier{
		Name:            name,
		MonthlyFeePaisa: fee,
		IncludedQuotas: map[string]int64{
			"email":    email,
			"whatsapp": whatsapp,
			"sms":      sms,
			"push":     push,
		},
		OverageRates: overage,
		BYOCFees:     byoc,
		PlatformFees: map[string]int64{
			"push":  0,
			"inapp": 0,
			"sse":   0,
		},
	}
}

// NormalizePlanName maps checkout aliases to canonical plan keys.
func NormalizePlanName(planName string) string {
	switch strings.TrimSpace(planName) {
	case "free_trial":
		return "free_trial"
	default:
		return strings.TrimSpace(planName)
	}
}

// ResolveRenewalPlan returns the credit-based plan applied after paid renewal or checkout.
func ResolveRenewalPlan(tier string) (PlanTier, bool) {
	name := NormalizePlanName(tier)
	if name == "free_trial" {
		name = "free"
	}
	plan, ok := DefaultRates()[name]
	return plan, ok
}

// ResolveCheckoutPlan resolves a tier for payment checkout (credit plans first, then legacy).
func ResolveCheckoutPlan(tier string) (PlanTier, bool) {
	name := NormalizePlanName(tier)
	if name == "" {
		return PlanTier{}, false
	}
	if plan, ok := DefaultRates()[name]; ok {
		return plan, true
	}
	if plan, ok := LegacyRates()[name]; ok {
		return plan, true
	}
	return PlanTier{}, false
}

// ResolveLegacyPlan looks up a plan only in the legacy per-channel quota rate card.
func ResolveLegacyPlan(planName string) (PlanTier, bool) {
	name := NormalizePlanName(planName)
	if name == "" {
		return PlanTier{}, false
	}
	plan, ok := LegacyRates()[name]
	return plan, ok
}

// ResolvePlan looks up a plan in the credit rate card, then the legacy rate card.
func ResolvePlan(planName string) (PlanTier, bool) {
	name := NormalizePlanName(planName)
	if name == "" {
		return PlanTier{}, false
	}
	if plan, ok := DefaultRates()[name]; ok {
		return plan, true
	}
	if plan, ok := LegacyRates()[name]; ok {
		return plan, true
	}
	return PlanTier{}, false
}

// BillingModel returns "credits" or "legacy" for a subscription record.
func BillingModel(sub *license.Subscription) string {
	if sub == nil {
		return BillingModelLegacy
	}
	if sub.CreditsTotal > 0 {
		return BillingModelCredits
	}
	if sub.Metadata != nil {
		if v, ok := sub.Metadata["billing_model"].(string); ok && v == BillingModelCredits {
			return BillingModelCredits
		}
	}
	return BillingModelLegacy
}

// LegacyMessageLimit returns the unified message cap for legacy subscriptions.
// metadata.message_limit wins when set; otherwise sums IncludedQuotas from the resolved plan.
func LegacyMessageLimit(sub *license.Subscription, plan PlanTier) int64 {
	if sub != nil && sub.Metadata != nil {
		if limit := metaInt64(sub.Metadata, "message_limit", 0); limit > 0 {
			return limit
		}
	}
	var total int64
	for _, q := range plan.IncludedQuotas {
		total += q
	}
	return total
}

// IsFreeTierPlan returns true for free / free_trial plans (daily caps apply).
func IsFreeTierPlan(planName string) bool {
	switch NormalizePlanName(planName) {
	case "free", "free_trial":
		return true
	default:
		return false
	}
}

// LegacyBillingChannel maps notification channels to legacy quota keys.
func LegacyBillingChannel(channel string) string {
	switch channel {
	case "in_app", "inapp", "push":
		return "push"
	case "slack", "discord", "teams", "custom", "webhook":
		return "push" // platform-style; quota uses push bucket when per-channel
	default:
		return channel
	}
}

// InferCredentialSource determines billing credential mode before send.
func InferCredentialSource(app *application.Application, channel string) string {
	if isPlatformChannel(channel) {
		return CredSourcePlatform
	}
	ch := LegacyBillingChannel(channel)
	if app == nil {
		return CredSourceSystem
	}
	switch ch {
	case "email":
		if cfg := app.Settings.EmailConfig; cfg != nil && cfg.ProviderType != "" && cfg.ProviderType != "system" {
			if cfg.SMTP != nil && cfg.SMTP.Host != "" {
				return CredSourceBYOC
			}
			if cfg.SendGrid != nil && cfg.SendGrid.APIKey != "" {
				return CredSourceBYOC
			}
		}
	case "whatsapp":
		if cfg := app.Settings.WhatsApp; cfg != nil {
			if cfg.AccountSID != "" && cfg.AuthToken != "" {
				return CredSourceBYOC
			}
			if cfg.MetaAccessToken != "" && cfg.MetaPhoneNumberID != "" {
				return CredSourceBYOC
			}
		}
	case "sms":
		if cfg := app.Settings.SMS; cfg != nil && cfg.AccountSID != "" && cfg.AuthToken != "" {
			return CredSourceBYOC
		}
	}
	return CredSourceSystem
}

func isPlatformChannel(ch string) bool {
	switch ch {
	case "push", "inapp", "in_app", "sse", "webhook",
		"slack", "discord", "teams", "custom":
		return true
	default:
		return false
	}
}

func metaInt64(meta map[string]interface{}, key string, defaultVal int64) int64 {
	if meta == nil {
		return defaultVal
	}
	v, ok := meta[key]
	if !ok {
		return defaultVal
	}
	switch n := v.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	default:
		return defaultVal
	}
}
