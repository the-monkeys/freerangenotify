package usecases

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
)

// richSendRequest returns a notification.SendRequest populated with every rich
// content field. These tests guard the regression where DTO-level fields were
// dropped before reaching the worker — see fix in
// internal/usecases/notification_service.go (Send / SendBulk / Broadcast).
func richSendRequest() notification.SendRequest {
	return notification.SendRequest{
		AppID:    "app-1",
		UserID:   "user-1",
		Channel:  notification.ChannelWebhook,
		Priority: notification.PriorityHigh,
		Title:    "Production Alert",
		Body:     "Disk usage exceeded 90% on web-01",
		Data: map[string]interface{}{
			"webhook_target": "Slack Alerts",
		},
		Attachments: []notification.Attachment{
			{Type: "image", URL: "https://cdn.example.com/graph.png", Name: "graph.png"},
		},
		Actions: []notification.Action{
			{Type: "link", Label: "Acknowledge", URL: "https://dash.example.com/ack/42"},
			{Type: "link", Label: "Runbook", URL: "https://dash.example.com/runbook"},
		},
		Fields: []notification.Field{
			{Key: "Host", Value: "web-01", Inline: true},
			{Key: "Region", Value: "us-east-1", Inline: true},
			{Key: "Severity", Value: "critical"},
		},
		Mentions: []notification.Mention{
			{Platform: "slack", PlatformID: "U123"},
		},
		Poll: &notification.Poll{
			Question: "Page on-call now?",
			Choices:  []notification.PollChoice{{Label: "Yes"}, {Label: "No"}},
		},
		Style: &notification.Style{Severity: "critical", Color: "#ff0000"},
	}
}

func assertRichContentMatches(t *testing.T, req notification.SendRequest, c notification.Content) {
	t.Helper()
	assert.True(t, reflect.DeepEqual(req.Attachments, c.Attachments),
		"attachments dropped: req=%v content=%v", req.Attachments, c.Attachments)
	assert.True(t, reflect.DeepEqual(req.Actions, c.Actions),
		"actions dropped: req=%v content=%v", req.Actions, c.Actions)
	assert.True(t, reflect.DeepEqual(req.Fields, c.Fields),
		"fields dropped: req=%v content=%v", req.Fields, c.Fields)
	assert.True(t, reflect.DeepEqual(req.Mentions, c.Mentions),
		"mentions dropped: req=%v content=%v", req.Mentions, c.Mentions)
	assert.True(t, reflect.DeepEqual(req.Poll, c.Poll),
		"poll dropped: req=%v content=%v", req.Poll, c.Poll)
	assert.True(t, reflect.DeepEqual(req.Style, c.Style),
		"style dropped: req=%v content=%v", req.Style, c.Style)
}

func TestNotificationService_Send_PropagatesRichFields(t *testing.T) {
	nrepo := &mockNotifRepoSend{}
	q := &mockQueueSend{}
	svc := newSendServiceForTest(defaultUsers(), defaultApps(), nrepo, q, nil)

	notif, err := svc.Send(context.Background(), richSendRequest())
	require.NoError(t, err)
	require.NotNil(t, notif)

	require.Len(t, nrepo.created, 1, "service must create exactly one notification")
	assertRichContentMatches(t, richSendRequest(), nrepo.created[0].Content)
	// The same rich content must also be present on the returned notification
	// so callers (and the API) can echo it back to clients.
	assertRichContentMatches(t, richSendRequest(), notif.Content)
}

func TestNotificationService_SendBulk_PropagatesRichFields(t *testing.T) {
	// Two users sharing the same rich payload — every enqueued notification
	// must carry an identical copy of the rich content.
	users := map[string]*user.User{
		"user-1": defaultUser(),
		"user-2": {
			UserID: "user-2",
			AppID:  "app-1",
			Email:  "bob@example.com",
			Preferences: user.Preferences{
				EmailEnabled: boolPtr(true),
				PushEnabled:  boolPtr(true),
				SMSEnabled:   boolPtr(true),
			},
		},
	}
	nrepo := &mockNotifRepoSend{}
	q := &mockQueueSend{}
	svc := newSendServiceForTest(users, defaultApps(), nrepo, q, nil)

	base := richSendRequest()
	bulk := notification.BulkSendRequest{
		AppID:       base.AppID,
		UserIDs:     []string{"user-1", "user-2"},
		Channel:     base.Channel,
		Priority:    base.Priority,
		Title:       base.Title,
		Body:        base.Body,
		Data:        base.Data,
		Attachments: base.Attachments,
		Actions:     base.Actions,
		Fields:      base.Fields,
		Mentions:    base.Mentions,
		Poll:        base.Poll,
		Style:       base.Style,
	}

	results, err := svc.SendBulk(context.Background(), bulk)
	require.NoError(t, err)
	require.Len(t, results, 2)

	require.Len(t, nrepo.created, 2, "bulk must persist one notification per user")
	for i, n := range nrepo.created {
		t.Run("user-"+n.UserID, func(t *testing.T) {
			assertRichContentMatches(t, base, n.Content)
			_ = i
		})
	}
}
