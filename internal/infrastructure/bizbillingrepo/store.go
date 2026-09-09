package bizbillingrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

// Store is a generic, typed Elasticsearch document store. It wraps
// BaseRepository so every biz billing entity gets Create/Get/Update/Delete/
// Find without repeating marshal/unmarshal boilerplate.
type Store[T any] struct {
	base   *repository.BaseRepository
	es     *elasticsearch.Client
	index  string
	logger *zap.Logger
}

// NewStore creates a typed store for the given index.
func NewStore[T any](es *elasticsearch.Client, index string, logger *zap.Logger) *Store[T] {
	return &Store[T]{
		base:   repository.NewBaseRepository(es, index, logger, repository.RefreshWaitFor),
		es:     es,
		index:  index,
		logger: logger,
	}
}

func (s *Store[T]) Create(ctx context.Context, id string, doc *T) error {
	if id == "" {
		return fmt.Errorf("bizbillingrepo: document ID is required")
	}
	return s.base.Create(ctx, id, doc)
}

func (s *Store[T]) Get(ctx context.Context, id string) (*T, error) {
	raw, err := s.base.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return decodeDoc[T](raw)
}

// Update applies a read-modify-write cycle: fetch, mutate via fn, replace.
func (s *Store[T]) Update(ctx context.Context, id string, fn func(*T) error) (*T, error) {
	doc, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := fn(doc); err != nil {
		return nil, err
	}
	if err := s.base.Replace(ctx, id, doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// Replace overwrites the full document.
func (s *Store[T]) Replace(ctx context.Context, id string, doc *T) error {
	return s.base.Replace(ctx, id, doc)
}

func (s *Store[T]) Delete(ctx context.Context, id string) error {
	return s.base.Delete(ctx, id)
}

// Find executes a full search request body and decodes the hits.
func (s *Store[T]) Find(ctx context.Context, searchBody map[string]interface{}) ([]*T, int64, error) {
	res, err := s.base.Search(ctx, searchBody)
	if err != nil {
		return nil, 0, err
	}

	docs := make([]*T, 0, len(res.Hits))
	for _, hit := range res.Hits {
		doc, err := decodeDoc[T](hit)
		if err != nil {
			s.logger.Error("bizbillingrepo: failed to decode document",
				zap.String("index", s.index), zap.Error(err))
			continue
		}
		docs = append(docs, doc)
	}
	return docs, res.Total, nil
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
