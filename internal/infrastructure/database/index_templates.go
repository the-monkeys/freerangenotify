package database

// IndexTemplates contains all Elasticsearch index mappings and settings
type IndexTemplates struct{}

// GetApplicationsTemplate returns the Elasticsearch mapping for applications index
func (it *IndexTemplates) GetApplicationsTemplate() map[string]interface{} {
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
				"app_id": map[string]interface{}{
					"type": "keyword",
				},
				"app_name": map[string]interface{}{
					"type":     "text",
					"analyzer": "standard_lowercase",
					"fields": map[string]interface{}{
						"keyword": map[string]interface{}{
							"type": "keyword",
						},
					},
				},
				"description": map[string]interface{}{
					"type": "text",
				},
				"api_key": map[string]interface{}{
					"type": "keyword",
				},
				"admin_user_id": map[string]interface{}{
					"type": "keyword",
				},
				"tenant_id": map[string]interface{}{
					"type": "keyword",
				},
				"webhook_url": map[string]interface{}{
					"type": "keyword",
				},
				"webhooks": map[string]interface{}{
					"type":    "object",
					"enabled": false,
				},
				"settings": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"rate_limit": map[string]interface{}{
							"type": "integer",
						},
						"retry_attempts": map[string]interface{}{
							"type": "integer",
						},
						"default_template": map[string]interface{}{
							"type": "keyword",
						},
						"enable_webhooks": map[string]interface{}{
							"type": "boolean",
						},
						"enable_analytics": map[string]interface{}{
							"type": "boolean",
						},
						"default_preferences": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"email_enabled": map[string]interface{}{
									"type": "boolean",
								},
								"push_enabled": map[string]interface{}{
									"type": "boolean",
								},
								"sms_enabled": map[string]interface{}{
									"type": "boolean",
								},
							},
						},
						"validation_url": map[string]interface{}{
							"type": "keyword",
						},
						"validation_config": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"method": map[string]interface{}{
									"type": "keyword",
								},
								"token_placement": map[string]interface{}{
									"type": "keyword",
								},
								"token_key": map[string]interface{}{
									"type": "keyword",
								},
								"static_headers": map[string]interface{}{
									"type":    "object",
									"enabled": false,
								},
							},
						},
					},
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
				"updated_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}

