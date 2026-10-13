package usecases

import (
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
)

func appendBroadcastOutcome(report *notification.BroadcastResult, n *notification.Notification, outcome string) {
	if len(report.Outcomes) >= 1000 {
		report.OutcomesTruncated = true
		return
	}
	report.Outcomes = append(report.Outcomes, notification.BroadcastRecipientOutcome{UserID: n.UserID, NotificationID: n.NotificationID, Outcome: outcome})
}
func recordUnknownBroadcastBatch(report *notification.BroadcastResult, confirmed, batch, rest []*notification.Notification, outcome string) {
	if report == nil {
		return
	}
	report.Accepted = len(confirmed)
	report.Unknown = len(batch)
	report.NotAttempted = len(rest)
	report.Partial = true
	for _, n := range confirmed {
		appendBroadcastOutcome(report, n, "accepted")
	}
	for _, n := range batch {
		appendBroadcastOutcome(report, n, outcome)
	}
	for _, n := range rest {
		appendBroadcastOutcome(report, n, "not_attempted")
	}
}
func containsQueueNotification(items []queue.NotificationQueueItem, id string) bool {
	for _, item := range items {
		if item.NotificationID == id {
			return true
		}
	}
	return false
}
