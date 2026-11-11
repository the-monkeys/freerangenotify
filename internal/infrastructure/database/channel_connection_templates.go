package database

func (*IndexTemplates) GetChannelConnectionsTemplate() map[string]interface{} {
	fields := map[string]interface{}{}
	for _, name := range []string{"id", "app_id", "environment_id", "channel", "provider", "credential_source", "account_id", "sender_id", "readiness"} {
		fields[name] = map[string]interface{}{"type": "keyword"}
	}
	fields["revision"] = map[string]interface{}{"type": "long"}
	fields["secrets"] = map[string]interface{}{"type": "object", "enabled": false}
	fields["facts"] = map[string]interface{}{"type": "object", "enabled": false}
	for _, name := range []string{"created_at", "updated_at"} {
		fields[name] = map[string]interface{}{"type": "date"}
	}
	return map[string]interface{}{"mappings": map[string]interface{}{"properties": map[string]interface{}{"current": map[string]interface{}{"properties": fields}, "history": map[string]interface{}{"type": "object", "enabled": false}}}}
}