// GetUsersTemplate returns the Elasticsearch mapping for users index
func (it *IndexTemplates) GetUsersTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"user_id": map[string]interface{}{
					"type": "keyword",
				},
				"app_id": map[string]interface{}{
					"type": "keyword",
				},
				"external_id": map[string]interface{}{
					"type": "keyword",
				},
				"email": map[string]interface{}{
					"type": "keyword",
				},
				"phone": map[string]interface{}{
					"type": "keyword",
				},
				"timezone": map[string]interface{}{
					"type": "keyword",
				},
				"language": map[string]interface{}{
					"type": "keyword",
				},
				"webhook_url": map[string]interface{}{
					"type": "keyword",
				},
				"preferences": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"email_enabled": map[string]interface{}{
							"type": "boolean",
						},
						"push_enabled": map[string]interface{}{
							"type": "boolean",
						},
						"sms_enabled": map[string]interface{}{
							"type": "boolean",
						},
						"quiet_hours": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"start": map[string]interface{}{
									"type": "keyword",
								},
								"end": map[string]interface{}{
									"type": "keyword",
								},
							},
						},
					},
				},
				"devices": map[string]interface{}{
					"type": "nested",
					"properties": map[string]interface{}{
						"device_id": map[string]interface{}{
							"type": "keyword",
						},
						"platform": map[string]interface{}{
							"type": "keyword",
						},
						"token": map[string]interface{}{
							"type":  "keyword",
							"index": false, // Don't index device tokens for security
						},
						"active": map[string]interface{}{
							"type": "boolean",
						},
						"last_seen": map[string]interface{}{
							"type": "date",
						},
					},
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
				"updated_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}

// GetNotificationsTemplate returns the Elasticsearch mapping for notifications index
func (it *IndexTemplates) GetNotificationsTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   3, // More shards for high-volume index
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"notification_id": map[string]interface{}{
					"type": "keyword",
				},
				"app_id": map[string]interface{}{
					"type": "keyword",
				},
				"user_id": map[string]interface{}{
					"type": "keyword",
				},
				"template_id": map[string]interface{}{
					"type": "keyword",
				},
				"channel": map[string]interface{}{
					"type": "keyword",
				},
				"priority": map[string]interface{}{
					"type": "keyword",
				},
				"status": map[string]interface{}{
					"type": "keyword",
				},
				"content": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type": "text",
						},
						"body": map[string]interface{}{
							"type": "text",
						},
						"data": map[string]interface{}{
							"type":    "object",
							"enabled": false, // Don't index dynamic data
						},
					},
				},
				"metadata": map[string]interface{}{
					"type":    "object",
					"enabled": false, // Don't index metadata for flexibility
				},
				"scheduled_at": map[string]interface{}{
					"type": "date",
				},
				"sent_at": map[string]interface{}{
					"type": "date",
				},
				"delivered_at": map[string]interface{}{
					"type": "date",
				},
				"read_at": map[string]interface{}{
					"type": "date",
				},
				"failed_at": map[string]interface{}{
					"type": "date",
				},
				"error_message": map[string]interface{}{
					"type": "text",
				},
				"retry_count": map[string]interface{}{
					"type": "integer",
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
				"updated_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}

// GetTemplatesTemplate returns the Elasticsearch mapping for templates index
func (it *IndexTemplates) GetTemplatesTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type": "keyword",
				},
				"app_id": map[string]interface{}{
					"type": "keyword",
				},
				"name": map[string]interface{}{
					"type": "keyword",
				},
				"description": map[string]interface{}{
					"type": "text",
				},
				"channel": map[string]interface{}{
					"type": "keyword",
				},
				"webhook_target": map[string]interface{}{
					"type": "keyword",
				},
				"subject": map[string]interface{}{
					"type": "text",
				},
				"body": map[string]interface{}{
					"type": "text",
				},
				"variables": map[string]interface{}{
					"type": "keyword",
				},
				"metadata": map[string]interface{}{
					"type":    "object",
					"enabled": false,
				},
				"version": map[string]interface{}{
					"type": "integer",
				},
				"status": map[string]interface{}{
					"type": "keyword",
				},
				"locale": map[string]interface{}{
					"type": "keyword",
				},
				"created_by": map[string]interface{}{
					"type": "keyword",
				},
				"updated_by": map[string]interface{}{
					"type": "keyword",
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
				"updated_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}

// GetAnalyticsTemplate returns the Elasticsearch mapping for analytics index
func (it *IndexTemplates) GetAnalyticsTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   5, // High volume, time-series data
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"event_id": map[string]interface{}{
					"type": "keyword",
				},
				"app_id": map[string]interface{}{
					"type": "keyword",
				},
				"notification_id": map[string]interface{}{
					"type": "keyword",
				},
				"user_id": map[string]interface{}{
					"type": "keyword",
				},
				"event_type": map[string]interface{}{
					"type": "keyword",
				},
				"channel": map[string]interface{}{
					"type": "keyword",
				},
				"timestamp": map[string]interface{}{
					"type": "date",
				},
				"metadata": map[string]interface{}{
					"type":    "object",
					"enabled": false, // Don't index metadata for flexibility
				},
			},
		},
	}
}

