package notification

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestChannelValid tests Channel.Valid() method
func TestChannelValid(t *testing.T) {
	tests := []struct {
		name     string
		channel  Channel
		expected bool
	}{
		{"Valid Push", ChannelPush, true},
		{"Valid Email", ChannelEmail, true},
		{"Valid SMS", ChannelSMS, true},
		{"Valid Webhook", ChannelWebhook, true},
		{"Valid InApp", ChannelInApp, true},
		{"Invalid Channel", Channel("invalid"), false},
		{"Empty Channel", Channel(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.channel.Valid()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestChannelString tests Channel.String() method
func TestChannelString(t *testing.T) {
	assert.Equal(t, "push", ChannelPush.String())
	assert.Equal(t, "email", ChannelEmail.String())
	assert.Equal(t, "sms", ChannelSMS.String())
}

// TestPriorityValid tests Priority.Valid() method
func TestPriorityValid(t *testing.T) {
	tests := []struct {
		name     string
		priority Priority
		expected bool
	}{
		{"Valid Low", PriorityLow, true},
		{"Valid Normal", PriorityNormal, true},
		{"Valid High", PriorityHigh, true},
		{"Valid Critical", PriorityCritical, true},
		{"Invalid Priority", Priority("invalid"), false},
		{"Empty Priority", Priority(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.priority.Valid()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPriorityString tests Priority.String() method
func TestPriorityString(t *testing.T) {
	assert.Equal(t, "low", PriorityLow.String())
	assert.Equal(t, "normal", PriorityNormal.String())
	assert.Equal(t, "high", PriorityHigh.String())
	assert.Equal(t, "critical", PriorityCritical.String())
}

// TestStatusValid tests Status.Valid() method
func TestStatusValid(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected bool
	}{
		{"Valid Pending", StatusPending, true},
		{"Valid Queued", StatusQueued, true},
		{"Valid Processing", StatusProcessing, true},
		{"Valid Sent", StatusSent, true},
		{"Valid Delivered", StatusDelivered, true},
		{"Valid Read", StatusRead, true},
		{"Valid Failed", StatusFailed, true},
		{"Valid Cancelled", StatusCancelled, true},
		{"Invalid Status", Status("invalid"), false},
		{"Empty Status", Status(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.Valid()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestStatusString tests Status.String() method
func TestStatusString(t *testing.T) {
	assert.Equal(t, "pending", StatusPending.String())
	assert.Equal(t, "queued", StatusQueued.String())
	assert.Equal(t, "sent", StatusSent.String())
}

// TestStatusIsFinal tests Status.IsFinal() method
func TestStatusIsFinal(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected bool
	}{
		{"Delivered is final", StatusDelivered, true},
		{"Read is final", StatusRead, true},
		{"Failed is final", StatusFailed, true},
		{"Cancelled is final", StatusCancelled, true},
		{"Pending is not final", StatusPending, false},
		{"Queued is not final", StatusQueued, false},
		{"Processing is not final", StatusProcessing, false},
		{"Sent is not final", StatusSent, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.IsFinal()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestNotificationValidate tests Notification.Validate() method
func TestNotificationValidate(t *testing.T) {
	validNotification := &Notification{
		NotificationID: "notif-123",
		AppID:          "app-123",
		UserID:         "user-123",
		Channel:        ChannelPush,
		Priority:       PriorityNormal,
		Status:         StatusPending,
		Content: Content{
			Title: "Test",
			Body:  "Test body",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	tests := []struct {
		name          string
		notification  *Notification
		expectedError error
	}{
		{"Valid notification", validNotification, nil},
		{
			"Missing NotificationID",
			&Notification{AppID: "app-123", UserID: "user-123", Channel: ChannelPush, Priority: PriorityNormal, Status: StatusPending, Content: Content{Title: "Test"}},
			ErrInvalidNotificationID,
		},
		{
			"Missing AppID",
			&Notification{NotificationID: "notif-123", UserID: "user-123", Channel: ChannelPush, Priority: PriorityNormal, Status: StatusPending, Content: Content{Title: "Test"}},
			ErrInvalidAppID,
		},
		{
			"Missing UserID",
			&Notification{NotificationID: "notif-123", AppID: "app-123", Channel: ChannelPush, Priority: PriorityNormal, Status: StatusPending, Content: Content{Title: "Test"}},
			ErrInvalidUserID,
		},
		{
			"Discord channel with empty UserID and TemplateID is valid",
			&Notification{NotificationID: "notif-123", AppID: "app-123", Channel: ChannelDiscord, TemplateID: "tmpl-1", Priority: PriorityNormal, Status: StatusPending, Content: Content{Title: "Test"}},
			nil,
		},
		{
			"Slack channel with empty UserID and TemplateID is valid",
			&Notification{NotificationID: "notif-123", AppID: "app-123", Channel: ChannelSlack, TemplateID: "tmpl-1", Priority: PriorityNormal, Status: StatusPending, Content: Content{Title: "Test"}},
			nil,
		},
		{
			"Teams channel with empty UserID and TemplateID is valid",
			&Notification{NotificationID: "notif-123", AppID: "app-123", Channel: ChannelTeams, TemplateID: "tmpl-1", Priority: PriorityNormal, Status: StatusPending, Content: Content{Title: "Test"}},
			nil,
		},
		{
			"Discord channel with empty UserID and no TemplateID requires webhook_url",
			&Notification{NotificationID: "notif-123", AppID: "app-123", Channel: ChannelDiscord, Priority: PriorityNormal, Status: StatusPending, Content: Content{Title: "Test"}},
			ErrInvalidUserID,
		},
		{
			"Webhook-like channel with metadata webhook_url is valid",
			&Notification{NotificationID: "notif-123", AppID: "app-123", Channel: ChannelDiscord, Priority: PriorityNormal, Status: StatusPending, Content: Content{Title: "Test"}, Metadata: map[string]interface{}{"webhook_url": "https://discord.com/api/webhooks/123/abc"}},
			nil,
		},
		{
			"Invalid Channel",
			&Notification{NotificationID: "notif-123", AppID: "app-123", UserID: "user-123", Channel: Channel("invalid"), Priority: PriorityNormal, Status: StatusPending, Content: Content{Title: "Test"}},
			ErrInvalidChannel,
		},
		{
			"Invalid Priority",
			&Notification{NotificationID: "notif-123", AppID: "app-123", UserID: "user-123", Channel: ChannelPush, Priority: Priority("invalid"), Status: StatusPending, Content: Content{Title: "Test"}},
			ErrInvalidPriority,
		},
		{
			"Invalid Status",
			&Notification{NotificationID: "notif-123", AppID: "app-123", UserID: "user-123", Channel: ChannelPush, Priority: PriorityNormal, Status: Status("invalid"), Content: Content{Title: "Test"}},
			ErrInvalidStatus,
		},
		{
			"Empty Content",
			&Notification{NotificationID: "notif-123", AppID: "app-123", UserID: "user-123", Channel: ChannelPush, Priority: PriorityNormal, Status: StatusPending, Content: Content{}},
			ErrEmptyContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.notification.Validate()
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestNotificationCanRetry tests Notification.CanRetry() method
func TestNotificationCanRetry(t *testing.T) {
	tests := []struct {
		name         string
		notification *Notification
		maxRetries   int
		expected     bool
	}{
		{
			"Can retry - failed with 0 retries",
			&Notification{Status: StatusFailed, RetryCount: 0},
			3,
			true,
		},
		{
			"Can retry - failed with 2 retries",
			&Notification{Status: StatusFailed, RetryCount: 2},
			3,
			true,
		},
		{
			"Cannot retry - max retries reached",
			&Notification{Status: StatusFailed, RetryCount: 3},
			3,
			false,
		},
		{
			"Cannot retry - status not failed",
			&Notification{Status: StatusPending, RetryCount: 0},
			3,
			false,
		},
		{
			"Cannot retry - delivered",
			&Notification{Status: StatusDelivered, RetryCount: 0},
			3,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.notification.CanRetry(tt.maxRetries)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestNotificationIsScheduled tests Notification.IsScheduled() method
func TestNotificationIsScheduled(t *testing.T) {
	now := time.Now()
	future := now.Add(1 * time.Hour)
	past := now.Add(-1 * time.Hour)

	tests := []struct {
		name         string
		notification *Notification
		expected     bool
	}{
		{
			"Scheduled for future",
			&Notification{ScheduledAt: &future},
			true,
		},
		{
			"Scheduled in past",
			&Notification{ScheduledAt: &past},
			false,
		},
		{
			"No schedule",
			&Notification{ScheduledAt: nil},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.notification.IsScheduled()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestDefaultFilter tests DefaultFilter() function
func TestDefaultFilter(t *testing.T) {
	filter := DefaultFilter()

	assert.Equal(t, 1, filter.Page)
	assert.Equal(t, 50, filter.PageSize)
	assert.Equal(t, "created_at", filter.SortBy)
	assert.Equal(t, "desc", filter.SortOrder)
}

// TestNotificationFilterValidate tests NotificationFilter.Validate() method
func TestNotificationFilterValidate(t *testing.T) {
	tests := []struct {
		name          string
		filter        *NotificationFilter
		expectedError error
		expectedPage  int
		expectedSize  int
		expectedOrder string
	}{
		{
			"Valid filter",
			&NotificationFilter{Page: 1, PageSize: 20, SortOrder: "asc"},
			nil,
			1,
			20,
			"asc",
		},
		{
			"Invalid page - corrected to 1",
			&NotificationFilter{Page: 0, PageSize: 20, SortOrder: "desc"},
			nil,
			1,
			20,
			"desc",
		},
		{
			"Invalid page size - corrected to 50",
			&NotificationFilter{Page: 1, PageSize: 0, SortOrder: "desc"},
			nil,
			1,
			50,
			"desc",
		},
		{
			"Page size too large - corrected to 50",
			&NotificationFilter{Page: 1, PageSize: 200, SortOrder: "desc"},
			nil,
			1,
			50,
			"desc",
		},
		{
			"Invalid sort order - corrected to desc",
			&NotificationFilter{Page: 1, PageSize: 20, SortOrder: "invalid"},
			nil,
			1,
			20,
			"desc",
		},
		{
			"Invalid channel",
			&NotificationFilter{Page: 1, PageSize: 20, SortOrder: "desc", Channel: Channel("invalid")},
			ErrInvalidChannel,
			1,
			20,
			"desc",
		},
		{
			"Invalid priority",
			&NotificationFilter{Page: 1, PageSize: 20, SortOrder: "desc", Priority: Priority("invalid")},
			ErrInvalidPriority,
			1,
			20,
			"desc",
		},
		{
			"Invalid status",
			&NotificationFilter{Page: 1, PageSize: 20, SortOrder: "desc", Status: Status("invalid")},
			ErrInvalidStatus,
			1,
			20,
			"desc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.filter.Validate()
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPage, tt.filter.Page)
				assert.Equal(t, tt.expectedSize, tt.filter.PageSize)
				assert.Equal(t, tt.expectedOrder, tt.filter.SortOrder)
			}
		})
	}
}

// TestIsWebhookLikeChannel tests IsWebhookLikeChannel() function
func TestIsWebhookLikeChannel(t *testing.T) {
	tests := []struct {
		name     string
		channel  Channel
		expected bool
	}{
		// True cases — all URL-based delivery channels
		{"Webhook", ChannelWebhook, true},
		{"Discord", ChannelDiscord, true},
		{"Slack", ChannelSlack, true},
		{"Teams", ChannelTeams, true},

		// False cases — user-targeted channels
		{"Push", ChannelPush, false},
		{"Email", ChannelEmail, false},
		{"SMS", ChannelSMS, false},
		{"InApp", ChannelInApp, false},
		{"SSE", ChannelSSE, false},
		{"WhatsApp", ChannelWhatsApp, false},

		// Edge cases
		{"Empty string", Channel(""), false},
		{"Invalid channel", Channel("fax"), false},
		{"Case-sensitive Webhook", Channel("Webhook"), false},
		{"Case-sensitive Discord", Channel("DISCORD"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsWebhookLikeChannel(tt.channel)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSendRequestValidate tests SendRequest.Validate() method
func TestSendRequestValidate(t *testing.T) {
	future := time.Now().Add(1 * time.Hour)
	past := time.Now().Add(-1 * time.Hour)

	tests := []struct {
		name          string
		req           SendRequest
		expectedError error
	}{
		{
			"Valid email send request",
			SendRequest{AppID: "app-1", UserID: "user-1", Channel: ChannelEmail, Priority: PriorityNormal, TemplateID: "tmpl-1"},
			nil,
		},
		{
			"Missing AppID",
			SendRequest{UserID: "user-1", Channel: ChannelEmail, Priority: PriorityNormal, TemplateID: "tmpl-1"},
			ErrInvalidAppID,
		},
		{
			"Missing UserID for non-webhook channel",
			SendRequest{AppID: "app-1", Channel: ChannelEmail, Priority: PriorityNormal, TemplateID: "tmpl-1"},
			ErrInvalidUserID,
		},
		{
			"Webhook channel with empty UserID and TemplateID is valid",
			SendRequest{AppID: "app-1", Channel: ChannelWebhook, Priority: PriorityNormal, TemplateID: "tmpl-1"},
			nil,
		},
		{
			"Discord channel with empty UserID and TemplateID is valid",
			SendRequest{AppID: "app-1", Channel: ChannelDiscord, Priority: PriorityNormal, TemplateID: "tmpl-1"},
			nil,
		},
		{
			"Slack channel with empty UserID and TemplateID is valid",
			SendRequest{AppID: "app-1", Channel: ChannelSlack, Priority: PriorityNormal, TemplateID: "tmpl-1"},
			nil,
		},
		{
			"Teams channel with empty UserID and TemplateID is valid",
			SendRequest{AppID: "app-1", Channel: ChannelTeams, Priority: PriorityNormal, TemplateID: "tmpl-1"},
			nil,
		},
		{
			"Webhook channel with webhook_url in Data is valid",
			SendRequest{AppID: "app-1", Channel: ChannelWebhook, Priority: PriorityNormal, TemplateID: "tmpl-1", Data: map[string]interface{}{"webhook_url": "https://example.com/hook"}},
			nil,
		},
		{
			"Discord channel with no TemplateID, no webhook_url → ErrInvalidUserID",
			SendRequest{AppID: "app-1", Channel: ChannelDiscord, Priority: PriorityNormal, Title: "T", Body: "B"},
			ErrInvalidUserID,
		},
		{
			"Discord channel with webhook_target in Data is valid",
			SendRequest{AppID: "app-1", Channel: ChannelDiscord, Priority: PriorityNormal, Title: "T", Body: "B", Data: map[string]interface{}{"webhook_target": "my-hook"}},
			nil,
		},
		{
			"TopicID substitutes for UserID",
			SendRequest{AppID: "app-1", Channel: ChannelEmail, Priority: PriorityNormal, TemplateID: "tmpl-1", TopicID: "topic-1"},
			nil,
		},
		{
			"Invalid channel string",
			SendRequest{AppID: "app-1", UserID: "user-1", Channel: Channel("fax"), Priority: PriorityNormal, TemplateID: "tmpl-1"},
			ErrInvalidChannel,
		},
		{
			"Empty channel with no TemplateID",
			SendRequest{AppID: "app-1", UserID: "user-1", Priority: PriorityNormal, Title: "T", Body: "B"},
			ErrInvalidChannel,
		},
		{
			"Empty channel with TemplateID is valid (inferred later)",
			SendRequest{AppID: "app-1", UserID: "user-1", Priority: PriorityNormal, TemplateID: "tmpl-1"},
			nil,
		},
		{
			"Invalid priority",
			SendRequest{AppID: "app-1", UserID: "user-1", Channel: ChannelEmail, Priority: Priority("urgent"), TemplateID: "tmpl-1"},
			ErrInvalidPriority,
		},
		{
			"Empty priority defaults to normal",
			SendRequest{AppID: "app-1", UserID: "user-1", Channel: ChannelEmail, TemplateID: "tmpl-1"},
			nil,
		},
		{
			"Missing TemplateID and incomplete inline content",
			SendRequest{AppID: "app-1", UserID: "user-1", Channel: ChannelEmail, Priority: PriorityNormal, Title: "Title only"},
			ErrTemplateRequired,
		},
		{
			"Missing TemplateID with full inline content",
			SendRequest{AppID: "app-1", UserID: "user-1", Channel: ChannelEmail, Priority: PriorityNormal, Title: "T", Body: "B"},
			nil,
		},
		{
			"Twilio content_sid in Data bypasses TemplateID requirement",
			SendRequest{AppID: "app-1", UserID: "user-1", Channel: ChannelWhatsApp, Priority: PriorityNormal, Data: map[string]interface{}{"content_sid": "HXabc"}},
			nil,
		},
		{
			"ScheduledAt in past",
			SendRequest{AppID: "app-1", UserID: "user-1", Channel: ChannelEmail, Priority: PriorityNormal, TemplateID: "tmpl-1", ScheduledAt: &past},
			ErrInvalidScheduleTime,
		},
		{
			"ScheduledAt in future is valid",
			SendRequest{AppID: "app-1", UserID: "user-1", Channel: ChannelEmail, Priority: PriorityNormal, TemplateID: "tmpl-1", ScheduledAt: &future},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
