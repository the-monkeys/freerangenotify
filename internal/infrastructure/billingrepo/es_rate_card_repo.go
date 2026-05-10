package billingrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"go.uber.org/zap"
)

const (
	rateCardIndex       = "frn_billing_rate_cards"
	billingRuntimeIndex = "frn_billing_runtime"
	activeRateCardDocID = "active_rate_card"
)

type ESRateCardRepo struct {
	es     *elasticsearch.Client
	logger *zap.Logger
}

func NewESRateCardRepo(es *elasticsearch.Client, logger *zap.Logger) *ESRateCardRepo {
	return &ESRateCardRepo{es: es, logger: logger}
}

func (r *ESRateCardRepo) CreateVersion(ctx context.Context, card *billing.RateCard) error {
	if card == nil {
		return fmt.Errorf("billingrepo: nil rate card")
	}
	if card.Version == "" {
		return fmt.Errorf("billingrepo: version is required")
	}

	now := time.Now().UTC()
	if card.CreatedAt.IsZero() {
		card.CreatedAt = now
	}
	card.UpdatedAt = now

	body, err := json.Marshal(card)
	if err != nil {
		return fmt.Errorf("billingrepo: marshal rate card: %w", err)
	}

	res, err := r.es.Index(
		rateCardIndex,
		strings.NewReader(string(body)),
		r.es.Index.WithDocumentID(card.Version),
		r.es.Index.WithContext(ctx),
		r.es.Index.WithRefresh("wait_for"),
	)
	if err != nil {
		return fmt.Errorf("billingrepo: create rate card version: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("billingrepo: create rate card version error: %s", res.String())
	}

	return nil
}

func (r *ESRateCardRepo) GetByVersion(ctx context.Context, version string) (*billing.RateCard, error) {
	if version == "" {
		return nil, nil
	}

	res, err := r.es.Get(rateCardIndex, version, r.es.Get.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("billingrepo: get rate card by version: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, nil
	}
	if res.IsError() {
		return nil, fmt.Errorf("billingrepo: get rate card by version error: %s", res.String())
	}

	var doc struct {
		Source billing.RateCard `json:"_source"`
	}
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("billingrepo: decode rate card by version: %w", err)
	}
	return &doc.Source, nil
}

func (r *ESRateCardRepo) GetActive(ctx context.Context) (*billing.RateCard, error) {
	version, err := r.getActiveVersion(ctx)
	if err != nil {
		return nil, err
	}
	if version != "" {
		return r.GetByVersion(ctx, version)
	}

	query := `{
		"size": 1,
		"sort": [{ "updated_at": { "order": "desc" } }],
		"query": {
			"bool": {
				"filter": [
					{ "term": { "active": true } }
				]
			}
		}
	}`
	res, err := r.es.Search(
		r.es.Search.WithIndex(rateCardIndex),
		r.es.Search.WithBody(strings.NewReader(query)),
		r.es.Search.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("billingrepo: search active rate card: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("billingrepo: search active rate card error: %s", res.String())
	}

	var raw struct {
		Hits struct {
			Hits []struct {
				Source billing.RateCard `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("billingrepo: decode active rate card: %w", err)
	}
	if len(raw.Hits.Hits) == 0 {
		return nil, nil
	}
	return &raw.Hits.Hits[0].Source, nil
}

func (r *ESRateCardRepo) SetActiveVersion(ctx context.Context, version string) error {
	if version == "" {
		return fmt.Errorf("billingrepo: active version is required")
	}
	card, err := r.GetByVersion(ctx, version)
	if err != nil {
		return err
	}
	if card == nil {
		return fmt.Errorf("billingrepo: rate card version %q not found", version)
	}

	runtimeDoc := map[string]interface{}{
		"id":             activeRateCardDocID,
		"active_version": version,
		"updated_at":     time.Now().UTC(),
	}
	body, err := json.Marshal(runtimeDoc)
	if err != nil {
		return fmt.Errorf("billingrepo: marshal billing runtime doc: %w", err)
	}

	res, err := r.es.Index(
		billingRuntimeIndex,
		strings.NewReader(string(body)),
		r.es.Index.WithDocumentID(activeRateCardDocID),
		r.es.Index.WithContext(ctx),
		r.es.Index.WithRefresh("wait_for"),
	)
	if err != nil {
		return fmt.Errorf("billingrepo: set active rate card version: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("billingrepo: set active rate card version error: %s", res.String())
	}

	r.logger.Info("Activated billing rate card version", zap.String("version", version))
	return nil
}

func (r *ESRateCardRepo) getActiveVersion(ctx context.Context) (string, error) {
	res, err := r.es.Get(
		billingRuntimeIndex,
		activeRateCardDocID,
		r.es.Get.WithContext(ctx),
	)
	if err != nil {
		return "", fmt.Errorf("billingrepo: get active rate card runtime doc: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return "", nil
	}
	if res.IsError() {
		return "", fmt.Errorf("billingrepo: get active rate card runtime doc error: %s", res.String())
	}

	var doc struct {
		Source struct {
			ActiveVersion string `json:"active_version"`
		} `json:"_source"`
	}
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		return "", fmt.Errorf("billingrepo: decode active rate card runtime doc: %w", err)
	}
	return doc.Source.ActiveVersion, nil
}
