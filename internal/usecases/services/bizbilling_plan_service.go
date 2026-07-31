package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// BizPlanService implements business logic for managing plans and addons
// in the business billing module.
type BizPlanService struct {
	planRepo    bizbilling.PlanRepository
	addonRepo   bizbilling.PlanAddonRepository
	productRepo bizbilling.ProductRepository
	logger      *zap.Logger
}

// NewBizPlanService creates a new BizPlanService.
func NewBizPlanService(
	planRepo bizbilling.PlanRepository,
	addonRepo bizbilling.PlanAddonRepository,
	productRepo bizbilling.ProductRepository,
	logger *zap.Logger,
) *BizPlanService {
	return &BizPlanService{
		planRepo:    planRepo,
		addonRepo:   addonRepo,
		productRepo: productRepo,
		logger:      logger,
	}
}

// GetAddon returns a single addon scoped to the app.
func (s *BizPlanService) GetAddon(ctx context.Context, appID, addonID string) (*bizbilling.PlanAddon, error) {
	addon, err := s.addonRepo.GetByID(ctx, addonID)
	if err != nil || addon.AppID != appID {
		return nil, errors.NotFound("addon", addonID)
	}
	return addon, nil
}

// Create creates a new pricing plan.
func (s *BizPlanService) Create(ctx context.Context, appID string, req bizbilling.CreatePlanRequest) (*bizbilling.Plan, error) {
	if appID == "" {
		return nil, errors.BadRequest("app_id is required")
	}
	if req.Name == "" {
		return nil, errors.BadRequest("plan name is required")
	}
	if req.ProductID == "" {
		return nil, errors.BadRequest("product_id is required")
	}

	// Verify product exists
	product, err := s.productRepo.GetByID(ctx, req.ProductID)
	if err != nil {
		return nil, errors.NotFound("product", req.ProductID)
	}
	if product.AppID != appID {
		return nil, errors.NotFound("product", req.ProductID)
	}

	// Validate billing cycle
	if !bizbilling.ValidBillingCycles[req.BillingCycle] {
		return nil, errors.BadRequest("invalid billing_cycle; valid values: weekly, monthly, quarterly, half_yearly, yearly, custom")
	}
	if req.BillingCycle == bizbilling.BillingCycleCustom && req.CycleDays <= 0 {
		return nil, errors.BadRequest("cycle_days is required when billing_cycle is custom")
	}

	// Validate pricing model
	pricingModel := req.PricingModel
	if pricingModel == "" {
		pricingModel = bizbilling.PricingModelFlat
	}
	if !bizbilling.ValidPricingModels[pricingModel] {
		return nil, errors.BadRequest("invalid pricing_model; valid values: flat, per_unit, tiered, volume")
	}

	// For flat and per_unit, amount is required
	if (pricingModel == bizbilling.PricingModelFlat || pricingModel == bizbilling.PricingModelPerUnit) && req.AmountPaisa <= 0 {
		return nil, errors.BadRequest("amount_paisa must be greater than 0 for flat and per_unit pricing")
	}

	// For tiered and volume, tiers are required
	if (pricingModel == bizbilling.PricingModelTiered || pricingModel == bizbilling.PricingModelVolume) && len(req.Tiers) == 0 {
		return nil, errors.BadRequest("tiers are required for tiered and volume pricing")
	}

	currency := req.Currency
	if currency == "" {
		currency = "INR"
	}

	now := time.Now().UTC()
	plan := &bizbilling.Plan{
		ID:            uuid.New().String(),
		AppID:         appID,
		ProductID:     req.ProductID,
		Name:          req.Name,
		AmountPaisa:   req.AmountPaisa,
		Currency:      currency,
		BillingCycle:  req.BillingCycle,
		CycleDays:     req.CycleDays,
		TrialDays:     req.TrialDays,
		SetupFeePaisa: req.SetupFeePaisa,
		TaxInclusive:  req.TaxInclusive,
		PricingModel:  pricingModel,
		Tiers:         req.Tiers,
		Active:        true,
		Metadata:      req.Metadata,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.planRepo.Create(ctx, plan); err != nil {
		s.logger.Error("Failed to create plan",
			zap.String("app_id", appID),
			zap.String("name", req.Name),
			zap.Error(err))
		return nil, err
	}

	s.logger.Info("Plan created",
		zap.String("plan_id", plan.ID),
		zap.String("app_id", appID),
		zap.String("product_id", plan.ProductID),
		zap.String("name", plan.Name))

	return plan, nil
}

// GetByID retrieves a plan by ID.
func (s *BizPlanService) GetByID(ctx context.Context, id string) (*bizbilling.Plan, error) {
	return s.planRepo.GetByID(ctx, id)
}

// Update updates a plan.
func (s *BizPlanService) Update(ctx context.Context, id string, req bizbilling.UpdatePlanRequest) (*bizbilling.Plan, error) {
	plan, err := s.planRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		plan.Name = *req.Name
	}
	if req.AmountPaisa != nil {
		plan.AmountPaisa = *req.AmountPaisa
	}
	if req.Currency != nil {
		plan.Currency = *req.Currency
	}
	if req.BillingCycle != nil {
		if !bizbilling.ValidBillingCycles[*req.BillingCycle] {
			return nil, errors.BadRequest("invalid billing_cycle")
		}
		plan.BillingCycle = *req.BillingCycle
	}
	if req.CycleDays != nil {
		plan.CycleDays = *req.CycleDays
	}
	if req.TrialDays != nil {
		plan.TrialDays = *req.TrialDays
	}
	if req.SetupFeePaisa != nil {
		plan.SetupFeePaisa = *req.SetupFeePaisa
	}
	if req.TaxInclusive != nil {
		plan.TaxInclusive = *req.TaxInclusive
	}
	if req.PricingModel != nil {
		if !bizbilling.ValidPricingModels[*req.PricingModel] {
			return nil, errors.BadRequest("invalid pricing_model")
		}
		plan.PricingModel = *req.PricingModel
	}
	if plan.BillingCycle == bizbilling.BillingCycleCustom && plan.CycleDays <= 0 {
		return nil, errors.BadRequest("cycle_days is required when billing_cycle is custom")
	}
	if req.Tiers != nil {
		plan.Tiers = req.Tiers
	}
	if req.Active != nil {
		plan.Active = *req.Active
	}
	if req.Metadata != nil {
		plan.Metadata = req.Metadata
	}
	plan.UpdatedAt = time.Now().UTC()

	if err := s.planRepo.Update(ctx, plan); err != nil {
		return nil, err
	}

	s.logger.Info("Plan updated", zap.String("plan_id", id))
	return plan, nil
}

