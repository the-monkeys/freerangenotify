"""
FreeRangeNotify API — Python Integration Example

Prerequisites:
    - Python 3.8+
    - requests library: pip install requests

Usage:
    export FRN_API_KEY="frn_your_api_key_here"
    python main.py
"""

import json
import os
import sys
from pathlib import Path

try:
    import requests
except ImportError:
    print("Error: 'requests' library is required.")
    print("  pip install requests")
    sys.exit(1)


# ──────────────────────────────────────────────────────────────────────────────
# Configuration
# ──────────────────────────────────────────────────────────────────────────────

BASE_URL = "https://freerangenotify.monkeys.support/v1"

def get_api_key() -> str:
    key = os.environ.get("FRN_API_KEY")
    if not key:
        print("Error: FRN_API_KEY environment variable is not set.", file=sys.stderr)
        print('  export FRN_API_KEY="frn_your_api_key_here"', file=sys.stderr)
        sys.exit(1)
    return key


def headers() -> dict:
    return {
        "X-API-Key": get_api_key(),
        "Content-Type": "application/json",
        "Accept": "application/json",
    }


def pretty(label: str, data: dict | list | str):
    """Pretty-print a JSON response."""
    print(f"\n=== {label} ===")
    if isinstance(data, (dict, list)):
        print(json.dumps(data, indent=2))
    else:
        print(data)
    print()


# ──────────────────────────────────────────────────────────────────────────────
# 1. Send a Notification
# ──────────────────────────────────────────────────────────────────────────────

def send_notification():
    """Send a single notification to a user via email."""
    print("──── Send Notification ────")

    payload = {
        "user_id": "user-uuid-or-external-id",
        "channel": "email",
        "priority": "normal",
        "template_id": "welcome-email",
        "data": {
            "name": "Jane Doe",
            "company": "Acme Corp",
        },
    }

    resp = requests.post(f"{BASE_URL}/notifications", json=payload, headers=headers())
    pretty("Send Notification Response", resp.json() if resp.ok else resp.text)
    return resp


# ──────────────────────────────────────────────────────────────────────────────
# 2. Send Bulk Notifications
# ──────────────────────────────────────────────────────────────────────────────

def send_bulk_notifications():
    """Send push notifications to multiple users at once."""
    print("──── Send Bulk Notifications ────")

    payload = {
        "user_ids": ["user-1-uuid", "user-2-uuid", "user-3-uuid"],
        "channel": "push",
        "priority": "high",
        "template_id": "flash-sale",
        "data": {
            "discount": "25%",
            "expires": "2026-06-10T00:00:00Z",
        },
    }

    resp = requests.post(f"{BASE_URL}/notifications/bulk", json=payload, headers=headers())
    pretty("Bulk Send Response", resp.json() if resp.ok else resp.text)
    return resp


# ──────────────────────────────────────────────────────────────────────────────
# 3. List Notifications
# ──────────────────────────────────────────────────────────────────────────────

def list_notifications(page: int = 1, page_size: int = 10):
    """Retrieve a paginated list of recent notifications."""
    print("──── List Notifications ────")

    params = {"page": page, "page_size": page_size}
    resp = requests.get(f"{BASE_URL}/notifications", params=params, headers=headers())
    pretty("List Notifications Response", resp.json() if resp.ok else resp.text)
    return resp


# ──────────────────────────────────────────────────────────────────────────────
# 4. Get Notification by ID
# ──────────────────────────────────────────────────────────────────────────────

def get_notification(notification_id: str):
    """Retrieve details of a single notification."""
    print("──── Get Notification ────")

    resp = requests.get(f"{BASE_URL}/notifications/{notification_id}", headers=headers())
    pretty("Get Notification Response", resp.json() if resp.ok else resp.text)
    return resp


# ──────────────────────────────────────────────────────────────────────────────
# 5. Send OTP
# ──────────────────────────────────────────────────────────────────────────────

def send_otp(channel: str = "sms", recipient: str = "+14155551234"):
    """Send a one-time passcode via SMS, WhatsApp, or email."""
    print("──── Send OTP ────")

    payload = {
        "channel": channel,
        "recipient": recipient,
        # Optional customisations:
        # "length": 6,
        # "ttl_seconds": 300,
        # "max_attempts": 5,
    }

    resp = requests.post(f"{BASE_URL}/otp/send", json=payload, headers=headers())
    pretty("Send OTP Response", resp.json() if resp.ok else resp.text)
    return resp


# ──────────────────────────────────────────────────────────────────────────────
# 6. Verify OTP
# ──────────────────────────────────────────────────────────────────────────────

def verify_otp(request_id: str, code: str):
    """Verify a previously-sent OTP code."""
    print("──── Verify OTP ────")

    payload = {
        "request_id": request_id,
        "code": code,
    }

    resp = requests.post(f"{BASE_URL}/otp/verify", json=payload, headers=headers())
    pretty("Verify OTP Response", resp.json() if resp.ok else resp.text)
    return resp


# ──────────────────────────────────────────────────────────────────────────────
# 7. Upload a File (Invoice)
# ──────────────────────────────────────────────────────────────────────────────

def upload_file(file_path: str):
    """Upload a file (invoice, receipt, etc.) via multipart form."""
    print("──── Upload File ────")

    path = Path(file_path)
    if not path.exists():
        print(f"File not found: {file_path}", file=sys.stderr)
        return None

    with open(path, "rb") as f:
        files = {"file": (path.name, f)}
        upload_headers = {
            "X-API-Key": get_api_key(),
            "Accept": "application/json",
        }
        resp = requests.post(f"{BASE_URL}/files", files=files, headers=upload_headers)

    pretty("Upload File Response", resp.json() if resp.ok else resp.text)
    return resp


# ──────────────────────────────────────────────────────────────────────────────
# 8. List Files
# ──────────────────────────────────────────────────────────────────────────────

def list_files():
    """List uploaded files for the current application."""
    print("──── List Files ────")

    resp = requests.get(f"{BASE_URL}/files", headers=headers())
    pretty("List Files Response", resp.json() if resp.ok else resp.text)
    return resp


# ──────────────────────────────────────────────────────────────────────────────
# 9. Quick Send (simplified endpoint)
# ──────────────────────────────────────────────────────────────────────────────

def quick_send():
    """Use the simplified quick-send endpoint for one-liner notifications."""
    print("──── Quick Send ────")

    payload = {
        "to": "jane@example.com",
        "channel": "email",
        "subject": "Your Invoice is Ready",
        "body": "Hi Jane, your invoice #1234 is attached.",
        "priority": "normal",
    }

    resp = requests.post(f"{BASE_URL}/quick-send", json=payload, headers=headers())
    pretty("Quick Send Response", resp.json() if resp.ok else resp.text)
    return resp


# ──────────────────────────────────────────────────────────────────────────────
# Main
# ──────────────────────────────────────────────────────────────────────────────

if __name__ == "__main__":
    print("╔══════════════════════════════════════════════════════════╗")
    print("║   FreeRangeNotify — Python Integration Examples         ║")
    print("╚══════════════════════════════════════════════════════════╝")

    # 1. Send a single notification
    send_notification()

    # 2. Send bulk notifications
    send_bulk_notifications()

    # 3. List recent notifications
    list_notifications()

    # 4. Send an OTP via SMS
    otp_resp = send_otp()

    # 5. Verify an OTP (replace with real values from step 4)
    verify_otp("req_placeholder_id", "123456")

    # 6. Upload a file (uncomment and provide a real path)
    # upload_file("/path/to/invoice.pdf")

    # 7. List files
    list_files()

    # 8. Quick send
    quick_send()
