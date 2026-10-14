package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"go.uber.org/zap"
)

// WhatsAppRepository implements whatsapp.Repository using Elasticsearch.
type WhatsAppRepository struct {
	whatsapp.EventRepository
	messages *BaseRepository
	client   *elasticsearch.Client
	logger   *zap.Logger
}

func inboxEnvironmentFilter(ctx context.Context) map[string]interface{} {
	env := channelconnection.EnvironmentID(ctx)
	choices := []interface{}{map[string]interface{}{"term": map[string]interface{}{"environment_id": env}}, map[string]interface{}{"term": map[string]interface{}{"environment_id.keyword": env}}}
	if env == "default" {
		choices = append(choices, map[string]interface{}{"bool": map[string]interface{}{"must_not": map[string]interface{}{"exists": map[string]interface{}{"field": "environment_id"}}}})
	}
	return map[string]interface{}{"bool": map[string]interface{}{"should": choices, "minimum_should_match": 1}}
}

// NewWhatsAppRepository creates a new WhatsApp message repository.
func NewWhatsAppRepository(client *elasticsearch.Client, logger *zap.Logger) whatsapp.Repository {
	return &WhatsAppRepository{
		messages:        NewBaseRepository(client, "whatsapp_messages", logger, RefreshWaitFor),
		EventRepository: NewChannelEventRepository(client),
		client:          client,
		logger:          logger,
	}
}

func (r *WhatsAppRepository) StoreMessage(ctx context.Context, msg *whatsapp.InboundMessage) error {
	raw, _ := json.Marshal(msg)
	req := esapi.CreateRequest{Index: "whatsapp_messages", DocumentID: msg.ID, Body: strings.NewReader(string(raw)), Refresh: "wait_for"}
	res, err := req.Do(ctx, r.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == 409 {
		return nil
	}
	if res.IsError() {
		return fmt.Errorf("inbox persistence unavailable")
	}
	return nil
}

func (r *WhatsAppRepository) GetByMetaMessageID(ctx context.Context, metaMessageID string) (*whatsapp.InboundMessage, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"meta_message_id": metaMessageID,
			},
		},
		"size": 1,
	}

	result, err := r.messages.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	if result.Total == 0 {
		return nil, fmt.Errorf("whatsapp message not found with meta_message_id %s", metaMessageID)
	}

	var msg whatsapp.InboundMessage
	if err := mapToStruct(result.Hits[0], &msg); err != nil {
		return nil, fmt.Errorf("failed to map document to whatsapp message: %w", err)
	}
	return &msg, nil
}

func (r *WhatsAppRepository) List(ctx context.Context, filter *whatsapp.MessageFilter) ([]*whatsapp.InboundMessage, int64, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	var filters []map[string]interface{}
	filters = append(filters, map[string]interface{}{
		"term": map[string]interface{}{"app_id": filter.AppID},
	})
	filters = append(filters, inboxEnvironmentFilter(ctx))

	if filter.ContactWAID != "" {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{"contact_wa_id": filter.ContactWAID},
		})
	}
	if filter.Direction != "" {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{"direction": filter.Direction},
		})
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": filters,
			},
		},
		"sort": []map[string]interface{}{
			{"timestamp": map[string]interface{}{"order": "desc"}},
		},
		"from": offset,
		"size": limit,
	}

	result, err := r.messages.Search(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	messages := make([]*whatsapp.InboundMessage, 0, len(result.Hits))
	for _, hit := range result.Hits {
		var msg whatsapp.InboundMessage
		if err := mapToStruct(hit, &msg); err != nil {
			continue
		}
		messages = append(messages, &msg)
	}
	return messages, result.Total, nil
}

