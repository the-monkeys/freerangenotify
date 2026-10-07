package billingrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"go.uber.org/zap"
)

const creditLedgerIndex = "credit_ledger"

type ESCreditLedgerRepo struct {
	es     *elasticsearch.Client
	logger *zap.Logger
}

func NewESCreditLedgerRepo(es *elasticsearch.Client, logger *zap.Logger) *ESCreditLedgerRepo {
	return &ESCreditLedgerRepo{es: es, logger: logger}
}

func (r *ESCreditLedgerRepo) Append(ctx context.Context, entry *billing.CreditLedgerEntry) error {
	if entry == nil {
		return fmt.Errorf("billingrepo: nil credit ledger entry")
	}
	if entry.TenantID == "" {
		return fmt.Errorf("billingrepo: tenant_id is required")
	}
	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}

	body, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("billingrepo: marshal credit ledger entry: %w", err)
	}

	res, err := r.es.Index(
		creditLedgerIndex,
		strings.NewReader(string(body)),
		r.es.Index.WithDocumentID(entry.ID),
		r.es.Index.WithOpType("create"),
		r.es.Index.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("billingrepo: append credit ledger entry: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == 409 {
		// A deterministic repair may repeat only the identical immutable row.
		existing, err := r.es.Get(creditLedgerIndex, entry.ID, r.es.Get.WithContext(ctx))
		if err != nil {
			return err
		}
		defer existing.Body.Close()
		if existing.IsError() {
			return fmt.Errorf("billingrepo: read existing ledger repair: %s", existing.String())
		}
		var raw struct {
			Source json.RawMessage `json:"_source"`
		}
		if err := json.NewDecoder(existing.Body).Decode(&raw); err != nil {
			return err
		}
		var want, got interface{}
		decoder := json.NewDecoder(strings.NewReader(string(body)))
		decoder.UseNumber()
		if err := decoder.Decode(&want); err != nil {
			return err
		}
		decoder = json.NewDecoder(strings.NewReader(string(raw.Source)))
		decoder.UseNumber()
		if err := decoder.Decode(&got); err != nil {
			return err
		}
		if !reflect.DeepEqual(want, got) {
			return fmt.Errorf("billingrepo: conflicting immutable credit ledger row %s", entry.ID)
		}
		return nil
	}

	if res.IsError() {
		return fmt.Errorf("billingrepo: append credit ledger entry error: %s", res.String())
	}

	if r.logger != nil {
		r.logger.Debug("Appended credit ledger entry",
			zap.String("tenant_id", entry.TenantID),
			zap.String("entry_type", string(entry.EntryType)),
			zap.Int64("credits_delta", entry.CreditsDelta))
	}

	return nil
}

func (r *ESCreditLedgerRepo) ListByTenantID(ctx context.Context, tenantID string, limit int) ([]billing.CreditLedgerEntry, error) {
	if tenantID == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	query := fmt.Sprintf(`{
		"size": %d,
		"sort": [{ "created_at": { "order": "desc" } }],
		"query": {
			"bool": {
				"filter": [
					{ "term": { "tenant_id": %q } }
				]
			}
		}
	}`, limit, tenantID)

	res, err := r.es.Search(
		r.es.Search.WithIndex(creditLedgerIndex),
		r.es.Search.WithBody(strings.NewReader(query)),
		r.es.Search.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("billingrepo: list credit ledger entries: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("billingrepo: list credit ledger entries error: %s", res.String())
	}

	var raw struct {
		Hits struct {
			Hits []struct {
				Source billing.CreditLedgerEntry `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("billingrepo: decode credit ledger entries: %w", err)
	}

	entries := make([]billing.CreditLedgerEntry, 0, len(raw.Hits.Hits))
	for _, h := range raw.Hits.Hits {
		entries = append(entries, h.Source)
	}
	return entries, nil
}
