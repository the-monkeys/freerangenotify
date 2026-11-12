package payment

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

type razorpayRecurringInvoice struct {
	ID             string `json:"id"`
	Entity         string `json:"entity"`
	SubscriptionID string `json:"subscription_id"`
	OrderID        string `json:"order_id"`
	PaymentID      string `json:"payment_id"`
	Status         string `json:"status"`
	Amount         int64  `json:"amount"`
	Paid           int64  `json:"amount_paid"`
	Due            int64  `json:"amount_due"`
	Currency       string `json:"currency"`
	Start          int64  `json:"billing_start"`
	End            int64  `json:"billing_end"`
}

func (g *bizRazorpayGateway) checkedRecurringInvoice(raw razorpayRecurringInvoice) (bizbilling.GatewayRecurringInvoice, error) {
	var result bizbilling.GatewayRecurringInvoice
	if raw.Entity != "invoice" || !providerEntityID.MatchString(raw.ID) || !providerEntityID.MatchString(raw.SubscriptionID) || (raw.OrderID != "" && !providerEntityID.MatchString(raw.OrderID)) || (raw.PaymentID != "" && !providerEntityID.MatchString(raw.PaymentID)) || raw.Currency != "INR" || raw.Amount < 0 || raw.Paid < 0 || raw.Due < 0 || raw.Paid > raw.Amount || raw.Due != raw.Amount-raw.Paid {
		return result, bizbilling.ErrInvalidGatewayProof
	}
	// Authorization-only charges have no billing bounds. Keep that distinction
	// explicit so reconciliation cannot mistake them for a paid first cycle.
	if (raw.Start != 0 || raw.End != 0) && (raw.Start <= 0 || raw.End <= raw.Start || raw.End > 253402300799) {
		return result, bizbilling.ErrInvalidGatewayProof
	}
	switch raw.Status {
	case "draft", "issued", "partially_paid", "paid", "cancelled", "expired":
	default:
		return result, bizbilling.ErrInvalidGatewayProof
	}
	result = bizbilling.GatewayRecurringInvoice{ID: raw.ID, SubscriptionID: raw.SubscriptionID, OrderID: raw.OrderID, PaymentID: raw.PaymentID, Status: raw.Status, AmountPaisa: raw.Amount, PaidPaisa: raw.Paid, DuePaisa: raw.Due, Currency: raw.Currency, Gateway: g.credentials.Reference}
	if raw.Start > 0 {
		result.Start, result.End = time.Unix(raw.Start, 0).UTC(), time.Unix(raw.End, 0).UTC()
	}
	return result, nil
}

func (g *bizRazorpayGateway) FetchRecurringInvoice(ctx context.Context, id string) (bizbilling.GatewayRecurringInvoice, error) {
	if !providerEntityID.MatchString(id) {
		return bizbilling.GatewayRecurringInvoice{}, bizbilling.ErrInvalidGatewayProof
	}
	var raw razorpayRecurringInvoice
	if err := g.request(ctx, http.MethodGet, "/invoices/"+url.PathEscape(id), nil, &raw); err != nil {
		return bizbilling.GatewayRecurringInvoice{}, err
	}
	if raw.ID != id {
		return bizbilling.GatewayRecurringInvoice{}, bizbilling.ErrInvalidGatewayProof
	}
	return g.checkedRecurringInvoice(raw)
}

func (g *bizRazorpayGateway) ListRecurringInvoices(ctx context.Context, subscription string, offset, size int) ([]bizbilling.GatewayRecurringInvoice, error) {
	if !providerEntityID.MatchString(subscription) || offset < 0 || size < 1 || size > 100 {
		return nil, bizbilling.ErrInvalidGatewayProof
	}
	query := url.Values{"subscription_id": {subscription}, "count": {strconv.Itoa(size)}, "skip": {strconv.Itoa(offset)}}
	var raw struct {
		Entity string                     `json:"entity"`
		Count  int                        `json:"count"`
		Items  []razorpayRecurringInvoice `json:"items"`
	}
	if err := g.request(ctx, http.MethodGet, "/invoices?"+query.Encode(), nil, &raw); err != nil {
		return nil, err
	}
	if raw.Entity != "collection" || raw.Count != len(raw.Items) || raw.Count > size {
		return nil, bizbilling.ErrInvalidGatewayProof
	}
	rows := make([]bizbilling.GatewayRecurringInvoice, 0, len(raw.Items))
	for _, item := range raw.Items {
		invoice, err := g.checkedRecurringInvoice(item)
		if err != nil {
			return nil, err
		}
		if invoice.SubscriptionID != subscription {
			return nil, bizbilling.ErrInvalidGatewayProof
		}
		rows = append(rows, invoice)
	}
	return rows, nil
}
