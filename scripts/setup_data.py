
import urllib.request
import urllib.error
import json
import sys

BASE_URL = "http://localhost:8080/v1"

def request(method, endpoint, data=None, headers=None):
    if headers is None:
        headers = {}
    
    url = f"{BASE_URL}{endpoint}"
    
    if data:
        json_data = json.dumps(data).encode('utf-8')
        headers['Content-Type'] = 'application/json'
    else:
        json_data = None

    req = urllib.request.Request(url, data=json_data, headers=headers, method=method)
    
    try:
        with urllib.request.urlopen(req) as response:
            return json.loads(response.read().decode('utf-8'))
    except urllib.error.HTTPError as e:
        print(f"Error {e.code} for {method} {url}: {e.read().decode('utf-8')}")
        sys.exit(1)
    except Exception as e:
        print(f"Request failed: {e}")
        sys.exit(1)

def main():
    print("--- Setting up FreeRangeNotify Test Data ---")

    # 1. Create Application
    print("[1/5] Creating Application...")
    app_payload = {
        "app_name": "Test App",
        "webhook_url": "http://receiver:3000/api/webhook" # Internal docker name for receiver
    }
    app_res = request("POST", "/apps", app_payload)
    api_key = app_res['data']['api_key']
    app_id = app_res['data']['app_id']
    print(f"    -> Success! App ID: {app_id}")
    print(f"    -> API Key: {api_key}")

    headers = {"Authorization": f"Bearer {api_key}"}

    # 2. Create User 1
    print("[2/5] Creating User 1...")
    user1_payload = {
        "external_user_id": "user-1",
        "email": "user1@example.com",
        "phone": "+15550101",
        "timezone": "UTC",
        "language": "en-US",
        "preferences": {
            "email_enabled": True,
            "push_enabled": True,
            "sms_enabled": True
        }
    }
    # Note: users endpoint is protected
    # The routes.go says: users := v1.Group("/users"); users.Use(auth)
    user1_res = request("POST", "/users", user1_payload, headers)
    user1_internal_id = user1_res['data']['user_id']
    print(f"    -> Success! Internal User ID: {user1_internal_id}")

    # 3. Create User 2
    print("[3/5] Creating User 2...")
    user2_payload = {
        "external_user_id": "user-2",
        "email": "user2@example.com",
        "preferences": {
            "email_enabled": True
        }
    }
    request("POST", "/users", user2_payload, headers)
    print("    -> Success!")

    # 4. Create Template
    print("[4/5] Creating Template...")
    template_payload = {
        "app_id": app_id,
        "name": "welcome_alert",
        "description": "Welcome message",
        "channel": "webhook", # Using webhook to test receiver logic
        "subject": "Welcome {{.name}}!",
        "body": "Hello {{.name}}, welcome to FreeRangeNotify! Your role is {{.role}}.",
        "variables": ["name", "role"],
        "locale": "en-US",
        "created_by": "setup_script"
    }
    request("POST", "/templates", template_payload, headers)
    print("    -> Success!")

    print("\n--- Setup Complete ---")
    print("Use these details for testing:")
    print(f"API_KEY={api_key}")
    print(f"USER_1_ID=user-1") # External ID
    print(f"TEMPLATE=welcome_alert")

if __name__ == "__main__":
    main()
