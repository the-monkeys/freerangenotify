package repository

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// ESQuery builds Elasticsearch queries with a fluent API.
type ESQuery struct {
	filters []map[string]interface{}
	sort    []map[string]interface{}
	size    int
	from    int
	after   []interface{}
}

// NewQuery creates a new query builder with a default size.
func NewQuery(size int) *ESQuery {
	if size <= 0 {
		size = 50
	}
	return &ESQuery{size: size}
}

// Term adds a term filter (exact match on keyword field).
func (q *ESQuery) Term(field string, value interface{}) *ESQuery {
	if value == nil {
		return q
	}
	switch v := value.(type) {
	case string:
		if v == "" {
			return q
		}
	case int:
		if v == 0 {
			return q
		}
	}
	q.filters = append(q.filters, map[string]interface{}{
		"term": map[string]interface{}{field: value},
	})
	return q
}

// Terms adds a terms filter (match any of the values).
func (q *ESQuery) Terms(field string, values interface{}) *ESQuery {
	q.filters = append(q.filters, map[string]interface{}{
		"terms": map[string]interface{}{field: values},
	})
	return q
}

// Range adds a range filter.
func (q *ESQuery) Range(field string, conditions map[string]interface{}) *ESQuery {
	q.filters = append(q.filters, map[string]interface{}{
		"range": map[string]interface{}{field: conditions},
	})
	return q
}

// Sort adds a sort clause. order should be "asc" or "desc".
func (q *ESQuery) Sort(field, order string) *ESQuery {
	if order == "" {
		order = "desc"
	}
	q.sort = append(q.sort, map[string]interface{}{
		field: map[string]interface{}{"order": order},
	})
	return q
}

// Offset sets the from value for traditional offset pagination.
func (q *ESQuery) Offset(from int) *ESQuery {
	q.from = from
	return q
}

// SearchAfter sets the search_after values for cursor pagination.
func (q *ESQuery) SearchAfter(values []interface{}) *ESQuery {
	q.after = values
	return q
}

// Build produces the final Elasticsearch query map.
func (q *ESQuery) Build() map[string]interface{} {
	result := map[string]interface{}{
		"size": q.size,
	}

	if len(q.filters) > 0 {
		result["query"] = map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": q.filters,
			},
		}
	}

	if len(q.sort) > 0 {
		result["sort"] = q.sort
	}

	if len(q.after) > 0 {
		result["search_after"] = q.after
	} else if q.from > 0 {
		result["from"] = q.from
	}

	return result
}

// EncodeCursor encodes search_after values into an opaque base64 cursor string.
func EncodeCursor(sortValues []interface{}) string {
	if len(sortValues) == 0 {
		return ""
	}
	data, err := json.Marshal(sortValues)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

// DecodeCursor decodes an opaque base64 cursor string back into search_after values.
func DecodeCursor(cursor string) ([]interface{}, error) {
	if cursor == "" {
		return nil, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor: %w", err)
	}
	var values []interface{}
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("invalid cursor data: %w", err)
	}
	return values, nil
}
