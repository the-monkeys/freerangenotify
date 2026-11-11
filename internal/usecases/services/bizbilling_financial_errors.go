package services

import (
	"errors"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func financialRejection(err error) (string, bool) {
	switch {
	case errors.Is(err, bizbilling.ErrLegacyReviewRequired):
		return "legacy_review_required", true
	case errors.Is(err, bizbilling.ErrCouponExhausted), errors.Is(err, bizbilling.ErrCouponInactive), errors.Is(err, bizbilling.ErrCouponExpired), errors.Is(err, bizbilling.ErrCouponNotStarted), errors.Is(err, bizbilling.ErrCouponPlanRestricted):
		return "coupon_unavailable", true

	case errors.Is(err, bizbilling.ErrCurrencyMismatch):
		return "currency_mismatch", true
	case errors.Is(err, bizbilling.ErrAutomaticCollectionInProgress):
		return "collection_locked", true
	case errors.Is(err, bizbilling.ErrMoneyOverflow):
		return "money_overflow", true
	case errors.Is(err, bizbilling.ErrInvalidMoney):
		return "invalid_financial_intent", true
	case apperrors.IsNotFound(err):
		return "resource_not_found", true
	}
	if appErr, ok := apperrors.AsAppError(err); ok {
		switch appErr.GetHTTPStatus() {
		case 400, 422:
			return "invalid_financial_intent", true
		case 409:
			return "resource_conflict", true
		case 403:
			return "resource_forbidden", true
		}
	}
	return "", false
}
func financialRejectedError(code string) error {
	switch code {
	case "legacy_review_required":
		return bizbilling.ErrLegacyReviewRequired
	case "resource_conflict":
		return apperrors.Conflict("This financial action conflicts with existing data")
	case "coupon_unavailable":
		return bizbilling.ErrCouponExhausted
	case "currency_mismatch":
		return bizbilling.ErrCurrencyMismatch
	case "collection_locked":
		return bizbilling.ErrAutomaticCollectionInProgress
	case "money_overflow":
		return bizbilling.ErrMoneyOverflow
	case "resource_not_found":
		return apperrors.NotFound("billing resource", "operation")
	case "resource_forbidden":
		return apperrors.Forbidden("billing resource is unavailable")
	default:
		return bizbilling.ErrInvalidMoney
	}
}
