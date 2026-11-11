package bizbilling

// CouponCapacityClaim is a single recoverable cross-customer counter update.
// It is never removed merely because a process lease expired.
type CouponCapacityClaim struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Stage  string `json:"stage"`
}

type CouponRedemption struct {
	ID                 string `json:"id"`
	Scope              Scope  `json:"scope"`
	CouponID           string `json:"coupon_id"`
	InvoiceID          string `json:"invoice_id"`
	DiscountPaisa      int64  `json:"discount_paisa"`
	MaxUsesPerUser     int    `json:"max_uses_per_user"`
	State              string `json:"state"`
	CapacityReleased   bool   `json:"capacity_released"`
	ReleaseOperationID string `json:"release_operation_id,omitempty"`
	LastSequence       int64  `json:"billing_sequence"`
}

func (r CouponRedemption) FinancialScope() Scope    { return r.Scope }
func (r CouponRedemption) FinancialSequence() int64 { return r.LastSequence }
func (r CouponRedemption) FinancialID() string      { return r.ID }

type CouponCustomerUsage struct {
	ID         string `json:"id"`
	AppID      string `json:"app_id"`
	UserID     string `json:"user_id"`
	CouponID   string `json:"coupon_id"`
	Uses       int    `json:"uses"`
	LastMarker string `json:"last_marker"`
}
