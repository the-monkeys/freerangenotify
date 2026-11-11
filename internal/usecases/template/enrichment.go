package template

import (
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"os"
)

// EnrichData is shared by admission and legacy worker rendering. Caller data
// overrides saved values, which override authored defaults.
func EnrichData(t *template.Template, payload map[string]interface{}, u *user.User, appName, userID, appID string) map[string]interface{} {
	data := map[string]interface{}{}
	for _, c := range t.Controls {
		if c.Default != "" {
			data[c.Key] = c.Default
		}
	}
	for k, v := range t.ControlValues {
		data[k] = v
	}
	for k, v := range payload {
		data[k] = v
	}
	if t.Metadata["category"] == "newsletter" {
		if _, exists := data["unsubscribe_url"]; !exists {
			data["unsubscribe_url"] = fmt.Sprintf("https://notify.example.com/v1/unsubscribe?user=%s&app=%s", userID, appID)
		}
	}
	ApplyUserAutoFill(t.Variables, data, u)
	for _, variable := range t.Variables {
		if v, ok := data[variable]; ok && v != nil && v != "" {
			continue
		}
		switch variable {
		case "product":
			if appName == "" {
				appName = "our platform"
			}
			data[variable] = appName
		case "cta_url":
			base := os.Getenv("FREERANGE_FRONTEND_URL")
			if base == "" {
				base = "http://localhost:3000"
			}
			data[variable] = base + "/login"
		}
	}
	return data
}
