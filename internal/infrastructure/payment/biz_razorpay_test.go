package payment

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

type bizProbeTransport struct{ destination *url.URL }

func (t bizProbeTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Scheme != "https" || request.URL.Host != "api.razorpay.com" {
		panic("unexpected merchant authentication destination")
	}
	copy := request.Clone(request.Context())
	endpoint := *request.URL
	endpoint.Scheme, endpoint.Host = t.destination.Scheme, t.destination.Host
	copy.URL = &endpoint
	return http.DefaultTransport.RoundTrip(copy)
}

func TestBizMerchantAuthenticationUsesReadOnlyOrders(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		ok         bool
	}{
		{"authenticated", `{"entity":"collection","count":0,"items":[]}`, 200, true},
		{"unauthorized", `{"error":{"description":"private-fixture-api-secret"}}`, 401, false},
		{"not-a-collection", `{"entity":"account"}`, 200, false},
		{"redirect", ``, 302, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "/v1/orders", r.URL.Path)
				require.Equal(t, "1", r.URL.Query().Get("count"))
				key, secret, ok := r.BasicAuth()
				require.True(t, ok)
				require.Equal(t, "rzp_test_biz1", key)
				require.Equal(t, "private-fixture-api-secret", secret)
				w.Header().Set("Location", "/must-not-follow")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			endpoint, err := url.Parse(server.URL)
			require.NoError(t, err)
			probe := NewBizRazorpayAuthenticator(&http.Client{Transport: bizProbeTransport{endpoint}})
			require.Greater(t, probe.client.Timeout, time.Duration(0))
			require.LessOrEqual(t, probe.client.Timeout, 10*time.Second)
			err = probe.Authenticate(context.Background(), bizbilling.MerchantCredentials{KeyID: "rzp_test_biz1", KeySecret: "private-fixture-api-secret"})
			if tc.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "private-fixture-api-secret")
			}
			require.Equal(t, 1, calls)
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := NewBizRazorpayAuthenticator(nil).Authenticate(ctx, bizbilling.MerchantCredentials{KeyID: "rzp_test_biz1", KeySecret: "fixture-secret"})
	require.Error(t, err)
	require.False(t, strings.Contains(err.Error(), "fixture-secret"))
}
