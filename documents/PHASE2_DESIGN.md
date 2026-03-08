# Phase 2 — Core Platform: Detailed Design Document

> **Status**: Ready for Implementation  
> **Depends On**: Phase 1 (Workflow Engine, Digest Engine, HMAC for SSE) — ✅ Complete  
> **Estimated Duration**: 5–6 weeks  
> **Feature Count**: 4 features (Topics, Per-Subscriber Throttle, Audit Logs, RBAC)

---

## Table of Contents

- [Section 0: Concepts — What Are These Features?](#section-0-concepts--what-are-these-features)
- [Section 1: Topics / Subscriber Groups](#section-1-topics--subscriber-groups)
  - [1.1 Domain Model](#11-domain-model)
  - [1.2 Repository (Elasticsearch)](#12-repository-elasticsearch)
  - [1.3 Service Layer](#13-service-layer)
  - [1.4 HTTP Handler](#14-http-handler)
  - [1.5 Elasticsearch Index Templates](#15-elasticsearch-index-templates)
  - [1.6 Notification Integration (Fan-Out)](#16-notification-integration-fan-out)
  - [1.7 API Contracts](#17-api-contracts)
- [Section 2: Per-Subscriber Throttle](#section-2-per-subscriber-throttle)
  - [2.1 Design](#21-design)
  - [2.2 New Types on User & Application Models](#22-new-types-on-user--application-models)
  - [2.3 Throttle Implementation (Redis)](#23-throttle-implementation-redis)
  - [2.4 Worker Integration](#24-worker-integration)
- [Section 3: Audit Logs](#section-3-audit-logs)
  - [3.1 Domain Model](#31-domain-model)
  - [3.2 Repository](#32-repository)
  - [3.3 Service Layer](#33-service-layer)
  - [3.4 Audit Middleware](#34-audit-middleware)
  - [3.5 HTTP Handler](#35-http-handler)
  - [3.6 Elasticsearch Index Template](#36-elasticsearch-index-template)
  - [3.7 API Contracts](#37-api-contracts)
- [Section 4: RBAC (Role-Based Access Control)](#section-4-rbac-role-based-access-control)
  - [4.1 Role & Permission Model](#41-role--permission-model)
  - [4.2 AppMembership Model](#42-appmembership-model)
  - [4.3 Membership Repository](#43-membership-repository)
  - [4.4 Team Service](#44-team-service)
  - [4.5 RBAC Middleware](#45-rbac-middleware)
  - [4.6 Team Handler](#46-team-handler)
  - [4.7 Elasticsearch Index Template](#47-elasticsearch-index-template)
  - [4.8 Route-Level Permission Map](#48-route-level-permission-map)
  - [4.9 API Contracts](#49-api-contracts)
- [Section 5: Wiring — Container & Routes](#section-5-wiring--container--routes)
  - [5.1 Feature Flags](#51-feature-flags)
  - [5.2 Container Changes](#52-container-changes)
  - [5.3 Route Changes](#53-route-changes)
  - [5.4 Index Manager Changes](#54-index-manager-changes)
- [Section 6: File Inventory](#section-6-file-inventory)
- [Section 7: Backward Compatibility Contract](#section-7-backward-compatibility-contract)
- [Section 8: Implementation Order](#section-8-implementation-order)

---

## Section 0: Concepts — What Are These Features?

### Topics / Subscriber Groups

A **Topic** is a named channel that users can subscribe to inside an application.  Instead of listing individual user IDs every time you send a notification, you create a topic (e.g., `project-123-watchers`), subscribe users to it, and then send a single notification targeting that topic.  The system automatically fans out the notification to every subscriber.

**Example:** A project management app creates a topic called `release-notes`.  When a new version ships, one API call with `topic_id` delivers the notification to all 500 subscribers — no need to enumerate 500 user IDs.

### Per-Subscriber Throttle

**Per-Subscriber Throttle** limits how many notifications a single user receives within a time window, per channel.  This prevents notification fatigue.  Even if your app sends 100 emails to a user in an hour, the throttle can cap it at 10 and silently cancel the rest.

**How it differs from existing rate limiting:** The current `Limiter` is app-scoped (e.g., "this app can send 1000 emails/day").  Per-subscriber throttle is user-scoped (e.g., "this user receives at most 5 push notifications per hour").

### Audit Logs

**Audit Logs** automatically record every mutating action (create, update, delete) performed through the API.  They answer "who did what, when, and from where."  This is a compliance requirement for enterprise customers and a debugging tool for operators.

**What gets logged:** Template creation, user deletion, workflow updates, team member invitations — any POST/PUT/PATCH/DELETE that succeeds.  Logs are append-only and retained for 90 days via Elasticsearch ILM.

### RBAC (Role-Based Access Control)

**RBAC** assigns roles to dashboard users *per application*.  Currently, every authenticated admin has identical access.  With RBAC, you can invite team members as `viewer` (read-only), `editor` (manage templates/workflows), `admin` (full access except deletion), or `owner` (full access including app deletion).

**Scope:** Roles are per-app.  Alice can be `owner` of App A and `viewer` of App B.

---

## Section 1: Topics / Subscriber Groups

### 1.1 Domain Model

**New file:** `internal/domain/topic/models.go`

```go
package topic

import (
    "context"
    "time"
)

// Topic represents a named subscriber group within an application.
type Topic struct {
    ID          string    `json:"id" es:"topic_id"`
    AppID       string    `json:"app_id" es:"app_id"`
    Name        string    `json:"name" es:"name"`
    Key         string    `json:"key" es:"key"` // Machine-readable slug ("project-123-watchers")
    Description string    `json:"description,omitempty" es:"description"`
    CreatedAt   time.Time `json:"created_at" es:"created_at"`
    UpdatedAt   time.Time `json:"updated_at" es:"updated_at"`
}

// TopicSubscription links a user to a topic.
type TopicSubscription struct {
    ID        string    `json:"id" es:"subscription_id"`
    TopicID   string    `json:"topic_id" es:"topic_id"`
    AppID     string    `json:"app_id" es:"app_id"`
    UserID    string    `json:"user_id" es:"user_id"`
    CreatedAt time.Time `json:"created_at" es:"created_at"`
}

// CreateRequest is the input for creating a new topic.
type CreateRequest struct {
    Name        string `json:"name" validate:"required,min=1,max=255"`
    Key         string `json:"key" validate:"required,min=1,max=128"`
    Description string `json:"description,omitempty" validate:"max=1024"`
}

// AddSubscribersRequest is the input for adding subscribers to a topic.
type AddSubscribersRequest struct {
    UserIDs []string `json:"user_ids" validate:"required,min=1"`
}

// Repository defines data access for topics and subscriptions.
type Repository interface {
    // Topic CRUD
    Create(ctx context.Context, topic *Topic) error
    GetByID(ctx context.Context, id string) (*Topic, error)
    GetByKey(ctx context.Context, appID, key string) (*Topic, error)
    List(ctx context.Context, appID string, limit, offset int) ([]*Topic, int64, error)
    Update(ctx context.Context, topic *Topic) error
    Delete(ctx context.Context, id string) error

    // Subscription management
    AddSubscribers(ctx context.Context, topicID, appID string, userIDs []string) error
    RemoveSubscribers(ctx context.Context, topicID string, userIDs []string) error
    GetSubscribers(ctx context.Context, topicID string, limit, offset int) ([]TopicSubscription, int64, error)
    GetSubscriberCount(ctx context.Context, topicID string) (int64, error)
    GetUserTopics(ctx context.Context, appID, userID string) ([]*Topic, error)
    IsSubscribed(ctx context.Context, topicID, userID string) (bool, error)
}

// Service defines the business logic for topics.
type Service interface {
    Create(ctx context.Context, appID string, req *CreateRequest) (*Topic, error)
    Get(ctx context.Context, id, appID string) (*Topic, error)
    GetByKey(ctx context.Context, appID, key string) (*Topic, error)
    List(ctx context.Context, appID string, limit, offset int) ([]*Topic, int64, error)
    Delete(ctx context.Context, id, appID string) error
    AddSubscribers(ctx context.Context, topicID, appID string, userIDs []string) error
    RemoveSubscribers(ctx context.Context, topicID, appID string, userIDs []string) error
    GetSubscribers(ctx context.Context, topicID, appID string, limit, offset int) ([]TopicSubscription, int64, error)
    GetSubscriberUserIDs(ctx context.Context, topicID, appID string) ([]string, error)
}
```

**Design decisions:**
- `Key` is a machine-readable slug (e.g., `project-123-watchers`) — unique within an app, used for lookups without knowing the UUID.
- `List` returns `(items, total, error)` to support pagination metadata in API responses.
- `GetSubscriberUserIDs` is a convenience method that returns just the user IDs — used internally by the notification fan-out.

---

### 1.2 Repository (Elasticsearch)

**New file:** `internal/infrastructure/repository/topic_repository.go`

The repository uses two ES indices: `topics` and `topic_subscriptions`.  It composes `BaseRepository` for basic CRUD and implements topic-specific queries.

```go
// Constructor signature (follows existing pattern):
func NewTopicRepository(client *elasticsearch.Client, logger *zap.Logger) topic.Repository
```

Key implementation details:
- `Create` stores a `Topic` document in the `topics` index with `topic_id` as the document ID.
- `AddSubscribers` performs a bulk index into `topic_subscriptions`, generating a composite document ID: `{topicID}_{userID}` to enforce uniqueness (one subscription per user per topic).
- `RemoveSubscribers` performs a delete-by-query on `topic_subscriptions` matching `topic_id` AND `user_id IN [...]`.
- `GetSubscribers` queries `topic_subscriptions` with `topic_id` filter, sorted by `created_at desc`.
- `GetByKey` queries `topics` with `app_id` AND `key` — must be unique within an app.
- `Delete` cascades: deletes the topic AND all matching subscriptions (delete-by-query on `topic_subscriptions` where `topic_id` matches).

---

### 1.3 Service Layer

**New file:** `internal/usecases/services/topic_service_impl.go`

```go
// Constructor:
func NewTopicService(
    repo topic.Repository,
    logger *zap.Logger,
) topic.Service
```

Key behaviors:
- `Create` — generates a UUID for `topic.ID`, sets `AppID` from the auth context, validates key uniqueness by calling `repo.GetByKey` first (returns `ErrConflict` if duplicate).
- `Delete` — verifies the topic belongs to the requesting app before deletion.
- `AddSubscribers` — validates that all `userIDs` are non-empty strings.  Does NOT validate that users exist (subscribers may be created before users — eventual consistency).
- `GetSubscriberUserIDs` — pages through all subscribers (batch size 500) and returns a flat `[]string`.

---

### 1.4 HTTP Handler

**New file:** `internal/interfaces/http/handlers/topic_handler.go`

```go
type TopicHandler struct {
    service   topic.Service
    validator *validator.Validator
    logger    *zap.Logger
}

func NewTopicHandler(
    service topic.Service,
    validator *validator.Validator,
    logger *zap.Logger,
) *TopicHandler
```

Methods:
| Method | Signature | Description |
|--------|-----------|-------------|
| `Create` | `func (h *TopicHandler) Create(c *fiber.Ctx) error` | Parse `CreateRequest`, call `service.Create` |
| `Get` | `func (h *TopicHandler) Get(c *fiber.Ctx) error` | Path param `:id` |
| `GetByKey` | `func (h *TopicHandler) GetByKey(c *fiber.Ctx) error` | Path param `:key` |
| `List` | `func (h *TopicHandler) List(c *fiber.Ctx) error` | Query params `limit`, `offset` |
| `Delete` | `func (h *TopicHandler) Delete(c *fiber.Ctx) error` | Path param `:id` |
| `AddSubscribers` | `func (h *TopicHandler) AddSubscribers(c *fiber.Ctx) error` | Path param `:id`, body `AddSubscribersRequest` |
| `RemoveSubscribers` | `func (h *TopicHandler) RemoveSubscribers(c *fiber.Ctx) error` | Path param `:id`, body `AddSubscribersRequest` |
| `GetSubscribers` | `func (h *TopicHandler) GetSubscribers(c *fiber.Ctx) error` | Path param `:id`, query `limit`, `offset` |

All handlers extract `app_id` from `c.Locals("app_id")` (set by `APIKeyAuth` middleware).

---

### 1.5 Elasticsearch Index Templates

Add two new methods to `IndexTemplates`:

**`GetTopicsTemplate()`**

```go
func (it *IndexTemplates) GetTopicsTemplate() map[string]interface{} {
    return map[string]interface{}{
        "mappings": map[string]interface{}{
            "properties": map[string]interface{}{
                "topic_id":    map[string]interface{}{"type": "keyword"},
                "app_id":      map[string]interface{}{"type": "keyword"},
                "name":        map[string]interface{}{
                    "type": "text",
                    "fields": map[string]interface{}{
                        "keyword": map[string]interface{}{"type": "keyword"},
                    },
                },
                "key":         map[string]interface{}{"type": "keyword"},
                "description": map[string]interface{}{"type": "text"},
                "created_at":  map[string]interface{}{"type": "date"},
                "updated_at":  map[string]interface{}{"type": "date"},
            },
        },
    }
}
```

**`GetTopicSubscriptionsTemplate()`**

```go
func (it *IndexTemplates) GetTopicSubscriptionsTemplate() map[string]interface{} {
    return map[string]interface{}{
        "mappings": map[string]interface{}{
            "properties": map[string]interface{}{
                "subscription_id": map[string]interface{}{"type": "keyword"},
                "topic_id":        map[string]interface{}{"type": "keyword"},
                "app_id":          map[string]interface{}{"type": "keyword"},
                "user_id":         map[string]interface{}{"type": "keyword"},
                "created_at":      map[string]interface{}{"type": "date"},
            },
        },
    }
}
```

Register both in `IndexManager.CreateIndices`:

```go
"topics":              im.templates.GetTopicsTemplate,
"topic_subscriptions": im.templates.GetTopicSubscriptionsTemplate,
```

---

### 1.6 Notification Integration (Fan-Out)

**Modify:** `internal/domain/notification/models.go` — Add `TopicID` to `SendRequest`:

```go
type SendRequest struct {
    // ... existing fields ...
    TopicID string `json:"topic_id,omitempty"` // Send to all subscribers of a topic
}
```

**Modify:** `internal/usecases/notification_service.go` — In `Send()`:

```go
// After template resolution and validation, before user-specific logic:
if req.TopicID != "" {
    return s.sendToTopic(ctx, req)
}
```

New method on `NotificationService`:

```go
func (s *NotificationService) sendToTopic(ctx context.Context, req notification.SendRequest) (*notification.Notification, error) {
    if s.topicService == nil {
        return nil, errors.New(errors.ErrCodeBadRequest, "topics feature is not enabled")
    }

    // Get all subscriber user IDs
    userIDs, err := s.topicService.GetSubscriberUserIDs(ctx, req.TopicID, req.AppID)
    if err != nil {
        return nil, fmt.Errorf("failed to get topic subscribers: %w", err)
    }

    if len(userIDs) == 0 {
        return nil, errors.New(errors.ErrCodeNotFound, "topic has no subscribers")
    }

    // Fan out using existing bulk send infrastructure
    bulkReq := notification.BulkSendRequest{
        AppID:       req.AppID,
        UserIDs:     userIDs,
        Channel:     req.Channel,
        Priority:    req.Priority,
        Title:       req.Title,
        Body:        req.Body,
        Data:        req.Data,
        TemplateID:  req.TemplateID,
        Category:    req.Category,
        ScheduledAt: req.ScheduledAt,
    }

    results, err := s.SendBulk(ctx, bulkReq)
    if err != nil {
        return nil, err
    }

    // Return the first notification as representative
    if len(results) > 0 {
        return results[0], nil
    }
    return nil, nil
}
```

This requires adding a `topicService` field to `NotificationService` and a `SetTopicService` setter (avoids circular dependency — TopicService is optional).

```go
// In NotificationService struct:
topicService topic.Service

// Setter (called from container.go after both services are initialized):
func (s *NotificationService) SetTopicService(ts topic.Service) {
    s.topicService = ts
}
```

---

### 1.7 API Contracts

| Method | Path | Auth | Request Body | Response |
|--------|------|------|-------------|----------|
| `POST` | `/v1/topics` | API Key | `{ "name": "...", "key": "...", "description": "..." }` | `201` + `Topic` |
| `GET` | `/v1/topics` | API Key | — | `200` + `{ "topics": [...], "total": N }` |
| `GET` | `/v1/topics/:id` | API Key | — | `200` + `Topic` |
| `GET` | `/v1/topics/key/:key` | API Key | — | `200` + `Topic` |
| `DELETE` | `/v1/topics/:id` | API Key | — | `204` |
| `POST` | `/v1/topics/:id/subscribers` | API Key | `{ "user_ids": ["...", "..."] }` | `200` + `{ "added": N }` |
| `DELETE` | `/v1/topics/:id/subscribers` | API Key | `{ "user_ids": ["...", "..."] }` | `200` + `{ "removed": N }` |
| `GET` | `/v1/topics/:id/subscribers` | API Key | — | `200` + `{ "subscribers": [...], "total": N }` |
| `POST` | `/v1/notifications` | API Key | `{ ..., "topic_id": "..." }` | `201` + `Notification` (fan-out) |

---

## Section 2: Per-Subscriber Throttle

### 2.1 Design

Per-subscriber throttle limits how many notifications a single user receives per channel within configurable time windows.  The throttle is checked in the **worker** (`processor.go`), not in the API server — so the notification is still accepted and queued, but the worker cancels it if the user has exceeded their limit.

**Redis key pattern:**

```
frn:throttle:{app_id}:{user_id}:{channel}:hourly
frn:throttle:{app_id}:{user_id}:{channel}:daily
```

**Resolution order (most specific wins):**
1. User-level: `user.Preferences.Throttle[channel]`
2. App-level: `application.Settings.SubscriberThrottle[channel]`
3. System default: no throttle (unlimited)

If the throttle limit is reached, the notification is marked `StatusCancelled` with `error_message: "throttled: subscriber {window} limit reached for {channel}"`.

### 2.2 New Types on User & Application Models

**Modify:** `internal/domain/user/models.go`

```go
// ThrottleConfig defines per-channel notification rate limits for a subscriber.
type ThrottleConfig struct {
    MaxPerHour int `json:"max_per_hour,omitempty" es:"max_per_hour"`
    MaxPerDay  int `json:"max_per_day,omitempty" es:"max_per_day"`
}

// Add to Preferences struct:
type Preferences struct {
    // ... existing fields ...
    Throttle map[string]ThrottleConfig `json:"throttle,omitempty" es:"throttle"` // key = channel name
}
```

**Modify:** `internal/domain/application/models.go`

```go
// Add to Settings struct (reuses user.ThrottleConfig):
type Settings struct {
    // ... existing fields ...
    SubscriberThrottle map[string]SubscriberThrottleConfig `json:"subscriber_throttle,omitempty" es:"subscriber_throttle"`
}

// SubscriberThrottleConfig defines app-wide default throttle for all subscribers.
type SubscriberThrottleConfig struct {
    MaxPerHour int `json:"max_per_hour,omitempty" es:"max_per_hour"`
    MaxPerDay  int `json:"max_per_day,omitempty" es:"max_per_day"`
}
```

> **Why separate types?** The user model lives in `domain/user`, the application model in `domain/application`.  Go packages cannot have circular imports, so each defines its own throttle config struct (same shape, different packages).

### 2.3 Throttle Implementation (Redis)

**New file:** `internal/infrastructure/limiter/subscriber_throttle.go`

```go
package limiter

import (
    "context"
    "fmt"
    "time"

    "github.com/go-redis/redis/v8"
    "go.uber.org/zap"
)

// SubscriberThrottle enforces per-user, per-channel rate limits.
type SubscriberThrottle struct {
    client *redis.Client
    logger *zap.Logger
}

// NewSubscriberThrottle creates a new subscriber throttle backed by Redis.
func NewSubscriberThrottle(client *redis.Client, logger *zap.Logger) *SubscriberThrottle {
    return &SubscriberThrottle{client: client, logger: logger}
}

// ThrottleCheck holds the resolved limits for a single check.
type ThrottleCheck struct {
    MaxPerHour int
    MaxPerDay  int
}

// Allow checks whether a notification is allowed under the subscriber's throttle.
// If allowed, it increments the counter.  Returns (allowed bool, reason string).
func (st *SubscriberThrottle) Allow(ctx context.Context, appID, userID, channel string, check ThrottleCheck) (bool, string) {
    // Check hourly limit
    if check.MaxPerHour > 0 {
        hourlyKey := fmt.Sprintf("frn:throttle:%s:%s:%s:hourly", appID, userID, channel)
        count, err := st.client.Get(ctx, hourlyKey).Int()
        if err != nil && err != redis.Nil {
            st.logger.Error("Failed to read hourly throttle counter", zap.Error(err))
            return true, "" // Fail open — don't block on Redis errors
        }
        if count >= check.MaxPerHour {
            return false, fmt.Sprintf("throttled: subscriber hourly limit reached for %s (%d/%d)", channel, count, check.MaxPerHour)
        }
    }

    // Check daily limit
    if check.MaxPerDay > 0 {
        dailyKey := fmt.Sprintf("frn:throttle:%s:%s:%s:daily", appID, userID, channel)
        count, err := st.client.Get(ctx, dailyKey).Int()
        if err != nil && err != redis.Nil {
            st.logger.Error("Failed to read daily throttle counter", zap.Error(err))
            return true, ""
        }
        if count >= check.MaxPerDay {
            return false, fmt.Sprintf("throttled: subscriber daily limit reached for %s (%d/%d)", channel, count, check.MaxPerDay)
        }
    }

    // Increment both counters
    if check.MaxPerHour > 0 {
        hourlyKey := fmt.Sprintf("frn:throttle:%s:%s:%s:hourly", appID, userID, channel)
        st.client.Incr(ctx, hourlyKey)
        st.client.Expire(ctx, hourlyKey, time.Hour)
    }
    if check.MaxPerDay > 0 {
        dailyKey := fmt.Sprintf("frn:throttle:%s:%s:%s:daily", appID, userID, channel)
        st.client.Incr(ctx, dailyKey)
        st.client.Expire(ctx, dailyKey, 24*time.Hour)
    }

    return true, ""
}
```

**Design note:** The throttle **fails open** — if Redis is unreachable, we allow the notification rather than silently dropping it.  This is intentional: throttling is a best-effort safeguard, not a hard security boundary.

### 2.4 Worker Integration

**Modify:** `cmd/worker/processor.go`

Add a `throttle *limiter.SubscriberThrottle` field to `NotificationProcessor`.  Inject it from `main.go`.

Insert the throttle check **after** user preference checks but **before** template rendering:

```go
// In processNotification(), after checkUserPreferences:
if p.throttle != nil && usr != nil {
    check := p.resolveThrottleLimits(ctx, usr, notif.AppID, notif.Channel)
    if check.MaxPerHour > 0 || check.MaxPerDay > 0 {
        allowed, reason := p.throttle.Allow(ctx, notif.AppID, notif.UserID, string(notif.Channel), check)
        if !allowed {
            logger.Info("Notification throttled",
                zap.String("notification_id", notif.NotificationID),
                zap.String("reason", reason))
            p.notifRepo.UpdateStatus(ctx, notif.NotificationID, notification.StatusCancelled)
            // Store throttle reason
            p.notifRepo.IncrementRetryCount(ctx, notif.NotificationID, reason)
            return
        }
    }
}
```

Throttle limit resolution method:

```go
func (p *NotificationProcessor) resolveThrottleLimits(
    ctx context.Context, usr *user.User, appID string, ch notification.Channel,
) limiter.ThrottleCheck {
    channel := string(ch)
    check := limiter.ThrottleCheck{}

    // 1. User-level throttle (most specific — wins)
    if usr.Preferences.Throttle != nil {
        if tc, ok := usr.Preferences.Throttle[channel]; ok {
            check.MaxPerHour = tc.MaxPerHour
            check.MaxPerDay = tc.MaxPerDay
            return check
        }
    }

    // 2. App-level throttle (fallback)
    app, err := p.appRepo.GetByID(ctx, appID)
    if err == nil && app != nil && app.Settings.SubscriberThrottle != nil {
        if tc, ok := app.Settings.SubscriberThrottle[channel]; ok {
            check.MaxPerHour = tc.MaxPerHour
            check.MaxPerDay = tc.MaxPerDay
        }
    }

    return check
}
```

---

## Section 3: Audit Logs

### 3.1 Domain Model

**New file:** `internal/domain/audit/models.go`

```go
package audit

import (
    "context"
    "time"
)

// AuditLog represents a single audit event.
type AuditLog struct {
    ID         string         `json:"id" es:"audit_id"`
    AppID      string         `json:"app_id,omitempty" es:"app_id"`
    ActorID    string         `json:"actor_id" es:"actor_id"`
    ActorType  string         `json:"actor_type" es:"actor_type"` // "admin_user", "api_key", "system"
    Action     string         `json:"action" es:"action"`         // "template.create", "user.delete", etc.
    Resource   string         `json:"resource" es:"resource"`     // "template", "user", "topic", etc.
    ResourceID string         `json:"resource_id,omitempty" es:"resource_id"`
    Changes    map[string]any `json:"changes,omitempty" es:"changes"`
    IPAddress  string         `json:"ip_address" es:"ip_address"`
    UserAgent  string         `json:"user_agent" es:"user_agent"`
    Timestamp  time.Time      `json:"timestamp" es:"timestamp"`
}

// Filter defines query parameters for listing audit logs.
type Filter struct {
    AppID      string     `json:"app_id,omitempty"`
    ActorID    string     `json:"actor_id,omitempty"`
    Action     string     `json:"action,omitempty"`
    Resource   string     `json:"resource,omitempty"`
    ResourceID string     `json:"resource_id,omitempty"`
    FromDate   *time.Time `json:"from_date,omitempty"`
    ToDate     *time.Time `json:"to_date,omitempty"`
    Limit      int        `json:"limit,omitempty"`
    Offset     int        `json:"offset,omitempty"`
}

// DefaultFilter returns a Filter with sensible defaults.
func DefaultFilter() Filter {
    return Filter{
        Limit:  50,
        Offset: 0,
    }
}

// Repository defines data access for audit logs (append-only).
type Repository interface {
    Create(ctx context.Context, log *AuditLog) error
    List(ctx context.Context, filter Filter) ([]*AuditLog, int64, error)
    GetByID(ctx context.Context, id string) (*AuditLog, error)
}

// Service defines business logic for audit logging.
type Service interface {
    Log(ctx context.Context, log *AuditLog) error
    List(ctx context.Context, filter Filter) ([]*AuditLog, int64, error)
    GetByID(ctx context.Context, id string) (*AuditLog, error)
}
```

**Design decisions:**
- No `Update` or `Delete` on the repository — audit logs are append-only by design.
- `Changes` is `map[string]any` with ES mapping `"enabled": false` — stored but not searchable (avoids schema explosion).
- `ActorType` distinguishes API key actions (protected routes) from admin user actions (JWT routes) from system actions (worker, scheduler).

---

### 3.2 Repository

**New file:** `internal/infrastructure/repository/audit_repository.go`

```go
func NewAuditRepository(client *elasticsearch.Client, logger *zap.Logger) audit.Repository
```

Uses `BaseRepository` with index name `audit_logs`.  Key methods:
- `Create` — standard index operation.
- `List` — builds a bool query from `Filter` fields (all optional), sorted by `timestamp desc`.
- `GetByID` — standard get-by-ID.

---

### 3.3 Service Layer

**New file:** `internal/usecases/services/audit_service_impl.go`

```go
func NewAuditService(repo audit.Repository, logger *zap.Logger) audit.Service
```

`Log` method:
- Validates that `ActorID` and `Action` are non-empty.
- Sets `Timestamp` to `time.Now().UTC()` if not already set.
- Generates `ID` via `uuid.New()` if not already set.
- Calls `repo.Create` — fire-and-forget semantics (errors are logged but do not bubble up to the caller in the middleware path).

---

### 3.4 Audit Middleware

**New file:** `internal/interfaces/http/middleware/audit_middleware.go`

```go
func AuditMiddleware(auditService audit.Service, logger *zap.Logger) fiber.Handler
```

Flow:
1. Skip non-mutating methods (`GET`, `HEAD`, `OPTIONS`).
2. Call `c.Next()` to execute the handler.
3. After the handler, if the response status is `2xx`:
   - Extract `app_id` from `c.Locals("app_id")` (API key routes) or derive from path.
   - Extract `actor_id` from `c.Locals("user_id")` (JWT routes) or `c.Locals("app_id")` (API key routes).
   - Determine `actor_type`: if `c.Locals("user_id")` is set → `"admin_user"`, else `"api_key"`.
   - Parse route path to derive `resource`, `resource_id`, `action` (see `parseRoute` helper below).
   - Call `auditService.Log(...)` in a goroutine (non-blocking).

**`parseRoute` helper:**

```go
// parseRoute extracts resource, resourceID, and action from a Fiber route.
// Example: POST /v1/templates → resource="template", action="template.create"
// Example: DELETE /v1/users/abc-123 → resource="user", resourceID="abc-123", action="user.delete"
func parseRoute(path, method string) (resource, resourceID, action string) {
    // Strip /v1/ prefix and split
    parts := strings.Split(strings.TrimPrefix(path, "/v1/"), "/")
    if len(parts) == 0 {
        return "unknown", "", "unknown"
    }

    resource = strings.TrimSuffix(parts[0], "s") // "templates" → "template"

    if len(parts) >= 2 {
        resourceID = parts[1]
    }

    verb := map[string]string{
        "POST":   "create",
        "PUT":    "update",
        "PATCH":  "update",
        "DELETE": "delete",
    }[method]

    action = resource + "." + verb
    return
}
```

---

### 3.5 HTTP Handler

**New file:** `internal/interfaces/http/handlers/audit_handler.go`

```go
type AuditHandler struct {
    service audit.Service
    logger  *zap.Logger
}

func NewAuditHandler(service audit.Service, logger *zap.Logger) *AuditHandler
```

Methods:
| Method | Signature | Description |
|--------|-----------|-------------|
| `List` | `func (h *AuditHandler) List(c *fiber.Ctx) error` | Query params: `app_id`, `actor_id`, `action`, `resource`, `from`, `to`, `limit`, `offset` |
| `Get` | `func (h *AuditHandler) Get(c *fiber.Ctx) error` | Path param `:id` |

Both methods are admin-only (JWT-protected).

---

### 3.6 Elasticsearch Index Template

**New method on `IndexTemplates`:**

```go
func (it *IndexTemplates) GetAuditLogsTemplate() map[string]interface{} {
    return map[string]interface{}{
        "mappings": map[string]interface{}{
            "properties": map[string]interface{}{
                "audit_id":    map[string]interface{}{"type": "keyword"},
                "app_id":      map[string]interface{}{"type": "keyword"},
                "actor_id":    map[string]interface{}{"type": "keyword"},
                "actor_type":  map[string]interface{}{"type": "keyword"},
                "action":      map[string]interface{}{"type": "keyword"},
                "resource":    map[string]interface{}{"type": "keyword"},
                "resource_id": map[string]interface{}{"type": "keyword"},
                "changes":     map[string]interface{}{"type": "object", "enabled": false},
                "ip_address":  map[string]interface{}{"type": "ip"},
                "user_agent":  map[string]interface{}{"type": "text"},
                "timestamp":   map[string]interface{}{"type": "date"},
            },
        },
        "settings": map[string]interface{}{
            "index": map[string]interface{}{
                "number_of_replicas": 1,
            },
        },
    }
}
```

Register in `IndexManager.CreateIndices`:
```go
"audit_logs": im.templates.GetAuditLogsTemplate,
```

---

### 3.7 API Contracts

| Method | Path | Auth | Request | Response |
|--------|------|------|---------|----------|
| `GET` | `/v1/admin/audit-logs` | JWT | Query: `app_id`, `actor_id`, `action`, `resource`, `from`, `to`, `limit`, `offset` | `200` + `{ "logs": [...], "total": N }` |
| `GET` | `/v1/admin/audit-logs/:id` | JWT | — | `200` + `AuditLog` |

---

## Section 4: RBAC (Role-Based Access Control)

### 4.1 Role & Permission Model

**Modify:** `internal/domain/auth/models.go` — Add at the end of the file:

```go
// ── RBAC Types ──

// Role defines the access level for a dashboard user within an application.
type Role string

const (
    RoleOwner  Role = "owner"  // Full access, can delete app
    RoleAdmin  Role = "admin"  // Full access except app deletion
    RoleEditor Role = "editor" // Manage templates, workflows, topics; cannot manage team
    RoleViewer Role = "viewer" // Read-only access
)

// Valid checks whether the role is a known role.
func (r Role) Valid() bool {
    switch r {
    case RoleOwner, RoleAdmin, RoleEditor, RoleViewer:
        return true
    default:
        return false
    }
}

// Permission defines a granular action that can be allowed or denied.
type Permission string

const (
    PermTemplateWrite    Permission = "template:write"
    PermTemplateRead     Permission = "template:read"
    PermWorkflowWrite    Permission = "workflow:write"
    PermWorkflowRead     Permission = "workflow:read"
    PermTopicManage      Permission = "topic:manage"
    PermUserManage       Permission = "user:manage"
    PermTeamManage       Permission = "team:manage"
    PermSettingsManage   Permission = "settings:manage"
    PermNotificationSend Permission = "notification:send"
    PermAuditRead        Permission = "audit:read"
    PermAnalyticsRead    Permission = "analytics:read"
    PermAppDelete        Permission = "app:delete"
)

// RolePermissions maps each role to its set of allowed permissions.
var RolePermissions = map[Role][]Permission{
    RoleOwner: {
        PermTemplateWrite, PermTemplateRead,
        PermWorkflowWrite, PermWorkflowRead,
        PermTopicManage, PermUserManage, PermTeamManage,
        PermSettingsManage, PermNotificationSend,
        PermAuditRead, PermAnalyticsRead, PermAppDelete,
    },
    RoleAdmin: {
        PermTemplateWrite, PermTemplateRead,
        PermWorkflowWrite, PermWorkflowRead,
        PermTopicManage, PermUserManage, PermTeamManage,
        PermSettingsManage, PermNotificationSend,
        PermAuditRead, PermAnalyticsRead,
    },
    RoleEditor: {
        PermTemplateWrite, PermTemplateRead,
        PermWorkflowWrite, PermWorkflowRead,
        PermTopicManage, PermNotificationSend,
        PermAnalyticsRead,
    },
    RoleViewer: {
        PermTemplateRead, PermWorkflowRead, PermAnalyticsRead,
    },
}

// HasPermission checks whether a role includes a given permission.
func (r Role) HasPermission(perm Permission) bool {
    perms, ok := RolePermissions[r]
    if !ok {
        return false
    }
    for _, p := range perms {
        if p == perm {
            return true
        }
    }
    return false
}
```

### 4.2 AppMembership Model

```go
// AppMembership links a dashboard user (AdminUser) to an application with a specific role.
type AppMembership struct {
    ID        string    `json:"id" es:"membership_id"`
    AppID     string    `json:"app_id" es:"app_id"`
    UserID    string    `json:"user_id" es:"user_id"`       // AdminUser.UserID
    UserEmail string    `json:"user_email" es:"user_email"` // Denormalized for display
    Role      Role      `json:"role" es:"role"`
    InvitedBy string    `json:"invited_by" es:"invited_by"` // AdminUser.UserID who invited
    CreatedAt time.Time `json:"created_at" es:"created_at"`
    UpdatedAt time.Time `json:"updated_at" es:"updated_at"`
}

// InviteMemberRequest represents a request to invite a team member.
type InviteMemberRequest struct {
    Email string `json:"email" validate:"required,email"`
    Role  Role   `json:"role" validate:"required"`
}

// UpdateMemberRoleRequest represents a request to change a member's role.
type UpdateMemberRoleRequest struct {
    Role Role `json:"role" validate:"required"`
}
```

### 4.3 Membership Repository

**New file:** `internal/infrastructure/repository/membership_repository.go`

```go
// MembershipRepository interface (defined in auth domain or as a separate concern)
type MembershipRepository interface {
    Create(ctx context.Context, membership *auth.AppMembership) error
    GetByID(ctx context.Context, id string) (*auth.AppMembership, error)
    GetByAppAndUser(ctx context.Context, appID, userID string) (*auth.AppMembership, error)
    List(ctx context.Context, appID string) ([]*auth.AppMembership, error)
    Update(ctx context.Context, membership *auth.AppMembership) error
    Delete(ctx context.Context, id string) error
    GetUserMemberships(ctx context.Context, userID string) ([]*auth.AppMembership, error)
}
```

Add `MembershipRepository` interface to `internal/domain/auth/models.go`.

Constructor:
```go
func NewMembershipRepository(client *elasticsearch.Client, logger *zap.Logger) auth.MembershipRepository
```

ES index: `app_memberships`.  Document ID: membership UUID.

Key query: `GetByAppAndUser` — used by RBAC middleware to resolve the current user's role for a specific app.

---

### 4.4 Team Service

**New file:** `internal/usecases/services/team_service_impl.go`

```go
// TeamService interface (add to auth domain or create separate interface file)
type TeamService interface {
    InviteMember(ctx context.Context, appID, inviterID string, req *auth.InviteMemberRequest) (*auth.AppMembership, error)
    ListMembers(ctx context.Context, appID string) ([]*auth.AppMembership, error)
    UpdateMemberRole(ctx context.Context, membershipID, appID, actorID string, req *auth.UpdateMemberRoleRequest) (*auth.AppMembership, error)
    RemoveMember(ctx context.Context, membershipID, appID, actorID string) error
    GetUserRole(ctx context.Context, appID, userID string) (auth.Role, error)
}
```

Add `TeamService` interface to `internal/domain/auth/models.go`.

Constructor:
```go
func NewTeamService(
    membershipRepo auth.MembershipRepository,
    authRepo auth.Repository,
    logger *zap.Logger,
) auth.TeamService
```

Key behaviors:
- `InviteMember` — looks up the invited user by email in `authRepo`.  If the user doesn't exist, creates a placeholder membership (the user will see it when they register/login).  Validates `req.Role.Valid()`.  The app creator is always `RoleOwner`.
- `UpdateMemberRole` — verifies the actor has `PermTeamManage`.  Cannot change the owner's role (only the owner can transfer ownership).  Cannot escalate above own role.
- `RemoveMember` — verifies the actor has `PermTeamManage`.  Cannot remove the owner.
- `GetUserRole` — returns `RoleOwner` if the user's `UserID` matches `application.AdminUserID` (backward compat: existing app creators are owners even without a membership record).

---

### 4.5 RBAC Middleware

**New file:** `internal/interfaces/http/middleware/rbac_middleware.go`

```go
package middleware

import (
    "github.com/gofiber/fiber/v2"
    "github.com/the-monkeys/freerangenotify/internal/domain/auth"
    pkgerrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

// RequirePermission returns a middleware that checks whether the current user has
// the given permission for the app referenced in the request.
func RequirePermission(teamService auth.TeamService, permission auth.Permission) fiber.Handler {
    return func(c *fiber.Ctx) error {
        userID, ok := c.Locals("user_id").(string)
        if !ok || userID == "" {
            return pkgerrors.New(pkgerrors.ErrCodeUnauthorized, "authentication required")
        }

        // Resolve appID from path or locals
        appID := c.Params("id")
        if appID == "" {
            appID, _ = c.Locals("app_id").(string)
        }
        if appID == "" {
            // No app context — skip RBAC (e.g., /admin/me)
            return c.Next()
        }

        role, err := teamService.GetUserRole(c.Context(), appID, userID)
        if err != nil {
            return pkgerrors.New(pkgerrors.ErrCodeForbidden, "no access to this application")
        }

        if !role.HasPermission(permission) {
            return pkgerrors.New(pkgerrors.ErrCodeForbidden, "insufficient permissions")
        }

        // Store role in context for downstream handlers
        c.Locals("role", role)
        return c.Next()
    }
}
```

**Backward compatibility:** When RBAC is disabled (no `TeamService` wired), the middleware is simply not applied.  Existing routes continue working as before.

---

### 4.6 Team Handler

**New file:** `internal/interfaces/http/handlers/team_handler.go`

```go
type TeamHandler struct {
    teamService auth.TeamService
    validator   *validator.Validator
    logger      *zap.Logger
}

func NewTeamHandler(
    teamService auth.TeamService,
    validator *validator.Validator,
    logger *zap.Logger,
) *TeamHandler
```

Methods:
| Method | Signature | Description |
|--------|-----------|-------------|
| `InviteMember` | `func (h *TeamHandler) InviteMember(c *fiber.Ctx) error` | Path `:id` (app), body `InviteMemberRequest` |
| `ListMembers` | `func (h *TeamHandler) ListMembers(c *fiber.Ctx) error` | Path `:id` (app) |
| `UpdateMemberRole` | `func (h *TeamHandler) UpdateMemberRole(c *fiber.Ctx) error` | Path `:id` (app), `:memberID`, body `UpdateMemberRoleRequest` |
| `RemoveMember` | `func (h *TeamHandler) RemoveMember(c *fiber.Ctx) error` | Path `:id` (app), `:memberID` |

---

### 4.7 Elasticsearch Index Template

```go
func (it *IndexTemplates) GetAppMembershipsTemplate() map[string]interface{} {
    return map[string]interface{}{
        "mappings": map[string]interface{}{
            "properties": map[string]interface{}{
                "membership_id": map[string]interface{}{"type": "keyword"},
                "app_id":        map[string]interface{}{"type": "keyword"},
                "user_id":       map[string]interface{}{"type": "keyword"},
                "user_email":    map[string]interface{}{"type": "keyword"},
                "role":          map[string]interface{}{"type": "keyword"},
                "invited_by":    map[string]interface{}{"type": "keyword"},
                "created_at":    map[string]interface{}{"type": "date"},
                "updated_at":    map[string]interface{}{"type": "date"},
            },
        },
    }
}
```

Register in `IndexManager.CreateIndices`:
```go
"app_memberships": im.templates.GetAppMembershipsTemplate,
```

---

### 4.8 Route-Level Permission Map

When RBAC is enabled, apply per-route permission checks on admin routes:

| Route Pattern | Permission |
|---------------|-----------|
| `POST /v1/apps/:id/members` | `team:manage` |
| `GET /v1/apps/:id/members` | `team:manage` |
| `PUT /v1/apps/:id/members/:memberID` | `team:manage` |
| `DELETE /v1/apps/:id/members/:memberID` | `team:manage` |
| `PUT /v1/apps/:id/settings` | `settings:manage` |
| `DELETE /v1/apps/:id` | `app:delete` |
| `GET /v1/admin/audit-logs` | `audit:read` |
| `GET /v1/admin/analytics/*` | `analytics:read` |

Non-RBAC'd routes (`/admin/me`, `/apps` list, health) remain accessible to any authenticated admin.

---

### 4.9 API Contracts

| Method | Path | Auth | Request Body | Response |
|--------|------|------|-------------|----------|
| `POST` | `/v1/apps/:id/members` | JWT | `{ "email": "...", "role": "editor" }` | `201` + `AppMembership` |
| `GET` | `/v1/apps/:id/members` | JWT | — | `200` + `{ "members": [...] }` |
| `PUT` | `/v1/apps/:id/members/:memberID` | JWT | `{ "role": "admin" }` | `200` + `AppMembership` |
| `DELETE` | `/v1/apps/:id/members/:memberID` | JWT | — | `204` |

---

## Section 5: Wiring — Container & Routes

### 5.1 Feature Flags

**Modify:** `internal/config/config.go`

```go
type FeaturesConfig struct {
    // Phase 1
    WorkflowEnabled bool `mapstructure:"workflow_enabled" yaml:"workflow_enabled"`
    DigestEnabled   bool `mapstructure:"digest_enabled" yaml:"digest_enabled"`
    SSEHMACEnforced bool `mapstructure:"sse_hmac_enforced" yaml:"sse_hmac_enforced"`

    // Phase 2
    TopicsEnabled   bool `mapstructure:"topics_enabled" yaml:"topics_enabled"`
    ThrottleEnabled bool `mapstructure:"throttle_enabled" yaml:"throttle_enabled"`
    AuditEnabled    bool `mapstructure:"audit_enabled" yaml:"audit_enabled"`
    RBACEnabled     bool `mapstructure:"rbac_enabled" yaml:"rbac_enabled"`
}
```

**Modify:** `config/config.yaml` — Add defaults under `features:`:

```yaml
features:
  # Phase 1
  workflow_enabled: false
  digest_enabled: false
  sse_hmac_enforced: false
  # Phase 2
  topics_enabled: false
  throttle_enabled: false
  audit_enabled: false
  rbac_enabled: false
```

### 5.2 Container Changes

**Modify:** `internal/container/container.go`

Add new fields:

```go
// Phase 2: Topics (feature-gated)
TopicService topic.Service
TopicHandler *handlers.TopicHandler

// Phase 2: Audit Logs (feature-gated)
AuditService audit.Service
AuditHandler *handlers.AuditHandler

// Phase 2: RBAC & Team Management (feature-gated)
TeamService  auth.TeamService
TeamHandler  *handlers.TeamHandler

// Phase 2: Per-Subscriber Throttle
SubscriberThrottle *limiter.SubscriberThrottle
```

Initialization (inside `NewContainer`, after existing Phase 1 blocks):

```go
// ── Phase 2: Topics (feature-gated) ──
if cfg.Features.TopicsEnabled {
    topicRepo := repository.NewTopicRepository(dbManager.Client.GetClient(), logger)
    container.TopicService = services.NewTopicService(topicRepo, logger)
    container.TopicHandler = handlers.NewTopicHandler(
        container.TopicService,
        container.Validator,
        logger,
    )
    // Wire topic service into notification service for fan-out
    if ns, ok := container.NotificationService.(*NotificationService); ok {
        ns.SetTopicService(container.TopicService)
    }
    logger.Info("Topics feature enabled")
}

// ── Phase 2: Per-Subscriber Throttle ──
if cfg.Features.ThrottleEnabled {
    container.SubscriberThrottle = limiter.NewSubscriberThrottle(redisClient, logger)
    logger.Info("Per-subscriber throttle enabled")
}

// ── Phase 2: Audit Logs (feature-gated) ──
if cfg.Features.AuditEnabled {
    auditRepo := repository.NewAuditRepository(dbManager.Client.GetClient(), logger)
    container.AuditService = services.NewAuditService(auditRepo, logger)
    container.AuditHandler = handlers.NewAuditHandler(
        container.AuditService,
        logger,
    )
    logger.Info("Audit logging enabled")
}

// ── Phase 2: RBAC (feature-gated) ──
if cfg.Features.RBACEnabled {
    membershipRepo := repository.NewMembershipRepository(dbManager.Client.GetClient(), logger)
    authRepo := repository.NewAuthRepository(dbManager.Client.GetClient(), logger)
    container.TeamService = services.NewTeamService(membershipRepo, authRepo, logger)
    container.TeamHandler = handlers.NewTeamHandler(
        container.TeamService,
        container.Validator,
        logger,
    )
    logger.Info("RBAC enabled")
}
```

### 5.3 Route Changes

**Modify:** `internal/interfaces/http/routes/routes.go`

In `setupProtectedRoutes`:

```go
// ── Phase 2: Topic routes (feature-gated) ──
if c.TopicHandler != nil {
    topics := v1.Group("/topics")
    topics.Use(auth)
    topics.Post("/", c.TopicHandler.Create)
    topics.Get("/", c.TopicHandler.List)
    topics.Get("/key/:key", c.TopicHandler.GetByKey)
    topics.Get("/:id", c.TopicHandler.Get)
    topics.Delete("/:id", c.TopicHandler.Delete)
    topics.Post("/:id/subscribers", c.TopicHandler.AddSubscribers)
    topics.Delete("/:id/subscribers", c.TopicHandler.RemoveSubscribers)
    topics.Get("/:id/subscribers", c.TopicHandler.GetSubscribers)
}
```

In `setupAdminRoutes`:

```go
// ── Phase 2: Audit log routes (feature-gated) ──
if c.AuditHandler != nil {
    adminAuth.Get("/audit-logs", c.AuditHandler.List)
    adminAuth.Get("/audit-logs/:id", c.AuditHandler.Get)
}

// ── Phase 2: Audit middleware (feature-gated) ──
// Apply to both protected and admin routes for comprehensive coverage
if c.AuditService != nil {
    // The middleware is applied at the v1 level so it catches all mutating requests.
    // It's a post-handler hook — reads c.Locals set by auth middleware.
}

// ── Phase 2: Team management routes (feature-gated) ──
if c.TeamHandler != nil {
    apps.Post("/:id/members", c.TeamHandler.InviteMember)
    apps.Get("/:id/members", c.TeamHandler.ListMembers)
    apps.Put("/:id/members/:memberID", c.TeamHandler.UpdateMemberRole)
    apps.Delete("/:id/members/:memberID", c.TeamHandler.RemoveMember)
}
```

### 5.4 Index Manager Changes

**Modify:** `internal/infrastructure/database/index_manager.go`

Add to the `indices` map in `CreateIndices`:

```go
// Phase 2 additions
"topics":              im.templates.GetTopicsTemplate,
"topic_subscriptions": im.templates.GetTopicSubscriptionsTemplate,
"audit_logs":          im.templates.GetAuditLogsTemplate,
"app_memberships":     im.templates.GetAppMembershipsTemplate,
```

---

## Section 6: File Inventory

### New Files (16)

| # | File | Feature |
|---|------|---------|
| 1 | `internal/domain/topic/models.go` | Topics |
| 2 | `internal/infrastructure/repository/topic_repository.go` | Topics |
| 3 | `internal/usecases/services/topic_service_impl.go` | Topics |
| 4 | `internal/interfaces/http/handlers/topic_handler.go` | Topics |
| 5 | `internal/infrastructure/limiter/subscriber_throttle.go` | Throttle |
| 6 | `internal/domain/audit/models.go` | Audit |
| 7 | `internal/infrastructure/repository/audit_repository.go` | Audit |
| 8 | `internal/usecases/services/audit_service_impl.go` | Audit |
| 9 | `internal/interfaces/http/middleware/audit_middleware.go` | Audit |
| 10 | `internal/interfaces/http/handlers/audit_handler.go` | Audit |
| 11 | `internal/infrastructure/repository/membership_repository.go` | RBAC |
| 12 | `internal/usecases/services/team_service_impl.go` | RBAC |
| 13 | `internal/interfaces/http/handlers/team_handler.go` | RBAC |
| 14 | `internal/interfaces/http/middleware/rbac_middleware.go` | RBAC |

### Modified Files (9)

| # | File | Changes |
|---|------|---------|
| 1 | `internal/config/config.go` | Add Phase 2 feature flags |
| 2 | `config/config.yaml` | Add Phase 2 feature flag defaults |
| 3 | `internal/domain/notification/models.go` | Add `TopicID` to `SendRequest` |
| 4 | `internal/domain/user/models.go` | Add `ThrottleConfig` to `Preferences` |
| 5 | `internal/domain/application/models.go` | Add `SubscriberThrottleConfig` to `Settings` |
| 6 | `internal/domain/auth/models.go` | Add Role, Permission, AppMembership, MembershipRepository, TeamService |
| 7 | `internal/infrastructure/database/index_templates.go` | Add 4 new template methods |
| 8 | `internal/infrastructure/database/index_manager.go` | Register 4 new indices |
| 9 | `internal/container/container.go` | Wire Phase 2 services |
| 10 | `internal/interfaces/http/routes/routes.go` | Register Phase 2 routes |
| 11 | `internal/usecases/notification_service.go` | Add topic fan-out logic |
| 12 | `cmd/worker/processor.go` | Add throttle check |
| 13 | `cmd/worker/main.go` | Inject `SubscriberThrottle` into processor |

---

## Section 7: Backward Compatibility Contract

Phase 2 follows the same backward-compatibility principles as Phase 1:

1. **All features are feature-gated.** When `topics_enabled`, `throttle_enabled`, `audit_enabled`, and `rbac_enabled` are all `false` (the default), the system behaves identically to pre-Phase-2.

2. **No existing struct fields are removed or renamed.** Only additive changes:
   - `SendRequest` gets a new optional field `TopicID` (zero value = unused).
   - `Preferences` gets a new optional field `Throttle` (nil = no throttle).
   - `Settings` gets a new optional field `SubscriberThrottle` (nil = no throttle).

3. **No existing API contracts change.** All new endpoints are under new paths.  The `/v1/notifications` POST endpoint gains optional `topic_id` — clients that don't send it are unaffected.

4. **No existing Elasticsearch indices are modified.** Four new indices are created.  Existing indices (`applications`, `users`, `notifications`, `templates`, etc.) are untouched.

5. **No existing Go interfaces grow.** Topic, Audit, Team are all new interfaces.  Existing `notification.Service`, `user.Repository`, etc. keep their exact method set.

6. **Throttle is worker-side only.** It doesn't reject API calls — the notification is still accepted and queued.  The worker cancels it during processing if throttled.

---

## Section 8: Implementation Order

| Step | Feature | Priority | Depends On |
|------|---------|----------|------------|
| 1 | Feature flags (config) | — | Nothing |
| 2 | Topics domain + repo + service + handler | P1 | Step 1 |
| 3 | Topic ES indices | P1 | Step 2 |
| 4 | Topic fan-out in notification service | P1 | Step 2 |
| 5 | Per-Subscriber Throttle (limiter + model changes) | P2 | Step 1 |
| 6 | Throttle worker integration | P2 | Step 5 |
| 7 | Audit domain + repo + service | P2 | Step 1 |
| 8 | Audit middleware + handler | P2 | Step 7 |
| 9 | Audit ES index | P2 | Step 7 |
| 10 | RBAC types in auth domain | P2 | Step 1 |
| 11 | Membership repo + team service | P2 | Step 10 |
| 12 | RBAC middleware + team handler | P2 | Step 11 |
| 13 | RBAC ES index | P2 | Step 11 |
| 14 | Wire everything in container + routes | — | Steps 2-13 |
| 15 | Build verification (`go build ./...` + `go vet ./...`) | — | Step 14 |

Steps 2-4 (Topics) are P1 and should be implemented first.  Steps 5-13 (Throttle, Audit, RBAC) are P2 and independent of each other — they can be implemented in any order.

---

*End of Phase 2 Design Document.*
