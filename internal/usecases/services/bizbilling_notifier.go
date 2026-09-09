package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/template"
	"go.uber.org/zap"
)

// BizNotifier delivers billing notifications (invoice issued, payment
// received, dunning reminders, …) through the standard notification pipeline.
// Templates are seeded on first use per app and can then be customized by the
// business like any other template.
//
// All sends are best-effort: billing state changes must never fail because a
// notification could not be dispatched.
type BizNotifier struct {
	notifSvc     notification.Service
	templateRepo template.Repository
	logger       *zap.Logger
}

// NewBizNotifier creates a new BizNotifier.
func NewBizNotifier(notifSvc notification.Service, templateRepo template.Repository, logger *zap.Logger) *BizNotifier {
	return &BizNotifier{notifSvc: notifSvc, templateRepo: templateRepo, logger: logger}
}

// bizTemplateSeed is the default content for a billing template.
type bizTemplateSeed struct {
	Channel   string
	Subject   string
	Body      string
	Variables []string
}

// Seed templates from the billing module proposal (section 9). The worker's
// auto-fill injects user_name; everything else comes from the send Data map.
var bizTemplateSeeds = map[string]bizTemplateSeed{
	"biz_invoice_issued": {
		Channel: "email", Subject: "Invoice {{invoice_number}} from {{company_name}}",
		Body:      "Hi {{user_name}},\n\nInvoice {{invoice_number}} for {{amount}} is due on {{due_date}}.\n\nPay securely here: {{payment_link}}\n\nThank you,\n{{company_name}}",
		Variables: []string{"user_name", "invoice_number", "amount", "due_date", "payment_link", "company_name"},
	},
	"biz_estimate_sent": {
		Channel: "email", Subject: "Estimate {{estimate_number}} from {{company_name}}",
		Body:      "Hi {{user_name}},\n\nWe've prepared estimate {{estimate_number}} for {{amount}}, valid until {{valid_until}}.\n\nReview it here: {{portal_link}}\n\nThank you,\n{{company_name}}",
		Variables: []string{"user_name", "estimate_number", "amount", "valid_until", "portal_link", "company_name"},
	},
	"biz_reminder_gentle": {
		Channel: "email", Subject: "Reminder: invoice {{invoice_number}} is due",
		Body:      "Hi {{user_name}},\n\nA friendly reminder that invoice {{invoice_number}} for {{amount}} is overdue.\n\nPay here: {{payment_link}}\n\nThank you,\n{{company_name}}",
		Variables: []string{"user_name", "invoice_number", "amount", "payment_link", "company_name"},
	},
	"biz_reminder_whatsapp": {
		Channel: "whatsapp", Subject: "",
		Body:      "Hi {{user_name}}, invoice {{invoice_number}} for {{amount}} is still unpaid. Pay here: {{payment_link}}",
		Variables: []string{"user_name", "invoice_number", "amount", "payment_link"},
	},
	"biz_reminder_final": {
		Channel: "sms", Subject: "",
		Body:      "Final notice: invoice {{invoice_number}} ({{amount}}) is overdue. Pay now: {{payment_link}}",
		Variables: []string{"invoice_number", "amount", "payment_link"},
	},
	"biz_payment_received": {
		Channel: "email", Subject: "Payment received for invoice {{invoice_number}}",
		Body:      "Hi {{user_name}},\n\nWe received your payment of {{amount}} for invoice {{invoice_number}}. Thank you!\n\n{{company_name}}",
		Variables: []string{"user_name", "invoice_number", "amount", "company_name"},
	},
	"biz_payment_failed": {
		Channel: "email", Subject: "Payment failed for invoice {{invoice_ref}}",
		Body:      "Hi {{user_name}},\n\nYour payment of {{amount}} for invoice {{invoice_ref}} could not be processed. Please try again or use a different payment method.\n\n{{company_name}}",
		Variables: []string{"user_name", "invoice_ref", "amount", "company_name"},
	},
	"biz_subscription_activated": {
		Channel: "email", Subject: "Your {{plan_name}} subscription is active",
		Body:      "Hi {{user_name}},\n\nYour subscription to {{plan_name}} is now active. Next billing date: {{next_billing_date}}.\n\n{{company_name}}",
		Variables: []string{"user_name", "plan_name", "next_billing_date", "company_name"},
	},
	"biz_subscription_canceled": {
		Channel: "email", Subject: "Your subscription has been canceled",
		Body:      "Hi {{user_name}},\n\nYour subscription to {{plan_name}} has been canceled.\n\n{{company_name}}",
		Variables: []string{"user_name", "plan_name", "company_name"},
	},
	"biz_subscription_paused": {
		Channel: "email", Subject: "Your subscription is paused",
		Body:      "Hi {{user_name}},\n\nYour subscription to {{plan_name}} has been paused.\n\n{{company_name}}",
		Variables: []string{"user_name", "plan_name", "company_name"},
	},
	"biz_trial_ending": {
		Channel: "email", Subject: "Your trial ends soon",
		Body:      "Hi {{user_name}},\n\nYour trial of {{plan_name}} ends on {{trial_end}}. Add a payment method to keep your subscription.\n\n{{company_name}}",
		Variables: []string{"user_name", "plan_name", "trial_end", "company_name"},
	},
	"biz_contract_sent": {
		Channel: "email", Subject: "Contract {{contract_number}} for your review",
		Body:      "Hi {{user_name}},\n\nContract {{contract_number}} is ready for your review and acceptance: {{portal_link}}\n\n{{company_name}}",
		Variables: []string{"user_name", "contract_number", "portal_link", "company_name"},
	},
	"biz_contract_expiring": {
		Channel: "email", Subject: "Contract {{contract_number}} expires soon",
		Body:      "Hi {{user_name}},\n\nContract {{contract_number}} expires on {{end_date}}.\n\n{{company_name}}",
		Variables: []string{"user_name", "contract_number", "end_date", "company_name"},
	},
	"biz_credit_note_issued": {
		Channel: "email", Subject: "Credit note {{credit_note_number}} issued",
		Body:      "Hi {{user_name}},\n\nA credit of {{amount}} ({{credit_note_number}}) has been added to your account.\n\n{{company_name}}",
		Variables: []string{"user_name", "credit_note_number", "amount", "company_name"},
	},
	"biz_refund_processed": {
		Channel: "email", Subject: "Your refund has been processed",
		Body:      "Hi {{user_name}},\n\nA refund of {{amount}} has been processed to your original payment method.\n\n{{company_name}}",
		Variables: []string{"user_name", "amount", "company_name"},
	},
}

