package billing

// All prices in paisa (int64). 1 INR = 100 paisa.
// Credits are the canonical billing unit: 1 credit = INR 0.01.

// PlanTier defines the pricing structure for a subscription plan.
type PlanTier struct {
	Name              string           // "free" | "starter" | "pro" | "growth" | "scale"
	MonthlyFeePaisa   int64            // Monthly flat fee in paisa (0 for free trial)
	CreditsIncluded   int64            // Shared workspace credit bundle for the plan
	ChannelCreditCost map[string]int64 // channel -> credits burned per message/event
	OveragePerMessage map[string]int64 // channel -> overage price in paisa per message/event
	CreditValueINR    float64          // Value of 1 credit in INR
	BYOCFees          map[string]int64 // channel -> paisa per message (BYOC platform fee)
	PlatformFees      map[string]int64 // channel -> paisa per message (platform, push/inapp)
	// Legacy quota model (pre-credit subscriptions); populated only in LegacyRates().
	IncludedQuotas map[string]int64 `json:"included_quotas,omitempty"`
	OverageRates   map[string]int64 `json:"overage_rates,omitempty"`
}

const defaultCreditValueINR = 0.01

var defaultChannelCreditCost = map[string]int64{
	"inapp":    1,
	"webhook":  1,
	"sse":      1,
	"email":    3,
	"sms":      80,
	"whatsapp": 108,
}

var defaultOveragePerMessage = map[string]int64{
	"inapp":    3,   // ₹0.03
	"email":    6,   // ₹0.06
	"sms":      150, // ₹1.50
	"whatsapp": 200, // ₹2.00
}

// DefaultRates returns the canonical India 2026 rate card.
func DefaultRates() map[string]PlanTier {
	free := newCreditPlan("free", 0, 500)
	starter := newCreditPlan("starter", 50000, 15000)
	pro := newCreditPlan("pro", 149900, 55000)
	growth := newCreditPlan("growth", 499900, 185000)
	scale := newCreditPlan("scale", 1499900, 550000)

	return map[string]PlanTier{
		"free":    free,
		"starter": starter,
		"pro":     pro,
		"growth":  growth,
		"scale":   scale,
	}
}

// DefaultPlanBundles returns the fallback checkout catalog used when the active
// DB-backed rate card does not yet contain plan bundles.
func DefaultPlanBundles() map[string]PlanBundle {
	rates := DefaultRates()
	order := map[string]int{
		"free":    10,
		"starter": 20,
		"pro":     30,
		"growth":  40,
		"scale":   50,
	}
	names := map[string]string{
		"free":    "Free",
		"starter": "Starter",
		"pro":     "Pro",
		"growth":  "Growth",
		"scale":   "Scale",
	}
	bundles := make(map[string]PlanBundle, len(rates))
	for id, plan := range rates {
		bundles[id] = PlanBundle{
			ID:              id,
			Name:            names[id],
			AmountPaisa:     plan.MonthlyFeePaisa,
			Currency:        "INR",
			CreditsIncluded: plan.CreditsIncluded,
			ValidityDays:    365,
			Active:          true,
			DisplayOrder:    order[id],
		}
	}
	return bundles
}

func newCreditPlan(name string, monthlyFeePaisa, creditsIncluded int64) PlanTier {
	return PlanTier{
		Name:              name,
		MonthlyFeePaisa:   monthlyFeePaisa,
		CreditsIncluded:   creditsIncluded,
		ChannelCreditCost: cloneInt64Map(defaultChannelCreditCost),
		OveragePerMessage: cloneInt64Map(defaultOveragePerMessage),
		CreditValueINR:    defaultCreditValueINR,
		BYOCFees: map[string]int64{
			"email":    2,
			"whatsapp": 3,
			"sms":      3,
			"push":     0,
		},
		PlatformFees: map[string]int64{
			"push":    0,
			"inapp":   0,
			"webhook": 0,
			"sse":     0,
		},
	}
}

func cloneInt64Map(src map[string]int64) map[string]int64 {
	dst := make(map[string]int64, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

// CarrierCosts returns our actual carrier cost per message (in paisa) for India.
// Used for internal margin tracking — NOT exposed to users.
func CarrierCosts() map[string]int64 {
	return map[string]int64{
		"email":              8,   // ~₹0.08 (SES/SMTP)
		"whatsapp_utility":   15,  // ~₹0.145 (rounded)
		"whatsapp_marketing": 109, // ~₹1.09
		"sms":                17,  // ~₹0.17
		"push":               0,   // FCM/APNS free
	}
}
