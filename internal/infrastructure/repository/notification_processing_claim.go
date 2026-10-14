package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"strings"
	"time"
)

func (r *NotificationRepository) ClaimNotificationProcessing(ctx context.Context, n *notification.Notification) (bool, error) {
	token, _ := n.Metadata["worker_claim"].(string)
	if token == "" {
		return false, fmt.Errorf("processing identity required")
	}
	source := `if(ctx._source.app_id!=params.app){throw new IllegalArgumentException('wrong processing owner');}
 if(ctx._source.status=='sent'||ctx._source.status=='delivered'||ctx._source.status=='read'||ctx._source.status=='cancelled'||ctx._source.status=='archived'){ctx.op='noop';return;}
 if(ctx._source.retry_count!=null&&ctx._source.retry_count!=params.attempt){ctx.op='noop';return;}
 if(ctx._source.metadata==null){ctx._source.metadata=[:];}
 def state=ctx._source.metadata.delivery_attempt_state;
 if(state=='dispatching'||state=='unknown'||state=='accepted'){ctx.op='noop';return;}
 if(ctx._source.metadata.worker_claim_until!=null && ctx._source.metadata.worker_claim_until>params.now){ctx.op='noop';return;}
 ctx._source.metadata.worker_claim=params.token;ctx._source.metadata.worker_claim_until=params.until;
 ctx._source.status='processing';ctx._source.updated_at=params.stamp;`
	now := time.Now().UTC()
	n.Metadata["worker_claim_until"] = now.Add(2 * time.Minute).UnixMilli()
	return r.runClaimScript(ctx, n.NotificationID, source, map[string]interface{}{"app": n.AppID, "attempt": n.RetryCount, "token": token, "now": now.UnixMilli(), "until": now.Add(2 * time.Minute).UnixMilli(), "stamp": now.Format(time.RFC3339Nano)})
}
func (r *NotificationRepository) runClaimScript(ctx context.Context, id, source string, params map[string]interface{}) (bool, error) {
	b, e := json.Marshal(map[string]interface{}{"script": map[string]interface{}{"lang": "painless", "source": source, "params": params}})
	if e != nil {
		return false, e
	}
	response, e := r.client.Update(r.indexName, id, strings.NewReader(string(b)), r.client.Update.WithContext(ctx), r.client.Update.WithRetryOnConflict(10))
	if e != nil {
		return false, e
	}
	defer response.Body.Close()
	if response.IsError() {
		return false, fmt.Errorf("processing transition unavailable (%d)", response.StatusCode)
	}
	var result struct {
		Result string `json:"result"`
	}
	if json.NewDecoder(response.Body).Decode(&result) != nil {
		return false, fmt.Errorf("processing transition unknown")
	}
	if result.Result == "noop" {
		return false, nil
	}
	if result.Result != "updated" {
		return false, fmt.Errorf("processing transition unknown")
	}
	return true, nil
}
func (r *NotificationRepository) updateWithProcessingClaim(ctx context.Context, id string, doc interface{}) error {
	token := notification.ProcessingClaim(ctx)
	if token == "" {
		return r.BaseRepository.Update(ctx, id, doc)
	}
	source := `if(ctx._source.metadata==null || ctx._source.metadata.worker_claim!=params.token){ctx.op='noop';return;}
 if(ctx._source.status=='delivered'||ctx._source.status=='read'||ctx._source.status=='cancelled'||ctx._source.status=='archived'){ctx.op='noop';return;}
 ctx._source.putAll(params.doc);
 if(ctx._source.status=='queued'||ctx._source.status=='failed'){ctx._source.metadata.worker_claim_until=0;}`
	changed, e := r.runClaimScript(ctx, id, source, map[string]interface{}{"token": token, "doc": doc})
	if e != nil {
		return e
	}
	if !changed {
		return notification.ErrProcessingClaimLost
	}
	return nil
}
