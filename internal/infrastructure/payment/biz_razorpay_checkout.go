package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

type BizRazorpayFactory struct {
	resolve    func(context.Context, bizbilling.GatewayRef) (bizbilling.MerchantCredentials, error)
	resolveAPI func(context.Context, bizbilling.GatewayRef) (bizbilling.MerchantCredentials, error)
	client     *http.Client
}

func NewBizRazorpayFactory(resolve func(context.Context, bizbilling.GatewayRef) (bizbilling.MerchantCredentials, error), client *http.Client) *BizRazorpayFactory {
	return &BizRazorpayFactory{resolve: resolve, client: NewBizRazorpayAuthenticator(client).client}
}
func (f *BizRazorpayFactory) For(ctx context.Context, ref bizbilling.GatewayRef) (bizbilling.BizGateway, error) {
	gateway, err := f.exact(ctx, ref)
	if err != nil {
		return nil, err
	}
	if f.resolveAPI != nil {
		api, err := f.resolveAPI(ctx, ref)
		if err != nil {
			return nil, err
		}
		if api.AppID != gateway.credentials.AppID || api.Reference.ConnectionID != ref.ConnectionID || api.Reference.AccountID != ref.AccountID || api.Reference.Mode != ref.Mode || api.Reference.CredentialRevision < ref.CredentialRevision || !strings.HasPrefix(api.KeyID, "rzp_"+string(ref.Mode)+"_") || api.KeySecret == "" {
			return nil, bizbilling.ErrMerchantUnavailable
		}
		gateway.apiCredentials = api
	}
	return gateway, nil
}
func (f *BizRazorpayFactory) SetAPIResolver(resolve func(context.Context, bizbilling.GatewayRef) (bizbilling.MerchantCredentials, error)) {
	f.resolveAPI = resolve
}
func (f *BizRazorpayFactory) ForExact(ctx context.Context, ref bizbilling.GatewayRef) (bizbilling.BizGateway, error) {
	return f.exact(ctx, ref)
}
func (f *BizRazorpayFactory) exact(ctx context.Context, ref bizbilling.GatewayRef) (*bizRazorpayGateway, error) {
	if f.resolve == nil {
		return nil, bizbilling.ErrMerchantUnavailable
	}
	credentials, err := f.resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	if credentials.Reference != ref || credentials.AppID == "" || credentials.KeyID == "" || credentials.KeySecret == "" {
		return nil, bizbilling.ErrMerchantUnavailable
	}
	return &bizRazorpayGateway{credentials: credentials, apiCredentials: credentials, client: f.client}, nil
}

type bizRazorpayGateway struct {
	credentials    bizbilling.MerchantCredentials
	apiCredentials bizbilling.MerchantCredentials
	client         *http.Client
}

// No provider response bodies, credentials or transport URLs escape as errors.
func (g *bizRazorpayGateway) request(ctx context.Context, method, path string, input, output any) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var payload []byte
	var err error
	if input != nil {
		payload, err = json.Marshal(input)
		if err != nil {
			return bizbilling.ErrInvalidGatewayProof
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, "https://api.razorpay.com/v1"+path, bytes.NewReader(payload))
	if err != nil {
		return bizbilling.ErrMerchantUnavailable
	}
	request.SetBasicAuth(g.apiCredentials.KeyID, g.apiCredentials.KeySecret)
	request.Header.Set("Accept", "application/json")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := g.client.Do(request)
	if err != nil {
		return bizbilling.ErrGatewayPending
	}
	defer response.Body.Close()
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return bizbilling.ErrMerchantAuthentication
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return bizbilling.ErrGatewayPending
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if decoder.Decode(output) != nil {
		return bizbilling.ErrInvalidGatewayProof
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return bizbilling.ErrInvalidGatewayProof
	}
	return nil
}

type razorpayBizOrder struct {
	ID       string            `json:"id"`
	Entity   string            `json:"entity"`
	Amount   int64             `json:"amount"`
	Currency string            `json:"currency"`
	Receipt  string            `json:"receipt"`
	Notes    map[string]string `json:"notes"`
}

