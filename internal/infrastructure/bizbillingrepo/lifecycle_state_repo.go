package bizbillingrepo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"net/http"
)

const LifecycleStateIndex = "frn_biz_lifecycle_states"

type LifecycleStateRepo struct{ es *elasticsearch.Client }

func NewLifecycleStateRepo(es *elasticsearch.Client) *LifecycleStateRepo {
	return &LifecycleStateRepo{es: es}
}
func (r *LifecycleStateRepo) read(ctx context.Context, app string) (bizbilling.LifecycleState, int, int, bool, error) {
	var s bizbilling.LifecycleState
	res, err := (esapi.GetRequest{Index: LifecycleStateIndex, DocumentID: app}).Do(ctx, r.es)
	if err != nil {
		return s, 0, 0, false, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return bizbilling.DefaultLifecycleState(app), 0, 0, false, nil
	}
	if res.IsError() {
		return s, 0, 0, false, fmt.Errorf("billing lifecycle read: HTTP %d", res.StatusCode)
	}
	var doc struct {
		Source bizbilling.LifecycleState `json:"_source"`
		Seq    int                       `json:"_seq_no"`
		Term   int                       `json:"_primary_term"`
	}
	err = json.NewDecoder(res.Body).Decode(&doc)
	return doc.Source, doc.Seq, doc.Term, true, err
}
func (r *LifecycleStateRepo) Load(ctx context.Context, app string) (bizbilling.LifecycleState, error) {
	s, _, _, _, err := r.read(ctx, app)
	return s, err
}
func (r *LifecycleStateRepo) Mutate(ctx context.Context, app string, fn func(*bizbilling.LifecycleState) error) error {
	for attempt := 0; attempt < 8; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		s, seq, term, exists, err := r.read(ctx, app)
		if err != nil {
			return err
		}
		if err = fn(&s); err != nil {
			return err
		}
		data, err := json.Marshal(s)
		if err != nil {
			return err
		}
		req := esapi.IndexRequest{Index: LifecycleStateIndex, DocumentID: app, Body: bytes.NewReader(data)}
		if exists {
			req.IfSeqNo = &seq
			req.IfPrimaryTerm = &term
		} else {
			req.OpType = "create"
		}
		res, err := req.Do(ctx, r.es)
		if err != nil {
			return err
		}
		status := res.StatusCode
		res.Body.Close()
		if status == http.StatusConflict {
			continue
		}
		if status >= 300 {
			return fmt.Errorf("billing lifecycle write: HTTP %d", status)
		}
		return nil
	}
	return bizbilling.ErrConflict
}
