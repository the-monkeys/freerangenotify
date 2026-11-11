package channelconnection

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type DLTTemplate struct {
	ID                string `json:"id"`
	Body              string `json:"body"`
	VariableMaxLength int    `json:"variable_max_length"`
}
type DeliveryLimits struct {
	MessagesPerMinute int64  `json:"messages_per_minute"`
	UnitsPerMinute    int64  `json:"units_per_minute"`
	DailyUnits        int64  `json:"daily_units"`
	DailySpendMinor   int64  `json:"daily_spend_minor"`
	CostMinorPerUnit  int64  `json:"cost_minor_per_unit"`
	Currency          string `json:"currency"`
	MaxSegments       int64  `json:"max_segments"`
	RateReference     string `json:"rate_reference"`
	RateVerifiedAt    string `json:"rate_verified_at"`
}
type DomesticSMSConfig struct {
	Region                string         `json:"region"`
	EntityID              string         `json:"entity_id"`
	SMSType               string         `json:"sms_type"`
	VerificationMessageID string         `json:"verification_message_id"`
	ApprovalReference     string         `json:"approval_reference"`
	ApprovedAt            string         `json:"approved_at"`
	Templates             []DLTTemplate  `json:"templates"`
	Limits                DeliveryLimits `json:"limits"`
}

func VerifiedReference(reference, stamp string) bool {
	u, e := url.Parse(reference)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	at, e := time.Parse(time.RFC3339, stamp)
	return e == nil && !at.After(time.Now().UTC())
}
func (c *DomesticSMSConfig) Validate() error {
	if c == nil || (c.Region != "mumbai" && c.Region != "singapore") || !regexp.MustCompile(`^[0-9]{6,30}$`).MatchString(c.EntityID) || !VerifiedReference(c.ApprovalReference, c.ApprovedAt) {
		return fmt.Errorf("DLT entity, approval evidence and region required")
	}
	if c.SMSType != "transactional" && c.SMSType != "transactional_opt_in" && c.SMSType != "promotional" {
		return fmt.Errorf("invalid SMS type")
	}
	p := c.Limits
	if p.MessagesPerMinute < 1 || p.MessagesPerMinute > 200 || p.UnitsPerMinute < 1 || p.UnitsPerMinute > 1000000000 || p.DailyUnits < 1 || p.DailyUnits > 1000000000 || p.DailySpendMinor < 1 || p.DailySpendMinor > 9000000000000 || p.CostMinorPerUnit < 1 || p.CostMinorPerUnit > 1000000000 || p.MaxSegments < 1 || p.MaxSegments > 10 || !regexp.MustCompile(`^[A-Z]{3}$`).MatchString(p.Currency) || !VerifiedReference(p.RateReference, p.RateVerifiedAt) {
		return fmt.Errorf("verified cost, segment cap and positive sender budgets required")
	}
	if len(c.Templates) < 1 || len(c.Templates) > 50 {
		return fmt.Errorf("one to fifty approved DLT bindings required")
	}
	ids := map[string]bool{}
	for _, t := range c.Templates {
		if !regexp.MustCompile(`^[0-9]{6,30}$`).MatchString(t.ID) || ids[t.ID] || t.Body == "" || len(t.Body) > 10000 || t.VariableMaxLength < 1 || t.VariableMaxLength > 1000 {
			return fmt.Errorf("invalid or duplicate DLT template")
		}
		ids[t.ID] = true
	}
	return nil
}
func (c *DomesticSMSConfig) Match(sender, phone, body, id string) (*DLTTemplate, error) {
	if e := c.Validate(); e != nil {
		return nil, e
	}
	header := `^[A-Za-z]{6}$`
	if c.SMSType == "promotional" {
		header = `^[0-9]{6}$`
	}
	if !regexp.MustCompile(header).MatchString(sender) || !regexp.MustCompile(`^\+91[6-9][0-9]{9}$`).MatchString(phone) {
		return nil, fmt.Errorf("approved header and Indian mobile destination required")
	}
	if id == "" && len(c.Templates) == 1 {
		id = c.Templates[0].ID
	}
	for i := range c.Templates {
		t := &c.Templates[i]
		if t.ID != id {
			continue
		}
		parts := strings.Split(t.Body, "{#var#}")
		pattern := "\\A"
		for j, part := range parts {
			if j > 0 {
				pattern += fmt.Sprintf("(?s:.{1,%d})", t.VariableMaxLength)
			}
			pattern += regexp.QuoteMeta(part)
		}
		pattern += "\\z"
		r, e := regexp.Compile(pattern)
		if e != nil || !r.MatchString(body) {
			return nil, fmt.Errorf("rendered body differs from approved DLT template")
		}
		return t, nil
	}
	return nil, fmt.Errorf("approved DLT template binding required")
}
