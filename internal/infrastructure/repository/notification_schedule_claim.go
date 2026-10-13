package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Claim before publishing. A stale search result or competing scheduler cannot
// queue an already claimed/processed notification or overwrite delivery history.
func (r *NotificationRepository) ClaimScheduledNotification(ctx context.Context, id string) (bool, error) {
	source := `
if (ctx._source.status != 'pending' && ctx._source.status != 'snoozed') { ctx.op='noop'; return; }
if (ctx._source.metadata != null && (ctx._source.metadata.delivery_attempt_state == 'dispatching' || ctx._source.metadata.delivery_attempt_state == 'unknown')) { ctx.op='noop'; return; }
def now = ZonedDateTime.parse(params.now);
if (ctx._source.status == 'snoozed') {
 if (ctx._source.snoozed_until == null || ZonedDateTime.parse(ctx._source.snoozed_until).isAfter(now)) { ctx.op='noop'; return; }
} else if (ctx._source.scheduled_at != null && ZonedDateTime.parse(ctx._source.scheduled_at).isAfter(now)) { ctx.op='noop'; return; }
ctx._source.status='queued';
ctx._source.updated_at=params.now;
`
	body, err := json.Marshal(map[string]interface{}{"script": map[string]interface{}{"lang": "painless", "source": source, "params": map[string]interface{}{"now": time.Now().UTC().Format(time.RFC3339Nano)}}})
	if err != nil {
		return false, err
	}
	res, err := r.client.Update(r.indexName, id, strings.NewReader(string(body)), r.client.Update.WithContext(ctx), r.client.Update.WithRetryOnConflict(5), r.client.Update.WithRefresh("false"))
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	if res.StatusCode == 404 {
		return false, nil
	}
	if res.IsError() {
		return false, fmt.Errorf("scheduled notification claim failed: status %d", res.StatusCode)
	}
	var outcome struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&outcome); err != nil {
		return false, err
	}
	switch outcome.Result {
	case "updated":
		return true, nil
	case "noop":
		return false, nil
	default:
		return false, fmt.Errorf("unknown scheduled claim outcome")
	}
}
