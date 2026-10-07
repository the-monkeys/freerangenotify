package integration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

func TestDeliveryHistoryRealESClearsOldFailureFields(t *testing.T) {
	fixture := newCreditFixture(t)
	body, err := json.Marshal((&database.IndexTemplates{}).GetNotificationsTemplate())
	require.NoError(t, err)
	response, err := fixture.es.Indices.Create("notifications", fixture.es.Indices.Create.WithBody(strings.NewReader(string(body))))
	require.NoError(t, err)
	require.True(t, !response.IsError() || response.StatusCode == 400)
	response.Body.Close()
	ctx := context.Background()
	repo := repository.NewNotificationRepository(fixture.es, zap.NewNop())
	oldFailure := time.Now().Add(-time.Minute)
	id := fixture.tenant + "-history"
	notif := &notification.Notification{NotificationID: id, AppID: fixture.tenant, UserID: "fixture-user",
		Channel: notification.ChannelEmail, Status: notification.StatusFailed, ErrorMessage: "old failure", FailedAt: &oldFailure,
		Metadata: map[string]interface{}{"failure_code": "authentication", "credits_available": 700, "custom_marker": "preserve", "reservation_id": "prior-attempt"}}
	require.NoError(t, repo.Create(ctx, notif))
	t.Cleanup(func() {
		response, err := fixture.es.Delete("notifications", id)
		if err == nil {
			response.Body.Close()
		}
	})
	cleaner, ok := repo.(interface {
		ClearDeliveryFailure(context.Context, string) error
	})
	require.True(t, ok)
	require.NoError(t, cleaner.ClearDeliveryFailure(ctx, id))
	now := time.Now()
	notif.Status = notification.StatusSent
	notif.SentAt = &now
	notif.ErrorMessage = ""
	notif.FailedAt = nil
	notif.Metadata = map[string]interface{}{"custom_marker": "preserve", "provider_message_id": "fixture-success", "reservation_id": "successful-attempt", "delivery_reservation_id": "successful-attempt"}
	require.NoError(t, repo.Update(ctx, notif))
	reloaded, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, notification.StatusSent, reloaded.Status)
	require.Empty(t, reloaded.ErrorMessage)
	require.Nil(t, reloaded.FailedAt)
	require.NotContains(t, reloaded.Metadata, "failure_code")
	require.NotContains(t, reloaded.Metadata, "credits_available")
	require.Equal(t, "preserve", reloaded.Metadata["custom_marker"])
	require.Equal(t, "fixture-success", reloaded.Metadata["provider_message_id"])
	require.NotNil(t, reloaded.SentAt)
	require.Equal(t, "successful-attempt", reloaded.Metadata["reservation_id"])
	require.Equal(t, "successful-attempt", reloaded.Metadata["delivery_reservation_id"])
}
