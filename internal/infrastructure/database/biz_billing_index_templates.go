package database

// ─── Business Billing Module Index Templates ─────────────────────────────
// These templates are only used when features.biz_billing_enabled is true.
// All indices are prefixed with "frn_biz_" for clear separation.

// GetBizProductsTemplate returns the Elasticsearch mapping for the frn_biz_products index.
func (it *IndexTemplates) GetBizProductsTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
			"analysis": map[string]interface{}{
				"analyzer": map[string]interface{}{
					"standard_lowercase": map[string]interface{}{
						"tokenizer": "standard",
						"filter":    []string{"lowercase"},
					},
				},
			},
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id":     map[string]interface{}{"type": "keyword"},
				"app_id": map[string]interface{}{"type": "keyword"},
				"name": map[string]interface{}{
					"type":     "text",
					"analyzer": "standard_lowercase",
					"fields": map[string]interface{}{
						"keyword": map[string]interface{}{"type": "keyword"},
					},
				},
				"description": map[string]interface{}{"type": "text"},
				"hsn_code":    map[string]interface{}{"type": "keyword"},
				"tax_rate":    map[string]interface{}{"type": "integer"},
				"unit":        map[string]interface{}{"type": "keyword"},
				"active":      map[string]interface{}{"type": "boolean"},
				"metadata":    map[string]interface{}{"type": "object", "enabled": false},
				"created_at":  map[string]interface{}{"type": "date"},
				"updated_at":  map[string]interface{}{"type": "date"},
			},
		},
	}
}

// GetBizPlansTemplate returns the Elasticsearch mapping for the frn_biz_plans index.
func (it *IndexTemplates) GetBizPlansTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
			"analysis": map[string]interface{}{
				"analyzer": map[string]interface{}{
					"standard_lowercase": map[string]interface{}{
						"tokenizer": "standard",
						"filter":    []string{"lowercase"},
					},
				},
			},
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id":         map[string]interface{}{"type": "keyword"},
				"app_id":     map[string]interface{}{"type": "keyword"},
				"product_id": map[string]interface{}{"type": "keyword"},
				"name": map[string]interface{}{
					"type":     "text",
					"analyzer": "standard_lowercase",
					"fields": map[string]interface{}{
						"keyword": map[string]interface{}{"type": "keyword"},
					},
				},
				"amount_paisa":    map[string]interface{}{"type": "long"},
				"currency":        map[string]interface{}{"type": "keyword"},
				"billing_cycle":   map[string]interface{}{"type": "keyword"},
				"cycle_days":      map[string]interface{}{"type": "integer"},
				"trial_days":      map[string]interface{}{"type": "integer"},
				"setup_fee_paisa": map[string]interface{}{"type": "long"},
				"tax_inclusive":   map[string]interface{}{"type": "boolean"},
				"pricing_model":   map[string]interface{}{"type": "keyword"},
				"tiers": map[string]interface{}{
					"type": "nested",
					"properties": map[string]interface{}{
						"up_to":      map[string]interface{}{"type": "long"},
						"unit_paisa": map[string]interface{}{"type": "long"},
						"flat_paisa": map[string]interface{}{"type": "long"},
					},
				},
				"active":     map[string]interface{}{"type": "boolean"},
				"metadata":   map[string]interface{}{"type": "object", "enabled": false},
				"created_at": map[string]interface{}{"type": "date"},
				"updated_at": map[string]interface{}{"type": "date"},
			},
		},
	}
}

// GetBizPlanAddonsTemplate returns the Elasticsearch mapping for the frn_biz_plan_addons index.
func (it *IndexTemplates) GetBizPlanAddonsTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id":      map[string]interface{}{"type": "keyword"},
				"app_id":  map[string]interface{}{"type": "keyword"},
				"plan_id": map[string]interface{}{"type": "keyword"},
				"name": map[string]interface{}{
					"type": "text",
					"fields": map[string]interface{}{
						"keyword": map[string]interface{}{"type": "keyword"},
					},
				},
				"amount_paisa":  map[string]interface{}{"type": "long"},
				"billing_cycle": map[string]interface{}{"type": "keyword"},
				"active":        map[string]interface{}{"type": "boolean"},
				"created_at":    map[string]interface{}{"type": "date"},
				"updated_at":    map[string]interface{}{"type": "date"},
			},
		},
	}
}

// GetBizConfigsTemplate returns the Elasticsearch mapping for the frn_biz_configs index.
func (it *IndexTemplates) GetBizConfigsTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"app_id":      map[string]interface{}{"type": "keyword"},
				"config_type": map[string]interface{}{"type": "keyword"},
				"data":        map[string]interface{}{"type": "object", "enabled": false},
				"updated_at":  map[string]interface{}{"type": "date"},
			},
		},
	}
}

// GetBizSubscriptionsTemplate returns the mapping for frn_biz_subscriptions
func (it *IndexTemplates) GetBizSubscriptionsTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{"number_of_shards": 1, "number_of_replicas": 0},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id":                   map[string]interface{}{"type": "keyword"},
				"app_id":               map[string]interface{}{"type": "keyword"},
				"user_id":              map[string]interface{}{"type": "keyword"},
				"plan_id":              map[string]interface{}{"type": "keyword"},
				"addon_ids":            map[string]interface{}{"type": "keyword"},
				"contract_id":          map[string]interface{}{"type": "keyword"},
				"status":               map[string]interface{}{"type": "keyword"},
				"quantity":             map[string]interface{}{"type": "integer"},
				"current_period_start": map[string]interface{}{"type": "date"},
				"current_period_end":   map[string]interface{}{"type": "date"},
				"trial_start":          map[string]interface{}{"type": "date"},
				"trial_end":            map[string]interface{}{"type": "date"},
				"canceled_at":          map[string]interface{}{"type": "date"},
				"cancel_at_period_end": map[string]interface{}{"type": "boolean"},
				"paused_at":            map[string]interface{}{"type": "date"},
				"non_renewing":         map[string]interface{}{"type": "boolean"},
				"dunning_workflow_id":  map[string]interface{}{"type": "keyword"},
				"metadata":             map[string]interface{}{"type": "object", "enabled": false},
				"created_at":           map[string]interface{}{"type": "date"},
				"updated_at":           map[string]interface{}{"type": "date"},
			},
		},
	}
}

