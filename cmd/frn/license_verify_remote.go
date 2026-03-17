package main

import (
	"fmt"
	"net/http"
	"strings"
)

func verifySelfHostedLicense(verifyURL, licenseKey string) error {
	if strings.TrimSpace(verifyURL) == "" {
		return fmt.Errorf("license verify URL is empty")
	}
	if strings.TrimSpace(licenseKey) == "" {
		return fmt.Errorf("license key is required for self-hosted install")
	}

	payload := map[string]interface{}{"license_key": strings.TrimSpace(licenseKey)}
	_, err := doJSONRequest(http.MethodPost, verifyURL, payload, map[string]string{})
	if err != nil {
		return fmt.Errorf("license verification failed: %w", err)
	}
	return nil
}
