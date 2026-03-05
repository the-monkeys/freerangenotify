package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"go.uber.org/zap"
)

// AnalyticsHandler handles analytics-related HTTP requests
type AnalyticsHandler struct {
	notifRepo notification.Repository
	logger    *zap.Logger
}

// NewAnalyticsHandler creates a new AnalyticsHandler
func NewAnalyticsHandler(notifRepo notification.Repository, logger *zap.Logger) *AnalyticsHandler {
	return &AnalyticsHandler{
		notifRepo: notifRepo,
		logger:    logger,
	}
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

// GetSummary handles GET /v1/admin/analytics/summary?period=7d
func (h *AnalyticsHandler) GetSummary(c *fiber.Ctx) error {
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

	// Count by statuses
	totalSent, _ := h.notifRepo.Count(ctx, &notification.NotificationFilter{
		Status:   notification.StatusSent,
		FromDate: &fromDate,
		ToDate:   &now,
	})
	totalDelivered, _ := h.notifRepo.Count(ctx, &notification.NotificationFilter{
		Status:   notification.StatusDelivered,
		FromDate: &fromDate,
		ToDate:   &now,
	})
	totalFailed, _ := h.notifRepo.Count(ctx, &notification.NotificationFilter{
		Status:   notification.StatusFailed,
		FromDate: &fromDate,
		ToDate:   &now,
	})
	totalPending, _ := h.notifRepo.Count(ctx, &notification.NotificationFilter{
		Status:   notification.StatusPending,
		FromDate: &fromDate,
		ToDate:   &now,
	})
	totalRead, _ := h.notifRepo.Count(ctx, &notification.NotificationFilter{
		Status:   notification.StatusRead,
		FromDate: &fromDate,
		ToDate:   &now,
	})

	totalAll := totalSent + totalDelivered + totalFailed + totalPending + totalRead
	successCount := totalSent + totalDelivered + totalRead
	var successRate float64
	if totalAll > 0 {
		successRate = float64(successCount) / float64(totalAll) * 100
	}

	// Count by channel
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
			Channel:  ch,
			Status:   notification.StatusSent,
			FromDate: &fromDate,
			ToDate:   &now,
		})
		delivered, _ := h.notifRepo.Count(ctx, &notification.NotificationFilter{
			Channel:  ch,
			Status:   notification.StatusDelivered,
			FromDate: &fromDate,
			ToDate:   &now,
		})
		failed, _ := h.notifRepo.Count(ctx, &notification.NotificationFilter{
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

	// Daily breakdown for the period
	dailyBreakdown := make([]DailyStat, 0)
	daysInPeriod := int(now.Sub(fromDate).Hours() / 24)
	if daysInPeriod > 30 {
		daysInPeriod = 30 // Cap at 30 days for daily breakdown
	}
	for i := daysInPeriod - 1; i >= 0; i-- {
		dayStart := time.Date(now.Year(), now.Month(), now.Day()-i, 0, 0, 0, 0, now.Location())
		dayEnd := dayStart.Add(24 * time.Hour)
		count, _ := h.notifRepo.Count(ctx, &notification.NotificationFilter{
			FromDate: &dayStart,
			ToDate:   &dayEnd,
		})
		dailyBreakdown = append(dailyBreakdown, DailyStat{
			Date:  dayStart.Format("2006-01-02"),
			Count: count,
		})
	}

	return c.JSON(fiber.Map{
		"period":          period,
		"total_sent":      totalSent,
		"total_delivered":  totalDelivered,
		"total_failed":    totalFailed,
		"total_pending":   totalPending,
		"total_read":      totalRead,
		"total_all":       totalAll,
		"success_rate":    successRate,
		"by_channel":      byChannel,
		"daily_breakdown": dailyBreakdown,
	})
}
