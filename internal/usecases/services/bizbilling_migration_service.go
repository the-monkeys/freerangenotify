package services

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

type MigrationOptions = bizbilling.MigrationOptions
type MigrationReport = bizbilling.MigrationReport
type LegacyResolution = bizbilling.LegacyResolution

type BizMigrationService struct {
	newCollectionEnabled bool
	reviews              *bizbillingrepo.MigrationReviewRepo
	apps                 *bizbillingrepo.Store[struct {
		AppID string `json:"app_id"`
	}]
	gate       *BizLifecycleGate
	operations *BizOperationService
}

func NewBizMigrationService(es *elasticsearch.Client, gate *BizLifecycleGate, operations *BizOperationService, logger *zap.Logger) *BizMigrationService {
	return &BizMigrationService{reviews: bizbillingrepo.NewMigrationReviewRepo(es, logger), apps: bizbillingrepo.NewStore[struct {
		AppID string `json:"app_id"`
	}](es, "applications", logger), gate: gate, operations: operations}
}

func (s *BizMigrationService) Prepare(ctx context.Context, appID string, options MigrationOptions) (*MigrationReport, error) {
	report, _, err := s.inventory(ctx, appID, options.PrimaryCurrency)
	if err != nil {
		return nil, err
	}
	if options.ExpectedReportID != "" && options.ExpectedReportID != report.ID {
		return nil, apperrors.Conflict("Billing sources changed; review a new inventory")
	}
	if options.DryRun {
		return report, nil
	}
	if options.ExpectedReportID == "" {
		return nil, apperrors.BadRequest("A reviewed report is required")
	}
	ctx, done, err := BeginBizMutation(ctx, s.gate, appID)
	if err != nil {
		return nil, err
	}
	defer releaseBizMutation(s.operations.logger, done)
	report.WritersUpgraded = options.WritersUpgraded
	created, err := s.reviews.Reports.CreateIfAbsent(ctx, report.ID, report)
	if err != nil {
		return nil, err
	}
	if created {
		return report, nil
	}
	stored, err := s.reviews.Reports.Update(ctx, report.ID, func(current *MigrationReport) error {
		if current.AppID != appID || current.PrimaryCurrency != report.PrimaryCurrency {
			return bizbilling.ErrReconciliationRequired
		}
		checkpoint := current.Checkpoint
		attested := current.WritersUpgraded
		*current = *report
		current.WritersUpgraded = current.WritersUpgraded || attested
		if checkpoint == "facts_started" || checkpoint == "activated" {
			current.Checkpoint = checkpoint
		}
		return nil
	})
	return stored, err
}

type migrationHeader struct {
	AppID            string   `json:"app_id"`
	UserID           string   `json:"user_id"`
	BilledToUser     string   `json:"billed_to_user"`
	Currency         string   `json:"currency"`
	BalanceCurrency  string   `json:"biz_balance_currency"`
	Balance          int64    `json:"balance_paisa"`
	Amount           int64    `json:"amount_paisa"`
	Billable         bool     `json:"billable"`
	Status           string   `json:"status"`
	InvoiceID        string   `json:"invoice_id"`
	InvoiceIDs       []string `json:"invoice_ids"`
	SubscriptionID   string   `json:"subscription_id"`
	GatewayOrderID   string   `json:"gateway_order_id"`
	GatewayPaymentID string   `json:"gateway_payment_id"`
	ExcessCredit     int64    `json:"excess_credit_paisa"`
	CurrentUses      int64    `json:"current_uses"`
	MaxUsesPerUser   int      `json:"max_uses_per_user"`
}

