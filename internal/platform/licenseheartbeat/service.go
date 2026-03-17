package licenseheartbeat

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/config"
	"go.uber.org/zap"
)

type payload struct {
	InstanceID         string `json:"instance_id"`
	LicenseFingerprint string `json:"license_fingerprint"`
	Version            string `json:"version"`
	DeploymentMode     string `json:"deployment_mode"`
	Timestamp          string `json:"timestamp"`
}

type Service struct {
	endpoint   string
	interval   time.Duration
	licenseKey string
	version    string
	mode       string
	instanceID string
	client     *http.Client
	logger     *zap.Logger
}

func New(cfg *config.Config, logger *zap.Logger) *Service {
	if cfg == nil || !cfg.Licensing.Enabled || cfg.Licensing.DeploymentMode != "self_hosted" {
		return nil
	}

	licenseKey := strings.TrimSpace(cfg.Licensing.SelfHosted.LicenseKey)
	if licenseKey == "" {
		return nil
	}

	endpoint := strings.TrimSpace(cfg.Licensing.SelfHosted.HeartbeatURL)
	if endpoint == "" {
		endpoint = deriveHeartbeatURL(cfg.Licensing.SelfHosted.LicenseServerURL)
	}
	if endpoint == "" {
		return nil
	}

	seconds := cfg.Licensing.SelfHosted.HeartbeatIntervalSeconds
	if seconds <= 0 {
		seconds = 21600
	}

	instanceID := resolveInstanceID()
	if instanceID == "" {
		instanceID = "unknown-instance"
	}

	return &Service{
		endpoint:   endpoint,
		interval:   time.Duration(seconds) * time.Second,
		licenseKey: licenseKey,
		version:    cfg.App.Version,
		mode:       cfg.Licensing.DeploymentMode,
		instanceID: instanceID,
		client:     &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}
}

func (s *Service) Start(ctx context.Context) {
	if s == nil {
		return
	}

	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.logger.Info("Started self-hosted license heartbeat loop", zap.Duration("interval", s.interval), zap.String("endpoint", s.endpoint))

		// Send one heartbeat on startup to avoid waiting for the first interval.
		s.send(ctx)

		for {
			select {
			case <-ctx.Done():
				s.logger.Info("Stopped self-hosted license heartbeat loop")
				return
			case <-ticker.C:
				s.send(ctx)
			}
		}
	}()
}

func (s *Service) send(ctx context.Context) {
	now := time.Now().UTC()
	pl := payload{
		InstanceID:         s.instanceID,
		LicenseFingerprint: fingerprint(s.licenseKey),
		Version:            s.version,
		DeploymentMode:     s.mode,
		Timestamp:          now.Format(time.RFC3339),
	}

	body, err := json.Marshal(pl)
	if err != nil {
		s.logger.Error("Failed to encode license heartbeat payload", zap.Error(err))
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		s.logger.Error("Failed to create license heartbeat request", zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-License-Fingerprint", pl.LicenseFingerprint)
	req.Header.Set("X-Heartbeat-Signature", signHeartbeat(s.licenseKey, pl))

	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Warn("License heartbeat request failed", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		s.logger.Warn("License heartbeat rejected", zap.Int("status", resp.StatusCode))
		return
	}

	s.logger.Debug("License heartbeat sent", zap.String("instance_id", pl.InstanceID))
}

func resolveInstanceID() string {
	if v := strings.TrimSpace(os.Getenv("FREERANGE_INSTANCE_ID")); v != "" {
		return v
	}
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(hostname)
}

func signHeartbeat(licenseKey string, pl payload) string {
	message := fmt.Sprintf("%s|%s|%s|%s|%s", pl.InstanceID, pl.LicenseFingerprint, pl.Version, pl.DeploymentMode, pl.Timestamp)
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(licenseKey)))
	_, _ = mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func fingerprint(licenseKey string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(licenseKey)))
	return hex.EncodeToString(sum[:])
}

func deriveHeartbeatURL(verifyURL string) string {
	verifyURL = strings.TrimSpace(verifyURL)
	if verifyURL == "" {
		return ""
	}
	if strings.HasSuffix(verifyURL, "/verify") {
		return strings.TrimSuffix(verifyURL, "/verify") + "/heartbeat"
	}
	return verifyURL
}