func (r *WhatsAppRepository) ListConversations(ctx context.Context, appID string, limit, offset int) ([]*whatsapp.Conversation, int64, error) {
	if limit <= 0 {
		limit = 50
	}

	query := map[string]interface{}{
		"size": 0,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{"filter": []interface{}{map[string]interface{}{"term": map[string]interface{}{"app_id": appID}}, inboxEnvironmentFilter(ctx)}},
		},
		"aggs": map[string]interface{}{
			"contacts": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "contact_wa_id",
					"size":  limit + offset,
					"order": map[string]interface{}{"latest_msg": "desc"},
				},
				"aggs": map[string]interface{}{
					"unread": map[string]interface{}{"filter": map[string]interface{}{"bool": map[string]interface{}{"must": []interface{}{map[string]interface{}{"term": map[string]interface{}{"direction": "inbound"}}}, "must_not": []interface{}{map[string]interface{}{"exists": map[string]interface{}{"field": "read_at"}}}}}},
					"latest_msg": map[string]interface{}{
						"max": map[string]interface{}{"field": "timestamp"},
					},
					"top_hit": map[string]interface{}{
						"top_hits": map[string]interface{}{
							"size":    1,
							"sort":    []map[string]interface{}{{"timestamp": map[string]interface{}{"order": "desc"}}},
							"_source": []string{"contact_wa_id", "contact_name", "text_body", "direction", "timestamp"},
						},
					},
				},
			},
		},
	}

	data, _ := json.Marshal(query)
	req := esapi.SearchRequest{
		Index: []string{"whatsapp_messages"},
		Body:  strings.NewReader(string(data)),
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return nil, 0, fmt.Errorf("conversation aggregation failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, 0, fmt.Errorf("elasticsearch error: %s", res.String())
	}

	var raw map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, 0, fmt.Errorf("failed to decode aggregation: %w", err)
	}

	aggs, _ := raw["aggregations"].(map[string]interface{})
	contacts, _ := aggs["contacts"].(map[string]interface{})
	buckets, _ := contacts["buckets"].([]interface{})

	total := int64(len(buckets))

	// Apply offset
	if offset > 0 && offset < len(buckets) {
		buckets = buckets[offset:]
	} else if offset >= len(buckets) {
		return nil, total, nil
	}
	if len(buckets) > limit {
		buckets = buckets[:limit]
	}

	conversations := make([]*whatsapp.Conversation, 0, len(buckets))
	for _, b := range buckets {
		bucket, ok := b.(map[string]interface{})
		if !ok {
			continue
		}

		conv := &whatsapp.Conversation{
			ContactWAID: fmt.Sprintf("%v", bucket["key"]),
		}
		unread, _ := bucket["unread"].(map[string]interface{})
		if dc, ok := unread["doc_count"].(float64); ok {
			conv.UnreadCount = int64(dc)
		}

		topHit, _ := bucket["top_hit"].(map[string]interface{})
		if topHit != nil {
			hits, _ := topHit["hits"].(map[string]interface{})
			hitsList, _ := hits["hits"].([]interface{})
			if len(hitsList) > 0 {
				hit, _ := hitsList[0].(map[string]interface{})
				source, _ := hit["_source"].(map[string]interface{})
				if source != nil {
					conv.ContactName, _ = source["contact_name"].(string)
					conv.LastMessage, _ = source["text_body"].(string)
					conv.LastDirection, _ = source["direction"].(string)
					if ts, ok := source["timestamp"].(string); ok {
						conv.LastMessageAt, _ = time.Parse(time.RFC3339Nano, ts)
					}
				}
			}
		}

		conversations = append(conversations, conv)
	}

	return conversations, total, nil
}

func (r *WhatsAppRepository) MarkContactRead(ctx context.Context, appID, contact string, at time.Time) error {
	body, _ := json.Marshal(map[string]interface{}{"query": map[string]interface{}{"bool": map[string]interface{}{"filter": []interface{}{map[string]interface{}{"term": map[string]interface{}{"app_id": appID}}, map[string]interface{}{"term": map[string]interface{}{"contact_wa_id": contact}}, map[string]interface{}{"term": map[string]interface{}{"direction": "inbound"}}, inboxEnvironmentFilter(ctx)}, "must_not": []interface{}{map[string]interface{}{"exists": map[string]interface{}{"field": "read_at"}}}}}, "script": map[string]interface{}{"source": "ctx._source.read_at=params.at", "params": map[string]interface{}{"at": at.UTC().Format(time.RFC3339Nano)}}})
	res, err := r.client.UpdateByQuery([]string{"whatsapp_messages"}, r.client.UpdateByQuery.WithContext(ctx), r.client.UpdateByQuery.WithBody(strings.NewReader(string(body))), r.client.UpdateByQuery.WithRefresh(true))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("inbox read state update unavailable")
	}
	var out struct {
		Failures []interface{} `json:"failures"`
	}
	if err = json.NewDecoder(res.Body).Decode(&out); err != nil {
		return err
	}
	if len(out.Failures) > 0 {
		return fmt.Errorf("inbox read state update incomplete")
	}
	return nil
}
