package bizbilling

import "time"

// An operator must confirm the origin has stopped AND its storage requests have
// drained. Age alone is not evidence authorizing a permit to be removed.
type WriterPermitResolution struct {
	WriterIDs        []string `json:"writer_ids,omitempty"`
	WriterID         string   `json:"writer_id,omitempty"`
	ExpectedEpoch    int64    `json:"expected_epoch"`
	PrimaryCurrency  string   `json:"primary_currency"`
	ActorID          string   `json:"actor_id"`
	Reason           string   `json:"reason"`
	ConfirmedDrained bool     `json:"confirmed_drained"`
}

type WriterPermitReviewRecord struct {
	Permits    []MutationPermit       `json:"permits,omitempty"`
	ID         string                 `json:"id"`
	AppID      string                 `json:"app_id"`
	ReportID   string                 `json:"report_id"`
	Kind       string                 `json:"kind"`
	IntentHash string                 `json:"intent_hash"`
	Resolution WriterPermitResolution `json:"resolution"`
	Status     string                 `json:"status"`
}

// The fence is bounded to one reviewed action, not a history of writer IDs.
type WriterPermitReviewFence struct {
	ID          string    `json:"id"`
	Owner       string    `json:"owner"`
	PriorStatus string    `json:"prior_status"`
	LeaseUntil  time.Time `json:"lease_until"`
}