// Send delivers a billing notification using the named biz template,
// optionally overriding the seeded channel. Returns the notification ID
// (empty string when the send failed — errors are logged, not returned).
func (n *BizNotifier) Send(ctx context.Context, appID, userID, templateName, channelOverride string, data map[string]interface{}) string {
	if n == nil || n.notifSvc == nil || userID == "" {
		return ""
	}

	tmpl, err := n.ensureTemplate(ctx, appID, templateName)
	if err != nil {
		n.logger.Warn("bizbilling: could not resolve notification template",
			zap.String("app_id", appID), zap.String("template", templateName), zap.Error(err))
		return ""
	}

	channel := tmpl.Channel
	if channelOverride != "" {
		channel = channelOverride
	}

	notif, err := n.notifSvc.Send(ctx, notification.SendRequest{
		AppID:      appID,
		UserID:     userID,
		Channel:    notification.Channel(channel),
		Priority:   notification.PriorityNormal,
		TemplateID: tmpl.ID,
		Data:       data,
		Metadata:   map[string]interface{}{"source": "biz_billing"},
	})
	if err != nil {
		n.logger.Warn("bizbilling: notification send failed",
			zap.String("app_id", appID), zap.String("template", templateName), zap.Error(err))
		return ""
	}
	return notif.NotificationID
}

// ensureTemplate returns the app's template with the given name, seeding the
// default when it does not exist yet.
func (n *BizNotifier) ensureTemplate(ctx context.Context, appID, name string) (*template.Template, error) {
	if tmpl, err := n.templateRepo.GetByAppAndName(ctx, appID, name, "en"); err == nil && tmpl != nil {
		return tmpl, nil
	}

	seed, ok := bizTemplateSeeds[name]
	if !ok {
		seed = bizTemplateSeed{Channel: "email", Subject: name, Body: "{{message}}", Variables: []string{"message"}}
	}

	now := time.Now().UTC()
	tmpl := &template.Template{
		ID:          uuid.New().String(),
		AppID:       appID,
		Name:        name,
		Description: "Seeded by the business billing module",
		Channel:     seed.Channel,
		Subject:     seed.Subject,
		Body:        seed.Body,
		Variables:   seed.Variables,
		Version:     1,
		Status:      "active",
		Locale:      "en",
		CreatedBy:   "system",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := n.templateRepo.Create(ctx, tmpl); err != nil {
		return nil, err
	}
	n.logger.Info("bizbilling: seeded notification template",
		zap.String("app_id", appID), zap.String("template", name))
	return tmpl, nil
}
