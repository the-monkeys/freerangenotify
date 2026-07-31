package bizbillingrepo

import (
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"go.uber.org/zap"
)

// Index names for the extended biz billing entities.
const (
	EstimateIndex        = "frn_biz_estimates"
	CouponIndex          = "frn_biz_coupons"
	CreditNoteIndex      = "frn_biz_credit_notes"
	RetainerIndex        = "frn_biz_retainers"
	ContractIndex        = "frn_biz_contracts"
	UsageMeterIndex      = "frn_biz_usage_meters"
	UsageEventIndex      = "frn_biz_usage_events"
	ExpenseIndex         = "frn_biz_expenses"
	RevenueScheduleIndex = "frn_biz_revenue_schedules"
	ConnectorIndex       = "frn_biz_connectors"
	PortalTokenIndex     = "frn_biz_portal_tokens"
)

// Stores groups the typed document stores for all extended biz billing
// entities so container wiring stays compact.
//
// Invoices/Subscriptions/Payments stores provide flexible querying
// (scheduler scans, analytics aggregations) on the same indices the
// LifecycleRepo writes to.
type Stores struct {
	Invoices         *Store[bizbilling.BizInvoice]
	Subscriptions    *Store[bizbilling.BizSubscription]
	Payments         *Store[bizbilling.BizPayment]
	Estimates        *Store[bizbilling.Estimate]
	Coupons          *Store[bizbilling.Coupon]
	CreditNotes      *Store[bizbilling.CreditNote]
	Retainers        *Store[bizbilling.Retainer]
	Contracts        *Store[bizbilling.Contract]
	UsageMeters      *Store[bizbilling.UsageMeter]
	UsageEvents      *Store[bizbilling.BizUsageEvent]
	Expenses         *Store[bizbilling.Expense]
	RevenueSchedules *Store[bizbilling.RevenueSchedule]
	Connectors       *Store[bizbilling.Connector]
	PortalTokens     *Store[bizbilling.PortalToken]
}

// NewStores creates all extended entity stores.
func NewStores(es *elasticsearch.Client, logger *zap.Logger) *Stores {
	return &Stores{
		Invoices:         NewStore[bizbilling.BizInvoice](es, invoiceIndex, logger),
		Subscriptions:    NewStore[bizbilling.BizSubscription](es, subscriptionIndex, logger),
		Payments:         NewStore[bizbilling.BizPayment](es, paymentIndex, logger),
		Estimates:        NewStore[bizbilling.Estimate](es, EstimateIndex, logger),
		Coupons:          NewStore[bizbilling.Coupon](es, CouponIndex, logger),
		CreditNotes:      NewStore[bizbilling.CreditNote](es, CreditNoteIndex, logger),
		Retainers:        NewStore[bizbilling.Retainer](es, RetainerIndex, logger),
		Contracts:        NewStore[bizbilling.Contract](es, ContractIndex, logger),
		UsageMeters:      NewStore[bizbilling.UsageMeter](es, UsageMeterIndex, logger),
		UsageEvents:      NewStore[bizbilling.BizUsageEvent](es, UsageEventIndex, logger),
		Expenses:         NewStore[bizbilling.Expense](es, ExpenseIndex, logger),
		RevenueSchedules: NewStore[bizbilling.RevenueSchedule](es, RevenueScheduleIndex, logger),
		Connectors:       NewStore[bizbilling.Connector](es, ConnectorIndex, logger),
		PortalTokens:     NewStore[bizbilling.PortalToken](es, PortalTokenIndex, logger),
	}
}
