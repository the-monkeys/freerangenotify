package database

// New lifecycle metadata is additive; this never recreates historical indices.
func (it *IndexTemplates) GetBizLifecycleStatesTemplate() map[string]interface{} {
	return map[string]interface{}{"settings": map[string]interface{}{"number_of_shards": 1, "number_of_replicas": 0}, "mappings": map[string]interface{}{"dynamic": false, "properties": map[string]interface{}{
		"merchant_slot_revisions": map[string]interface{}{"type": "object", "enabled": false}, "merchant_switch": map[string]interface{}{"type": "object", "enabled": false},
		"write_revision": billingLong(), "app_id": map[string]interface{}{"type": "keyword"}, "lifecycle_version": map[string]interface{}{"type": "integer"}, "status": map[string]interface{}{"type": "keyword"}, "epoch": map[string]interface{}{"type": "long"}, "primary_currency": map[string]interface{}{"type": "keyword"}, "mutation_permits": map[string]interface{}{"type": "object", "enabled": false}, "merchant_slots": map[string]interface{}{"type": "object", "enabled": false},
	}}}
}

func billingKeyword() map[string]interface{} { return map[string]interface{}{"type": "keyword"} }
func (it *IndexTemplates) GetBizGatewayEventsTemplate() map[string]interface{} {
	return billingFactTemplate(map[string]interface{}{"id": billingKeyword(), "kind": billingKeyword(), "app_id": billingKeyword(), "state": billingKeyword(), "body_digest": billingKeyword(), "gateway": map[string]interface{}{"type": "object", "enabled": false}, "data": map[string]interface{}{"type": "object", "enabled": false}, "result": map[string]interface{}{"type": "object", "enabled": false}, "header_conflict": map[string]interface{}{"type": "boolean"}, "created_at": map[string]interface{}{"type": "date"}, "updated_at": map[string]interface{}{"type": "date"}})
}
func (it *IndexTemplates) GetBizGatewayEffectsTemplate() map[string]interface{} {
	return billingFactTemplate(map[string]interface{}{
		"id": billingKeyword(), "kind": billingKeyword(), "scope": billingScope(), "provider_id": billingKeyword(), "state": billingKeyword(),
		"gateway":      map[string]interface{}{"type": "object", "properties": map[string]interface{}{"connection_id": billingKeyword(), "account_id": billingKeyword(), "mode": billingKeyword(), "credential_revision": billingLong()}},
		"order_intent": map[string]interface{}{"type": "object", "enabled": false}, "order": map[string]interface{}{"type": "object", "enabled": false}, "attempted": map[string]interface{}{"type": "boolean"}, "created_at": map[string]interface{}{"type": "date"}, "updated_at": map[string]interface{}{"type": "date"},
	})
}
func billingLong() map[string]interface{} { return map[string]interface{}{"type": "long"} }
func billingScope() map[string]interface{} {
	return map[string]interface{}{"type": "object", "dynamic": false, "properties": map[string]interface{}{"app_id": billingKeyword(), "user_id": billingKeyword(), "currency": billingKeyword()}}
}
func billingFactTemplate(properties map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"settings": map[string]interface{}{"number_of_shards": 1, "number_of_replicas": 0}, "mappings": map[string]interface{}{"dynamic": false, "properties": properties}}
}
func (it *IndexTemplates) GetBizOperationsTemplate() map[string]interface{} {
	return billingFactTemplate(map[string]interface{}{
		"id": billingKeyword(), "app_id": billingKeyword(), "scope": billingScope(), "intent_hash": billingKeyword(), "state": billingKeyword(), "head_released": map[string]interface{}{"type": "boolean"},
		"command": map[string]interface{}{"type": "object", "enabled": false}, "event": map[string]interface{}{"type": "object", "enabled": false}, "resources": map[string]interface{}{"type": "object", "enabled": false}, "error_code": billingKeyword(),
		"created_at": map[string]interface{}{"type": "date"}, "updated_at": map[string]interface{}{"type": "date"},
	})
}
func (it *IndexTemplates) GetBizJournalTemplate() map[string]interface{} {
	return billingFactTemplate(map[string]interface{}{
		"id": billingKeyword(), "app_id": billingKeyword(), "scope": billingScope(), "operation_id": billingKeyword(), "sequence": billingLong(), "kind": billingKeyword(), "created_at": map[string]interface{}{"type": "date"},
		"balance_after": map[string]interface{}{"type": "object", "enabled": false}, "notifications": map[string]interface{}{"type": "object", "enabled": false},
	})
}
func (it *IndexTemplates) GetBizStreamsTemplate() map[string]interface{} {
	return billingFactTemplate(map[string]interface{}{
		"id": billingKeyword(), "scope": billingScope(), "committed_sequence": billingLong(), "active_operation_id": billingKeyword(), "owner": billingKeyword(), "generation": billingLong(), "lease_until": map[string]interface{}{"type": "date"},
	})
}
func (it *IndexTemplates) GetBizCustomerBalancesTemplate() map[string]interface{} {
	return billingFactTemplate(map[string]interface{}{
		"scope": billingScope(), "available_paisa": billingLong(), "reserved_paisa": billingLong(), "last_sequence": billingLong(), "unresolved_legacy": map[string]interface{}{"type": "boolean"},
	})
}

