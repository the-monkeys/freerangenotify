package database

// ─── Business Billing Module — Extended Index Templates ──────────────────
// Estimates, coupons, credit notes, retainers, contracts, usage metering,
// expenses, revenue schedules, connectors, and portal tokens.
// Only created when features.biz_billing_enabled is true.

// bizMapping builds a standard single-shard index template from a property map.
func bizMapping(props map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": props,
		},
	}
}

func kw() map[string]interface{}   { return map[string]interface{}{"type": "keyword"} }
func date() map[string]interface{} { return map[string]interface{}{"type": "date"} }
func long() map[string]interface{} { return map[string]interface{}{"type": "long"} }
func boolean() map[string]interface{} { return map[string]interface{}{"type": "boolean"} }
func text() map[string]interface{} { return map[string]interface{}{"type": "text"} }
func object() map[string]interface{} {
	return map[string]interface{}{"type": "object", "enabled": false}
}

// bizLineItems is the shared nested mapping for invoice/estimate line items.
func bizLineItems() map[string]interface{} {
	return map[string]interface{}{
		"type": "nested",
		"properties": map[string]interface{}{
			"description":    text(),
			"hsn_code":       kw(),
			"quantity":       map[string]interface{}{"type": "integer"},
			"unit_price":     long(),
			"discount_paisa": long(),
			"tax_rate":       map[string]interface{}{"type": "integer"},
			"amount_paisa":   long(),
		},
	}
}

// GetBizEstimatesTemplate returns the mapping for frn_biz_estimates.
func (it *IndexTemplates) GetBizEstimatesTemplate() map[string]interface{} {
	return bizMapping(map[string]interface{}{
		"id":                   kw(),
		"app_id":               kw(),
		"user_id":              kw(),
		"estimate_number":      kw(),
		"status":               kw(),
		"line_items":           bizLineItems(),
		"subtotal_paisa":       long(),
		"tax_paisa":            long(),
		"total_paisa":          long(),
		"currency":             kw(),
		"valid_until":          date(),
		"notes":                text(),
		"terms":                text(),
		"accepted_at":          date(),
		"rejected_at":          date(),
		"converted_invoice_id": kw(),
		"notification_id":      kw(),
		"metadata":             object(),
		"created_at":           date(),
		"updated_at":           date(),
	})
}

// GetBizCouponsTemplate returns the mapping for frn_biz_coupons.
func (it *IndexTemplates) GetBizCouponsTemplate() map[string]interface{} {
	return bizMapping(map[string]interface{}{
		"id":                  kw(),
		"app_id":              kw(),
		"code":                kw(),
		"discount_type":       kw(),
		"discount_value":      long(),
		"max_uses":            map[string]interface{}{"type": "integer"},
		"current_uses":        map[string]interface{}{"type": "integer"},
		"max_uses_per_user":   map[string]interface{}{"type": "integer"},
		"applicable_plan_ids": kw(),
		"stackable":           boolean(),
		"valid_from":          date(),
		"valid_until":         date(),
		"active":              boolean(),
		"created_at":          date(),
		"updated_at":          date(),
	})
}

// GetBizCreditNotesTemplate returns the mapping for frn_biz_credit_notes.
func (it *IndexTemplates) GetBizCreditNotesTemplate() map[string]interface{} {
	return bizMapping(map[string]interface{}{
		"id":                 kw(),
		"app_id":             kw(),
		"user_id":            kw(),
		"invoice_id":         kw(),
		"credit_note_number": kw(),
		"amount_paisa":       long(),
		"balance_paisa":      long(),
		"reason":             text(),
		"status":             kw(),
		"applied_invoices":   kw(),
		"created_at":         date(),
		"updated_at":         date(),
	})
}

// GetBizRetainersTemplate returns the mapping for frn_biz_retainers.
func (it *IndexTemplates) GetBizRetainersTemplate() map[string]interface{} {
	return bizMapping(map[string]interface{}{
		"id":               kw(),
		"app_id":           kw(),
		"user_id":          kw(),
		"amount_paisa":     long(),
		"balance_paisa":    long(),
		"status":           kw(),
		"notes":            text(),
		"applied_invoices": kw(),
		"created_at":       date(),
		"updated_at":       date(),
	})
}

