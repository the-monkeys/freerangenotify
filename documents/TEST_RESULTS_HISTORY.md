# Test Results History (Dec 3, 2025)

This document consolidates both manual and automated integration test results from the Dec 3 testing cycle.

## Summary
- **Automated Tests**: 93% PASS (57/61)
- **Manual API Tests**: 83% PASS (15/18 tested)
- **Overall Status**: **Excellent** core functionality, with documented issues in Template filtering.

---

## 🟢 Automated Integration Results
- **Application APIs**: 100% Pass (14/14)
- **User APIs**: 100% Pass (15/15)
- **Notification APIs**: 93% Pass (14/15) - SMS provider not configured.
- **Template APIs**: 82% Pass (14/17) - Known app_id filtering issues.

### Automated Key Fixes Applied:
- Fixed status code expectations (202 Accepted for async).
- Fixed response parsing (direct objects instead of wrapped responses).
- Added validation error handling for common DTO failures.

---

## 🔵 Manual API Verification Results
- **Resource IDs created**:
  - App: `e6ec11d1-3bd1-4922-b23b-1891dbfc0166`
  - User: `718805b7-a6fb-46d3-80bf-10ac3ed704f0`
  - Template: `60b1f26b-e3a7-4850-8919-59a2b3341f0b`

### Manual Test Highlights:
- ✅ Application creation with API key generation.
- ✅ Notification queuing verified via GET endpoints.
- ✅ Template rendering with variable substitution working perfectly.

---

## 🐛 Documented Issues

### 1. Template Repository - app_id Filter
- **Severity**: HIGH
- **Problem**: `GET /v1/templates?app_id={id}` returns 0 templates even when they exist.
- **Root Cause**: Match query on keyword field in `template_repository.go`.
- **Solution**: Use `TermQuery` for exact match on `.keyword` field.

### 2. CreateVersion Design Conflict
- **Severity**: MEDIUM
- **Question**: Should `CreateVersion` allow body modification or just copy the current body?
- **Status**: Backend currently ignores the body sent in the DTO.

### 3. Device API Field Mismatch
- **Severity**: LOW
- **Resolution**: Documentation updated to use `token` and `platform` instead of `device_token` and `device_type`.

---

*For detailed historical context, see the original reports (now consolidated).*
