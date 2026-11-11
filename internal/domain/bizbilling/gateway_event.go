package bizbilling

import (
	"errors"
	"time"
)

var ErrGatewaySignature = errors.New("payment callback signature could not be verified")

type GatewayEventAcknowledgment struct {
	ID             string `json:"id"`
	State          string `json:"state"`
	HeaderConflict bool   `json:"header_conflict"`
}

// Only the fields needed for reconciliation are retained. Raw customer/card
// data is deliberately excluded; the digest identifies the exact signed bytes.
type GatewayEventData struct {
	Event       string `json:"event"`
	AccountID   string `json:"account_id"`
	PaymentID   string `json:"payment_id,omitempty"`
	OrderID     string `json:"order_id,omitempty"`
	RefundID    string `json:"refund_id,omitempty"`
	AmountPaisa int64  `json:"amount_paisa,omitempty"`
	Currency    string `json:"currency,omitempty"`
}
type GatewayEventReceipt struct {
	SchemaVersion    int              `json:"schema_version"`
	ID               string           `json:"id"`
	Kind             string           `json:"kind"`
	AppID            string           `json:"app_id"`
	Gateway          GatewayRef       `json:"gateway"`
	BodyDigest       string           `json:"body_digest"`
	Data             GatewayEventData `json:"data"`
	State            string           `json:"state"`
	HeaderConflict   bool             `json:"header_conflict"`
	VerifiedRevision int64            `json:"verified_revision,omitempty"`
	Result           *OperationResult `json:"result,omitempty"`
	FailureCode      string           `json:"failure_code,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}
type GatewayEventAlias struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	AppID      string `json:"app_id"`
	ReceiptID  string `json:"receipt_id"`
	BodyDigest string `json:"body_digest"`
}
type GatewayHeaderCollision struct {
	ID              string    `json:"id"`
	Kind            string    `json:"kind"`
	AppID           string    `json:"app_id"`
	FirstReceiptID  string    `json:"first_receipt_id"`
	SecondReceiptID string    `json:"second_receipt_id"`
	CreatedAt       time.Time `json:"created_at"`
}

type MerchantAccountProof struct {
	AppID       string     `json:"app_id"`
	Gateway     GatewayRef `json:"gateway"`
	EventDigest string     `json:"event_digest"`
	PaymentID   string     `json:"payment_id"`
	VerifiedAt  time.Time  `json:"verified_at"`
}
