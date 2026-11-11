package database

// New lifecycle metadata is additive; this never recreates historical indices.
func (it *IndexTemplates) GetBizLifecycleStatesTemplate() map[string]interface{} {
	return map[string]interface{}{"settings": map[string]interface{}{"number_of_shards": 1, "number_of_replicas": 0}, "mappings": map[string]interface{}{"dynamic": false, "properties": map[string]interface{}{
		"app_id": map[string]interface{}{"type": "keyword"}, "lifecycle_version": map[string]interface{}{"type": "integer"}, "status": map[string]interface{}{"type": "keyword"}, "epoch": map[string]interface{}{"type": "long"}, "primary_currency": map[string]interface{}{"type": "keyword"}, "mutation_permits": map[string]interface{}{"type": "object", "enabled": false}, "merchant_slots": map[string]interface{}{"type": "object", "enabled": false},
	}}}
}
