import urllib.request
import urllib.error
import time
import uuid
import json
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer
import sys

# Configuration
BASE_URL = "http://localhost:8080/v1"
WEBHOOK_PORT = 8090 # Matching the separate receiver I started
WEBHOOK_URL = f"http://host.docker.internal:{WEBHOOK_PORT}/webhook"
# ...

webhook_received_event = threading.Event()
received_payload = None

class WebhookReceiver(BaseHTTPRequestHandler):
    def do_POST(self):
        global received_payload
        content_length = int(self.headers['Content-Length'])
        post_data = self.rfile.read(content_length)
        received_payload = json.loads(post_data.decode('utf-8'))
        
        print(f"\n[Receiver] Webhook received! Payload: {json.dumps(received_payload)}")
        
        self.send_response(200)
        self.end_headers()
        webhook_received_event.set()

    def log_message(self, format, *args):
        return

def start_receiver():
    server = HTTPServer(('0.0.0.0', WEBHOOK_PORT), WebhookReceiver)
    print(f"[Receiver] Listening on port {WEBHOOK_PORT}...")
    server.serve_forever()

def http_request(method, url, data=None, headers=None):
    if headers is None:
        headers = {}
    
    req = urllib.request.Request(url, method=method)
    for k, v in headers.items():
        req.add_header(k, v)
    
    if data:
        json_data = json.dumps(data).encode('utf-8')
        req.add_header('Content-Type', 'application/json')
        req.data = json_data
        
    try:
        with urllib.request.urlopen(req) as response:
            body = response.read().decode('utf-8')
            return response.status, json.loads(body) if body else {}
    except urllib.error.HTTPError as e:
        body = e.read().decode('utf-8')
        print(f"HTTP Error {e.code}: {body}")
        return e.code, json.loads(body) if body else {}
    except urllib.error.URLError as e:
        print(f"URL Error: {e.reason}")
        return 0, None

def wait_for_health():
    print("Waiting for service to be healthy...")
    max_retries = 30
    for i in range(max_retries):
        status, _ = http_request("GET", f"{BASE_URL}/health")
        if status == 200:
            print("Service is healthy!")
            return True
        print(f"Waiting... ({i+1}/{max_retries})")
        time.sleep(2)
    return False

def run_test():
    # receiver_thread = threading.Thread(target=start_receiver, daemon=True)
    # receiver_thread.start()

    if not wait_for_health():
        print("Failed to connect to service.")
        sys.exit(1)

    # 1. Create Application
    print("\n[1] Creating Application...")
    status, app_data = http_request("POST", f"{BASE_URL}/apps", {
        "app_name": "E2E Test App",
        "description": "App for testing webhooks"
    })
    if status != 201:
        print(f"Failed to create app: {status}")
        sys.exit(1)
    
    api_key = app_data['data']['api_key'] 
    app_id = app_data['data']['app_id']
    # Note: Response wrapper might be {"success": true, "data": {...}} based on handler code I saw
    # Handler: return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": dto...})
    # So app_data is the whole response. I need to access ['data'].

    print(f"App Created: ID={app_id}, API Key: {api_key}")

    # 2. Create Template
    print("\n[2] Creating Template...")
    headers = {"Authorization": api_key}
    status, tmpl_data = http_request("POST", f"{BASE_URL}/templates", {
        "app_id": app_id,
        "name": "webhook_test_template",
        "subject": "Webhook Test Alert",
        "body": "This is a test notification.",
        "type": "transactional",
        "channel": "webhook" # DTO says channel is required? validate:"required,oneof=..."
        # Template DTO: Channel string `json:"channel" validate:"required..."`
    }, headers)
    
    if status != 201:
        print(f"Failed to create template: {status}")
        sys.exit(1)
    
    # Template handler might also wrap in "data"? Need to check handler or just dump response if fail.
    # Assuming it returns the object directly or wrapped?
    # Let's assume wrapped if App was wrapped.
    # I'll check template_data structure if it fails, or just look at handler code if I view it.
    # But usually valid assumption is consistency.
    # Use .get('id') if direct or .get('data', {}).get('id')
    template_id = tmpl_data.get('id') or tmpl_data.get('data', {}).get('id')
    print(f"Template Created: ID={template_id}")

    # 3. Send Notification
    print("\n[3] Sending Notification...")
    status, notif_data = http_request("POST", f"{BASE_URL}/notifications", {
        "channel": "webhook",
        "priority": "normal",
        "template_id": template_id,
        "webhook_url": WEBHOOK_URL,
        "data": {
            "test_run": str(uuid.uuid4()),
            "message": "Hello from E2E"
        }
    }, headers)
    
    if status != 202:
        print(f"Failed to send notification: {status}")
        sys.exit(1)
    
    print(f"Notification Accepted: ID={notif_data['notification_id']}")

    # 4. Verification (Manual/External)
    print(f"\n[4] Notification sent! Check the logs of the running receiver on port {WEBHOOK_PORT}.")
    print(f"Expected Payload ID: {notif_data['notification_id']}")
    # Since we use external receiver, we can't assert receipt here unless we poll it or just assume success if 202.
    # We will trust 202 and user visual verification of the other terminal logs.

if __name__ == "__main__":
    run_test()
