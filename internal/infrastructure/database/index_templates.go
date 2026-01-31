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
