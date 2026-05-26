# Sending File Attachments

FreeRangeNotify supports three ways to attach files (PDFs, images, videos,
documents) to a notification:

| Source           | Field            | When to use                                                       |
|------------------|------------------|-------------------------------------------------------------------|
| Public URL       | `url`            | The file already lives on a CDN / S3 / public web server.         |
| Inline base64    | `content_base64` | Small files (≤10 MB decoded). One-shot, no server round-trip.     |
| FRN-managed file | `file_id`        | Large files (>10 MB) or files reused across many notifications.   |

Exactly **one** of the three must be set per attachment. Sending more than one,
or none, is rejected with `400 attachment_ambiguous_source` /
`400 attachment_missing_source`.

## Per-channel support

| Channel             | `url` | `content_base64` | `file_id` | Notes |
|---------------------|:-----:|:----------------:|:---------:|-------|
| email (SMTP)        | ✅    | ✅               | ✅        | Verified end-to-end against Gmail on 2026-05-26 for all three modes. Inline (`disposition: "inline"` + `content_id`) supported via multipart/related. |
| email (SES / SendGrid / Mailgun / Postmark / Resend) | ✅ | ✅ | ✅ | Code merged in `c7ac895` (P1a); unit-tested per vendor; not yet exercised against a vendor sandbox. |
| webhook             | ✅    | ✅               | ✅        | Bytes are POSTed in the configured shape (passthrough or rich); see webhook docs. |
| WhatsApp / Slack / Discord / Teams / MMS / Push | ⚠️ | ⚠️ | ⚠️ | Wiring tracked in `documents/plans/FILE_ATTACHMENTS_PLAN.md` §0; today these channels accept the input but the worker has not yet been updated per provider to consume bytes. Use `url` until the row for your channel reads DONE. |
| SMS / in-app / SSE  | ❌    | ❌               | ❌        | Fail fast with `ErrChannelUnsupportedAttachment` — these transports cannot carry binaries. |

## Limits

| Limit                                  | Default value      | How to change                              |
|----------------------------------------|--------------------|--------------------------------------------|
| Max upload size                        | 50 MiB             | `filestore.max_bytes` (bytes)              |
| Max inline base64 (encoded)            | ~14 MB             | hardcoded; encoded → ~10 MB decoded         |
| Retention before auto-delete           | 30 days            | `filestore.retention_days` (`-1` = pin forever) |
| Signed-URL TTL                         | 15 minutes         | `filestore.signed_url_ttl_seconds`         |
| Allowed MIME types                     | PDF; JPEG/PNG/GIF/WebP; MP3/OGG/WAV; MP4; CSV/TXT; ZIP; DOCX/XLSX/PPTX | `filestore.allowed_mime_types` (wildcards supported, e.g. `image/*`) |

## Authentication

All `/v1/files` and `/v1/notifications` endpoints require a tenant API key in
the `X-API-Key` header. For backward compatibility, the same key is also
accepted via `Authorization: Bearer <key>` (this is what most SDKs send). The
only endpoint that does **not** require a key is the public signed download
`GET /v1/files/download/:id?sig=...` — the signature in the query string is
the authentication.

```http
X-API-Key: frn_live_xxx
```

---

## 1. Public URL (simplest)

```http
POST /v1/notifications
Content-Type: application/json
X-API-Key: frn_live_xxx

{
  "user_id":  "u_123",
  "channel":  "email",
  "template": "monthly_report",
  "content": {
    "title": "Your May report",
    "body":  "See the attached PDF.",
    "attachments": [
      {
        "type": "file",
        "url":  "https://cdn.example.com/reports/2026-05.pdf",
        "name": "report-2026-05.pdf",
        "mime_type": "application/pdf"
      }
    ]
  }
}
```

The worker fetches the URL (cap: 25 MB, timeout: 20 s), forwards the bytes to
the provider, and records the SHA-256 against the notification for audit.

## 2. Inline base64

Best for one-shot small attachments where you already have the bytes in memory
and don't want a separate upload.

```jsonc
{
  "attachments": [{
    "type": "file",
    "content_base64": "JVBERi0xLjQKJ...",
    "name": "receipt.pdf",
    "mime_type": "application/pdf"
  }]
}
```

The payload accepts both standard (`+/=`) and URL-safe (`-_`) base64 and
strips any leading `data:application/pdf;base64,` prefix.

