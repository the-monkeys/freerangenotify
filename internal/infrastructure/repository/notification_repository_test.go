package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

func setupTestNotificationRepo(t *testing.T) (*repository.NotificationRepository, func()) {
	client, cleanup := database.SetupTestElasticsearch(t)

	// Create notification index
	err := database.CreateNotificationIndex(client)
	require.NoError(t, err)

	// Wait for index to be ready
	time.Sleep(2 * time.Second)

	// Create test repository with test index name
	logger, _ := zap.NewDevelopment()
	testRepo := &repository.NotificationRepository{
		BaseRepository: repository.NewBaseRepository(client, "frn_test_notifications", logger),
	}

	return testRepo, cleanup
}

func createTestNotification(appID, userID string) *notification.Notification {
	now := time.Now()
	return &notification.Notification{
		NotificationID: "test-notif-" + time.Now().Format("20060102150405"),
		AppID:          appID,
		UserID:         userID,
		Channel:        notification.ChannelPush,
		Priority:       notification.PriorityNormal,
		Status:         notification.StatusPending,
		Content: notification.Content{
			Title: "Test Notification",
			Body:  "This is a test notification",
			Data:  map[string]interface{}{"key": "value"},
		},
		RetryCount: 0,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func TestNotificationRepository_Create(t *testing.T) {
	repo, cleanup := setupTestNotificationRepo(t)
	defer cleanup()

	ctx := context.Background()
	notif := createTestNotification("app-1", "user-1")

	err := repo.Create(ctx, notif)
	assert.NoError(t, err)
	assert.NotEmpty(t, notif.NotificationID)

	// Verify timestamps set
	assert.False(t, notif.CreatedAt.IsZero())
	assert.False(t, notif.UpdatedAt.IsZero())
}

func TestNotificationRepository_GetByID(t *testing.T) {
	repo, cleanup := setupTestNotificationRepo(t)
	defer cleanup()

	ctx := context.Background()
	notif := createTestNotification("app-1", "user-1")

	err := repo.Create(ctx, notif)
	require.NoError(t, err)

	// Wait for Elasticsearch to index
	time.Sleep(1 * time.Second)

	retrieved, err := repo.GetByID(ctx, notif.NotificationID)
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, notif.NotificationID, retrieved.NotificationID)
	assert.Equal(t, notif.Content.Title, retrieved.Content.Title)
	assert.Equal(t, notif.Content.Body, retrieved.Content.Body)
}

func TestNotificationRepository_GetByID_NotFound(t *testing.T) {
	repo, cleanup := setupTestNotificationRepo(t)
	defer cleanup()

	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nonexistent-id")
	assert.Error(t, err)
	assert.Equal(t, notification.ErrNotificationNotFound, err)
}

func TestNotificationRepository_Update(t *testing.T) {
	repo, cleanup := setupTestNotificationRepo(t)
	defer cleanup()

	ctx := context.Background()
	notif := createTestNotification("app-1", "user-1")

	err := repo.Create(ctx, notif)
	require.NoError(t, err)

	// Update notification
	notif.Content.Title = "Updated Title"
	notif.Content.Body = "Updated Body"

	err = repo.Update(ctx, notif)
	assert.NoError(t, err)

	// Wait for Elasticsearch to index
	time.Sleep(1 * time.Second)

	// Verify update
	retrieved, err := repo.GetByID(ctx, notif.NotificationID)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Title", retrieved.Content.Title)
	assert.Equal(t, "Updated Body", retrieved.Content.Body)
}

func TestNotificationRepository_UpdateStatus(t *testing.T) {
	repo, cleanup := setupTestNotificationRepo(t)
	defer cleanup()

	ctx := context.Background()
	notif := createTestNotification("app-1", "user-1")

	err := repo.Create(ctx, notif)
	require.NoError(t, err)

	tests := []struct {
		name           string
		status         notification.Status
		checkTimestamp string
	}{
		{"Update to Sent", notification.StatusSent, "sent_at"},
		{"Update to Delivered", notification.StatusDelivered, "delivered_at"},
		{"Update to Read", notification.StatusRead, "read_at"},
		{"Update to Failed", notification.StatusFailed, "failed_at"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notif := createTestNotification("app-1", "user-1")
			err := repo.Create(ctx, notif)
			require.NoError(t, err)

			err = repo.UpdateStatus(ctx, notif.NotificationID, tt.status)
			assert.NoError(t, err)

			// Wait for Elasticsearch to index
			time.Sleep(1 * time.Second)

			retrieved, err := repo.GetByID(ctx, notif.NotificationID)
			assert.NoError(t, err)
			assert.Equal(t, tt.status, retrieved.Status)

			// Check appropriate timestamp is set
			switch tt.status {
			case notification.StatusSent:
				assert.NotNil(t, retrieved.SentAt)
			case notification.StatusDelivered:
				assert.NotNil(t, retrieved.DeliveredAt)
			case notification.StatusRead:
				assert.NotNil(t, retrieved.ReadAt)
			case notification.StatusFailed:
				assert.NotNil(t, retrieved.FailedAt)
			}
		})
	}
}