func (s *BizMigrationService) inventory(ctx context.Context, appID, currency string) (*MigrationReport, []bizbillingrepo.MigrationSource, error) {
	if err := (bizbilling.Scope{AppID: appID, UserID: "inventory", Currency: currency}).Validate(); err != nil {
		return nil, nil, apperrors.BadRequest("An application and primary currency are required")
	}
	app, err := s.apps.Get(ctx, appID)
	if err != nil {
		return nil, nil, err
	}
	if app.AppID != appID {
		return nil, nil, bizbilling.ErrReconciliationRequired
	}
	sources, err := s.reviews.Inventory(ctx, appID)
	if err != nil {
		return nil, nil, err
	}
	report := &MigrationReport{AppID: appID, PrimaryCurrency: currency, SourceHashes: map[string]string{}, ResourceCounts: map[string]int64{}, Issues: []bizbilling.MigrationIssue{}, LegacyGatewayBindings: []bizbilling.ResourceRef{}, OpeningBalances: []bizbilling.MigrationOpening{}, Checkpoint: "review"}
	users := map[string]bool{}
	invoices := map[string]bool{}
	decisions := map[string]bizbilling.LegacyReviewDecision{}
	normalized := map[string]migrationHeader{}
	subscriptions := map[string]bool{}
	for _, source := range sources {
		report.SourceHashes[source.Index+"/"+source.ID] = source.Hash
		report.ResourceCounts[source.Index]++
		if source.Index == "users" {
			users[source.ID] = true
		}
		if source.Index == "frn_biz_invoices" {
			invoices[source.ID] = true
		}
		if source.Index == "frn_biz_subscriptions" {
			subscriptions[source.ID] = true
		}
	}
	issue := func(source bizbillingrepo.MigrationSource, code string) {
		report.Issues = append(report.Issues, bizbilling.MigrationIssue{ID: bizbillingrepo.MigrationDigest([]string{source.Index, source.ID, code, source.Hash}), Index: source.Index, ResourceID: source.ID, Code: code, SourceHash: source.Hash})
	}
	for _, source := range sources {
		var value migrationHeader
		if err = json.Unmarshal(source.Source, &value); err != nil {
			return nil, nil, err
		}
		if value.AppID != appID {
			return nil, nil, bizbilling.ErrReconciliationRequired
		}
		if value.UserID == "" {
			value.UserID = value.BilledToUser
		}
		if source.Index == "users" {
			value.UserID = source.ID
			value.Currency = value.BalanceCurrency
		}
		isMoney := source.Index == "users" || source.Index == "frn_biz_invoices" || source.Index == "frn_biz_payments" || source.Index == bizbillingrepo.CreditNoteIndex || source.Index == bizbillingrepo.RetainerIndex || source.Index == bizbillingrepo.EstimateIndex || source.Index == bizbillingrepo.ExpenseIndex
		if isMoney && value.Currency == "" && (source.Index != "users" || value.Balance != 0) {
			id := bizbillingrepo.LegacyDecisionID(appID, source.Index, source.ID, source.ContentHash)
			decision, readErr := s.reviews.Decisions.Get(ctx, id)
			if readErr != nil && !apperrors.IsNotFound(readErr) {
				return nil, nil, readErr
			}
			if decision != nil && decision.AppID == appID && decision.Index == source.Index && decision.ResourceID == source.ID && decision.Resolution.ExpectedSourceHash == source.Hash && decision.SourceContentHash == source.ContentHash {
				value.Currency = decision.Resolution.Currency
				decisions[id] = *decision
			} else {
				issue(source, "currency_required")
			}
		}
		if isMoney && value.UserID != "" && !users[value.UserID] {
			issue(source, "customer_reference_missing")
		}
		if isMoney && source.Index != "users" && (source.Index != bizbillingrepo.ExpenseIndex || value.Billable) && value.UserID == "" {
			issue(source, "customer_reference_missing")
		}
		if isMoney && value.Currency != "" {
			if err := (bizbilling.Scope{AppID: appID, UserID: "inventory", Currency: value.Currency}).Validate(); err != nil {
				issue(source, "invalid_currency")
			}
		}
		normalized[source.Index+"/"+source.ID] = value
		if source.Index == bizbillingrepo.CreditNoteIndex || source.Index == bizbillingrepo.RetainerIndex {
			if value.Amount < 0 || value.Balance < 0 || value.Balance > value.Amount {
				issue(source, "credit_source_totals_need_review")
			}
		}
		if value.SubscriptionID != "" && !subscriptions[value.SubscriptionID] {
			issue(source, "subscription_reference_missing")
		}
		if source.Index == "users" {
			if value.Balance < 0 {
				issue(source, "negative_legacy_balance")
			} else if value.Currency != "" || value.Balance == 0 {
				selected := value.Currency
				if selected == "" {
					selected = currency
				}
				report.OpeningBalances = append(report.OpeningBalances, bizbilling.MigrationOpening{Scope: bizbilling.Scope{AppID: appID, UserID: source.ID, Currency: selected}, AvailablePaisa: value.Balance})
			}
		}
		if source.Index == "frn_biz_invoices" {
			var inv bizbilling.BizInvoice
			if err = json.Unmarshal(source.Source, &inv); err != nil {
				return nil, nil, err
			}
			if invoiceAccountingInvariant(&inv) != nil {
				issue(source, "invoice_totals_need_review")
			}
		}
		if value.InvoiceID != "" && !invoices[value.InvoiceID] {
			issue(source, "invoice_reference_missing")
		}
		for _, id := range value.InvoiceIDs {
			if !invoices[id] {
				issue(source, "invoice_reference_missing")
			}
		}
		if value.GatewayOrderID != "" || value.GatewayPaymentID != "" {
			report.LegacyGatewayBindings = append(report.LegacyGatewayBindings, bizbilling.ResourceRef{Kind: source.Index, ID: source.ID})
			issue(source, "legacy_gateway_adapter_required")
		}
		if source.Index == "frn_biz_subscriptions" && value.Status != string(bizbilling.SubscriptionStatusCanceled) {
			issue(source, "subscription_lifecycle_not_ready")
		}
		if source.Index == "frn_biz_payments" && value.ExcessCredit > 0 {
			issue(source, "legacy_excess_provenance_requires_reconciliation")
		}
		if source.Index == bizbillingrepo.CouponIndex && value.CurrentUses > 0 && value.MaxUsesPerUser > 0 {
			issue(source, "legacy_coupon_usage_requires_reconciliation")
		}
	}
	for _, source := range sources {
		value := normalized[source.Index+"/"+source.ID]
		refs := append([]string(nil), value.InvoiceIDs...)
		if value.InvoiceID != "" {
			refs = append(refs, value.InvoiceID)
		}
		for _, id := range refs {
			invoice, exists := normalized["frn_biz_invoices/"+id]
			if !exists {
				continue
			}
			if value.UserID != "" && invoice.UserID != value.UserID {
				issue(source, "invoice_owner_mismatch")
			}
			if value.Currency != "" && invoice.Currency != "" && value.Currency != invoice.Currency {
				issue(source, "invoice_currency_mismatch")
			}
		}
	}
	sort.Slice(report.OpeningBalances, func(i, j int) bool {
		a, b := report.OpeningBalances[i].Scope, report.OpeningBalances[j].Scope
		if a.UserID == b.UserID {
			return a.Currency < b.Currency
		}
		return a.UserID < b.UserID
	})
	state, stateErr := s.gate.State(ctx, appID)
	if stateErr != nil {
		return nil, nil, stateErr
	}
	for _, permit := range state.MutationPermits {
		report.WriterPermits = append(report.WriterPermits, permit)
	}
	sort.Slice(report.WriterPermits, func(i, j int) bool { return report.WriterPermits[i].WriterID < report.WriterPermits[j].WriterID })
	report.ID = bizbillingrepo.MigrationDigest([]any{appID, currency, report.SourceHashes})
	report.ResolutionHash = bizbillingrepo.MigrationDigest(decisions)
	report.Ready = len(report.Issues) == 0
	return report, sources, nil
}

