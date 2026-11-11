package bizbillingrepo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

const maxCASAttempts = 8

// Version binds an ES revision and its original source to one document. The
// private snapshots preserve fields written by newer application versions.
type Version struct {
	SeqNo, PrimaryTerm int64
	index, id          string
	source, before     json.RawMessage
}

func (s *Store[T]) GetVersioned(ctx context.Context, id string) (*T, Version, error) {
	var version Version
	if id == "" {
		return nil, version, fmt.Errorf("bizbillingrepo: document ID is required")
	}
	res, err := (esapi.GetRequest{Index: s.index, DocumentID: id}).Do(ctx, s.es)
	if err != nil {
		return nil, version, fmt.Errorf("bizbillingrepo: read: %w", err)
	}
	defer res.Body.Close()
	var body struct {
		Found       bool            `json:"found"`
		SeqNo       int64           `json:"_seq_no"`
		PrimaryTerm int64           `json:"_primary_term"`
		Source      json.RawMessage `json:"_source"`
		Error       json.RawMessage `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, version, fmt.Errorf("bizbillingrepo: read response: %w", err)
	}
	if res.StatusCode == http.StatusNotFound && !body.Found && len(body.Error) == 0 {
		return nil, version, apperrors.NotFound(s.index, id)
	}
	if res.IsError() {
		return nil, version, fmt.Errorf("bizbillingrepo: read HTTP %d", res.StatusCode)
	}
	if !body.Found || body.PrimaryTerm < 1 || body.SeqNo < 0 || len(body.Source) == 0 {
		return nil, version, fmt.Errorf("bizbillingrepo: invalid document revision")
	}
	var doc T
	if err := json.Unmarshal(body.Source, &doc); err != nil {
		return nil, version, fmt.Errorf("bizbillingrepo: decode document: %w", err)
	}
	before, err := json.Marshal(&doc)
	if err != nil {
		return nil, version, err
	}
	if s.financialReader != nil {
		normalized, err := s.financialReader(ctx, s.index, id, body.Source)
		if err != nil {
			return nil, version, err
		}
		if err = json.Unmarshal(normalized, &doc); err != nil {
			return nil, version, err
		}
	}
	return &doc, Version{SeqNo: body.SeqNo, PrimaryTerm: body.PrimaryTerm, index: s.index, id: id, source: body.Source, before: before}, nil
}

func (s *Store[T]) CreateIfAbsent(ctx context.Context, id string, doc *T) (bool, error) {
	if id == "" || doc == nil {
		return false, fmt.Errorf("bizbillingrepo: document and ID are required")
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return false, err
	}
	done, err := s.guardFinancialWrite(ctx, nil, data)
	if err != nil {
		return false, err
	}
	defer done()
	req := esapi.IndexRequest{Index: s.index, DocumentID: id, OpType: "create", Refresh: string(s.refresh), Body: bytes.NewReader(data)}
	err = s.write(ctx, req)
	if err == bizbilling.ErrConflict {
		return false, nil
	}
	return err == nil, err
}

func (s *Store[T]) ReplaceVersioned(ctx context.Context, id string, doc *T, expected Version) error {
	if doc == nil || id == "" || expected.id != id || expected.index != s.index || len(expected.source) == 0 || len(expected.before) == 0 || expected.SeqNo < 0 || expected.PrimaryTerm < 1 {
		return fmt.Errorf("bizbillingrepo: version does not belong to this document")
	}
	seq, term := int(expected.SeqNo), int(expected.PrimaryTerm)
	if int64(seq) != expected.SeqNo || int64(term) != expected.PrimaryTerm {
		return fmt.Errorf("bizbillingrepo: revision exceeds supported integer range")
	}
	after, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	merged, err := mergeDeclaredChanges(expected.source, expected.before, after)
	if err != nil {
		return err
	}
	done, err := s.guardFinancialWrite(ctx, expected.source, merged)
	if err != nil {
		return err
	}
	defer done()
	return s.write(ctx, esapi.IndexRequest{Index: s.index, DocumentID: id, IfSeqNo: &seq, IfPrimaryTerm: &term, Refresh: string(s.refresh), Body: bytes.NewReader(merged)})
}

func (s *Store[T]) write(ctx context.Context, req esapi.IndexRequest) error {
	res, err := req.Do(ctx, s.es)
	if err != nil {
		return fmt.Errorf("bizbillingrepo: write outcome unknown: %w", err)
	}
	defer res.Body.Close()
	// Drain without retaining source data or gateway secrets in error messages.
	_, _ = io.Copy(io.Discard, res.Body)
	if res.StatusCode == http.StatusConflict {
		return bizbilling.ErrConflict
	}
	if res.IsError() {
		return fmt.Errorf("bizbillingrepo: write HTTP %d", res.StatusCode)
	}
	return nil
}

// Apply the typed before/after difference to the original JSON. Unknown source
// keys and unchanged values retain their exact representation (including int64).
// Changed arrays are intentional replacements, not merges by unstable position.
func mergeDeclaredChanges(source, before, after json.RawMessage) (json.RawMessage, error) {
	if jsonEqual(before, after) {
		return source, nil
	}
	var original, previous, next map[string]json.RawMessage
	if json.Unmarshal(source, &original) != nil || json.Unmarshal(before, &previous) != nil || json.Unmarshal(after, &next) != nil || original == nil || previous == nil || next == nil {
		return after, nil
	}
	for key, old := range previous {
		value, exists := next[key]
		if !exists {
			delete(original, key)
			continue
		}
		if jsonEqual(old, value) {
			continue
		}
		merged, err := mergeDeclaredChanges(original[key], old, value)
		if err != nil {
			return nil, err
		}
		original[key] = merged
	}
	for key, value := range next {
		if _, exists := previous[key]; !exists {
			original[key] = value
		}
	}
	return json.Marshal(original)
}

func jsonEqual(a, b json.RawMessage) bool {
	decode := func(raw json.RawMessage) (any, error) {
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		var v any
		err := d.Decode(&v)
		return v, err
	}
	left, err := decode(a)
	if err != nil {
		return false
	}
	right, err := decode(b)
	return err == nil && reflect.DeepEqual(left, right)
}
