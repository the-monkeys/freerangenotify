package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"strings"
)

// BulkDocument keeps request and response order stable, including repeated payload values.
type BulkDocument struct {
	ID       string
	Document any
}

// BulkCreateResults returns one error per item. A request/decode error has an unknown
// write outcome; callers must not publish, trigger hooks, or automatically retry it.
func (r *BaseRepository) BulkCreateResults(ctx context.Context, documents []BulkDocument) ([]error, error) {
	if len(documents) == 0 {
		return nil, nil
	}
	var body strings.Builder
	seen := make(map[string]bool, len(documents))
	for _, doc := range documents {
		if doc.ID == "" || seen[doc.ID] {
			return nil, fmt.Errorf("bulk document IDs must be nonempty and unique")
		}
		seen[doc.ID] = true
		action, _ := json.Marshal(map[string]any{"index": map[string]string{"_index": r.indexName, "_id": doc.ID}})
		data, err := json.Marshal(doc.Document)
		if err != nil {
			return nil, fmt.Errorf("marshal bulk document: %w", err)
		}
		body.Write(action)
		body.WriteByte('\n')
		body.Write(data)
		body.WriteByte('\n')
	}
	req := esapi.BulkRequest{Body: strings.NewReader(body.String()), Refresh: string(r.defaultRefresh)}
	res, err := req.Do(ctx, r.client)
	if err != nil {
		return nil, fmt.Errorf("bulk write outcome unknown: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("bulk request rejected (status %d)", res.StatusCode)
	}
	var result struct {
		Items []map[string]struct {
			ID     string          `json:"_id"`
			Status int             `json:"status"`
			Error  json.RawMessage `json:"error"`
		} `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("bulk response outcome unknown: %w", err)
	}
	if len(result.Items) != len(documents) {
		return nil, fmt.Errorf("incomplete bulk response: write outcome unknown")
	}
	outcomes := make([]error, len(documents))
	for i, item := range result.Items {
		write, ok := item["index"]
		if !ok || write.ID != documents[i].ID || write.Status == 0 {
			return nil, fmt.Errorf("invalid bulk item response: write outcome unknown")
		}
		if write.Status < 200 || write.Status >= 300 || (len(write.Error) > 0 && string(write.Error) != "null") {
			outcomes[i] = fmt.Errorf("storage rejected row (status %d)", write.Status)
		}
	}
	return outcomes, nil
}
