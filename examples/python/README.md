# FreeRangeNotify — Python Example

## Prerequisites

- Python 3.8+
- `requests` library

## Setup

```bash
cd examples/python
pip install -r requirements.txt
```

## Run

```bash
export FRN_API_KEY="frn_your_api_key_here"
python main.py
```

## What's Covered

| Function | Description |
|----------|-------------|
| `send_notification()` | Send a single notification via email |
| `send_bulk_notifications()` | Send push notifications to multiple users |
| `list_notifications()` | Paginated list of recent notifications |
| `get_notification()` | Get a specific notification by ID |
| `send_otp()` | Send a one-time passcode via SMS/WhatsApp/email |
| `verify_otp()` | Verify a received OTP code |
| `upload_file()` | Upload an invoice/file via multipart form |
| `list_files()` | List uploaded files |
| `quick_send()` | Simplified send using email or external ID |
