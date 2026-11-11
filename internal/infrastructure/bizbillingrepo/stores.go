package bizbillingrepo

import (
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

// Index names for the extended biz billing entities.
const (
	CouponRedemptionIndex    = "frn_biz_coupon_redemptions"
	CouponCustomerUsageIndex = "frn_biz_coupon_customer_usage"
	CreditApplicationIndex   = "frn_biz_credit_applications"
	EstimateIndex            = "frn_biz_estimates"
	CouponIndex              = "frn_biz_coupons"
	CreditNoteIndex          = "frn_biz_credit_notes"
	RetainerIndex            = "frn_biz_retainers"
	ContractIndex            = "frn_biz_contracts"
	UsageMeterIndex          = "frn_biz_usage_meters"
	UsageEventIndex          = "frn_biz_usage_events"
	ExpenseIndex             = "frn_biz_expenses"
	RevenueScheduleIndex     = "frn_biz_revenue_schedules"
	ConnectorIndex           = "frn_biz_connectors"
	PortalTokenIndex         = "frn_biz_portal_tokens"
	CreditLotIndex           = "frn_biz_credit_lots"
	RefundReservationIndex   = "frn_biz_refund_reservations"
)

// Stores groups the typed document stores for all extended biz billing
// entities so container wiring stays compact.
//
// Invoices/Subscriptions/Payments stores provide flexible querying
// (scheduler scans, analytics aggregations) on the same indices the
// LifecycleRepo writes to.
type Stores struct {
	CouponRedemptions  *Store[bizbilling.CouponRedemption]
	CreditApplications *Store[bizbilling.CreditApplication]
	UserBalances       *Store[bizbilling.UserBalanceMirror]
	Invoices           *Store[bizbilling.BizInvoice]
	Subscriptions      *Store[bizbilling.BizSubscription]
	Payments           *Store[bizbilling.BizPayment]
	Estimates          *Store[bizbilling.Estimate]
	Coupons            *Store[bizbilling.Coupon]
	CreditNotes        *Store[bizbilling.CreditNote]
	Retainers          *Store[bizbilling.Retainer]
	Contracts          *Store[bizbilling.Contract]
	UsageMeters        *Store[bizbilling.UsageMeter]
	UsageEvents        *Store[bizbilling.BizUsageEvent]
	Expenses           *Store[bizbilling.Expense]
	RevenueSchedules   *Store[bizbilling.RevenueSchedule]
	Connectors         *Store[bizbilling.Connector]
	PortalTokens       *Store[bizbilling.PortalToken]
	CreditLots         *Store[bizbilling.CreditLot]
	RefundReservations *Store[bizbilling.RefundReservation]
}

// NewStores creates all extended entity stores.
func NewStores(es *elasticsearch.Client, logger *zap.Logger, policies ...repository.RefreshPolicy) *Stores {
	return &Stores{
		CouponRedemptions:  NewStore[bizbilling.CouponRedemption](es, CouponRedemptionIndex, logger, policies...),
		CreditApplications: NewStore[bizbilling.CreditApplication](es, CreditApplicationIndex, logger, policies...),
		UserBalances:       NewStore[bizbilling.UserBalanceMirror](es, "users", logger, policies...),
		CreditLots:         NewStore[bizbilling.CreditLot](es, CreditLotIndex, logger, policies...),
		RefundReservations: NewStore[bizbilling.RefundReservation](es, RefundReservationIndex, logger, policies...),
		Invoices:           NewStore[bizbilling.BizInvoice](es, invoiceIndex, logger, policies...),
		Subscriptions:      NewStore[bizbilling.BizSubscription](es, subscriptionIndex, logger, policies...),
		Payments:           NewStore[bizbilling.BizPayment](es, paymentIndex, logger, policies...),
		Estimates:          NewStore[bizbilling.Estimate](es, EstimateIndex, logger, policies...),
		Coupons:            NewStore[bizbilling.Coupon](es, CouponIndex, logger, policies...),
		CreditNotes:        NewStore[bizbilling.CreditNote](es, CreditNoteIndex, logger, policies...),
		Retainers:          NewStore[bizbilling.Retainer](es, RetainerIndex, logger, policies...),
		Contracts:          NewStore[bizbilling.Contract](es, ContractIndex, logger, policies...),
		UsageMeters:        NewStore[bizbilling.UsageMeter](es, UsageMeterIndex, logger, policies...),
		UsageEvents:        NewStore[bizbilling.BizUsageEvent](es, UsageEventIndex, logger, policies...),
		Expenses:           NewStore[bizbilling.Expense](es, ExpenseIndex, logger, policies...),
		RevenueSchedules:   NewStore[bizbilling.RevenueSchedule](es, RevenueScheduleIndex, logger, policies...),
		Connectors:         NewStore[bizbilling.Connector](es, ConnectorIndex, logger, policies...),
		PortalTokens:       NewStore[bizbilling.PortalToken](es, PortalTokenIndex, logger, policies...),
	}
}
