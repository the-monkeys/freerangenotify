# JWT Authentication Implementation Guide

## Overview

This document describes the JWT-based authentication system implemented for FreeRangeNotify. The system provides secure email/password authentication with password reset functionality for the admin dashboard.

## Architecture

### Backend Components

#### 1. Domain Layer (`internal/domain/auth/`)
- **models.go**: Core auth domain models
  - `AdminUser`: Admin user entity with email, password hash, and metadata
  - `PasswordResetToken`: Tokens for password reset flow
  - `RefreshToken`: Long-lived tokens for access token renewal
  - Request/Response DTOs for all auth operations

#### 2. JWT Package (`pkg/jwt/`)
- **jwt.go**: JWT token management
  - `Manager`: Handles token generation and validation
  - `Claims`: Custom JWT claims with user ID and email
  - Access tokens: 15 minutes (default)
  - Refresh tokens: 7 days (default)
  - Uses HS256 signing algorithm

#### 3. Repository Layer (`internal/infrastructure/repository/`)
- **auth_repository.go**: Elasticsearch persistence
  - Stores admin users in `auth_users` index
  - Manages password reset tokens in `password_reset_tokens` index
  - Handles refresh tokens in `refresh_tokens` index
  - Efficient querying with proper indexing

#### 4. Service Layer (`internal/usecases/services/`)
- **auth_service_impl.go**: Business logic
  - User registration with bcrypt password hashing
  - Login with credential validation
  - Token refresh mechanism
  - Password reset flow (generate token → validate → update)
  - Change password with old password verification
  - Automatic token revocation on security events

#### 5. HTTP Layer (`internal/interfaces/http/`)
- **handlers/auth_handler.go**: HTTP request handlers
- **dto/auth_dto.go**: Request/Response DTOs
- **middleware/jwt_auth.go**: JWT authentication middleware

#### 6. Database Indices (`internal/infrastructure/database/`)
- **index_templates.go**: Elasticsearch mappings for auth indices

### Frontend Components

#### 1. Authentication Context (`ui/src/contexts/AuthContext.tsx`)
- Global auth state management
- Auto-fetch current user on mount
- Login, register, logout functions
- Loading states for better UX

#### 2. Auth Pages
- **Login.tsx**: Email/password login
- **Register.tsx**: User registration with validation
- **ForgotPassword.tsx**: Request password reset email
- **ResetPassword.tsx**: Reset password with token

#### 3. Protected Routes (`ui/src/components/ProtectedRoute.tsx`)
- Redirect to login if unauthenticated
- Show loading state during auth check
- Preserve intended destination

#### 4. API Integration (`ui/src/services/api.ts`)
- Axios interceptors for automatic JWT token attachment
- Token refresh on 401 responses
- Automatic redirect to login on auth failure

## Authentication Flow

### Registration Flow
```
1. User submits email, password, full name
2. Backend validates input (email format, password length)
3. Check if email already exists
4. Hash password with bcrypt (cost: 10)
5. Create user in Elasticsearch
6. Generate access + refresh tokens
7. Store refresh token in database
8. Return user data and tokens to frontend
9. Frontend stores tokens in localStorage
10. Redirect to dashboard
```

### Login Flow
```
1. User submits email and password
2. Backend retrieves user by email
3. Verify password with bcrypt.CompareHashAndPassword
4. Check if user is active
5. Update last_login_at timestamp
6. Generate new access + refresh tokens
7. Store refresh token in database
8. Return user data and tokens
9. Frontend stores tokens and redirects
```

### Token Refresh Flow
```
1. Access token expires (15 minutes)
2. Frontend receives 401 from API
3. Axios interceptor catches error
4. Send refresh token to /v1/auth/refresh
5. Backend validates refresh token:
   - Verify JWT signature
   - Check token exists in database
   - Check not revoked
   - Check not expired
6. Generate new access + refresh tokens
7. Revoke old refresh token
8. Return new tokens
9. Retry original request with new access token
```

### Password Reset Flow
```
1. User clicks "Forgot Password"
2. Submit email address
3. Backend generates secure random token (32 bytes hex)
4. Store token in database with 1-hour expiry
5. Send email with reset link containing token
   Format: https://app.com/reset-password?token=<token>
6. User clicks link
7. Frontend shows reset form
8. User submits new password
9. Backend validates token:
   - Check exists and not used
   - Check not expired
10. Hash new password
11. Update user password
12. Mark token as used
13. Revoke all user's refresh tokens (security)
14. Redirect to login
```

## API Endpoints

### Public Endpoints (No Auth Required)

#### Register
```
POST /v1/auth/register
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "secure_password",
  "full_name": "John Doe"
}

Response 201:
{
  "user": {
    "user_id": "uuid",
    "email": "user@example.com",
    "full_name": "John Doe",
    "is_active": true,
    "created_at": "2026-01-23T...",
    "updated_at": "2026-01-23T..."
  },
  "access_token": "eyJhbGc...",
  "refresh_token": "eyJhbGc...",
  "expires_at": "2026-01-23T..."
}
```

#### Login
```
POST /v1/auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "secure_password"
}

Response 200: Same as register
```

#### Refresh Token
```
POST /v1/auth/refresh
Content-Type: application/json

{
  "refresh_token": "eyJhbGc..."
}

Response 200:
{
  "access_token": "eyJhbGc...",
  "refresh_token": "eyJhbGc...",
  "expires_at": "2026-01-23T..."
}
```

#### Forgot Password
```
POST /v1/auth/forgot-password
Content-Type: application/json

{
  "email": "user@example.com"
}

Response 200:
{
  "message": "Password reset instructions have been sent to your email"
}
```

