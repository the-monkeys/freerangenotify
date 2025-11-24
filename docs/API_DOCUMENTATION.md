# API Documentation

FreeRangeNotify provides comprehensive REST API documentation using OpenAPI 3.0 (Swagger) specification.

## Accessing API Documentation

### Interactive Swagger UI

Once the server is running, access the interactive API documentation at:

```
http://localhost:8080/swagger/index.html
```

The Swagger UI provides:
- **Interactive testing** - Try out API endpoints directly from the browser
- **Request/response examples** - See sample payloads for all endpoints
- **Schema documentation** - Detailed models and data structures
- **Authentication setup** - Easy Bearer token configuration

### OpenAPI Specification Files

The raw OpenAPI specification is available in multiple formats:

- **YAML**: `/swagger/doc.yaml` or `docs/openapi/swagger.yaml`
- **JSON**: `/swagger/doc.json` or `docs/swagger.json`

## API Overview

### Base URL
```
http://localhost:8080
```

### Authentication

Protected endpoints require Bearer token authentication using your application's API key:

```http
Authorization: Bearer frn_YOUR_API_KEY_HERE
```

**Example with curl:**
```bash
curl -X POST http://localhost:8080/v1/users \
  -H "Authorization: Bearer frn_GxSftc5urm7bxVrYh-RHNEmNDvS4auxwOhwjqXwC-kM=" \
  -H "Content-Type: application/json" \
  -d '{
    "external_user_id": "user-123",
    "email": "user@example.com"
  }'
```

**Example with PowerShell:**
```powershell
$headers = @{
    "Authorization" = "Bearer frn_YOUR_API_KEY"
    "Content-Type" = "application/json"
}
$body = @{
    external_user_id = "user-123"
    email = "user@example.com"
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8080/v1/users" `
    -Method Post -Headers $headers -Body $body
```

## API Endpoints

### System Endpoints

#### Health Check
```http
GET /health
```
Returns service health status and database connectivity.

**Response:**
```json
{
  "service": "FreeRangeNotify",
  "version": "1.0.0",
  "database": "ok"
}
```

---

### Application Management (Public)

#### Create Application
```http
POST /v1/apps
```
Create a new application and receive an API key.

**Request Body:**
```json
{
  "app_name": "My App",
  "webhook_url": "https://example.com/webhook",
  "settings": {
    "rate_limit": 1000,
    "retry_attempts": 3,
    "enable_webhooks": true,
    "enable_analytics": true
  }
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "app_id": "uuid",
    "app_name": "My App",
    "api_key": "frn_FULL_API_KEY_HERE",
    "webhook_url": "https://example.com/webhook"
  }
}
```

#### Get Application
```http
GET /v1/apps/{app_id}
```

#### List Applications
```http
GET /v1/apps?page=1&page_size=10
```

#### Update Application
```http
PUT /v1/apps/{app_id}
```

#### Delete Application
```http
DELETE /v1/apps/{app_id}
```

#### Regenerate API Key
```http
POST /v1/apps/{app_id}/regenerate-key
```

#### Get/Update Settings
```http
GET /v1/apps/{app_id}/settings
PUT /v1/apps/{app_id}/settings
```

---

### User Management (Protected)

All user endpoints require Bearer authentication.

#### Create User
```http
POST /v1/users
Authorization: Bearer {api_key}
```

**Request Body:**
```json
{
  "external_user_id": "user-123",
  "email": "user@example.com",
  "phone_number": "+1234567890",
  "preferences": {
    "channels": ["email", "push"],
    "timezone": "America/New_York",
    "language": "en",
    "quiet_hours": {
      "enabled": true,
      "start": "22:00",
      "end": "08:00"
    }
  }
}
```

#### Get User
```http
GET /v1/users/{user_id}
Authorization: Bearer {api_key}
```

#### List Users
```http
GET /v1/users?page=1&page_size=10
Authorization: Bearer {api_key}
```

#### Update User
```http
PUT /v1/users/{user_id}
Authorization: Bearer {api_key}
```

#### Delete User
```http
DELETE /v1/users/{user_id}
Authorization: Bearer {api_key}
```

---

### Device Management (Protected)

#### Add Device
```http
POST /v1/users/{user_id}/devices
Authorization: Bearer {api_key}
```

**Request Body:**
```json
{
  "platform": "ios",
  "token": "device-token-abc123"
}
```

**Supported platforms:** `ios`, `android`, `web`

#### Get User Devices
```http
GET /v1/users/{user_id}/devices
Authorization: Bearer {api_key}
```

#### Delete Device
```http
DELETE /v1/users/{user_id}/devices/{device_id}
Authorization: Bearer {api_key}
```

---

### Preference Management (Protected)

#### Get Preferences
```http
GET /v1/users/{user_id}/preferences
Authorization: Bearer {api_key}
```

#### Update Preferences
```http
PUT /v1/users/{user_id}/preferences
Authorization: Bearer {api_key}
```

**Request Body:**
```json
{
  "channels": ["email", "push", "sms"],
  "timezone": "America/Los_Angeles",
  "language": "es",
  "quiet_hours": {
    "enabled": true,
    "start": "23:00",
    "end": "07:00"
  }
}
```

---

## Data Models

### Application
```json
{
  "app_id": "uuid",
  "app_name": "string",
  "api_key": "string (masked after creation)",
  "webhook_url": "string (url)",
  "settings": {
    "rate_limit": "integer",
    "retry_attempts": "integer",
    "default_template": "string",
    "enable_webhooks": "boolean",
    "enable_analytics": "boolean"
  },
  "created_at": "datetime",
  "updated_at": "datetime"
}
```

### User
```json
{
  "user_id": "uuid",
  "app_id": "uuid",
  "external_user_id": "string",
  "email": "string (email format)",
  "phone_number": "string (E.164 format)",
  "preferences": {
    "email_enabled": "boolean",
    "push_enabled": "boolean",
    "sms_enabled": "boolean",
    "quiet_hours": {}
  },
  "created_at": "datetime",
  "updated_at": "datetime"
}
```

### Device
```json
{
  "device_id": "uuid",
  "platform": "enum (ios, android, web)",
  "token": "string",
  "active": "boolean",
  "registered_at": "datetime",
  "last_seen": "datetime"
}
```

---

## Error Responses

All error responses follow this format:

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message",
    "details": {
      "field_name": "Specific validation error"
    }
  }
}
```

