package repository

import (
	"context"
	"encoding/json"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"strings"
	"time"
)

// UpdateSettingsIfRevision guards the read/merge/write against concurrent connect
// and disconnect operations. No whole-application index write is performed.
func (r *ApplicationRepository) UpdateSettingsIfRevision(ctx context.Context, id string, settings application.Settings, revision string) error {
	body, _ := json.Marshal(map[string]interface{}{"script": map[string]interface{}{"source": `if (ctx._source.updated_at != params.revision) { ctx.op = 'none'; } else { ctx._source.settings = params.settings; ctx._source.updated_at = params.now; }`, "params": map[string]interface{}{"revision": revision, "settings": settings, "now": time.Now().Format(time.RFC3339Nano)}}})
	retries:=3
	req := esapi.UpdateRequest{Index: r.indexName, DocumentID: id, Body: strings.NewReader(string(body)), Refresh: "wait_for", RetryOnConflict: &retries}
	res, err := req.Do(ctx, r.client)
	if err != nil {
		return apperrors.Internal("Settings storage unavailable", nil)
	}
	defer res.Body.Close()
	if res.IsError() {
		return apperrors.Internal("Settings storage unavailable", nil)
	}
	var out struct {
		Result string `json:"result"`
	}
	if json.NewDecoder(res.Body).Decode(&out) != nil {
		return apperrors.Internal("Settings response unavailable", nil)
	}
	if out.Result != "updated" {
		return apperrors.Conflict("Settings changed; reload before saving")
	}
	return nil
}
