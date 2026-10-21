package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"strings"
	"time"
)

// The financial receipt alone cannot prevent two queue deliveries from sending.
// Claim the irreversible provider boundary in one operation on the notification.
func (r *NotificationRepository) ClaimChannelDeliveryAttempt(ctx context.Context, n *notification.Notification) (bool, error) {
	if n == nil || n.NotificationID == "" {
		return false, fmt.Errorf("delivery identity required")
	}
	body, err := json.Marshal(map[string]interface{}{"script": map[string]interface{}{"lang": "painless", "source": `
 if (ctx._source.app_id != params.app) {throw new IllegalArgumentException('wrong delivery owner');}
 if (ctx._source.status == 'sent' || ctx._source.status == 'delivered' || ctx._source.status == 'read' || ctx._source.status == 'cancelled') {ctx.op='noop';return;}
 if (ctx._source.metadata == null) {ctx._source.metadata=[:];}
 if (params.metadata.worker_claim != null && ctx._source.metadata.worker_claim != params.metadata.worker_claim) {ctx.op='noop';return;}
 def state=ctx._source.metadata.delivery_attempt_state;
 if (state == 'dispatching' || state == 'unknown' || state == 'accepted') {ctx.op='noop';return;}
 ctx._source.metadata.putAll(params.metadata);ctx._source.metadata.delivery_attempt_state='dispatching';
 ctx._source.content=params.content;ctx._source.status='processing';ctx._source.updated_at=params.now;`, "params": map[string]interface{}{"app": n.AppID, "metadata": n.Metadata, "content": n.Content, "now": time.Now().UTC().Format(time.RFC3339Nano)}}})
	if err != nil {
		return false, err
	}
	res, err := r.client.Update(r.indexName, n.NotificationID, strings.NewReader(string(body)), r.client.Update.WithContext(ctx), r.client.Update.WithRetryOnConflict(10))
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return false, fmt.Errorf("delivery boundary claim unavailable (%d)", res.StatusCode)
	}
	var out struct {
		Result string `json:"result"`
	}
	if json.NewDecoder(res.Body).Decode(&out) != nil {
		return false, fmt.Errorf("delivery claim response unknown")
	}
	if out.Result == "noop" {
		return false, nil
	}
	if out.Result != "updated" {
		return false, fmt.Errorf("delivery claim outcome unknown")
	}
	return true, nil
}