// GetAuthUsersTemplate returns the Elasticsearch mapping for admin users index
func (it *IndexTemplates) GetAuthUsersTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"user_id": map[string]interface{}{
					"type": "keyword",
				},
				"email": map[string]interface{}{
					"type": "text",
					"fields": map[string]interface{}{
						"keyword": map[string]interface{}{
							"type": "keyword",
						},
					},
				},
				"password_hash": map[string]interface{}{
					"type":  "keyword",
					"index": false, // Don't index password hash
				},
				"full_name": map[string]interface{}{
					"type": "text",
				},
				"is_active": map[string]interface{}{
					"type": "boolean",
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
				"updated_at": map[string]interface{}{
					"type": "date",
				},
				"last_login_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}

// GetPasswordResetTokensTemplate returns the Elasticsearch mapping for password reset tokens
func (it *IndexTemplates) GetPasswordResetTokensTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"token_id": map[string]interface{}{
					"type": "keyword",
				},
				"user_id": map[string]interface{}{
					"type": "keyword",
				},
				"token": map[string]interface{}{
					"type": "text",
					"fields": map[string]interface{}{
						"keyword": map[string]interface{}{
							"type": "keyword",
						},
					},
				},
				"expires_at": map[string]interface{}{
					"type": "date",
				},
				"used": map[string]interface{}{
					"type": "boolean",
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}

// GetRefreshTokensTemplate returns the Elasticsearch mapping for refresh tokens
func (it *IndexTemplates) GetRefreshTokensTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"token_id": map[string]interface{}{
					"type": "keyword",
				},
				"user_id": map[string]interface{}{
					"type": "keyword",
				},
				"token": map[string]interface{}{
					"type": "text",
					"fields": map[string]interface{}{
						"keyword": map[string]interface{}{
							"type": "keyword",
						},
					},
				},
				"expires_at": map[string]interface{}{
					"type": "date",
				},
				"revoked": map[string]interface{}{
					"type": "boolean",
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}

// --- Phase 1: Workflow & Digest Index Templates ---

// GetWorkflowsTemplate returns the Elasticsearch mapping for workflows index.
func (it *IndexTemplates) GetWorkflowsTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type": "keyword",
				},
				"app_id": map[string]interface{}{
					"type": "keyword",
				},
				"name": map[string]interface{}{
					"type": "text",
					"fields": map[string]interface{}{
						"keyword": map[string]interface{}{
							"type": "keyword",
						},
					},
				},
				"trigger_id": map[string]interface{}{
					"type": "keyword",
				},
				"description": map[string]interface{}{
					"type": "text",
				},
				"steps": map[string]interface{}{
					"type": "nested",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type": "keyword",
						},
						"name": map[string]interface{}{
							"type": "text",
						},
						"type": map[string]interface{}{
							"type": "keyword",
						},
						"order": map[string]interface{}{
							"type": "integer",
						},
						"config": map[string]interface{}{
							"type":    "object",
							"enabled": false,
						},
						"on_success": map[string]interface{}{
							"type": "keyword",
						},
						"on_failure": map[string]interface{}{
							"type": "keyword",
						},
						"skip_if": map[string]interface{}{
							"type":    "object",
							"enabled": false,
						},
					},
				},
				"status": map[string]interface{}{
					"type": "keyword",
				},
				"version": map[string]interface{}{
					"type": "integer",
				},
				"created_by": map[string]interface{}{
					"type": "keyword",
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
				"updated_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}

// GetWorkflowExecutionsTemplate returns the ES mapping for workflow_executions index.
func (it *IndexTemplates) GetWorkflowExecutionsTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type": "keyword",
				},
				"workflow_id": map[string]interface{}{
					"type": "keyword",
				},
				"app_id": map[string]interface{}{
					"type": "keyword",
				},
				"user_id": map[string]interface{}{
					"type": "keyword",
				},
				"transaction_id": map[string]interface{}{
					"type": "keyword",
				},
				"current_step_id": map[string]interface{}{
					"type": "keyword",
				},
				"status": map[string]interface{}{
					"type": "keyword",
				},
				"payload": map[string]interface{}{
					"type":    "object",
					"enabled": false,
				},
				"step_results": map[string]interface{}{
					"type":    "object",
					"enabled": false,
				},
				"started_at": map[string]interface{}{
					"type": "date",
				},
				"completed_at": map[string]interface{}{
					"type": "date",
				},
				"updated_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}

