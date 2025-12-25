import urllib.request
import json
import sys

API_KEY = "frn_-J8tDmXVDCXqGInAQTc5mmOwKS1pYsmKpDKGbi7FczU="
APP_ID = "38743c80-0b9f-431a-b84b-783fa713a32b"
USER_ID = "7998188a-1649-40f9-b2e1-de26a130c367"  # This matches the SSE receiver
TEMPLATE_ID = "0f1eae0d-fab3-4da2-bd2f-c79b1f5902a9"

payload = {
    "app_id": APP_ID,
    "user_id": USER_ID,
    "template_id": TEMPLATE_ID,
    "channel": "sse",
    "priority": "high",
    "data": {
        "name": "Dave",
        "role": "Admin"
    }
}

req = urllib.request.Request(
    "http://localhost:8080/v1/notifications",
    data=json.dumps(payload).encode('utf-8'),
    headers={
        "Authorization": f"Bearer {API_KEY}",
        "Content-Type": "application/json"
    },
    method="POST"
)

try:
    with urllib.request.urlopen(req) as response:
        print(response.read().decode('utf-8'))
except Exception as e:
    print(f"Error: {e}")
    if hasattr(e, 'read'):
        print(e.read().decode('utf-8'))
    sys.exit(1)
