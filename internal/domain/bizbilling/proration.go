package bizbilling

import "time"

// ─── Proration (pure functions) ──────────────────────────────────────────

// PlanChargeFor computes the base charge for a plan given a quantity,
// honoring the plan's pricing model. Quantity <= 0 is treated as 1.
func PlanChargeFor(plan *Plan, quantity int) int64 {
	if quantity <= 0 {
		quantity = 1
	}
	switch plan.PricingModel {
	case PricingModelPerUnit:
		return plan.AmountPaisa * int64(quantity)
	case PricingModelTiered:
		return TieredUsageCharge(plan.Tiers, int64(quantity))
	case PricingModelVolume:
		return volumeCharge(plan.Tiers, int64(quantity))
	default: // flat
		return plan.AmountPaisa
	}
}

// volumeCharge: the tier the total quantity falls in prices ALL units.
func volumeCharge(tiers []PricingTier, quantity int64) int64 {
	if quantity <= 0 || len(tiers) == 0 {
		return 0
	}
	for _, tier := range tiers {
		if tier.UpTo <= 0 || quantity <= tier.UpTo {
			return quantity*tier.UnitPaisa + tier.FlatPaisa
		}
	}
	last := tiers[len(tiers)-1]
	return quantity*last.UnitPaisa + last.FlatPaisa
}

// ProrationResult describes the credit and charge for a mid-cycle plan change.
type ProrationResult struct {
	CreditPaisa int64 // Unused portion of the old plan (refund as credit note)
	ChargePaisa int64 // Prorated cost of the new plan for the remaining period
	NetPaisa    int64 // Charge - Credit (negative = net credit to customer)
}

// ProratePlanChange computes Salesforce-style proration when switching from
// oldPlan to newPlan at changeTime, given the current billing period.
// Both credit and charge are proportional to the time remaining in the period.
func ProratePlanChange(oldPlan, newPlan *Plan, quantity int, periodStart, periodEnd, changeTime time.Time) ProrationResult {
	total := periodEnd.Sub(periodStart)
	remaining := periodEnd.Sub(changeTime)
	if total <= 0 || remaining <= 0 {
		return ProrationResult{}
	}
	if remaining > total {
		remaining = total
	}

	ratio := float64(remaining) / float64(total)
	credit := int64(float64(PlanChargeFor(oldPlan, quantity)) * ratio)
	charge := int64(float64(PlanChargeFor(newPlan, quantity)) * ratio)

	return ProrationResult{
		CreditPaisa: credit,
		ChargePaisa: charge,
		NetPaisa:    charge - credit,
	}
}

// NextPeriodEnd advances `from` by one billing cycle.
func NextPeriodEnd(from time.Time, cycle BillingCycle, cycleDays int) time.Time {
	switch cycle {
	case BillingCycleWeekly:
		return from.AddDate(0, 0, 7)
	case BillingCycleMonthly:
		return from.AddDate(0, 1, 0)
	case BillingCycleQuarterly:
		return from.AddDate(0, 3, 0)
	case BillingCycleHalfYearly:
		return from.AddDate(0, 6, 0)
	case BillingCycleYearly:
		return from.AddDate(1, 0, 0)
	case BillingCycleCustom:
		if cycleDays > 0 {
			return from.AddDate(0, 0, cycleDays)
		}
		return from.AddDate(0, 1, 0)
	default:
		return from.AddDate(0, 1, 0)
	}
}
