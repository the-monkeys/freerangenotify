package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// BizProductService implements business logic for managing products
// in the business billing module.
type BizProductService struct {
	repo   bizbilling.ProductRepository
	logger *zap.Logger
}

// NewBizProductService creates a new BizProductService.
func NewBizProductService(repo bizbilling.ProductRepository, logger *zap.Logger) *BizProductService {
	return &BizProductService{
		repo:   repo,
		logger: logger,
	}
}

// Create creates a new product.
func (s *BizProductService) Create(ctx context.Context, appID string, req bizbilling.CreateProductRequest) (*bizbilling.Product, error) {
	if appID == "" {
		return nil, errors.BadRequest("app_id is required")
	}
	if req.Name == "" {
		return nil, errors.BadRequest("product name is required")
	}
	if req.TaxRate < 0 || req.TaxRate > 28 {
		return nil, errors.BadRequest("tax_rate must be between 0 and 28")
	}

	now := time.Now().UTC()
	product := &bizbilling.Product{
		ID:          uuid.New().String(),
		AppID:       appID,
		Name:        req.Name,
		Description: req.Description,
		HSNCode:     req.HSNCode,
		TaxRate:     req.TaxRate,
		Unit:        req.Unit,
		Active:      true,
		Metadata:    req.Metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.Create(ctx, product); err != nil {
		s.logger.Error("Failed to create product",
			zap.String("app_id", appID),
			zap.String("name", req.Name),
			zap.Error(err))
		return nil, err
	}

	s.logger.Info("Product created",
		zap.String("product_id", product.ID),
		zap.String("app_id", appID),
		zap.String("name", product.Name))

	return product, nil
}

// GetByID retrieves a product by ID.
func (s *BizProductService) GetByID(ctx context.Context, id string) (*bizbilling.Product, error) {
	return s.repo.GetByID(ctx, id)
}

// Update updates a product.
func (s *BizProductService) Update(ctx context.Context, id string, req bizbilling.UpdateProductRequest) (*bizbilling.Product, error) {
	product, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		product.Name = *req.Name
	}
	if req.Description != nil {
		product.Description = *req.Description
	}
	if req.HSNCode != nil {
		product.HSNCode = *req.HSNCode
	}
	if req.TaxRate != nil {
		if *req.TaxRate < 0 || *req.TaxRate > 28 {
			return nil, errors.BadRequest("tax_rate must be between 0 and 28")
		}
		product.TaxRate = *req.TaxRate
	}
	if req.Unit != nil {
		product.Unit = *req.Unit
	}
	if req.Active != nil {
		product.Active = *req.Active
	}
	if req.Metadata != nil {
		product.Metadata = req.Metadata
	}
	product.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, product); err != nil {
		return nil, err
	}

	s.logger.Info("Product updated",
		zap.String("product_id", id))

	return product, nil
}

// Delete archives (soft-deletes) a product by setting active=false.
func (s *BizProductService) Delete(ctx context.Context, id string) error {
	product, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	product.Active = false
	product.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, product); err != nil {
		return err
	}

	s.logger.Info("Product archived",
		zap.String("product_id", id))
	return nil
}

// List returns products for an app with optional filters.
func (s *BizProductService) List(ctx context.Context, filter bizbilling.ProductFilter) ([]*bizbilling.Product, int64, error) {
	return s.repo.List(ctx, filter)
}