func TestNotificationRepository_UpdateStatus_WithErrorMessage(t *testing.T) {
	repo, cleanup := setupTestNotificationRepo(t)
	defer cleanup()

	ctx := context.Background()
	notif := createTestNotification("app-1", "user-1")

	err := repo.Create(ctx, notif)
	require.NoError(t, err)

	errorMsg := "Failed to send: connection timeout"
	err = repo.UpdateStatus(ctx, notif.NotificationID, notification.StatusFailed)
	assert.NoError(t, err)

	// Wait for Elasticsearch to index
	time.Sleep(1 * time.Second)

	retrieved, err := repo.GetByID(ctx, notif.NotificationID)
	assert.NoError(t, err)
	assert.Equal(t, notification.StatusFailed, retrieved.Status)
	assert.Equal(t, errorMsg, retrieved.ErrorMessage)
	assert.NotNil(t, retrieved.FailedAt)
}

func TestNotificationRepository_Delete(t *testing.T) {
	repo, cleanup := setupTestNotificationRepo(t)
	defer cleanup()

	ctx := context.Background()
	notif := createTestNotification("app-1", "user-1")

	err := repo.Create(ctx, notif)
	require.NoError(t, err)

	// Wait for Elasticsearch to index
	time.Sleep(1 * time.Second)

	err = repo.Delete(ctx, notif.NotificationID)
	assert.NoError(t, err)

	// Wait for deletion to propagate
	time.Sleep(1 * time.Second)

	_, err = repo.GetByID(ctx, notif.NotificationID)
	assert.Error(t, err)
	assert.Equal(t, notification.ErrNotificationNotFound, err)
}

func TestNotificationRepository_List(t *testing.T) {
	repo, cleanup := setupTestNotificationRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple notifications
	for i := 0; i < 5; i++ {
		notif := createTestNotification("app-1", "user-1")
		notif.NotificationID = notif.NotificationID + string(rune('A'+i))
		err := repo.Create(ctx, notif)
		require.NoError(t, err)
	}

	// Wait for Elasticsearch to index
	time.Sleep(2 * time.Second)

	filter := notification.DefaultFilter()
	filter.AppID = "app-1"
	filter.PageSize = 10

	notifications, err := repo.List(ctx, &filter)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(notifications), 5)
}

func TestNotificationRepository_List_WithFilters(t *testing.T) {
	repo, cleanup := setupTestNotificationRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create notifications with different statuses
	statuses := []notification.Status{
		notification.StatusPending,
		notification.StatusQueued,
		notification.StatusSent,
	}

	for i, status := range statuses {
		notif := createTestNotification("app-1", "user-1")
		notif.NotificationID = notif.NotificationID + string(rune('A'+i))
		notif.Status = status
		err := repo.Create(ctx, notif)
		require.NoError(t, err)
	}

	// Wait for Elasticsearch to index
	time.Sleep(2 * time.Second)

	// Filter by status
	filter := notification.DefaultFilter()
	filter.AppID = "app-1"
	filter.Status = notification.StatusPending
	filter.PageSize = 10

	notifications, err := repo.List(ctx, &filter)
	assert.NoError(t, err)
	assert.Greater(t, len(notifications), 0)

	// Verify all returned notifications have pending status
	for _, n := range notifications {
		assert.Equal(t, notification.StatusPending, n.Status)
	}
}

func TestNotificationRepository_List_Pagination(t *testing.T) {
	repo, cleanup := setupTestNotificationRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create 15 notifications
	for i := 0; i < 15; i++ {
		notif := createTestNotification("app-1", "user-1")
		notif.NotificationID = notif.NotificationID + string(rune('A'+i))
		err := repo.Create(ctx, notif)
		require.NoError(t, err)
	}

	// Wait for Elasticsearch to index
	time.Sleep(2 * time.Second)

	// Get first page
	filter := notification.DefaultFilter()
	filter.AppID = "app-1"
	filter.Page = 1
	filter.PageSize = 5

	page1, err := repo.List(ctx, &filter)
	assert.NoError(t, err)
	assert.Len(t, page1, 5)

	// Get second page
	filter.Page = 2
	page2, err := repo.List(ctx, &filter)
	assert.NoError(t, err)
	assert.Len(t, page2, 5)

	// Verify pages are different
	assert.NotEqual(t, page1[0].NotificationID, page2[0].NotificationID)
}

