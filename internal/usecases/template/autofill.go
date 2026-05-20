// Package template hosts shared helpers used by both the worker render path
// and any future synchronous render path that has a *user.User in scope.
//
// The functions here are deliberately small and side-effect free except for
// the explicit data-map mutation in ApplyUserAutoFill, which mirrors the
// existing "product"/"cta_url" auto-injection pattern in cmd/worker/processor.go.
package template

import (
	"strings"
	"unicode"

	"github.com/the-monkeys/freerangenotify/internal/domain/user"
)

// ApplyUserAutoFill injects user-derived values for template variables declared
// in vars when data does not already provide them. It is channel-agnostic and
// only mutates keys that the template actually declares — preventing payload
// pollution and matching the existing auto-injection contract.
//
// Variables handled (when declared in vars and missing/empty in data):
//   - name, user_name, full_name -> resolved display name
//   - first_name                 -> first token of the resolved name
//   - last_name                  -> remaining tokens of the resolved name
//
// Name resolution priority (see ResolveUserName):
//  1. user.FullName (trimmed)
//  2. user.Email local-part, humanized
//  3. user.ExternalID local-part if it looks like an email, humanized
//  4. "there"
func ApplyUserAutoFill(vars []string, data map[string]interface{}, usr *user.User) {
	if data == nil || len(vars) == 0 {
		return
	}

	name := ResolveUserName(usr)
	first, last := splitName(name)

	inject := func(key, value string) {
		if value == "" {
			return
		}
		if !containsString(vars, key) {
			return
		}
		if !needTemplateVar(data, key) {
			return
		}
		data[key] = value
	}

	inject("name", name)
	inject("user_name", name)
	inject("full_name", name)
	inject("first_name", first)
	inject("last_name", last)
}

// ResolveUserName returns the best display name for a user, in priority order:
// FullName, Email local-part, ExternalID local-part (if email-like), then
// the literal "there" fallback. It never returns an empty string.
func ResolveUserName(usr *user.User) string {
	if usr != nil {
		if n := strings.TrimSpace(usr.FullName); n != "" {
			return n
		}
		if n := humanizeEmailLocalPart(usr.Email); n != "" {
			return n
		}
		if strings.Contains(usr.ExternalID, "@") {
			if n := humanizeEmailLocalPart(usr.ExternalID); n != "" {
				return n
			}
		}
	}
	return "there"
}

// splitName partitions a resolved name into first/last tokens. A single-token
// name yields an empty last name; the caller's gating on tmpl.Variables
// ensures we won't inject an empty last_name into a template that expects it.
func splitName(full string) (first, last string) {
	parts := strings.Fields(full)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], ""
	default:
		return parts[0], strings.Join(parts[1:], " ")
	}
}

// humanizeEmailLocalPart turns "john.doe@example.com" into "John Doe".
// Returns "" when the input has no usable local-part.
func humanizeEmailLocalPart(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return ""
	}
	local := email
	if at := strings.Index(email, "@"); at > 0 {
		local = email[:at]
	}
	local = strings.ReplaceAll(local, ".", " ")
	local = strings.ReplaceAll(local, "_", " ")
	local = strings.TrimSpace(local)
	if local == "" {
		return ""
	}
	words := strings.Fields(local)
	for i, w := range words {
		r := []rune(w)
		if len(r) > 0 {
			r[0] = unicode.ToUpper(r[0])
			words[i] = string(r)
		}
	}
	return strings.Join(words, " ")
}

// needTemplateVar reports whether data is missing a non-empty string value
// for the given key. Non-string values are treated as "present" so we never
// overwrite structured payloads the caller provided.
func needTemplateVar(data map[string]interface{}, key string) bool {
	v, ok := data[key]
	if !ok {
		return true
	}
	s, isString := v.(string)
	if !isString {
		return false
	}
	return strings.TrimSpace(s) == ""
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
