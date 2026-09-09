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

// BizContractService manages contracts: creation, digital acceptance,
// amendments, renewals, terminations, and expiry reminders.
type BizContractService struct {
	contracts *bizbillingrepo.Store[bizbilling.Contract]
	configSvc *BizConfigService
	portalSvc *BizPortalService
	notifier  *BizNotifier
	logger    *zap.Logger
}

// NewBizContractService creates a new BizContractService.
func NewBizContractService(stores *bizbillingrepo.Stores, configSvc *BizConfigService, portalSvc *BizPortalService, notifier *BizNotifier, logger *zap.Logger) *BizContractService {
	return &BizContractService{
		contracts: stores.Contracts,
		configSvc: configSvc,
		portalSvc: portalSvc,
		notifier:  notifier,
		logger:    logger,
	}
}

// Create creates a draft contract.
func (s *BizContractService) Create(ctx context.Context, appID string, req *bizbilling.CreateContractRequest) (*bizbilling.Contract, error) {
	if !req.EndDate.After(req.StartDate) {
		return nil, errors.BadRequest("end_date must be after start_date")
	}

	number, err := s.configSvc.NextNumber(ctx, appID, NumberKindContract)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	contract := &bizbilling.Contract{
		ID:             uuid.New().String(),
		AppID:          appID,
		UserID:         req.UserID,
		SubscriptionID: req.SubscriptionID,
		ContractNumber: number,
		Status:         bizbilling.ContractStatusDraft,
		Terms:          req.Terms,
		ValuePaisa:     req.ValuePaisa,
		StartDate:      req.StartDate,
		EndDate:        req.EndDate,
		AutoRenew:      req.AutoRenew,
		Metadata:       req.Metadata,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.contracts.Create(ctx, contract.ID, contract); err != nil {
		return nil, err
	}
	return contract, nil
}

// Get returns a contract scoped to the app.
func (s *BizContractService) Get(ctx context.Context, appID, id string) (*bizbilling.Contract, error) {
	contract, err := s.contracts.Get(ctx, id)
	if err != nil || contract.AppID != appID {
		return nil, errors.NotFound("contract", id)
	}
	return contract, nil
}

// List lists contracts for an app.
func (s *BizContractService) List(ctx context.Context, filter bizbilling.ContractFilter) ([]*bizbilling.Contract, int64, error) {
	body := bizbillingrepo.NewQuery(filter.AppID).
		Term("user_id", filter.UserID).
		Term("status", string(filter.Status)).
		Body("created_at", filter.Limit, filter.Offset)
	return s.contracts.Find(ctx, body)
}

// Update updates a draft contract.
func (s *BizContractService) Update(ctx context.Context, appID, id string, req *bizbilling.UpdateContractRequest) (*bizbilling.Contract, error) {
	return s.contracts.Update(ctx, id, func(c *bizbilling.Contract) error {
		if c.AppID != appID {
			return errors.NotFound("contract", id)
		}
		if c.Status != bizbilling.ContractStatusDraft {
			return errors.BadRequest("only draft contracts can be updated — use amendments for active contracts")
		}
		if req.Terms != nil {
			c.Terms = *req.Terms
		}
		if req.ValuePaisa != nil {
			c.ValuePaisa = *req.ValuePaisa
		}
		if req.StartDate != nil {
			c.StartDate = *req.StartDate
		}
		if req.EndDate != nil {
			c.EndDate = *req.EndDate
		}
		if req.AutoRenew != nil {
			c.AutoRenew = *req.AutoRenew
		}
		c.UpdatedAt = time.Now().UTC()
		return nil
	})
}

// Send delivers the contract for digital acceptance via the portal.
func (s *BizContractService) Send(ctx context.Context, appID, id string) (*bizbilling.Contract, error) {
	contract, err := s.contracts.Update(ctx, id, func(c *bizbilling.Contract) error {
		if c.AppID != appID {
			return errors.NotFound("contract", id)
		}
		if c.Status != bizbilling.ContractStatusDraft && c.Status != bizbilling.ContractStatusSent {
			return errors.BadRequest("contract cannot be sent in its current status")
		}
		c.Status = bizbilling.ContractStatusSent
		c.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return nil, err
	}

	portalLink := ""
	if token, terr := s.portalSvc.CreateToken(ctx, appID, contract.UserID, 0); terr == nil {
		portalLink = s.portalSvc.PortalLink(token.Token)
	}
	notifID := s.notifier.Send(ctx, appID, contract.UserID, "biz_contract_sent", "", map[string]interface{}{
		"contract_number": contract.ContractNumber,
		"portal_link":     portalLink,
	})
	if notifID != "" {
		contract, _ = s.contracts.Update(ctx, id, func(c *bizbilling.Contract) error {
			c.NotificationID = notifID
			return nil
		})
	}
	return contract, nil
}

// Accept records the customer's digital acceptance (portal path).
func (s *BizContractService) Accept(ctx context.Context, appID, userID, id string) (*bizbilling.Contract, error) {
	return s.contracts.Update(ctx, id, func(c *bizbilling.Contract) error {
		if c.AppID != appID {
			return errors.NotFound("contract", id)
		}
		if userID != "" && c.UserID != userID {
			return errors.NotFound("contract", id)
		}
		if c.Status != bizbilling.ContractStatusSent {
			return errors.BadRequest("only sent contracts can be accepted")
		}
		now := time.Now().UTC()
		c.Status = bizbilling.ContractStatusActive
		c.AcceptedAt = &now
		c.UpdatedAt = now
		return nil
	})
}

// Amend appends an amendment to an active contract.
func (s *BizContractService) Amend(ctx context.Context, appID, id string, req *bizbilling.AmendContractRequest) (*bizbilling.Contract, error) {
	return s.contracts.Update(ctx, id, func(c *bizbilling.Contract) error {
		if c.AppID != appID {
			return errors.NotFound("contract", id)
		}
		if c.Status != bizbilling.ContractStatusActive {
			return errors.BadRequest("only active contracts can be amended")
		}
		now := time.Now().UTC()
		c.Amendments = append(c.Amendments, bizbilling.ContractAmendment{
			ID:            uuid.New().String(),
			Description:   req.Description,
			EffectiveDate: req.EffectiveDate,
			CreatedAt:     now,
		})
		c.UpdatedAt = now
		return nil
	})
}

// Renew extends the contract by its original duration (or to newEndDate).
func (s *BizContractService) Renew(ctx context.Context, appID, id string, newEndDate *time.Time) (*bizbilling.Contract, error) {
	return s.contracts.Update(ctx, id, func(c *bizbilling.Contract) error {
		if c.AppID != appID {
			return errors.NotFound("contract", id)
		}
		if c.Status != bizbilling.ContractStatusActive && c.Status != bizbilling.ContractStatusExpired {
			return errors.BadRequest("only active or expired contracts can be renewed")
		}

		now := time.Now().UTC()
		if newEndDate != nil {
			if !newEndDate.After(c.EndDate) {
				return errors.BadRequest("new end date must be after the current end date")
			}
			c.EndDate = *newEndDate
		} else {
			c.EndDate = c.EndDate.Add(c.EndDate.Sub(c.StartDate))
		}
		c.Status = bizbilling.ContractStatusActive
		c.ExpiryNotifiedAt = nil
		c.UpdatedAt = now
		return nil
	})
}

// Terminate ends a contract early.
func (s *BizContractService) Terminate(ctx context.Context, appID, id string) (*bizbilling.Contract, error) {
	return s.contracts.Update(ctx, id, func(c *bizbilling.Contract) error {
		if c.AppID != appID {
			return errors.NotFound("contract", id)
		}
		if c.Status == bizbilling.ContractStatusTerminated {
			return errors.BadRequest("contract is already terminated")
		}
		now := time.Now().UTC()
		c.Status = bizbilling.ContractStatusTerminated
		c.TerminatedAt = &now
		c.UpdatedAt = now
		return nil
	})
}

// ProcessExpiries sends expiry reminders (7 days ahead), auto-renews flagged
// contracts, and expires the rest. Called by the billing scheduler.
func (s *BizContractService) ProcessExpiries(ctx context.Context) {
	now := time.Now().UTC()

	// 1. Expiry reminders for contracts ending within 7 days.
	expiring, _, err := s.contracts.Find(ctx, map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"status": string(bizbilling.ContractStatusActive)}},
					{"range": map[string]interface{}{"end_date": map[string]interface{}{
						"gte": now, "lte": now.AddDate(0, 0, 7),
					}}},
				},
			},
		},
		"size": 100,
	})
	if err == nil {
		for _, c := range expiring {
			if c.ExpiryNotifiedAt != nil {
				continue
			}
			s.notifier.Send(ctx, c.AppID, c.UserID, "biz_contract_expiring", "", map[string]interface{}{
				"contract_number": c.ContractNumber,
				"end_date":        c.EndDate.Format("02 Jan 2006"),
			})
			_, _ = s.contracts.Update(ctx, c.ID, func(cc *bizbilling.Contract) error {
				cc.ExpiryNotifiedAt = &now
				return nil
			})
		}
	}

	// 2. Auto-renew or expire contracts past their end date.
	ended, _, err := s.contracts.Find(ctx, map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"status": string(bizbilling.ContractStatusActive)}},
					{"range": map[string]interface{}{"end_date": map[string]interface{}{"lt": now}}},
				},
			},
		},
		"size": 100,
	})
	if err != nil {
		return
	}
	for _, c := range ended {
		if c.AutoRenew {
			if _, err := s.Renew(ctx, c.AppID, c.ID, nil); err != nil {
				s.logger.Warn("bizbilling: contract auto-renew failed",
					zap.String("contract_id", c.ID), zap.Error(err))
			}
			continue
		}
		_, _ = s.contracts.Update(ctx, c.ID, func(cc *bizbilling.Contract) error {
			cc.Status = bizbilling.ContractStatusExpired
			cc.UpdatedAt = now
			return nil
		})
	}
}
