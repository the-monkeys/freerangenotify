package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"go.uber.org/zap"
)

// AnalyticsHandler handles analytics-related HTTP requests
type AnalyticsHandler struct {
	notifRepo    notification.Repository
	userRepo     user.Repository
	templateRepo template.Repository
	workflowRepo workflow.Repository // optional, nil when workflows disabled
	appRepo      application.Repository
	logger       *zap.Logger
}

// NewAnalyticsHandler creates a new AnalyticsHandler
func NewAnalyticsHandler(
	notifRepo notification.Repository,
	userRepo user.Repository,
	templateRepo template.Repository,
	workflowRepo workflow.Repository,
	appRepo application.Repository,
	logger *zap.Logger,
) *AnalyticsHandler {
	return &AnalyticsHandler{
		notifRepo:    notifRepo,
		userRepo:     userRepo,
		templateRepo: templateRepo,
		workflowRepo: workflowRepo,
		appRepo:      appRepo,
		logger:       logger,
	}
}

// SetWorkflowRepo sets the workflow repository (called after feature-gate check).
func (h *AnalyticsHandler) SetWorkflowRepo(repo workflow.Repository) {
	h.workflowRepo = repo
}

// ChannelAnalytics holds per-channel stats
type ChannelAnalytics struct {
	Channel   string  `json:"channel"`
	Sent      int64   `json:"sent"`
	Delivered int64   `json:"delivered"`
	Failed    int64   `json:"failed"`
	Total     int64   `json:"total"`
	Rate      float64 `json:"success_rate"`
}

// DailyStat holds per-day stats
type DailyStat struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// getAdminAppIDs returns the list of app IDs owned by the authenticated admin user.
func (h *AnalyticsHandler) getAdminAppIDs(c *fiber.Ctx) ([]string, error) {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "authentication required")
	}

	apps, err := h.appRepo.List(c.Context(), application.ApplicationFilter{AdminUserID: userID})
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "failed to fetch admin apps")
	}

	appIDs := make([]string, len(apps))
	for i, app := range apps {
		appIDs[i] = app.AppID
	}
	return appIDs, nil
}

