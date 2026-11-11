package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

type UserBillingBalanceWriter func(context.Context, string, *user.User, int64, string, string) (*user.BillingBalanceReceipt, error)

func (h *UserHandler) SetBillingBalanceWriter(writer UserBillingBalanceWriter) {
	h.billingBalanceWriter = writer
}

func userBillingRequestKey(c *fiber.Ctx) string {
	key := c.Get("Idempotency-Key")
	if key == "" {
		key = c.Get("X-Idempotency-Key")
	}
	return key
}

func assignBillingCustomerID(c *fiber.Ctx, profile *user.User, discriminator string) {
	if profile.UserID != "" {
		return
	}
	key := userBillingRequestKey(c)
	if key == "" {
		profile.UserID = uuid.NewString()
		return
	}
	profile.UserID = uuid.NewSHA1(uuid.NameSpaceOID, []byte("biz-customer:"+profile.AppID+":"+key+":"+discriminator)).String()
}

func (h *UserHandler) prepareUserBalance(c *fiber.Ctx, profile *user.User, target int64, reason string, request any) (context.Context, *user.BillingBalanceReceipt, error) {
	ctx := c.UserContext()
	if h.billingBalanceWriter == nil {
		return ctx, nil, nil
	}
	key := userBillingRequestKey(c)
	if len(key) > 200 {
		return ctx, nil, apperrors.BadRequest("Idempotency-Key must be at most 200 characters")
	}
	appID, err := h.getAppID(c)
	if err != nil {
		return ctx, nil, err
	}
	if profile.AppID != appID {
		return ctx, nil, apperrors.NotFound("customer", profile.UserID)
	}
	ctx = services.WithBizOperationKey(ctx, key)
	if actor, ok := c.Locals("user_id").(string); ok {
		ctx = services.WithBizOperationActor(ctx, actor)
	}
	data, err := json.Marshal(request)
	if err != nil {
		return ctx, nil, err
	}
	fingerprint := sha256.Sum256(data)
	receipt, err := h.billingBalanceWriter(ctx, appID, profile, target, reason, hex.EncodeToString(fingerprint[:]))
	if err != nil {
		return ctx, nil, err
	}
	if receipt != nil {
		profile.BalancePaisa = receipt.BalancePaisa
		profile.BizBalanceSequence = receipt.Sequence
		profile.BizBalanceCurrency = receipt.Currency
	}
	return ctx, receipt, nil
}

func (h *UserHandler) userBillingError(c *fiber.Ctx, err error) error {
	return (&BizBillingHandler{logger: h.logger}).handleError(c, err)
}

// Check identity before creating an opening credit. A duplicated email must not
// accidentally create credit for another generated customer identifier.
func (h *UserHandler) checkBillingCustomerIdentity(c *fiber.Ctx, profile *user.User) error {
	if current, err := h.service.GetByID(c.UserContext(), profile.UserID); err == nil {
		if current.AppID != profile.AppID {
			return apperrors.NotFound("customer", profile.UserID)
		}
	} else if !apperrors.IsNotFound(err) {
		return err
	}
	if profile.Email != "" {
		current, err := h.service.GetByEmail(c.UserContext(), profile.AppID, profile.Email)
		if err == nil && current.UserID != profile.UserID {
			return apperrors.Conflict("User with this email already exists")
		}
		if err != nil && !apperrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}