func (s *BizMigrationService) Resolve(ctx context.Context, appID, reportID string, decision LegacyResolution) error {
	if strings.TrimSpace(decision.ActorID) == "" || strings.TrimSpace(decision.Reason) == "" || len(decision.Reason) > 1000 || len(decision.ActorID) > 200 {
		return apperrors.BadRequest("A reviewer and reason are required")
	}
	ctx, done, err := BeginBizMutation(ctx, s.gate, appID)
	if err != nil {
		return err
	}
	defer releaseBizMutation(s.operations.logger, done)
	stored, err := s.reviews.Reports.Get(ctx, reportID)
	if err != nil {
		return err
	}
	if stored.AppID != appID {
		return apperrors.NotFound("billing review", reportID)
	}
	if stored.Checkpoint != "review" {
		return apperrors.Conflict("This review has already entered cutover")
	}
	var requested *bizbilling.MigrationIssue
	for i := range stored.Issues {
		if stored.Issues[i].ID == decision.IssueID {
			requested = &stored.Issues[i]
			break
		}
	}
	if requested == nil || requested.Code != "currency_required" || requested.SourceHash != decision.ExpectedSourceHash {
		return apperrors.BadRequest("This issue needs a matching currency review; other inconsistencies require reconciliation")
	}
	if err := (bizbilling.Scope{AppID: appID, UserID: "review", Currency: decision.Currency}).Validate(); err != nil {
		return apperrors.BadRequest("A valid assigned currency is required")
	}
	current, sources, err := s.inventory(ctx, appID, stored.PrimaryCurrency)
	if err != nil {
		return err
	}
	if current.ID != stored.ID {
		return apperrors.Conflict("Billing sources changed; review a new inventory")
	}
	for _, source := range sources {
		if source.Index == requested.Index && source.ID == requested.ResourceID && source.Hash == decision.ExpectedSourceHash {
			record := &bizbilling.LegacyReviewDecision{ID: bizbillingrepo.LegacyDecisionID(appID, source.Index, source.ID, source.ContentHash), AppID: appID, ReportID: reportID, Index: source.Index, ResourceID: source.ID, SourceContentHash: source.ContentHash, Resolution: decision}
			created, err := s.reviews.Decisions.CreateIfAbsent(ctx, record.ID, record)
			if err != nil {
				return err
			}
			if !created {
				previous, err := s.reviews.Decisions.Get(ctx, record.ID)
				if err != nil {
					return err
				}
				if bizbillingrepo.MigrationDigest(previous) != bizbillingrepo.MigrationDigest(record) {
					return bizbilling.ErrIdempotencyConflict
				}
			}
			return nil
		}
	}
	return bizbilling.ErrReconciliationRequired
}