// GetWorkflowSchedulesTemplate returns the Elasticsearch mapping for workflow_schedules index (Phase 6)
func (it *IndexTemplates) GetWorkflowSchedulesTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id":             map[string]interface{}{"type": "keyword"},
				"app_id":         map[string]interface{}{"type": "keyword"},
				"environment_id": map[string]interface{}{"type": "keyword"},
				"name": map[string]interface{}{
					"type":   "text",
					"fields": map[string]interface{}{"keyword": map[string]interface{}{"type": "keyword"}},
				},
				"workflow_trigger_id": map[string]interface{}{"type": "keyword"},
				"cron":                map[string]interface{}{"type": "keyword"},
				"target_type":         map[string]interface{}{"type": "keyword"},
				"topic_id":            map[string]interface{}{"type": "keyword"},
				"payload":             map[string]interface{}{"type": "object", "enabled": false},
				"status":              map[string]interface{}{"type": "keyword"},
				"last_run_at":         map[string]interface{}{"type": "date"},
				"created_at":          map[string]interface{}{"type": "date"},
				"updated_at":          map[string]interface{}{"type": "date"},
			},
		},
	}
}

// GetDigestRulesTemplate returns the Elasticsearch mapping for digest_rules index.
func (it *IndexTemplates) GetDigestRulesTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type": "keyword",
				},
				"app_id": map[string]interface{}{
					"type": "keyword",
				},
				"name": map[string]interface{}{
					"type": "text",
					"fields": map[string]interface{}{
						"keyword": map[string]interface{}{
							"type": "keyword",
						},
					},
				},
				"digest_key": map[string]interface{}{
					"type": "keyword",
				},
				"window": map[string]interface{}{
					"type": "keyword",
				},
				"channel": map[string]interface{}{
					"type": "keyword",
				},
				"template_id": map[string]interface{}{
					"type": "keyword",
				},
				"max_batch": map[string]interface{}{
					"type": "integer",
				},
				"status": map[string]interface{}{
					"type": "keyword",
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
				"updated_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}

// ── Phase 2 Index Templates ──

// GetTopicsTemplate returns the Elasticsearch mapping for topics index.
func (it *IndexTemplates) GetTopicsTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"topic_id": map[string]interface{}{
					"type": "keyword",
				},
				"app_id": map[string]interface{}{
					"type": "keyword",
				},
				"name": map[string]interface{}{
					"type": "text",
					"fields": map[string]interface{}{
						"keyword": map[string]interface{}{
							"type": "keyword",
						},
					},
				},
				"key": map[string]interface{}{
					"type": "keyword",
				},
				"description": map[string]interface{}{
					"type": "text",
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
				"updated_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}

// GetTopicSubscriptionsTemplate returns the Elasticsearch mapping for topic_subscriptions index.
func (it *IndexTemplates) GetTopicSubscriptionsTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"subscription_id": map[string]interface{}{
					"type": "keyword",
				},
				"topic_id": map[string]interface{}{
					"type": "keyword",
				},
				"app_id": map[string]interface{}{
					"type": "keyword",
				},
				"user_id": map[string]interface{}{
					"type": "keyword",
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}

// GetAuditLogsTemplate returns the Elasticsearch mapping for audit_logs index.
func (it *IndexTemplates) GetAuditLogsTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 1,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"audit_id": map[string]interface{}{
					"type": "keyword",
				},
				"app_id": map[string]interface{}{
					"type": "keyword",
				},
				"actor_id": map[string]interface{}{
					"type": "keyword",
				},
				"actor_type": map[string]interface{}{
					"type": "keyword",
				},
				"action": map[string]interface{}{
					"type": "keyword",
				},
				"resource": map[string]interface{}{
					"type": "keyword",
				},
				"resource_id": map[string]interface{}{
					"type": "keyword",
				},
				"changes": map[string]interface{}{
					"type":    "object",
					"enabled": false,
				},
				"ip_address": map[string]interface{}{
					"type": "ip",
				},
				"user_agent": map[string]interface{}{
					"type": "text",
				},
				"timestamp": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}

// GetEnvironmentsTemplate returns the Elasticsearch mapping for the environments index.
func (it *IndexTemplates) GetEnvironmentsTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type": "keyword",
				},
				"app_id": map[string]interface{}{
					"type": "keyword",
				},
				"name": map[string]interface{}{
					"type": "keyword",
				},
				"slug": map[string]interface{}{
					"type": "keyword",
				},
				"api_key": map[string]interface{}{
					"type": "keyword",
				},
				"is_default": map[string]interface{}{
					"type": "boolean",
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
				"updated_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}

// GetResourceLinksTemplate returns the Elasticsearch mapping for app_resource_links index.
func (it *IndexTemplates) GetResourceLinksTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"link_id": map[string]interface{}{
					"type": "keyword",
				},
				"target_app_id": map[string]interface{}{
					"type": "keyword",
				},
				"source_app_id": map[string]interface{}{
					"type": "keyword",
				},
				"resource_type": map[string]interface{}{
					"type": "keyword",
				},
				"resource_id": map[string]interface{}{
					"type": "keyword",
				},
				"linked_by": map[string]interface{}{
					"type": "keyword",
				},
				"linked_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}

// GetAppMembershipsTemplate returns the Elasticsearch mapping for app_memberships index.
func (it *IndexTemplates) GetAppMembershipsTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"membership_id": map[string]interface{}{
					"type": "keyword",
				},
				"app_id": map[string]interface{}{
					"type": "keyword",
				},
				"user_id": map[string]interface{}{
					"type": "keyword",
				},
				"user_email": map[string]interface{}{
					"type": "keyword",
				},
				"role": map[string]interface{}{
					"type": "keyword",
				},
				"invited_by": map[string]interface{}{
					"type": "keyword",
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
				"updated_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}

// GetTenantsTemplate returns the Elasticsearch mapping for the tenants index (C1).
func (it *IndexTemplates) GetTenantsTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id":         map[string]interface{}{"type": "keyword"},
				"name":       map[string]interface{}{"type": "keyword"},
				"slug":       map[string]interface{}{"type": "keyword"},
				"created_by": map[string]interface{}{"type": "keyword"},
				"created_at": map[string]interface{}{"type": "date"},
				"updated_at": map[string]interface{}{"type": "date"},
			},
		},
	}
}

