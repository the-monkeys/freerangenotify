package main

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"go.uber.org/zap"
)

func (p *NotificationProcessor) claimScheduledItems(ctx context.Context, items []queue.NotificationQueueItem) []queue.NotificationQueueItem {
	repo, ok := p.notifRepo.(interface {
		ClaimScheduledNotification(context.Context, string) (bool, error)
	})
	if !ok {
		p.logger.Error("Scheduled notification repository has no atomic claim capability")
		return nil
	}
	claimed := make([]queue.NotificationQueueItem, 0, len(items))
	for _, item := range items {
		accepted, err := repo.ClaimScheduledNotification(ctx, item.NotificationID)
		if err != nil {
			p.logger.Error("Scheduled claim outcome unknown; publication withheld", zap.String("notification_id", item.NotificationID), zap.Error(err))
			continue
		}
		if accepted {
			claimed = append(claimed, item)
		}
	}
	return claimed
}
