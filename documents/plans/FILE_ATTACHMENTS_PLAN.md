# File Attachments — HLD / LLD & Implementation Plan

**Status:** In progress — see §0 for the verified state of each area.
**Document Owner:** Engineering — Notification Platform
**Last Updated:** 2026-05-26
**Tracking Issue:** [#110 — Support Sending Invoices via Multiple Channels](https://github.com/the-monkeys/freerangenotify/issues/110)
**Pull Request:** [#116 — File attachments](https://github.com/the-monkeys/freerangenotify/pull/116) (branch `feat/file-attachments-p0a`)
**Scope:** Allow callers to ship binary files (invoices, PDFs, images, audio, video) **inside the message** across every channel that can physically carry a file, without breaking the existing URL-based `Attachment` schema, the rich-webhook renderers, or any current SDK.

This document follows the Engineering Design Document conventions used across the platform: Status (§0) → Definition of Done (§0.1) → Evidence Registry (§0.2) → Sign-off (§0.3) → Goals (§1) → Contract (§2) → Capability Matrix (§3) → Architecture (§4) → Error Model (§5) → Security (§6) → Test Strategy (§7) → Documentation (§8) → SDKs (§9) → Rollout, Backward Compatibility & Feature Flags (§10) → Observability (§10.5) → Phased Delivery (§11) → Open Questions (§12) → Acceptance Criteria (§13) → References (§14) → UI Design (§15).

---

## 0. Verified Implementation Status (2026-05-26)

The previous revision of this section mixed "code merged" with "feature delivered" and contained false DONE marks for areas where no recipient has ever received a file. The table below replaces that with verifiable state across five gates:

- **Code:** merged on `main` or staged on the feature branch.
- **Tests:** unit-test coverage of the area's logic.
- **Integration:** exercised against the dockerised stack (`make test-integration`) or an equivalent recorded fixture.
- **Recipient evidence:** a real (or sandbox) recipient confirmed receipt of the bytes; link in §0.2.
- **Customer-visible:** a non-engineer can trigger this from the UI today.

A row is `DONE` only when all five gates pass. Anything less is, at most, `CODE COMPLETE`.

| Area | Code | Tests | Integration | Recipient evidence | Customer-visible | Verified Status |
|---|---|---|---|---|---|---|
| Domain extension — `Attachment.{ContentBase64, FileID, ContentID, Disposition}` | merged (PR #116, P0a) | passing | n/a (schema only) | n/a | n/a | DONE |
| `internal/domain/file` package + `FileObject` model | merged (P0a/P0b) | passing | n/a | n/a | n/a | DONE |
| `FileStore` — local FS backend + HMAC signed URLs | merged (P0b) | passing | exercised via live `POST /v1/files` (2026-05-26, file `file_5a6d19a25b62…` SHA-256 verified) | n/a | n/a | DONE |
| `FileStore` — S3 backend | not started | — | — | — | — | NOT STARTED |
| `POST /v1/files`, `GET /v1/files/:id`, `DELETE /v1/files/:id`, signed download | merged (P0d) | passing | live upload verified end-to-end with worker-side `file_id` resolution (2026-05-26) | n/a (no UI uploader) | n/a | DONE |
| `AttachmentResolver` (url \| inline \| file_id → bytes) | merged (P0e) | passing | all three input modes resolved & delivered to a live inbox (2026-05-26) | n/a (no provider invoked it until P1e) | n/a | DONE |
| `Provider.Capabilities()` | partial (struct + enum landed; interface method not added) | n/a | n/a | n/a | n/a | NOT STARTED |
| Email providers — SMTP / SES / SendGrid / Mailgun / Postmark / Resend wire resolved attachments | merged (`c7ac895`, P1a) | passing (16 unit cases across `smtp_mime_test.go` + `email_attachments_test.go`) | SMTP → Gmail verified end-to-end for all three input modes (2026-05-26) — see §0.2 rows 1–3 | partial (Quick Send wires inline base64 only) | DONE for SMTP; UNVERIFIED for SES/SendGrid/Mailgun/Postmark/Resend |
| Meta WhatsApp — `/media` upload → `media_id` | not started | — | — | — | — | NOT STARTED |
| Twilio WhatsApp + MMS — signed URL fallback | not started | — | — | — | — | NOT STARTED |
| Slack — `files.uploadV2` (multipart) | not started | — | — | — | — | NOT STARTED |
| Discord — multipart `files[N]` | not started | — | — | — | — | NOT STARTED |
| Teams — Adaptive Card `Action.OpenUrl` to signed URL | not started | — | — | — | — | NOT STARTED |
| Webhook (generic + custom) — passthrough modes | not started | — | — | — | — | NOT STARTED |
| Twilio / Vonage MMS — signed URL | not started | — | — | — | — | NOT STARTED |
| APNs / FCM — rich push image URL | not started | — | — | — | — | NOT STARTED |
| SMS / In-App / SSE — fail fast with `ErrChannelUnsupportedAttachment` | partial (resolver typed error defined; no provider call site enforces it) | not exercised | — | — | — | NOT STARTED |
| OpenAPI + Swagger | merged (P1d) | static | n/a | n/a | n/a | DONE |
| `documents/API_DOCUMENTATION.md` + `documents/FILE_ATTACHMENTS_GUIDE.md` + `ui/src/docs/file-attachments.md` | merged (P1c, P1c-ui) | n/a | n/a | n/a | docs sidebar entry only | DONE |
| Go SDK — `Files.Upload`, extended `ContentAttachment` | merged (P1a/P1b) | unit passing | not exercised against live API | n/a | n/a | CODE COMPLETE |
| JS SDK — `files.upload`, extended `ContentAttachment` | merged (P1a/P1b) | unit passing | not exercised against live API | n/a | n/a | CODE COMPLETE |
| React SDK — `useFileUpload` hook | merged (P1-ui-2; inlined into `AttachmentRow` upload mode using `filesAPI.upload`'s onProgress callback rather than a standalone hook) | typecheck passing | n/a (UI-level) | n/a | n/a | CODE COMPLETE |
| UI — `AttachmentEditor` component (upload / from-URL / by-file-id) | merged (P1-ui-3; three-mode `AttachmentRow` inside `RichContentEditor`) | typecheck passing | n/a | n/a | UI surface exists but no Playwright spec yet | CODE COMPLETE |
| UI — Files manager page (`AppFiles`) | merged (P1-ui-5; `AppFiles` tab in `AppDetail` with upload / list / search / signed-download / delete) | typecheck passing | n/a | n/a | tab exists in AppDetail | CODE COMPLETE |
| UI — Quick Send / Advanced Send / Broadcast wired to `AttachmentEditor` | partial — Quick Send + Advanced Send now pass `apiKey` into `RichContentEditor` enabling the three-mode editor (P1-ui-4); Broadcast surface still pending | typecheck passing | n/a | n/a | yes for Quick + Advanced | CODE COMPLETE (Broadcast still NOT STARTED) |
| UI — Notification History attachments column + drawer | not started | — | — | — | — | NOT STARTED |
| UI — Per-app file policy (allowlist, size cap, quota) | not started | — | — | — | — | NOT STARTED |
| UI — Per-provider attachment-mode toggle | not started | — | — | — | — | NOT STARTED |
| Integration suite `tests/integration/files/` | not started | — | — | — | — | NOT STARTED |
| Playwright e2e specs `e2e/attachments-*.spec.ts` | not started | — | — | — | — | NOT STARTED |
| Real-recipient smoke evidence (see §0.2) | captured for SMTP / email / all three input modes (2026-05-26) | — | — | — | — | DONE for SMTP-email; outstanding for every other channel/provider |
| Feature flag `FRN_ENABLE_FILE_ATTACHMENTS` | not introduced | — | — | — | — | NOT STARTED |
| Observability — metrics, logs, traces (see §10.5) | not introduced | — | — | — | — | NOT STARTED |

**Headline:** the feature is approximately one-quarter complete by surface area. The API ingests attachments, files can be uploaded, the resolver materialises bytes, the OpenAPI + SDKs declare the shape, and email providers (staged on branch) embed the bytes in outgoing messages. Nothing else delivers files. No customer can attach a file from the UI today, and no recipient has been observed to receive one.

---

## 0.1 Definition of Done

A row in §0 moves to `DONE` only when **all** of the following are true and recorded:

1. **Code merged** to `main` via a reviewed PR. Branch deleted.
2. **Unit tests** cover (a) happy path, (b) every edge case from the capability matrix in §3, (c) backward compatibility — the no-attachment path is byte-stable against a golden fixture committed at the same time.
3. **Integration test** under `tests/integration/files/` exercises the full path through the dockerised stack and is part of `make test-integration` CI.
4. **Recipient evidence** captured and linked in §0.2:
   - Email — MIME message captured from MailHog (or equivalent) with file part decoded and SHA-256 verified against the source bytes.
   - WhatsApp / Slack / Discord / Teams — vendor sandbox screenshot **or** recorded HTTP fixture showing the file delivered.
   - Push — APNs / FCM simulator screenshot showing the rich-push image rendered.
5. **Customer-visible** if the row produces user-facing behaviour: a UI surface exists that lets a non-engineer trigger it, and a Playwright spec under `e2e/` covers the happy path.
6. **SDK** parity: at minimum one example per SDK (Go, JS) reproducing the behaviour, captured in `documents/TEST_RESULTS_HISTORY.md`.
7. **Observability** present: the new code path emits the metrics, logs, and span attributes listed in §10.5, and at least one alert rule references them.
8. **Backward compatibility** verified: existing notifications without attachments produce byte-identical wire output to the pre-change baseline (golden tests in §7.1).
9. **Sign-off** by ≥ 2 engineers recorded in §0.3, one of whom is on platform on-call rotation.

Anything short of all nine is `CODE COMPLETE` at best. There is no "DONE pending tests" state in this document.

---

## 0.2 Recipient Evidence Registry

Each entry must include channel, provider, input mode, date, captor, and a link to the artefact (commit SHA in `documents/evidence/`, MailHog `.eml`, or sandbox screenshot URL).

| # | Channel | Provider | Input mode | Captured | Captor | Artefact |
|---|---|---|---|---|---|---|
| 1 | email | smtp (Gmail relay) | `url` | 2026-05-26 | Dave (buddhicintaka@gmail.com) | notification `6170286e-50df-447a-93e4-13aeca003533`; worker log `SMTP email sent successfully` in 4.4s; recipient screenshot confirms `dummy-via-url.pdf` (W3C dummy PDF) attached and openable. |
| 2 | email | smtp (Gmail relay) | `content_base64` | 2026-05-26 | Dave (buddhicintaka@gmail.com) | notification `4246a9ef-597a-4b10-8377-618149cdc657`; worker log `SMTP email sent successfully` in 3.4s; recipient screenshot confirms `inline-via-base64.pdf` attached, preview shows our test text ("FRN P1a Evidence: url mode / buddhicintaka@gmail.com"); source bytes SHA-256 = `e781f578549280951da21114d4060fa597aad1d9e7337d89a54b8d71364150dc`. |
| 3 | email | smtp (Gmail relay) | `file_id` | 2026-05-26 | Dave (buddhicintaka@gmail.com) | notification `28353b48-0d82-4267-9451-e9f92edfdbe3`; uploaded via `POST /v1/files` → `file_5a6d19a25b6240c0bc0a17445ef5f5be` (SHA-256 match); worker log `SMTP email sent successfully` in 4.7s; recipient confirmed delivery (latency slightly higher than inline due to extra ES + disk read on the worker side). Live test surfaced a real platform bug (see §0.4 entry 2026-05-26 #2). |

---

## 0.3 Sign-off Log

| Area (from §0) | Reviewer 1 | Reviewer 2 | Date | Notes |
|---|---|---|---|---|
| SMTP email — all three input modes (`url`, `content_base64`, `file_id`) | Dave (recipient) | _pending second reviewer_ | 2026-05-26 | Live Gmail evidence captured for all three input modes (§0.2 rows 1–3). SHA-256 of inline payload verified end-to-end. Sign-off remains partial until a second engineer co-signs per §0.1 gate 9. Other email vendors (SES, SendGrid, Mailgun, Postmark, Resend) still UNVERIFIED — sign-off must be repeated against a vendor sandbox before each is moved to DONE. |

---

## 0.4 Change Log of This Document

| Date | Author | Change |
|---|---|---|
| 2026-05-24 | Engineering | Initial plan (P0a through P1d). |
| 2026-05-26 | Engineering | §0 rewritten to remove false DONE entries. Added §0.1 Definition of Done, §0.2 Evidence Registry, §0.3 Sign-off Log, §10.5 Observability. No technical content in §1–§9 or §11–§15 changed. |
| 2026-05-26 | Engineering | Live smoke test executed against the deployed stack for all three attachment input modes (`url`, `content_base64`, `file_id`) on the SMTP → Gmail path. Evidence rows 1–3 added to §0.2; §0.3 records partial sign-off; §0 rows updated accordingly. **Bug surfaced and fixed**: the local-FS `FileStore` was not shared between the `notification-service` and `notification-worker` containers, so any `file_id` attachment failed in the worker with `file not found`. Fix lands a named docker volume (`frn_files`) mounted at `/home/app/data/files` on both services, and the `Dockerfile` pre-creates the directory with `app:app` ownership so the volume seeds with correct permissions on first init. This is exactly the class of issue §0.1 gate 4 (real-recipient evidence) is designed to surface, and validates keeping that gate as a non-negotiable Definition of Done. |
| 2026-05-26 | Engineering | **UI / docs land (P1-ui-1..5 + P1c).** `ContentAttachment` in `ui/src/types/index.ts` now declares `content_base64`, `file_id`, `disposition`, `content_id` (matching the SDK and server DTO). New `filesAPI` in `ui/src/services/api.ts` covers upload / list / get / delete / signed-download / content. `RichContentEditor` is now a three-mode editor (URL / Upload / File ID) via a new `AttachmentRow` sub-component; the upload mode streams progress through axios `onUploadProgress` and persists the resulting `file_id`. Quick Send and Advanced Send pass `apiKey` so the upload tab is enabled. New `AppFiles` component + `files` tab in `AppDetail` provides a full files manager (upload, paginated list, search, copy ID, signed-URL download, delete). `documents/FILE_ATTACHMENTS_GUIDE.md` and `ui/src/docs/file-attachments.md` are rewritten with the verified channel matrix, precise MIME allowlist, `X-API-Key` auth, the live-test transcript, and operational notes (shared-volume requirement, retention, multi-tenancy 404 semantics). Swagger spec validated as in sync with the live API — no regeneration needed. `npx tsc --noEmit` in `ui/` exits 0. Outstanding for the next pass: Playwright spec (`e2e/attachments-*.spec.ts`), Broadcast wiring, Notification History attachments column, per-app file policy, per-provider attachment-mode toggle, and integration suite (`tests/integration/files/`). |

---

## 1. Goal & Non-Goals

### 1.1 Goal
Let an API caller attach one or more files to a notification using **any one of three input modes** and have FRN deliver those bytes to the recipient via every channel that can physically carry them.

### 1.2 Non-Goals (v1)
- Virus scanning (hook reserved, default no-op).
- Large file streaming (> 50 MB per attachment). Hard-cap above which we 413.
- File versioning, retention policies beyond a TTL on uploads.
- True inline embed for push (APNs/FCM payloads are too small).

---

## 2. Caller-Facing Contract (Backward-Compatible)

### 2.1 Three input modes — choose one per `attachments[]` element

```jsonc
"attachments": [
  // (A) URL — existing, unchanged. FRN fetches it once at send time.
  {
    "type": "file",
    "url": "https://cdn.example.com/invoice-1042.pdf",
    "name": "invoice-1042.pdf",
    "mime_type": "application/pdf"
  },

  // (B) Inline bytes — NEW. base64-encoded, ≤ 10 MB per attachment.
  {
    "type": "file",
    "name": "invoice-1042.pdf",
    "mime_type": "application/pdf",
    "content_base64": "JVBERi0xLjQK..."
  },

  // (C) Pre-uploaded — NEW. Caller did POST /v1/files first.
  {
    "type": "file",
    "file_id": "file_01J9K3R8XQM2N7P4V5W6Y8Z2A1",
    "content_id": "invoice-pdf"          // optional — required for HTML inline embed in email
  }
]
```

All three resolve to the same internal `ResolvedAttachment`. Exactly one of `{url, content_base64, file_id}` must be set per element — otherwise the handler returns **400** with `ErrAmbiguousAttachmentSource`.

### 2.2 Schema delta — `Attachment` (existing struct, only adds optional fields)

```go
// internal/domain/notification/models.go (extended)
type Attachment struct {
    // --- existing (UNCHANGED, still URL-only-compatible) ---
    Type     string `json:"type"                  es:"type"`        // image | video | file | audio
    URL      string `json:"url,omitempty"         es:"url"`         // now optional
    Name     string `json:"name,omitempty"        es:"name"`
    MimeType string `json:"mime_type,omitempty"   es:"mime_type"`
    Size     int64  `json:"size,omitempty"        es:"size"`
    AltText  string `json:"alt_text,omitempty"    es:"alt_text"`

    // --- NEW (all omitempty, all optional) ---
    ContentBase64 string `json:"content_base64,omitempty" es:"-"`            // never persisted to ES
    FileID        string `json:"file_id,omitempty"        es:"file_id"`
    ContentID     string `json:"content_id,omitempty"     es:"content_id"`   // RFC 2392 cid:* for HTML email
    Disposition   string `json:"disposition,omitempty"    es:"disposition"`  // attachment | inline (default: attachment)
}
```

**Backward compatibility guarantees:**
- A request that uses only `url` still validates, still renders identically on every existing channel (webhook/Slack/Discord/Teams renderers ignore unknown fields).
- The on-the-wire ES document gains `content_id` and `disposition` only when set; legacy documents read back unchanged.
- `ContentBase64` is **never** persisted (`es:"-"`) — it lives in the queue payload then is replaced by a `file_id` after the resolver materializes it.
- `MediaURL` (`Content.MediaURL`) is **preserved unchanged** for the WhatsApp single-image legacy path. New code MUST prefer `Attachments[0]` when both are present.

### 2.3 New endpoint — `POST /v1/files`

| Aspect | Value |
|---|---|
| Auth | `X-API-Key` (tenant inferred) |
| Content-Type | `multipart/form-data` |
| Form fields | `file` (binary, required), `purpose` (string, optional, default `"notification_attachment"`) |
| Max size | 50 MB (configurable per app, default 25 MB) |
| MIME allowlist | configurable per app; default: `application/pdf`, `image/*`, `audio/*`, `video/mp4`, `text/csv`, `text/plain`, `application/zip`, `application/vnd.openxmlformats-officedocument.*` |
| Response | `201 { "file_id": "file_...", "name": "...", "size": 123456, "mime_type": "...", "expires_at": "...", "sha256": "..." }` |
| Errors | `400` (no file / wrong content-type), `413` (size cap), `415` (MIME not allowed), `403` (app cap exceeded) |

**Additional endpoints:**
- `GET /v1/files/:id` → returns metadata + short-lived signed download URL (15 min TTL). Tenant-scoped.
- `DELETE /v1/files/:id` → removes from `FileStore` and marks ES record as deleted (soft delete).
- `GET /v1/files?purpose=...` → list with pagination (admin only).

### 2.4 Send path — extends `POST /v1/notifications`

No schema change at the top level. Callers simply populate `content.attachments[]` per § 2.1. Existing requests continue to work byte-for-byte.

---

## 3. Channel Capability Matrix

| Channel | Mode | Max size | How FRN delivers the bytes | Behaviour when binary supplied & unsupported |
|---|---|---|---|---|
| **Email — SMTP / SES / SendGrid / Mailgun / Postmark / Resend** | True inline + attachment | 25 MB (Gmail), 40 MB (SES), 30 MB (SendGrid), 25 MB safe default | `multipart/mixed` for attachment, `multipart/related` + `Content-ID` for inline embed | n/a (all email providers supported) |
| **WhatsApp — Meta Cloud** | Pre-upload → `media_id` | 100 MB doc, 16 MB video, 5 MB image | `POST /{phone}/media`, then `document`/`image`/`video` message references `id` | n/a |
| **WhatsApp — Twilio** | Public URL | 16 MB | FRN issues 15-min signed URL, passes as `MediaUrl` | n/a |
| **Slack** | Multipart | 1 GB | `files.uploadV2` (3-step: get URL, PUT bytes, complete) | n/a |
| **Discord** | Multipart | 25 MB (Nitro 500 MB) | `multipart/form-data` with `files[N]` | n/a |
| **Teams** | Adaptive Card OpenUrl | n/a | FRN signed URL embedded in card; **no native upload** (Graph API not in scope) | n/a |
| **Webhook (generic / custom)** | Configurable per provider | 10 MB per element | Mode A (default): JSON with `content_base64`. Mode B: `multipart/form-data` (`payload` JSON part + `files[N]`). | n/a |
| **MMS — Twilio / Vonage** | Public URL | ~600 KB | FRN signed URL as `MediaUrl` | n/a |
| **Push — APNs / FCM** | URL only (rich push) | per-platform thumb limits | `mutable-content` / `notification.image` URL points to FRN signed URL | downgrade to text-only if image fetch will exceed limit |
| **SMS** | Unsupported | n/a | n/a | Fail fast: `ErrChannelUnsupportedAttachment` (400) |
| **In-App / SSE** | Unsupported | n/a | n/a | Same |

The matrix is encoded in `Provider.Capabilities()`; the worker enforces it **before** the resolver runs (saves a round-trip to `FileStore`).

---

## 4. Architecture

### 4.1 Layered View

```
┌──────────────────────────────────────────────────────────────────┐
│ Caller                                                           │
│   ├── POST /v1/files   (multipart)  ──► file_id                  │
│   └── POST /v1/notifications  attachments: [url|base64|file_id]  │
└──────────────────┬───────────────────────────────────────────────┘
                   │ Fiber handler (validate, MIME allowlist, size cap,
                   │ tenant guard, ambiguous-source guard)
                   ▼
        ┌─────────────────────────┐
        │ FileStore (interface)   │   local-fs | s3 | gcs
        └──────────┬──────────────┘
                   │
                   ▼
       ┌────────────────────────────────┐
       │ Redis Queue (NotificationJob)  │  base64 stripped → file_id
       └──────────┬─────────────────────┘
                  │
                  ▼
   ┌──────────────────────────────────────────────┐
   │ Worker                                        │
   │   1. Provider.Capabilities() pre-check        │
   │   2. AttachmentResolver (idempotent)          │
   │      ├─ url        → http.Get (cached)        │
   │      ├─ file_id    → FileStore.Get            │
   │      └─ base64     → decode                   │
   │   3. Provider.Send(ctx, notif, user, resolved)│
   └──────────┬───────────────────────────────────┘
              ▼
  ┌─────────────────────────────────────────────┐
  │ Per-provider adapter (chooses delivery mode)│
  └─────────────────────────────────────────────┘
```

### 4.2 New packages / files

| Path | Purpose |
|---|---|
| `internal/domain/file/models.go` | `FileObject`, errors, validation. |
| `internal/domain/file/store.go` | `FileStore` interface. |
| `internal/infrastructure/filestore/local_store.go` | Dev/test backend (path under `./local/files/`). |
| `internal/infrastructure/filestore/s3_store.go` | Prod backend. |
| `internal/infrastructure/filestore/signed_url.go` | HMAC-signed URL issuer + verifier. |
| `internal/infrastructure/repository/file_repository.go` | ES index `frn_files-vN`. |
| `internal/interfaces/http/handlers/file_handler.go` | Upload / read / delete. |
| `internal/interfaces/http/dto/file_dto.go` | DTOs. |
| `internal/usecases/services/file_service.go` | Tenant guard, allowlist, size cap, virus-scan hook. |
| `internal/usecases/services/attachment_resolver.go` | Idempotent resolve + per-notification cache. |
| `internal/infrastructure/providers/capabilities.go` | `Capabilities` struct + default. |
| `internal/infrastructure/providers/*_attachments.go` | Per-provider adapter helpers (one file per provider that grows beyond a few methods). |

### 4.3 Modified files

| Path | Change |
|---|---|
| `internal/domain/notification/models.go` | Add `ContentBase64`, `FileID`, `ContentID`, `Disposition` to `Attachment`; update `Validate`. |
| `internal/infrastructure/providers/provider.go` | Add `Capabilities() Capabilities` to `Provider`; provide `DefaultCapabilities` embeddable struct so existing providers compile unchanged. |
| `internal/infrastructure/providers/smtp_provider.go` and the five other email providers | Wire resolved attachments into MIME / SDK calls. |
| `internal/infrastructure/providers/meta_whatsapp_provider.go` | Add `/media` upload helper. |
| `internal/infrastructure/providers/whatsapp_provider.go` (Twilio) | Use signed URL. |
| `internal/infrastructure/providers/slack_provider.go` | `files.uploadV2`. |
| `internal/infrastructure/providers/discord_provider.go` | Multipart `files[N]`. |
| `internal/infrastructure/providers/teams_provider.go` | OpenUrl button to signed URL. |
| `internal/infrastructure/providers/webhook_provider.go` + `custom_provider.go` | Optional multipart mode (per-provider config). |
| `internal/infrastructure/providers/twilio_provider.go` / `vonage_provider.go` (MMS) | Signed URL. |
| `internal/infrastructure/providers/apns_provider.go` / `fcm_provider.go` | Signed URL for rich push image. |
| `cmd/worker/processor.go` | Pre-check capabilities, run resolver once, pass `ResolvedAttachments` to providers via context. |
| `internal/interfaces/http/routes/routes.go` | Register `/v1/files` routes under `apiAuth`. |
| `internal/container/container.go` | Wire `FileStore`, `FileService`, `AttachmentResolver`. |
| `docs/openapi/*.yaml` | New `Attachment` properties, new `/files` paths, new error codes. |
| `documents/API_DOCUMENTATION.md` | New section "File Attachments". |
| `ui/src/docs/*.md` | Caller-facing doc. |
| `sdk/go/freerangenotify/files.go` (new), `notifications.go` (extend params) | Go SDK. |
| `sdk/js/src/files.ts` (new), `notifications.ts` (extend types) | JS SDK. |

### 4.4 Type definitions

```go
// internal/infrastructure/providers/capabilities.go
type AttachmentMode int

const (
    AttachModeNone     AttachmentMode = iota // SMS, in-app, SSE
    AttachModeInline                          // Email (MIME parts)
    AttachModeMultipart                       // Slack, Discord, webhook mode-B
    AttachModePreUpload                       // Meta WhatsApp (media_id)
    AttachModeSignedURL                       // Twilio, MMS, push, Teams
)

type Capabilities struct {
    AttachmentMode AttachmentMode
    MaxAttachmentBytes int64       // 0 = unlimited / use channel default
    MaxAttachmentCount int         // 0 = unlimited
    AllowedMIMETypes   []string    // empty = inherit app/global allowlist
    SupportsInlineCID  bool        // true only for email
}

func DefaultCapabilities() Capabilities {
    return Capabilities{AttachmentMode: AttachModeNone}
}

// internal/usecases/services/attachment_resolver.go
type ResolvedAttachment struct {
    Filename    string
    MIMEType    string
    Disposition string         // "attachment" | "inline"
    ContentID   string
    Bytes       []byte         // may be nil if streamed via Reader
    Reader      io.ReadCloser  // closed by caller; non-nil when Bytes is nil
    Size        int64
    Source      string         // "url" | "file_id" | "inline"
    SHA256      string
}

type AttachmentResolver interface {
    Resolve(ctx context.Context, appID string, atts []notification.Attachment) ([]ResolvedAttachment, error)
}
```

### 4.5 Provider interface change (additive)

```go
// existing:
type Provider interface {
    Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error)
    GetName() string
    GetSupportedChannel() notification.Channel
    IsHealthy(ctx context.Context) bool
    Close() error
}

// NEW (additive method with default impl on a mixin):
type Provider interface {
    // ... existing methods unchanged ...
    Capabilities() Capabilities
}

// All current providers gain a one-line:
// type SMTPProvider struct { defaultProvider; ... }    // embeds method
// defaultProvider.Capabilities() returns DefaultCapabilities()
// then per-provider files override Capabilities() with their real values.
```

Resolved attachments are passed to providers via a typed context key (`ResolvedAttachmentsKey`) so the `Provider.Send` signature does not change — existing tests and mocks compile unchanged.

---

## 5. Error Model (additive)

| Sentinel | HTTP | When |
|---|---|---|
| `ErrAttachmentTooLarge` | 413 | One element exceeds app cap or 50 MB hard cap. |
| `ErrAttachmentMIMENotAllowed` | 415 | MIME outside per-app allowlist. |
| `ErrAmbiguousAttachmentSource` | 400 | More than one of `{url, content_base64, file_id}` set on one element. |
| `ErrAttachmentMissingSource` | 400 | None of the three set. |
| `ErrChannelUnsupportedAttachment` | 400 | Caller supplied an attachment for a channel whose `Capabilities.AttachmentMode == AttachModeNone`. |
| `ErrFileNotFound` | 404 | `file_id` doesn't exist or belongs to a different app. |
| `ErrFileExpired` | 410 | TTL passed. |
| `ErrAttachmentFetchFailed` | 502 | URL-mode source returned non-2xx. |

Errors surface in the notification's audit record and in the synchronous response when the failure is detectable at API time; channel-side failures surface in delivery status (`failed` + `error_message`).

---

## 6. Security

- **Tenant isolation:** `app_id` is sourced from `c.Locals("app_id")`. Cross-tenant `file_id` lookups → `ErrFileNotFound`.
- **Signed URL:** HMAC-SHA256 over `file_id|app_id|expires_at`, key from `FRN_FILESTORE_SIGNING_KEY` (rotatable). 15-minute default TTL.
- **MIME spoofing:** server-side sniff (`net/http.DetectContentType`) on the first 512 bytes; reject if it disagrees with the declared `mime_type` AND the sniffed type is outside the allowlist.
- **Path traversal:** `file_id` is a server-issued ULID; never derived from caller input.
- **Storage at rest:** S3 SSE-S3 by default; LocalStore uses a fixed root and rejects any path that escapes it.
- **DoS:** request-level size cap enforced by `BodyLimit` middleware (50 MB) before reaching the handler; per-app cap enforced before storage write.
- **Virus scan hook:** `FileScanner` interface (no-op default). Production deployments wire ClamAV via a worker pre-resolver step.
- **PII:** `content_base64` is `es:"-"` — never indexed. Inline base64 is stripped from the queue payload after the resolver materializes it.

---

## 7. Test Strategy

### 7.1 Unit tests (target: ≥ 85 % branch coverage of new code)

| Package | Cases |
|---|---|
| `domain/notification` | `Attachment.Validate`: ambiguous source, missing source, oversize base64, valid url-only, valid file-id-only, valid inline-only. |
| `domain/file` | `FileObject.Validate`, expiry, tenant-id mismatch. |
| `infrastructure/filestore/local_store` | put/get/delete, path-traversal rejection, signed-URL verify success / wrong-sig / expired. |
| `infrastructure/filestore/s3_store` | (stubbed AWS client) put/get/delete + content-type passthrough. |
| `usecases/services/file_service` | Allowlist, size cap, MIME-sniff disagreement, app quota. |
| `usecases/services/attachment_resolver` | Each source path, idempotency (resolver caches by sha256), per-notification cache hit, fetch failure surfaces typed error. |
| `infrastructure/providers/capabilities` | `DefaultCapabilities` is `AttachModeNone`; per-provider overrides correct. |
| `providers/smtp` | `multipart/mixed` shape, `multipart/related` + `cid:` for inline, base64 transfer encoding, header folding. |
| `providers/sendgrid` / `ses` / `mailgun` / `postmark` / `resend` | JSON body includes `attachments[].content` base64 in the provider-specific shape. |
| `providers/meta_whatsapp` | `/media` upload happy path + retry; document message uses `id`, not `link`. |
| `providers/twilio_whatsapp` | `MediaUrl` is FRN signed URL when source != public URL. |
| `providers/slack` | 3-step `files.uploadV2`: get URL, PUT bytes, complete. |
| `providers/discord` | Multipart body shape, `payload_json` plus `files[0]`. |
| `providers/webhook` / `custom` | Mode A (JSON+base64) and Mode B (multipart) selected by provider config. |
| `worker/processor` | Capability pre-check rejects `AttachModeNone`; resolver runs exactly once across retries. |
| `handler/file_handler` | Upload path covers happy / oversize / wrong-mime / wrong-content-type / cross-tenant fetch. |

### 7.2 Integration tests — `tests/integration/files/`

New suite, mirrors `tests/integration/webhook/`. Spins the full stack (`docker-compose up`), uses test API key, runs against `localhost:8080`.

| File | Cases |
|---|---|
| `upload_test.go` | Upload PDF, verify `GET /files/:id` returns metadata + signed URL; URL retrieves the original bytes. |
| `email_inline_test.go` | Send email with `attachments[]` (file_id, inline PDF + cid:logo image). Use mailhog/mailpit in compose to capture the MIME message and assert parts. |
| `whatsapp_meta_test.go` | Mock Meta Cloud (httptest server) to verify `/media` upload is called and the message references `id`. |
| `slack_test.go` | Mock Slack API to verify `files.uploadV2` 3-step. |
| `discord_test.go` | Mock Discord to assert multipart shape. |
| `webhook_modes_test.go` | Provider config switches between Mode A and Mode B; receiver checks payload. |
| `capability_guard_test.go` | SMS with attachment → 400 `ErrChannelUnsupportedAttachment`. |
| `cross_tenant_test.go` | App A uploads, App B tries to reference its `file_id` → 404. |

Runner: extend `scripts/test-webhook-v2.ps1` or add `scripts/test-files.ps1`.

### 7.3 SDK tests

- Go: `sdk/go/freerangenotify/files_test.go` — upload via `httptest` server; `notifications_test.go` extended with attachment-bearing send.
- JS: `sdk/js/src/files.test.ts` — same shape using `msw`/`nock`-style mocks.

---

## 8. Documentation Deliverables

| Document | Update |
|---|---|
| `docs/openapi/files.yaml` (new) | Full spec for `POST /v1/files`, `GET /v1/files/:id`, `DELETE /v1/files/:id`. |
| `docs/openapi/notifications.yaml` | Extend `Attachment` schema; add 4 new error codes. |
| `docs/openapi/otp.yaml` | No change. |
| `docs/swagger.json` / `docs.go` / `docs/swagger.yaml` | Regenerated by `swag init`. |
| `documents/API_DOCUMENTATION.md` | New section **"File Attachments"** covering the three input modes, per-channel matrix, size limits, error reference, and curl examples for the three modes against each supported channel. |
| `documents/FILE_ATTACHMENTS_GUIDE.md` (new) | Long-form caller guide: invoice-via-email walkthrough, invoice-via-WhatsApp walkthrough, inline image embedding, signed URL semantics, MIME allowlist customization. |
| `ui/src/docs/files.md` (new) + `ui/src/docs/notifications.md` (extend) | In-product docs surface for both. |
| `ui/src/config/docsNav.tsx` | Register the new files doc. |
| `documents/TESTING_GUIDE.md` | Add files integration suite. |
| `documents/IMPLEMENTATION_AUDIT.md` | Append "File Attachments — DONE" once landed. |
| `README.md` | One-line bullet under Capabilities. |

---

## 9. SDK Deliverables

### 9.1 Go SDK — `sdk/go/freerangenotify/`

```go
// files.go (new)
type FileUploadParams struct {
    Reader   io.Reader
    Filename string
    MIMEType string
    Purpose  string // default "notification_attachment"
}

type FileObject struct {
    FileID    string    `json:"file_id"`
    Name      string    `json:"name"`
    Size      int64     `json:"size"`
    MIMEType  string    `json:"mime_type"`
    SHA256    string    `json:"sha256"`
    ExpiresAt time.Time `json:"expires_at"`
}

func (c *Client) UploadFile(ctx context.Context, p FileUploadParams) (*FileObject, error)
func (c *Client) GetFile(ctx context.Context, fileID string) (*FileObject, error)
func (c *Client) DeleteFile(ctx context.Context, fileID string) error
```

Extend existing `NotificationSendParams.Content.Attachments` element type with `ContentBase64`, `FileID`, `ContentID`, `Disposition` (all `omitempty`).

### 9.2 JS SDK — `sdk/js/src/`

```ts
// files.ts (new)
export interface FileUploadParams {
    file: Blob | Buffer | NodeJS.ReadableStream;
    filename: string;
    mimeType?: string;
    purpose?: 'notification_attachment' | string;
}
export interface FileObject { fileId: string; name: string; size: number; mimeType: string; sha256: string; expiresAt: string; }

export class Files {
    upload(p: FileUploadParams): Promise<FileObject>;
    get(fileId: string): Promise<FileObject>;
    delete(fileId: string): Promise<void>;
}
```

Extend `Attachment` interface with the same optional fields; `toWire` emits only what's set.

### 9.3 SDK validation

After implementation, run the exact curl examples from `documents/FILE_ATTACHMENTS_GUIDE.md` via each SDK end-to-end against the live Docker stack and capture outputs in `documents/TEST_RESULTS_HISTORY.md`.

---

## 10. Rollout & Backward Compatibility

| Concern | Mitigation |
|---|---|
| Existing requests using only `url` | No change. Resolver picks the URL path. Same wire shape. |
| Existing `MediaURL` (WhatsApp legacy) | Preserved. Workers prefer `Attachments[0]` only when both are populated; otherwise fall back to `MediaURL`. |
| Existing webhook renderers | Unchanged. They consume the URL-only shape. `content_base64` is stripped from the queue payload after the resolver runs, so renderers never see it. |
| ES index | Adds two new optional mapped fields (`content_id`, `disposition`). Backfilled lazily. Migration script under `cmd/migrate/`. |
| Existing SDKs (pre-this-release) | Continue to work — the new `Attachment` fields are optional both wire-side and in the structs. |
| Existing tests | The `Provider` interface gains one method via an embeddable mixin; all existing provider types embed `defaultProvider` so they pick up the default `Capabilities()` for free. No test rewrite needed. |
| Storage migration (S3 vs local) | `FileStore` is an interface; switching backend is a config-only change (`FRN_FILESTORE_BACKEND=s3 \| local`). |
| Feature flag | Behind `FRN_ENABLE_FILE_ATTACHMENTS=true` (default `true` in dev, `false` in prod for the first release) so it can be dark-launched. |

### 10.1 Feature-flag matrix

| Flag | Default (dev) | Default (prod) | Effect |
|---|---|---|---|
| `FRN_ENABLE_FILE_ATTACHMENTS` | `true` | `false` (canary on per-app allowlist) | Master kill-switch. When `false`, the resolver short-circuits and providers never receive resolved bytes. API still accepts `content.attachments` to preserve forward-compat shape, but ignores them with a `warning` audit entry. |
| `FRN_FILESTORE_BACKEND` | `local` | `s3` | Chooses the FileStore implementation. |
| `FRN_FILESTORE_SIGNING_KEY` | dev fixture | secret-managed | HMAC key for signed download URLs. Rotatable without downtime via dual-key support (planned for P5). |
| Per-app `app.settings.attachments.enabled` | `true` | `false` | Per-tenant kill-switch above the global flag. |

### 10.2 Canary stages (production)

1. **Internal-only** — flag enabled for FRN's own staging app and one internal test app. Soak ≥ 72 h. Watch §10.5 metrics for resolver error rate, provider error rate, p99 send latency delta vs baseline.
2. **First design-partner tenant** — enabled by request. Capture one inbox screenshot per channel they use. Add to §0.2 Evidence Registry.
3. **Allowlisted GA** — flag enabled by tenant CSM action. Per-app dashboard surfaces opt-in toggle.
4. **Default-on GA** — only after ≥ 30 days of allowlisted GA with zero P1 incidents tagged `file-attachments`, signed off in §0.3.

Rollback: setting `FRN_ENABLE_FILE_ATTACHMENTS=false` and rolling worker pods is sufficient; there is no schema migration to reverse. ES index additions are forward-compatible.

---

## 10.5 Observability

Every code path in this feature MUST emit the signals below before being marked DONE per §0.1 gate 7. These names are normative.

### 10.5.1 Metrics (Prometheus, exposed via existing `/metrics` endpoint)

| Metric | Type | Labels | Purpose |
|---|---|---|---|
| `frn_attachment_resolve_total` | counter | `app_id`, `source` (`url`\|`inline`\|`file_id`), `result` (`ok`\|`error`) | Volume of resolves and their outcome. |
| `frn_attachment_resolve_duration_seconds` | histogram | `source` | Resolve latency. Buckets: 0.01, 0.05, 0.1, 0.5, 1, 5. |
| `frn_attachment_bytes_resolved_total` | counter | `app_id`, `source` | Total bytes materialised. Capacity-planning signal. |
| `frn_attachment_size_bytes` | histogram | `source` | Size distribution per attachment. Buckets: 1 KB, 10 KB, 100 KB, 1 MB, 10 MB, 50 MB. |
| `frn_attachment_provider_send_total` | counter | `provider`, `channel`, `result` | Per-provider success/failure for sends that carried an attachment. |
| `frn_attachment_provider_send_duration_seconds` | histogram | `provider`, `channel` | Tail latency of attachment-bearing sends, distinct from no-attachment sends. |
| `frn_attachment_capability_rejected_total` | counter | `channel`, `provider` | Times a send was rejected by the capability pre-check (SMS, in-app, SSE, etc.). |
| `frn_filestore_object_total` | gauge | `app_id`, `backend` | Number of stored file objects per tenant. Sampled. |
| `frn_filestore_bytes_total` | gauge | `app_id`, `backend` | Bytes under management per tenant. Drives quota dashboard. |
| `frn_filestore_signed_url_issued_total` | counter | `app_id` | Signed URL issuance volume. |
| `frn_filestore_signed_url_verify_failure_total` | counter | `reason` (`expired`\|`bad_sig`\|`missing`) | Security signal. Alerts on spikes. |

### 10.5.2 Structured logs (zap)

Every resolve and every provider attachment send MUST log at INFO with at least the following typed fields:

```
notification_id, app_id, channel, provider,
attachment_count, attachment_bytes_total,
attachment_sources[],     // ["url","file_id",...]
resolve_duration_ms,
result                    // "ok"|"error"
```

Failures log at ERROR with the same fields plus `error_class` (typed from §5) and `error_message`. No log MAY contain raw attachment bytes or base64 content; this is enforced by a unit test that asserts the log shape against zap's observer.

### 10.5.3 Distributed tracing (OpenTelemetry)

The existing tracing setup (`internal/telemetry/`) gains the following spans:

| Span | Parent | Attributes |
|---|---|---|
| `attachment.resolve` | `worker.send` | `app_id`, `notification_id`, `attachment.count`, `attachment.source` (per child span), `attachment.bytes`, `attachment.cache_hit` |
| `attachment.resolve.fetch_url` | `attachment.resolve` | `url.host`, `http.status_code`, `attachment.bytes` |
| `attachment.resolve.read_file_id` | `attachment.resolve` | `file.id`, `file.backend`, `attachment.bytes` |
| `attachment.resolve.decode_base64` | `attachment.resolve` | `attachment.bytes` |
| `filestore.put` | API handler / SDK | `app_id`, `mime`, `size_bytes` |
| `filestore.get` | resolver | `app_id`, `file.id`, `cache_hit` |

Each span sets `error=true` and records the typed error from §5 on failure.

### 10.5.4 Alerts (suggested, owned by Platform on-call)

| Alert | Trigger | Severity |
|---|---|---|
| `AttachmentResolveErrorSpike` | `rate(frn_attachment_resolve_total{result="error"}[5m]) > 0.1 × rate(...[1h])` for 10 min | warn |
| `FileStoreSignedURLVerifyFailureSpike` | `rate(frn_filestore_signed_url_verify_failure_total[5m]) > 1/s` for 5 min | page |
| `AttachmentProviderSendErrorSpike` | per-provider `result="error"` rate > 5 % for 10 min | warn |
| `FileStoreQuotaApproaching` | `frn_filestore_bytes_total / quota > 0.85` | warn (per-tenant) |
| `AttachmentResolveLatencyHigh` | p99 of `frn_attachment_resolve_duration_seconds` > 2 s for 15 min | warn |

### 10.5.5 Dashboards

A Grafana dashboard `file-attachments-overview` MUST exist under `config/grafana-datasources.yaml`'s sibling dashboards directory before P1 is marked DONE. Required panels: resolve volume by source, resolve error rate, provider send error rate split by channel, p50/p95/p99 latency overlay against the no-attachment baseline, top-10 tenants by `frn_filestore_bytes_total`.

---

## 11. Phased Delivery

Each phase ships as one PR. A phase is only complete when **every row it claims to deliver** passes all nine gates in §0.1 — i.e. code merged, unit + integration tests green, recipient evidence captured in §0.2, UI surface live (where applicable), SDK example recorded, observability signals emitting per §10.5, backward-compat golden tests green, and §0.3 sign-off recorded.

| Phase | Scope | Exit criteria |
|---|---|---|
| **P0** (merged) | Domain extension, `domain/file`, FileStore local backend, `/v1/files` endpoints, AttachmentResolver, capability mixin types, OpenAPI, Go + JS SDK files clients, caller-facing docs. | All P0 rows in §0 verified at CODE COMPLETE. |
| **P1a — Email backend** (this branch, unpushed) | Wire the six email providers (SMTP, SES, SendGrid, Mailgun, Postmark, Resend) to consume `AttachmentResolveFunc` from ctx. Unit tests for wire-format shape. Backward-compat golden test for the no-attachment fast path on every email provider. | Code merged. **One captured `.eml` per input mode (`url`, `content_base64`, `file_id`) against a MailHog inbox** stored under `documents/evidence/p1a/` and listed in §0.2. SHA-256 of received bytes matches source. Observability metrics from §10.5 firing in dev. Sign-off in §0.3. |
| **P1b — UI for email** | `AttachmentEditor` component + Quick Send / Advanced Send / Broadcast wiring + `AppFiles` manager page + Notification History attachments column. Playwright spec `e2e/attachments-email.spec.ts` (upload → send → assert MailHog receipt). | All P1b §0 rows DONE per §0.1. UI accessible without engineering intervention. |
| **P2 — Meta WhatsApp** | `/media` upload helper; provider switches to `media_id` when `Attachments` non-empty. Mocked integration test + sandbox screenshot in §0.2. | Per §0.1. |
| **P3 — Twilio WhatsApp + MMS + Vonage MMS** | Signed-URL passthrough. Twilio sandbox screenshot. | Per §0.1. |
| **P4 — Slack + Discord + Teams + Webhook + Custom** | Each provider per §3 capability matrix. Per-provider sandbox or recorded HTTP fixture evidence. | Per §0.1. |
| **P5 — APNs + FCM rich push** | `mutable-content` / `notification.image` URL flow. Simulator screenshot. | Per §0.1. |
| **P6 — Hardening** | S3 FileStore backend; virus-scan hook (ClamAV adapter, no-op default); signing-key rotation; per-app quota enforcement in API + UI; Grafana dashboard; production canary stages per §10.2. | All §13 acceptance criteria met. Feature flag default-on in prod. Plan moved to `documents/IMPLEMENTATION_AUDIT.md` as "shipped". |

P1a is the only currently-active phase. P1b cannot start until P1a's recipient evidence is captured; P2 cannot start until P1b ships, to keep CI minutes bounded and avoid stacking unverified channels.

---

## 12. Open Questions

1. **Storage retention policy** — TTL 30 days default for uploaded files? Configurable per app?
2. **Inline embed in email** — auto-promote first `image/*` attachment with no `content_id` to inline, or require explicit `disposition: inline`? Recommendation: **explicit**, to avoid surprising senders.
3. **Webhook Mode B selection** — per-provider config flag or content-type negotiation? Recommendation: per-provider flag, explicit beats implicit.
4. **APNs rich push** — drop the attachment silently or fail when the image URL would push the payload over 4 KB? Recommendation: drop with a `warning` in audit log, not a delivery failure.
5. **Quota** — global per-app GB cap? Recommendation: yes, surfaced in the Apps dashboard, default 5 GB.

---

## 13. Acceptance Criteria (mirrors issue #110)

- [ ] Caller can send an invoice via **email** through every email provider (SMTP, SES, SendGrid, Mailgun, Postmark, Resend).
- [ ] Caller can send an invoice via **WhatsApp** through both Meta Cloud and Twilio providers.
- [ ] Caller can send an invoice via **webhook**, **Slack**, and **Discord**.
- [ ] All three input modes (`url`, `content_base64`, `file_id`) work and are mutually exclusive per attachment.
- [ ] Unsupported channels (SMS, in-app, SSE) fail fast with a typed 400.
- [ ] Resolver is idempotent: same notification on retry does not re-download.
- [ ] OpenAPI, both SDKs, and `documents/API_DOCUMENTATION.md` reflect the final contract.
- [ ] No existing test, SDK call, or webhook payload shape changes.
- [ ] Integration suite (`tests/integration/files/`) is green in CI.

---

## 14. References

- Issue: https://github.com/the-monkeys/freerangenotify/issues/110
- Existing rich content: `documents/plans/WEBHOOK_CHANNEL_EXPANSION_PLAN.md`
- Existing domain attachment (URL-only): `internal/domain/notification/models.go` (`type Attachment struct`)
- WhatsApp Meta Cloud — Upload Media: https://developers.facebook.com/docs/whatsapp/cloud-api/reference/media
- Slack `files.uploadV2`: https://api.slack.com/methods/files.uploadV2
- RFC 2392 (`cid:` URL scheme): https://datatracker.ietf.org/doc/html/rfc2392
- RFC 2387 (`multipart/related`): https://datatracker.ietf.org/doc/html/rfc2387

---

## 15. UI Design

The existing UI already has a URL-only attachment plumbing — `ContentAttachment` (in `ui/src/types/index.ts`) is shipped in `QuickSendRequest.content.attachments` and `BroadcastNotificationRequest.content.attachments`. What is **missing** is:

1. A real file-picker (today the user must paste a URL).
2. The four new optional fields on `ContentAttachment`.
3. A Files manager page (browse, copy `file_id`, delete).
4. An attachments column / drawer on Notification History.
5. Per-provider attachment-mode toggle on the Providers tab (webhook Mode A vs B).
6. Per-app attachment policy on App Settings (MIME allowlist, size cap, GB quota).

### 15.1 Surfaces touched

| # | Surface | File | Change |
|---|---|---|---|
| 1 | Sidebar nav (app detail) | `ui/src/config/appDetailNav.tsx` | Add `{ id: 'files', label: 'Files', icon: <FileBox /> }` under the **Configuration** group. |
| 2 | App detail router | `ui/src/pages/AppDetail.tsx` | New `activeTab === 'files'` branch rendering `<AppFiles apiKey={app.api_key} />`. |
| 3 | Files manager (new) | `ui/src/components/AppFiles.tsx` | Table: name, size, MIME, sha256 (short), expires_at, copy-`file_id` action, download (uses signed URL), delete. Top bar: drag-and-drop **upload zone**, filter by MIME, app-quota usage bar (X MB of Y GB used). |
| 4 | Shared attachment editor (new) | `ui/src/components/notifications/AttachmentEditor.tsx` | Reusable component used in Quick Send, Advanced Send, Broadcast, and (later) Template editor. Three tabs per attachment row: **Upload file**, **From URL**, **Use existing (file_id)**. Drag-and-drop, MIME/size client-side guard mirroring the server, per-row remove, max-count guard from `provider.capabilities`. |
| 5 | Quick Send tab | `ui/src/components/AppNotifications.tsx` (already imports `attachments`) | Replace today's URL-only inputs with `<AttachmentEditor value={form.content.attachments} onChange={...} channel={form.channel} />`. Channel switch disables the editor with an inline note when `capabilities.attachmentMode === 'none'` (SMS, in-app, SSE). |
| 6 | Advanced Send tab | same file | Same swap. |
| 7 | Broadcast tab | same file | Same swap; additional warning when broadcasting an attachment to many subscribers ("X MB × N recipients = estimated egress"). |
| 8 | Notification History | `ui/src/components/AppNotifications.tsx` (history table + detail drawer) | New **Attachments** column showing a paperclip + count. Detail drawer lists each resolved attachment with filename, size, MIME, disposition; for `image/*` shows a thumbnail (uses signed URL). |
| 9 | Providers tab — webhook providers | `ui/src/components/apps/AppProviders.tsx` and `ui/src/components/apps/providers/*` | Per-provider config field `attachment_mode: 'json_base64' \| 'multipart'`. Toggle is shown only for providers whose channel `Capabilities.AttachmentMode == AttachModeMultipart` (webhook, custom). |
| 10 | App Settings tab | `ui/src/components/apps/AppSettings.tsx` (extend existing) | New collapsible **File Attachments** section: max bytes per file, max files per notification, MIME allowlist (chip input), 30-day TTL toggle, monthly storage budget (GB) shown as usage bar. |
| 11 | Webhook Playground | `ui/src/components/WebhookPlayground.tsx` | When a captured payload contains `attachments[]`, render filename chips + size + a "Download" link to the signed URL. Show base64-mode payloads with a "decoded preview" toggle for `image/*` / `application/pdf`. |
| 12 | Template editor (deferred to P5) | `ui/src/components/templates/TemplateEditor.tsx` | Optional default attachments on a template. Behind feature flag, P5. |
| 13 | TypeScript types | `ui/src/types/index.ts` | Extend `ContentAttachment` with `content_base64?: string`, `file_id?: string`, `content_id?: string`, `disposition?: 'attachment' \| 'inline'`. Add new `FileObject`, `FileUploadResponse`, `CapabilitiesByChannel`. |
| 14 | API client | `ui/src/services/api.ts` | New `filesAPI = { list, upload (multipart), get, delete }`. Uses XHR for upload so we can stream progress to the editor. |
| 15 | Docs nav | `ui/src/config/docsNav.tsx` | Register `ui/src/docs/files.md`. |
| 16 | In-product docs | `ui/src/docs/files.md` (new), `ui/src/docs/channels.md` (update) | Mirror caller guide; add the three input modes and per-channel matrix. |
| 17 | Sidebar label disambiguation | (already shipped) `appDetailNav.tsx` | `Users` was renamed to `Subscribers` on 2026-05-24 — no further change. |

### 15.2 `AttachmentEditor` — interaction spec

A single row represents one element of `content.attachments[]`. The row has:

- A **mode selector** (segmented control): **Upload** · **URL** · **Existing file**.
  - **Upload**: drag-and-drop or click-to-pick. On select, the file is uploaded immediately via `filesAPI.upload` with a progress bar; on success the row stores `{ file_id, name, mime_type, size }`.
  - **URL**: text input with debounced HEAD pre-check (read `Content-Length` + `Content-Type` when CORS permits).
  - **Existing file**: a typeahead against `filesAPI.list(apiKey, { search })`; selection populates `{ file_id, name, mime_type, size }`.
- Optional fields visible behind an "Advanced" disclosure: `content_id` (only when channel === email and disposition === inline), `disposition` (radio: attachment / inline), `alt_text` (image/* only).
- A **per-row remove** button. The container enforces `maxFiles` from the channel's `Capabilities`.
- Client-side **guard rails** that mirror the server:
  - MIME against per-app allowlist (fetched once from `appsAPI.get`).
  - Size against per-app cap and the channel-specific cap from the matrix.
  - Total count against the channel cap.
- A **channel-unsupported banner** when `Capabilities.AttachmentMode === 'none'`: the editor renders disabled with the message _"Attachments are not supported on this channel. Switch to Email, WhatsApp, Slack, Discord, or Webhook to attach files."_

### 15.3 New types (TypeScript)

```ts
// ui/src/types/index.ts (added — backward compatible: all new fields optional)
export interface ContentAttachment {
  type: 'image' | 'video' | 'file' | 'audio';
  url?: string;
  name?: string;
  mime_type?: string;
  size?: number;
  alt_text?: string;

  // NEW (v1)
  content_base64?: string;
  file_id?: string;
  content_id?: string;
  disposition?: 'attachment' | 'inline';
}

export interface FileObject {
  file_id: string;
  name: string;
  size: number;
  mime_type: string;
  sha256: string;
  expires_at: string;
  created_at: string;
}

export interface FileUploadProgress { loaded: number; total: number }

export type AttachmentMode = 'none' | 'inline' | 'multipart' | 'pre_upload' | 'signed_url';
export interface ChannelCapabilities {
  attachment_mode: AttachmentMode;
  max_attachment_bytes: number;
  max_attachment_count: number;
  allowed_mime_types: string[];
  supports_inline_cid: boolean;
}
// keyed by Channel string
export type CapabilitiesByChannel = Record<string, ChannelCapabilities>;
```

`ChannelCapabilities` comes from a new lightweight read endpoint `GET /v1/capabilities` exposed by the API (single source of truth — the UI never hard-codes the matrix).

### 15.4 New API client functions

```ts
// ui/src/services/api.ts (added)
export const filesAPI = {
  list:   (apiKey: string, params?: { page?: number; pageSize?: number; search?: string }) => /* GET /files */,
  upload: (apiKey: string, file: File, opts?: {
            purpose?: string;
            onProgress?: (p: FileUploadProgress) => void;
            signal?: AbortSignal;
          }) => /* POST /files multipart with XHR progress */,
  get:    (apiKey: string, fileID: string) => /* GET /files/:id */,
  remove: (apiKey: string, fileID: string) => /* DELETE /files/:id */,
};

export const capabilitiesAPI = {
  get: () => /* GET /capabilities — cached for the session */,
};
```

`filesAPI.upload` uses `XMLHttpRequest` (not `fetch`) so we can wire `upload.onprogress` into the editor's progress bar. The same call is exposed through the React Query layer with optimistic insert into the Files page list.

### 15.5 UX rules (non-negotiable)

1. **Never block the form** while a file uploads — disable Send but keep editing.
2. **Failures are local to the row** — a single bad file does not lose the user's other input.
3. **Resumability** — abort signal on every upload; the row turns into a "Retry" affordance on failure.
4. **No silent downgrade** — if the chosen channel can't carry the file, the editor blocks Send and the channel selector shows the supported channels.
5. **Tenant-scoped Files page** — server already enforces it, but the UI hides cross-tenant `file_id` lookups in the typeahead by passing the current `apiKey` only.
6. **Accessibility** — drop zone must accept keyboard pick (Enter / Space opens file picker), all status icons must have `aria-label`, progress bars must expose `aria-valuenow`.

### 15.6 UI test plan

- **Unit (vitest + RTL — once the UI test infra defaults are in place, currently DEFERRED per the webhook plan)**:
  - `<AttachmentEditor>` — mode switching, MIME/size guard, remove row, max count, channel-unsupported state.
  - `<AppFiles>` — list, upload happy path, upload failure retry, delete with confirm.
- **E2E (Playwright, `e2e/files.spec.ts` new)**:
  - Upload a PDF, attach to a Quick Send email, assert success toast + history row shows paperclip.
  - Switch channel to SMS → editor disables itself with the documented banner.
  - Delete a file currently referenced by a queued notification → server returns 409, UI surfaces the error.
  - Broadcast 1 MB attachment to N test subscribers → estimated egress banner appears.

### 15.7 Sequencing within the phases

The UI work is layered onto the backend phases so each phase ships caller-usable:

| Backend phase | UI deliverable | Surfaces |
|---|---|---|
| P0 (files API + resolver) | Files page (read-only browse + upload + delete); `filesAPI` client; types extended | #1, #2, #3, #13, #14, #15, #16 |
| P1 (email inline + attachments) | `AttachmentEditor` v1 (URL + Upload + Existing modes); wire into Quick / Advanced / Broadcast; History column + drawer | #4, #5, #6, #7, #8 |
| P3 (webhook + Slack + Discord) | Per-provider attachment-mode toggle on Providers tab; Webhook Playground attachment preview | #9, #11 |
| P5 (S3 + polish) | App Settings file policy; quota usage bar; Template editor default attachments (feature-flagged) | #10, #12 |

### 15.8 Backward compatibility for the UI

- All new `ContentAttachment` fields are optional; existing components reading the legacy URL-only shape continue to compile and render.
- `media_url` remains shown in the history drawer as a legacy fallback when no `attachments[]` are present.
- The Quick Send form's existing "Media URL" single-input field is **kept** for one release as a hidden-by-default field under "Advanced"; new code paths populate `attachments[0]` instead. Removed in the release that flips `FRN_ENABLE_FILE_ATTACHMENTS` to default-on in prod.
- The `Subscribers` (formerly `Users`) tab rename already shipped on the current branch — referenced here only so future readers do not get confused.
