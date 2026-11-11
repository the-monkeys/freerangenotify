package repository

import (
	"context"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

// Dedicated pre-auth callback lookup is bound to four upstream identities. It
// does not relax the generic provider lookup's mandatory application scope.
// Only the server uses the result to retrieve a signing key; it is never returned
// to the unauthenticated caller. Source fallback supports pre-indexed records.
func (r *NotificationRepository) FindAdmittedChannelStatus(ctx context.Context, provider, account, sender, message string) (*notification.Notification, error) {
	if provider != "meta" || account == "" || sender == "" || message == "" {
		return nil, fmt.Errorf("complete callback sender identity required")
	}
	script := `def m=params._source['metadata'];if(m==null)return;def d=m['channel_delivery'];if(!(d instanceof Map)||d['provider']!=params.provider||d['account_id']!=params.account||d['sender_id']!=params.sender)return;def id=params._source['provider_message_id'];if(id==null||id=='')id=m['provider_message_id'];if(id instanceof String)emit(id);`
	for _, legacy := range []bool{false, true} {
		filters := []interface{}{map[string]interface{}{"term": map[string]interface{}{"channel": "whatsapp"}}, map[string]interface{}{"term": map[string]interface{}{"frn_admitted_status": message}}}
		if !legacy {
			filters = append(filters, map[string]interface{}{"bool": map[string]interface{}{"minimum_should_match": 1, "should": []interface{}{map[string]interface{}{"term": map[string]interface{}{"provider_message_id": message}}, map[string]interface{}{"term": map[string]interface{}{"provider_message_id.keyword": message}}, map[string]interface{}{"term": map[string]interface{}{"metadata.provider_message_id": message}}}}})
		}
		query := map[string]interface{}{"size": 2, "query": map[string]interface{}{"bool": map[string]interface{}{"filter": filters}}, "runtime_mappings": map[string]interface{}{"frn_admitted_status": map[string]interface{}{"type": "keyword", "script": map[string]interface{}{"source": script, "params": map[string]interface{}{"provider": provider, "account": account, "sender": sender}}}}}
		result, err := r.BaseRepository.Search(ctx, query)
		if err != nil {
			return nil, err
		}
		if len(result.Hits) > 1 {
			return nil, fmt.Errorf("ambiguous admitted provider identity")
		}
		if len(result.Hits) == 1 {
			var n notification.Notification
			if err := mapToStruct(result.Hits[0], &n); err != nil {
				return nil, err
			}
			return &n, nil
		}
	}
	return nil, nil
}
