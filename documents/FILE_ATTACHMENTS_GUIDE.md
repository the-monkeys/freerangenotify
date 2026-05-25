# Sending File Attachments

FreeRangeNotify supports three ways to attach files (PDFs, images, videos,
documents) to a notification:

| Source           | Field            | When to use                                                       |
|------------------|------------------|-------------------------------------------------------------------|
| Public URL       | `url`            | The file already lives on a CDN / S3 / public web server.         |
| Inline base64    | `content_base64` | Small files (≤10 MB decoded). One-shot, no server round-trip.     |
| FRN-managed file | `file_id`        | Large files (>10 MB) or files reused across many notifications.   |

Exactly **one** of the three must be set per attachment. Sending more than one,
or none, is rejected with `400 Bad Request`.

> The capability matrix per channel (which channels accept which sources) is
> driven by provider capabilities exposed by the worker. Today's wiring is in
> progress — the API accepts all three sources; the worker is being updated
> per provider to consume them. Until then, the safest source for every
> channel is `url`.

## Limits

| Limit                                  | Default value      | How to change                              |
|----------------------------------------|--------------------|--------------------------------------------|
| Max upload size                        | 50 MiB             | `filestore.max_bytes`                      |
| Max inline base64 (encoded)            | ~14 MB             | hardcoded; encoded → ~10 MB decoded         |
| Retention before auto-delete           | 30 days            | `filestore.retention_days` (`-1` = never)  |
| Signed-URL TTL                         | 15 minutes         | `filestore.signed_url_ttl_seconds`         |
| Allowed MIME types                     | common image/doc   | `filestore.allowed_mime_types` (wildcards) |

## Authentication

All `/v1/files` endpoints require a tenant API key
(`Authorization: Bearer frn_...`) **except** the public download path
`GET /v1/files/download/:id`, which is authenticated by the signature in its
query string.

---

## 1. Public URL (simplest)

```http
POST /v1/notifications
Content-Type: application/json
Authorization: Bearer frn_live_xxx

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
Authorization: Bearer frn_live_xxx

(multipart field "file" — binary)
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
