package billing

import (
	"context"
	"fmt"
	"time"
)

// Invoice represents a computed billing statement for a tenant over a period.
type Invoice struct {
	TenantID    string     `json:"tenant_id"`
	PlanName    string     `json:"plan_name"`
	PeriodStart time.Time  `json:"period_start"`
	PeriodEnd   time.Time  `json:"period_end"`
	LineItems   []LineItem `json:"line_items"`
	TotalPaisa  int64      `json:"total_paisa"` // total charged amount
	Currency    string     `json:"currency"`    // "INR"
	GeneratedAt time.Time  `json:"generated_at"`
}

// LineItem is one row on the invoice (per channel per credential mode).
type LineItem struct {
	Channel          string `json:"channel"`
	CredentialSource string `json:"credential_source"`
	MessageCount     int64  `json:"message_count"`
	IncludedCount    int64  `json:"included_count"` // from plan quota
	BillableCount    int64  `json:"billable_count"` // count incurring charges
	UnitPricePaisa   int64  `json:"unit_price_paisa"`
	SubtotalPaisa    int64  `json:"subtotal_paisa"`
	Description      string `json:"description"`
}

// Calculator computes invoices from usage summaries and plan tiers.
type Calculator struct {
	rates map[string]PlanTier
}

// NewCalculator creates a Calculator with the provided rate card.
// Pass DefaultRates() for production; inject mocked rates in tests.
func NewCalculator(rates map[string]PlanTier) *Calculator {
	return &Calculator{rates: rates}
}

// ComputeInvoice generates an Invoice for a tenant given their plan and
// aggregated usage for a billing period. Returns an error only on
// configuration problems (e.g. unknown plan name) — never on zero usage.
func (c *Calculator) ComputeInvoice(
	ctx context.Context,
	tenantID string,
	planName string,
	usage []UsageSummary,
	periodStart, periodEnd time.Time,
) (*Invoice, error) {
	plan, ok := c.rates[planName]
	if !ok {
		return nil, fmt.Errorf("billing: unknown plan %q", planName)
	}

	inv := &Invoice{
		TenantID:    tenantID,
		PlanName:    planName,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		Currency:    "INR",
		GeneratedAt: time.Now().UTC(),
	}

	// Track how many credits have already been consumed from the shared wallet.
	var usedCredits int64

	for _, u := range usage {
		var item LineItem
		item.Channel = u.Channel
		item.CredentialSource = u.CredentialSource
		item.MessageCount = u.MessageCount

		switch u.CredentialSource {
		case CredSourceSystem:
			// Deduct from shared credits first; overage is charged per message.
			channelCost := plan.ChannelCreditCost[u.Channel]
			if channelCost <= 0 {
				channelCost = 1
			}
			totalRowCredits := u.MessageCount * channelCost

			remainingCredits := plan.CreditsIncluded - usedCredits
			if remainingCredits < 0 {
				remainingCredits = 0
			}
			coveredCredits := totalRowCredits
			if coveredCredits > remainingCredits {
				coveredCredits = remainingCredits
			}
			coveredMessages := coveredCredits / channelCost
			billableMessages := u.MessageCount - coveredMessages
			usedCredits += coveredCredits

			unitPrice := plan.OveragePerMessage[u.Channel]
			subtotal := billableMessages * unitPrice

			item.IncludedCount = coveredMessages
			item.BillableCount = billableMessages
			item.UnitPricePaisa = unitPrice
			item.SubtotalPaisa = subtotal
			item.Description = fmt.Sprintf("%s system creds: %d incl + %d overage @ ₹%.2f/msg",
				u.Channel, coveredMessages, billableMessages, float64(unitPrice)/100)

		case CredSourceBYOC:
			// Platform fee only — no carrier cost for us.
			unitPrice := plan.BYOCFees[u.Channel]
			subtotal := u.MessageCount * unitPrice

			item.IncludedCount = 0
			item.BillableCount = u.MessageCount
			item.UnitPricePaisa = unitPrice
			item.SubtotalPaisa = subtotal
			item.Description = fmt.Sprintf("%s BYOC: %d msgs @ ₹%.2f platform fee",
				u.Channel, u.MessageCount, float64(unitPrice)/100)

		case CredSourcePlatform:
			// Push/in-app/SSE — free or negligible infra fee.
			unitPrice := plan.PlatformFees[u.Channel]
			subtotal := u.MessageCount * unitPrice

			item.IncludedCount = 0
			item.BillableCount = u.MessageCount
			item.UnitPricePaisa = unitPrice
			item.SubtotalPaisa = subtotal
			item.Description = fmt.Sprintf("%s platform: %d msgs (free)", u.Channel, u.MessageCount)

		default:
			// Unknown credential source — skip silently, log in production.
			continue
		}

		inv.LineItems = append(inv.LineItems, item)
		inv.TotalPaisa += item.SubtotalPaisa
	}

	return inv, nil
}
