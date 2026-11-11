package user

// BillingBalanceReceipt proves the compatible scalar mirror is derived from a
// completed financial action. Public request DTOs cannot supply these fields.
type BillingBalanceReceipt struct {
	BalancePaisa int64
	Sequence     int64
	Currency     string
}
