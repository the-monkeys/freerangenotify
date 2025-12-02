# API Testing Tracker - FreeRangeNotify

**Testing Date:** December 3, 2025  
**Testing Order:** Application → User → Notification → Template APIs

---

## Testing Environment Setup

### Prerequisites
```powershell
# 1. Start Docker services
docker-compose up -d elasticsearch redis

# 2. Wait for services to be ready (30 seconds)
Start-Sleep -Seconds 30

# 3. Run migrations
.\bin\migrate.exe

# 4. Start the server
.\bin\server.exe
```

### Service Endpoints
- **API Server:** http://localhost:8080
- **Elasticsearch:** http://localhost:9200
- **Redis:** localhost:6379
- **Health Check:** http://localhost:8080/health

---

## Resource Tracking

### Created Applications
| App ID | App Name | API Key | Status | Created At |
|--------|----------|---------|--------|------------|
|        |          |         |        |            |

### Created Users
| User ID | App ID | Email | Phone | External ID | Status | Created At |
|---------|--------|-------|-------|-------------|--------|------------|
|         |        |       |       |             |        |            |

### Created Devices
| Device ID | User ID | Platform | Token | Active | Created At |
|-----------|---------|----------|-------|--------|------------|
|           |         |          |       |        |            |

### Created Notifications
| Notification ID | User ID | Channel | Priority | Status | Created At |
|-----------------|---------|---------|----------|--------|------------|
|                 |         |         |          |        |            |

### Created Templates
| Template ID | App ID | Name | Channel | Version | Locale | Status | Created At |
|-------------|--------|------|---------|---------|--------|--------|------------|
|             |        |      |         |         |        |        |            |

---

## API Test Results

## 1. Application APIs (/v1/apps)

### 1.1 Create Application ✅ / ❌
**Endpoint:** `POST /v1/apps`

**Request:**
```json
{
  "app_name": "Test App 1",
  "webhook_url": "https://example.com/webhook",
  "enable_webhooks": true,
  "enable_analytics": true
}
```

**Expected Response:** 201 Created
```json
{
  "id": "...",
  "app_name": "Test App 1",
  "api_key": "...",
  "webhook_url": "https://example.com/webhook",
  "settings": {
    "rate_limit": 1000,
    "retry_attempts": 3,
    "enable_webhooks": true,
    "enable_analytics": true
  },
  "created_at": "...",
  "updated_at": "..."
}
```

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 1.2 Get Application by ID ✅ / ❌
**Endpoint:** `GET /v1/apps/{app_id}`

**Request:** `GET /v1/apps/{APP_ID_FROM_1.1}`

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 1.3 List Applications ✅ / ❌
**Endpoint:** `GET /v1/apps`

**Request:** `GET /v1/apps?limit=10&offset=0`

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 1.4 Update Application ✅ / ❌
**Endpoint:** `PUT /v1/apps/{app_id}`

**Request:**
```json
{
  "app_name": "Test App 1 - Updated",
  "webhook_url": "https://example.com/webhook-v2"
}
```

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 1.5 Update Application Settings ✅ / ❌
**Endpoint:** `PUT /v1/apps/{app_id}/settings`

**Request:**
```json
{
  "rate_limit": 2000,
  "retry_attempts": 5,
  "enable_webhooks": true,
  "enable_analytics": true
}
```

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 1.6 Get Application Settings ✅ / ❌
**Endpoint:** `GET /v1/apps/{app_id}/settings`

**Request:** `GET /v1/apps/{APP_ID}/settings`

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 1.7 Regenerate API Key ✅ / ❌
**Endpoint:** `POST /v1/apps/{app_id}/regenerate-key`

**Request:** `POST /v1/apps/{APP_ID}/regenerate-key`

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

## 2. User APIs (/v1/users)

**Note:** All user APIs require API Key authentication  
**Header:** `Authorization: Bearer {API_KEY}`

### 2.1 Create User ✅ / ❌
**Endpoint:** `POST /v1/users`

**Request:**
```json
{
  "external_user_id": "user001",
  "email": "user001@example.com",
  "phone": "+1234567890",
  "timezone": "America/New_York",
  "language": "en",
  "metadata": {
    "source": "web_signup"
  }
}
```

**Expected Response:** 201 Created

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 2.2 Get User by ID ✅ / ❌
**Endpoint:** `GET /v1/users/{user_id}`

**Request:** `GET /v1/users/{USER_ID_FROM_2.1}`

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 2.3 List Users ✅ / ❌
**Endpoint:** `GET /v1/users`

**Request:** `GET /v1/users?limit=10&offset=0`

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 2.4 Update User ✅ / ❌
**Endpoint:** `PUT /v1/users/{user_id}`

**Request:**
```json
{
  "email": "user001_updated@example.com",
  "phone": "+1234567899",
  "timezone": "America/Los_Angeles"
}
```

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 2.5 Add Device ✅ / ❌
**Endpoint:** `POST /v1/users/{user_id}/devices`

**Request:**
```json
{
  "device_id": "device-ios-001",
  "platform": "ios",
  "token": "ios-fcm-token-123456",
  "metadata": {
    "model": "iPhone 15",
    "os_version": "17.0"
  }
}
```

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 2.6 Get User Devices ✅ / ❌
**Endpoint:** `GET /v1/users/{user_id}/devices`

**Request:** `GET /v1/users/{USER_ID}/devices`

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 2.7 Remove Device ✅ / ❌
**Endpoint:** `DELETE /v1/users/{user_id}/devices/{device_id}`

