package payment

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

type razorpayBizRefund struct {
	ID        string            `json:"id"`
	Entity    string            `json:"entity"`
	PaymentID string            `json:"payment_id"`
	Amount    int64             `json:"amount"`
	Currency  string            `json:"currency"`
	Receipt   string            `json:"receipt"`
	Status    string            `json:"status"`
	Notes     map[string]string `json:"notes"`
}

func (g *bizRazorpayGateway) checkedRefund(raw razorpayBizRefund, intent bizbilling.RefundIntent) (bizbilling.GatewayRefund, error) {
	if !providerEntityID.MatchString(raw.ID) || raw.Entity != "refund" || raw.PaymentID != intent.PaymentID || raw.Amount != intent.AmountPaisa || raw.Currency != intent.Currency || raw.Receipt != intent.Receipt || raw.Notes["frn_operation"] != intent.OperationID || raw.Notes["frn_reservation"] != intent.ReservationID || raw.Notes["frn_app"] != intent.Scope.AppID || intent.Gateway != g.credentials.Reference || intent.Scope.AppID != g.credentials.AppID {
		return bizbilling.GatewayRefund{}, bizbilling.ErrInvalidGatewayProof
	}
	switch raw.Status {
	case "pending", "processed", "failed":
	default:
		return bizbilling.GatewayRefund{}, bizbilling.ErrInvalidGatewayProof
	}
	return bizbilling.GatewayRefund{ID: raw.ID, Intent: intent, Status: raw.Status}, nil
}
func (g *bizRazorpayGateway) Refund(ctx context.Context, intent bizbilling.RefundIntent) (bizbilling.GatewayRefund, error) {
	if intent.Gateway != g.credentials.Reference || intent.Scope.AppID != g.credentials.AppID || !providerEntityID.MatchString(intent.PaymentID) || intent.AmountPaisa <= 0 || intent.Currency != intent.Scope.Currency || intent.OperationID == "" || intent.ReservationID == "" || intent.Receipt == "" || len(intent.Receipt) > 40 {
		return bizbilling.GatewayRefund{}, bizbilling.ErrInvalidGatewayProof
	}
	var response razorpayBizRefund
	err := g.request(ctx, http.MethodPost, "/payments/"+url.PathEscape(intent.PaymentID)+"/refund", map[string]any{"amount": intent.AmountPaisa, "speed": "normal", "receipt": intent.Receipt, "notes": map[string]string{"frn_operation": intent.OperationID, "frn_reservation": intent.ReservationID, "frn_app": intent.Scope.AppID}}, &response)
	if err != nil {
		return bizbilling.GatewayRefund{}, err
	}
	return g.checkedRefund(response, intent)
}
func (g *bizRazorpayGateway) FindRefund(ctx context.Context, intent bizbilling.RefundIntent) (*bizbilling.GatewayRefund, error) {
	if intent.Gateway != g.credentials.Reference || intent.Scope.AppID != g.credentials.AppID || !providerEntityID.MatchString(intent.PaymentID) {
		return nil, bizbilling.ErrInvalidGatewayProof
	}
	var found *bizbilling.GatewayRefund
	for skip := 0; skip < 1000; skip += 100 {
		var response struct {
			Entity string              `json:"entity"`
			Count  int                 `json:"count"`
			Items  []razorpayBizRefund `json:"items"`
		}
		query := url.Values{"count": {"100"}, "skip": {strconv.Itoa(skip)}}
		if err := g.request(ctx, http.MethodGet, "/payments/"+url.PathEscape(intent.PaymentID)+"/refunds?"+query.Encode(), nil, &response); err != nil {
			return nil, err
		}
		if response.Entity != "collection" || response.Items == nil || response.Count != len(response.Items) || response.Count > 100 {
			return nil, bizbilling.ErrGatewayPending
		}
		for _, candidate := range response.Items {
			if candidate.Receipt != intent.Receipt {
				continue
			}
			refund, err := g.checkedRefund(candidate, intent)
			if err != nil {
				return nil, err
			}
			if found != nil {
				return nil, bizbilling.ErrReconciliationRequired
			}
			found = &refund
		}
		if response.Count < 100 {
			return found, nil
		}
	}
	return nil, bizbilling.ErrGatewayPending
}
