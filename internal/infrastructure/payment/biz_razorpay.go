package payment

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

// BizRazorpayAuthenticator checks API access without creating an order or
// requiring partner-only linked-account APIs. It does not attest account ID
// or recurring capabilities; those are separate verified readiness facts.
type BizRazorpayAuthenticator struct{ client *http.Client }

func NewBizRazorpayAuthenticator(client *http.Client) *BizRazorpayAuthenticator {
	copy := http.Client{}
	if client != nil {
		copy = *client
	}
	if copy.Timeout <= 0 || copy.Timeout > 10*time.Second {
		copy.Timeout = 10 * time.Second
	}
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &BizRazorpayAuthenticator{client: &copy}
}

func (a *BizRazorpayAuthenticator) Authenticate(ctx context.Context, credentials bizbilling.MerchantCredentials) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.razorpay.com/v1/orders?count=1", nil)
	if err != nil {
		return bizbilling.ErrMerchantUnavailable
	}
	request.SetBasicAuth(credentials.KeyID, credentials.KeySecret)
	request.Header.Set("Accept", "application/json")
	response, err := a.client.Do(request)
	if err != nil {
		return bizbilling.ErrMerchantUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return bizbilling.ErrMerchantAuthentication
	}
	if response.StatusCode != http.StatusOK {
		return bizbilling.ErrMerchantUnavailable
	}
	var body struct {
		Entity string            `json:"entity"`
		Count  int               `json:"count"`
		Items  []json.RawMessage `json:"items"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if decoder.Decode(&body) != nil || body.Entity != "collection" || body.Items == nil || body.Count != len(body.Items) || body.Count > 1 {
		return bizbilling.ErrMerchantUnavailable
	}
	var extra interface{}
	if decoder.Decode(&extra) != io.EOF {
		return bizbilling.ErrMerchantUnavailable
	}
	return nil
}