### Common Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `VALIDATION_ERROR` | 400 | Request validation failed |
| `UNAUTHORIZED` | 401 | Missing authorization header |
| `INVALID_API_KEY` | 401 | Invalid or expired API key |
| `NOT_FOUND` | 404 | Resource not found |
| `INTERNAL_ERROR` | 500 | Internal server error |
| `DATABASE_ERROR` | 500 | Database operation failed |

---

## Rate Limiting

Rate limits are configurable per application (default: 1000 requests/minute).

**Headers returned:**
```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 950
X-RateLimit-Reset: 1638360000
```

---

## Pagination

List endpoints support pagination with query parameters:

- `page` - Page number (starts at 1, default: 1)
- `page_size` - Items per page (default: 10, max: 100)

**Example:**
```http
GET /v1/users?page=2&page_size=20
```

**Response:**
```json
{
  "success": true,
  "data": {
    "users": [...],
    "total_count": 150,
    "page": 2,
    "page_size": 20
  }
}
```

---

## Filtering

Some endpoints support filtering:

**Applications:**
- `app_name` - Filter by application name

**Users:**
- `external_user_id` - Filter by external user ID
- `email` - Filter by email address

**Example:**
```http
GET /v1/users?email=john@example.com
```

---

## Generating/Updating Documentation

### Generate Swagger Docs

```bash
# Using Makefile
make swagger

# Or directly
swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal
```

### Format Swagger Annotations

```bash
make swagger-fmt
```

---

## Using Swagger UI

1. **Start the server:**
   ```bash
   docker-compose up -d
   # or
   go run cmd/server/main.go
   ```

2. **Open Swagger UI:**
   Navigate to `http://localhost:8080/swagger/index.html`

3. **Authorize (for protected endpoints):**
   - Click "Authorize" button at the top
   - Enter your API key: `frn_YOUR_API_KEY`
   - Click "Authorize" then "Close"

4. **Try out endpoints:**
   - Expand any endpoint
   - Click "Try it out"
   - Fill in parameters/body
   - Click "Execute"
   - View response

---

## Postman Collection

You can import the OpenAPI spec into Postman:

1. Open Postman
2. Click "Import"
3. Select "Link" tab
4. Enter: `http://localhost:8080/swagger/doc.json`
5. Click "Continue" and "Import"

---

## Integration Examples

### JavaScript/Node.js
```javascript
const axios = require('axios');

const api = axios.create({
  baseURL: 'http://localhost:8080',
  headers: {
    'Authorization': 'Bearer frn_YOUR_API_KEY',
    'Content-Type': 'application/json'
  }
});

// Create user
const response = await api.post('/v1/users', {
  external_user_id: 'user-123',
  email: 'user@example.com'
});
```

### Python
```python
import requests

headers = {
    'Authorization': 'Bearer frn_YOUR_API_KEY',
    'Content-Type': 'application/json'
}

response = requests.post(
    'http://localhost:8080/v1/users',
    headers=headers,
    json={
        'external_user_id': 'user-123',
        'email': 'user@example.com'
    }
)
```

### Go
```go
client := &http.Client{}
body := strings.NewReader(`{"external_user_id":"user-123","email":"user@example.com"}`)

req, _ := http.NewRequest("POST", "http://localhost:8080/v1/users", body)
req.Header.Set("Authorization", "Bearer frn_YOUR_API_KEY")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

---

## Support

For API support or questions:
- Email: support@freerangenotify.com
- Documentation: http://localhost:8080/swagger/
- GitHub: https://github.com/the-monkeys/freerangenotify