## 3. Upload-then-reference (`file_id`) — recommended for invoices, contracts, large files

This is the recommended path for files larger than ~10 MB or for any file you
will reference more than once.

### Step 1 — Upload

```http
POST /v1/files
Content-Type: multipart/form-data; boundary=...
X-API-Key: frn_live_xxx

(multipart field "file" — binary)
```

Response (`201 Created`):

```json
{
  "file_id":    "file_5a6d19a25b6240c0bc0a17445ef5f5be",
  "name":       "invoice-2026-05.pdf",
  "size":       182437,
  "mime_type":  "application/pdf",
  "sha256":     "9c1185a5...",
  "expires_at": "2026-06-24T20:14:00Z",
  "created_at": "2026-05-25T20:14:00Z"
}
```

`file_id` is the format `file_<32 lowercase hex>` and is opaque to the caller.
The response also includes the server-computed `sha256` so clients can verify
the upload was not corrupted in transit.

### Step 2 — Reference in a notification

```jsonc
{
  "user_id": "u_123",
  "channel": "email",
  "template": "invoice_email",
  "content": {
    "title": "Invoice for May 2026",
    "body":  "Your invoice is attached.",
    "attachments": [{
      "type":    "file",
      "file_id": "file_5a6d19a25b6240c0bc0a17445ef5f5be",
      "name":    "invoice-2026-05.pdf"
    }]
  }
}
```

### Pinned (never-expire) files

For files that must outlive the default 30-day retention (e.g. legal
contracts, archived invoices), set `filestore.retention_days: -1` for the
tenant — newly uploaded files will be pinned and `expires_at` will be omitted
from their metadata.

### Other file endpoints

| Method   | Path                              | Purpose                                                          |
|----------|-----------------------------------|------------------------------------------------------------------|
| `GET`    | `/v1/files`                       | List the tenant's files (`?limit=&offset=`).                     |
| `GET`    | `/v1/files/:id`                   | File metadata.                                                   |
| `DELETE` | `/v1/files/:id`                   | Permanently delete bytes + metadata.                             |
| `GET`    | `/v1/files/:id/content`           | Stream the bytes back to an authenticated caller.                |
| `GET`    | `/v1/files/:id/download-url`      | Mint a signed URL for unauthenticated download (default 15 min). |
| `GET`    | `/v1/files/download/:id?...`      | Public signed download (no API key; signature is the auth).      |

---

## Inline images in email

Use `disposition: "inline"` and a `content_id` to embed images directly in the
HTML body via `cid:` references.

```jsonc
{
  "attachments": [{
    "type":        "image",
    "file_id":     "file_5a6d19a25b6240c0bc0a17445ef5f5be",
    "content_id":  "logo",
    "disposition": "inline",
    "mime_type":   "image/png"
  }]
}
```

In the HTML template body:

```html
<img src="cid:logo" alt="Logo">
```

---

## SDK examples

### Go

```go
import "github.com/the-monkeys/freerangenotify/sdk/go/freerangenotify"

client := freerangenotify.New("frn_live_xxx")

// Upload once
f, _ := os.Open("invoice-2026-05.pdf")
defer f.Close()
obj, err := client.Files.Upload(ctx, freerangenotify.UploadFileParams{
    Name:     "invoice-2026-05.pdf",
    MIMEType: "application/pdf",
    Reader:   f,
})

// Send by reference
_, err = client.Notifications.Send(ctx, freerangenotify.SendParams{
    UserID:   "u_123",
    Channel:  "email",
    Template: "invoice_email",
    Content: &freerangenotify.Content{
        Title: "Invoice for May 2026",
        Body:  "Your invoice is attached.",
        Attachments: []freerangenotify.ContentAttachment{{
            Type:   "file",
            FileID: obj.FileID,
            Name:   obj.Name,
        }},
    },
})
```

### JavaScript / TypeScript

```ts
import { FreeRangeNotify } from '@freerangenotify/sdk';

const client = new FreeRangeNotify('frn_live_xxx');

// Upload once (Node 18+ or modern browser)
const pdf = await fs.promises.readFile('invoice-2026-05.pdf');
const obj = await client.files.upload({
  data: pdf,
  name: 'invoice-2026-05.pdf',
  mime_type: 'application/pdf',
});

// Send by reference
await client.notifications.send({
  user_id: 'u_123',
  channel: 'email',
  template: 'invoice_email',
  content: {
    title: 'Invoice for May 2026',
    body:  'Your invoice is attached.',
    attachments: [{ type: 'file', file_id: obj.file_id, name: obj.name }],
  },
});
```

