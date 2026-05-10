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

const creditBalanceIndex = "frn_credit_balances"

type ESCreditBalanceRepo struct {
	es     *elasticsearch.Client
	logger *zap.Logger
}

func NewESCreditBalanceRepo(es *elasticsearch.Client, logger *zap.Logger) *ESCreditBalanceRepo {
	return &ESCreditBalanceRepo{es: es, logger: logger}
}

func (r *ESCreditBalanceRepo) GetByTenantID(ctx context.Context, tenantID string) (*billing.CreditBalance, error) {
	if tenantID == "" {
		return nil, nil
	}

	res, err := r.es.Get(
		creditBalanceIndex,
		tenantID,
		r.es.Get.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("billingrepo: get credit balance: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, nil
	}
	if res.IsError() {
		return nil, fmt.Errorf("billingrepo: get credit balance error: %s", res.String())
	}

	var doc struct {
		Source billing.CreditBalance `json:"_source"`
	}
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("billingrepo: decode credit balance: %w", err)
	}
	if doc.Source.ID == "" {
		doc.Source.ID = tenantID
	}
	if doc.Source.TenantID == "" {
		doc.Source.TenantID = tenantID
	}
	return &doc.Source, nil
}

func (r *ESCreditBalanceRepo) Upsert(ctx context.Context, balance *billing.CreditBalance) error {
	if balance == nil {
		return fmt.Errorf("billingrepo: nil credit balance")
	}
	if balance.TenantID == "" {
		return fmt.Errorf("billingrepo: tenant_id is required")
	}
	if balance.ID == "" {
		balance.ID = balance.TenantID
	}

	now := time.Now().UTC()
	if balance.CreatedAt.IsZero() {
		balance.CreatedAt = now
	}
	balance.UpdatedAt = now

	body, err := json.Marshal(balance)
	if err != nil {
		return fmt.Errorf("billingrepo: marshal credit balance: %w", err)
	}

	res, err := r.es.Index(
		creditBalanceIndex,
		strings.NewReader(string(body)),
		r.es.Index.WithDocumentID(balance.ID),
		r.es.Index.WithContext(ctx),
		r.es.Index.WithRefresh("wait_for"),
	)
	if err != nil {
		return fmt.Errorf("billingrepo: upsert credit balance: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("billingrepo: upsert credit balance error: %s", res.String())
	}

	r.logger.Debug("Upserted credit balance",
		zap.String("tenant_id", balance.TenantID),
		zap.Int64("credits_remaining", balance.CreditsRemaining))

	return nil
}
