package billingrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"go.uber.org/zap"
)

const notificationPreferencesIndex = "payment_notification_preferences"

// ESNotificationPreferencesRepo stores one document per tenant, keyed by tenant ID.
type ESNotificationPreferencesRepo struct {
	es     *elasticsearch.Client
	logger *zap.Logger
}

func NewESNotificationPreferencesRepo(es *elasticsearch.Client, logger *zap.Logger) *ESNotificationPreferencesRepo {
	return &ESNotificationPreferencesRepo{es: es, logger: logger}
}

func (r *ESNotificationPreferencesRepo) GetByTenantID(ctx context.Context, tenantID string) (*billing.PaymentNotificationPreferences, error) {
	if tenantID == "" {
		return nil, nil
	}

	res, err := r.es.Get(
		notificationPreferencesIndex,
		tenantID,
		r.es.Get.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("billingrepo: get notification preferences: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, nil
	}
	if res.IsError() {
		return nil, fmt.Errorf("billingrepo: get notification preferences error: %s", res.String())
	}

	var result struct {
		Source billing.PaymentNotificationPreferences `json:"_source"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("billingrepo: decode notification preferences: %w", err)
	}

	return &result.Source, nil
}

func (r *ESNotificationPreferencesRepo) Upsert(ctx context.Context, prefs *billing.PaymentNotificationPreferences) error {
	if prefs == nil || prefs.TenantID == "" {
		return fmt.Errorf("billingrepo: tenant_id is required")
	}
	prefs.UpdatedAt = time.Now().UTC()

	body, err := json.Marshal(prefs)
	if err != nil {
		return fmt.Errorf("billingrepo: marshal notification preferences: %w", err)
	}

	res, err := r.es.Index(
		notificationPreferencesIndex,
		strings.NewReader(string(body)),
		r.es.Index.WithDocumentID(prefs.TenantID),
		r.es.Index.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("billingrepo: upsert notification preferences: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("billingrepo: upsert notification preferences error: %s", res.String())
	}

	r.logger.Debug("Upserted payment notification preferences", zap.String("tenant_id", prefs.TenantID))
	return nil
}
