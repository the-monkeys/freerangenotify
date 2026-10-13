package dto

import (
	"encoding/json"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"net/url"
	"strings"
)

// SafeApplicationSettings preserves the public shape without serializing credentials.
type SafeApplicationSettings struct {
	application.Settings
	SecretConfigured map[string]bool `json:"secret_configured,omitempty"`
}

func ToSafeApplicationSettings(settings application.Settings) SafeApplicationSettings {
	raw, _ := json.Marshal(settings)
	var fields map[string]interface{}
	_ = json.Unmarshal(raw, &fields)
	configured := map[string]bool{}
	redactSettings(fields, "", configured)
	raw, _ = json.Marshal(fields)
	var safe application.Settings
	_ = json.Unmarshal(raw, &safe)
	return SafeApplicationSettings{Settings: safe, SecretConfigured: configured}
}

func secretField(key string) bool {
	switch key {
	case "password", "api_key", "auth_token", "meta_access_token", "meta_app_secret", "bot_token", "signing_key", "secret", "app_secret":
		return true
	}
	return false
}

func SafeEndpoint(value string) string {
	u, err := url.Parse(value)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func redactSettings(fields map[string]interface{}, prefix string, configured map[string]bool) {
	for key, value := range fields {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if secretField(key) || key == "headers" || key == "static_headers" {
			present := value != nil && value != ""
			if m, ok := value.(map[string]interface{}); ok {
				present = len(m) > 0
			}
			configured[path] = present
			delete(fields, key)
			continue
		}
		if strings.HasSuffix(key, "url") {
			if s, ok := value.(string); ok && s != "" {
				configured[path] = true
				fields[key] = SafeEndpoint(s)
			}
		}
		switch v := fields[key].(type) {
		case map[string]interface{}:
			redactSettings(v, path, configured)
		case []interface{}:
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					redactSettings(m, path, configured)
				}
			}
		}
	}
}

// PreserveSettingsSecrets accepts old request shapes but does not turn blank or
// masked display fields into replacement credentials. Clear is a separate action.
func PreserveSettingsSecrets(current, incoming application.Settings, clear []string) (application.Settings, error) {
	oldRaw, _ := json.Marshal(current)
	newRaw, _ := json.Marshal(incoming)
	var oldFields, newFields map[string]interface{}
	_ = json.Unmarshal(oldRaw, &oldFields)
	_ = json.Unmarshal(newRaw, &newFields)
	preserve(oldFields, newFields, "")
	for _, path := range clear {
		parts := strings.Split(path, ".")
		leaf := parts[len(parts)-1]
		if !secretField(leaf) && leaf != "headers" && leaf != "static_headers" {
			return incoming, fmt.Errorf("invalid secret clear path")
		}
		parent := newFields
		for _, key := range parts[:len(parts)-1] {
			next, ok := parent[key].(map[string]interface{})
			if !ok {
				return incoming, fmt.Errorf("invalid secret clear path")
			}
			parent = next
		}
		if leaf == "headers" || leaf == "static_headers" {
			delete(parent, leaf)
		} else {
			parent[leaf] = ""
		}
	}
	raw, err := json.Marshal(newFields)
	if err != nil {
		return incoming, err
	}
	var result application.Settings
	err = json.Unmarshal(raw, &result)
	return result, err
}

func preserve(old, next map[string]interface{}, prefix string) {
	for key, value := range old {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		current, exists := next[key]
		if oldItems, ok := value.([]interface{}); ok {
			if current == nil {
				next[key] = oldItems
				continue
			}
			if newItems, ok := current.([]interface{}); ok {
				for _, item := range newItems {
					newMap, ok := item.(map[string]interface{})
					if !ok {
						continue
					}
					id, _ := newMap["provider_id"].(string)
					if id == "" {
						continue
					}
					for _, previous := range oldItems {
						oldMap, ok := previous.(map[string]interface{})
						if ok && oldMap["provider_id"] == id {
							preserve(oldMap, newMap, path)
						}
					}
				}
			}
			continue
		}
		if oldMap, ok := value.(map[string]interface{}); ok {
			newMap, ok := current.(map[string]interface{})
			if !ok {
				if !exists || current == nil {
					next[key] = oldMap
				}
				continue
			}
			preserve(oldMap, newMap, path)
			continue
		}
		// Preserve absent credential/account fields as old clients send partial blocks.
		sensitive := secretField(key) || key == "headers" || key == "static_headers" || key == "account_sid"
		if sensitive {
			s, _ := current.(string)
			if !exists || current == nil || s == "" || strings.HasPrefix(s, "****") {
				next[key] = value
			}
		}
		if strings.HasSuffix(key, "url") {
			s, _ := value.(string)
			n, _ := current.(string)
			if !exists || n == SafeEndpoint(s) {
				next[key] = value
			}
		}
	}
}
