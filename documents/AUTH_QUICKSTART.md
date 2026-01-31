# Quick Start: Testing Authentication

## Prerequisites
- Docker and Docker Compose installed
- Go 1.21+ installed
- Node.js 18+ and npm installed

## Step 1: Environment Setup

Create a `.env` file in the project root:
```bash
cat > .env << EOF
# JWT Configuration
JWT_SECRET=your-super-secret-jwt-key-change-this-in-production-min-32-chars

# Database
ELASTICSEARCH_URL=http://localhost:9200

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
EOF
```

## Step 2: Start Services

```bash
# Clean start
docker-compose down -v

# Build and start all services
docker-compose build
docker-compose up -d

# Wait for services to be ready (about 30 seconds)
sleep 30

# Initialize database indices (including auth indices)
docker-compose exec notification-service /app/migrate
```

## Step 3: Start Frontend

```bash
cd ui
npm install
npm run dev
```

The UI will be available at http://localhost:3000

## Step 4: Test Authentication Flow

### Option A: Using the Web UI

1. **Navigate to http://localhost:3000**
2. **Click "Sign Up" in the header**
3. **Register a new account:**
   - Email: admin@example.com
   - Password: SecurePass123
   - Full Name: Admin User
4. **You'll be automatically logged in and redirected to /apps**
5. **Test logout by clicking "Logout" in header**
6. **Test login again with the same credentials**
7. **Test "Forgot Password" flow**

### Option B: Using cURL

#### 1. Register a user
```bash
curl -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "SecurePass123",
    "full_name": "Admin User"
  }'
```

Save the `access_token` from the response.

#### 2. Get current user
```bash
# Replace <TOKEN> with the access_token from step 1
curl -X GET http://localhost:8080/v1/admin/me \
  -H "Authorization: Bearer <TOKEN>"
```

#### 3. Login (to get new tokens)
```bash
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "SecurePass123"
  }'
```

#### 4. Request password reset
```bash
curl -X POST http://localhost:8080/v1/auth/forgot-password \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com"
  }'
```

Check the logs for the reset token:
```bash
docker-compose logs -f notification-service | grep "Password reset token"
```

#### 5. Reset password
```bash
# Replace <TOKEN> with the token from logs
curl -X POST http://localhost:8080/v1/auth/reset-password \
  -H "Content-Type: application/json" \
  -d '{
    "token": "<TOKEN>",
    "new_password": "NewSecurePass456"
  }'
```

#### 6. Login with new password
```bash
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "NewSecurePass456"
  }'
```

## Step 5: Verify Data in Elasticsearch

### Check auth_users index
```bash
curl -X GET "http://localhost:9200/auth_users/_search?pretty" \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "match_all": {}
    }
  }'
```

### Check refresh_tokens index
```bash
curl -X GET "http://localhost:9200/refresh_tokens/_search?pretty" \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "match_all": {}
    }
  }'
```

### Check password_reset_tokens index
```bash
curl -X GET "http://localhost:9200/password_reset_tokens/_search?pretty" \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "match_all": {}
    }
  }'
```

## Common Issues & Solutions

### Issue: "JWT secret is required"
**Solution:** Set JWT_SECRET environment variable or update config.yaml

### Issue: "Index does not exist"
**Solution:** Run the migration:
```bash
docker-compose exec notification-service /app/migrate
```

### Issue: "Cannot connect to Elasticsearch"
**Solution:** Ensure Elasticsearch is running:
```bash
curl http://localhost:9200
docker-compose logs elasticsearch
```

### Issue: Frontend shows "Network Error"
**Solution:** Check API is accessible:
```bash
curl http://localhost:8080/v1/health
```

### Issue: "Token expired" immediately after login
**Solution:** Check system time is synchronized. JWT tokens are time-sensitive.

### Issue: Can't access protected routes in UI
**Solution:** 
1. Open browser DevTools → Application → Local Storage
2. Verify `access_token` and `refresh_token` are stored
3. If not, clear storage and login again

## Testing Checklist

- [ ] User registration works
- [ ] Login with correct credentials works
- [ ] Login with wrong password fails
- [ ] Access token is attached to requests
- [ ] Protected routes redirect to login when not authenticated
- [ ] Token refresh works automatically
- [ ] Logout clears tokens
- [ ] Forgot password generates token
- [ ] Reset password with valid token works
- [ ] Reset password with invalid token fails
- [ ] Change password works
- [ ] User data persists across restarts

## Next Steps

1. **Configure Email Provider**: Update auth service to send actual emails for password reset
2. **Set Production JWT Secret**: Use a strong, random 32+ character secret
3. **Enable HTTPS**: Configure TLS certificates for production
4. **Add Rate Limiting**: Protect against brute force attacks
5. **Setup Monitoring**: Track auth events and failures

## Useful Commands

```bash
# View service logs
docker-compose logs -f notification-service
docker-compose logs -f notification-worker

# Restart specific service
docker-compose restart notification-service

# Clean restart (removes all data)
docker-compose down -v
docker-compose up -d
docker-compose exec notification-service /app/migrate

# Check Elasticsearch indices
curl http://localhost:9200/_cat/indices?v

# Check Redis keys
docker-compose exec redis redis-cli KEYS "*"

# Build and restart after code changes
docker-compose build notification-service
docker-compose restart notification-service
```

## Success Indicators

✅ Registration returns user object and tokens
✅ Login returns tokens with correct expiry
✅ Protected endpoints return 401 without token
✅ Protected endpoints work with valid token
✅ Token refresh extends session
✅ Logout revokes refresh tokens
✅ Password reset flow completes successfully

Your JWT authentication system is now ready to use! 🎉
