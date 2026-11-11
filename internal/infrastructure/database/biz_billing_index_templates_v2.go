package database

// New lifecycle metadata is additive; this never recreates historical indices.
func (it *IndexTemplates) GetBizLifecycleStatesTemplate() map[string]interface{} {
	return map[string]interface{}{"settings": map[string]interface{}{"number_of_shards": 1, "number_of_replicas": 0}, "mappings": map[string]interface{}{"dynamic": false, "properties": map[string]interface{}{
		"app_id": map[string]interface{}{"type": "keyword"}, "lifecycle_version": map[string]interface{}{"type": "integer"}, "status": map[string]interface{}{"type": "keyword"}, "epoch": map[string]interface{}{"type": "long"}, "primary_currency": map[string]interface{}{"type": "keyword"}, "mutation_permits": map[string]interface{}{"type": "object", "enabled": false}, "merchant_slots": map[string]interface{}{"type": "object", "enabled": false},
	}}}
}

func billingKeyword() map[string]interface{} { return map[string]interface{}{"type": "keyword"} }
func billingLong() map[string]interface{}    { return map[string]interface{}{"type": "long"} }
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
