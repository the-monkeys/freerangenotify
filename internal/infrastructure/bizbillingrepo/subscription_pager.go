package bizbillingrepo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

var ErrSubscriptionPITExpired = errors.New("subscription paging snapshot expired")

type SubscriptionPage struct {
	PITID string
	Rows  []bizbilling.BizSubscription
	After []json.RawMessage
}

type SubscriptionPager struct{ es *elasticsearch.Client }

func NewSubscriptionPager(es *elasticsearch.Client) *SubscriptionPager {
	return &SubscriptionPager{es: es}
}

func (p *SubscriptionPager) Open(ctx context.Context) (string, error) {
	res, err := p.es.OpenPointInTime([]string{subscriptionIndex}, "30s", p.es.OpenPointInTime.WithContext(ctx))
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.IsError() {
		return "", bizbilling.ErrReconciliationRequired
	}
	var result struct {
		ID     string `json:"id"`
		Shards struct {
			Failed int `json:"failed"`
		} `json:"_shards"`
	}
	if json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&result) != nil || result.ID == "" || result.Shards.Failed != 0 {
		return "", bizbilling.ErrReconciliationRequired
	}
	return result.ID, nil
}

func (p *SubscriptionPager) Close(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	body, _ := json.Marshal(map[string]string{"id": id})
	res, err := p.es.ClosePointInTime(p.es.ClosePointInTime.WithContext(ctx), p.es.ClosePointInTime.WithBody(bytes.NewReader(body)))
	if err == nil {
		res.Body.Close()
	}
}

func (p *SubscriptionPager) Page(ctx context.Context, pit, app, field string, statuses []string, now time.Time, after []json.RawMessage, size int) (SubscriptionPage, error) {
	var page SubscriptionPage
	if field != "current_period_end" && field != "trial_end" {
		return page, bizbilling.ErrInvalidPeriod
	}
	query := NewQuery(app).Terms("status", statuses).Range(field, "1970-01-01T00:00:00Z", now.UTC().Format(time.RFC3339Nano))
	if size < 1 || size > 100 {
		size = 100
	}
	body := map[string]any{"size": size, "track_total_hits": false, "pit": map[string]any{"id": pit, "keep_alive": "30s"}, "query": query.Bool(), "sort": []any{map[string]any{field: "asc"}, map[string]any{"id": "asc"}, map[string]any{"_shard_doc": "asc"}}, "_source": []string{"id", "app_id", "status", "current_period_end", "trial_end"}}
	if len(after) > 0 {
		body["search_after"] = after
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return page, err
	}
	res, err := p.es.Search(p.es.Search.WithContext(ctx), p.es.Search.WithBody(bytes.NewReader(raw)))
	if err != nil {
		return page, err
	}
	defer res.Body.Close()
	if res.StatusCode == 404 {
		return page, ErrSubscriptionPITExpired
	}
	if res.IsError() {
		return page, bizbilling.ErrReconciliationRequired
	}
	var result struct {
		PITID    string `json:"pit_id"`
		TimedOut bool   `json:"timed_out"`
		Shards   struct {
			Failed int `json:"failed"`
		} `json:"_shards"`
		Hits struct {
			Hits []struct {
				Source bizbilling.BizSubscription `json:"_source"`
				Sort   []json.RawMessage          `json:"sort"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&result) != nil || result.TimedOut || result.Shards.Failed != 0 {
		return page, bizbilling.ErrReconciliationRequired
	}
	page.PITID = result.PITID
	if page.PITID == "" {
		page.PITID = pit
	}
	for _, hit := range result.Hits.Hits {
		if hit.Source.ID == "" || hit.Source.AppID == "" || len(hit.Sort) != 3 {
			return SubscriptionPage{}, bizbilling.ErrReconciliationRequired
		}
		page.Rows = append(page.Rows, hit.Source)
		page.After = hit.Sort
	}
	return page, nil
}
