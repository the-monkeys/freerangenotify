# FreeRangeNotify — Go Example

## Prerequisites

- Go 1.21+
- A FreeRangeNotify API key (get one from the [dashboard](https://freerangenotify.monkeys.support))

## Run

```bash
export FRN_API_KEY="frn_your_api_key_here"
cd examples/golang
go run main.go
```

## What's Covered

| Function | Description |
|----------|-------------|
| `sendNotification()` | Send a single notification via email |
| `sendBulkNotifications()` | Send push notifications to multiple users |
| `listNotifications()` | Paginated list of recent notifications |
| `sendOTP()` | Send a one-time passcode via SMS |
| `verifyOTP()` | Verify a received OTP code |
| `uploadFile()` | Upload an invoice/file via multipart form |
| `quickSend()` | Simplified send using email or external ID |

## Notes

- The example uses only the Go standard library (no external dependencies).
- All HTTP calls go through an `X-API-Key` header for authentication.
- Adjust `baseURL` if you're running a self-hosted instance.
