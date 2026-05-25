# File Attachments

Attach files (PDFs, images, videos, documents) to a notification using one of three input modes. Pick the one that fits your use case; FRN resolves them all to the same internal payload before delivery.

| Source           | Field            | Best for                                                              |
| ---------------- | ---------------- | --------------------------------------------------------------------- |
| Public URL       | `url`            | The file already lives on a CDN, S3 bucket, or public web server.     |
| Inline base64    | `content_base64` | One-shot small files (≤ 10 MB decoded). No upload round-trip.         |
| FRN-managed file | `file_id`        | Files > 10 MB **or** reused across many notifications (e.g. invoices).|

> Exactly **one** of the three must be set per attachment. The API rejects ambiguous payloads with `400 attachment_ambiguous_source`.

---

## Limits & defaults

| Limit                          | Default value      | Override key                              |
| ------------------------------ | ------------------ | ----------------------------------------- |
| Max upload size                | 50 MiB             | `filestore.max_bytes`                     |
| Max inline base64 (encoded)    | ~14 MB             | _hardcoded; ~10 MB decoded_               |
| Retention before auto-delete   | 30 days            | `filestore.retention_days` (`-1` = never) |
| Signed-URL TTL                 | 15 minutes         | `filestore.signed_url_ttl_seconds`        |
| Allowed MIME types             | common image/doc   | `filestore.allowed_mime_types` (wildcards)|

---

## 1. Public URL (simplest)

Send any URL FRN can reach. The worker fetches the bytes once (cap 25 MB, timeout 20 s) and forwards them to the provider.

```bash
curl -X POST http://localhost:8080/v1/notifications \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id":  "u_123",
    "channel":  "email",
    "template": "monthly_report",
    "content": {
      "title": "Your May report",
      "body":  "See the attached PDF.",
      "attachments": [{
        "type": "file",
        "url":  "https://cdn.example.com/reports/2026-05.pdf",
        "name": "report-2026-05.pdf",
        "mime_type": "application/pdf"
      }]
    }
  }'
```

## 2. Inline base64

Best for one-shot small attachments where you already have the bytes in memory.

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

Accepts both standard (`+/=`) and URL-safe (`-_`) base64 and strips a leading `data:application/pdf;base64,` prefix if present.

## 3. Upload-then-reference (`file_id`) — recommended for invoices, contracts, large files

This is the recommended path for files larger than ~10 MB or for any file you will reference more than once.

### Step 1 — Upload

```bash
curl -X POST http://localhost:8080/v1/files \
  -H "X-API-Key: $API_KEY" \
  -F "file=@invoice-2026-05.pdf;type=application/pdf"
```

Response (`201 Created`):

```json
{
  "file_id":    "f_01HXYZ...",
  "name":       "invoice-2026-05.pdf",
  "size":       182437,
  "mime_type":  "application/pdf",
  "sha256":     "9c1185a5...",
  "expires_at": "2026-06-24T20:14:00Z",
  "created_at": "2026-05-25T20:14:00Z"
}
```

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
      "file_id": "f_01HXYZ...",
      "name":    "invoice-2026-05.pdf"
    }]
  }
}
```

### Pinned (never-expire) files

For files that must outlive the default 30-day retention (legal contracts, archived invoices), set `filestore.retention_days: -1` for the tenant. Newly uploaded files will be pinned and `expires_at` will be omitted from their metadata.

---

## Files endpoints

| Method   | Path                              | Auth      | Purpose                                  |
| -------- | --------------------------------- | --------- | ---------------------------------------- |
| `POST`   | `/v1/files`                       | API key   | Upload (multipart `file` field).         |
| `GET`    | `/v1/files`                       | API key   | List tenant files (`?limit=&offset=`).   |
| `GET`    | `/v1/files/:id`                   | API key   | File metadata.                           |
| `DELETE` | `/v1/files/:id`                   | API key   | Delete bytes + metadata.                 |
| `GET`    | `/v1/files/:id/content`           | API key   | Stream the bytes.                        |
| `GET`    | `/v1/files/:id/download-url`      | API key   | Mint a short-lived signed URL.           |
| `GET`    | `/v1/files/download/:id`          | Signature | Public download (signature in query).    |

---

## Inline images in email

Use `disposition: "inline"` plus a `content_id` to embed images in the HTML body via `cid:` references.

```jsonc
{
  "attachments": [{
    "type":        "image",
    "file_id":     "f_logo123",
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

## Channel support

> Per-channel native binary delivery is being rolled out. The API accepts all three input modes today; until per-provider wiring lands, `url` is the most reliable source across every channel. Track the rollout in the implementation plan.

| Channel       | URL          | Inline base64 | `file_id`    | Notes                                              |
| ------------- | ------------ | ------------- | ------------ | -------------------------------------------------- |
| Email         | ✓            | ✓             | ✓            | Inline (cid:) supported via `disposition: inline`. |
| WhatsApp      | ✓            | (via upload)  | (via upload) | Meta requires media upload before send.            |
| Slack         | ✓            | ✓             | ✓            | Native `files.uploadV2` for binary.                |
| Discord       | ✓            | ✓             | ✓            | Multipart upload.                                  |
| Webhook       | ✓            | ✓             | ✓            | Caller chooses passthrough vs. multipart.          |
| Push (APNs/FCM)| Image URL only | —          | —            | Payload size cap (≤ 4 KB).                         |
| SMS / In-App / SSE | —      | —             | —            | Channel cannot physically carry binaries.          |

---

## Error reference

| HTTP | Domain error                  | Meaning                                                  |
| ---- | ----------------------------- | -------------------------------------------------------- |
| 400  | `attachment_missing_source`   | None of `url` / `content_base64` / `file_id` supplied.   |
| 400  | `attachment_ambiguous_source` | More than one source supplied on the same attachment.    |
| 400  | `attachment_too_large`        | Inline base64 > ~14 MB encoded.                          |
| 404  | `file_not_found`              | `file_id` doesn't exist or belongs to another tenant.    |
| 410  | `file_expired`                | `file_id` was past its retention window.                 |
| 413  | `file_too_large`              | Multipart upload exceeded `filestore.max_bytes`.         |
| 415  | `unsupported_mime_type`       | MIME type not in `filestore.allowed_mime_types`.         |
