package bizbilling

// ─── Tax Engine (pure functions — no I/O) ────────────────────────────────
//
// GST rules implemented here:
//   - Intra-state supply (customer state == business state): CGST + SGST split.
//   - Inter-state supply (or unknown states): IGST.
//   - Tax-inclusive pricing: the line amount already contains tax, which is
//     back-calculated: taxable = amount * 100 / (100 + rate).

// InvoiceTotals is the result of a full invoice calculation.
type InvoiceTotals struct {
	SubtotalPaisa int64        // Sum of taxable line amounts (after line discounts)
	DiscountPaisa int64        // Sum of line-level discounts
	TaxPaisa      int64        // Total tax
	Breakdown     TaxBreakdown // CGST/SGST or IGST split
	TotalPaisa    int64        // Subtotal + Tax
}

// ComputeInvoiceTotals calculates subtotal, tax, and total for a set of line
// items, mutating each item's AmountPaisa to the canonical value.
//
// customerState is the customer's billing state code; when it matches
// cfg.BusinessState the tax is split CGST+SGST, otherwise IGST applies.
func ComputeInvoiceTotals(items []InvoiceLineItem, cfg TaxConfig, customerState string) InvoiceTotals {
	defaultRate := cfg.DefaultTaxRate
	if defaultRate == 0 {
		defaultRate = 18
	}

	var totals InvoiceTotals
	for i := range items {
		item := &items[i]

		qty := int64(item.Quantity)
		if qty <= 0 {
			qty = 1
			item.Quantity = 1
		}

		rate := item.TaxRate
		if rate == 0 {
			rate = defaultRate
			item.TaxRate = rate
		}

		gross := item.UnitPrice*qty - item.DiscountPaisa
		if gross < 0 {
			gross = 0
		}

		taxable := gross
		var tax int64
		if cfg.TaxInclusive {
			// Back-calculate: gross already includes tax.
			taxable = gross * 100 / int64(100+rate)
			tax = gross - taxable
		} else {
			tax = taxable * int64(rate) / 100
		}

		item.AmountPaisa = taxable
		totals.SubtotalPaisa += taxable
		totals.DiscountPaisa += item.DiscountPaisa
		totals.TaxPaisa += tax
	}

	intraState := customerState != "" && cfg.BusinessState != "" && equalState(customerState, cfg.BusinessState)
	if intraState {
		half := totals.TaxPaisa / 2
		totals.Breakdown = TaxBreakdown{CGSTPaisa: half, SGSTPaisa: totals.TaxPaisa - half}
	} else {
		totals.Breakdown = TaxBreakdown{IGSTPaisa: totals.TaxPaisa}
	}

	totals.TotalPaisa = totals.SubtotalPaisa + totals.TaxPaisa
	return totals
}

func equalState(a, b string) bool {
	return normalizeState(a) == normalizeState(b)
}

func normalizeState(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			out = append(out, r+('a'-'A'))
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = append(out, r)
		}
	}
	return string(out)
}

// LateFeeFor computes the late fee for an invoice per the app's late fee
// configuration. Returns 0 when disabled or within the grace period.
// daysOverdue is the number of whole days past the invoice due date.
func LateFeeFor(cfg LateFeeConfig, amountDuePaisa int64, daysOverdue int) int64 {
	if !cfg.Enabled || amountDuePaisa <= 0 || daysOverdue <= cfg.GraceDays {
		return 0
	}

	var fee int64
	switch cfg.Type {
	case "percentage":
		// Value is in basis points (200 = 2%).
		fee = amountDuePaisa * cfg.Value / 10000
	case "fixed":
		fee = cfg.Value
	default:
		return 0
	}

	if cfg.MaxLateFee > 0 && fee > cfg.MaxLateFee {
		fee = cfg.MaxLateFee
	}
	return fee
}
