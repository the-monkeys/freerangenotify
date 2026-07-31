package services

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// BizCouponService manages discount coupons.
type BizCouponService struct {
	coupons *bizbillingrepo.Store[bizbilling.Coupon]
	logger  *zap.Logger
}

// NewBizCouponService creates a new BizCouponService.
func NewBizCouponService(stores *bizbillingrepo.Stores, logger *zap.Logger) *BizCouponService {
	return &BizCouponService{coupons: stores.Coupons, logger: logger}
}

// Create creates a coupon; codes are unique per app.
func (s *BizCouponService) Create(ctx context.Context, appID string, req *bizbilling.CreateCouponRequest) (*bizbilling.Coupon, error) {
	code := normalizeCouponCode(req.Code)
	if code == "" {
		return nil, errors.BadRequest("coupon code is required")
	}
	if req.DiscountType != bizbilling.DiscountTypePercentage && req.DiscountType != bizbilling.DiscountTypeFixed {
		return nil, errors.BadRequest("discount_type must be 'percentage' or 'fixed'")
	}
	if req.DiscountType == bizbilling.DiscountTypePercentage && (req.DiscountValue <= 0 || req.DiscountValue > 100) {
		return nil, errors.BadRequest("percentage discount must be between 1 and 100")
	}

	if existing, err := s.FindByCode(ctx, appID, code); err == nil && existing != nil {
		return nil, errors.Conflict("coupon code already exists")
	}

	now := time.Now().UTC()
	coupon := &bizbilling.Coupon{
		ID:                uuid.New().String(),
		AppID:             appID,
		Code:              code,
		DiscountType:      req.DiscountType,
		DiscountValue:     req.DiscountValue,
		MaxUses:           req.MaxUses,
		MaxUsesPerUser:    req.MaxUsesPerUser,
		ApplicablePlanIDs: req.ApplicablePlanIDs,
		Stackable:         req.Stackable,
		ValidFrom:         req.ValidFrom,
		ValidUntil:        req.ValidUntil,
		Active:            true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.coupons.Create(ctx, coupon.ID, coupon); err != nil {
		return nil, err
	}
	return coupon, nil
}

// BulkGenerate creates N unique coupon codes sharing the same rules.
func (s *BizCouponService) BulkGenerate(ctx context.Context, appID string, req *bizbilling.BulkGenerateCouponsRequest) ([]*bizbilling.Coupon, error) {
	if req.Count <= 0 || req.Count > 1000 {
		return nil, errors.BadRequest("count must be between 1 and 1000")
	}

	prefix := normalizeCouponCode(req.Prefix)
	coupons := make([]*bizbilling.Coupon, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		rules := req.Rules
		rules.Code = generateCouponCode(prefix)
		coupon, err := s.Create(ctx, appID, &rules)
		if err != nil {
			// Extremely unlikely random collision — retry once, then give up on this code.
			rules.Code = generateCouponCode(prefix)
			if coupon, err = s.Create(ctx, appID, &rules); err != nil {
				return coupons, err
			}
		}
		coupons = append(coupons, coupon)
	}
	return coupons, nil
}

// Get returns a coupon scoped to the app.
func (s *BizCouponService) Get(ctx context.Context, appID, id string) (*bizbilling.Coupon, error) {
	coupon, err := s.coupons.Get(ctx, id)
	if err != nil || coupon.AppID != appID {
		return nil, errors.NotFound("coupon", id)
	}
	return coupon, nil
}

// FindByCode looks a coupon up by its code.
func (s *BizCouponService) FindByCode(ctx context.Context, appID, code string) (*bizbilling.Coupon, error) {
	found, _, err := s.coupons.Find(ctx,
		bizbillingrepo.NewQuery(appID).Term("code", normalizeCouponCode(code)).Body("created_at", 1, 0))
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, errors.NotFound("coupon", code)
	}
	return found[0], nil
}

// List lists coupons for an app.
func (s *BizCouponService) List(ctx context.Context, filter bizbilling.CouponFilter) ([]*bizbilling.Coupon, int64, error) {
	q := bizbillingrepo.NewQuery(filter.AppID).Term("code", normalizeCouponCode(filter.Code))
	if filter.Active != nil {
		q.Term("active", *filter.Active)
	}
	return s.coupons.Find(ctx, q.Body("created_at", filter.Limit, filter.Offset))
}

// Update updates coupon rules.
func (s *BizCouponService) Update(ctx context.Context, appID, id string, req *bizbilling.UpdateCouponRequest) (*bizbilling.Coupon, error) {
	return s.coupons.Update(ctx, id, func(cp *bizbilling.Coupon) error {
		if cp.AppID != appID {
			return errors.NotFound("coupon", id)
		}
		if req.DiscountValue != nil {
			cp.DiscountValue = *req.DiscountValue
		}
		if req.MaxUses != nil {
			cp.MaxUses = *req.MaxUses
		}
		if req.MaxUsesPerUser != nil {
			cp.MaxUsesPerUser = *req.MaxUsesPerUser
		}
		if req.ApplicablePlanIDs != nil {
			cp.ApplicablePlanIDs = req.ApplicablePlanIDs
		}
		if req.Stackable != nil {
			cp.Stackable = *req.Stackable
		}
		if req.ValidFrom != nil {
			cp.ValidFrom = req.ValidFrom
		}
		if req.ValidUntil != nil {
			cp.ValidUntil = req.ValidUntil
		}
		if req.Active != nil {
			cp.Active = *req.Active
		}
		cp.UpdatedAt = time.Now().UTC()
		return nil
	})
}

// Delete removes a coupon.
func (s *BizCouponService) Delete(ctx context.Context, appID, id string) error {
	if _, err := s.Get(ctx, appID, id); err != nil {
		return err
	}
	return s.coupons.Delete(ctx, id)
}

// normalizeCouponCode upper-cases and strips whitespace.
func normalizeCouponCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// generateCouponCode returns PREFIX-XXXXXXXX using an unambiguous alphabet.
func generateCouponCode(prefix string) string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	raw := make([]byte, 8)
	_, _ = rand.Read(raw)
	for i := range raw {
		raw[i] = alphabet[int(raw[i])%len(alphabet)]
	}
	if prefix == "" {
		return string(raw)
	}
	return fmt.Sprintf("%s-%s", prefix, raw)
}
