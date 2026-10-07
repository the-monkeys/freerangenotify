package providers

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/textproto"
	"strconv"
	"strings"
)

// DeliveryError retains the cause for errors.Is/As while exposing only safe
// categorical text to history. Err may contain sensitive upstream details;
// persist Error() or SafeDeliveryErrorMessage, never Err.Error().
type DeliveryError struct {
	Provider         string
	ErrorType        string
	Stage            string
	CredentialSource string
	Retryable        bool
	Err              error
}

func (e *DeliveryError) Error() string {
	if e == nil {
		return "notification delivery failed"
	}
	return fmt.Sprintf("%s delivery failed at %s (%s)", safeProviderName(e.Provider), safeFailureStage(e.Stage), safeErrorType(e.ErrorType))
}

func (e *DeliveryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// SafeDeliveryErrorMessage finds typed causes through outer wrappers and avoids
// copying arbitrary upstream text (including wrapper text) into persisted history.
func SafeDeliveryErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	var e *DeliveryError
	if errors.As(err, &e) && e != nil {
		return e.Error()
	}
	return "notification delivery failed"
}

// DeliveryErrorMetadata returns optional, allowlisted history metadata.
// failure_code is the existing ErrorType category, never an upstream body.
func DeliveryErrorMetadata(err error) map[string]interface{} {
	var e *DeliveryError
	if !errors.As(err, &e) || e == nil {
		return nil
	}
	md := map[string]interface{}{"failure_code": safeErrorType(e.ErrorType), "failure_stage": safeFailureStage(e.Stage), "provider": safeProviderName(e.Provider), "retryable": e.Retryable}
	switch e.CredentialSource {
	case CredSourceSystem, CredSourceBYOC, CredSourcePlatform:
		md["credential_source"] = e.CredentialSource
	}
	return md
}

func safeProviderName(value string) string {
	if len(value) == 0 || len(value) > 64 {
		return "provider"
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return "provider"
		}
	}
	return value
}
func safeFailureStage(value string) string {
	switch value {
	case "configuration", "health", "connect", "tls", "auth", "mail", "rcpt", "data", "send", "breaker":
		return value
	}
	return "send"
}
func safeErrorType(value string) string {
	switch value {
	case ErrorTypeNetwork, ErrorTypeAuth, ErrorTypeInvalid, ErrorTypeRateLimit, ErrorTypeProviderAPI, ErrorTypeTimeout, ErrorTypeConfiguration, ErrorTypeUnknown:
		return value
	}
	return ErrorTypeUnknown
}

func newDeliveryError(provider, stage, source, category string, cause error) *DeliveryError {
	var existing *DeliveryError
	if errors.As(cause, &existing) && existing != nil {
		copy := *existing
		// Keep wrappers and joined sibling causes rather than replacing the
		// original chain with only the innermost provider cause.
		copy.Err = cause
		return &copy
	}
	classified := classifyDeliveryCause(provider, cause)
	if category == "" || category == ErrorTypeUnknown || category == ErrorTypeProviderAPI {
		if classified != ErrorTypeUnknown || category == "" {
			category = classified
		}
	}
	category = safeErrorType(category)
	return &DeliveryError{Provider: provider, ErrorType: category, Stage: stage, CredentialSource: source, Retryable: category != ErrorTypeAuth && category != ErrorTypeInvalid && category != ErrorTypeConfiguration, Err: cause}
}

func classifyDeliveryCause(provider string, err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorTypeTimeout
	}
	var network net.Error
	if errors.As(err, &network) {
		if network.Timeout() {
			return ErrorTypeTimeout
		}
		return ErrorTypeNetwork
	}
	if errors.Is(err, context.Canceled) {
		return ErrorTypeNetwork
	}
	var cert x509.UnknownAuthorityError
	var host x509.HostnameError
	var invalid x509.CertificateInvalidError
	if errors.As(err, &cert) || errors.As(err, &host) || errors.As(err, &invalid) {
		return ErrorTypeConfiguration
	}
	// SMTP reply codes share numbers with HTTP statuses but have different
	// terminal/retry semantics. Never parse other providers' text as SMTP.
	if provider != "smtp" {
		return ErrorTypeUnknown
	}
	code := 0
	var smtpErr *textproto.Error
	if errors.As(err, &smtpErr) {
		code = smtpErr.Code
	} else if err != nil {
		// Preserve classification for the legacy injected EmailSender error surface.
		fields := strings.Fields(err.Error())
		if len(fields) > 0 {
			code, _ = strconv.Atoi(fields[0])
		}
	}
	switch code {
	case 530, 534, 535, 538:
		return ErrorTypeAuth
	}
	if code >= 400 && code < 500 {
		return ErrorTypeProviderAPI
	}
	if code >= 500 && code < 600 {
		return ErrorTypeInvalid
	}
	return ErrorTypeUnknown
}
