package bizbilling

// PlanChargeForChecked retains the existing pricing models while refusing
// negative prices, malformed tiers and overflowing minor-unit arithmetic.
func PlanChargeForChecked(plan *Plan, quantity int) (int64, error) {
	if plan == nil || quantity < 0 || plan.AmountPaisa < 0 {
		return 0, ErrInvalidMoney
	}
	if quantity == 0 {
		quantity = 1
	}
	q := int64(quantity)
	switch plan.PricingModel {
	case "", PricingModelFlat:
		return plan.AmountPaisa, nil
	case PricingModelPerUnit:
		return CheckedMul(plan.AmountPaisa, q)
	case PricingModelTiered, PricingModelVolume:
		if len(plan.Tiers) == 0 {
			return 0, ErrInvalidMoney
		}
		previous := int64(0)
		for i, tier := range plan.Tiers {
			if tier.UnitPaisa < 0 || tier.FlatPaisa < 0 || tier.UpTo < 0 || (tier.UpTo == 0 && i != len(plan.Tiers)-1) || (tier.UpTo > 0 && tier.UpTo <= previous) {
				return 0, ErrInvalidMoney
			}
			previous = tier.UpTo
		}
		if plan.PricingModel == PricingModelVolume {
			selected := plan.Tiers[len(plan.Tiers)-1]
			for _, tier := range plan.Tiers {
				if tier.UpTo == 0 || q <= tier.UpTo {
					selected = tier
					break
				}
			}
			amount, err := CheckedMul(q, selected.UnitPaisa)
			if err != nil {
				return 0, err
			}
			return CheckedAdd(amount, selected.FlatPaisa)
		}
		covered, total := int64(0), int64(0)
		for _, tier := range plan.Tiers {
			if covered >= q {
				break
			}
			upper := tier.UpTo
			if upper == 0 || upper > q {
				upper = q
			}
			units := upper - covered
			if units <= 0 {
				continue
			}
			amount, err := CheckedMul(units, tier.UnitPaisa)
			if err != nil {
				return 0, err
			}
			amount, err = CheckedAdd(amount, tier.FlatPaisa)
			if err != nil {
				return 0, err
			}
			total, err = CheckedAdd(total, amount)
			if err != nil {
				return 0, err
			}
			covered = upper
		}
		if covered < q {
			amount, err := CheckedMul(q-covered, plan.Tiers[len(plan.Tiers)-1].UnitPaisa)
			if err != nil {
				return 0, err
			}
			return CheckedAdd(total, amount)
		}
		return total, nil
	default:
		return 0, ErrInvalidMoney
	}
}
