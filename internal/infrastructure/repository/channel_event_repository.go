package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"strings"
	"time"
)

type ChannelEventRepository struct{ client *elasticsearch.Client }

func NewChannelEventRepository(client *elasticsearch.Client) *ChannelEventRepository {
	return &ChannelEventRepository{client}
}
func (r *ChannelEventRepository) AcceptEvent(ctx context.Context, receipt *whatsapp.EventReceipt) error {
	raw, _ := json.Marshal(receipt)
	req := esapi.CreateRequest{Index: "channel_events", DocumentID: receipt.ID, Body: strings.NewReader(string(raw))}
	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("callback durable acceptance unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode == 409 {
		return nil
	}
	if res.IsError() {
		return fmt.Errorf("callback durable acceptance status %d", res.StatusCode)
	}
	return nil
}
func (r *ChannelEventRepository) get(ctx context.Context, id string) (*whatsapp.EventReceipt, error) {
	req := esapi.GetRequest{Index: "channel_events", DocumentID: id}
	res, err := req.Do(ctx, r.client)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("callback receipt unavailable")
	}
	var raw struct {
		Source      whatsapp.EventReceipt `json:"_source"`
		Sequence    int64                 `json:"_seq_no"`
		PrimaryTerm int64                 `json:"_primary_term"`
	}
	if err = json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, err
	}
	raw.Source.Sequence = raw.Sequence
	raw.Source.PrimaryTerm = raw.PrimaryTerm
	return &raw.Source, nil
}
func (r *ChannelEventRepository) SaveEvent(ctx context.Context, event *whatsapp.EventReceipt) error {
	event.UpdatedAt = time.Now().UTC()
	raw, _ := json.Marshal(event)
	sequence, primaryTerm := int(event.Sequence), int(event.PrimaryTerm)
	req := esapi.IndexRequest{Index: "channel_events", DocumentID: event.ID, Body: strings.NewReader(string(raw)), IfSeqNo: &sequence, IfPrimaryTerm: &primaryTerm}
	res, err := req.Do(ctx, r.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("callback transition rejected: %d", res.StatusCode)
	}
	var out struct {
		Sequence    int64 `json:"_seq_no"`
		PrimaryTerm int64 `json:"_primary_term"`
	}
	if err = json.NewDecoder(res.Body).Decode(&out); err != nil {
		return err
	}
	event.Sequence = out.Sequence
	event.PrimaryTerm = out.PrimaryTerm
	return nil
}
func (r *ChannelEventRepository) ClaimEvent(ctx context.Context, id string, now time.Time) (*whatsapp.EventReceipt, error) {
	event, err := r.get(ctx, id)
	if err != nil {
		return nil, err
	}
	if event.State == "done" || event.State == "unknown" || event.State == "dead_letter" || event.LeaseUntil.After(now) || event.NextAttempt.After(now) {
		return nil, nil
	}
	for _, state := range event.Steps {
		if state == "dispatching" {
			event.State = "unknown"
			event.LastError = "side effect outcome requires reconciliation"
			return nil, r.SaveEvent(ctx, event)
		}
	}
	event.State = "processing"
	event.LeaseToken = uuid.NewString()
	event.LeaseUntil = now.Add(time.Minute)
	event.Attempts++
	if err := r.SaveEvent(ctx, event); err != nil {
		return nil, err
	}
	return event, nil
}
func (r *ChannelEventRepository) PendingEvents(ctx context.Context, now time.Time, limit int) ([]string, error) {
	return r.pendingEvents(ctx, now, limit, "")
}
func (r *ChannelEventRepository) PendingEventsForKind(ctx context.Context, now time.Time, limit int, kind string) ([]string, error) {
	return r.pendingEvents(ctx, now, limit, kind)
}
func (r *ChannelEventRepository) pendingEvents(ctx context.Context, now time.Time, limit int, kind string) ([]string, error) {
	body, _ := json.Marshal(map[string]interface{}{"size": limit, "query": map[string]interface{}{"bool": map[string]interface{}{"filter": []interface{}{map[string]interface{}{"terms": map[string]interface{}{"state.keyword": []string{"pending", "processing"}}}, map[string]interface{}{"range": map[string]interface{}{"lease_until": map[string]interface{}{"lte": now.Format(time.RFC3339Nano)}}}, map[string]interface{}{"range": map[string]interface{}{"next_attempt": map[string]interface{}{"lte": now.Format(time.RFC3339Nano)}}}}}}, "sort": []interface{}{map[string]interface{}{"created_at": "asc"}}})
	var query map[string]interface{}
	_ = json.Unmarshal(body, &query)
	b := query["query"].(map[string]interface{})["bool"].(map[string]interface{})
	term := map[string]interface{}{"term": map[string]interface{}{"event.kind.keyword": "exotel_status"}}
	if kind == "" {
		b["must_not"] = []interface{}{term}
	} else {
		b["filter"] = append(b["filter"].([]interface{}), map[string]interface{}{"term": map[string]interface{}{"event.kind.keyword": kind}})
	}
	body, _ = json.Marshal(query)
	req := esapi.SearchRequest{Index: []string{"channel_events"}, Body: strings.NewReader(string(body))}
	res, err := req.Do(ctx, r.client)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == 404 {
		return nil, nil
	}
	if res.IsError() {
		return nil, fmt.Errorf("callback recovery unavailable")
	}
	var out struct {
		Hits struct {
			Hits []struct {
				ID string `json:"_id"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err = json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(out.Hits.Hits))
	for _, hit := range out.Hits.Hits {
		ids = append(ids, hit.ID)
	}
	return ids, nil
}
