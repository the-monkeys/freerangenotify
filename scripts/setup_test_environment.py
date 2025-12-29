import urllib.request
import json
import time

BASE_URL = "http://localhost:8080/v1"

def create_app():
    print("Creating application...")
    payload = {
        "app_name": "Test SSE App",
        "description": "App for testing SSE",
        "email_sender": "noreply@test.com"
    }
    
    req = urllib.request.Request(
        f"{BASE_URL}/apps",
        data=json.dumps(payload).encode('utf-8'),
        headers={"Content-Type": "application/json"},
        method="POST"
    )
    
    try:
        with urllib.request.urlopen(req) as response:
            data = json.loads(response.read().decode('utf-8'))
            print(f"App created: ID={data['data']['app_id']}, Key={data['data']['api_key']}")
            return data['data']['app_id'], data['data']['api_key']
    except Exception as e:
        print(f"Failed to create app: {e}")
        if hasattr(e, 'read'):
            print(e.read().decode('utf-8'))
        return None, None

def create_user(api_key):
    print("Creating user...")
    payload = {
        "external_user_id": "test-user-sse",
        "email": "test@example.com",
        "name": "SSE Tester",
        "preferences": {
            "categories": {
                "default": {
                    "enabled": True,
                    "enabled_channels": ["sse"]
                }
            }
        }
    }
    
    req = urllib.request.Request(
        f"{BASE_URL}/users",
        data=json.dumps(payload).encode('utf-8'),
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {api_key}"
        },
        method="POST"
    )
    
    try:
        with urllib.request.urlopen(req) as response:
            data = json.loads(response.read().decode('utf-8'))
            print(f"User created: ID={data['data']['user_id']}")
            return data['data']['user_id']
    except Exception as e:
        print(f"Failed to create user: {e}")
        if hasattr(e, 'read'):
            print(e.read().decode('utf-8'))
        return None

def create_template(api_key, app_id):
    print("Creating template...")
    payload = {
        "app_id": app_id,
        "name": "sse_test_template",
        "channel": "sse",
        "subject": "Test Notification",
        "body": "Hello {{.name}}, this is a test.",
        "variables": ["name"]
    }
    
    req = urllib.request.Request(
        f"{BASE_URL}/templates",
        data=json.dumps(payload).encode('utf-8'),
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {api_key}"
        },
        method="POST"
    )
    
    try:
        with urllib.request.urlopen(req) as response:
            data = json.loads(response.read().decode('utf-8'))
            if 'data' in data:
                 print(f"Template created: ID={data['data']['template_id']}")
                 return data['data']['template_id']
            elif 'template_id' in data:
                 print(f"Template created: ID={data['template_id']}")
                 return data['template_id']
            elif 'id' in data:
                 print(f"Template created: ID={data['id']}")
                 return data['id']
            else:
                 print(f"Unexpected template response: {data}")
                 return None

    except Exception as e:
        print(f"Failed to create template: {e}")
        if hasattr(e, 'read'):
            print(e.read().decode('utf-8'))
        return None

def send_notification(api_key, user_id, template_id):
    print("Sending notification...")
    payload = {
        "user_id": user_id,
        "channel": "sse",
        "priority": "high",
        "template_id": template_id,
        "data": {
            "name": "Tester"
        }
    }
    
    req = urllib.request.Request(
        f"{BASE_URL}/notifications",
        data=json.dumps(payload).encode('utf-8'),
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {api_key}"
        },
        method="POST"
    )
    
    try:
        with urllib.request.urlopen(req) as response:
            data = json.loads(response.read().decode('utf-8'))
            # Notification response might not be wrapped in 'data' based on dto.FromNotification
            # But let's check both
            if 'data' in data and 'notification_id' in data['data']:
                 print(f"Notification sent: ID={data['data']['notification_id']}")
                 return data['data']['notification_id']
            elif 'notification_id' in data:
                 print(f"Notification sent: ID={data['notification_id']}")
                 return data['notification_id']
            else:
                 print(f"Unexpected notification response: {data}")
                 return None
    except Exception as e:
        print(f"Failed to send notification: {e}")
        if hasattr(e, 'read'):
            print(e.read().decode('utf-8'))
        return None

def main():
    print("Waiting for services to be ready...")
    time.sleep(10) # Wait for startup
    
    app_id, api_key = create_app()
    if not app_id:
        return
        
    user_id = create_user(api_key)
    if not user_id:
        return
        
    template_id = create_template(api_key, app_id)
    if not template_id:
        return
        
    print("\nIMPORTANT: Update your SSE receiver to use these credentials:")
    print(f"USER_ID={user_id}")
    print("\nSending test notification in 5 seconds...")
    time.sleep(5)
    
    send_notification(api_key, user_id, template_id)

if __name__ == "__main__":
    main()