#### Reset Password
```
POST /v1/auth/reset-password
Content-Type: application/json

{
  "token": "hex_token_from_email",
  "new_password": "new_secure_password"
}

Response 200:
{
  "message": "Password has been reset successfully"
}
```

### Protected Endpoints (JWT Required)

#### Get Current User
```
GET /v1/admin/me
Authorization: Bearer <access_token>

Response 200:
{
  "user_id": "uuid",
  "email": "user@example.com",
  "full_name": "John Doe",
  "is_active": true,
  "created_at": "2026-01-23T...",
  "updated_at": "2026-01-23T...",
  "last_login_at": "2026-01-23T..."
}
```

#### Logout
```
POST /v1/auth/logout
Authorization: Bearer <access_token>

Response 200:
{
  "message": "Logged out successfully"
}
```

#### Change Password
```
POST /v1/auth/change-password
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "old_password": "current_password",
  "new_password": "new_secure_password"
}

Response 200:
{
  "message": "Password has been changed successfully"
}
```

## Security Considerations

### Password Security
- Passwords hashed with bcrypt (cost factor: 10)
- Minimum password length: 8 characters
- Passwords never returned in API responses

### Token Security
- JWT tokens signed with HS256
- Secret key stored in environment variable
- Access tokens short-lived (15 minutes)
- Refresh tokens long-lived but revocable
- All tokens revoked on password change/reset

### Database Security
- Password hashes not indexed in Elasticsearch
- Reset tokens expire after 1 hour
- Used reset tokens cannot be reused
- Refresh tokens can be revoked

### Frontend Security
- Tokens stored in localStorage
- Automatic token refresh on expiry
- Protected routes redirect to login
- CORS properly configured

## Configuration

### Backend Configuration (`config/config.yaml`)
```yaml
security:
  jwt_secret: "${JWT_SECRET:-dev-secret-key-change-in-production}"
  jwt_access_expiration: 15  # minutes
  jwt_refresh_expiration: 10080  # 7 days in minutes
  api_key_header: "X-API-Key"
  rate_limit: 100
  rate_limit_window: 3600
```

### Environment Variables
```bash
# JWT Secret (MUST be set in production)
export JWT_SECRET="your-secure-random-secret-key-min-32-chars"

# Or via config
export FREERANGE_SECURITY_JWT_SECRET="your-secure-secret"
export FREERANGE_SECURITY_JWT_ACCESS_EXPIRATION=15
export FREERANGE_SECURITY_JWT_REFRESH_EXPIRATION=10080
```

## Database Setup

### Initialize Auth Indices
```bash
# Run migration to create indices
docker-compose exec notification-service /app/migrate
```

This creates three new indices:
- `auth_users`: Admin user accounts
- `password_reset_tokens`: Password reset tokens
- `refresh_tokens`: JWT refresh tokens

## Testing

### Manual Testing

#### 1. Register a User
```bash
curl -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "SecurePass123",
    "full_name": "Admin User"
  }'
```

#### 2. Login
```bash
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "SecurePass123"
  }'
```

#### 3. Get Current User
```bash
curl -X GET http://localhost:8080/v1/admin/me \
  -H "Authorization: Bearer <access_token>"
```

#### 4. Refresh Token
```bash
curl -X POST http://localhost:8080/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "<refresh_token>"
  }'
```

### Frontend Testing

1. Start the UI: `cd ui && npm run dev`
2. Navigate to http://localhost:3000
3. Test registration flow
4. Test login flow
5. Test protected routes (should redirect if not logged in)
6. Test logout
7. Test password reset flow

## Migration from Existing Setup

If you have an existing FreeRangeNotify setup:

1. **Update Dependencies**
   ```bash
   go get github.com/golang-jwt/jwt/v5 golang.org/x/crypto/bcrypt
   ```

2. **Update Configuration**
   - Add JWT settings to `config/config.yaml`
   - Set `JWT_SECRET` environment variable

3. **Run Migration**
   ```bash
   docker-compose exec notification-service /app/migrate
   ```

4. **Update Frontend**
   - Install no new dependencies needed (already using axios)
   - Deploy updated UI code

5. **Create First Admin User**
   - Use registration endpoint or API
   - Or manually create via Elasticsearch

## Troubleshooting

### "Invalid or expired token" errors
- Check JWT_SECRET is consistent across restarts
- Verify token expiration times in config
- Check system clock synchronization

### "User not found" after registration
- Verify Elasticsearch indices created
- Check Elasticsearch connection
- Review auth_users index mapping

### Password reset email not sending
- Email provider not yet implemented in service
- Check logs for reset token generation
- Tokens logged at INFO level during development

### Frontend redirects to login constantly
- Check localStorage for tokens
- Verify API base URL configuration
- Check CORS settings in backend

## Future Enhancements

### Planned Features
1. **Email Integration**: Actual email sending for password reset
2. **Rate Limiting**: Prevent brute force attacks on login
3. **2FA Support**: Optional two-factor authentication
4. **Session Management**: View and revoke active sessions
5. **Role-Based Access**: Admin, user, viewer roles
6. **OAuth Integration**: Google, GitHub login
7. **Audit Logging**: Track all auth events

### Security Improvements
1. Password strength requirements
2. Account lockout after failed attempts
3. Email verification on registration
4. Remember me functionality
5. Device tracking and management

## Summary

The JWT authentication system provides:
✅ Secure email/password authentication
✅ Token-based session management
✅ Password reset flow
✅ Protected API routes
✅ Frontend auth context and routing
✅ Automatic token refresh
✅ Proper error handling

All components follow clean architecture principles and integrate seamlessly with the existing FreeRangeNotify infrastructure.
