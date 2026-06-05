# FreeRangeNotify — JavaScript / Node.js Example

## Prerequisites

- Node.js 18+ (uses native `fetch`, no external dependencies)
- A FreeRangeNotify API key

## Run

```bash
export FRN_API_KEY="frn_your_api_key_here"
cd examples/javascript
node main.mjs
```

## What's Covered

| Function | Description |
|----------|-------------|
| `sendNotification()` | Send a single notification via email |
| `sendBulkNotifications()` | Send push notifications to multiple users |
| `listNotifications()` | Paginated list of recent notifications |
| `sendOTP()` | Send a one-time passcode via SMS |
| `verifyOTP()` | Verify a received OTP code |
| `uploadFile()` | Upload an invoice/file via multipart FormData |
| `listFiles()` | List uploaded files |
| `quickSend()` | Simplified send using email or external ID |

## Notes

- **Zero external dependencies** — uses Node.js 18+ native `fetch` and `FormData`.
- Uses ESM (`.mjs`). If your project uses CommonJS, rename to `.js` and adjust imports.