func (s *BizMigrationService) Activate(ctx context.Context, appID, reportID string) error {
	report, err := s.reviews.Reports.Get(ctx, reportID)
	if err != nil {
		return err
	}
	if report.AppID != appID {
		return apperrors.NotFound("billing review", reportID)
	}
	if !report.Ready || !report.WritersUpgraded {
		return apperrors.Conflict("Billing review or writer rollout is incomplete")
	}
	state, err := s.gate.State(ctx, appID)
	if err != nil {
		return err
	}
	if state.LifecycleVersion == 2 {
		if state.MigrationReportID == reportID && state.Status == "active" {
			_, err = s.reviews.Reports.Update(ctx, reportID, func(r *MigrationReport) error { r.Checkpoint = "activated"; return nil })
			return err
		}
		return bizbilling.ErrConflict
	}
	// Disabling new collection stops admission, not an already accepted cutover.
	recovering := state.Status == "preparing" && state.MigrationReportID == reportID
	if !s.newCollectionEnabled && !recovering {
		return apperrors.Conflict("New billing lifecycle activation is disabled for this installation")
	}
	ctx, release, err := s.gate.beginMigration(ctx, appID, reportID)
	if err != nil {
		return err
	}
	defer release()
	stopRenewing := keepBillingLease(ctx, 20*time.Second, func(renewCtx context.Context) error { return s.gate.renewMigration(renewCtx, appID) })
	defer stopRenewing()
	if err = s.reviews.RefreshSources(ctx); err != nil {
		return err
	}
	fresh, _, err := s.inventory(ctx, appID, report.PrimaryCurrency)
	if err != nil {
		return err
	}
	if fresh.ID != report.ID || fresh.ResolutionHash != report.ResolutionHash || !fresh.Ready {
		return apperrors.Conflict("Billing sources or review decisions changed; prepare a new reviewed report")
	}
	pending, err := s.operations.operations.PendingForApp(ctx, appID)
	if err != nil {
		return err
	}
	for _, op := range pending {
		if op.Command.Kind != bizbilling.CommandOpeningBalance || !strings.HasPrefix(op.Command.IdempotencyKey, "migration:"+reportID+":") {
			return bizbilling.ErrActiveLegacyWriter
		}
	}
	if err = s.gate.startMigrationFacts(ctx, appID); err != nil {
		return err
	}
	_, err = s.reviews.Reports.Update(ctx, reportID, func(r *MigrationReport) error {
		if r.AppID != appID {
			return bizbilling.ErrConflict
		}
		r.Checkpoint = "facts_started"
		return nil
	})
	if err != nil {
		return err
	}
	for _, opening := range report.OpeningBalances {
		if err = s.gate.renewMigration(ctx, appID); err != nil {
			return err
		}
		key := "migration:" + reportID + ":" + bizbillingrepo.ScopeID(opening.Scope)
		result, applyErr := s.operations.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandOpeningBalance, Scope: opening.Scope, IdempotencyKey: key, ActorID: "reviewed-migration", OpeningBalance: &bizbilling.OpeningBalance{AvailablePaisa: opening.AvailablePaisa}})
		if applyErr != nil {
			return &bizbilling.OperationFailure{Result: result, Cause: applyErr}
		}
		balance, err := s.operations.Balance(ctx, opening.Scope)
		if err != nil {
			return err
		}
		if balance.LastSequence != 1 || balance.AvailablePaisa != opening.AvailablePaisa || balance.ReservedPaisa != 0 {
			return bizbilling.ErrReconciliationRequired
		}
	}
	last, _, err := s.inventory(ctx, appID, report.PrimaryCurrency)
	if err != nil {
		return err
	}
	if last.ID != report.ID || last.ResolutionHash != report.ResolutionHash {
		return bizbilling.ErrReconciliationRequired
	}
	if err = s.gate.completeMigration(ctx, appID, report.PrimaryCurrency); err != nil {
		return err
	}
	_, err = s.reviews.Reports.Update(ctx, reportID, func(r *MigrationReport) error { r.Checkpoint = "activated"; return nil })
	return err
}

