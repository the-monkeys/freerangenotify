// Package freerangenotify — webhook builder helpers.
//
// These builders construct NotificationSendParams pre-populated for the
// rich-content webhook channel. They make sending Discord/Slack/Teams
// notifications a one-liner without forcing callers to remember the
// channel + webhook_target conventions.
//
// Per-provider rendering capabilities (matrix below) are decided server-side
// by the worker's renderer; the builders simply set the optional rich
// fields. Fields not supported by a target are gracefully degraded — for
// example, polls render as a numbered list on Slack/Teams and as a real
// interactive poll on Discord.
//
//	+-------------+---------+-------+-------+---------+
//	| Field       | Discord | Slack | Teams | Generic |
//	+-------------+---------+-------+-------+---------+
//	| Attachments |   yes   |  yes  |  yes  |   raw   |
//	| Actions     |   yes   |  yes  |  yes  |   raw   |
//	| Fields      |   yes   |  yes  |  yes  |   raw   |
//	| Poll        |  native |  list |  list |   raw   |
//	| Style.Color |   yes   |  bar  |  yes  |   n/a   |
//	| Mentions    |   yes   |  yes  |  yes  |   n/a   |
//	+-------------+---------+-------+-------+---------+
package freerangenotify

// NewWebhookNotification returns a NotificationSendParams pre-configured for
// the webhook channel routed to the named custom provider on the application.
// Callers populate UserID and any rich fields they need before calling
// NotificationsClient.Send.
func NewWebhookNotification(target, title, body string) NotificationSendParams {
	return NotificationSendParams{
		Channel:       "webhook",
		Title:         title,
		Body:          body,
		WebhookTarget: target,
	}
}

// NewDiscordAlert returns a NotificationSendParams routed to a Discord
// custom provider with severity color applied.
//
// `target` must match the Name of a registered custom provider on the
// application (kind == "discord", channel == "webhook"). `severity` accepts
// "info", "success", "warning", "danger", "critical" — the renderer maps
// these to embed colors.
func NewDiscordAlert(target, title, body, severity string) NotificationSendParams {
	p := NewWebhookNotification(target, title, body)
	if severity != "" {
		p.Style = &ContentStyle{Severity: severity}
	}
	return p
}

// NewSlackAlert returns a NotificationSendParams routed to a Slack custom
// provider with severity color applied (renders as the colored sidebar bar
// on the message attachment).
//
// `target` must match the Name of a registered custom provider on the
// application (kind == "slack", channel == "webhook").
func NewSlackAlert(target, title, body, severity string) NotificationSendParams {
	p := NewWebhookNotification(target, title, body)
	if severity != "" {
		p.Style = &ContentStyle{Severity: severity}
	}
	return p
}

// NewTeamsAlert returns a NotificationSendParams routed to a Microsoft
// Teams custom provider with severity color applied (renders as themeColor).
//
// `target` must match the Name of a registered custom provider on the
// application (kind == "teams", channel == "webhook").
func NewTeamsAlert(target, title, body, severity string) NotificationSendParams {
	p := NewWebhookNotification(target, title, body)
	if severity != "" {
		p.Style = &ContentStyle{Severity: severity}
	}
	return p
}

// WithFields appends key/value Fields to the notification (renders as embed
// fields on Discord, section fields on Slack, FactSet on Teams).
func (p NotificationSendParams) WithFields(fields ...ContentField) NotificationSendParams {
	p.Fields = append(p.Fields, fields...)
	return p
}

// WithActions appends call-to-action items (renders as link buttons on
// Slack/Teams and as a markdown link list on Discord, since Discord
// incoming webhooks silently drop interactive components).
func (p NotificationSendParams) WithActions(actions ...ContentAction) NotificationSendParams {
	p.Actions = append(p.Actions, actions...)
	return p
}

// WithAttachments appends media attachments (images render inline; files as
// links).
func (p NotificationSendParams) WithAttachments(attachments ...ContentAttachment) NotificationSendParams {
	p.Attachments = append(p.Attachments, attachments...)
	return p
}

// WithPoll attaches a poll. Discord renders this as a native interactive
// poll. Slack and Teams have no native poll element on incoming webhooks
// and fall back to a numbered list of choices.
func (p NotificationSendParams) WithPoll(question string, choices ...string) NotificationSendParams {
	c := make([]ContentPollChoice, 0, len(choices))
	for _, label := range choices {
		c = append(c, ContentPollChoice{Label: label})
	}
	p.Poll = &ContentPoll{Question: question, Choices: c}
	return p
}

// WithMentions appends platform-specific mentions (e.g. `<@id>` on Discord
// and Slack). Mentions whose Platform does not match the resolved target
// are ignored by the renderer.
func (p NotificationSendParams) WithMentions(mentions ...ContentMention) NotificationSendParams {
	p.Mentions = append(p.Mentions, mentions...)
	return p
}

// WithSeverity sets or replaces Style.Severity ("info", "success",
// "warning", "danger", "critical"). The renderer maps this to the
// appropriate color preset for the resolved target.
func (p NotificationSendParams) WithSeverity(severity string) NotificationSendParams {
	if p.Style == nil {
		p.Style = &ContentStyle{}
	}
	p.Style.Severity = severity
	return p
}

// WithColor sets or replaces Style.Color (hex string, with or without
// leading "#"). Overrides any severity-derived color preset.
func (p NotificationSendParams) WithColor(hex string) NotificationSendParams {
	if p.Style == nil {
		p.Style = &ContentStyle{}
	}
	p.Style.Color = hex
	return p
}

// To sets the recipient user ID. Returns the params for chaining.
func (p NotificationSendParams) To(userID string) NotificationSendParams {
	p.UserID = userID
	return p
}
