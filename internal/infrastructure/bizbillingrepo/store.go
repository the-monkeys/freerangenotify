package bizbillingrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

// Store is a generic, typed Elasticsearch document store. It wraps
// BaseRepository so every biz billing entity gets Create/Get/Update/Delete/
// Find without repeating marshal/unmarshal boilerplate.
type Store[T any] struct {
	financialGuard  FinancialWriteGuard
	financialReader FinancialReadOverlay
	base            *repository.BaseRepository
	es              *elasticsearch.Client
	index           string
	logger          *zap.Logger
	refresh         repository.RefreshPolicy
}

// NewStore creates a typed store for the given index.
func NewStore[T any](es *elasticsearch.Client, index string, logger *zap.Logger, policies ...repository.RefreshPolicy) *Store[T] {
	refresh := repository.RefreshWaitFor
	if len(policies) > 0 {
		refresh = policies[0]
	}
	return &Store[T]{
		base:    repository.NewBaseRepository(es, index, logger, refresh),
		es:      es,
		index:   index,
		logger:  logger,
		refresh: refresh,
	}
}

func (s *Store[T]) Create(ctx context.Context, id string, doc *T) error {
	if id == "" {
		return fmt.Errorf("bizbillingrepo: document ID is required")
	}
	created, err := s.CreateIfAbsent(ctx, id, doc)
	if err == nil && !created {
		return bizbilling.ErrConflict
	}
	return err
}

func (s *Store[T]) Get(ctx context.Context, id string) (*T, error) {
	doc, _, err := s.GetVersioned(ctx, id)
	return doc, err
}

// Update retries only version conflicts. fn must be pure: it may run more than
// once and must never send notifications, create orders, or mint identifiers.
func (s *Store[T]) Update(ctx context.Context, id string, fn func(*T) error) (*T, error) {
	for attempt := 0; attempt < maxCASAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		doc, version, err := s.GetVersioned(ctx, id)
		if err != nil {
			return nil, err
		}
		if err = fn(doc); err != nil {
			return nil, err
		}
		if err = s.ReplaceVersioned(ctx, id, doc, version); errors.Is(err, bizbilling.ErrConflict) {
			continue
		} else if err != nil {
			return nil, err
		}
		return doc, nil
	}
	return nil, bizbilling.ErrConflict
}

// Replace replaces declared fields with a conditional write. Callers computing
// changes from a previous read must use Update instead to avoid stale values.
func (s *Store[T]) Replace(ctx context.Context, id string, doc *T) error {
	_, version, err := s.GetVersioned(ctx, id)
	if err != nil {
		return err
	}
	return s.ReplaceVersioned(ctx, id, doc, version)
}

func (s *Store[T]) Delete(ctx context.Context, id string) error {
	if s.financialGuard == nil {
		return s.base.Delete(ctx, id)
	}
	_, version, err := s.GetVersioned(ctx, id)
	if err != nil {
		return err
	}
	done, err := s.guardFinancialWrite(ctx, version.source, nil)
	if err != nil {
		return err
	}
	defer done()
	seq, term := int(version.SeqNo), int(version.PrimaryTerm)
	if int64(seq) != version.SeqNo || int64(term) != version.PrimaryTerm {
		return bizbilling.ErrConflict
	}
	request := esapi.DeleteRequest{Index: s.index, DocumentID: id, IfSeqNo: &seq, IfPrimaryTerm: &term, Refresh: string(s.refresh)}
	res, err := request.Do(ctx, s.es)
	if err != nil {
		return fmt.Errorf("billing deletion outcome unknown: %w", err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	if res.StatusCode == 409 {
		return bizbilling.ErrConflict
	}
	if res.IsError() {
		return fmt.Errorf("billing deletion HTTP %d", res.StatusCode)
	}
	return nil

}

// Find executes a full search request body and decodes the hits.
func (s *Store[T]) Find(ctx context.Context, searchBody map[string]interface{}) ([]*T, int64, error) {
	data, err := json.Marshal(searchBody)
	if err != nil {
		return nil, 0, err
	}
	res, err := (esapi.SearchRequest{Index: []string{s.index}, Body: strings.NewReader(string(data))}).Do(ctx, s.es)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, 0, fmt.Errorf("bizbillingrepo: search HTTP %d", res.StatusCode)
	}
	var result struct {
		TimedOut bool `json:"timed_out"`
		Shards   struct {
			Failed int `json:"failed"`
		} `json:"_shards"`
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				ID     string          `json:"_id"`
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err = json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, 0, err
	}
	if result.TimedOut || result.Shards.Failed != 0 {
		return nil, 0, bizbilling.ErrReconciliationRequired
	}
	docs := make([]*T, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		var doc T
		source := hit.Source
		if s.financialReader != nil {
			source, err = s.financialReader(ctx, s.index, hit.ID, source)
			if err != nil {
				return nil, 0, err
			}
		}
		if err = json.Unmarshal(source, &doc); err != nil {
			return nil, 0, fmt.Errorf("bizbillingrepo: invalid search document: %w", err)
		}
		docs = append(docs, &doc)
	}
	return docs, result.Hits.Total.Value, nil
}

// Refresh makes new discovery records searchable before a stream is committed.
// Monetary resource reads themselves always use realtime GET.
func (s *Store[T]) Refresh(ctx context.Context) error {
	res, err := s.es.Indices.Refresh(s.es.Indices.Refresh.WithIndex(s.index), s.es.Indices.Refresh.WithContext(ctx))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("bizbillingrepo: refresh HTTP %d", res.StatusCode)
	}
	return nil
}

