package contracts

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/handlers"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

// Only persistence is replaced. Published examples use the actual billing
// handler/service and the onboarding router's real API-key middleware.
type guideBillingConfigs struct {
	rows map[string]*bizbilling.BizConfig
}

func (r *guideBillingConfigs) Get(_ context.Context, app string, kind bizbilling.ConfigType) (*bizbilling.BizConfig, error) {
	return r.rows[app+":"+string(kind)], nil
}
func (r *guideBillingConfigs) Upsert(_ context.Context, config *bizbilling.BizConfig) error {
	r.rows[config.AppID+":"+string(config.ConfigType)] = config
	return nil
}
func billingGuideRoutes(router fiber.Router, logger *zap.Logger) {
	config := services.NewBizConfigService(&guideBillingConfigs{rows: map[string]*bizbilling.BizConfig{}}, logger)
	h := handlers.NewBizBillingHandler(handlers.BizBillingServices{Config: config}, logger)
	router.Get("/biz/config/:config_type", h.GetConfig)
	router.Put("/biz/config/:config_type", h.UpdateConfig)
}
