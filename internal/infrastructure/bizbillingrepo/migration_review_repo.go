package bizbillingrepo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

const MigrationReviewIndex = "frn_biz_migration_reviews"
const LegacyResolutionIndex = "frn_biz_legacy_resolutions"

// All original financial inputs participate in stale-report detection. Source
// scans are bounded, paged and exact; a partial scan never produces readiness.
var migrationSourceIndices = []string{"users", invoiceIndex, paymentIndex, subscriptionIndex, EstimateIndex, ExpenseIndex, CreditNoteIndex, RetainerIndex, CouponIndex, ContractIndex, UsageMeterIndex, UsageEventIndex, RevenueScheduleIndex, planIndex, "frn_biz_plan_addons", "frn_biz_products", configIndex}

type MigrationSource struct {
	Index, ID, Hash, ContentHash string
	SeqNo, PrimaryTerm           int64
	Source                       json.RawMessage
}
type MigrationReviewRepo struct {
	WriterReviews *Store[bizbilling.WriterPermitReviewRecord]
	es            *elasticsearch.Client
	Reports       *Store[bizbilling.MigrationReport]
	Decisions     *Store[bizbilling.LegacyReviewDecision]
}

func NewMigrationReviewRepo(es *elasticsearch.Client, logger *zap.Logger) *MigrationReviewRepo {
	return &MigrationReviewRepo{WriterReviews: NewStore[bizbilling.WriterPermitReviewRecord](es, LegacyResolutionIndex, logger, repository.RefreshFalse), es: es, Reports: NewStore[bizbilling.MigrationReport](es, MigrationReviewIndex, logger, repository.RefreshFalse), Decisions: NewStore[bizbilling.LegacyReviewDecision](es, LegacyResolutionIndex, logger, repository.RefreshFalse)}
}
func MigrationDigest(value any) string {
	data, _ := json.Marshal(value)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
func SourceContentHash(raw json.RawMessage) string {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return ""
	}
	return MigrationDigest(value)
}
func LegacyDecisionID(app, index, id, content string) string {
	return MigrationDigest([]string{app, index, id, content})
}

func (r *MigrationReviewRepo) Inventory(ctx context.Context, app string) ([]MigrationSource, error) {
	all := []MigrationSource{}
	rows, err := r.scan(ctx, migrationSourceIndices, app)
	if err != nil {
		return nil, err
	}
	all = append(all, rows...)
	sort.Slice(all, func(i, j int) bool {
		if all[i].Index == all[j].Index {
			return all[i].ID < all[j].ID
		}
		return all[i].Index < all[j].Index
	})
	return all, nil
}
func (r *MigrationReviewRepo) RefreshSources(ctx context.Context) error {
	res, err := r.es.Indices.Refresh(r.es.Indices.Refresh.WithIndex(migrationSourceIndices...), r.es.Indices.Refresh.WithContext(ctx))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	if res.IsError() {
		return fmt.Errorf("billing source refresh HTTP %d", res.StatusCode)
	}
	return nil
}
func (r *MigrationReviewRepo) scan(ctx context.Context, indices []string, app string) ([]MigrationSource, error) {
	body, _ := json.Marshal(map[string]interface{}{"query": NewQuery(app).Bool(), "size": 200, "sort": []string{"_doc"}, "seq_no_primary_term": true, "track_total_hits": true})
	res, err := (esapi.SearchRequest{Index: indices, Body: bytes.NewReader(body), Scroll: time.Minute}).Do(ctx, r.es)
	if err != nil {
		return nil, err
	}
	scroll := ""
	defer func() {
		if scroll != "" {
			closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			out, err := r.es.ClearScroll(r.es.ClearScroll.WithScrollID(scroll), r.es.ClearScroll.WithContext(closeCtx))
			if err == nil {
				out.Body.Close()
			}
		}
	}()
	result := []MigrationSource{}
	expected := int64(-1)
	for {
		var page struct {
			Scroll   string `json:"_scroll_id"`
			TimedOut bool   `json:"timed_out"`
			Shards   struct {
				Failed int `json:"failed"`
			} `json:"_shards"`
			Hits struct {
				Total struct {
					Value    int64  `json:"value"`
					Relation string `json:"relation"`
				} `json:"total"`
				Hits []struct {
					Index       string          `json:"_index"`
					ID          string          `json:"_id"`
					SeqNo       int64           `json:"_seq_no"`
					PrimaryTerm int64           `json:"_primary_term"`
					Source      json.RawMessage `json:"_source"`
				} `json:"hits"`
			} `json:"hits"`
		}
		if res.IsError() {
			res.Body.Close()
			return nil, fmt.Errorf("billing source scan HTTP %d", res.StatusCode)
		}
		err = json.NewDecoder(res.Body).Decode(&page)
		res.Body.Close()
		if err != nil {
			return nil, err
		}
		if page.Scroll != "" {
			scroll = page.Scroll
		}
		if page.TimedOut || page.Shards.Failed != 0 {
			return nil, bizbilling.ErrReconciliationRequired
		}
		if expected < 0 {
			if page.Hits.Total.Relation != "eq" {
				return nil, bizbilling.ErrReconciliationRequired
			}
			expected = page.Hits.Total.Value
		}
		if len(page.Hits.Hits) == 0 {
			if int64(len(result)) != expected {
				return nil, bizbilling.ErrReconciliationRequired
			}
			return result, nil
		}
		for _, hit := range page.Hits.Hits {
			if hit.ID == "" || hit.PrimaryTerm < 1 || hit.SeqNo < 0 {
				return nil, bizbilling.ErrReconciliationRequired
			}
			content := SourceContentHash(hit.Source)
			result = append(result, MigrationSource{Index: hit.Index, ID: hit.ID, SeqNo: hit.SeqNo, PrimaryTerm: hit.PrimaryTerm, Source: hit.Source, ContentHash: content, Hash: MigrationDigest([]any{hit.SeqNo, hit.PrimaryTerm, content})})
			if len(result) > 100000 {
				return nil, bizbilling.ErrReconciliationRequired
			}
		}
		if int64(len(result)) == expected {
			return result, nil
		}
		if scroll == "" {
			return nil, bizbilling.ErrReconciliationRequired
		}
		res, err = r.es.Scroll(r.es.Scroll.WithScrollID(scroll), r.es.Scroll.WithScroll(time.Minute), r.es.Scroll.WithContext(ctx))
		if err != nil {
			return nil, err
		}
	}
}