// GetBizInvoicesTemplate returns the mapping for frn_biz_invoices
func (it *IndexTemplates) GetBizInvoicesTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{"number_of_shards": 1, "number_of_replicas": 0},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id":                map[string]interface{}{"type": "keyword"},
				"app_id":            map[string]interface{}{"type": "keyword"},
				"user_id":           map[string]interface{}{"type": "keyword"},
				"subscription_id":   map[string]interface{}{"type": "keyword"},
				"estimate_id":       map[string]interface{}{"type": "keyword"},
				"contract_id":       map[string]interface{}{"type": "keyword"},
				"invoice_number":    map[string]interface{}{"type": "keyword"},
				"invoice_type":      map[string]interface{}{"type": "keyword"},
				"status":            map[string]interface{}{"type": "keyword"},
				"line_items": map[string]interface{}{
					"type": "nested",
					"properties": map[string]interface{}{
						"description":    map[string]interface{}{"type": "text"},
						"hsn_code":       map[string]interface{}{"type": "keyword"},
						"quantity":       map[string]interface{}{"type": "integer"},
						"unit_price":     map[string]interface{}{"type": "long"},
						"discount_paisa": map[string]interface{}{"type": "long"},
						"tax_rate":       map[string]interface{}{"type": "integer"},
						"amount_paisa":   map[string]interface{}{"type": "long"},
					},
				},
				"subtotal_paisa":   map[string]interface{}{"type": "long"},
				"discount_paisa":   map[string]interface{}{"type": "long"},
				"coupon_id":        map[string]interface{}{"type": "keyword"},
				"tax_amount_paisa": map[string]interface{}{"type": "long"},
				"tax_breakdown": map[string]interface{}{
					"properties": map[string]interface{}{
						"cgst_paisa": map[string]interface{}{"type": "long"},
						"sgst_paisa": map[string]interface{}{"type": "long"},
						"igst_paisa": map[string]interface{}{"type": "long"},
					},
				},
				"late_fee_paisa":    map[string]interface{}{"type": "long"},
				"total_paisa":       map[string]interface{}{"type": "long"},
				"amount_paid_paisa": map[string]interface{}{"type": "long"},
				"amount_due_paisa":  map[string]interface{}{"type": "long"},
				"credits_applied":   map[string]interface{}{"type": "long"},
				"currency":          map[string]interface{}{"type": "keyword"},
				"exchange_rate":     map[string]interface{}{"type": "float"},
				"payment_link":      map[string]interface{}{"type": "keyword"},
				"gateway_order_id":  map[string]interface{}{"type": "keyword"},
				"payment_terms":     map[string]interface{}{"type": "keyword"},
				"due_date":          map[string]interface{}{"type": "date"},
				"issued_at":         map[string]interface{}{"type": "date"},
				"paid_at":           map[string]interface{}{"type": "date"},
				"voided_at":         map[string]interface{}{"type": "date"},
				"written_off_at":    map[string]interface{}{"type": "date"},
				"e_invoice_irn":     map[string]interface{}{"type": "keyword"},
				"e_invoice_qr":      map[string]interface{}{"type": "text"},
				"attachments":       map[string]interface{}{"type": "keyword"},
				"notification_id":   map[string]interface{}{"type": "keyword"},
				"notes":             map[string]interface{}{"type": "text"},
				"metadata":          map[string]interface{}{"type": "object", "enabled": false},
				"created_at":        map[string]interface{}{"type": "date"},
				"updated_at":        map[string]interface{}{"type": "date"},
			},
		},
	}
}

// GetBizPaymentsTemplate returns the mapping for frn_biz_payments
func (it *IndexTemplates) GetBizPaymentsTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{"number_of_shards": 1, "number_of_replicas": 0},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id":                 map[string]interface{}{"type": "keyword"},
				"app_id":             map[string]interface{}{"type": "keyword"},
				"user_id":            map[string]interface{}{"type": "keyword"},
				"invoice_ids":        map[string]interface{}{"type": "keyword"},
				"amount_paisa":       map[string]interface{}{"type": "long"},
				"currency":           map[string]interface{}{"type": "keyword"},
				"method":             map[string]interface{}{"type": "keyword"},
				"gateway":            map[string]interface{}{"type": "keyword"},
				"gateway_payment_id": map[string]interface{}{"type": "keyword"},
				"status":             map[string]interface{}{"type": "keyword"},
				"refunded_paisa":     map[string]interface{}{"type": "long"},
				"tds_paisa":          map[string]interface{}{"type": "long"},
				"notes":              map[string]interface{}{"type": "text"},
				"metadata":           map[string]interface{}{"type": "object", "enabled": false},
				"created_at":         map[string]interface{}{"type": "date"},
				"updated_at":         map[string]interface{}{"type": "date"},
			},
		},
	}
}
