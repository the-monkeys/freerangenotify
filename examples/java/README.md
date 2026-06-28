# FreeRangeNotify — Java Example

## Prerequisites

- Java 11+ (uses `java.net.http.HttpClient`, no external dependencies)
- A FreeRangeNotify API key

## Compile & Run

```bash
export FRN_API_KEY="frn_your_api_key_here"
cd examples/java
javac FreeRangeNotifyExample.java
java FreeRangeNotifyExample
```

## What's Covered

| Method | Description |
|--------|-------------|
| `sendNotification()` | Send a single notification via email |
| `sendBulkNotifications()` | Send push notifications to multiple users |
| `listNotifications()` | Paginated list of recent notifications |
| `sendOTP()` | Send a one-time passcode via SMS |
| `verifyOTP()` | Verify a received OTP code |
| `uploadFile()` | Upload an invoice/file via multipart form |
| `quickSend()` | Simplified send using email or external ID |

## Notes

- Uses **text blocks** (Java 15+) for JSON literals. If you need Java 11-14 compatibility, replace text blocks with string concatenation.
- Zero external dependencies — uses only the standard library.
