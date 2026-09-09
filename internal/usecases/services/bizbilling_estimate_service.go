package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// BizEstimateService manages the estimate/quote lifecycle:
// draft → sent → accepted/rejected/expired → converted (to invoice).
type BizEstimateService struct {
	stores       *bizbillingrepo.Stores
	configSvc    *BizConfigService
	lifecycleSvc *BizLifecycleService
	portalSvc    *BizPortalService
	notifier     *BizNotifier
	logger       *zap.Logger
}

// NewBizEstimateService creates a new BizEstimateService.
func NewBizEstimateService(stores *bizbillingrepo.Stores, configSvc *BizConfigService, lifecycleSvc *BizLifecycleService, portalSvc *BizPortalService, notifier *BizNotifier, logger *zap.Logger) *BizEstimateService {
	return &BizEstimateService{
		stores:       stores,
		configSvc:    configSvc,
		lifecycleSvc: lifecycleSvc,
		portalSvc:    portalSvc,
		notifier:     notifier,
		logger:       logger,
	}
}

// Create creates a draft estimate with computed totals.
func (s *BizEstimateService) Create(ctx context.Context, appID string, req *bizbilling.CreateEstimateRequest) (*bizbilling.Estimate, error) {
	if len(req.LineItems) == 0 {
		return nil, errors.BadRequest("at least one line item is required")
	}

	taxCfg, err := s.configSvc.TaxConfig(ctx, appID)
	if err != nil {
		return nil, err
	}
	totals := bizbilling.ComputeInvoiceTotals(req.LineItems, taxCfg, "")

	number, err := s.configSvc.NextNumber(ctx, appID, NumberKindEstimate)
	if err != nil {
		return nil, err
	}

	currency := req.Currency
	if currency == "" {
		currency = "INR"
	}

	now := time.Now().UTC()
	est := &bizbilling.Estimate{
		ID:             uuid.New().String(),
		AppID:          appID,
		UserID:         req.UserID,
		EstimateNumber: number,
		Status:         bizbilling.EstimateStatusDraft,
		LineItems:      req.LineItems,
		SubtotalPaisa:  totals.SubtotalPaisa,
		TaxPaisa:       totals.TaxPaisa,
		TotalPaisa:     totals.TotalPaisa,
		Currency:       currency,
		ValidUntil:     req.ValidUntil,
		Notes:          req.Notes,
		Terms:          req.Terms,
		Metadata:       req.Metadata,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.stores.Estimates.Create(ctx, est.ID, est); err != nil {
		return nil, err
	}
	return est, nil
}

// Get returns an estimate scoped to the app.
func (s *BizEstimateService) Get(ctx context.Context, appID, id string) (*bizbilling.Estimate, error) {
	est, err := s.stores.Estimates.Get(ctx, id)
	if err != nil || est.AppID != appID {
		return nil, errors.NotFound("estimate", id)
	}
	return est, nil
}

// List lists estimates for an app.
func (s *BizEstimateService) List(ctx context.Context, filter bizbilling.EstimateFilter) ([]*bizbilling.Estimate, int64, error) {
	body := bizbillingrepo.NewQuery(filter.AppID).
		Term("user_id", filter.UserID).
		Term("status", string(filter.Status)).
		Body("created_at", filter.Limit, filter.Offset)
	return s.stores.Estimates.Find(ctx, body)
}

// Update updates a draft estimate and recomputes totals.
func (s *BizEstimateService) Update(ctx context.Context, appID, id string, req *bizbilling.UpdateEstimateRequest) (*bizbilling.Estimate, error) {
	taxCfg, err := s.configSvc.TaxConfig(ctx, appID)
	if err != nil {
		return nil, err
	}

	return s.stores.Estimates.Update(ctx, id, func(est *bizbilling.Estimate) error {
		if est.AppID != appID {
			return errors.NotFound("estimate", id)
		}
		if est.Status != bizbilling.EstimateStatusDraft {
			return errors.BadRequest("only draft estimates can be updated")
		}

		if req.LineItems != nil {
			est.LineItems = req.LineItems
			totals := bizbilling.ComputeInvoiceTotals(est.LineItems, taxCfg, "")
			est.SubtotalPaisa = totals.SubtotalPaisa
			est.TaxPaisa = totals.TaxPaisa
			est.TotalPaisa = totals.TotalPaisa
		}
		if req.ValidUntil != nil {
			est.ValidUntil = req.ValidUntil
		}
		if req.Notes != nil {
			est.Notes = *req.Notes
		}
		if req.Terms != nil {
			est.Terms = *req.Terms
		}
		if req.Metadata != nil {
			est.Metadata = req.Metadata
		}
		est.UpdatedAt = time.Now().UTC()
		return nil
	})
}

// Delete removes a draft estimate.
func (s *BizEstimateService) Delete(ctx context.Context, appID, id string) error {
	est, err := s.Get(ctx, appID, id)
	if err != nil {
		return err
	}
	if est.Status != bizbilling.EstimateStatusDraft {
		return errors.BadRequest("only draft estimates can be deleted")
	}
	return s.stores.Estimates.Delete(ctx, id)
}

// Send transitions draft → sent and delivers the estimate notification with
// a portal link where the customer can accept or reject it.
func (s *BizEstimateService) Send(ctx context.Context, appID, id string) (*bizbilling.Estimate, error) {
	est, err := s.stores.Estimates.Update(ctx, id, func(est *bizbilling.Estimate) error {
		if est.AppID != appID {
			return errors.NotFound("estimate", id)
		}
		if est.Status != bizbilling.EstimateStatusDraft && est.Status != bizbilling.EstimateStatusSent {
			return errors.BadRequest("estimate cannot be sent in its current status")
		}
		est.Status = bizbilling.EstimateStatusSent
		est.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return nil, err
	}

	portalLink := ""
	if token, terr := s.portalSvc.CreateToken(ctx, appID, est.UserID, 0); terr == nil {
		portalLink = s.portalSvc.PortalLink(token.Token)
	}
	validUntil := ""
	if est.ValidUntil != nil {
		validUntil = est.ValidUntil.Format("02 Jan 2006")
	}

	notifID := s.notifier.Send(ctx, appID, est.UserID, "biz_estimate_sent", "", map[string]interface{}{
		"estimate_number": est.EstimateNumber,
		"amount":          FormatPaisa(est.TotalPaisa, est.Currency),
		"valid_until":     validUntil,
		"portal_link":     portalLink,
	})
	if notifID != "" {
		est, _ = s.stores.Estimates.Update(ctx, id, func(e *bizbilling.Estimate) error {
			e.NotificationID = notifID
			return nil
		})
	}
	return est, nil
}

// SetDecision records the customer's accept/reject decision (portal or API).
func (s *BizEstimateService) SetDecision(ctx context.Context, appID, userID, id string, accepted bool) (*bizbilling.Estimate, error) {
	return s.stores.Estimates.Update(ctx, id, func(est *bizbilling.Estimate) error {
		if est.AppID != appID {
			return errors.NotFound("estimate", id)
		}
		if userID != "" && est.UserID != userID {
			return errors.NotFound("estimate", id)
		}
		if est.Status != bizbilling.EstimateStatusSent {
			return errors.BadRequest("only sent estimates can be accepted or rejected")
		}
		if est.ValidUntil != nil && time.Now().UTC().After(*est.ValidUntil) {
			est.Status = bizbilling.EstimateStatusExpired
			return errors.BadRequest("estimate has expired")
		}

		now := time.Now().UTC()
		if accepted {
			est.Status = bizbilling.EstimateStatusAccepted
			est.AcceptedAt = &now
		} else {
			est.Status = bizbilling.EstimateStatusRejected
			est.RejectedAt = &now
		}
		est.UpdatedAt = now
		return nil
	})
}

// Convert turns an accepted estimate into an issued invoice. lineIndexes
// optionally selects a subset of line items (partial conversion).
func (s *BizEstimateService) Convert(ctx context.Context, appID, id string, lineIndexes []int) (*bizbilling.BizInvoice, error) {
	est, err := s.Get(ctx, appID, id)
	if err != nil {
		return nil, err
	}
	if est.Status != bizbilling.EstimateStatusAccepted {
		return nil, errors.BadRequest("only accepted estimates can be converted")
	}

	items := est.LineItems
	if len(lineIndexes) > 0 {
		items = nil
		for _, idx := range lineIndexes {
			if idx < 0 || idx >= len(est.LineItems) {
				return nil, errors.BadRequest("invalid line item index")
			}
			items = append(items, est.LineItems[idx])
		}
	}

	inv, err := s.lifecycleSvc.CreateOneTimeInvoice(ctx, appID, &bizbilling.CreateInvoiceRequest{
		UserID:    est.UserID,
		LineItems: items,
		Currency:  est.Currency,
		Notes:     "Converted from estimate " + est.EstimateNumber,
	})
	if err != nil {
		return nil, err
	}

	// Link the estimate on the invoice, then issue it.
	inv, err = s.lifecycleSvc.updateOwnedInvoice(ctx, appID, inv.ID, func(i *bizbilling.BizInvoice) error {
		i.EstimateID = est.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	inv, err = s.lifecycleSvc.IssueInvoice(ctx, appID, inv.ID)
	if err != nil {
		return nil, err
	}

	if _, err := s.stores.Estimates.Update(ctx, id, func(e *bizbilling.Estimate) error {
		e.Status = bizbilling.EstimateStatusConverted
		e.ConvertedInvoiceID = inv.ID
		e.UpdatedAt = time.Now().UTC()
		return nil
	}); err != nil {
		s.logger.Warn("bizbilling: failed to mark estimate converted",
			zap.String("estimate_id", id), zap.Error(err))
	}
	return inv, nil
}

// ExpireStale marks sent estimates past their validity as expired.
// Called by the billing scheduler.
func (s *BizEstimateService) ExpireStale(ctx context.Context) int {
	now := time.Now().UTC()
	stale, _, err := s.stores.Estimates.Find(ctx, map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"status": string(bizbilling.EstimateStatusSent)}},
					{"range": map[string]interface{}{"valid_until": map[string]interface{}{"lt": now}}},
				},
			},
		},
		"size": 100,
	})
	if err != nil {
		s.logger.Warn("bizbilling: estimate expiry scan failed", zap.Error(err))
		return 0
	}

	expired := 0
	for _, est := range stale {
		if _, err := s.stores.Estimates.Update(ctx, est.ID, func(e *bizbilling.Estimate) error {
			e.Status = bizbilling.EstimateStatusExpired
			e.UpdatedAt = now
			return nil
		}); err == nil {
			expired++
		}
	}
	return expired
}