// Aggregate runs a search with aggregations and returns the raw "aggregations"
// section of the response (BaseRepository.Search drops aggs).
func (s *Store[T]) Aggregate(ctx context.Context, searchBody map[string]interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(searchBody)
	if err != nil {
		return nil, fmt.Errorf("bizbillingrepo: marshal agg query: %w", err)
	}

	req := esapi.SearchRequest{
		Index: []string{s.index},
		Body:  strings.NewReader(string(data)),
	}
	res, err := req.Do(ctx, s.es)
	if err != nil {
		return nil, fmt.Errorf("bizbillingrepo: agg search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("bizbillingrepo: elasticsearch error: %s", res.String())
	}

	var result struct {
		Aggregations map[string]interface{} `json:"aggregations"`
		Hits         struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("bizbillingrepo: decode agg response: %w", err)
	}
	if result.Aggregations == nil {
		result.Aggregations = map[string]interface{}{}
	}
	result.Aggregations["_total_hits"] = result.Hits.Total.Value
	return result.Aggregations, nil
}

func decodeDoc[T any](raw map[string]interface{}) (*T, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("bizbillingrepo: marshal doc: %w", err)
	}
	var doc T
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("bizbillingrepo: unmarshal doc: %w", err)
	}
	return &doc, nil
}

// ─── Query building helpers ──────────────────────────────────────────────

// Query incrementally builds a bool/term search body.
type Query struct {
	must []map[string]interface{}
}

// NewQuery starts a query scoped to an app.
func NewQuery(appID string) *Query {
	q := &Query{}
	return q.Term("app_id", appID)
}

// Term adds a term filter when value is non-empty.
func (q *Query) Term(field string, value interface{}) *Query {
	switch v := value.(type) {
	case string:
		if v == "" {
			return q
		}
	case nil:
		return q
	}
	q.must = append(q.must, map[string]interface{}{
		"term": map[string]interface{}{field: value},
	})
	return q
}

// Terms adds a terms (any-of) filter when values is non-empty.
func (q *Query) Terms(field string, values []string) *Query {
	if len(values) == 0 {
		return q
	}
	q.must = append(q.must, map[string]interface{}{
		"terms": map[string]interface{}{field: values},
	})
	return q
}

// Range adds a range filter; pass nil bounds to skip.
func (q *Query) Range(field string, gte, lte interface{}) *Query {
	bounds := map[string]interface{}{}
	if gte != nil {
		bounds["gte"] = gte
	}
	if lte != nil {
		bounds["lte"] = lte
	}
	if len(bounds) == 0 {
		return q
	}
	q.must = append(q.must, map[string]interface{}{
		"range": map[string]interface{}{field: bounds},
	})
	return q
}

// Bool returns just the query section (for use inside custom bodies).
func (q *Query) Bool() map[string]interface{} {
	if len(q.must) == 0 {
		return map[string]interface{}{"match_all": map[string]interface{}{}}
	}
	return map[string]interface{}{
		"bool": map[string]interface{}{"must": q.must},
	}
}

// Body builds a complete search body with sane pagination defaults.
func (q *Query) Body(sortField string, limit, offset int) map[string]interface{} {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	body := map[string]interface{}{
		"query": q.Bool(),
		"size":  limit,
		"from":  offset,
	}
	if sortField != "" {
		body["sort"] = []map[string]interface{}{
			{sortField: map[string]interface{}{"order": "desc"}},
		}
	}
	return body
}
