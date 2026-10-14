package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"strings"
)

func (r *NotificationRepository) CreateIfAbsent(ctx context.Context, n *notification.Notification) (bool, error) {
	b, err := json.Marshal(n)
	if err != nil {
		return false, err
	}
	res, err := r.client.Index(r.indexName, strings.NewReader(string(b)), r.client.Index.WithDocumentID(n.NotificationID), r.client.Index.WithOpType("create"), r.client.Index.WithContext(ctx), r.client.Index.WithRefresh("wait_for"))
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	if res.StatusCode == 409 {
		return false, nil
	}
	if res.IsError() {
		return false, fmt.Errorf("notification creation unavailable (%d)", res.StatusCode)
	}
	return true, nil
}
func (r *WhatsAppRepository) StoreOutboundMessage(ctx context.Context, m *whatsapp.InboundMessage) error {
	body, err := json.Marshal(map[string]interface{}{"scripted_upsert": true, "upsert": m, "script": map[string]interface{}{"lang": "painless", "source": `
 if (ctx.op == 'create') {return;}
 if (ctx._source.app_id != params.message.app_id || ctx._source.notification_id != params.message.notification_id || ctx._source.account_id != params.message.account_id || ctx._source.sender_id != params.message.sender_id) {throw new IllegalArgumentException('outbound owner mismatch');}
 if (params.message.meta_message_id != null && params.message.meta_message_id != '') {ctx._source.meta_message_id=params.message.meta_message_id;}
 def rank=['pending':0,'queued':1,'processing':2,'failed':3,'sent':3,'delivered':4,'read':5,'cancelled':3,'archived':6];
 if(ctx._source.delivery_state=='failed' && (params.message.delivery_state=='queued'||params.message.delivery_state=='processing')){return;}
 def old=rank[ctx._source.delivery_state];def next=rank[params.message.delivery_state];
 if (next != null && (old == null || next >= old)) {ctx._source.delivery_state=params.message.delivery_state;ctx._source.timestamp=params.message.timestamp;}
 `, "params": map[string]interface{}{"message": m}}})
	if err != nil {
		return err
	}
	res, err := r.client.Update("whatsapp_messages", m.ID, strings.NewReader(string(body)), r.client.Update.WithContext(ctx), r.client.Update.WithRetryOnConflict(5), r.client.Update.WithRefresh("wait_for"))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("outbound conversation update unavailable (%d)", res.StatusCode)
	}
	return nil
}
