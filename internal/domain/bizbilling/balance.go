package bizbilling

// CustomerBalance is scoped by business, customer, and currency. LastSequence
// changes in the same conditional write as the amounts, preventing double apply.
type CustomerBalance struct {
	Scope            Scope `json:"scope"`
	AvailablePaisa   int64 `json:"available_paisa"`
	ReservedPaisa    int64 `json:"reserved_paisa"`
	LastSequence     int64 `json:"last_sequence"`
	UnresolvedLegacy bool  `json:"unresolved_legacy"`
}

type OpeningBalance struct {
	AvailablePaisa   int64 `json:"available_paisa"`
	UnresolvedLegacy bool  `json:"unresolved_legacy"`
}

type BalanceAdjustment struct {
	DeltaPaisa int64  `json:"delta_paisa"`
	Reason     string `json:"reason"`
}