func (g *bizRazorpayGateway) checkedOrder(order razorpayBizOrder, intent bizbilling.OrderIntent) (bizbilling.GatewayOrder, error) {
	if order.Entity != "order" || !providerEntityID.MatchString(order.ID) || order.Receipt != intent.Receipt || order.Amount != intent.AmountPaisa || order.Currency != intent.Currency || order.Notes["frn_operation"] != intent.OperationID || order.Notes["frn_invoice"] != intent.InvoiceID || order.Notes["frn_app"] != intent.Scope.AppID || intent.Gateway != g.credentials.Reference || intent.Scope.AppID != g.credentials.AppID {
		return bizbilling.GatewayOrder{}, bizbilling.ErrInvalidGatewayProof
	}
	return bizbilling.GatewayOrder{ID: order.ID, Intent: intent}, nil
}
func (g *bizRazorpayGateway) CreateOrder(ctx context.Context, intent bizbilling.OrderIntent) (bizbilling.GatewayOrder, error) {
	if intent.Gateway != g.credentials.Reference || intent.Scope.AppID != g.credentials.AppID || intent.AmountPaisa <= 0 || intent.Currency != intent.Scope.Currency || len(intent.Receipt) > 40 || intent.Receipt == "" || intent.OperationID == "" || intent.InvoiceID == "" {
		return bizbilling.GatewayOrder{}, bizbilling.ErrInvalidGatewayProof
	}
	var response razorpayBizOrder
	err := g.request(ctx, http.MethodPost, "/orders", map[string]any{"amount": intent.AmountPaisa, "currency": intent.Currency, "receipt": intent.Receipt, "notes": map[string]string{"frn_operation": intent.OperationID, "frn_invoice": intent.InvoiceID, "frn_app": intent.Scope.AppID}}, &response)
	if err != nil {
		return bizbilling.GatewayOrder{}, err
	}
	return g.checkedOrder(response, intent)
}
func (g *bizRazorpayGateway) FindOrder(ctx context.Context, intent bizbilling.OrderIntent) (*bizbilling.GatewayOrder, error) {
	if intent.Gateway != g.credentials.Reference || intent.Scope.AppID != g.credentials.AppID {
		return nil, bizbilling.ErrInvalidGatewayProof
	}
	var found *bizbilling.GatewayOrder
	for skip := 0; skip < 1000; skip += 100 {
		var collection struct {
			Entity string             `json:"entity"`
			Count  int                `json:"count"`
			Items  []razorpayBizOrder `json:"items"`
		}
		query := url.Values{"receipt": {intent.Receipt}, "count": {"100"}, "skip": {strconv.Itoa(skip)}}
		if err := g.request(ctx, http.MethodGet, "/orders?"+query.Encode(), nil, &collection); err != nil {
			return nil, err
		}
		if collection.Entity != "collection" || collection.Items == nil || collection.Count != len(collection.Items) || collection.Count > 100 {
			return nil, bizbilling.ErrGatewayPending
		}
		for _, candidate := range collection.Items {
			if candidate.Receipt != intent.Receipt {
				continue
			}
			order, err := g.checkedOrder(candidate, intent)
			if err != nil {
				return nil, err
			}
			if found != nil {
				return nil, bizbilling.ErrReconciliationRequired
			}
			found = &order
		}
		if collection.Count < 100 {
			return found, nil
		}
	}
	return nil, bizbilling.ErrGatewayPending
}

var providerEntityID = regexp.MustCompile(`^[A-Za-z]+_[A-Za-z0-9_]{1,120}$`)

func (g *bizRazorpayGateway) FetchPayment(ctx context.Context, id string) (bizbilling.CapturedPayment, error) {
	if !providerEntityID.MatchString(id) {
		return bizbilling.CapturedPayment{}, bizbilling.ErrInvalidGatewayProof
	}
	var response struct {
		RefundedPaisa int64  `json:"amount_refunded"`
		ID            string `json:"id"`
		Entity        string `json:"entity"`
		OrderID       string `json:"order_id"`
		Amount        int64  `json:"amount"`
		Currency      string `json:"currency"`
		Status        string `json:"status"`
		Captured      bool   `json:"captured"`
	}
	if err := g.request(ctx, http.MethodGet, "/payments/"+url.PathEscape(id), nil, &response); err != nil {
		return bizbilling.CapturedPayment{}, err
	}
	if response.ID != id || response.Entity != "payment" || response.Amount <= 0 || response.RefundedPaisa < 0 || response.RefundedPaisa > response.Amount || !providerEntityID.MatchString(response.OrderID) {
		return bizbilling.CapturedPayment{}, bizbilling.ErrInvalidGatewayProof
	}
	return bizbilling.CapturedPayment{ID: id, OrderID: response.OrderID, Gateway: g.credentials.Reference, AmountPaisa: response.Amount, Currency: response.Currency, Status: response.Status, Captured: response.Captured, RefundedPaisa: response.RefundedPaisa}, nil
}
func (g *bizRazorpayGateway) VerifyCheckout(order, payment, signature string) error {
	if !providerEntityID.MatchString(order) || !providerEntityID.MatchString(payment) {
		return bizbilling.ErrInvalidGatewayProof
	}
	actual, err := hex.DecodeString(signature)
	if err != nil {
		return bizbilling.ErrInvalidGatewayProof
	}
	mac := hmac.New(sha256.New, []byte(g.credentials.KeySecret))
	mac.Write([]byte(order + "|" + payment))
	if !hmac.Equal(actual, mac.Sum(nil)) {
		return bizbilling.ErrInvalidGatewayProof
	}
	return nil
}
