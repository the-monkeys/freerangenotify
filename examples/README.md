# FreeRangeNotify API — Integration Examples

Ready-to-use client code for integrating with the [FreeRangeNotify](https://freerangenotify.monkeys.support) notification platform.

## Quick Start

1. **Sign up** at [freerangenotify.monkeys.support](https://freerangenotify.monkeys.support)
2. **Create an Application** from the dashboard — you'll receive an **API key**
3. **Copy** the example code for your language and replace the placeholder API key

## Available Languages

| Language | Directory | Min Version |
|----------|-----------|-------------|
| Go       | [`golang/`](./golang/) | Go 1.21+ |
| Python   | [`python/`](./python/) | Python 3.8+ |
| Java     | [`java/`](./java/)     | Java 11+ |
| JavaScript / Node.js | [`javascript/`](./javascript/) | Node.js 18+ |
| C++      | [`cpp/`](./cpp/)       | C++17, libcurl |
| Rust     | [`rust/`](./rust/)     | Rust 2021 |
| Ruby     | [`ruby/`](./ruby/)     | Ruby 3.0+ |

## API Base URL

```
https://freerangenotify.monkeys.support/v1
```

## Authentication

All API requests require an **API key** sent as a header:

```
X-API-Key: frn_your_api_key_here
```

API keys are scoped to applications. You can manage them in the dashboard under
**Applications → Settings → API Key**.

## Covered Operations

Each example demonstrates:

| Operation | Endpoint | Method |
|-----------|----------|--------|
| Send Notification | `/v1/notifications` | POST |
| Send Bulk Notifications | `/v1/notifications/bulk` | POST |
| List Notifications | `/v1/notifications` | GET |
| Get Notification | `/v1/notifications/:id` | GET |
| Send OTP | `/v1/otp/send` | POST |
| Verify OTP | `/v1/otp/verify` | POST |
| Upload File (Invoice) | `/v1/files` | POST |
| List Files | `/v1/files` | GET |
| Quick Send | `/v1/quick-send` | POST |

## Support

- **API Docs**: [freerangenotify.monkeys.support/docs](https://freerangenotify.monkeys.support/docs)
- **GitHub Issues**: [github.com/the-monkeys/freerangenotify/issues](https://github.com/the-monkeys/freerangenotify/issues)