func (it *IndexTemplates) GetBizCreditLotsTemplate() map[string]interface{} {
	return billingFactTemplate(map[string]interface{}{
		"id": billingKeyword(), "scope": billingScope(), "source_payment_id": billingKeyword(), "currency": billingKeyword(), "original_paisa": billingLong(), "available_paisa": billingLong(), "reserved_paisa": billingLong(), "refunded_paisa": billingLong(), "billing_sequence": billingLong(),
	})
}
func (it *IndexTemplates) GetBizRefundReservationsTemplate() map[string]interface{} {
	return billingFactTemplate(map[string]interface{}{
		"id": billingKeyword(), "scope": billingScope(), "payment_id": billingKeyword(), "amount_paisa": billingLong(), "excess_paisa": billingLong(), "state": billingKeyword(), "confirmation_id": billingKeyword(), "billing_sequence": billingLong(),
	})
}

func (it *IndexTemplates) GetBizCreditApplicationsTemplate() map[string]interface{} {
	return billingFactTemplate(map[string]interface{}{"id": billingKeyword(), "scope": billingScope(), "invoice_id": billingKeyword(), "source_type": billingKeyword(), "source_id": billingKeyword(), "amount_paisa": billingLong(), "non_refundable_paisa": billingLong(), "lot_debits": map[string]interface{}{"type": "object", "enabled": false}, "state": billingKeyword(), "billing_sequence": billingLong()})
}

func (it *IndexTemplates) GetBizCouponRedemptionsTemplate() map[string]interface{} {
	return billingFactTemplate(map[string]interface{}{"id": billingKeyword(), "scope": billingScope(), "coupon_id": billingKeyword(), "invoice_id": billingKeyword(), "discount_paisa": billingLong(), "state": billingKeyword(), "billing_sequence": billingLong()})
}
func (it *IndexTemplates) GetBizCouponCustomerUsageTemplate() map[string]interface{} {
	return billingFactTemplate(map[string]interface{}{"id": billingKeyword(), "app_id": billingKeyword(), "user_id": billingKeyword(), "coupon_id": billingKeyword(), "uses": billingLong(), "last_marker": billingKeyword()})
}

func (it *IndexTemplates) GetBizMigrationReviewsTemplate() map[string]interface{} {
	return billingFactTemplate(map[string]interface{}{"id": billingKeyword(), "app_id": billingKeyword(), "checkpoint": billingKeyword(), "primary_currency": billingKeyword(), "ready": map[string]interface{}{"type": "boolean"}})
}
func (it *IndexTemplates) GetBizLegacyResolutionsTemplate() map[string]interface{} {
	return billingFactTemplate(map[string]interface{}{"id": billingKeyword(), "app_id": billingKeyword(), "report_id": billingKeyword(), "index": billingKeyword(), "resource_id": billingKeyword(), "source_content_hash": billingKeyword()})
}

func (it *IndexTemplates) GetBizGatewayConnectionsTemplate() map[string]interface{} {
	return billingFactTemplate(map[string]interface{}{
		"id": billingKeyword(), "app_id": billingKeyword(), "account_id": billingKeyword(), "mode": billingKeyword(), "revision": billingLong(), "readiness": billingKeyword(),
		"api_authenticated": map[string]interface{}{"type": "boolean"}, "account_verified": map[string]interface{}{"type": "boolean"}, "automatic_ready": map[string]interface{}{"type": "boolean"},
		"secrets": map[string]interface{}{"type": "object", "enabled": false}, "created_at": map[string]interface{}{"type": "date"},
	})
}
