//go:build skip
// +build skip

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/metrics"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"go.uber.org/zap"
)

// setupE2ETest sets up the test environment with all dependencies
func setupE2ETest(t *testing.T) (
	context.Context,
	notification.Service,
	notification.Repository,
	user.Repository,
	queue.Queue,
	*metrics.NotificationMetrics,
	func(),
) {
	ctx := context.Background()

	// Setup Elasticsearch
	esClient, cleanup := database.SetupTestElasticsearch(t)

	// Create indices
	if err := database.CreateNotificationIndex(esClient); err != nil {
		t.Fatalf("Failed to create notification index: %v", err)
	}
	if err := database.CreateUserIndex(esClient); err != nil {
		t.Fatalf("Failed to create user index: %v", err)
	}

	// Create repositories
	notifRepo := database.NewNotificationRepository(esClient, "frn_test_notifications")
	userRepo := database.NewUserRepository(esClient, "frn_test_users")

	// Setup Redis for queue
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use separate DB for tests
	})

	// Clear any existing test data in Redis
	redisClient.FlushDB(ctx)

	logger, _ := zap.NewDevelopment()
	testQueue := queue.NewRedisQueue(redisClient, logger)

	// Create metrics
	testMetrics := metrics.NewNotificationMetrics()

	// Create notification service
	notifService := usecases.NewNotificationService(
		notifRepo,
		userRepo,
		testQueue,
		logger,
		usecases.NotificationServiceConfig{
			MaxRetries: 3,
		},
		testMetrics,
	)

	cleanupFunc := func() {
		redisClient.FlushDB(ctx)
		redisClient.Close()
		testQueue.Close()
		cleanup()
	}

	return ctx, notifService, notifRepo, userRepo, testQueue, testMetrics, cleanupFunc
}

