package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"strconv"
	"strings"
)

type ChannelConnectionRepository struct{ client *elasticsearch.Client }

func NewChannelConnectionRepository(client *elasticsearch.Client) *ChannelConnectionRepository {
	return &ChannelConnectionRepository{client}
}

type connectionDocument struct {
	Current channelconnection.Connection            `json:"current"`
	History map[string]channelconnection.Connection `json:"history"`
}

func (r *ChannelConnectionRepository) load(ctx context.Context, id string) (*connectionDocument, int, int, error) {
	res, err := (&esapi.GetRequest{Index: "channel_connections", DocumentID: id}).Do(ctx, r.client)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("connection storage unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode == 404 {
		return nil, 0, 0, channelconnection.ErrNotFound
	}
	if res.IsError() {
		return nil, 0, 0, fmt.Errorf("connection storage returned %d", res.StatusCode)
	}
	var out struct {
		Source   connectionDocument `json:"_source"`
		Sequence int                `json:"_seq_no"`
		Term     int                `json:"_primary_term"`
	}
	if err = json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, 0, 0, fmt.Errorf("invalid connection document")
	}
	hydrateConnectionPolicy(&out.Source)
	return &out.Source, out.Sequence, out.Term, nil
}
func (r *ChannelConnectionRepository) Get(ctx context.Context, id string) (*channelconnection.Connection, error) {
	doc, _, _, err := r.load(ctx, id)
	if err != nil {
		return nil, err
	}
	return &doc.Current, nil
}
func (r *ChannelConnectionRepository) GetRevision(ctx context.Context, id string, revision int64) (*channelconnection.Connection, error) {
	doc, _, _, err := r.load(ctx, id)
	if err != nil {
		return nil, err
	}
	c, ok := doc.History[strconv.FormatInt(revision, 10)]
	if !ok {
		return nil, channelconnection.ErrNotFound
	}
	return &c, nil
}
func (r *ChannelConnectionRepository) Save(ctx context.Context, c *channelconnection.Connection, expected int64) error {
	if c.Revision != expected+1 {
		return channelconnection.ErrConflict
	}
	doc := &connectionDocument{History: map[string]channelconnection.Connection{}}
	var seq, term int
	if expected > 0 {
		old, s, t, err := r.load(ctx, c.ID)
		if err != nil {
			return err
		}
		if old.Current.Revision != expected {
			return channelconnection.ErrConflict
		}
		doc = old
		seq = s
		term = t
	}
	doc.Current = *c
	// Provider policy is not searchable. Keep it in the existing unindexed
	// immutable history to prevent dynamic date inference from breaking drafts.
	doc.Current.Exotel = nil
	doc.History[strconv.FormatInt(c.Revision, 10)] = *c
	raw, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	req := esapi.IndexRequest{Index: "channel_connections", DocumentID: c.ID, Body: strings.NewReader(string(raw)), Refresh: "wait_for"}
	if expected == 0 {
		req.OpType = "create"
	} else {
		req.IfSeqNo = &seq
		req.IfPrimaryTerm = &term
	}
	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("connection write unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode == 409 {
		return channelconnection.ErrConflict
	}
	if res.IsError() {
		return fmt.Errorf("connection write returned %d", res.StatusCode)
	}
	return nil
}
func (r *ChannelConnectionRepository) find(ctx context.Context, provider, account, sender string) ([]*channelconnection.Connection, error) {
	filters := []interface{}{}
	for field, value := range map[string]string{"provider": provider, "account_id": account, "sender_id": sender} {
		if value != "" {
			filters = append(filters, map[string]interface{}{"bool": map[string]interface{}{"minimum_should_match": 1, "should": []interface{}{map[string]interface{}{"term": map[string]interface{}{"current." + field: value}}, map[string]interface{}{"term": map[string]interface{}{"current." + field + ".keyword": value}}}}})
		}
	}
	raw, _ := json.Marshal(map[string]interface{}{"size": 1000, "track_total_hits": true, "query": map[string]interface{}{"bool": map[string]interface{}{"filter": filters}}})
	res, err := (&esapi.SearchRequest{Index: []string{"channel_connections"}, Body: strings.NewReader(string(raw))}).Do(ctx, r.client)
	if err != nil {
		return nil, fmt.Errorf("connection lookup unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode == 404 {
		return nil, nil
	}
	if res.IsError() {
		return nil, fmt.Errorf("connection lookup returned %d", res.StatusCode)
	}
	var out struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source connectionDocument `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err = json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Hits.Total.Value > len(out.Hits.Hits) {
		return nil, fmt.Errorf("connection lookup exceeds safe limit")
	}
	var rows []*channelconnection.Connection
	for _, h := range out.Hits.Hits {
		hydrateConnectionPolicy(&h.Source)
		c := h.Source.Current
		rows = append(rows, &c)
	}
	return rows, nil
}
func (r *ChannelConnectionRepository) FindSender(ctx context.Context, provider, account, sender string) ([]*channelconnection.Connection, error) {
	return r.find(ctx, provider, account, sender)
}
func (r *ChannelConnectionRepository) FindAccount(ctx context.Context, provider, account string) ([]*channelconnection.Connection, error) {
	return r.find(ctx, provider, account, "")
}

func hydrateConnectionPolicy(doc *connectionDocument) {
	if doc.Current.Exotel == nil {
		doc.Current.Exotel = doc.History[strconv.FormatInt(doc.Current.Revision, 10)].Exotel
	}
}
