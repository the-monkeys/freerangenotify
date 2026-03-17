package license

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type remoteVerifyRequest struct {
	LicenseKey string `json:"license_key"`
	Mode       string `json:"mode,omitempty"`
}

type remoteVerifyResponse struct {
	Valid      *bool      `json:"valid,omitempty"`
	Allowed    *bool      `json:"allowed,omitempty"`
	Success    *bool      `json:"success,omitempty"`
	Reason     string     `json:"reason,omitempty"`
	Error      string     `json:"error,omitempty"`
	ValidUntil *time.Time `json:"valid_until,omitempty"`
}

type remoteVerificationResult struct {
	Allowed    bool
	Reason     string
	ValidUntil *time.Time
}

type remoteVerifier interface {
	Verify(ctx context.Context, licenseKey string) (remoteVerificationResult, error)
}

type httpRemoteVerifier struct {
	verifyURL string
	client    *http.Client
}

func newHTTPRemoteVerifier(verifyURL string, timeout time.Duration) remoteVerifier {
	if strings.TrimSpace(verifyURL) == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &httpRemoteVerifier{
		verifyURL: strings.TrimSpace(verifyURL),
		client:    &http.Client{Timeout: timeout},
	}
}

func (v *httpRemoteVerifier) Verify(ctx context.Context, licenseKey string) (remoteVerificationResult, error) {
	payload, err := json.Marshal(remoteVerifyRequest{LicenseKey: strings.TrimSpace(licenseKey), Mode: string(ModeSelfHosted)})
	if err != nil {
		return remoteVerificationResult{}, fmt.Errorf("marshal remote verification payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.verifyURL, bytes.NewReader(payload))
	if err != nil {
		return remoteVerificationResult{}, fmt.Errorf("create remote verification request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return remoteVerificationResult{}, fmt.Errorf("remote verification request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return remoteVerificationResult{}, fmt.Errorf("remote verification returned status %d", resp.StatusCode)
	}

	var body remoteVerifyResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&body); decodeErr != nil {
		// Treat empty/non-JSON body as an implicit success for backward compatibility.
		return remoteVerificationResult{Allowed: true, Reason: "remote_verified"}, nil
	}

	allowed := true
	if body.Valid != nil {
		allowed = *body.Valid
	} else if body.Allowed != nil {
		allowed = *body.Allowed
	} else if body.Success != nil {
		allowed = *body.Success
	}

	reason := strings.TrimSpace(body.Reason)
	if reason == "" {
		reason = strings.TrimSpace(body.Error)
	}
	if reason == "" {
		if allowed {
			reason = "remote_verified"
		} else {
			reason = "remote_rejected"
		}
	}

	return remoteVerificationResult{Allowed: allowed, Reason: reason, ValidUntil: body.ValidUntil}, nil
}