// createTestUser creates a test user with given preferences
func createTestUser(t *testing.T, ctx context.Context, userRepo user.Repository, userID, appID string, preferences user.Preferences) *user.User {
	usr := &user.User{
		UserID:         userID,
		AppID:          appID,
		ExternalUserID: "ext_" + userID,
		Email:          userID + "@example.com",
		Timezone:       "UTC",
		Language:       "en",
		Preferences:    preferences,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err := userRepo.Create(ctx, usr)
	require.NoError(t, err)

	return usr
}

// TestE2E_SendToQueueToProcess tests the complete flow: send → queue → process → status update
func TestE2E_SendToQueueToProcess(t *testing.T) {
	ctx, notifService, notifRepo, userRepo, testQueue, _, cleanup := setupE2ETest(t)
	defer cleanup()

	// Create test user
	usr := createTestUser(t, ctx, userRepo, "user1", "app1", user.Preferences{
		EmailEnabled: true,
		PushEnabled:  true,
		SMSEnabled:   false,
	})

	// Send notification
	req := notification.SendRequest{
		AppID:    "app1",
		UserID:   usr.UserID,
		Channel:  notification.ChannelEmail,
		Priority: notification.PriorityNormal,
		Title:    "Test Notification",
		Body:     "This is a test notification",
		Data:     map[string]interface{}{"key": "value"},
	}

	notif, err := notifService.Send(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, notif)
	assert.Equal(t, notification.StatusQueued, notif.Status)

	// Verify notification is in queue
	queueItem, err := testQueue.Dequeue(ctx)
	require.NoError(t, err)
	require.NotNil(t, queueItem)
	assert.Equal(t, notif.NotificationID, queueItem.NotificationID)
	assert.Equal(t, notification.PriorityNormal, queueItem.Priority)

	// Simulate processing: update status to processing
	err = notifRepo.UpdateStatus(ctx, notif.NotificationID, notification.StatusProcessing)
	require.NoError(t, err)

	// Simulate successful send: update to sent
	err = notifRepo.UpdateStatus(ctx, notif.NotificationID, notification.StatusSent)
	require.NoError(t, err)

	// Verify final status
	retrieved, err := notifRepo.GetByID(ctx, notif.NotificationID)
	require.NoError(t, err)
	assert.Equal(t, notification.StatusSent, retrieved.Status)
	assert.NotNil(t, retrieved.SentAt)
}

// TestE2E_PriorityQueueHandling tests that high priority notifications are processed before normal
func TestE2E_PriorityQueueHandling(t *testing.T) {
	ctx, notifService, _, userRepo, testQueue, _, cleanup := setupE2ETest(t)
	defer cleanup()

	// Create test user
	usr := createTestUser(t, ctx, userRepo, "user2", "app1", user.Preferences{
		EmailEnabled: true,
	})

	// Send normal priority notification first
	normalReq := notification.SendRequest{
		AppID:    "app1",
		UserID:   usr.UserID,
		Channel:  notification.ChannelEmail,
		Priority: notification.PriorityNormal,
		Title:    "Normal Priority",
		Body:     "Normal notification",
	}
	normalNotif, err := notifService.Send(ctx, normalReq)
	require.NoError(t, err)

	// Send high priority notification after
	highReq := notification.SendRequest{
		AppID:    "app1",
		UserID:   usr.UserID,
		Channel:  notification.ChannelEmail,
		Priority: notification.PriorityHigh,
		Title:    "High Priority",
		Body:     "High priority notification",
	}
	highNotif, err := notifService.Send(ctx, highReq)
	require.NoError(t, err)

	// Dequeue - high priority should come first
	firstItem, err := testQueue.Dequeue(ctx)
	require.NoError(t, err)
	assert.Equal(t, highNotif.NotificationID, firstItem.NotificationID)

	// Second dequeue should be normal priority
	secondItem, err := testQueue.Dequeue(ctx)
	require.NoError(t, err)
	assert.Equal(t, normalNotif.NotificationID, secondItem.NotificationID)
}

// TestE2E_FailureRetryFlow tests the retry mechanism
func TestE2E_FailureRetryFlow(t *testing.T) {
	ctx, notifService, notifRepo, userRepo, testQueue, _, cleanup := setupE2ETest(t)
	defer cleanup()

	// Create test user
	usr := createTestUser(t, ctx, userRepo, "user3", "app1", user.Preferences{
		PushEnabled: true,
	})

	// Send notification
	req := notification.SendRequest{
		AppID:    "app1",
		UserID:   usr.UserID,
		Channel:  notification.ChannelPush,
		Priority: notification.PriorityNormal,
		Title:    "Retry Test",
		Body:     "Testing retry mechanism",
	}
	notif, err := notifService.Send(ctx, req)
	require.NoError(t, err)

	// Dequeue notification
	item, err := testQueue.Dequeue(ctx)
	require.NoError(t, err)

	// Simulate first failure
	err = notifRepo.IncrementRetryCount(ctx, notif.NotificationID, "simulated failure")
	require.NoError(t, err)

	// Re-enqueue for retry
	redisQueue, ok := testQueue.(*queue.RedisQueue)
	require.True(t, ok)

	err = redisQueue.EnqueueRetry(ctx, *item, 1*time.Second)
	require.NoError(t, err)

	// Wait for retry delay
	time.Sleep(2 * time.Second)

	// Get retryable items
	retryItems, err := redisQueue.GetRetryableItems(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, retryItems, 1)
	assert.Equal(t, notif.NotificationID, retryItems[0].NotificationID)

	// Verify retry count was incremented
	retrieved, err := notifRepo.GetByID(ctx, notif.NotificationID)
	require.NoError(t, err)
	assert.Equal(t, 1, retrieved.RetryCount)
}

// TestE2E_MaxRetriesDeadLetterQueue tests moving to DLQ after max retries
func TestE2E_MaxRetriesDeadLetterQueue(t *testing.T) {
	ctx, notifService, notifRepo, userRepo, testQueue, _, cleanup := setupE2ETest(t)
	defer cleanup()

	maxRetries := 3

	// Create test user
	usr := createTestUser(t, ctx, userRepo, "user4", "app1", user.Preferences{
		SMSEnabled: true,
	})

	// Send notification
	req := notification.SendRequest{
		AppID:    "app1",
		UserID:   usr.UserID,
		Channel:  notification.ChannelSMS,
		Priority: notification.PriorityNormal,
		Title:    "DLQ Test",
		Body:     "Testing dead letter queue",
	}
	notif, err := notifService.Send(ctx, req)
	require.NoError(t, err)

	// Dequeue notification
	item, err := testQueue.Dequeue(ctx)
	require.NoError(t, err)

	redisQueue, ok := testQueue.(*queue.RedisQueue)
	require.True(t, ok)

	// Simulate max retries
	for i := 0; i < maxRetries; i++ {
		err = notifRepo.IncrementRetryCount(ctx, notif.NotificationID, "failure "+string(rune(i)))
		require.NoError(t, err)
	}

	// Should now move to DLQ
	err = redisQueue.EnqueueDeadLetter(ctx, *item, "max retries exceeded")
	require.NoError(t, err)

	// Update status to failed
	err = notifRepo.UpdateStatus(ctx, notif.NotificationID, notification.StatusFailed)
	require.NoError(t, err)

	// Verify status
	retrieved, err := notifRepo.GetByID(ctx, notif.NotificationID)
	require.NoError(t, err)
	assert.Equal(t, notification.StatusFailed, retrieved.Status)
	assert.Equal(t, maxRetries, retrieved.RetryCount)
}

// TestE2E_UserPreferencesQuietHours tests that quiet hours are respected
func TestE2E_UserPreferencesQuietHours(t *testing.T) {
	ctx, notifService, _, userRepo, _, _, cleanup := setupE2ETest(t)
	defer cleanup()

	// Create user with quiet hours (current time should be in quiet hours for test)
	now := time.Now().UTC()
	startHour := now.Add(-1 * time.Hour).Format("15:04")
	endHour := now.Add(1 * time.Hour).Format("15:04")

	usr := createTestUser(t, ctx, userRepo, "user5", "app1", user.Preferences{
		EmailEnabled: true,
		QuietHours: user.QuietHours{
			Start: startHour,
			End:   endHour,
		},
	})

	// Try to send normal priority notification during quiet hours
	req := notification.SendRequest{
		AppID:    "app1",
		UserID:   usr.UserID,
		Channel:  notification.ChannelEmail,
		Priority: notification.PriorityNormal,
		Title:    "Quiet Hours Test",
		Body:     "Should be blocked",
	}

	_, err := notifService.Send(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "quiet hours")

	// Critical priority should still go through
	criticalReq := notification.SendRequest{
		AppID:    "app1",
		UserID:   usr.UserID,
		Channel:  notification.ChannelEmail,
		Priority: notification.PriorityCritical,
		Title:    "Critical Alert",
		Body:     "Should go through",
	}

	criticalNotif, err := notifService.Send(ctx, criticalReq)
	require.NoError(t, err)
	assert.NotNil(t, criticalNotif)
}

// TestE2E_ChannelDisabled tests that disabled channels are rejected
func TestE2E_ChannelDisabled(t *testing.T) {
	ctx, notifService, _, userRepo, _, _, cleanup := setupE2ETest(t)
	defer cleanup()

	// Create user with SMS disabled
	usr := createTestUser(t, ctx, userRepo, "user6", "app1", user.Preferences{
		EmailEnabled: true,
		PushEnabled:  true,
		SMSEnabled:   false,
	})

	// Try to send SMS notification
	req := notification.SendRequest{
		AppID:    "app1",
		UserID:   usr.UserID,
		Channel:  notification.ChannelSMS,
		Priority: notification.PriorityNormal,
		Title:    "SMS Test",
		Body:     "Should be rejected",
	}

	_, err := notifService.Send(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not enabled")
}

// TestE2E_ScheduledNotification tests scheduled notifications
func TestE2E_ScheduledNotification(t *testing.T) {
	ctx, notifService, notifRepo, userRepo, _, _, cleanup := setupE2ETest(t)
	defer cleanup()

	// Create test user
	usr := createTestUser(t, ctx, userRepo, "user7", "app1", user.Preferences{
		EmailEnabled: true,
	})

	// Schedule notification for 5 seconds in future
	scheduledTime := time.Now().Add(5 * time.Second)
	req := notification.SendRequest{
		AppID:       "app1",
		UserID:      usr.UserID,
		Channel:     notification.ChannelEmail,
		Priority:    notification.PriorityNormal,
		Title:       "Scheduled Test",
		Body:        "Scheduled notification",
		ScheduledAt: &scheduledTime,
	}

	notif, err := notifService.Send(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, notification.StatusPending, notif.Status)
	assert.True(t, notif.IsScheduled())

	// Should not be queued yet
	pendingNotifs, err := notifRepo.GetPending(ctx)
	require.NoError(t, err)
	assert.Len(t, pendingNotifs, 1)
	assert.Equal(t, notif.NotificationID, pendingNotifs[0].NotificationID)
}

// TestE2E_BulkSend tests sending notifications to multiple users
func TestE2E_BulkSend(t *testing.T) {
	ctx, notifService, _, userRepo, _, _, cleanup := setupE2ETest(t)
	defer cleanup()

	// Create multiple test users
	users := make([]*user.User, 3)
	for i := 0; i < 3; i++ {
		userID := "bulkuser" + string(rune('1'+i))
		users[i] = createTestUser(t, ctx, userRepo, userID, "app1", user.Preferences{
			PushEnabled: true,
		})
	}

	// Send bulk notification
	bulkReq := notification.BulkSendRequest{
		AppID:    "app1",
		UserIDs:  []string{users[0].UserID, users[1].UserID, users[2].UserID},
		Channel:  notification.ChannelPush,
		Priority: notification.PriorityNormal,
		Title:    "Bulk Test",
		Body:     "Bulk notification",
	}

	notifs, err := notifService.SendBulk(ctx, bulkReq)
	require.NoError(t, err)
	assert.Len(t, notifs, 3)

	// Verify all notifications were created
	for i, notif := range notifs {
		assert.Equal(t, users[i].UserID, notif.UserID)
		assert.Equal(t, notification.StatusQueued, notif.Status)
	}
}