---

## Error reference

| HTTP | Domain error                  | Meaning                                                  |
|------|-------------------------------|----------------------------------------------------------|
| 400  | `attachment_missing_source`   | None of `url` / `content_base64` / `file_id` supplied.   |
| 400  | `attachment_ambiguous_source` | More than one source supplied on the same attachment.    |
| 400  | `attachment_too_large`        | Inline base64 > ~14 MB encoded.                          |
| 404  | `file_not_found`              | `file_id` doesn't exist or belongs to another tenant.    |
| 410  | `file_expired`                | `file_id` was past its retention window.                 |
| 413  | `file_too_large`              | Multipart upload exceeded `filestore.max_bytes`.         |
| 415  | `unsupported_mime_type`       | MIME type not in `filestore.allowed_mime_types`.         |

---

## Verified end-to-end example

The transcript below is the live smoke test that produced row 3 of the
Recipient Evidence Registry in `documents/plans/FILE_ATTACHMENTS_PLAN.md §0.2`.
It exercises the `file_id` mode against the deployed stack (SMTP → Gmail).

```bash
# 1. Upload the binary
curl -sS -X POST \
  -H "X-API-Key: $FRN_API_KEY" \
  -F "file=@invoice-2026-05.pdf;type=application/pdf" \
  https://api.example.com/v1/files/
# => 201
# {"file_id":"file_5a6d19a25b6240c0bc0a17445ef5f5be","sha256":"e781f578...",...}

# 2. Reference it in a notification
curl -sS -X POST \
  -H "X-API-Key: $FRN_API_KEY" \
  -H "Content-Type: application/json" \
  --data @- https://api.example.com/v1/notifications/ <<'JSON'
{
  "user_id":    "c0fa6e2f-691a-4e06-a9f9-07ab4f595a83",
  "channel":    "email",
  "priority":   "normal",
  "template_id":"c3cd9209-5088-44a6-bc68-610c9ce31069",
  "title":      "FRN evidence: file_id mode",
  "body":       "Attachment via file_id.",
  "attachments":[{
    "type":     "file",
    "file_id":  "file_5a6d19a25b6240c0bc0a17445ef5f5be",
    "name":     "invoice-2026-05.pdf",
    "mime_type":"application/pdf"
  }]
}
JSON
# => 202 { "notification_id": "<uuid>", "status": "queued" }
```

The same workflow with `content_base64` (Test 2) or a public `url` (Test 1)
returns `202 queued` and the worker delivers in ~3–5 seconds against Gmail
SMTP. Inline (`content_base64`) is the fastest path because it skips both the
worker's metadata lookup and the disk read.

## Operational notes

### Local `FileStore` backend requires a shared volume

When `filestore.backend: local` (the default), the API and worker processes
**must** see the same directory at `filestore.local_root`. The bundled
`docker-compose.yml` does this by declaring a named docker volume
(`frn_files`) and mounting it on both `notification-service` and
`notification-worker` at `/home/app/data/files`. If you deploy the two
services on separate hosts with `backend: local`, every `file_id` attachment
will fail in the worker with `attachment resolver: file <id>: file not found`
even though the upload returned `201 Created` (the file was written to the
API host's disk and the worker host can't see it).

For multi-host deployments use `filestore.backend: s3` (and configure the S3
credentials) so the bytes live in object storage and both services read from
the same source.

### Quotas, retention, and pinned files

- `filestore.retention_days: 30` is the default. Files past that age are
  swept by the background cleaner.
- `filestore.retention_days: -1` pins every newly uploaded file forever and
  omits `expires_at` from the metadata response. Use this for invoices,
  contracts, and other artefacts you must retain for compliance.
- Disk usage per tenant is tracked in Elasticsearch (`files` index, scoped
  by `app_id`); per-tenant quotas are on the roadmap and not yet enforced.

### Multi-tenancy

All file queries are scoped by `app_id`. A `file_id` belonging to another
tenant returns `404 file_not_found` (never `403`) so tenant existence cannot
be probed.

