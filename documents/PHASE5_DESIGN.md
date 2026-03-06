# Phase 5 — Client SDKs & Inbox: Detailed Design Document

> **Status:** Ready for Implementation
> **Dependencies:** Phase 4 Complete (verified `go build ./...` clean)
> **Duration:** 4 weeks (Weeks 17–20)
> **Feature Count:** 7 features (3 backend, 2 SDK rewrites, 1 React component library, 1 worker enhancement)

---

## Table of Contents

- [Section 0: Concepts](#section-0-concepts)
- [Section 1: Codebase Audit — Current SDK & Inbox State](#section-1-codebase-audit)
  - [1.1 Go SDK (Current)](#11-go-sdk-current)
  - [1.2 JS/TS SDK (Current)](#12-jsts-sdk-current)
  - [1.3 React SDK (Current)](#13-react-sdk-current)
  - [1.4 Backend API Surface](#14-backend-api-surface)
  - [1.5 Gaps & Debt](#15-gaps--debt)
- [Section 2: Feature 5.1 — Backend: Snooze, Archive & Bulk Actions](#section-2-feature-51--backend-snooze-archive--bulk-actions)
  - [2.1 New Notification Statuses](#21-new-notification-statuses)
  - [2.2 New Notification Fields](#22-new-notification-fields)
  - [2.3 New Service Methods](#23-new-service-methods)
  - [2.4 New API Endpoints](#24-new-api-endpoints)
  - [2.5 Snooze Worker (Un-snooze Loop)](#25-snooze-worker-un-snooze-loop)
  - [2.6 Implementation](#26-implementation)
- [Section 3: Feature 5.2 — Backend: Subscriber Hash Endpoint](#section-3-feature-52--backend-subscriber-hash-endpoint)
  - [3.1 Design](#31-design)
  - [3.2 Implementation](#32-implementation)
- [Section 4: Feature 5.3 — Backend: DTO Alignment for Phase 3 Channels](#section-4-feature-53--backend-dto-alignment-for-phase-3-channels)
  - [4.1 Gaps](#41-gaps)
  - [4.2 Implementation](#42-implementation)
- [Section 5: Feature 5.4 — Go SDK: Complete End-to-End Client](#section-5-feature-54--go-sdk-complete-end-to-end-client)
  - [5.1 Architecture](#51-architecture)
  - [5.2 Core Client & HTTP Transport](#52-core-client--http-transport)
  - [5.3 Notifications Sub-Client](#53-notifications-sub-client)
  - [5.4 Users Sub-Client](#54-users-sub-client)
  - [5.5 Templates Sub-Client](#55-templates-sub-client)
  - [5.6 Workflows Sub-Client](#56-workflows-sub-client)
  - [5.7 Topics Sub-Client](#57-topics-sub-client)
  - [5.8 Presence Sub-Client](#58-presence-sub-client)
  - [5.9 Types & Models](#59-types--models)
  - [5.10 Error Handling](#510-error-handling)
  - [5.11 File Inventory](#511-file-inventory)
- [Section 6: Feature 5.5 — JS/TS SDK: Complete End-to-End Client](#section-6-feature-55--jsts-sdk-complete-end-to-end-client)
  - [6.1 Architecture](#61-architecture)
  - [6.2 Core Client & HTTP Transport](#62-core-client--http-transport)
  - [6.3 Notifications Module](#63-notifications-module)
  - [6.4 Users Module](#64-users-module)
  - [6.5 Templates Module](#65-templates-module)
  - [6.6 Workflows Module](#66-workflows-module)
  - [6.7 Topics Module](#67-topics-module)
  - [6.8 SSE Connection](#68-sse-connection)
  - [6.9 Types](#69-types)
  - [6.10 File Inventory](#610-file-inventory)
- [Section 7: Feature 5.6 — React SDK: Component Library](#section-7-feature-56--react-sdk-component-library)
  - [7.1 Provider & Context](#71-provider--context)
  - [7.2 NotificationBell (Rewrite)](#72-notificationbell-rewrite)
  - [7.3 Preferences Component](#73-preferences-component)
  - [7.4 Headless Hooks](#74-headless-hooks)
  - [7.5 File Inventory](#75-file-inventory)
- [Section 8: Feature 5.7 — Snooze Worker Enhancement](#section-8-feature-57--snooze-worker-enhancement)
- [Section 9: Wiring & Container Integration](#section-9-wiring--container-integration)
- [Section 10: Implementation Order](#section-10-implementation-order)
- [Section 11: File Inventory (All Phases)](#section-11-file-inventory-all-phases)

---

## Section 0: Concepts

### Snooze
A user defers a notification to reappear later. The notification moves to `snoozed` status with a `snoozed_until` timestamp. A background loop in the worker checks for due snoozed notifications every 30 seconds, re-queues them as `pending`, and publishes an SSE event so the client sees it again.

### Archive
A permanent "dismiss" action. Archived notifications are removed from the inbox but remain in the system for auditing. Status becomes `archived`, `archived_at` is set.

### Bulk Actions
Mark multiple notifications as read, archive them, or mark all unread as read for a user — all in a single API call.

### Subscriber Hash
An HMAC-SHA256 hash that authenticates an SSE connection. The backend signs `userId` with the application's API key. The client passes this hash when connecting, and the SSE handler validates it to prevent impersonation. The SDK needs a method to obtain this hash, and the backend needs an endpoint to generate it.

### Complete SDK
An SDK that covers **every** API endpoint exposed by FreeRangeNotify. Not just `Send` and `CreateUser` — the full surface: notifications (send, list, get, mark-read, snooze, archive, cancel, retry, broadcast, bulk), users (CRUD, devices, preferences), templates (CRUD, versions, rollback, diff, render, test, library), workflows (CRUD, trigger, executions), topics (CRUD, subscribers), presence (check-in), and analytics (summary).

---

## Section 1: Codebase Audit

### 1.1 Go SDK (Current)

**File:** `sdk/go/freerangenotify/client.go` (208 lines, single file)

**What exists:**
| Method | API Endpoint |
|--------|-------------|
| `Send()` | `POST /v1/quick-send` |
| `Broadcast()` | `POST /v1/notifications/broadcast` |
| `CreateUser()` | `POST /v1/users/` |
| `UpdateUser()` | `PUT /v1/users/:id` |

**Coverage: 4 out of ~75 API endpoints (5%).**

**What's missing:** Everything else — notification CRUD, list, mark-read, unread count, bulk, templates, workflows, topics, presence, devices, preferences, analytics, subscriber-hash.

### 1.2 JS/TS SDK (Current)

**File:** `sdk/js/src/index.ts` (single file)

**What exists:**
| Method | API Endpoint |
|--------|-------------|
| `send()` | `POST /v1/quick-send` |
| `broadcast()` | `POST /v1/notifications/broadcast` |
| `createUser()` | `POST /v1/users/` |
| `updateUser()` | `PUT /v1/users/:id` |
| `listUsers()` | `GET /v1/users/` |
| `connectSSE()` | `GET /v1/sse` |

**Coverage: 6 out of ~75 API endpoints (8%).**

### 1.3 React SDK (Current)

**File:** `sdk/react/src/index.tsx` (single file)

Provides a single `<NotificationBell>` component with SSE connection and dropdown display. No headless hooks, no preferences, no inbox actions, no API data fetching.

### 1.4 Backend API Surface

The full API has **75+ endpoints** across these domains:

| Domain | Endpoints | Auth |
|--------|-----------|------|
| Auth | 7 (register, login, refresh, forgot/reset password, SSO) | Public |
| Health | 1 | Public |
| SSE | 2 (connect, admin activity feed) | Public/JWT |
| Users | 11 (CRUD, bulk, devices, preferences) | API Key |
| Presence | 1 | API Key |
| Quick-Send | 1 | API Key |
| Notifications | 12 (send, bulk, batch, broadcast, list, unread, mark-read, get, status, cancel, retry) | API Key |
| Templates | 12 (CRUD, library, clone, render, versions, rollback, diff, test) | API Key |
| Workflows | 9 (CRUD, trigger, executions, cancel) | API Key |
| Digest Rules | 5 (CRUD) | API Key |
| Topics | 8 (CRUD, subscribers) | API Key |
| Applications | 9 (CRUD, settings, regenerate-key, providers) | JWT |
| Admin | 7 (queues, DLQ, providers health, playground, analytics, audit) | JWT |
| Team | 4 (invite, list, update role, remove) | JWT |

### 1.5 Gaps & Debt

| Issue | Impact |
|-------|--------|
| Go SDK covers 5% of API | Developers must hand-roll HTTP calls for everything beyond send/broadcast/user-create |
| JS SDK covers 8% of API | Same — no template management, no workflow triggers, no inbox actions |
| React SDK has no headless hooks | Vue/Svelte/vanilla JS users get nothing; React users can't customize UI |
| No `Snooze` or `Archive` in backend | Can't implement inbox defer/dismiss in any SDK |
| No subscriber-hash endpoint | Frontend can't securely obtain HMAC for SSE without a backend call |
| `UpdatePreferencesRequest` DTO missing Phase 3 channels | Slack/Discord/WhatsApp preference toggles silently ignored |
| `DefaultPreferencesDTO` missing Phase 3 channels | App-level defaults can't set Slack/Discord/WhatsApp defaults |

---

## Section 2: Feature 5.1 — Backend: Snooze, Archive & Bulk Actions

### 2.1 New Notification Statuses

**File:** `internal/domain/notification/models.go`

```go
const (
    // Existing statuses...
    StatusSnoozed  Status = "snoozed"
    StatusArchived Status = "archived"
)
```

### 2.2 New Notification Fields

**File:** `internal/domain/notification/models.go`

Add to `Notification` struct:

```go
type Notification struct {
    // ... existing fields ...
    SnoozedUntil *time.Time `json:"snoozed_until,omitempty" es:"snoozed_until"`
    ArchivedAt   *time.Time `json:"archived_at,omitempty"   es:"archived_at"`
}
```

### 2.3 New Service Methods

**File:** `internal/domain/notification/models.go` — Service interface additions:

```go
type Service interface {
    // ... existing methods ...
    Snooze(ctx context.Context, notificationID, appID string, until time.Time) error
    Unsnooze(ctx context.Context, notificationID string) error
    Archive(ctx context.Context, notificationIDs []string, appID, userID string) error
    MarkAllRead(ctx context.Context, userID, appID, category string) error
    ListSnoozedDue(ctx context.Context) ([]*Notification, error)
}
```

**File:** `internal/domain/notification/models.go` — Repository interface additions:

```go
type Repository interface {
    // ... existing methods ...
    UpdateSnooze(ctx context.Context, notificationID string, status Status, snoozedUntil *time.Time) error
    BulkUpdateStatus(ctx context.Context, notificationIDs []string, status Status) error
    MarkAllRead(ctx context.Context, userID, appID, category string) (int, error)
    ListSnoozedDue(ctx context.Context, now time.Time) ([]*Notification, error)
    BulkArchive(ctx context.Context, notificationIDs []string, archivedAt time.Time) error
}
```

### 2.4 New API Endpoints

| Method | Path | Description | Request Body |
|--------|------|-------------|-------------|
| `POST` | `/v1/notifications/:id/snooze` | Snooze a notification | `{ "duration": "2h" }` or `{ "until": "2026-03-07T14:00:00Z" }` |
| `POST` | `/v1/notifications/:id/unsnooze` | Un-snooze immediately | — |
| `PATCH` | `/v1/notifications/bulk/archive` | Archive multiple notifications | `{ "notification_ids": [...], "user_id": "..." }` |
| `POST` | `/v1/notifications/read-all` | Mark all unread as read | `{ "user_id": "...", "category": "..." }` |

**DTOs:**

```go
// File: internal/interfaces/http/dto/notification_dto.go

type SnoozeRequest struct {
    Duration string     `json:"duration,omitempty"`  // e.g. "2h", "30m", "1d"
    Until    *time.Time `json:"until,omitempty"`
}

type BulkArchiveRequest struct {
    NotificationIDs []string `json:"notification_ids" validate:"required,min=1,max=100"`
    UserID          string   `json:"user_id"          validate:"required"`
}

type MarkAllReadRequest struct {
    UserID   string `json:"user_id"   validate:"required"`
    Category string `json:"category,omitempty"`
}
```

### 2.5 Snooze Worker (Un-snooze Loop)

**File:** `cmd/worker/main.go`

A separate goroutine runs every 30 seconds:

```go
func (p *Processor) runUnsnoozeLoop(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            due, err := p.notifRepo.ListSnoozedDue(ctx, time.Now())
            if err != nil {
                p.logger.Error("Failed to fetch snoozed notifications", zap.Error(err))
                continue
            }
            for _, notif := range due {
                // Update status back to pending
                if err := p.notifRepo.UpdateSnooze(ctx, notif.NotificationID, notification.StatusPending, nil); err != nil {
                    p.logger.Error("Failed to unsnooze notification", zap.String("id", notif.NotificationID), zap.Error(err))
                    continue
                }
                // Publish SSE event so browser clients see it resurface
                if p.sseBroadcaster != nil {
                    p.sseBroadcaster.Publish(notif.AppID, notif.UserID, notif)
                }
                p.logger.Info("Un-snoozed notification", zap.String("id", notif.NotificationID))
            }
        }
    }
}
```

**ES Query for `ListSnoozedDue`:**

```json
{
    "query": {
        "bool": {
            "filter": [
                { "term": { "status": "snoozed" } },
                { "range": { "snoozed_until": { "lte": "now" } } }
            ]
        }
    },
    "size": 100,
    "sort": [{ "snoozed_until": "asc" }]
}
```

### 2.6 Implementation

| Step | Action | File |
|------|--------|------|
| 1 | Add `StatusSnoozed`, `StatusArchived` constants | `internal/domain/notification/models.go` |
| 2 | Add `SnoozedUntil`, `ArchivedAt` fields to `Notification` | `internal/domain/notification/models.go` |
| 3 | Add `Snooze`, `Unsnooze`, `Archive`, `MarkAllRead`, `ListSnoozedDue` to `Service` interface | `internal/domain/notification/models.go` |
| 4 | Add `UpdateSnooze`, `BulkUpdateStatus`, `MarkAllRead`, `ListSnoozedDue`, `BulkArchive` to `Repository` interface | `internal/domain/notification/models.go` |
| 5 | Implement repository methods | `internal/infrastructure/database/notification_repository.go` |
| 6 | Implement service methods | `internal/usecases/notification_service.go` |
| 7 | Add `SnoozeRequest`, `BulkArchiveRequest`, `MarkAllReadRequest` DTOs | `internal/interfaces/http/dto/notification_dto.go` |
| 8 | Add `Snooze`, `Unsnooze`, `BulkArchive`, `MarkAllRead` handlers | `internal/interfaces/http/handlers/notification_handler.go` |
| 9 | Register new routes | `internal/interfaces/http/routes/routes.go` |
| 10 | Add un-snooze loop to worker | `cmd/worker/processor.go` or `cmd/worker/main.go` |

---

## Section 3: Feature 5.2 — Backend: Subscriber Hash Endpoint

### 3.1 Design

The `pkg/utils/hmac.go` already has `GenerateSubscriberHash(userID, apiKey string) string`. We need an endpoint that generates and returns this hash so frontend SDKs can securely connect to SSE.

**Endpoint:** `GET /v1/users/:id/subscriber-hash`

**Response:**
```json
{
    "user_id": "uuid",
    "subscriber_hash": "abc123..."
}
```

The handler resolves the user, takes the app's API key from context (set by the auth middleware), and calls `utils.GenerateSubscriberHash(userID, apiKey)`.

### 3.2 Implementation

| Step | Action | File |
|------|--------|------|
| 1 | Add `GetSubscriberHash` handler method | `internal/interfaces/http/handlers/user_handler.go` |
| 2 | Register route `GET /v1/users/:id/subscriber-hash` | `internal/interfaces/http/routes/routes.go` |

Handler pseudocode:

```go
func (h *UserHandler) GetSubscriberHash(c *fiber.Ctx) error {
    userID := c.Params("id")
    apiKey := c.Locals("api_key").(string) // Set by APIKeyAuth middleware
    hash := utils.GenerateSubscriberHash(userID, apiKey)
    return c.JSON(fiber.Map{
        "user_id":         userID,
        "subscriber_hash": hash,
    })
}
```

---

## Section 4: Feature 5.3 — Backend: DTO Alignment for Phase 3 Channels

### 4.1 Gaps

| DTO | Missing Fields |
|-----|---------------|
| `UpdatePreferencesRequest` | `SlackEnabled *bool`, `DiscordEnabled *bool`, `WhatsAppEnabled *bool`, `Throttle map[string]user.ThrottleConfig` |
| `DefaultPreferencesDTO` | `SlackEnabled *bool`, `DiscordEnabled *bool`, `WhatsAppEnabled *bool` |

### 4.2 Implementation

| Step | Action | File |
|------|--------|------|
| 1 | Add missing fields to `UpdatePreferencesRequest` | `internal/interfaces/http/dto/user_dto.go` |
| 2 | Add missing fields to `DefaultPreferencesDTO` | `internal/interfaces/http/dto/application_dto.go` |

---

## Section 5: Feature 5.4 — Go SDK: Complete End-to-End Client

### 5.1 Architecture

The SDK moves from a single-file flat structure to a multi-file, resource-oriented design:

```
sdk/go/freerangenotify/
├── client.go              # Core client: config, HTTP transport, auth, sub-clients
├── notifications.go       # NotificationsClient: send, list, get, mark-read, snooze, archive, etc.
├── users.go               # UsersClient: CRUD, devices, preferences, subscriber-hash
├── templates.go           # TemplatesClient: CRUD, versions, rollback, diff, render, test, library
├── workflows.go           # WorkflowsClient: CRUD, trigger, executions
├── topics.go              # TopicsClient: CRUD, subscribers
├── presence.go            # PresenceClient: check-in
├── types.go               # All shared types: Notification, User, Template, etc.
├── errors.go              # APIError, error handling
├── README.md              # Updated documentation
```

The client uses a **sub-client pattern** (like Stripe, Twilio, Novu SDKs):

```go
client := freerangenotify.New("frn_xxx", freerangenotify.WithBaseURL("http://localhost:8080/v1"))

// Sub-clients:
client.Notifications.Send(ctx, params)
client.Notifications.List(ctx, opts)
client.Notifications.MarkRead(ctx, ids)
client.Users.Create(ctx, params)
client.Users.GetPreferences(ctx, userID)
client.Templates.List(ctx, opts)
client.Workflows.Trigger(ctx, params)
client.Topics.AddSubscribers(ctx, topicID, userIDs)
client.Presence.CheckIn(ctx, params)
```

**Backward compatibility:** The existing `client.Send()` and `client.Broadcast()` top-level methods remain as convenience wrappers that delegate to `client.Notifications`.

### 5.2 Core Client & HTTP Transport

**File:** `sdk/go/freerangenotify/client.go` (rewrite)

```go
package freerangenotify

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "time"
)

// Client communicates with the FreeRangeNotify API.
type Client struct {
    apiKey  string
    baseURL string
    http    *http.Client

    // Sub-clients (resource-oriented)
    Notifications *NotificationsClient
    Users         *UsersClient
    Templates     *TemplatesClient
    Workflows     *WorkflowsClient
    Topics        *TopicsClient
    Presence      *PresenceClient
}

// Option configures the Client.
type Option func(*Client)

func WithBaseURL(url string) Option { return func(c *Client) { c.baseURL = url } }
func WithHTTPClient(hc *http.Client) Option { return func(c *Client) { c.http = hc } }
func WithTimeout(d time.Duration) Option {
    return func(c *Client) { c.http.Timeout = d }
}

// New creates a FreeRangeNotify client with the given API key.
func New(apiKey string, opts ...Option) *Client {
    c := &Client{
        apiKey:  apiKey,
        baseURL: "http://localhost:8080/v1",
        http:    &http.Client{Timeout: 30 * time.Second},
    }
    for _, opt := range opts {
        opt(c)
    }
    c.Notifications = &NotificationsClient{client: c}
    c.Users = &UsersClient{client: c}
    c.Templates = &TemplatesClient{client: c}
    c.Workflows = &WorkflowsClient{client: c}
    c.Topics = &TopicsClient{client: c}
    c.Presence = &PresenceClient{client: c}
    return c
}

// ── Backward-compatible convenience methods ──

// Send delivers a notification via Quick-Send (delegates to Notifications.QuickSend).
func (c *Client) Send(ctx context.Context, params SendParams) (*SendResult, error) {
    return c.Notifications.QuickSend(ctx, params)
}

// Broadcast sends a notification to all users (delegates to Notifications.Broadcast).
func (c *Client) Broadcast(ctx context.Context, params BroadcastParams) (*BroadcastResult, error) {
    return c.Notifications.Broadcast(ctx, params)
}

// CreateUser registers a new user (delegates to Users.Create).
func (c *Client) CreateUser(ctx context.Context, params CreateUserParams) (*User, error) {
    return c.Users.Create(ctx, params)
}

// UpdateUser updates a user (delegates to Users.Update).
func (c *Client) UpdateUser(ctx context.Context, userID string, params UpdateUserParams) (*User, error) {
    return c.Users.Update(ctx, userID, params)
}

// ── Internal HTTP Transport ──

func (c *Client) do(ctx context.Context, method, path string, payload interface{}, out interface{}) error {
    var body io.Reader
    if payload != nil && method != http.MethodGet && method != http.MethodDelete {
        data, err := json.Marshal(payload)
        if err != nil {
            return fmt.Errorf("freerangenotify: marshal request: %w", err)
        }
        body = bytes.NewReader(data)
    }

    req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
    if err != nil {
        return fmt.Errorf("freerangenotify: create request: %w", err)
    }
    req.Header.Set("Authorization", "Bearer "+c.apiKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.http.Do(req)
    if err != nil {
        return fmt.Errorf("freerangenotify: request failed: %w", err)
    }
    defer resp.Body.Close()

    respBody, _ := io.ReadAll(resp.Body)

    if resp.StatusCode >= 400 {
        return &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
    }

    if out != nil && len(respBody) > 0 {
        if err := json.Unmarshal(respBody, out); err != nil {
            return fmt.Errorf("freerangenotify: decode response: %w", err)
        }
    }
    return nil
}

// doWithQuery is like do but appends query parameters to the path.
func (c *Client) doWithQuery(ctx context.Context, method, path string, query url.Values, out interface{}) error {
    if len(query) > 0 {
        path = path + "?" + query.Encode()
    }
    return c.do(ctx, method, path, nil, out)
}
```

### 5.3 Notifications Sub-Client

**File:** `sdk/go/freerangenotify/notifications.go`

```go
package freerangenotify

import (
    "context"
    "fmt"
    "net/url"
    "strconv"
)

// NotificationsClient handles notification operations.
type NotificationsClient struct {
    client *Client
}

// ── Quick-Send ──

func (n *NotificationsClient) QuickSend(ctx context.Context, params SendParams) (*SendResult, error) {
    var result SendResult
    if err := n.client.do(ctx, "POST", "/quick-send", params, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// ── Standard Send ──

func (n *NotificationsClient) Send(ctx context.Context, params NotificationSendParams) (*NotificationResponse, error) {
    var result NotificationResponse
    if err := n.client.do(ctx, "POST", "/notifications/", params, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// ── Bulk Send ──

func (n *NotificationsClient) SendBulk(ctx context.Context, params BulkSendParams) (*BulkSendResult, error) {
    var result BulkSendResult
    if err := n.client.do(ctx, "POST", "/notifications/bulk", params, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// ── Batch Send ──

func (n *NotificationsClient) SendBatch(ctx context.Context, notifications []NotificationSendParams) (*BulkSendResult, error) {
    var result BulkSendResult
    payload := map[string]interface{}{"notifications": notifications}
    if err := n.client.do(ctx, "POST", "/notifications/batch", payload, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// ── Broadcast ──

func (n *NotificationsClient) Broadcast(ctx context.Context, params BroadcastParams) (*BroadcastResult, error) {
    var result BroadcastResult
    if err := n.client.do(ctx, "POST", "/notifications/broadcast", params, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// ── List ──

type ListNotificationsOptions struct {
    UserID     string
    AppID      string
    Channel    string
    Status     string
    Category   string
    Priority   string
    Page       int
    PageSize   int
    UnreadOnly bool
}

func (n *NotificationsClient) List(ctx context.Context, opts ListNotificationsOptions) (*NotificationListResponse, error) {
    q := url.Values{}
    if opts.UserID != "" { q.Set("user_id", opts.UserID) }
    if opts.AppID != "" { q.Set("app_id", opts.AppID) }
    if opts.Channel != "" { q.Set("channel", opts.Channel) }
    if opts.Status != "" { q.Set("status", opts.Status) }
    if opts.Category != "" { q.Set("category", opts.Category) }
    if opts.Priority != "" { q.Set("priority", opts.Priority) }
    if opts.Page > 0 { q.Set("page", strconv.Itoa(opts.Page)) }
    if opts.PageSize > 0 { q.Set("page_size", strconv.Itoa(opts.PageSize)) }
    if opts.UnreadOnly { q.Set("unread_only", "true") }

    var result NotificationListResponse
    if err := n.client.doWithQuery(ctx, "GET", "/notifications/", q, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// ── Get ──

func (n *NotificationsClient) Get(ctx context.Context, notificationID string) (*NotificationResponse, error) {
    var result NotificationResponse
    if err := n.client.do(ctx, "GET", "/notifications/"+notificationID, nil, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// ── Unread Count ──

func (n *NotificationsClient) GetUnreadCount(ctx context.Context, userID string) (int, error) {
    q := url.Values{}
    if userID != "" { q.Set("user_id", userID) }
    var result struct { Count int `json:"count"` }
    if err := n.client.doWithQuery(ctx, "GET", "/notifications/unread/count", q, &result); err != nil {
        return 0, err
    }
    return result.Count, nil
}

// ── List Unread ──

func (n *NotificationsClient) ListUnread(ctx context.Context, userID string, page, pageSize int) (*NotificationListResponse, error) {
    q := url.Values{}
    if userID != "" { q.Set("user_id", userID) }
    if page > 0 { q.Set("page", strconv.Itoa(page)) }
    if pageSize > 0 { q.Set("page_size", strconv.Itoa(pageSize)) }

    var result NotificationListResponse
    if err := n.client.doWithQuery(ctx, "GET", "/notifications/unread", q, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// ── Mark Read ──

func (n *NotificationsClient) MarkRead(ctx context.Context, userID string, notificationIDs []string) error {
    payload := map[string]interface{}{
        "user_id":          userID,
        "notification_ids": notificationIDs,
    }
    return n.client.do(ctx, "POST", "/notifications/read", payload, nil)
}

// ── Mark All Read ──

func (n *NotificationsClient) MarkAllRead(ctx context.Context, userID, category string) error {
    payload := map[string]interface{}{
        "user_id": userID,
    }
    if category != "" {
        payload["category"] = category
    }
    return n.client.do(ctx, "POST", "/notifications/read-all", payload, nil)
}

// ── Update Status ──

func (n *NotificationsClient) UpdateStatus(ctx context.Context, notificationID, status, errorMessage string) error {
    payload := map[string]interface{}{
        "status": status,
    }
    if errorMessage != "" {
        payload["error_message"] = errorMessage
    }
    return n.client.do(ctx, "PUT", "/notifications/"+notificationID+"/status", payload, nil)
}

// ── Cancel ──

func (n *NotificationsClient) Cancel(ctx context.Context, notificationID string) error {
    return n.client.do(ctx, "DELETE", "/notifications/"+notificationID, nil, nil)
}

// ── Cancel Batch ──

func (n *NotificationsClient) CancelBatch(ctx context.Context, notificationIDs []string) error {
    payload := map[string]interface{}{"notification_ids": notificationIDs}
    return n.client.do(ctx, "DELETE", "/notifications/batch", payload, nil)
}

// ── Retry ──

func (n *NotificationsClient) Retry(ctx context.Context, notificationID string) (*NotificationResponse, error) {
    var result NotificationResponse
    if err := n.client.do(ctx, "POST", "/notifications/"+notificationID+"/retry", nil, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// ── Snooze ──

func (n *NotificationsClient) Snooze(ctx context.Context, notificationID, duration string) error {
    payload := map[string]interface{}{"duration": duration}
    return n.client.do(ctx, "POST", "/notifications/"+notificationID+"/snooze", payload, nil)
}

// ── Unsnooze ──

func (n *NotificationsClient) Unsnooze(ctx context.Context, notificationID string) error {
    return n.client.do(ctx, "POST", "/notifications/"+notificationID+"/unsnooze", nil, nil)
}

// ── Archive ──

func (n *NotificationsClient) Archive(ctx context.Context, userID string, notificationIDs []string) error {
    payload := map[string]interface{}{
        "notification_ids": notificationIDs,
        "user_id":          userID,
    }
    return n.client.do(ctx, "PATCH", "/notifications/bulk/archive", payload, nil)
}
```

### 5.4 Users Sub-Client

**File:** `sdk/go/freerangenotify/users.go`

```go
package freerangenotify

import (
    "context"
    "fmt"
    "net/url"
    "strconv"
)

type UsersClient struct {
    client *Client
}

func (u *UsersClient) Create(ctx context.Context, params CreateUserParams) (*User, error) {
    var result User
    if err := u.client.do(ctx, "POST", "/users/", params, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

func (u *UsersClient) BulkCreate(ctx context.Context, users []CreateUserParams) (*BulkCreateUsersResult, error) {
    var result BulkCreateUsersResult
    payload := map[string]interface{}{"users": users}
    if err := u.client.do(ctx, "POST", "/users/bulk", payload, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

func (u *UsersClient) Get(ctx context.Context, userID string) (*User, error) {
    var result User
    if err := u.client.do(ctx, "GET", "/users/"+userID, nil, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

func (u *UsersClient) Update(ctx context.Context, userID string, params UpdateUserParams) (*User, error) {
    var result User
    if err := u.client.do(ctx, "PUT", "/users/"+userID, params, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

func (u *UsersClient) Delete(ctx context.Context, userID string) error {
    return u.client.do(ctx, "DELETE", "/users/"+userID, nil, nil)
}

func (u *UsersClient) List(ctx context.Context, page, pageSize int) (*UserListResponse, error) {
    q := url.Values{}
    if page > 0 { q.Set("page", strconv.Itoa(page)) }
    if pageSize > 0 { q.Set("page_size", strconv.Itoa(pageSize)) }

    var result UserListResponse
    if err := u.client.doWithQuery(ctx, "GET", "/users/", q, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// ── Devices ──

func (u *UsersClient) AddDevice(ctx context.Context, userID string, params AddDeviceParams) (*Device, error) {
    var result Device
    if err := u.client.do(ctx, "POST", fmt.Sprintf("/users/%s/devices", userID), params, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

func (u *UsersClient) GetDevices(ctx context.Context, userID string) ([]Device, error) {
    var result struct { Devices []Device `json:"devices"` }
    if err := u.client.do(ctx, "GET", fmt.Sprintf("/users/%s/devices", userID), nil, &result); err != nil {
        return nil, err
    }
    return result.Devices, nil
}

func (u *UsersClient) RemoveDevice(ctx context.Context, userID, deviceID string) error {
    return u.client.do(ctx, "DELETE", fmt.Sprintf("/users/%s/devices/%s", userID, deviceID), nil, nil)
}

// ── Preferences ──

func (u *UsersClient) GetPreferences(ctx context.Context, userID string) (*Preferences, error) {
    var result struct { Preferences *Preferences `json:"preferences"` }
    if err := u.client.do(ctx, "GET", fmt.Sprintf("/users/%s/preferences", userID), nil, &result); err != nil {
        return nil, err
    }
    return result.Preferences, nil
}

func (u *UsersClient) UpdatePreferences(ctx context.Context, userID string, prefs Preferences) (*Preferences, error) {
    var result struct { Preferences *Preferences `json:"preferences"` }
    if err := u.client.do(ctx, "PUT", fmt.Sprintf("/users/%s/preferences", userID), prefs, &result); err != nil {
        return nil, err
    }
    return result.Preferences, nil
}

// ── Subscriber Hash (HMAC for SSE) ──

func (u *UsersClient) GetSubscriberHash(ctx context.Context, userID string) (string, error) {
    var result struct { SubscriberHash string `json:"subscriber_hash"` }
    if err := u.client.do(ctx, "GET", fmt.Sprintf("/users/%s/subscriber-hash", userID), nil, &result); err != nil {
        return "", err
    }
    return result.SubscriberHash, nil
}
```

### 5.5 Templates Sub-Client

**File:** `sdk/go/freerangenotify/templates.go`

```go
package freerangenotify

import (
    "context"
    "fmt"
    "net/url"
    "strconv"
)

type TemplatesClient struct {
    client *Client
}

func (t *TemplatesClient) Create(ctx context.Context, params CreateTemplateParams) (*Template, error) {
    var result Template
    if err := t.client.do(ctx, "POST", "/templates/", params, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

func (t *TemplatesClient) Get(ctx context.Context, templateID string) (*Template, error) {
    var result Template
    if err := t.client.do(ctx, "GET", "/templates/"+templateID, nil, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

func (t *TemplatesClient) Update(ctx context.Context, templateID string, params UpdateTemplateParams) (*Template, error) {
    var result Template
    if err := t.client.do(ctx, "PUT", "/templates/"+templateID, params, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

func (t *TemplatesClient) Delete(ctx context.Context, templateID string) error {
    return t.client.do(ctx, "DELETE", "/templates/"+templateID, nil, nil)
}

type ListTemplatesOptions struct {
    AppID    string
    Channel  string
    Name     string
    Status   string
    Locale   string
    Limit    int
    Offset   int
}

func (t *TemplatesClient) List(ctx context.Context, opts ListTemplatesOptions) (*TemplateListResponse, error) {
    q := url.Values{}
    if opts.AppID != "" { q.Set("app_id", opts.AppID) }
    if opts.Channel != "" { q.Set("channel", opts.Channel) }
    if opts.Name != "" { q.Set("name", opts.Name) }
    if opts.Status != "" { q.Set("status", opts.Status) }
    if opts.Locale != "" { q.Set("locale", opts.Locale) }
    if opts.Limit > 0 { q.Set("limit", strconv.Itoa(opts.Limit)) }
    if opts.Offset > 0 { q.Set("offset", strconv.Itoa(opts.Offset)) }

    var result TemplateListResponse
    if err := t.client.doWithQuery(ctx, "GET", "/templates/", q, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// ── Library ──

func (t *TemplatesClient) GetLibrary(ctx context.Context, category string) ([]Template, error) {
    q := url.Values{}
    if category != "" { q.Set("category", category) }

    var result struct { Templates []Template `json:"templates"` }
    if err := t.client.doWithQuery(ctx, "GET", "/templates/library", q, &result); err != nil {
        return nil, err
    }
    return result.Templates, nil
}

func (t *TemplatesClient) CloneFromLibrary(ctx context.Context, name string, params CloneTemplateParams) (*Template, error) {
    var result Template
    if err := t.client.do(ctx, "POST", "/templates/library/"+name+"/clone", params, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// ── Versioning ──

func (t *TemplatesClient) GetVersions(ctx context.Context, appID, name string) ([]Template, error) {
    var result struct { Versions []Template `json:"versions"` }
    if err := t.client.do(ctx, "GET", fmt.Sprintf("/templates/%s/%s/versions", appID, name), nil, &result); err != nil {
        return nil, err
    }
    return result.Versions, nil
}

func (t *TemplatesClient) CreateVersion(ctx context.Context, appID, name string, params CreateVersionParams) (*Template, error) {
    var result Template
    if err := t.client.do(ctx, "POST", fmt.Sprintf("/templates/%s/%s/versions", appID, name), params, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// ── Rollback ──

func (t *TemplatesClient) Rollback(ctx context.Context, templateID string, version int, updatedBy string) (*Template, error) {
    var result Template
    payload := map[string]interface{}{"version": version, "updated_by": updatedBy}
    if err := t.client.do(ctx, "POST", "/templates/"+templateID+"/rollback", payload, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// ── Diff ──

func (t *TemplatesClient) Diff(ctx context.Context, templateID string, fromVersion, toVersion int) (*TemplateDiff, error) {
    q := url.Values{}
    q.Set("from", strconv.Itoa(fromVersion))
    q.Set("to", strconv.Itoa(toVersion))

    var result TemplateDiff
    if err := t.client.doWithQuery(ctx, "GET", "/templates/"+templateID+"/diff", q, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// ── Render ──

func (t *TemplatesClient) Render(ctx context.Context, templateID string, data map[string]interface{}) (string, error) {
    var result struct { RenderedBody string `json:"rendered_body"` }
    payload := map[string]interface{}{"data": data}
    if err := t.client.do(ctx, "POST", "/templates/"+templateID+"/render", payload, &result); err != nil {
        return "", err
    }
    return result.RenderedBody, nil
}

// ── Send Test ──

func (t *TemplatesClient) SendTest(ctx context.Context, templateID, toEmail string, sampleData map[string]interface{}) error {
    payload := map[string]interface{}{
        "to_email":    toEmail,
        "sample_data": sampleData,
    }
    return t.client.do(ctx, "POST", "/templates/"+templateID+"/test", payload, nil)
}
```

### 5.6 Workflows Sub-Client

**File:** `sdk/go/freerangenotify/workflows.go`

```go
package freerangenotify

import (
    "context"
    "net/url"
    "strconv"
)

type WorkflowsClient struct {
    client *Client
}

func (w *WorkflowsClient) Create(ctx context.Context, params CreateWorkflowParams) (*Workflow, error) {
    var result Workflow
    if err := w.client.do(ctx, "POST", "/workflows/", params, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

func (w *WorkflowsClient) Get(ctx context.Context, workflowID string) (*Workflow, error) {
    var result Workflow
    if err := w.client.do(ctx, "GET", "/workflows/"+workflowID, nil, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

func (w *WorkflowsClient) Update(ctx context.Context, workflowID string, params UpdateWorkflowParams) (*Workflow, error) {
    var result Workflow
    if err := w.client.do(ctx, "PUT", "/workflows/"+workflowID, params, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

func (w *WorkflowsClient) Delete(ctx context.Context, workflowID string) error {
    return w.client.do(ctx, "DELETE", "/workflows/"+workflowID, nil, nil)
}

func (w *WorkflowsClient) List(ctx context.Context, page, pageSize int) (*WorkflowListResponse, error) {
    q := url.Values{}
    if page > 0 { q.Set("page", strconv.Itoa(page)) }
    if pageSize > 0 { q.Set("page_size", strconv.Itoa(pageSize)) }

    var result WorkflowListResponse
    if err := w.client.doWithQuery(ctx, "GET", "/workflows/", q, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

func (w *WorkflowsClient) Trigger(ctx context.Context, params TriggerWorkflowParams) (*WorkflowExecution, error) {
    var result WorkflowExecution
    if err := w.client.do(ctx, "POST", "/workflows/trigger", params, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

func (w *WorkflowsClient) GetExecution(ctx context.Context, executionID string) (*WorkflowExecution, error) {
    var result WorkflowExecution
    if err := w.client.do(ctx, "GET", "/workflows/executions/"+executionID, nil, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

func (w *WorkflowsClient) ListExecutions(ctx context.Context, page, pageSize int) (*ExecutionListResponse, error) {
    q := url.Values{}
    if page > 0 { q.Set("page", strconv.Itoa(page)) }
    if pageSize > 0 { q.Set("page_size", strconv.Itoa(pageSize)) }

    var result ExecutionListResponse
    if err := w.client.doWithQuery(ctx, "GET", "/workflows/executions", q, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

func (w *WorkflowsClient) CancelExecution(ctx context.Context, executionID string) error {
    return w.client.do(ctx, "POST", "/workflows/executions/"+executionID+"/cancel", nil, nil)
}
```

### 5.7 Topics Sub-Client

**File:** `sdk/go/freerangenotify/topics.go`

```go
package freerangenotify

import (
    "context"
    "net/url"
    "strconv"
)

type TopicsClient struct {
    client *Client
}

func (t *TopicsClient) Create(ctx context.Context, params CreateTopicParams) (*Topic, error) {
    var result Topic
    if err := t.client.do(ctx, "POST", "/topics/", params, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

func (t *TopicsClient) Get(ctx context.Context, topicID string) (*Topic, error) {
    var result Topic
    if err := t.client.do(ctx, "GET", "/topics/"+topicID, nil, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

func (t *TopicsClient) GetByKey(ctx context.Context, key string) (*Topic, error) {
    var result Topic
    if err := t.client.do(ctx, "GET", "/topics/key/"+key, nil, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

func (t *TopicsClient) Delete(ctx context.Context, topicID string) error {
    return t.client.do(ctx, "DELETE", "/topics/"+topicID, nil, nil)
}

func (t *TopicsClient) List(ctx context.Context, page, pageSize int) (*TopicListResponse, error) {
    q := url.Values{}
    if page > 0 { q.Set("page", strconv.Itoa(page)) }
    if pageSize > 0 { q.Set("page_size", strconv.Itoa(pageSize)) }

    var result TopicListResponse
    if err := t.client.doWithQuery(ctx, "GET", "/topics/", q, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

func (t *TopicsClient) AddSubscribers(ctx context.Context, topicID string, userIDs []string) error {
    payload := map[string]interface{}{"user_ids": userIDs}
    return t.client.do(ctx, "POST", "/topics/"+topicID+"/subscribers", payload, nil)
}

func (t *TopicsClient) RemoveSubscribers(ctx context.Context, topicID string, userIDs []string) error {
    payload := map[string]interface{}{"user_ids": userIDs}
    return t.client.do(ctx, "DELETE", "/topics/"+topicID+"/subscribers", payload, nil)
}

func (t *TopicsClient) GetSubscribers(ctx context.Context, topicID string, page, pageSize int) (*SubscriberListResponse, error) {
    q := url.Values{}
    if page > 0 { q.Set("page", strconv.Itoa(page)) }
    if pageSize > 0 { q.Set("page_size", strconv.Itoa(pageSize)) }

    var result SubscriberListResponse
    if err := t.client.doWithQuery(ctx, "GET", "/topics/"+topicID+"/subscribers", q, &result); err != nil {
        return nil, err
    }
    return &result, nil
}
```

### 5.8 Presence Sub-Client

**File:** `sdk/go/freerangenotify/presence.go`

```go
package freerangenotify

import "context"

type PresenceClient struct {
    client *Client
}

type CheckInParams struct {
    UserID     string `json:"user_id"`
    WebhookURL string `json:"webhook_url,omitempty"`
}

func (p *PresenceClient) CheckIn(ctx context.Context, params CheckInParams) error {
    return p.client.do(ctx, "POST", "/presence/check-in", params, nil)
}
```

### 5.9 Types & Models

**File:** `sdk/go/freerangenotify/types.go`

```go
package freerangenotify

import "time"

// ── Notification Types ──

type NotificationSendParams struct {
    UserID        string                 `json:"user_id"`
    Channel       string                 `json:"channel,omitempty"`
    Priority      string                 `json:"priority,omitempty"`
    Title         string                 `json:"title,omitempty"`
    Body          string                 `json:"body,omitempty"`
    Data          map[string]interface{} `json:"data,omitempty"`
    TemplateID    string                 `json:"template_id,omitempty"`
    Category      string                 `json:"category,omitempty"`
    ScheduledAt   *time.Time             `json:"scheduled_at,omitempty"`
    WebhookURL    string                 `json:"webhook_url,omitempty"`
    WebhookTarget string                 `json:"webhook_target,omitempty"`
}

type SendParams struct {
    To          string                 `json:"to"`
    Template    string                 `json:"template,omitempty"`
    Subject     string                 `json:"subject,omitempty"`
    Body        string                 `json:"body,omitempty"`
    Data        map[string]interface{} `json:"data,omitempty"`
    Channel     string                 `json:"channel,omitempty"`
    Priority    string                 `json:"priority,omitempty"`
    ScheduledAt *time.Time             `json:"scheduled_at,omitempty"`
}

type SendResult struct {
    NotificationID string `json:"notification_id"`
    Status         string `json:"status"`
    UserID         string `json:"user_id"`
    Channel        string `json:"channel"`
}

type BulkSendParams struct {
    UserIDs    []string               `json:"user_ids"`
    Channel    string                 `json:"channel,omitempty"`
    Priority   string                 `json:"priority,omitempty"`
    Title      string                 `json:"title,omitempty"`
    Body       string                 `json:"body,omitempty"`
    Data       map[string]interface{} `json:"data,omitempty"`
    TemplateID string                 `json:"template_id,omitempty"`
    Category   string                 `json:"category,omitempty"`
}

type BulkSendResult struct {
    Sent    int          `json:"sent"`
    Total   int          `json:"total"`
    Items   []SendResult `json:"items"`
}

type BroadcastParams struct {
    Template string                 `json:"template_id"`
    Data     map[string]interface{} `json:"data,omitempty"`
    Channel  string                 `json:"channel,omitempty"`
    Priority string                 `json:"priority,omitempty"`
}

type BroadcastResult struct {
    TotalSent     int          `json:"total_sent"`
    Notifications []SendResult `json:"notifications"`
}

type NotificationResponse struct {
    NotificationID string                 `json:"notification_id"`
    AppID          string                 `json:"app_id"`
    UserID         string                 `json:"user_id"`
    Channel        string                 `json:"channel"`
    Priority       string                 `json:"priority"`
    Status         string                 `json:"status"`
    Content        *NotificationContent   `json:"content"`
    TemplateID     string                 `json:"template_id,omitempty"`
    Category       string                 `json:"category,omitempty"`
    ScheduledAt    *time.Time             `json:"scheduled_at,omitempty"`
    SentAt         *time.Time             `json:"sent_at,omitempty"`
    DeliveredAt    *time.Time             `json:"delivered_at,omitempty"`
    ReadAt         *time.Time             `json:"read_at,omitempty"`
    FailedAt       *time.Time             `json:"failed_at,omitempty"`
    SnoozedUntil   *time.Time             `json:"snoozed_until,omitempty"`
    ArchivedAt     *time.Time             `json:"archived_at,omitempty"`
    ErrorMessage   string                 `json:"error_message,omitempty"`
    RetryCount     int                    `json:"retry_count"`
    CreatedAt      time.Time              `json:"created_at"`
    UpdatedAt      time.Time              `json:"updated_at"`
}

type NotificationContent struct {
    Title string                 `json:"title"`
    Body  string                 `json:"body"`
    Data  map[string]interface{} `json:"data,omitempty"`
}

type NotificationListResponse struct {
    Notifications []NotificationResponse `json:"notifications"`
    Total         int                    `json:"total"`
    Page          int                    `json:"page"`
    PageSize      int                    `json:"page_size"`
}

// ── User Types ──

type CreateUserParams struct {
    Email      string       `json:"email,omitempty"`
    Phone      string       `json:"phone,omitempty"`
    Timezone   string       `json:"timezone,omitempty"`
    Language   string       `json:"language,omitempty"`
    ExternalID string       `json:"external_id,omitempty"`
    WebhookURL string       `json:"webhook_url,omitempty"`
    Preferences *Preferences `json:"preferences,omitempty"`
}

type UpdateUserParams struct {
    ExternalID string       `json:"external_id,omitempty"`
    Email      string       `json:"email,omitempty"`
    Phone      string       `json:"phone,omitempty"`
    Timezone   string       `json:"timezone,omitempty"`
    Language   string       `json:"language,omitempty"`
    WebhookURL string       `json:"webhook_url,omitempty"`
    Preferences *Preferences `json:"preferences,omitempty"`
}

type User struct {
    UserID     string       `json:"user_id"`
    AppID      string       `json:"app_id"`
    ExternalID string       `json:"external_id"`
    Email      string       `json:"email"`
    Phone      string       `json:"phone"`
    Timezone   string       `json:"timezone"`
    Language   string       `json:"language"`
    WebhookURL string       `json:"webhook_url"`
    Preferences *Preferences `json:"preferences,omitempty"`
    Devices    []Device     `json:"devices,omitempty"`
    CreatedAt  string       `json:"created_at"`
    UpdatedAt  string       `json:"updated_at"`
}

type UserListResponse struct {
    Users      []User `json:"users"`
    TotalCount int    `json:"total_count"`
    Page       int    `json:"page"`
    PageSize   int    `json:"page_size"`
}

type BulkCreateUsersResult struct {
    Created int      `json:"created"`
    Total   int      `json:"total"`
    Errors  []string `json:"errors,omitempty"`
}

type Preferences struct {
    EmailEnabled    *bool                        `json:"email_enabled,omitempty"`
    PushEnabled     *bool                        `json:"push_enabled,omitempty"`
    SMSEnabled      *bool                        `json:"sms_enabled,omitempty"`
    SlackEnabled    *bool                        `json:"slack_enabled,omitempty"`
    DiscordEnabled  *bool                        `json:"discord_enabled,omitempty"`
    WhatsAppEnabled *bool                        `json:"whatsapp_enabled,omitempty"`
    QuietHours      *QuietHours                  `json:"quiet_hours,omitempty"`
    DND             bool                         `json:"dnd,omitempty"`
    Categories      map[string]CategoryPreference `json:"categories,omitempty"`
    DailyLimit      int                          `json:"daily_limit,omitempty"`
}

type QuietHours struct {
    Start string `json:"start"` // HH:MM
    End   string `json:"end"`   // HH:MM
}

type CategoryPreference struct {
    Enabled         bool     `json:"enabled"`
    EnabledChannels []string `json:"enabled_channels,omitempty"`
}

type AddDeviceParams struct {
    Platform string `json:"platform"`
    Token    string `json:"token"`
}

type Device struct {
    DeviceID     string `json:"device_id"`
    Platform     string `json:"platform"`
    Active       bool   `json:"active"`
    RegisteredAt string `json:"registered_at"`
}

// ── Template Types ──

type CreateTemplateParams struct {
    AppID         string                 `json:"app_id"`
    Name          string                 `json:"name"`
    Description   string                 `json:"description,omitempty"`
    Channel       string                 `json:"channel"`
    WebhookTarget string                 `json:"webhook_target,omitempty"`
    Subject       string                 `json:"subject,omitempty"`
    Body          string                 `json:"body"`
    Variables     []string               `json:"variables,omitempty"`
    Metadata      map[string]interface{} `json:"metadata,omitempty"`
    Locale        string                 `json:"locale,omitempty"`
    CreatedBy     string                 `json:"created_by,omitempty"`
}

type UpdateTemplateParams struct {
    Description   string                 `json:"description,omitempty"`
    WebhookTarget string                 `json:"webhook_target,omitempty"`
    Subject       string                 `json:"subject,omitempty"`
    Body          string                 `json:"body,omitempty"`
    Variables     []string               `json:"variables,omitempty"`
    Metadata      map[string]interface{} `json:"metadata,omitempty"`
    Status        string                 `json:"status,omitempty"`
    Locale        string                 `json:"locale,omitempty"`
    UpdatedBy     string                 `json:"updated_by,omitempty"`
}

type CreateVersionParams struct {
    Description string                 `json:"description,omitempty"`
    Subject     string                 `json:"subject,omitempty"`
    Body        string                 `json:"body,omitempty"`
    Variables   []string               `json:"variables,omitempty"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
    Locale      string                 `json:"locale,omitempty"`
    CreatedBy   string                 `json:"created_by,omitempty"`
}

type CloneTemplateParams struct {
    AppID string `json:"app_id"`
}

type Template struct {
    ID            string                 `json:"id"`
    AppID         string                 `json:"app_id"`
    Name          string                 `json:"name"`
    Description   string                 `json:"description"`
    Channel       string                 `json:"channel"`
    WebhookTarget string                 `json:"webhook_target,omitempty"`
    Subject       string                 `json:"subject"`
    Body          string                 `json:"body"`
    Variables     []string               `json:"variables"`
    Metadata      map[string]interface{} `json:"metadata,omitempty"`
    Version       int                    `json:"version"`
    Status        string                 `json:"status"`
    Locale        string                 `json:"locale"`
    CreatedBy     string                 `json:"created_by"`
    UpdatedBy     string                 `json:"updated_by"`
    CreatedAt     string                 `json:"created_at"`
    UpdatedAt     string                 `json:"updated_at"`
}

type TemplateListResponse struct {
    Templates []Template `json:"templates"`
    Total     int        `json:"total"`
    Limit     int        `json:"limit"`
    Offset    int        `json:"offset"`
}

type TemplateDiff struct {
    FromVersion int           `json:"from_version"`
    ToVersion   int           `json:"to_version"`
    Changes     []FieldChange `json:"changes"`
}

type FieldChange struct {
    Field string      `json:"field"`
    From  interface{} `json:"from"`
    To    interface{} `json:"to"`
}

// ── Workflow Types ──

type CreateWorkflowParams struct {
    Name        string         `json:"name"`
    Description string         `json:"description,omitempty"`
    TriggerID   string         `json:"trigger_id"`
    Steps       []WorkflowStep `json:"steps"`
}

type UpdateWorkflowParams struct {
    Name        string         `json:"name,omitempty"`
    Description string         `json:"description,omitempty"`
    Steps       []WorkflowStep `json:"steps,omitempty"`
    Status      string         `json:"status,omitempty"`
}

type TriggerWorkflowParams struct {
    TriggerID string                 `json:"trigger_id"`
    UserID    string                 `json:"user_id"`
    Payload   map[string]interface{} `json:"payload,omitempty"`
}

type Workflow struct {
    ID          string         `json:"id"`
    AppID       string         `json:"app_id"`
    Name        string         `json:"name"`
    Description string         `json:"description"`
    TriggerID   string         `json:"trigger_id"`
    Steps       []WorkflowStep `json:"steps"`
    Status      string         `json:"status"`
    Version     int            `json:"version"`
    CreatedBy   string         `json:"created_by"`
    CreatedAt   string         `json:"created_at"`
    UpdatedAt   string         `json:"updated_at"`
}

type WorkflowStep struct {
    ID            string                 `json:"id"`
    Type          string                 `json:"type"` // "channel", "delay", "digest", "condition"
    Name          string                 `json:"name,omitempty"`
    Channel       string                 `json:"channel,omitempty"`
    TemplateID    string                 `json:"template_id,omitempty"`
    DelayDuration string                 `json:"delay_duration,omitempty"`
    DigestKey     string                 `json:"digest_key,omitempty"`
    DigestWindow  string                 `json:"digest_window,omitempty"`
    Condition     *StepCondition         `json:"condition,omitempty"`
    Config        map[string]interface{} `json:"config,omitempty"`
}

type StepCondition struct {
    Field    string      `json:"field"`
    Operator string      `json:"operator"`
    Value    interface{} `json:"value"`
    OnTrue   string      `json:"on_true,omitempty"`
    OnFalse  string      `json:"on_false,omitempty"`
}

type WorkflowExecution struct {
    ID            string                 `json:"id"`
    WorkflowID    string                 `json:"workflow_id"`
    AppID         string                 `json:"app_id"`
    UserID        string                 `json:"user_id"`
    TransactionID string                 `json:"transaction_id"`
    CurrentStepID string                 `json:"current_step_id"`
    Status        string                 `json:"status"`
    Payload       map[string]interface{} `json:"payload,omitempty"`
    StartedAt     string                 `json:"started_at"`
    CompletedAt   *string                `json:"completed_at,omitempty"`
    UpdatedAt     string                 `json:"updated_at"`
}

type WorkflowListResponse struct {
    Workflows []Workflow `json:"workflows"`
    Total     int        `json:"total"`
    Page      int        `json:"page"`
    PageSize  int        `json:"page_size"`
}

type ExecutionListResponse struct {
    Executions []WorkflowExecution `json:"executions"`
    Total      int                 `json:"total"`
    Page       int                 `json:"page"`
    PageSize   int                 `json:"page_size"`
}

// ── Topic Types ──

type CreateTopicParams struct {
    Name        string `json:"name"`
    Key         string `json:"key"`
    Description string `json:"description,omitempty"`
}

type Topic struct {
    ID          string `json:"id"`
    AppID       string `json:"app_id"`
    Name        string `json:"name"`
    Key         string `json:"key"`
    Description string `json:"description"`
    CreatedAt   string `json:"created_at"`
    UpdatedAt   string `json:"updated_at"`
}

type TopicListResponse struct {
    Topics []Topic `json:"topics"`
    Total  int     `json:"total"`
    Page   int     `json:"page"`
    PageSize int   `json:"page_size"`
}

type TopicSubscription struct {
    ID        string `json:"id"`
    TopicID   string `json:"topic_id"`
    UserID    string `json:"user_id"`
    CreatedAt string `json:"created_at"`
}

type SubscriberListResponse struct {
    Subscribers []TopicSubscription `json:"subscribers"`
    Total       int                 `json:"total"`
    Page        int                 `json:"page"`
    PageSize    int                 `json:"page_size"`
}
```

### 5.10 Error Handling

**File:** `sdk/go/freerangenotify/errors.go`

```go
package freerangenotify

import "fmt"

// APIError represents an error response from the FreeRangeNotify API.
type APIError struct {
    StatusCode int
    Body       string
}

func (e *APIError) Error() string {
    return fmt.Sprintf("freerangenotify: API error %d: %s", e.StatusCode, e.Body)
}

// IsNotFound returns true if the error is a 404.
func (e *APIError) IsNotFound() bool { return e.StatusCode == 404 }

// IsUnauthorized returns true if the error is a 401.
func (e *APIError) IsUnauthorized() bool { return e.StatusCode == 401 }

// IsRateLimited returns true if the error is a 429.
func (e *APIError) IsRateLimited() bool { return e.StatusCode == 429 }

// IsValidationError returns true if the error is a 400/422.
func (e *APIError) IsValidationError() bool {
    return e.StatusCode == 400 || e.StatusCode == 422
}
```

### 5.11 File Inventory

| Action | File | Description |
|--------|------|-------------|
| **REWRITE** | `sdk/go/freerangenotify/client.go` | Core client with sub-client pattern + backward-compat wrappers |
| **CREATE** | `sdk/go/freerangenotify/notifications.go` | Notifications sub-client (16 methods) |
| **CREATE** | `sdk/go/freerangenotify/users.go` | Users sub-client (10 methods) |
| **CREATE** | `sdk/go/freerangenotify/templates.go` | Templates sub-client (11 methods) |
| **CREATE** | `sdk/go/freerangenotify/workflows.go` | Workflows sub-client (9 methods) |
| **CREATE** | `sdk/go/freerangenotify/topics.go` | Topics sub-client (8 methods) |
| **CREATE** | `sdk/go/freerangenotify/presence.go` | Presence sub-client (1 method) |
| **CREATE** | `sdk/go/freerangenotify/types.go` | All shared types and models |
| **CREATE** | `sdk/go/freerangenotify/errors.go` | Error types with helpers |
| **REWRITE** | `sdk/go/freerangenotify/README.md` | Updated documentation |

**Method Coverage:**

| Sub-Client | Methods | API Endpoints Covered |
|------------|---------|----------------------|
| `Notifications` | 16 | QuickSend, Send, SendBulk, SendBatch, Broadcast, List, Get, UnreadCount, ListUnread, MarkRead, MarkAllRead, UpdateStatus, Cancel, CancelBatch, Retry, Snooze, Unsnooze, Archive |
| `Users` | 10 | Create, BulkCreate, Get, Update, Delete, List, AddDevice, GetDevices, RemoveDevice, GetPreferences, UpdatePreferences, GetSubscriberHash |
| `Templates` | 11 | Create, Get, Update, Delete, List, GetLibrary, CloneFromLibrary, GetVersions, CreateVersion, Rollback, Diff, Render, SendTest |
| `Workflows` | 9 | Create, Get, Update, Delete, List, Trigger, GetExecution, ListExecutions, CancelExecution |
| `Topics` | 8 | Create, Get, GetByKey, Delete, List, AddSubscribers, RemoveSubscribers, GetSubscribers |
| `Presence` | 1 | CheckIn |
| **Total** | **55** | **~73% of all API endpoints** (admin/JWT-only endpoints excluded — those are dashboard-only) |

---

## Section 6: Feature 5.5 — JS/TS SDK: Complete End-to-End Client

### 6.1 Architecture

Same sub-client pattern as Go, adapted for TypeScript idioms:

```
sdk/js/src/
├── index.ts               # Re-exports + FreeRangeNotify class (convenience wrapper)
├── client.ts              # Core HTTP transport
├── notifications.ts       # NotificationsClient
├── users.ts               # UsersClient
├── templates.ts           # TemplatesClient
├── workflows.ts           # WorkflowsClient
├── topics.ts              # TopicsClient
├── presence.ts            # PresenceClient
├── sse.ts                 # SSE connection (extracted from index.ts)
├── types.ts               # All TypeScript interfaces
├── errors.ts              # FreeRangeNotifyError class
```

```typescript
import { FreeRangeNotify } from '@freerangenotify/sdk';

const client = new FreeRangeNotify('frn_xxx', { baseURL: 'http://localhost:8080/v1' });

// Sub-clients:
await client.notifications.send({ userID: '...', channel: 'email', templateID: 'welcome_email' });
await client.notifications.list({ userID: '...', unreadOnly: true });
await client.notifications.markRead('user-id', ['notif-1', 'notif-2']);
await client.notifications.snooze('notif-id', '2h');
await client.users.create({ email: 'alice@example.com' });
await client.users.getPreferences('user-id');
await client.templates.list({ channel: 'email' });
await client.workflows.trigger({ triggerID: 'welcome', userID: '...' });
await client.topics.addSubscribers('topic-id', ['user-1', 'user-2']);

// Backward-compat convenience methods:
await client.send({ to: 'user@example.com', template: 'welcome_email' });
await client.broadcast({ template: 'alert', data: { msg: 'hi' } });
```

### 6.2 Core Client & HTTP Transport

**File:** `sdk/js/src/client.ts`

```typescript
export class HttpClient {
    constructor(private readonly apiKey: string, private readonly baseURL: string) {}

    async request<T>(method: string, path: string, body?: unknown, query?: Record<string, string>): Promise<T> {
        let url = this.baseURL + path;
        if (query) {
            const params = new URLSearchParams(
                Object.fromEntries(Object.entries(query).filter(([, v]) => v !== undefined && v !== ''))
            );
            if (params.toString()) url += '?' + params.toString();
        }
        const headers: Record<string, string> = {
            'Authorization': `Bearer ${this.apiKey}`,
            'Content-Type': 'application/json',
        };
        const init: RequestInit = { method, headers };
        if (body && method !== 'GET') init.body = JSON.stringify(body);

        const res = await fetch(url, init);
        if (!res.ok) {
            const text = await res.text().catch(() => '');
            throw new FreeRangeNotifyError(res.status, text);
        }
        return res.json() as Promise<T>;
    }
}
```

### 6.3 Notifications Module

**File:** `sdk/js/src/notifications.ts`

Complete API coverage matching the Go SDK:

```typescript
export class NotificationsClient {
    constructor(private http: HttpClient) {}

    async quickSend(params: QuickSendParams): Promise<SendResult> { ... }
    async send(params: NotificationSendParams): Promise<NotificationResponse> { ... }
    async sendBulk(params: BulkSendParams): Promise<BulkSendResult> { ... }
    async sendBatch(notifications: NotificationSendParams[]): Promise<BulkSendResult> { ... }
    async broadcast(params: BroadcastParams): Promise<BroadcastResult> { ... }
    async list(opts?: ListNotificationsOptions): Promise<NotificationListResponse> { ... }
    async get(notificationId: string): Promise<NotificationResponse> { ... }
    async getUnreadCount(userId: string): Promise<number> { ... }
    async listUnread(userId: string, page?: number, pageSize?: number): Promise<NotificationListResponse> { ... }
    async markRead(userId: string, notificationIds: string[]): Promise<void> { ... }
    async markAllRead(userId: string, category?: string): Promise<void> { ... }
    async updateStatus(notificationId: string, status: string, errorMessage?: string): Promise<void> { ... }
    async cancel(notificationId: string): Promise<void> { ... }
    async cancelBatch(notificationIds: string[]): Promise<void> { ... }
    async retry(notificationId: string): Promise<NotificationResponse> { ... }
    async snooze(notificationId: string, duration: string): Promise<void> { ... }
    async unsnooze(notificationId: string): Promise<void> { ... }
    async archive(userId: string, notificationIds: string[]): Promise<void> { ... }
}
```

### 6.4 Users Module

**File:** `sdk/js/src/users.ts`

```typescript
export class UsersClient {
    constructor(private http: HttpClient) {}

    async create(params: CreateUserParams): Promise<User> { ... }
    async bulkCreate(users: CreateUserParams[]): Promise<BulkCreateUsersResult> { ... }
    async get(userId: string): Promise<User> { ... }
    async update(userId: string, params: UpdateUserParams): Promise<User> { ... }
    async delete(userId: string): Promise<void> { ... }
    async list(page?: number, pageSize?: number): Promise<UserListResponse> { ... }
    async addDevice(userId: string, params: AddDeviceParams): Promise<Device> { ... }
    async getDevices(userId: string): Promise<Device[]> { ... }
    async removeDevice(userId: string, deviceId: string): Promise<void> { ... }
    async getPreferences(userId: string): Promise<Preferences> { ... }
    async updatePreferences(userId: string, prefs: Partial<Preferences>): Promise<Preferences> { ... }
    async getSubscriberHash(userId: string): Promise<string> { ... }
}
```

### 6.5 Templates Module

**File:** `sdk/js/src/templates.ts`

```typescript
export class TemplatesClient {
    constructor(private http: HttpClient) {}

    async create(params: CreateTemplateParams): Promise<Template> { ... }
    async get(templateId: string): Promise<Template> { ... }
    async update(templateId: string, params: UpdateTemplateParams): Promise<Template> { ... }
    async delete(templateId: string): Promise<void> { ... }
    async list(opts?: ListTemplatesOptions): Promise<TemplateListResponse> { ... }
    async getLibrary(category?: string): Promise<Template[]> { ... }
    async cloneFromLibrary(name: string, appId: string): Promise<Template> { ... }
    async getVersions(appId: string, name: string): Promise<Template[]> { ... }
    async createVersion(appId: string, name: string, params: CreateVersionParams): Promise<Template> { ... }
    async rollback(templateId: string, version: number, updatedBy: string): Promise<Template> { ... }
    async diff(templateId: string, fromVersion: number, toVersion: number): Promise<TemplateDiff> { ... }
    async render(templateId: string, data: Record<string, unknown>): Promise<string> { ... }
    async sendTest(templateId: string, toEmail: string, sampleData?: Record<string, unknown>): Promise<void> { ... }
}
```

### 6.6 Workflows Module

**File:** `sdk/js/src/workflows.ts`

```typescript
export class WorkflowsClient {
    constructor(private http: HttpClient) {}

    async create(params: CreateWorkflowParams): Promise<Workflow> { ... }
    async get(workflowId: string): Promise<Workflow> { ... }
    async update(workflowId: string, params: UpdateWorkflowParams): Promise<Workflow> { ... }
    async delete(workflowId: string): Promise<void> { ... }
    async list(page?: number, pageSize?: number): Promise<WorkflowListResponse> { ... }
    async trigger(params: TriggerWorkflowParams): Promise<WorkflowExecution> { ... }
    async getExecution(executionId: string): Promise<WorkflowExecution> { ... }
    async listExecutions(page?: number, pageSize?: number): Promise<ExecutionListResponse> { ... }
    async cancelExecution(executionId: string): Promise<void> { ... }
}
```

### 6.7 Topics Module

**File:** `sdk/js/src/topics.ts`

```typescript
export class TopicsClient {
    constructor(private http: HttpClient) {}

    async create(params: CreateTopicParams): Promise<Topic> { ... }
    async get(topicId: string): Promise<Topic> { ... }
    async getByKey(key: string): Promise<Topic> { ... }
    async delete(topicId: string): Promise<void> { ... }
    async list(page?: number, pageSize?: number): Promise<TopicListResponse> { ... }
    async addSubscribers(topicId: string, userIds: string[]): Promise<void> { ... }
    async removeSubscribers(topicId: string, userIds: string[]): Promise<void> { ... }
    async getSubscribers(topicId: string, page?: number, pageSize?: number): Promise<SubscriberListResponse> { ... }
}
```

### 6.8 SSE Connection

**File:** `sdk/js/src/sse.ts`

Extracted from current `index.ts`, with additional event types:

```typescript
export interface SSEConnectionOptions {
    onNotification: (notification: SSENotification) => void;
    onConnected?: () => void;
    onError?: (event: Event) => void;
    onUnreadCountChange?: (count: number) => void;
    onConnectionChange?: (connected: boolean) => void;
    subscriberHash?: string;
    autoReconnect?: boolean;
    reconnectInterval?: number; // ms, default 3000
}

export function connectSSE(baseURL: string, apiKey: string, userId: string, options: SSEConnectionOptions): SSEConnection { ... }
```

### 6.9 Types

**File:** `sdk/js/src/types.ts`

All TypeScript interfaces mirroring the Go SDK types — `NotificationSendParams`, `NotificationResponse`, `User`, `Template`, `Workflow`, `Topic`, `Preferences`, etc.

### 6.10 File Inventory

| Action | File | Description |
|--------|------|-------------|
| **REWRITE** | `sdk/js/src/index.ts` | Main class with sub-clients + backward-compat wrappers |
| **CREATE** | `sdk/js/src/client.ts` | Core HTTP transport |
| **CREATE** | `sdk/js/src/notifications.ts` | Notifications client (18 methods) |
| **CREATE** | `sdk/js/src/users.ts` | Users client (12 methods) |
| **CREATE** | `sdk/js/src/templates.ts` | Templates client (13 methods) |
| **CREATE** | `sdk/js/src/workflows.ts` | Workflows client (9 methods) |
| **CREATE** | `sdk/js/src/topics.ts` | Topics client (8 methods) |
| **CREATE** | `sdk/js/src/presence.ts` | Presence client (1 method) |
| **CREATE** | `sdk/js/src/sse.ts` | SSE connection (extracted) |
| **CREATE** | `sdk/js/src/types.ts` | All TypeScript interfaces |
| **CREATE** | `sdk/js/src/errors.ts` | Error class |
| **REWRITE** | `sdk/js/README.md` | Updated documentation |

---

## Section 7: Feature 5.6 — React SDK: Component Library

### 7.1 Provider & Context

**File:** `sdk/react/src/FreeRangeProvider.tsx`

A React context provider that initializes the JS SDK client and shares it across all components:

```tsx
import React, { createContext, useContext, useMemo } from 'react';
import { FreeRangeNotify } from '@freerangenotify/sdk';

interface FreeRangeContextValue {
    client: FreeRangeNotify;
    userId: string;
    subscriberHash?: string;
}

const FreeRangeContext = createContext<FreeRangeContextValue | null>(null);

export interface FreeRangeProviderProps {
    apiKey: string;
    userId: string;
    apiBaseURL?: string;
    subscriberHash?: string;
    children: React.ReactNode;
}

export function FreeRangeProvider({ apiKey, userId, apiBaseURL, subscriberHash, children }: FreeRangeProviderProps) {
    const client = useMemo(
        () => new FreeRangeNotify(apiKey, { baseURL: apiBaseURL }),
        [apiKey, apiBaseURL],
    );

    return (
        <FreeRangeContext.Provider value={{ client, userId, subscriberHash }}>
            {children}
        </FreeRangeContext.Provider>
    );
}

export function useFreeRange(): FreeRangeContextValue {
    const ctx = useContext(FreeRangeContext);
    if (!ctx) throw new Error('useFreeRange must be used within <FreeRangeProvider>');
    return ctx;
}
```

### 7.2 NotificationBell (Rewrite)

**File:** `sdk/react/src/NotificationBell.tsx`

Rewrite to use `FreeRangeProvider` context instead of standalone props. Add tabs, bulk actions, and category filtering:

```tsx
export interface NotificationBellProps {
    maxItems?: number;
    className?: string;
    bellIcon?: React.ReactNode;
    tabs?: Array<{ label: string; category: string }>;
    onNotification?: (notification: NotificationItem) => void;
    // Styling
    theme?: 'light' | 'dark';
}

const DEFAULT_TABS = [
    { label: 'All', category: '' },
    { label: 'Alerts', category: 'alert' },
    { label: 'Updates', category: 'update' },
];
```

Key additions:
- Tabs with category filtering
- "Mark all read" calls `client.notifications.markAllRead()`
- "Archive" swipe/button calls `client.notifications.archive()`
- "Snooze" option in notification menu
- Fetches unread count via `client.notifications.getUnreadCount()` on mount
- Infinite scroll via `client.notifications.list()` with pagination

### 7.3 Preferences Component

**File:** `sdk/react/src/Preferences.tsx`

```tsx
export interface PreferencesProps {
    theme?: 'light' | 'dark';
    onSave?: (preferences: Preferences) => void;
    className?: string;
}

export function Preferences({ theme, onSave, className }: PreferencesProps) {
    const { client, userId } = useFreeRange();
    const [prefs, setPrefs] = useState<Preferences | null>(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        client.users.getPreferences(userId)
            .then(p => { setPrefs(p); setLoading(false); })
            .catch(() => setLoading(false));
    }, [client, userId]);

    const handleSave = async () => {
        if (!prefs) return;
        await client.users.updatePreferences(userId, prefs);
        onSave?.(prefs);
    };

    // Render channel toggles, quiet hours editor, category preferences, etc.
}
```

### 7.4 Headless Hooks

**File:** `sdk/react/src/hooks.ts`

```tsx
// ── useNotifications: fetch & manage notification list ──
export function useNotifications(options?: {
    category?: string;
    pageSize?: number;
    unreadOnly?: boolean;
}): {
    notifications: NotificationItem[];
    loading: boolean;
    unreadCount: number;
    markRead: (ids: string[]) => Promise<void>;
    markAllRead: () => Promise<void>;
    archive: (ids: string[]) => Promise<void>;
    snooze: (id: string, duration: string) => Promise<void>;
    loadMore: () => Promise<void>;
    hasMore: boolean;
}

// ── usePreferences: fetch & update user preferences ──
export function usePreferences(): {
    preferences: Preferences | null;
    loading: boolean;
    update: (prefs: Partial<Preferences>) => Promise<void>;
}

// ── useSSE: real-time notification stream ──
export function useSSE(options?: {
    onNotification?: (notification: SSENotification) => void;
}): {
    connected: boolean;
    lastNotification: SSENotification | null;
}

// ── useUnreadCount: reactive unread count ──
export function useUnreadCount(): {
    count: number;
    loading: boolean;
    refresh: () => Promise<void>;
}
```

### 7.5 File Inventory

| Action | File | Description |
|--------|------|-------------|
| **CREATE** | `sdk/react/src/FreeRangeProvider.tsx` | React context provider |
| **REWRITE** | `sdk/react/src/NotificationBell.tsx` | Rewritten with tabs, actions, context |
| **CREATE** | `sdk/react/src/Preferences.tsx` | Preferences management component |
| **CREATE** | `sdk/react/src/hooks.ts` | Headless hooks (useNotifications, usePreferences, useSSE, useUnreadCount) |
| **CREATE** | `sdk/react/src/components/ChannelToggle.tsx` | Channel toggle UI component |
| **CREATE** | `sdk/react/src/components/QuietHoursEditor.tsx` | Quiet hours editor component |
| **REWRITE** | `sdk/react/src/index.tsx` | Re-exports all components and hooks |
| **REWRITE** | `sdk/react/README.md` | Updated documentation |

---

## Section 8: Feature 5.7 — Snooze Worker Enhancement

The un-snooze loop (Section 2.5) runs inside the existing worker process. It needs:

1. Access to `notifRepo` (already available on `Processor`)
2. Access to `sseBroadcaster` (need to check if available in worker — if not, publish via Redis pub/sub)
3. A `context.Context` with cancellation for graceful shutdown

**File:** `cmd/worker/processor.go` — Add `runUnsnoozeLoop` method, start it as a goroutine in the processor's `Start()` or `Run()` method.

**Redis alternative (if SSE broadcaster not in worker):**
Instead of direct SSE publish, push unsnooze events to Redis pub/sub channel `frn:sse:notifications`. The SSE broadcaster in the API server picks them up and delivers to connected clients.

---

## Section 9: Wiring & Container Integration

| Change | File | Description |
|--------|------|-------------|
| Add snooze/archive routes | `internal/interfaces/http/routes/routes.go` | `POST /:id/snooze`, `POST /:id/unsnooze`, `PATCH /bulk/archive`, `POST /read-all` |
| Add subscriber-hash route | `internal/interfaces/http/routes/routes.go` | `GET /users/:id/subscriber-hash` |
| No container changes needed | — | Handlers already have access to notification service and user handler |

**Route placement:**

```go
// After existing notification routes:
notifications.Post("/:id/snooze", c.NotificationHandler.Snooze)
notifications.Post("/:id/unsnooze", c.NotificationHandler.Unsnooze)
notifications.Patch("/bulk/archive", c.NotificationHandler.BulkArchive)
notifications.Post("/read-all", c.NotificationHandler.MarkAllRead)

// After existing user routes:
users.Get("/:id/subscriber-hash", c.UserHandler.GetSubscriberHash)
```

**Important:** `/read-all` must be registered BEFORE `/:id` to avoid Fiber matching `read-all` as an `:id`.

---

## Section 10: Implementation Order

```
Week 17:
  ├── 5.1  Backend: Snooze, Archive & Bulk Actions (domain → repo → service → handler → routes)
  ├── 5.2  Backend: Subscriber Hash Endpoint
  └── 5.3  Backend: DTO Alignment

Week 18:
  └── 5.4  Go SDK: Complete rewrite (client → sub-clients → types → errors → README)

Week 19:
  ├── 5.5  JS/TS SDK: Complete rewrite (client → modules → types → SSE → errors → README)
  └── 5.7  Snooze Worker Enhancement (un-snooze loop in processor)

Week 20:
  ├── 5.6  React SDK: Component library (provider → bell rewrite → preferences → hooks → README)
  └──      Integration testing & documentation
```

**Step-by-step implementation order (for `proceed` command):**

| Step | Feature | Files |
|------|---------|-------|
| 1 | Read all files that need modification | — |
| 2 | Add `StatusSnoozed`, `StatusArchived`, `SnoozedUntil`, `ArchivedAt`, new service/repo methods | `notification/models.go` |
| 3 | Implement new repository methods | `notification_repository.go` |
| 4 | Implement new service methods | `notification_service.go` |
| 5 | Add Snooze/Archive/MarkAllRead DTOs | `notification_dto.go` |
| 6 | Add Snooze/Unsnooze/BulkArchive/MarkAllRead handlers | `notification_handler.go` |
| 7 | Add `GetSubscriberHash` handler | `user_handler.go` |
| 8 | Fix DTO alignment (Phase 3 channel preferences) | `user_dto.go`, `application_dto.go` |
| 9 | Register new routes (snooze, archive, read-all, subscriber-hash) | `routes.go` |
| 10 | Add un-snooze loop to worker | `processor.go` |
| 11 | Go SDK: Rewrite `client.go` + create sub-client files | `sdk/go/freerangenotify/` |
| 12 | Go SDK: Create `types.go` + `errors.go` | `sdk/go/freerangenotify/` |
| 13 | Go SDK: Update `README.md` | `sdk/go/freerangenotify/` |
| 14 | JS SDK: Create `client.ts`, `errors.ts`, `types.ts` | `sdk/js/src/` |
| 15 | JS SDK: Create sub-client modules | `sdk/js/src/` |
| 16 | JS SDK: Rewrite `index.ts` + create `sse.ts` | `sdk/js/src/` |
| 17 | JS SDK: Update `README.md` | `sdk/js/` |
| 18 | React SDK: Create `FreeRangeProvider.tsx` + `hooks.ts` | `sdk/react/src/` |
| 19 | React SDK: Rewrite `NotificationBell.tsx` + create `Preferences.tsx` | `sdk/react/src/` |
| 20 | React SDK: Create component sub-files + rewrite `index.tsx` | `sdk/react/src/` |
| 21 | React SDK: Update `README.md` | `sdk/react/` |
| 22 | Build verification (`go build ./...`, `go vet ./...`) | — |

---

## Section 11: File Inventory (All Phases)

### Backend (Go)

| Action | File | Lines (est.) |
|--------|------|-------------|
| MODIFY | `internal/domain/notification/models.go` | +40 |
| MODIFY | `internal/infrastructure/database/notification_repository.go` | +120 |
| MODIFY | `internal/usecases/notification_service.go` | +80 |
| MODIFY | `internal/interfaces/http/dto/notification_dto.go` | +25 |
| MODIFY | `internal/interfaces/http/handlers/notification_handler.go` | +100 |
| MODIFY | `internal/interfaces/http/handlers/user_handler.go` | +20 |
| MODIFY | `internal/interfaces/http/dto/user_dto.go` | +10 |
| MODIFY | `internal/interfaces/http/dto/application_dto.go` | +6 |
| MODIFY | `internal/interfaces/http/routes/routes.go` | +10 |
| MODIFY | `cmd/worker/processor.go` | +40 |

### Go SDK

| Action | File | Lines (est.) |
|--------|------|-------------|
| REWRITE | `sdk/go/freerangenotify/client.go` | ~120 |
| CREATE | `sdk/go/freerangenotify/notifications.go` | ~200 |
| CREATE | `sdk/go/freerangenotify/users.go` | ~120 |
| CREATE | `sdk/go/freerangenotify/templates.go` | ~160 |
| CREATE | `sdk/go/freerangenotify/workflows.go` | ~110 |
| CREATE | `sdk/go/freerangenotify/topics.go` | ~100 |
| CREATE | `sdk/go/freerangenotify/presence.go` | ~20 |
| CREATE | `sdk/go/freerangenotify/types.go` | ~350 |
| CREATE | `sdk/go/freerangenotify/errors.go` | ~35 |
| REWRITE | `sdk/go/freerangenotify/README.md` | ~200 |

### JS/TS SDK

| Action | File | Lines (est.) |
|--------|------|-------------|
| REWRITE | `sdk/js/src/index.ts` | ~80 |
| CREATE | `sdk/js/src/client.ts` | ~45 |
| CREATE | `sdk/js/src/notifications.ts` | ~180 |
| CREATE | `sdk/js/src/users.ts` | ~110 |
| CREATE | `sdk/js/src/templates.ts` | ~140 |
| CREATE | `sdk/js/src/workflows.ts` | ~90 |
| CREATE | `sdk/js/src/topics.ts` | ~80 |
| CREATE | `sdk/js/src/presence.ts` | ~15 |
| CREATE | `sdk/js/src/sse.ts` | ~70 |
| CREATE | `sdk/js/src/types.ts` | ~300 |
| CREATE | `sdk/js/src/errors.ts` | ~20 |
| REWRITE | `sdk/js/README.md` | ~200 |

### React SDK

| Action | File | Lines (est.) |
|--------|------|-------------|
| CREATE | `sdk/react/src/FreeRangeProvider.tsx` | ~50 |
| REWRITE | `sdk/react/src/NotificationBell.tsx` | ~300 |
| CREATE | `sdk/react/src/Preferences.tsx` | ~200 |
| CREATE | `sdk/react/src/hooks.ts` | ~150 |
| CREATE | `sdk/react/src/components/ChannelToggle.tsx` | ~40 |
| CREATE | `sdk/react/src/components/QuietHoursEditor.tsx` | ~60 |
| REWRITE | `sdk/react/src/index.tsx` | ~30 |
| REWRITE | `sdk/react/README.md` | ~200 |

**Grand Total: ~3,920 lines across 41 files (10 modify, 23 create, 8 rewrite)**
