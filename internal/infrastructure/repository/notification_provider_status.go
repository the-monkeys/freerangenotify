package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"strings"
	"time"
)

// ApplyProviderDeliveryStatus is an atomic, identity-bound transition. A late
// failure cannot overwrite a delivered/read notification.
func (r *NotificationRepository) ApplyProviderDeliveryStatus(ctx context.Context, id, appID, providerID string, status notification.Status, at time.Time) (bool, error) {
	source := `if (ctx._source.app_id != params.app || ctx._source.metadata == null || ctx._source.metadata.provider_message_id != params.message) {ctx.op='noop'; return;}
def ranks=['sent':1,'delivered':2,'read':3,'failed':0]; def previous=ctx._source.status;
if (previous==params.status || (params.status=='failed' && (previous=='delivered'||previous=='read')) || (params.status!='failed' && ranks.containsKey(previous) && ranks[params.status]<=ranks[previous])) {ctx.op='noop'; return;}
ctx._source.status=params.status;ctx._source.updated_at=params.now;
if(params.status=='sent'){ctx._source.sent_at=params.at;}if(params.status=='delivered'){ctx._source.delivered_at=params.at;}if(params.status=='read'){ctx._source.read_at=params.at;}if(params.status=='failed'){ctx._source.failed_at=params.at;}`
	body, _ := json.Marshal(map[string]interface{}{"script": map[string]interface{}{"source": source, "params": map[string]interface{}{"app": appID, "message": providerID, "status": string(status), "at": at.UTC().Format(time.RFC3339Nano), "now": time.Now().UTC().Format(time.RFC3339Nano)}}})
	res, err := r.client.Update(r.indexName, id, strings.NewReader(string(body)), r.client.Update.WithContext(ctx), r.client.Update.WithRetryOnConflict(5))
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return false, fmt.Errorf("provider status transition unavailable")
	}
	var out struct {
		Result string `json:"result"`
	}
	if err = json.NewDecoder(res.Body).Decode(&out); err != nil {
		return false, err
	}
	return out.Result == "updated", nil
}