// GetSummary handles GET /v1/admin/analytics/summary?period=7d
// @Summary Get analytics summary
// @Description Retrieve aggregated analytics including notification counts, success rates, channel breakdown, and daily trends
// @Tags Analytics
// @Produce json
// @Param period query string false "Time period (1d, 7d, 30d, 90d)" default(7d)
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security BearerAuth
// @Router /v1/admin/analytics/summary [get]
func (h *AnalyticsHandler) GetSummary(c *fiber.Ctx) error {
	// Scope all data to the authenticated admin's applications
	appIDs, err := h.getAdminAppIDs(c)
	if err != nil {
		return err
	}

	// If admin has no apps, return empty analytics
	if len(appIDs) == 0 {
		return c.JSON(fiber.Map{
			"period":           c.Query("period", "7d"),
			"total_sent":       0,
			"total_delivered":  0,
			"total_failed":     0,
			"total_pending":    0,
			"total_read":       0,
			"total_queued":     0,
			"total_processing": 0,
			"total_all":        0,
			"success_rate":     float64(0),
			"total_users":      0,
			"total_templates":  0,
			"total_workflows":  0,
			"by_channel":       []ChannelAnalytics{},
			"daily_breakdown":  []DailyStat{},
		})
	}

	period := c.Query("period", "7d")

	// Parse period to duration
	now := time.Now()
	var fromDate time.Time
	switch period {
	case "1d":
		fromDate = now.Add(-24 * time.Hour)
	case "7d":
		fromDate = now.Add(-7 * 24 * time.Hour)
	case "30d":
		fromDate = now.Add(-30 * 24 * time.Hour)
	case "90d":
		fromDate = now.Add(-90 * 24 * time.Hour)
	default:
		fromDate = now.Add(-7 * 24 * time.Hour)
	}

	ctx := c.Context()

	// Count by statuses — scoped to admin's apps
	totalSent, _ := h.notifRepo.Count(ctx, &notification.NotificationFilter{
		AppIDs:   appIDs,
		Status:   notification.StatusSent,
		FromDate: &fromDate,
		ToDate:   &now,
	})
	totalDelivered, _ := h.notifRepo.Count(ctx, &notification.NotificationFilter{
		AppIDs:   appIDs,
		Status:   notification.StatusDelivered,
		FromDate: &fromDate,
		ToDate:   &now,
	})
	totalFailed, _ := h.notifRepo.Count(ctx, &notification.NotificationFilter{
		AppIDs:   appIDs,
		Status:   notification.StatusFailed,
		FromDate: &fromDate,
		ToDate:   &now,
	})
	totalPending, _ := h.notifRepo.Count(ctx, &notification.NotificationFilter{
		AppIDs:   appIDs,
		Status:   notification.StatusPending,
		FromDate: &fromDate,
		ToDate:   &now,
	})
	totalRead, _ := h.notifRepo.Count(ctx, &notification.NotificationFilter{
		AppIDs:   appIDs,
		Status:   notification.StatusRead,
		FromDate: &fromDate,
		ToDate:   &now,
	})
	totalQueued, _ := h.notifRepo.Count(ctx, &notification.NotificationFilter{
		AppIDs:   appIDs,
		Status:   notification.StatusQueued,
		FromDate: &fromDate,
		ToDate:   &now,
	})
	totalProcessing, _ := h.notifRepo.Count(ctx, &notification.NotificationFilter{
		AppIDs:   appIDs,
		Status:   notification.StatusProcessing,
		FromDate: &fromDate,
		ToDate:   &now,
	})

	totalAll := totalSent + totalDelivered + totalFailed + totalPending + totalRead + totalQueued + totalProcessing
	successCount := totalSent + totalDelivered + totalRead
	var successRate float64
	if totalAll > 0 {
		successRate = float64(successCount) / float64(totalAll) * 100
	}

	// Entity counts — scoped to admin's apps
	totalUsers, _ := h.userRepo.Count(ctx, user.UserFilter{AppIDs: appIDs})
	totalTemplates, _ := h.templateRepo.CountByFilter(ctx, template.Filter{AppIDs: appIDs})
	var totalWorkflows int64
	if h.workflowRepo != nil {
		totalWorkflows, _ = h.workflowRepo.CountByAppIDs(ctx, appIDs)
	}

	// Count by channel — scoped to admin's apps
	channels := []notification.Channel{
		notification.ChannelEmail,
		notification.ChannelPush,
		notification.ChannelSMS,
		notification.ChannelWebhook,
		notification.ChannelInApp,
		notification.ChannelSSE,
	}

	byChannel := make([]ChannelAnalytics, 0)
	for _, ch := range channels {
		sent, _ := h.notifRepo.Count(ctx, &notification.NotificationFilter{
			AppIDs:   appIDs,
			Channel:  ch,
			Status:   notification.StatusSent,
			FromDate: &fromDate,
			ToDate:   &now,
		})
		delivered, _ := h.notifRepo.Count(ctx, &notification.NotificationFilter{
			AppIDs:   appIDs,
			Channel:  ch,
			Status:   notification.StatusDelivered,
			FromDate: &fromDate,
			ToDate:   &now,
		})
		failed, _ := h.notifRepo.Count(ctx, &notification.NotificationFilter{
			AppIDs:   appIDs,
			Channel:  ch,
			Status:   notification.StatusFailed,
			FromDate: &fromDate,
			ToDate:   &now,
		})

		total := sent + delivered + failed
		if total == 0 {
			continue
		}

		rate := float64(0)
		if total > 0 {
			rate = float64(sent+delivered) / float64(total) * 100
		}

		byChannel = append(byChannel, ChannelAnalytics{
			Channel:   string(ch),
			Sent:      sent,
			Delivered: delivered,
			Failed:    failed,
			Total:     total,
			Rate:      rate,
		})
	}

	// Daily breakdown — scoped to admin's apps
	dailyBreakdown := make([]DailyStat, 0)
	daysInPeriod := int(now.Sub(fromDate).Hours() / 24)
	if daysInPeriod > 30 {
		daysInPeriod = 30 // Cap at 30 days for daily breakdown
	}
	for i := daysInPeriod - 1; i >= 0; i-- {
		dayStart := time.Date(now.Year(), now.Month(), now.Day()-i, 0, 0, 0, 0, now.Location())
		dayEnd := dayStart.Add(24 * time.Hour)
		count, _ := h.notifRepo.Count(ctx, &notification.NotificationFilter{
			AppIDs:   appIDs,
			FromDate: &dayStart,
			ToDate:   &dayEnd,
		})
		dailyBreakdown = append(dailyBreakdown, DailyStat{
			Date:  dayStart.Format("2006-01-02"),
			Count: count,
		})
	}

	return c.JSON(fiber.Map{
		"period":           period,
		"total_sent":       totalSent,
		"total_delivered":  totalDelivered,
		"total_failed":     totalFailed,
		"total_pending":    totalPending,
		"total_read":       totalRead,
		"total_queued":     totalQueued,
		"total_processing": totalProcessing,
		"total_all":        totalAll,
		"success_rate":     successRate,
		"total_users":      totalUsers,
		"total_templates":  totalTemplates,
		"total_workflows":  totalWorkflows,
		"by_channel":       byChannel,
		"daily_breakdown":  dailyBreakdown,
	})
}