// GetTenantMembersTemplate returns the Elasticsearch mapping for the tenant_members index (C1).
func (it *IndexTemplates) GetTenantMembersTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id":         map[string]interface{}{"type": "keyword"},
				"tenant_id":  map[string]interface{}{"type": "keyword"},
				"user_id":    map[string]interface{}{"type": "keyword"},
				"user_email": map[string]interface{}{"type": "keyword"},
				"role":       map[string]interface{}{"type": "keyword"},
				"invited_by": map[string]interface{}{"type": "keyword"},
				"created_at": map[string]interface{}{"type": "date"},
				"updated_at": map[string]interface{}{"type": "date"},
			},
		},
	}
}

// GetDashboardNotificationsTemplate returns the Elasticsearch mapping for platform dashboard notifications.
func (it *IndexTemplates) GetDashboardNotificationsTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id":         map[string]interface{}{"type": "keyword"},
				"user_id":    map[string]interface{}{"type": "keyword"},
				"title":      map[string]interface{}{"type": "text"},
				"body":       map[string]interface{}{"type": "text"},
				"category":   map[string]interface{}{"type": "keyword"},
				"data":       map[string]interface{}{"type": "object", "enabled": false},
				"read_at":    map[string]interface{}{"type": "date"},
				"created_at": map[string]interface{}{"type": "date"},
			},
		},
	}
}

// GetSubscriptionsTemplate returns the Elasticsearch mapping for hosted subscriptions.
func (it *IndexTemplates) GetSubscriptionsTemplate() map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type": "keyword",
				},
				"tenant_id": map[string]interface{}{
					"type": "keyword",
				},
				"app_id": map[string]interface{}{
					"type": "keyword",
				},
				"plan": map[string]interface{}{
					"type": "keyword",
				},
				"status": map[string]interface{}{
					"type": "keyword",
				},
				"current_period_start": map[string]interface{}{
					"type": "date",
				},
				"current_period_end": map[string]interface{}{
					"type": "date",
				},
				"metadata": map[string]interface{}{
					"type":    "object",
					"enabled": false,
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
				"updated_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}
}
