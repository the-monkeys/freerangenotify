# Files and attachments

Use your application API key from a trusted backend. Upload with multipart field `file`:

```bash
curl -sS -X POST "$BASE_URL/files" \
  -H "Authorization: Bearer $API_KEY" \
  -F "file=@report.pdf"
```

The response contains `file_id`. Save that ID as `FILE_ID`; it is scoped to this application. The upload cap is the operator-configured value, up to the hard 50 MiB maximum. The reverse proxy must also allow that request size. Retention is deployment-wide; expiry makes a file unavailable logically, while storage deletion follows cleanup. Do not assume a per-tenant retention policy.

Each top-level notification attachment supplies exactly one source: `url`, `content_base64`, or `file_id`. Up to ten attachments are accepted. Use `name`, `mime_type`, and `type` to describe the file. Email inline images also use `disposition: "inline"` and a `content_id` matching the HTML `cid:` reference. URL fetches are bounded to 25 MiB; inline base64 is capped at 14 MiB encoded (about 10 MiB decoded). The normal JSON request cap is 16 MiB. Provider limits may be lower than any of these limits.

## Email with multiple attachments

The following example assumes a previously uploaded `FILE_ID` and registered `USER_ID`. It combines an inline image with a managed file. HTTP 202 accepts the request; the worker later resolves the attachment and sends the email.

## Email attachments

HTTP 202. Response below shows the relevant fields.

<!-- api-example:email-attachments:request -->
```json
{
  "user_id": "${USER_ID}",
  "channel": "email",
  "title": "Report",
  "body": "<img src=\"cid:logo-image\">",
  "attachments": [
    {
      "type": "image",
      "name": "logo.png",
      "mime_type": "image/png",
      "content_base64": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl6uvsAAAAASUVORK5CYII=",
      "content_id": "logo-image",
      "disposition": "inline"
    },
    {
      "type": "file",
      "file_id": "${FILE_ID}",
      "name": "report.pdf"
    }
  ]
}
```

<!-- api-example:email-attachments:curl -->
```bash
curl -sS -X POST "$BASE_URL/notifications" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "user_id": "${USER_ID}",
  "channel": "email",
  "title": "Report",
  "body": "<img src=\"cid:logo-image\">",
  "attachments": [
    {
      "type": "image",
      "name": "logo.png",
      "mime_type": "image/png",
      "content_base64": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl6uvsAAAAASUVORK5CYII=",
      "content_id": "logo-image",
      "disposition": "inline"
    },
    {
      "type": "file",
      "file_id": "${FILE_ID}",
      "name": "report.pdf"
    }
  ]
}
JSON
```

<!-- api-example:email-attachments:response -->
```json
{
  "status": "queued",
  "content": {
    "attachments": [
      {
        "type": "image",
        "name": "logo.png",
        "mime_type": "image/png",
        "content_base64": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl6uvsAAAAASUVORK5CYII=",
        "content_id": "logo-image",
        "disposition": "inline"
      },
      {
        "type": "file",
        "file_id": "${FILE_ID}",
        "name": "report.pdf"
      }
    ]
  }
}
```

## Missing file acceptance

HTTP 202. Response below shows the relevant fields.

<!-- api-example:missing-file-acceptance:request -->
```json
{
  "user_id": "${USER_ID}",
  "channel": "email",
  "title": "Report",
  "body": "See attached",
  "attachments": [
    {
      "type": "file",
      "file_id": "file_missing",
      "name": "missing.pdf"
    }
  ]
}
```

<!-- api-example:missing-file-acceptance:curl -->
```bash
curl -sS -X POST "$BASE_URL/notifications" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "user_id": "${USER_ID}",
  "channel": "email",
  "title": "Report",
  "body": "See attached",
  "attachments": [
    {
      "type": "file",
      "file_id": "file_missing",
      "name": "missing.pdf"
    }
  ]
}
JSON
```

<!-- api-example:missing-file-acceptance:response -->
```json
{
  "status": "queued"
}
```

A missing, expired or cross-application file can be accepted with HTTP 202 and fail asynchronously during resolution. Poll the notification status and inspect its error. Upload a fresh file and issue a new send when appropriate; do not treat queued as proof the attachment exists.

For external links, request `GET /v1/files/{file_id}/download-url` with the application key, then share the returned URL. Keep its `app_id`, `exp` and `sig` query values intact. The URL is a bearer capability until expiry; do not expose your app API key in a download link. URL expiry and the file's own expiry both apply. Custom webhook renderers generate these links automatically for managed files.