// Archive soft-deletes a plan by setting active=false.
func (s *BizPlanService) Archive(ctx context.Context, id string) error {
	plan, err := s.planRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	plan.Active = false
	plan.UpdatedAt = time.Now().UTC()

	if err := s.planRepo.Update(ctx, plan); err != nil {
		return err
	}

	s.logger.Info("Plan archived", zap.String("plan_id", id))
	return nil
}

// List returns plans for an app with optional filters.
func (s *BizPlanService) List(ctx context.Context, filter bizbilling.PlanFilter) ([]*bizbilling.Plan, int64, error) {
	return s.planRepo.List(ctx, filter)
}

// ─── Addon Operations ────────────────────────────────────────────────────

// AddAddon creates an addon for a plan.
func (s *BizPlanService) AddAddon(ctx context.Context, appID, planID string, req bizbilling.CreatePlanAddonRequest) (*bizbilling.PlanAddon, error) {
	// Verify plan exists
	plan, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return nil, errors.NotFound("plan", planID)
	}
	if plan.AppID != appID {
		return nil, errors.NotFound("plan", planID)
	}

	if req.Name == "" {
		return nil, errors.BadRequest("addon name is required")
	}
	if req.AmountPaisa <= 0 {
		return nil, errors.BadRequest("amount_paisa must be greater than 0")
	}

	billingCycle := req.BillingCycle
	if billingCycle == "" {
		billingCycle = plan.BillingCycle // Inherit from parent plan
	}
	if !bizbilling.ValidBillingCycles[billingCycle] {
		return nil, errors.BadRequest("invalid billing_cycle")
	}

	now := time.Now().UTC()
	addon := &bizbilling.PlanAddon{
		ID:           uuid.New().String(),
		AppID:        appID,
		PlanID:       planID,
		Name:         req.Name,
		AmountPaisa:  req.AmountPaisa,
		BillingCycle: billingCycle,
		Active:       true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.addonRepo.Create(ctx, addon); err != nil {
		return nil, err
	}

	s.logger.Info("Plan addon created",
		zap.String("addon_id", addon.ID),
		zap.String("plan_id", planID))

	return addon, nil
}

// RemoveAddon removes an addon from a plan by archiving it.
func (s *BizPlanService) RemoveAddon(ctx context.Context, appID, planID, addonID string) error {
	addon, err := s.addonRepo.GetByID(ctx, addonID)
	if err != nil {
		return err
	}
	if addon.AppID != appID || addon.PlanID != planID {
		return errors.NotFound("addon", addonID)
	}

	// Soft delete — archive the addon
	addon.Active = false
	addon.UpdatedAt = time.Now().UTC()

	return s.addonRepo.Update(ctx, addon)
}

// ListAddons lists addons for a plan.
func (s *BizPlanService) ListAddons(ctx context.Context, appID, planID string) ([]*bizbilling.PlanAddon, error) {
	return s.addonRepo.ListByPlanID(ctx, appID, planID)
}
