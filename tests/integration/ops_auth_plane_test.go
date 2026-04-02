package integration

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type OpsAuthPlaneSuite struct {
	IntegrationTestSuite
	opsSecret string
}

func TestOpsAuthPlaneSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	suite.Run(t, new(OpsAuthPlaneSuite))
}

func (s *OpsAuthPlaneSuite) SetupSuite() {
	s.IntegrationTestSuite.SetupSuite()
	s.opsSecret = strings.TrimSpace(os.Getenv("FREERANGE_OPS_SECRET"))
}

func (s *OpsAuthPlaneSuite) TestOpsRouteRejectsJWTToken() {
	status := s.probeOpsRouteWithJWT()
	if status == http.StatusNotFound {
		s.T().Skip("ops routes unavailable (likely ops disabled or self-hosted build)")
	}
	s.Equal(http.StatusUnauthorized, status)
}

func (s *OpsAuthPlaneSuite) TestOpsRouteRejectsAPIKeyStyleBearerToken() {
	headers := map[string]string{"Authorization": "Bearer app_key_dummy"}
	payload := map[string]interface{}{
		"tenant_id": "ops-int-apikey-denied",
		"months":    1,
	}

	resp, _ := s.makeRequest(http.MethodPost, "/v1/ops/subscriptions/renew", payload, headers)
	if resp.StatusCode == http.StatusNotFound {
		s.T().Skip("ops routes unavailable (likely ops disabled or self-hosted build)")
	}
	s.Equal(http.StatusUnauthorized, resp.StatusCode)
}

func (s *OpsAuthPlaneSuite) TestOpsRouteAcceptsSignedOpsAuth() {
	if s.opsSecret == "" {
		s.T().Skip("FREERANGE_OPS_SECRET not set; skipping signed ops auth integration test")
	}

	headers := s.buildSignedOpsHeaders(http.MethodPost, "/v1/ops/subscriptions/renew", time.Now().UTC(), "ops-int-nonce-1")
	payload := map[string]interface{}{
		"tenant_id": "ops-int-tenant-1",
		"months":    1,
		"plan":      "ops_granted",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/ops/subscriptions/renew", payload, headers)
	if resp.StatusCode == http.StatusNotFound {
		s.T().Skip("ops routes unavailable (likely ops disabled or self-hosted build)")
	}

	// Auth plane passes when request is not rejected as unauthorized/rate-limited.
	s.NotEqual(http.StatusUnauthorized, resp.StatusCode, string(body))
	s.NotEqual(http.StatusTooManyRequests, resp.StatusCode, string(body))
}

func (s *OpsAuthPlaneSuite) TestOpsRouteRejectsReplayNonce() {
	if s.opsSecret == "" {
		s.T().Skip("FREERANGE_OPS_SECRET not set; skipping replay integration test")
	}

	ts := time.Now().UTC()
	nonce := "ops-int-replay-nonce"
	headers := s.buildSignedOpsHeaders(http.MethodPost, "/v1/ops/subscriptions/renew", ts, nonce)
	payload := map[string]interface{}{
		"tenant_id": "ops-int-tenant-replay",
		"months":    1,
	}

	resp1, body1 := s.makeRequest(http.MethodPost, "/v1/ops/subscriptions/renew", payload, headers)
	if resp1.StatusCode == http.StatusNotFound {
		s.T().Skip("ops routes unavailable (likely ops disabled or self-hosted build)")
	}
	s.NotEqual(http.StatusUnauthorized, resp1.StatusCode, string(body1))

	resp2, body2 := s.makeRequest(http.MethodPost, "/v1/ops/subscriptions/renew", payload, headers)
	s.Equal(http.StatusUnauthorized, resp2.StatusCode, string(body2))
}

func (s *OpsAuthPlaneSuite) probeOpsRouteWithJWT() int {
	headers := map[string]string{"Authorization": "Bearer dummy.jwt.value"}
	payload := map[string]interface{}{
		"tenant_id": "ops-int-jwt-denied",
		"months":    1,
	}
	resp, _ := s.makeRequest(http.MethodPost, "/v1/ops/subscriptions/renew", payload, headers)
	return resp.StatusCode
}

func (s *OpsAuthPlaneSuite) buildSignedOpsHeaders(method, requestPath string, ts time.Time, nonce string) map[string]string {
	timestamp := strconv.FormatInt(ts.UTC().Unix(), 10)
	sig := signOpsMessageForIntegration(s.opsSecret, method, requestPath, timestamp, nonce)
	return map[string]string{
		"Authorization":   "Bearer ops:" + s.opsSecret,
		"X-Ops-Timestamp": timestamp,
		"X-Ops-Nonce":     nonce,
		"X-Ops-Signature": sig,
	}
}

func signOpsMessageForIntegration(secret, method, requestURI, ts, nonce string) string {
	message := fmt.Sprintf("%s\n%s\n%s\n%s", strings.ToUpper(strings.TrimSpace(method)), strings.TrimSpace(requestURI), strings.TrimSpace(ts), strings.TrimSpace(nonce))
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	_, _ = mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}