// GetBizContractsTemplate returns the mapping for frn_biz_contracts.
func (it *IndexTemplates) GetBizContractsTemplate() map[string]interface{} {
	return bizMapping(map[string]interface{}{
		"id":              kw(),
		"app_id":          kw(),
		"user_id":         kw(),
		"subscription_id": kw(),
		"contract_number": kw(),
		"status":          kw(),
		"terms":           text(),
		"value_paisa":     long(),
		"start_date":      date(),
		"end_date":        date(),
		"auto_renew":      boolean(),
		"accepted_at":     date(),
		"terminated_at":   date(),
		"amendments": map[string]interface{}{
			"type": "nested",
			"properties": map[string]interface{}{
				"id":             kw(),
				"description":    text(),
				"effective_date": date(),
				"created_at":     date(),
			},
		},
		"notification_id":    kw(),
		"metadata":           object(),
		"expiry_notified_at": date(),
		"created_at":         date(),
		"updated_at":         date(),
	})
}

// GetBizUsageMetersTemplate returns the mapping for frn_biz_usage_meters.
func (it *IndexTemplates) GetBizUsageMetersTemplate() map[string]interface{} {
	return bizMapping(map[string]interface{}{
		"id":          kw(),
		"app_id":      kw(),
		"name":        kw(),
		"unit":        kw(),
		"aggregation": kw(),
		"created_at":  date(),
	})
}

// GetBizUsageEventsTemplate returns the mapping for frn_biz_usage_events.
func (it *IndexTemplates) GetBizUsageEventsTemplate() map[string]interface{} {
	return bizMapping(map[string]interface{}{
		"id":              kw(),
		"app_id":          kw(),
		"user_id":         kw(),
		"subscription_id": kw(),
		"meter_id":        kw(),
		"quantity":        long(),
		"timestamp":       date(),
	})
}

// GetBizExpensesTemplate returns the mapping for frn_biz_expenses.
func (it *IndexTemplates) GetBizExpensesTemplate() map[string]interface{} {
	return bizMapping(map[string]interface{}{
		"id":               kw(),
		"app_id":           kw(),
		"category":         kw(),
		"description":      text(),
		"amount_paisa":     long(),
		"vendor":           text(),
		"date":             date(),
		"billable":         boolean(),
		"billed_to_user":   kw(),
		"invoice_id":       kw(),
		"receipt_file_id":  kw(),
		"recurring":        boolean(),
		"recurrence_cycle": kw(),
		"metadata":         object(),
		"created_at":       date(),
		"updated_at":       date(),
	})
}

// GetBizRevenueSchedulesTemplate returns the mapping for frn_biz_revenue_schedules.
func (it *IndexTemplates) GetBizRevenueSchedulesTemplate() map[string]interface{} {
	return bizMapping(map[string]interface{}{
		"id":               kw(),
		"app_id":           kw(),
		"invoice_id":       kw(),
		"subscription_id":  kw(),
		"total_paisa":      long(),
		"recognized_paisa": long(),
		"deferred_paisa":   long(),
		"entries": map[string]interface{}{
			"type": "nested",
			"properties": map[string]interface{}{
				"period":       kw(),
				"amount_paisa": long(),
				"recognized":   boolean(),
			},
		},
		"created_at": date(),
		"updated_at": date(),
	})
}

// GetBizConnectorsTemplate returns the mapping for frn_biz_connectors.
func (it *IndexTemplates) GetBizConnectorsTemplate() map[string]interface{} {
	return bizMapping(map[string]interface{}{
		"id":             kw(),
		"app_id":         kw(),
		"provider":       kw(),
		"webhook_secret": kw(),
		"event_mappings": object(),
		"active":         boolean(),
		"created_at":     date(),
		"updated_at":     date(),
	})
}

// GetBizPortalTokensTemplate returns the mapping for frn_biz_portal_tokens.
func (it *IndexTemplates) GetBizPortalTokensTemplate() map[string]interface{} {
	return bizMapping(map[string]interface{}{
		"token":      kw(),
		"app_id":     kw(),
		"user_id":    kw(),
		"expires_at": date(),
		"created_at": date(),
	})
}
