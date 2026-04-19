package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"go.uber.org/zap"
)

// WhatsAppRepository implements whatsapp.Repository using Elasticsearch.
type WhatsAppRepository struct {
	messages *BaseRepository
	client   *elasticsearch.Client
	logger   *zap.Logger
}

// NewWhatsAppRepository creates a new WhatsApp message repository.
func NewWhatsAppRepository(client *elasticsearch.Client, logger *zap.Logger) whatsapp.Repository {
	return &WhatsAppRepository{
		messages: NewBaseRepository(client, "whatsapp_messages", logger, RefreshWaitFor),
		client:   client,
		logger:   logger,
	}
}

func (r *WhatsAppRepository) StoreMessage(ctx context.Context, msg *whatsapp.InboundMessage) error {
	return r.messages.Create(ctx, msg.ID, msg)
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
			"term": map[string]interface{}{"app_id": appID},
		},
		"aggs": map[string]interface{}{
			"contacts": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "contact_wa_id",
					"size":  limit + offset,
					"order": map[string]interface{}{"latest_msg": "desc"},
				},
				"aggs": map[string]interface{}{
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
		if dc, ok := bucket["doc_count"].(float64); ok {
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