**Request:** `DELETE /v1/users/{USER_ID}/devices/{DEVICE_ID}`

**Expected Response:** 204 No Content

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 2.8 Update User Preferences ✅ / ❌
**Endpoint:** `PUT /v1/users/{user_id}/preferences`

**Request:**
```json
{
  "channels": {
    "email": true,
    "push": true,
    "sms": false
  },
  "quiet_hours": {
    "enabled": true,
    "start": "22:00",
    "end": "08:00",
    "timezone": "America/New_York"
  }
}
```

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 2.9 Get User Preferences ✅ / ❌
**Endpoint:** `GET /v1/users/{user_id}/preferences`

**Request:** `GET /v1/users/{USER_ID}/preferences`

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

## 3. Notification APIs (/v1/notifications)

### 3.1 Send Notification ✅ / ❌
**Endpoint:** `POST /v1/notifications`

**Request:**
```json
{
  "user_ids": ["{USER_ID}"],
  "channel": "email",
  "priority": "normal",
  "content": {
    "title": "Welcome!",
    "body": "Thank you for joining us."
  },
  "data": {
    "action_url": "https://example.com/welcome"
  }
}
```

**Expected Response:** 202 Accepted

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 3.2 Send Bulk Notifications ✅ / ❌
**Endpoint:** `POST /v1/notifications/bulk`

**Request:**
```json
{
  "user_ids": ["{USER_ID_1}", "{USER_ID_2}"],
  "channel": "push",
  "priority": "high",
  "content": {
    "title": "New Feature!",
    "body": "Check out our new feature."
  }
}
```

**Expected Response:** 202 Accepted

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 3.3 Get Notification by ID ✅ / ❌
**Endpoint:** `GET /v1/notifications/{notification_id}`

**Request:** `GET /v1/notifications/{NOTIFICATION_ID}`

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 3.4 List Notifications ✅ / ❌
**Endpoint:** `GET /v1/notifications`

**Request:** `GET /v1/notifications?user_id={USER_ID}&status=sent&limit=10`

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 3.5 Update Notification Status ✅ / ❌
**Endpoint:** `PUT /v1/notifications/{notification_id}/status`

**Request:**
```json
{
  "status": "delivered"
}
```

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 3.6 Cancel Notification ✅ / ❌
**Endpoint:** `DELETE /v1/notifications/{notification_id}`

**Request:** `DELETE /v1/notifications/{NOTIFICATION_ID}`

**Expected Response:** 204 No Content

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 3.7 Retry Notification ✅ / ❌
**Endpoint:** `POST /v1/notifications/{notification_id}/retry`

**Request:** `POST /v1/notifications/{NOTIFICATION_ID}/retry`

**Expected Response:** 202 Accepted

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

## 4. Template APIs (/v1/templates)

### 4.1 Create Template ✅ / ❌
**Endpoint:** `POST /v1/templates`

**Request:**
```json
{
  "app_id": "{APP_ID}",
  "name": "welcome_email",
  "description": "Welcome email template",
  "channel": "email",
  "subject": "Welcome to {{.AppName}}!",
  "body": "Hello {{.UserName}}, welcome to our platform!",
  "variables": ["AppName", "UserName"],
  "locale": "en-US",
  "created_by": "admin"
}
```

**Expected Response:** 201 Created

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 4.2 Get Template by ID ✅ / ❌
**Endpoint:** `GET /v1/templates/{template_id}`

**Request:** `GET /v1/templates/{TEMPLATE_ID}`

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 4.3 List Templates ✅ / ❌
**Endpoint:** `GET /v1/templates`

**Request:** `GET /v1/templates?app_id={APP_ID}&channel=email&limit=10`

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 4.4 Update Template ✅ / ❌
**Endpoint:** `PUT /v1/templates/{template_id}`

**Request:**
```json
{
  "description": "Updated welcome email template",
  "body": "Hello {{.UserName}}, welcome! We're excited to have you.",
  "updated_by": "admin"
}
```

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 4.5 Render Template ✅ / ❌
**Endpoint:** `POST /v1/templates/{template_id}/render`

**Request:**
```json
{
  "data": {
    "AppName": "FreeRangeNotify",
    "UserName": "John Doe"
  }
}
```

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 4.6 Create Template Version ✅ / ❌
**Endpoint:** `POST /v1/templates/{app_id}/{name}/versions`

**Request:**
```json
{
  "body": "Hello {{.UserName}}, welcome to {{.AppName}} v2!",
  "description": "Version 2 of welcome email",
  "created_by": "admin"
}
```

**Expected Response:** 201 Created

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

### 4.7 Get Template Versions ✅ / ❌
**Endpoint:** `GET /v1/templates/{app_id}/{name}/versions`

**Request:** `GET /v1/templates/{APP_ID}/welcome_email/versions?locale=en-US`

**Expected Response:** 200 OK

**Result:**
- Status Code: 
- Response: 
- Notes: 

**Documentation Updated:** [ ] Yes [ ] No

---

## Summary

### Statistics
- **Total APIs Tested:** 0 / 35
- **Passed:** 0
- **Failed:** 0
- **Documentation Updated:** 0 / 35

### Issues Found
1. 
2. 
3. 

### Next Steps
1. 
2. 
3. 

---

**Testing Completed:** [ ] Yes [ ] No  
**Ready for Production:** [ ] Yes [ ] No
