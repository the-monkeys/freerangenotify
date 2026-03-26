# Hybrid SMS Implementation Plan

This document outlines the end-to-end integration of a "Hybrid" SMS service in FreeRangeNotify, allowing users to choose between system-provided Twilio credentials and their own (BYOC).

## Overview

- **System Mode**: Uses `FREERANGE_PROVIDERS_SMS_*` from `.env`. Billed to FreeRangeNotify.
- **BYOC Mode**: Uses per-app `AccountSID`, `AuthToken`, and `FromNumber`. Billed to the user's Twilio account.
- **Priority**: App Settings > System Settings.

---

## Phase 1: Backend Integration

### 1.1 Twilio Provider Finalization
- **File**: `internal/infrastructure/providers/twilio_provider.go`
- **Actions**:
  - Replace the simulated send logic with a real Twilio client implementation.
  - Implement proper error handling (mapping Twilio error codes like `21211` to `ErrorTypeInvalid`).
  - Ensure `credential_source` is correctly tagged as `system` or `byoc` based on the context.

### 1.2 Context Wiring (Worker)
- **File**: `cmd/worker/processor.go` (or wherever `Manager.Send` is called)
- **Actions**:
  - Extract `app.Settings.SMS` from the database.
  - Inject it into the context using `SMSConfigKey` before calling `Manager.Send()`.
  ```go
  if app.Settings.SMS != nil && app.Settings.SMS.AccountSID != "" {
      ctx = context.WithValue(ctx, providers.SMSConfigKey, app.Settings.SMS)
  }
  ```

### 1.3 SMS Send Gate (Security)
- **File**: `cmd/worker/processor.go`
- **Actions**:
  - Similar to WhatsApp, gate system-credential SMS behind phone verification.
  - If `user.PhoneVerified` is false and `credSource == system`, block the send and return `phone_verification_required`.

---

## Phase 2: Frontend Integration

### 2.1 App Settings UI
- **File**: `ui/src/pages/AppDetail.tsx` (or a dedicated settings component)
- **Actions**:
  - Add a "SMS Configuration" section.
  - Include fields for: `Account SID`, `Auth Token`, `From Number`.
  - Add a "Validate Credentials" button that sends a test SMS.
  - Display a badge: "Using System SMS" vs "Using Custom Twilio".

### 2.2 Proactive Gating
- **File**: `ui/src/components/AppNotifications.tsx`
- **Actions**:
  - Update `checkVerificationAndBlock` to also check for `sms` channel sends.
  - Auto-trigger `VerifyPhoneDialog` if the user attempts to send system-SMS without verification.

### 2.3 Template Testing
- **File**: `ui/src/components/templates/TemplateTestPanel.tsx`
- **Actions**:
  - Update `handleSendTest` to include SMS gating logic.

---

## Phase 3: Billing & Metering

### 3.1 Usage Tracking
- **File**: `internal/domain/billing/usage.go`
- **Actions**:
  - Ensure `sms` is a valid `billing_channel`.
  - The `TwilioProvider` already tags `billing_channel: "sms"`, ensuring metrics flow into Elasticsearch automatically.

### 3.2 Calculator Configuration
- **File**: `internal/domain/billing/rates.go`
- **Actions**:
  - Verify `sms` rates (Included Quota, Overage Rate, BYOC Platform Fee) match the product specification.

---

## Verification Plan

### Automated
- **Unit Tests**: Test `TwilioProvider` credential switching logic.
- **Integration Tests**: Send SMS with and without per-app config, verify `credential_source` in ES.

### Manual
- Configure custom Twilio creds in UI, send message, verify it appears in the user's Twilio logs (not FreeRange's).
- Remove custom creds, verify it falls back to system creds (if verified).
- Test unverified account behavior (should trigger modal).
