package bizbillingrepo

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

const NotificationOutboxIndex = "frn_biz_notification_outbox"

type NotificationOutboxRepo struct {
	Records *Store[bizbilling.NotificationDelivery]
}

func NewNotificationOutboxRepo(es *elasticsearch.Client, logger *zap.Logger) *NotificationOutboxRepo {
	return &NotificationOutboxRepo{Records: NewStore[bizbilling.NotificationDelivery](es, NotificationOutboxIndex, logger, repository.RefreshFalse)}
}

// Historical facts are unchanged. Only the new required checkpoint generates
// delivery work; upgrading must not resend a backlog of old invoices.
func (r *NotificationOutboxRepo) Materialize(ctx context.Context, event bizbilling.JournalEvent) error {
	if event.SchemaVersion < 5 {
		return nil
	}
	for _, intent := range event.Notifications {
		if intent.AppID != event.Scope.AppID || intent.UserID != event.Scope.UserID || intent.EventID != event.ID || intent.TemplateID == "" || intent.Channel == "" {
			return bizbilling.ErrReconciliationRequired
		}
		for key := range intent.Payload {
			if strings.Contains(strings.ToLower(key), "token") || strings.Contains(strings.ToLower(key), "secret") || key == "payment_link" || key == "portal_link" {
				return bizbilling.ErrReconciliationRequired
			}
		}
		id := bizbilling.NotificationIntentID(intent)
		row := &bizbilling.NotificationDelivery{ID: id, AppID: intent.AppID, Scope: event.Scope, Sequence: event.Sequence, Intent: intent, NotificationID: uuid.NewSHA1(uuid.NameSpaceOID, []byte("biz-notification:"+id)).String(), Status: "pending", NextAttemptAt: event.CreatedAt, CreatedAt: event.CreatedAt, UpdatedAt: event.CreatedAt}
		created, err := r.Records.CreateIfAbsent(ctx, id, row)
		if err != nil {
			return err
		}
		if !created {
			saved, err := r.Records.Get(ctx, id)
			if err != nil {
				return err
			}
			a, _ := json.Marshal(saved.Intent)
			b, _ := json.Marshal(intent)
			if saved.Scope != row.Scope || saved.Sequence != row.Sequence || string(a) != string(b) || saved.NotificationID != row.NotificationID {
				return bizbilling.ErrReconciliationRequired
			}
		}
	}
	return nil
}
