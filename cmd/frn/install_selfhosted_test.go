package main

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSelfHostedInstall_VerifyFailsBeforeDownload(t *testing.T) {
	tmpDir := t.TempDir()

	origVerify := verifySelfHostedLicenseFn
	origDownload := downloadFileFn
	defer func() {
		verifySelfHostedLicenseFn = origVerify
		downloadFileFn = origDownload
	}()

	verifySelfHostedLicenseFn = func(verifyURL, licenseKey string) error {
		return errors.New("license invalid")
	}

	downloadCalled := false
	downloadFileFn = func(url, target string) error {
		downloadCalled = true
		return nil
	}

	cfg := installConfig{
		InstallDir:       tmpDir,
		DeploymentMode:   "self_hosted",
		LicenseKey:       "bad-license",
		LicenseVerifyURL: "https://example.com/v1/license/verify",
		ServerBinaryURL:  "https://example.com/server",
		WorkerBinaryURL:  "https://example.com/worker",
		SkipPreflight:    true,
		StartAfterSetup:  false,
		RedisHost:        "localhost",
		RedisPort:        "6379",
		ServerPort:       "8080",
		Elasticsearch:    "http://localhost:9200",
	}

	err := runSelfHostedInstall(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "license invalid")
	assert.False(t, downloadCalled, "download should not run when license verify fails")

	serverBin := filepath.Join(tmpDir, "bin", "server")
	workerBin := filepath.Join(tmpDir, "bin", "worker")
	assert.NoFileExists(t, serverBin)
	assert.NoFileExists(t, workerBin)
}