func TestNotificationRepository_Count(t *testing.T) {
	repo, cleanup := setupTestNotificationRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create notifications
	for i := 0; i < 7; i++ {
		notif := createTestNotification("app-1", "user-1")
		notif.NotificationID = notif.NotificationID + string(rune('A'+i))
		err := repo.Create(ctx, notif)
		require.NoError(t, err)
	}

	// Wait for Elasticsearch to index
	time.Sleep(2 * time.Second)

	filter := notification.DefaultFilter()
	filter.AppID = "app-1"

	count, err := repo.Count(ctx, &filter)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(7))
}

func TestNotificationRepository_BulkUpdateStatus(t *testing.T) {
	repo, cleanup := setupTestNotificationRepo(t)
	defer cleanup()

	ctx := context.Background()

	var ids []string
	for i := 0; i < 3; i++ {
		notif := createTestNotification("app-1", "user-1")
		notif.NotificationID = notif.NotificationID + string(rune('A'+i))
		err := repo.Create(ctx, notif)
		require.NoError(t, err)
		ids = append(ids, notif.NotificationID)
	}

	// Wait for Elasticsearch to index
	time.Sleep(1 * time.Second)

	err := repo.BulkUpdateStatus(ctx, ids, notification.StatusQueued)
	assert.NoError(t, err)

	// Wait for updates to propagate
	time.Sleep(1 * time.Second)

	// Verify all notifications updated
	for _, id := range ids {
		retrieved, err := repo.GetByID(ctx, id)
		assert.NoError(t, err)
		assert.Equal(t, notification.StatusQueued, retrieved.Status)
	}
}

func TestNotificationRepository_GetPending(t *testing.T) {
	repo, cleanup := setupTestNotificationRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create pending notification scheduled in the past
	pastTime := time.Now().Add(-5 * time.Minute)
	notif := createTestNotification("app-1", "user-1")
	notif.Status = notification.StatusPending
	notif.ScheduledAt = &pastTime
	err := repo.Create(ctx, notif)
	require.NoError(t, err)

	// Create pending notification scheduled in the future
	futureTime := time.Now().Add(5 * time.Minute)
	notif2 := createTestNotification("app-1", "user-2")
	notif2.NotificationID = notif2.NotificationID + "B"
	notif2.Status = notification.StatusPending
	notif2.ScheduledAt = &futureTime
	err = repo.Create(ctx, notif2)
	require.NoError(t, err)

	// Create pending notification with no schedule (send immediately)
	notif3 := createTestNotification("app-1", "user-3")
	notif3.NotificationID = notif3.NotificationID + "C"
	notif3.Status = notification.StatusPending
	err = repo.Create(ctx, notif3)
	require.NoError(t, err)

	// Wait for Elasticsearch to index
	time.Sleep(2 * time.Second)

	pending, err := repo.GetPending(ctx)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(pending), 2) // Should get notif1 and notif3, not notif2

	// Verify none are scheduled in the future
	for _, n := range pending {
		if n.ScheduledAt != nil {
			assert.True(t, n.ScheduledAt.Before(time.Now()) || n.ScheduledAt.Equal(time.Now()))
		}
	}
}

func TestNotificationRepository_GetRetryable(t *testing.T) {
	repo, cleanup := setupTestNotificationRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create failed notification with retry count < max
	notif1 := createTestNotification("app-1", "user-1")
	notif1.Status = notification.StatusFailed
	notif1.RetryCount = 1
	err := repo.Create(ctx, notif1)
	require.NoError(t, err)

	// Create failed notification with retry count >= max
	notif2 := createTestNotification("app-1", "user-2")
	notif2.NotificationID = notif2.NotificationID + "B"
	notif2.Status = notification.StatusFailed
	notif2.RetryCount = 5
	err = repo.Create(ctx, notif2)
	require.NoError(t, err)

	// Wait for Elasticsearch to index
	time.Sleep(2 * time.Second)

	maxRetries := 3
	retryable, err := repo.GetRetryable(ctx, maxRetries)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(retryable), 1) // Should get notif1, not notif2

	// Verify all have retry count < max
	for _, n := range retryable {
		assert.Less(t, n.RetryCount, maxRetries)
		assert.Equal(t, notification.StatusFailed, n.Status)
	}
}

func TestNotificationRepository_IncrementRetryCount(t *testing.T) {
	repo, cleanup := setupTestNotificationRepo(t)
	defer cleanup()

	ctx := context.Background()
	notif := createTestNotification("app-1", "user-1")

	err := repo.Create(ctx, notif)
	require.NoError(t, err)

	initialRetryCount := notif.RetryCount
	errorMsg := "Retry attempt failed"

	err = repo.IncrementRetryCount(ctx, notif.NotificationID, errorMsg)
	assert.NoError(t, err)

	// Wait for Elasticsearch to index
	time.Sleep(1 * time.Second)

	retrieved, err := repo.GetByID(ctx, notif.NotificationID)
	assert.NoError(t, err)
	assert.Equal(t, initialRetryCount+1, retrieved.RetryCount)
	assert.Equal(t, errorMsg, retrieved.ErrorMessage)
}
