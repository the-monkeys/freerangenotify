package billing

// All prices in paisa (int64). 1 INR = 100 paisa.
// Rates are for India market, effective 2026.

// PlanTier defines the pricing structure for a subscription plan.
type PlanTier struct {
	Name            string           // "free_trial" | "starter" | "growth" | "scale" | "enterprise"
	MonthlyFeePaisa int64            // Monthly flat fee in paisa (0 for free trial)
	IncludedQuotas  map[string]int64 // channel -> included message count (system cred only)
	OverageRates    map[string]int64 // channel -> paisa per message (system cred overage)
	BYOCFees        map[string]int64 // channel -> paisa per message (BYOC platform fee)
	PlatformFees    map[string]int64 // channel -> paisa per message (platform, push/inapp)
}

// DefaultRates returns the canonical India 2026 rate card.
// These are used when no custom plan overrides are present.
func DefaultRates() map[string]PlanTier {
	return DefaultRatesWithQuotas(500, 50, 50, 1000)
}

// DefaultRatesWithQuotas returns the rate card with configurable free-trial quotas.
func DefaultRatesWithQuotas(emailQuota, whatsappQuota, smsQuota, pushQuota int64) map[string]PlanTier {
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
				"email":    25, // ₹0.25
				"whatsapp": 35, // ₹0.35
				"sms":      40, // ₹0.40
				"push":     5,  // ₹0.05
			},
			BYOCFees: map[string]int64{
				"email":    2, // ₹0.02
				"whatsapp": 3, // ₹0.03
				"sms":      3, // ₹0.03
				"push":     0,
			},
			PlatformFees: map[string]int64{
				"push":     0,
				"inapp":    0,
				"sse":      0,
			},
		},
		"starter": {
			Name:            "starter",
			MonthlyFeePaisa: 99900, // ₹999
			IncludedQuotas: map[string]int64{
				"email":    5000,
				"whatsapp": 500,
				"sms":      500,
				"push":     10000,
			},
			OverageRates: map[string]int64{
				"email":    15,   // ₹0.15
				"whatsapp": 25,   // ₹0.25 (utility)
				"sms":      30,   // ₹0.30
				"push":     2,    // ₹0.02
			},
			BYOCFees: map[string]int64{
				"email":    1,    // ₹0.01
				"whatsapp": 2,    // ₹0.02
				"sms":      2,    // ₹0.02
				"push":     0,    // ₹0 (user pays Apple/Google directly or free)
			},
			PlatformFees: map[string]int64{
				"push":  0, // push is free
				"inapp": 0,
				"sse":   0,
			},
		},
		"growth": {
			Name:            "growth",
			MonthlyFeePaisa: 499900, // ₹4,999
			IncludedQuotas: map[string]int64{
				"email":    25000,
				"whatsapp": 2500,
				"sms":      2500,
				"push":     50000,
			},
			OverageRates: map[string]int64{
				"email":    15,
				"whatsapp": 25,
				"sms":      30,
				"push":     2,
			},
			BYOCFees: map[string]int64{
				"email":    1,
				"whatsapp": 2,
				"sms":      2,
				"push":     0,
			},
			PlatformFees: map[string]int64{
				"push":  0,
				"inapp": 0,
				"sse":   0,
			},
		},
		"scale": {
			Name:            "scale",
			MonthlyFeePaisa: 1499900, // ₹14,999
			IncludedQuotas: map[string]int64{
				"email":    100000,
				"whatsapp": 10000,
				"sms":      10000,
				"push":     200000,
			},
			OverageRates: map[string]int64{
				"email":    15,
				"whatsapp": 25,
				"sms":      30,
				"push":     2,
			},
			BYOCFees: map[string]int64{
				"email":    1,
				"whatsapp": 2,
				"sms":      2,
				"push":     0,
			},
			PlatformFees: map[string]int64{
				"push":  0,
				"inapp": 0,
				"sse":   0,
			},
		},
	}
}

// CarrierCosts returns our actual carrier cost per message (in paisa) for India.
// Used for internal margin tracking — NOT exposed to users.
func CarrierCosts() map[string]int64 {
	return map[string]int64{
		"email":              8,   // ₹0.08 (SES/SMTP)
		"whatsapp_utility":   15,  // ₹0.145 (rounded)
		"whatsapp_marketing": 109, // ₹1.09
		"sms":                17,  // ₹0.17 (Twilio India ~$0.002 × ₹85)
		"push":               0,   // FCM/APNS free
	}
}