func (s *BizMigrationService) SetNewCollectionEnabled(enabled bool) { s.newCollectionEnabled = enabled }

func (s *BizMigrationService) Lifecycle(ctx context.Context, appID string) (bizbilling.LifecycleState, bool, error) {
	state, err := s.gate.State(ctx, appID)
	return state, s.newCollectionEnabled, err
}
func (s *BizMigrationService) Reviews(ctx context.Context, appID, after string, limit int) ([]map[string]interface{}, string, error) {
	if len(after) > 200 {
		return nil, "", apperrors.BadRequest("Invalid review cursor")
	}
	if limit < 1 || limit > 25 {
		limit = 10
	}
	q := bizbillingrepo.NewQuery(appID)
	body := q.Body("id", limit, 0)
	body["sort"] = []interface{}{map[string]interface{}{"id": "asc"}}
	if after != "" {
		body["query"] = map[string]interface{}{"bool": map[string]interface{}{"filter": []interface{}{q.Bool(), map[string]interface{}{"range": map[string]interface{}{"id": map[string]interface{}{"gt": after}}}}}}
	}
	body["_source"] = map[string]interface{}{"excludes": []string{"source_hashes", "opening_balances"}}
	rows, _, err := s.reviews.Reports.Find(ctx, body)
	if err != nil {
		return nil, "", err
	}
	result := make([]map[string]interface{}, 0, len(rows))
	next := ""
	for _, r := range rows {
		if r.AppID != appID {
			return nil, "", bizbilling.ErrReconciliationRequired
		}
		result = append(result, map[string]interface{}{"id": r.ID, "app_id": r.AppID, "primary_currency": r.PrimaryCurrency, "ready": r.Ready, "writers_upgraded": r.WritersUpgraded, "checkpoint": r.Checkpoint, "issues": r.Issues, "resource_counts": r.ResourceCounts})
	}
	if len(rows) == limit {
		next = rows[len(rows)-1].ID
	}
	return result, next, nil
}

func (s *BizMigrationService) ReviewedReport(ctx context.Context, appID, reportID string) (*MigrationReport, error) {
	r, err := s.reviews.Reports.Get(ctx, reportID)
	if err != nil {
		return nil, err
	}
	if r.AppID != appID {
		return nil, apperrors.NotFound("billing review", reportID)
	}
	return r, nil
}
