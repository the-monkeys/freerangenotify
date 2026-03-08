# Phase 1 — Detailed Design Document

> **Date:** March 7, 2026  
> **Scope:** Workflow Engine, Digest Engine, HMAC for SSE/Inbox  
> **Module:** `github.com/the-monkeys/freerangenotify`  
> **Constraint:** Production is live. Every change MUST be backward compatible. Existing client integrations, API contracts, SDK behavior, Elasticsearch indices, and Redis key schemas must continue to work without modification after deployment.

---

## Table of Contents

0. [Concepts — What Are These Features?](#0-concepts--what-are-these-features)
1. [Backward Compatibility Contract](#1-backward-compatibility-contract)
2. [Migration Strategy](#2-migration-strategy)
3. [Feature 1: Workflow Engine](#3-feature-1-workflow-engine)
4. [Feature 2: Digest / Batching Engine](#4-feature-2-digest--batching-engine)
5. [Feature 3: HMAC for SSE / Inbox](#5-feature-3-hmac-for-sse--inbox)
6. [Deployment Sequence](#6-deployment-sequence)
7. [Rollback Plan](#7-rollback-plan)
8. [Testing Strategy](#8-testing-strategy)

---

## 0. Concepts — What Are These Features?

This section explains each Phase 1 feature in plain English before diving into the technical design.

### 0.1 Workflow Engine

A **workflow** is a multi-step notification pipeline triggered by a single event. Instead of sending one notification at a time, you define a sequence of steps that execute automatically.

**Example:** A user signs up → workflow `user_onboarding` fires:

1. **Step 1** (email): Send welcome email immediately.
2. **Step 2** (delay): Wait 24 hours.
3. **Step 3** (push): Send "Complete your profile" push notification.
4. **Step 4** (condition): If `payload.plan == "pro"` → send premium guide; otherwise → skip.

Each step can branch on success/failure (`on_success`, `on_failure`), skip conditionally (`skip_if`), and the engine tracks execution state per-user with idempotency via `transaction_id`. The orchestrator runs as background goroutines in the worker process, with automatic recovery for stale executions (stuck > 5 minutes).

**Why it matters:** Without a workflow engine, FreeRangeNotify is a delivery service — each notification is an isolated unit. With it, FRN becomes a notification *platform* that can orchestrate complex multi-channel sequences, the single largest feature gap vs. Novu.

**API surface:** `POST /v1/workflows` (CRUD), `POST /v1/workflows/trigger`, `GET /v1/workflows/executions`.

### 0.2 Digest / Batching Engine

A **digest** batches multiple notifications into a single consolidated message instead of spamming users with individual ones.

**Example:** A user gets 15 "new comment" notifications in 10 minutes. Instead of 15 separate pushes, a digest rule with `window: "30m"` accumulates them in a Redis sorted set, then flushes once as: *"You have 15 new notifications"*.

Key properties of a digest rule:
- **`digest_key`**: Metadata field that activates digesting (e.g., `"new_comment"`). Only notifications whose `metadata.digest_key` matches are batched.
- **`window`**: Time duration to accumulate events before flushing (`"5m"`, `"1h"`, `"24h"`).
- **`max_batch`**: Maximum events to collect per digest (cap for very high-volume scenarios).
- **`channel` / `template_id`**: How and with what template the consolidated notification is delivered.

The processor intercepts matching notifications *before* delivery, accumulates them in Redis, and a background flush poller sends the consolidated message when the window expires. If no digest rules are configured, the system behaves identically to before — zero impact.

**Why it matters:** Digest is Novu's headline feature. High-frequency apps (CI/CD, project management, social feeds) need batching to avoid notification fatigue.

**API surface:** `POST /v1/digest-rules` (CRUD).

### 0.3 HMAC for SSE / Inbox

**HMAC subscriber authentication** prevents unauthorized users from opening SSE connections and consuming another user's notifications.

Without HMAC, anyone who knows (or guesses) a `user_id` can connect to `GET /v1/sse?user_id=xxx` and read that user's real-time notification stream. HMAC adds a cryptographic proof: the backend generates `HMAC-SHA256(user_id, api_key)` and the client must present this hash as `subscriber_hash` when connecting.

- **Backward compatible:** If `features.sse_hmac_enforced` is `false` (default), SSE works exactly as before — no hash required.
- **Opt-in enforcement:** Set the flag to `true` and only connections with a valid `subscriber_hash` are accepted.
- **SDK support:** Both `@freerangenotify/sdk` (JS) and `@freerangenotify/react` (`<NotificationBell>`) accept an optional `subscriberHash` parameter.

**Why it matters:** Security best practice for any in-app inbox. Required for production deployments where user impersonation must be prevented.

---

## 1. Backward Compatibility Contract

These are hard rules. Any design that violates them is rejected.

### 1.1 API Surface — Zero Breaking Changes

| Rule | Detail |
|------|--------|
| **No existing endpoint is removed or renamed** | All current routes in [routes.go](../internal/interfaces/http/routes/routes.go) remain unchanged. |
| **No existing request field becomes required** | `POST /v1/notifications` continues to work with the exact same payload. No new required fields are added to existing DTOs. |
| **No existing response field is removed** | `NotificationResponse` in [notification_dto.go](../internal/interfaces/http/dto/notification_dto.go) keeps every field. New fields are additive (with `omitempty` so clients ignoring them see no change). |
| **No existing validation rule is tightened** | `SendNotificationRequest.Channel` remains `validate:"omitempty"`. `UserID` remains optional for webhook channel. |
| **HTTP status codes are unchanged** | Same codes for same inputs. New endpoints use standard codes. |

### 1.2 Elasticsearch — Additive Only

| Rule | Detail |
|------|--------|
| **No existing index schema is modified** | The `notifications`, `users`, `applications`, `templates`, `analytics`, `auth_users` indices are untouched. |
| **New indices are additive** | `workflows`, `workflow_executions`, `digest_rules` are created alongside existing indices. |
| **No reindexing required** | Existing documents are never touched by the migration. |
| **Index creation is idempotent** | `CreateIndices` checks `IndexExists` before creating — already the case in [index_manager.go](../internal/infrastructure/database/index_manager.go). New indices follow this pattern. |

### 1.3 Redis — Separate Key Namespaces

| Rule | Detail |
|------|--------|
| **Existing Redis keys are untouched** | Keys `frn:queue:high`, `frn:queue:normal`, `frn:queue:low`, `frn:retry`, `frn:dlq`, `frn:scheduled`, `frn:presence:*` are not modified. |
| **New features use new key prefixes** | Workflow queue: `frn:queue:workflow`. Workflow delays: `frn:workflow:delayed`. Digest accumulators: `frn:digest:*`. Digest flush schedule: `frn:digest:flush`. |
| **Queue interface is extended, not modified** | New methods are added to the `Queue` interface. Existing methods are unchanged. The `RedisQueue` struct gains new methods without altering existing ones. |

### 1.4 Go Interfaces — Extension, Not Modification

| Rule | Detail |
|------|--------|
| **`notification.Service` is unchanged** | All 14 methods stay exactly as they are. No signature changes. |
| **`notification.Repository` is unchanged** | All 11 methods stay. |
| **`queue.Queue` is extended** | New workflow/digest methods are added. BUT: to avoid breaking any existing code that implements `Queue`, the new methods go into a **new interface** `WorkflowQueue` that embeds `Queue`. The worker asserts this interface at runtime. |
| **`providers.Provider` is unchanged** | `Send`, `GetName`, `GetSupportedChannel`, `IsHealthy`, `Close` remain as-is. |
| **Container struct is extended** | New fields are added; no existing field is removed or changed. Existing handlers/services keep their types and constructor signatures. |

### 1.5 Worker — Side-by-Side Processing

| Rule | Detail |
|------|--------|
| **Existing notification processing loop is untouched** | The `worker()` goroutine in [processor.go](../cmd/worker/processor.go) dequeues from the same `frn:queue:*` keys and processes notifications identically. |
| **New workflow processing runs in a separate goroutine pool** | A new `WorkflowProcessor` struct runs alongside `NotificationProcessor` with its own goroutines. It reads from `frn:queue:workflow` only. |
| **Digest check is the only modification to existing code** | One optional check is inserted into `processNotification()` — if a notification matches a digest rule, it is accumulated instead of delivered. If no digest rules exist (default), behavior is identical to today. |

### 1.6 SDK — Backward Compatible

| Rule | Detail |
|------|--------|
| **SSE connection without `hash` continues to work** | The HMAC hash is opt-in. If `hash` query param is absent, the existing auth flow (token-based) continues. HMAC is enforced only when the app enables it via a new settings flag. |
| **JS SDK and React component work without changes** | `connectSSE()` and `<NotificationBell>` continue to use `token=` auth. HMAC support is added as an optional `subscriberHash` parameter. |

---

## 2. Migration Strategy

### 2.1 Database Migration (Zero Downtime)

The migration creates new Elasticsearch indices without touching existing ones.

**File to modify:** `internal/infrastructure/database/index_manager.go`

Add three new indices to the `indices` map inside `CreateIndices()`:

```go
// Existing indices (unchanged)
"applications":          im.templates.GetApplicationsTemplate,
"users":                 im.templates.GetUsersTemplate,
"notifications":         im.templates.GetNotificationsTemplate,
"templates":             im.templates.GetTemplatesTemplate,
"analytics":             im.templates.GetAnalyticsTemplate,
"auth_users":            im.templates.GetAuthUsersTemplate,
"password_reset_tokens": im.templates.GetPasswordResetTokensTemplate,
"refresh_tokens":        im.templates.GetRefreshTokensTemplate,

// Phase 1 additions (additive only)
"workflows":             im.templates.GetWorkflowsTemplate,
"workflow_executions":   im.templates.GetWorkflowExecutionsTemplate,
"digest_rules":          im.templates.GetDigestRulesTemplate,
```

**Why this is safe:** `createIndex()` already calls `IndexExists()` and returns `"index already exists"` if it does. Existing indices are skipped. New indices are created. No reindex.

### 2.2 Deployment Order

```
Step 1: Deploy new Docker images (server + worker) with feature flags OFF
Step 2: Run migrate (creates new indices alongside existing ones)
Step 3: Smoke test — all existing API calls work identically
Step 4: Enable workflow/digest features via config (no restart needed if using env vars)
```

### 2.3 Feature Flags

New features are gated behind configuration flags. When disabled, the system behaves exactly as the current production.

**File to modify:** `internal/config/config.go`

```go
type FeaturesConfig struct {
    WorkflowEnabled bool `mapstructure:"workflow_enabled" yaml:"workflow_enabled"`
    DigestEnabled   bool `mapstructure:"digest_enabled" yaml:"digest_enabled"`
    SSEHMACEnforced bool `mapstructure:"sse_hmac_enforced" yaml:"sse_hmac_enforced"`
}
```

**Config YAML:**
```yaml
features:
  workflow_enabled: false   # Default OFF — no behavior change
  digest_enabled: false     # Default OFF — no behavior change
  sse_hmac_enforced: false  # Default OFF — SSE works as before
```

**Environment variables:**
```
FREERANGE_FEATURES_WORKFLOW_ENABLED=false
FREERANGE_FEATURES_DIGEST_ENABLED=false
FREERANGE_FEATURES_SSE_HMAC_ENFORCED=false
```

---

## 3. Feature 1: Workflow Engine

### 3.1 Design Principle

The workflow engine is an **additive layer** on top of the existing notification system. A workflow is a sequence of steps that, when triggered, creates and sends notifications using the existing `notification.Service.Send()` path. The workflow engine does not replace or bypass the notification pipeline — it orchestrates it.

```
             ┌─────────────────────────────────────────┐
             │         NEW: Workflow Engine             │
             │  (Orchestrates multi-step notification   │
             │   pipelines using existing primitives)   │
             └────────────────┬────────────────────────┘
                              │ Creates notifications via
                              │ existing notification.Service.Send()
                              ▼
    ┌──────────────────────────────────────────────────────────┐
    │        EXISTING (unchanged): Notification Pipeline       │
    │  API → ES (pending) → Redis Queue → Worker → Provider   │
    └──────────────────────────────────────────────────────────┘
```

### 3.2 Domain Model

**New file:** `internal/domain/workflow/models.go`

```go
package workflow

import (
    "context"
    "time"
)

// StepType defines the kind of action a workflow step performs.
type StepType string

const (
    StepTypeChannel   StepType = "channel"   // Deliver via a channel
    StepTypeDelay     StepType = "delay"     // Wait for a duration
    StepTypeDigest    StepType = "digest"    // Aggregate events over a window
    StepTypeCondition StepType = "condition" // Branch based on a condition
)

// WorkflowStatus represents the lifecycle state of a workflow definition.
type WorkflowStatus string

const (
    WorkflowStatusDraft    WorkflowStatus = "draft"
    WorkflowStatusActive   WorkflowStatus = "active"
    WorkflowStatusInactive WorkflowStatus = "inactive"
)

// ExecutionStatus represents the lifecycle state of a single workflow run.
type ExecutionStatus string

const (
    ExecStatusRunning   ExecutionStatus = "running"
    ExecStatusPaused    ExecutionStatus = "paused"
    ExecStatusCompleted ExecutionStatus = "completed"
    ExecStatusFailed    ExecutionStatus = "failed"
    ExecStatusCancelled ExecutionStatus = "cancelled"
)

// StepStatus represents the outcome of one step within an execution.
type StepStatus string

const (
    StepStatusPending   StepStatus = "pending"
    StepStatusRunning   StepStatus = "running"
    StepStatusCompleted StepStatus = "completed"
    StepStatusFailed    StepStatus = "failed"
    StepStatusSkipped   StepStatus = "skipped"
)

// ConditionOperator for conditional steps.
type ConditionOperator string

const (
    OpEquals      ConditionOperator = "eq"
    OpNotEquals   ConditionOperator = "neq"
    OpContains    ConditionOperator = "contains"
    OpGreaterThan ConditionOperator = "gt"
    OpLessThan    ConditionOperator = "lt"
    OpExists      ConditionOperator = "exists"
    OpNotRead     ConditionOperator = "not_read"
)

// Workflow defines a multi-step notification pipeline.
type Workflow struct {
    ID          string         `json:"id"`
    AppID       string         `json:"app_id"`
    Name        string         `json:"name"`
    Description string         `json:"description"`
    TriggerID   string         `json:"trigger_id"`   // External identifier clients use to invoke
    Steps       []Step         `json:"steps"`
    Status      WorkflowStatus `json:"status"`
    Version     int            `json:"version"`
    CreatedBy   string         `json:"created_by"`
    CreatedAt   time.Time      `json:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at"`
}

// Step is one node in the workflow pipeline.
type Step struct {
    ID        string          `json:"id"`
    Name      string          `json:"name"`
    Type      StepType        `json:"type"`
    Order     int             `json:"order"`
    Config    StepConfig      `json:"config"`
    OnSuccess string          `json:"on_success,omitempty"` // Next step ID on success
    OnFailure string          `json:"on_failure,omitempty"` // Fallback step ID on failure
    SkipIf    *Condition      `json:"skip_if,omitempty"`
    Metadata  map[string]any  `json:"metadata,omitempty"`
}

// StepConfig holds type-specific configuration.
type StepConfig struct {
    // Channel step
    Channel    string `json:"channel,omitempty"`     // "email", "push", "sms", etc.
    TemplateID string `json:"template_id,omitempty"`
    Provider   string `json:"provider,omitempty"`    // Override default provider

    // Delay step
    Duration string `json:"duration,omitempty"`      // Go duration: "1h", "30m"

    // Digest step
    DigestKey string `json:"digest_key,omitempty"`   // Grouping key
    Window    string `json:"window,omitempty"`        // "1h", "24h"
    MaxBatch  int    `json:"max_batch,omitempty"`

    // Condition step
    Condition *Condition `json:"condition,omitempty"`
}

// Condition defines a conditional expression.
type Condition struct {
    Field    string            `json:"field"`    // e.g. "payload.amount"
    Operator ConditionOperator `json:"operator"`
    Value    any               `json:"value"`
}

// WorkflowExecution tracks a single run of a workflow for one subscriber.
type WorkflowExecution struct {
    ID            string                `json:"id"`
    WorkflowID    string                `json:"workflow_id"`
    AppID         string                `json:"app_id"`
    UserID        string                `json:"user_id"`
    TransactionID string               `json:"transaction_id"` // Idempotency key
    CurrentStepID string                `json:"current_step_id"`
    Status        ExecutionStatus       `json:"status"`
    Payload       map[string]any        `json:"payload"`
    StepResults   map[string]StepResult `json:"step_results"`
    StartedAt     time.Time             `json:"started_at"`
    CompletedAt   *time.Time            `json:"completed_at,omitempty"`
    UpdatedAt     time.Time             `json:"updated_at"`
}

// StepResult records the outcome of one step execution.
type StepResult struct {
    StepID         string     `json:"step_id"`
    Status         StepStatus `json:"status"`
    NotificationID string     `json:"notification_id,omitempty"` // For channel steps
    DigestCount    int        `json:"digest_count,omitempty"`    // For digest steps
    StartedAt      *time.Time `json:"started_at,omitempty"`
    CompletedAt    *time.Time `json:"completed_at,omitempty"`
    Error          string     `json:"error,omitempty"`
}
```

**New file:** `internal/domain/workflow/repository.go`

```go
package workflow

import "context"

// Repository defines data operations for workflows and their executions.
type Repository interface {
    // Workflow CRUD
    CreateWorkflow(ctx context.Context, wf *Workflow) error
    GetWorkflow(ctx context.Context, id string) (*Workflow, error)
    GetWorkflowByTrigger(ctx context.Context, appID, triggerID string) (*Workflow, error)
    UpdateWorkflow(ctx context.Context, wf *Workflow) error
    DeleteWorkflow(ctx context.Context, id string) error
    ListWorkflows(ctx context.Context, appID string, limit, offset int) ([]*Workflow, int64, error)

    // Execution CRUD
    CreateExecution(ctx context.Context, exec *WorkflowExecution) error
    GetExecution(ctx context.Context, id string) (*WorkflowExecution, error)
    UpdateExecution(ctx context.Context, exec *WorkflowExecution) error
    ListExecutions(ctx context.Context, workflowID string, limit, offset int) ([]*WorkflowExecution, int64, error)
    GetActiveExecutions(ctx context.Context, userID, workflowID string) ([]*WorkflowExecution, error)

    // Recovery: find executions stuck in "running" for too long
    GetStaleExecutions(ctx context.Context, staleSince time.Time, limit int) ([]*WorkflowExecution, error)
}
```

**New file:** `internal/domain/workflow/service.go`

```go
package workflow

import "context"

// Service defines the business logic interface for workflows.
type Service interface {
    // CRUD
    Create(ctx context.Context, appID string, req *CreateRequest) (*Workflow, error)
    Get(ctx context.Context, id, appID string) (*Workflow, error)
    Update(ctx context.Context, id, appID string, req *UpdateRequest) (*Workflow, error)
    Delete(ctx context.Context, id, appID string) error
    List(ctx context.Context, appID string, limit, offset int) ([]*Workflow, int64, error)

    // Execution
    Trigger(ctx context.Context, appID string, req *TriggerRequest) (*WorkflowExecution, error)
    CancelExecution(ctx context.Context, executionID, appID string) error
    GetExecution(ctx context.Context, executionID, appID string) (*WorkflowExecution, error)
    ListExecutions(ctx context.Context, workflowID, appID string, limit, offset int) ([]*WorkflowExecution, int64, error)
}

// CreateRequest is the input for creating a workflow.
type CreateRequest struct {
    Name        string `json:"name" validate:"required,min=3,max=100"`
    Description string `json:"description"`
    TriggerID   string `json:"trigger_id" validate:"required,min=1,max=100"`
    Steps       []Step `json:"steps" validate:"required,min=1,dive"`
}

// UpdateRequest is the input for updating a workflow.
type UpdateRequest struct {
    Name        *string         `json:"name,omitempty" validate:"omitempty,min=3,max=100"`
    Description *string         `json:"description,omitempty"`
    Steps       []Step          `json:"steps,omitempty" validate:"omitempty,min=1,dive"`
    Status      *WorkflowStatus `json:"status,omitempty"`
}

// TriggerRequest is the input for triggering a workflow execution.
type TriggerRequest struct {
    TriggerID     string         `json:"trigger_id" validate:"required"`
    UserID        string         `json:"user_id" validate:"required"`
    Payload       map[string]any `json:"payload"`
    TransactionID string         `json:"transaction_id"` // Optional idempotency key
    Overrides     map[string]any `json:"overrides"`      // Per-trigger template variable overrides
}
```

### 3.3 Elasticsearch Indices

**File to modify:** `internal/infrastructure/database/index_templates.go`

Add these methods to `IndexTemplates`:

```go
// GetWorkflowsTemplate returns the Elasticsearch mapping for workflows index.
func (it *IndexTemplates) GetWorkflowsTemplate() map[string]interface{} {
    return map[string]interface{}{
        "settings": map[string]interface{}{
            "number_of_shards":   1,
            "number_of_replicas": 0,
        },
        "mappings": map[string]interface{}{
            "properties": map[string]interface{}{
                "workflow_id": map[string]interface{}{"type": "keyword"},
                "app_id":      map[string]interface{}{"type": "keyword"},
                "name": map[string]interface{}{
                    "type": "text",
                    "fields": map[string]interface{}{
                        "keyword": map[string]interface{}{"type": "keyword"},
                    },
                },
                "trigger_id":  map[string]interface{}{"type": "keyword"},
                "description": map[string]interface{}{"type": "text"},
                "steps": map[string]interface{}{
                    "type": "nested",
                    "properties": map[string]interface{}{
                        "step_id":    map[string]interface{}{"type": "keyword"},
                        "name":       map[string]interface{}{"type": "text"},
                        "type":       map[string]interface{}{"type": "keyword"},
                        "order":      map[string]interface{}{"type": "integer"},
                        "config":     map[string]interface{}{"type": "object", "enabled": false},
                        "on_success": map[string]interface{}{"type": "keyword"},
                        "on_failure": map[string]interface{}{"type": "keyword"},
                        "skip_if":    map[string]interface{}{"type": "object", "enabled": false},
                    },
                },
                "status":     map[string]interface{}{"type": "keyword"},
                "version":    map[string]interface{}{"type": "integer"},
                "created_by": map[string]interface{}{"type": "keyword"},
                "created_at": map[string]interface{}{"type": "date"},
                "updated_at": map[string]interface{}{"type": "date"},
            },
        },
    }
}

// GetWorkflowExecutionsTemplate returns the ES mapping for workflow_executions index.
func (it *IndexTemplates) GetWorkflowExecutionsTemplate() map[string]interface{} {
    return map[string]interface{}{
        "settings": map[string]interface{}{
            "number_of_shards":   1,
            "number_of_replicas": 0,
        },
        "mappings": map[string]interface{}{
            "properties": map[string]interface{}{
                "execution_id":    map[string]interface{}{"type": "keyword"},
                "workflow_id":     map[string]interface{}{"type": "keyword"},
                "app_id":          map[string]interface{}{"type": "keyword"},
                "user_id":         map[string]interface{}{"type": "keyword"},
                "transaction_id":  map[string]interface{}{"type": "keyword"},
                "current_step_id": map[string]interface{}{"type": "keyword"},
                "status":          map[string]interface{}{"type": "keyword"},
                "payload":         map[string]interface{}{"type": "object", "enabled": false},
                "step_results":    map[string]interface{}{"type": "object", "enabled": false},
                "started_at":      map[string]interface{}{"type": "date"},
                "completed_at":    map[string]interface{}{"type": "date"},
                "updated_at":      map[string]interface{}{"type": "date"},
            },
        },
    }
}
```

**Impact on existing indices:** None. These are new indices created by the same idempotent `CreateIndices()` flow.

### 3.4 Queue Extension — Interface Segregation

**Problem:** Adding methods directly to the `Queue` interface in [queue.go](../internal/infrastructure/queue/queue.go) would break any test doubles or alternative implementations that implement the current `Queue` interface.

**Solution:** Define a new `WorkflowQueue` interface in a separate file. The worker asserts `queue.Queue` to `WorkflowQueue` at startup.

**New file:** `internal/infrastructure/queue/workflow_queue.go`

```go
package queue

import (
    "context"
    "time"
)

// WorkflowQueueItem represents a workflow execution step in the queue.
type WorkflowQueueItem struct {
    ExecutionID string    `json:"execution_id"`
    StepID      string    `json:"step_id"`
    EnqueuedAt  time.Time `json:"enqueued_at"`
}

// WorkflowQueue extends Queue with workflow-specific operations.
// Any Queue implementation that also supports workflow processing
// should implement this interface.
type WorkflowQueue interface {
    Queue // Embeds existing Queue — backward compatible

    EnqueueWorkflow(ctx context.Context, item WorkflowQueueItem) error
    DequeueWorkflow(ctx context.Context) (*WorkflowQueueItem, error)
    EnqueueWorkflowDelayed(ctx context.Context, item WorkflowQueueItem, executeAt time.Time) error
    GetDelayedWorkflowItems(ctx context.Context, limit int64) ([]WorkflowQueueItem, error)
}
```

**File to modify:** `internal/infrastructure/queue/redis_queue.go`

Add new methods to `RedisQueue` struct. Existing methods are untouched.

```go
// --- Workflow Queue Methods (Phase 1) ---

const workflowQueueKey = "frn:queue:workflow"
const workflowDelayedKey = "frn:workflow:delayed"

func (q *RedisQueue) EnqueueWorkflow(ctx context.Context, item WorkflowQueueItem) error {
    data, err := json.Marshal(item)
    if err != nil {
        return fmt.Errorf("failed to marshal workflow queue item: %w", err)
    }
    return q.client.RPush(ctx, workflowQueueKey, data).Err()
}

func (q *RedisQueue) DequeueWorkflow(ctx context.Context) (*WorkflowQueueItem, error) {
    result, err := q.client.BLPop(ctx, 5*time.Second, workflowQueueKey).Result()
    if err != nil {
        if err == redis.Nil {
            return nil, nil
        }
        return nil, err
    }
    var item WorkflowQueueItem
    if err := json.Unmarshal([]byte(result[1]), &item); err != nil {
        return nil, fmt.Errorf("failed to unmarshal workflow queue item: %w", err)
    }
    return &item, nil
}

func (q *RedisQueue) EnqueueWorkflowDelayed(ctx context.Context, item WorkflowQueueItem, executeAt time.Time) error {
    data, err := json.Marshal(item)
    if err != nil {
        return fmt.Errorf("failed to marshal workflow queue item: %w", err)
    }
    return q.client.ZAdd(ctx, workflowDelayedKey, &redis.Z{
        Score:  float64(executeAt.Unix()),
        Member: string(data),
    }).Err()
}

func (q *RedisQueue) GetDelayedWorkflowItems(ctx context.Context, limit int64) ([]WorkflowQueueItem, error) {
    now := float64(time.Now().Unix())
    results, err := q.client.ZRangeByScore(ctx, workflowDelayedKey, &redis.ZRangeBy{
        Min:   "-inf",
        Max:   fmt.Sprintf("%f", now),
        Count: limit,
    }).Result()
    if err != nil {
        return nil, err
    }
    var items []WorkflowQueueItem
    for _, r := range results {
        var item WorkflowQueueItem
        if err := json.Unmarshal([]byte(r), &item); err != nil {
            continue
        }
        // Remove from sorted set atomically
        q.client.ZRem(ctx, workflowDelayedKey, r)
        items = append(items, item)
    }
    return items, nil
}
```

**Backward compatibility proof:**

- The `Queue` interface in [queue.go](../internal/infrastructure/queue/queue.go) is NOT modified.
- `RedisQueue` already satisfies `Queue`. It now also satisfies `WorkflowQueue`.
- All existing code that uses `queue.Queue` continues to compile and work.
- The workflow processor does a type assertion: `wq, ok := q.(queue.WorkflowQueue)`. If the assertion fails (e.g., a test mock), the workflow processor logs a warning and does not start.

### 3.5 Repository Implementation

**New file:** `internal/infrastructure/repository/workflow_repository.go`

Implements `workflow.Repository` using the ES client. Follows the exact same pattern as existing repositories in the same package:
- Uses `esapi.IndexRequest` for create/update
- Uses `esapi.GetRequest` for get by ID
- Uses `esapi.SearchRequest` with query DSL for list/filter
- Uses `esapi.DeleteRequest` for delete
- Document ID is `Workflow.ID` (UUID generated in service layer)

Key queries:
- `GetWorkflowByTrigger(appID, triggerID)`: `bool { must: [ {term: {app_id}}, {term: {trigger_id}} ] }`
- `GetStaleExecutions(staleSince, limit)`: `bool { must: [ {term: {status: "running"}}, {range: {updated_at: {lt: staleSince}}} ] }`
- `GetActiveExecutions(userID, workflowID)`: `bool { must: [ {term: {user_id}}, {term: {workflow_id}}, {term: {status: "running"}} ] }`

### 3.6 Service Implementation

**New file:** `internal/usecases/services/workflow_service_impl.go`

```go
package services

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/the-monkeys/freerangenotify/internal/domain/workflow"
    "github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
    "go.uber.org/zap"
)

type workflowService struct {
    repo   workflow.Repository
    queue  queue.WorkflowQueue
    logger *zap.Logger
}

func NewWorkflowService(
    repo workflow.Repository,
    wq queue.WorkflowQueue,
    logger *zap.Logger,
) workflow.Service {
    return &workflowService{
        repo:   repo,
        queue:  wq,
        logger: logger,
    }
}

func (s *workflowService) Trigger(ctx context.Context, appID string, req *workflow.TriggerRequest) (*workflow.WorkflowExecution, error) {
    // 1. Resolve workflow by trigger_id
    wf, err := s.repo.GetWorkflowByTrigger(ctx, appID, req.TriggerID)
    if err != nil {
        return nil, fmt.Errorf("workflow not found for trigger %s: %w", req.TriggerID, err)
    }

    if wf.Status != workflow.WorkflowStatusActive {
        return nil, fmt.Errorf("workflow %s is not active (status: %s)", wf.ID, wf.Status)
    }

    // 2. Idempotency check
    if req.TransactionID != "" {
        existing, _ := s.repo.GetActiveExecutions(ctx, req.UserID, wf.ID)
        for _, exec := range existing {
            if exec.TransactionID == req.TransactionID {
                return exec, nil // Already running, return existing
            }
        }
    }

    // 3. Create execution (first step ID from the sorted steps)
    firstStepID := ""
    if len(wf.Steps) > 0 {
        firstStepID = wf.Steps[0].ID
    }

    exec := &workflow.WorkflowExecution{
        ID:            uuid.New().String(),
        WorkflowID:    wf.ID,
        AppID:         appID,
        UserID:        req.UserID,
        TransactionID: req.TransactionID,
        CurrentStepID: firstStepID,
        Status:        workflow.ExecStatusRunning,
        Payload:       req.Payload,
        StepResults:   make(map[string]workflow.StepResult),
        StartedAt:     time.Now(),
        UpdatedAt:     time.Now(),
    }

    if err := s.repo.CreateExecution(ctx, exec); err != nil {
        return nil, fmt.Errorf("failed to create execution: %w", err)
    }

    // 4. Enqueue first step
    if err := s.queue.EnqueueWorkflow(ctx, queue.WorkflowQueueItem{
        ExecutionID: exec.ID,
        StepID:      firstStepID,
        EnqueuedAt:  time.Now(),
    }); err != nil {
        return nil, fmt.Errorf("failed to enqueue workflow: %w", err)
    }

    return exec, nil
}

// Create, Get, Update, Delete, List — standard CRUD, follow same
// pattern as application_service.go and template_service.go
// ...
```

### 3.7 Workflow Processor (Worker Side)

**New file:** `internal/infrastructure/orchestrator/engine.go`

The orchestrator is a state machine that runs in the worker process. It is separate from `NotificationProcessor`.

```go
package orchestrator

import (
    "context"
    "fmt"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/go-redis/redis/v8"
    "github.com/the-monkeys/freerangenotify/internal/domain/notification"
    "github.com/the-monkeys/freerangenotify/internal/domain/workflow"
    "github.com/the-monkeys/freerangenotify/internal/infrastructure/metrics"
    "github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
    "go.uber.org/zap"
)

// Engine orchestrates workflow step execution.
type Engine struct {
    workflowRepo  workflow.Repository
    notifService  notification.Service
    wfQueue       queue.WorkflowQueue
    redisClient   *redis.Client
    logger        *zap.Logger
    metrics       *metrics.NotificationMetrics

    workerCount int
    wg          sync.WaitGroup
    stopChan    chan struct{}
}

// NewEngine creates a new workflow orchestrator engine.
func NewEngine(
    workflowRepo workflow.Repository,
    notifService notification.Service,
    wfQueue queue.WorkflowQueue,
    redisClient *redis.Client,
    logger *zap.Logger,
    metrics *metrics.NotificationMetrics,
    workerCount int,
) *Engine {
    return &Engine{
        workflowRepo: workflowRepo,
        notifService: notifService,
        wfQueue:      wfQueue,
        redisClient:  redisClient,
        logger:       logger,
        metrics:      metrics,
        workerCount:  workerCount,
        stopChan:     make(chan struct{}),
    }
}

// Start launches workflow workers and the delayed step poller.
func (e *Engine) Start(ctx context.Context) {
    for i := 0; i < e.workerCount; i++ {
        e.wg.Add(1)
        go e.worker(ctx, i)
    }

    // Delayed step poller — moves delayed items to the main workflow queue
    e.wg.Add(1)
    go e.delayedPoller(ctx)

    // Recovery — re-enqueues stale executions
    e.wg.Add(1)
    go e.recovery(ctx)

    e.logger.Info("Workflow engine started",
        zap.Int("worker_count", e.workerCount))
}

// Shutdown gracefully stops the engine.
func (e *Engine) Shutdown(ctx context.Context) error {
    close(e.stopChan)
    done := make(chan struct{})
    go func() { e.wg.Wait(); close(done) }()
    select {
    case <-done:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (e *Engine) worker(ctx context.Context, id int) {
    defer e.wg.Done()
    logger := e.logger.With(zap.Int("wf_worker_id", id))

    for {
        select {
        case <-ctx.Done():
            return
        case <-e.stopChan:
            return
        default:
            item, err := e.wfQueue.DequeueWorkflow(ctx)
            if err != nil || item == nil {
                time.Sleep(1 * time.Second)
                continue
            }
            e.executeStep(ctx, item, logger)
        }
    }
}

func (e *Engine) executeStep(ctx context.Context, item *queue.WorkflowQueueItem, logger *zap.Logger) {
    exec, err := e.workflowRepo.GetExecution(ctx, item.ExecutionID)
    if err != nil {
        logger.Error("Failed to get workflow execution", zap.Error(err))
        return
    }

    if exec.Status != workflow.ExecStatusRunning {
        logger.Info("Execution is not running, skipping",
            zap.String("execution_id", exec.ID),
            zap.String("status", string(exec.Status)))
        return
    }

    wf, err := e.workflowRepo.GetWorkflow(ctx, exec.WorkflowID)
    if err != nil {
        logger.Error("Failed to get workflow", zap.Error(err))
        return
    }

    // Find the current step
    var step *workflow.Step
    for i := range wf.Steps {
        if wf.Steps[i].ID == item.StepID {
            step = &wf.Steps[i]
            break
        }
    }

    if step == nil {
        logger.Error("Step not found in workflow",
            zap.String("step_id", item.StepID))
        e.failExecution(ctx, exec, "step not found: "+item.StepID)
        return
    }

    // Check skip_if condition
    if step.SkipIf != nil && e.evaluateCondition(step.SkipIf, exec) {
        logger.Info("Step skipped by condition",
            zap.String("step_id", step.ID))
        e.advanceToNext(ctx, exec, wf, step, workflow.StepStatusSkipped, logger)
        return
    }

    // Execute based on step type
    now := time.Now()
    result := workflow.StepResult{
        StepID:    step.ID,
        Status:    workflow.StepStatusRunning,
        StartedAt: &now,
    }

    switch step.Type {
    case workflow.StepTypeChannel:
        notifID, err := e.executeChannelStep(ctx, exec, step)
        if err != nil {
            result.Status = workflow.StepStatusFailed
            result.Error = err.Error()
        } else {
            result.Status = workflow.StepStatusCompleted
            result.NotificationID = notifID
        }

    case workflow.StepTypeDelay:
        e.executeDelayStep(ctx, exec, step, item, logger)
        return // Don't advance — the delayed item will re-enter the queue

    case workflow.StepTypeDigest:
        count, err := e.executeDigestStep(ctx, exec, step)
        if err != nil {
            result.Status = workflow.StepStatusFailed
            result.Error = err.Error()
        } else {
            result.Status = workflow.StepStatusCompleted
            result.DigestCount = count
        }

    case workflow.StepTypeCondition:
        // Evaluate condition and advance to either on_success or on_failure
        matched := e.evaluateCondition(step.Config.Condition, exec)
        result.Status = workflow.StepStatusCompleted
        completedAt := time.Now()
        result.CompletedAt = &completedAt
        exec.StepResults[step.ID] = result

        nextStepID := step.OnFailure
        if matched {
            nextStepID = step.OnSuccess
        }
        if nextStepID == "" {
            e.completeExecution(ctx, exec)
            return
        }
        exec.CurrentStepID = nextStepID
        exec.UpdatedAt = time.Now()
        e.workflowRepo.UpdateExecution(ctx, exec)
        e.wfQueue.EnqueueWorkflow(ctx, queue.WorkflowQueueItem{
            ExecutionID: exec.ID,
            StepID:      nextStepID,
            EnqueuedAt:  time.Now(),
        })
        return

    default:
        result.Status = workflow.StepStatusFailed
        result.Error = fmt.Sprintf("unknown step type: %s", step.Type)
    }

    completedAt := time.Now()
    result.CompletedAt = &completedAt
    exec.StepResults[step.ID] = result

    e.advanceToNext(ctx, exec, wf, step, result.Status, logger)
}

// executeChannelStep sends a notification using the existing notification.Service.
// This is the critical backward compatibility point — we reuse Send() which
// goes through the exact same validation, queue, and provider pipeline.
func (e *Engine) executeChannelStep(ctx context.Context, exec *workflow.WorkflowExecution, step *workflow.Step) (string, error) {
    req := notification.SendRequest{
        AppID:      exec.AppID,
        UserID:     exec.UserID,
        Channel:    notification.Channel(step.Config.Channel),
        Priority:   notification.PriorityNormal,
        TemplateID: step.Config.TemplateID,
        Data:       exec.Payload, // Pass workflow payload as template data
    }

    notif, err := e.notifService.Send(ctx, req)
    if err != nil {
        return "", err
    }
    return notif.NotificationID, nil
}

// executeDelayStep schedules re-execution after the configured duration.
func (e *Engine) executeDelayStep(ctx context.Context, exec *workflow.WorkflowExecution, step *workflow.Step, item *queue.WorkflowQueueItem, logger *zap.Logger) {
    d, err := time.ParseDuration(step.Config.Duration)
    if err != nil {
        logger.Error("Invalid delay duration", zap.String("duration", step.Config.Duration))
        e.failExecution(ctx, exec, "invalid delay duration: "+step.Config.Duration)
        return
    }

    // Record the step as running (it's "waiting")
    now := time.Now()
    exec.StepResults[step.ID] = workflow.StepResult{
        StepID:    step.ID,
        Status:    workflow.StepStatusRunning,
        StartedAt: &now,
    }
    exec.UpdatedAt = time.Now()
    e.workflowRepo.UpdateExecution(ctx, exec)

    // Find next step to enqueue after delay
    nextStepID := step.OnSuccess
    if nextStepID == "" {
        // No next step — complete after delay (edge case: delay as final step)
        nextStepID = step.ID // Re-enter same step; the poller will mark complete
    }

    executeAt := time.Now().Add(d)
    e.wfQueue.EnqueueWorkflowDelayed(ctx, queue.WorkflowQueueItem{
        ExecutionID: exec.ID,
        StepID:      nextStepID,
        EnqueuedAt:  time.Now(),
    }, executeAt)

    logger.Info("Delay step scheduled",
        zap.String("execution_id", exec.ID),
        zap.String("step_id", step.ID),
        zap.Duration("delay", d),
        zap.Time("execute_at", executeAt))
}

func (e *Engine) advanceToNext(ctx context.Context, exec *workflow.WorkflowExecution, wf *workflow.Workflow, currentStep *workflow.Step, status workflow.StepStatus, logger *zap.Logger) {
    nextStepID := ""
    if status == workflow.StepStatusCompleted || status == workflow.StepStatusSkipped {
        nextStepID = currentStep.OnSuccess
    } else if status == workflow.StepStatusFailed {
        nextStepID = currentStep.OnFailure
    }

    if nextStepID == "" {
        // No next step — workflow is done
        if status == workflow.StepStatusFailed {
            e.failExecution(ctx, exec, "step failed: "+currentStep.ID)
        } else {
            e.completeExecution(ctx, exec)
        }
        return
    }

    exec.CurrentStepID = nextStepID
    exec.UpdatedAt = time.Now()
    e.workflowRepo.UpdateExecution(ctx, exec)

    e.wfQueue.EnqueueWorkflow(ctx, queue.WorkflowQueueItem{
        ExecutionID: exec.ID,
        StepID:      nextStepID,
        EnqueuedAt:  time.Now(),
    })
}

func (e *Engine) completeExecution(ctx context.Context, exec *workflow.WorkflowExecution) {
    now := time.Now()
    exec.Status = workflow.ExecStatusCompleted
    exec.CompletedAt = &now
    exec.UpdatedAt = now
    e.workflowRepo.UpdateExecution(ctx, exec)
}

func (e *Engine) failExecution(ctx context.Context, exec *workflow.WorkflowExecution, reason string) {
    now := time.Now()
    exec.Status = workflow.ExecStatusFailed
    exec.CompletedAt = &now
    exec.UpdatedAt = now
    e.workflowRepo.UpdateExecution(ctx, exec)
}

func (e *Engine) evaluateCondition(cond *workflow.Condition, exec *workflow.WorkflowExecution) bool {
    if cond == nil {
        return false
    }

    // Resolve field value from payload or step results
    val := e.resolveField(cond.Field, exec)

    switch cond.Operator {
    case workflow.OpEquals:
        return fmt.Sprintf("%v", val) == fmt.Sprintf("%v", cond.Value)
    case workflow.OpNotEquals:
        return fmt.Sprintf("%v", val) != fmt.Sprintf("%v", cond.Value)
    case workflow.OpContains:
        return strings.Contains(fmt.Sprintf("%v", val), fmt.Sprintf("%v", cond.Value))
    case workflow.OpExists:
        return val != nil
    case workflow.OpGreaterThan:
        a, _ := strconv.ParseFloat(fmt.Sprintf("%v", val), 64)
        b, _ := strconv.ParseFloat(fmt.Sprintf("%v", cond.Value), 64)
        return a > b
    case workflow.OpLessThan:
        a, _ := strconv.ParseFloat(fmt.Sprintf("%v", val), 64)
        b, _ := strconv.ParseFloat(fmt.Sprintf("%v", cond.Value), 64)
        return a < b
    case workflow.OpNotRead:
        // Check if a previous step's notification was not read
        stepID := cond.Field // e.g., "steps.step1"
        if result, ok := exec.StepResults[stepID]; ok {
            // Notification was sent but not read
            return result.NotificationID != "" && result.Status == workflow.StepStatusCompleted
        }
        return true // Step not executed → treat as "not read"
    default:
        return false
    }
}

func (e *Engine) resolveField(field string, exec *workflow.WorkflowExecution) any {
    parts := strings.SplitN(field, ".", 2)
    if len(parts) < 2 {
        if v, ok := exec.Payload[field]; ok {
            return v
        }
        return nil
    }

    switch parts[0] {
    case "payload":
        return exec.Payload[parts[1]]
    case "steps":
        stepParts := strings.SplitN(parts[1], ".", 2)
        if result, ok := exec.StepResults[stepParts[0]]; ok {
            if len(stepParts) == 2 {
                switch stepParts[1] {
                case "status":
                    return string(result.Status)
                case "notification_id":
                    return result.NotificationID
                case "error":
                    return result.Error
                }
            }
            return result
        }
    }
    return nil
}

// delayedPoller moves delayed workflow items to the main queue when their time arrives.
func (e *Engine) delayedPoller(ctx context.Context) {
    defer e.wg.Done()
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-e.stopChan:
            return
        case <-ticker.C:
            items, err := e.wfQueue.GetDelayedWorkflowItems(ctx, 100)
            if err != nil {
                e.logger.Error("Failed to get delayed workflow items", zap.Error(err))
                continue
            }
            for _, item := range items {
                if err := e.wfQueue.EnqueueWorkflow(ctx, item); err != nil {
                    e.logger.Error("Failed to re-enqueue delayed workflow item", zap.Error(err))
                }
            }
        }
    }
}

// recovery scans for stale executions and re-enqueues them.
func (e *Engine) recovery(ctx context.Context) {
    defer e.wg.Done()
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-e.stopChan:
            return
        case <-ticker.C:
            staleSince := time.Now().Add(-5 * time.Minute)
            stale, err := e.workflowRepo.GetStaleExecutions(ctx, staleSince, 50)
            if err != nil {
                e.logger.Error("Failed to get stale executions", zap.Error(err))
                continue
            }
            for _, exec := range stale {
                e.logger.Warn("Recovering stale workflow execution",
                    zap.String("execution_id", exec.ID),
                    zap.String("current_step", exec.CurrentStepID))
                e.wfQueue.EnqueueWorkflow(ctx, queue.WorkflowQueueItem{
                    ExecutionID: exec.ID,
                    StepID:      exec.CurrentStepID,
                    EnqueuedAt:  time.Now(),
                })
            }
        }
    }
}
```

### 3.8 HTTP Handler & DTOs

**New file:** `internal/interfaces/http/handlers/workflow_handler.go`

```go
package handlers

import (
    "github.com/gofiber/fiber/v2"
    "github.com/the-monkeys/freerangenotify/internal/domain/workflow"
    "github.com/the-monkeys/freerangenotify/pkg/validator"
    "go.uber.org/zap"
)

type WorkflowHandler struct {
    service   workflow.Service
    validator *validator.Validator
    logger    *zap.Logger
}

func NewWorkflowHandler(service workflow.Service, v *validator.Validator, logger *zap.Logger) *WorkflowHandler {
    return &WorkflowHandler{service: service, validator: v, logger: logger}
}

// CreateWorkflow handles POST /v1/workflows
func (h *WorkflowHandler) Create(c *fiber.Ctx) error { /* ... */ }

// ListWorkflows handles GET /v1/workflows
func (h *WorkflowHandler) List(c *fiber.Ctx) error { /* ... */ }

// GetWorkflow handles GET /v1/workflows/:id
func (h *WorkflowHandler) Get(c *fiber.Ctx) error { /* ... */ }

// UpdateWorkflow handles PUT /v1/workflows/:id
func (h *WorkflowHandler) Update(c *fiber.Ctx) error { /* ... */ }

// DeleteWorkflow handles DELETE /v1/workflows/:id
func (h *WorkflowHandler) Delete(c *fiber.Ctx) error { /* ... */ }

// TriggerWorkflow handles POST /v1/workflows/trigger
func (h *WorkflowHandler) Trigger(c *fiber.Ctx) error {
    appID := c.Locals("app_id").(string)
    var req workflow.TriggerRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
    }
    exec, err := h.service.Trigger(c.Context(), appID, &req)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
    }
    return c.Status(fiber.StatusAccepted).JSON(exec)
}

// ListExecutions handles GET /v1/workflows/executions
func (h *WorkflowHandler) ListExecutions(c *fiber.Ctx) error { /* ... */ }

// GetExecution handles GET /v1/workflows/executions/:id
func (h *WorkflowHandler) GetExecution(c *fiber.Ctx) error { /* ... */ }

// CancelExecution handles POST /v1/workflows/executions/:id/cancel
func (h *WorkflowHandler) CancelExecution(c *fiber.Ctx) error { /* ... */ }
```

### 3.9 Route Registration

**File to modify:** `internal/interfaces/http/routes/routes.go`

Add to `setupProtectedRoutes()`, AFTER all existing registrations:

```go
    // Workflow routes (Phase 1 — gated by feature flag)
    if c.Config.Features.WorkflowEnabled {
        workflows := v1.Group("/workflows")
        workflows.Use(auth)
        workflows.Post("/", c.WorkflowHandler.Create)
        workflows.Get("/", c.WorkflowHandler.List)
        workflows.Post("/trigger", c.WorkflowHandler.Trigger)
        workflows.Get("/executions", c.WorkflowHandler.ListExecutions)
        workflows.Get("/executions/:id", c.WorkflowHandler.GetExecution)
        workflows.Post("/executions/:id/cancel", c.WorkflowHandler.CancelExecution)
        workflows.Get("/:id", c.WorkflowHandler.Get)
        workflows.Put("/:id", c.WorkflowHandler.Update)
        workflows.Delete("/:id", c.WorkflowHandler.Delete)
    }
```

**Backward compatibility:** When `WorkflowEnabled` is `false` (default), these routes are never registered. Existing routes are unaffected.

### 3.10 Container Wiring

**File to modify:** `internal/container/container.go`

Add to `Container` struct:
```go
    // Phase 1: Workflow
    WorkflowService workflow.Service
    WorkflowHandler *handlers.WorkflowHandler
```

Add to `NewContainer()`, AFTER existing handler initialization:
```go
    // Phase 1: Workflow (only wire if enabled)
    if cfg.Features.WorkflowEnabled {
        workflowRepo := repository.NewWorkflowRepository(dbManager.Client.GetClient(), logger)
        wfQueue, ok := container.Queue.(queue.WorkflowQueue)
        if !ok {
            logger.Warn("Queue does not support WorkflowQueue interface — workflow features disabled")
        } else {
            container.WorkflowService = services.NewWorkflowService(workflowRepo, wfQueue, logger)
            container.WorkflowHandler = handlers.NewWorkflowHandler(
                container.WorkflowService, container.Validator, logger,
            )
        }
    }
```

**Backward compatibility:** When the feature flag is off, these fields remain `nil`. No existing initialization is affected.

### 3.11 Worker Integration

**File to modify:** `cmd/worker/main.go`

Add AFTER the notification processor start, BEFORE the shutdown signal wait:

```go
    // Phase 1: Workflow engine (only if enabled)
    var wfEngine *orchestrator.Engine
    if cfg.Features.WorkflowEnabled {
        wfQueue, ok := c.Queue.(queue.WorkflowQueue)
        if ok {
            wfRepo := repository.NewWorkflowRepository(
                c.DatabaseManager.Client.GetClient(), logger,
            )
            wfEngine = orchestrator.NewEngine(
                wfRepo,
                c.NotificationService,
                wfQueue,
                c.RedisClient,
                logger,
                c.Metrics,
                2, // 2 workflow workers by default
            )
            wfEngine.Start(processorCtx)
            logger.Info("Workflow engine started")
        } else {
            logger.Warn("Queue does not support WorkflowQueue — workflow engine not started")
        }
    }
```

Add to shutdown:
```go
    if wfEngine != nil {
        wfEngine.Shutdown(shutdownCtx)
    }
```

**Backward compatibility:** If `WorkflowEnabled` is false or the queue doesn't support `WorkflowQueue`, nothing changes. The existing notification processor runs exactly as before.

### 3.12 API Contract Table

| Method | Path | Auth | Gated | Description |
|--------|------|------|-------|-------------|
| `POST` | `/v1/workflows` | API Key | `workflow_enabled` | Create a workflow |
| `GET` | `/v1/workflows` | API Key | `workflow_enabled` | List workflows |
| `GET` | `/v1/workflows/:id` | API Key | `workflow_enabled` | Get workflow by ID |
| `PUT` | `/v1/workflows/:id` | API Key | `workflow_enabled` | Update workflow |
| `DELETE` | `/v1/workflows/:id` | API Key | `workflow_enabled` | Delete workflow |
| `POST` | `/v1/workflows/trigger` | API Key | `workflow_enabled` | Trigger a workflow execution |
| `GET` | `/v1/workflows/executions` | API Key | `workflow_enabled` | List executions |
| `GET` | `/v1/workflows/executions/:id` | API Key | `workflow_enabled` | Get execution details |
| `POST` | `/v1/workflows/executions/:id/cancel` | API Key | `workflow_enabled` | Cancel execution |

**None of these conflict with any existing route.**

---

## 4. Feature 2: Digest / Batching Engine

### 4.1 Design Principle

The digest engine is **opt-in per notification**. If a notification does not match any active digest rule, it is delivered instantly through the existing pipeline. The digest check is a single `if` block inserted at the top of `processNotification()`.

```
Notification arrives at worker
        │
        ▼
   ┌─────────────────┐  YES   ┌─────────────┐
   │ Matches a digest ├───────►│ Accumulate   │
   │ rule?            │        │ in Redis     │
   └────────┬─────────┘        │ sorted set   │
            │ NO               └──────┬───────┘
            ▼                         │
   ┌─────────────────┐        ┌──────▼───────┐
   │ Existing flow   │        │ Digest flush │
   │ (template,      │        │ (background  │
   │  provider, etc) │        │  goroutine)  │
   └─────────────────┘        └──────────────┘
```

### 4.2 Domain Model

**New file:** `internal/domain/digest/models.go`

```go
package digest

import (
    "context"
    "time"
)

// DigestRule defines how notifications should be batched before delivery.
type DigestRule struct {
    ID         string    `json:"id"`
    AppID      string    `json:"app_id"`
    Name       string    `json:"name"`
    DigestKey  string    `json:"digest_key"`   // Field from notification metadata to group by
    Window     string    `json:"window"`        // "1h", "24h"
    Channel    string    `json:"channel"`       // Delivery channel for the digest summary
    TemplateID string    `json:"template_id"`   // Template with {{.Events}} variable
    MaxBatch   int       `json:"max_batch"`     // Max events per digest (0 = unlimited)
    Status     string    `json:"status"`        // active, inactive
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
}

// Repository for digest rules.
type Repository interface {
    Create(ctx context.Context, rule *DigestRule) error
    GetByID(ctx context.Context, id string) (*DigestRule, error)
    GetActiveByKey(ctx context.Context, appID, digestKey string) (*DigestRule, error)
    List(ctx context.Context, appID string, limit, offset int) ([]*DigestRule, int64, error)
    Update(ctx context.Context, rule *DigestRule) error
    Delete(ctx context.Context, id string) error
}

// Service for digest operations.
type Service interface {
    Create(ctx context.Context, appID string, req *CreateRequest) (*DigestRule, error)
    Get(ctx context.Context, id, appID string) (*DigestRule, error)
    List(ctx context.Context, appID string, limit, offset int) ([]*DigestRule, int64, error)
    Update(ctx context.Context, id, appID string, req *UpdateRequest) (*DigestRule, error)
    Delete(ctx context.Context, id, appID string) error
}

type CreateRequest struct {
    Name       string `json:"name" validate:"required,min=3,max=100"`
    DigestKey  string `json:"digest_key" validate:"required,min=1,max=100"`
    Window     string `json:"window" validate:"required"`   // "1h", "6h", "24h"
    Channel    string `json:"channel" validate:"required,oneof=push email sms webhook in_app sse"`
    TemplateID string `json:"template_id" validate:"required"`
    MaxBatch   int    `json:"max_batch,omitempty"`
}

type UpdateRequest struct {
    Name       *string `json:"name,omitempty" validate:"omitempty,min=3,max=100"`
    Window     *string `json:"window,omitempty"`
    Channel    *string `json:"channel,omitempty" validate:"omitempty,oneof=push email sms webhook in_app sse"`
    TemplateID *string `json:"template_id,omitempty"`
    MaxBatch   *int    `json:"max_batch,omitempty"`
    Status     *string `json:"status,omitempty" validate:"omitempty,oneof=active inactive"`
}
```

### 4.3 ES Index Template

**Add to** `index_templates.go`:

```go
func (it *IndexTemplates) GetDigestRulesTemplate() map[string]interface{} {
    return map[string]interface{}{
        "settings": map[string]interface{}{
            "number_of_shards":   1,
            "number_of_replicas": 0,
        },
        "mappings": map[string]interface{}{
            "properties": map[string]interface{}{
                "digest_rule_id": map[string]interface{}{"type": "keyword"},
                "app_id":         map[string]interface{}{"type": "keyword"},
                "name": map[string]interface{}{
                    "type": "text",
                    "fields": map[string]interface{}{
                        "keyword": map[string]interface{}{"type": "keyword"},
                    },
                },
                "digest_key":  map[string]interface{}{"type": "keyword"},
                "window":      map[string]interface{}{"type": "keyword"},
                "channel":     map[string]interface{}{"type": "keyword"},
                "template_id": map[string]interface{}{"type": "keyword"},
                "max_batch":   map[string]interface{}{"type": "integer"},
                "status":      map[string]interface{}{"type": "keyword"},
                "created_at":  map[string]interface{}{"type": "date"},
                "updated_at":  map[string]interface{}{"type": "date"},
            },
        },
    }
}
```

### 4.4 Digest Manager (Worker Side)

**New file:** `internal/infrastructure/orchestrator/digest.go`

```go
package orchestrator

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "time"

    "github.com/go-redis/redis/v8"
    "github.com/the-monkeys/freerangenotify/internal/domain/digest"
    "github.com/the-monkeys/freerangenotify/internal/domain/notification"
    "go.uber.org/zap"
)

// Redis key patterns for digest
const (
    digestKeyPrefix = "frn:digest:"    // frn:digest:{app_id}:{user_id}:{digest_key_value}
    digestFlushKey  = "frn:digest:flush"
)

// DigestManager manages notification digesting using Redis sorted sets.
type DigestManager struct {
    digestRepo   digest.Repository
    notifService notification.Service
    redisClient  *redis.Client
    logger       *zap.Logger

    wg       sync.WaitGroup
    stopChan chan struct{}
}

func NewDigestManager(
    digestRepo digest.Repository,
    notifService notification.Service,
    redisClient *redis.Client,
    logger *zap.Logger,
) *DigestManager {
    return &DigestManager{
        digestRepo:   digestRepo,
        notifService: notifService,
        redisClient:  redisClient,
        logger:       logger,
        stopChan:     make(chan struct{}),
    }
}

// MatchesDigestRule checks if a notification should be digested.
// Returns the matching rule or nil if no rule matches.
func (dm *DigestManager) MatchesDigestRule(ctx context.Context, notif *notification.Notification) (*digest.DigestRule, string) {
    if notif.Metadata == nil {
        return nil, ""
    }
    digestKey, ok := notif.Metadata["digest_key"]
    if !ok {
        return nil, ""
    }

    keyStr, ok := digestKey.(string)
    if !ok || keyStr == "" {
        return nil, ""
    }

    rule, err := dm.digestRepo.GetActiveByKey(ctx, notif.AppID, keyStr)
    if err != nil || rule == nil {
        return nil, ""
    }

    return rule, keyStr
}

// Accumulate adds a notification payload to the digest accumulator.
func (dm *DigestManager) Accumulate(ctx context.Context, notif *notification.Notification, rule *digest.DigestRule, digestKeyValue string) error {
    // Determine the grouping value from the notification metadata
    groupVal := ""
    if v, ok := notif.Metadata[rule.DigestKey]; ok {
        groupVal = fmt.Sprintf("%v", v)
    }

    redisKey := fmt.Sprintf("%s%s:%s:%s", digestKeyPrefix, notif.AppID, notif.UserID, groupVal)

    // Serialize notification payload for accumulation
    payload := map[string]interface{}{
        "notification_id": notif.NotificationID,
        "title":           notif.Content.Title,
        "body":            notif.Content.Body,
        "data":            notif.Content.Data,
        "category":        notif.Category,
        "created_at":      notif.CreatedAt.Format(time.RFC3339),
    }
    data, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("failed to marshal digest payload: %w", err)
    }

    // Add to sorted set (score = timestamp)
    if err := dm.redisClient.ZAdd(ctx, redisKey, &redis.Z{
        Score:  float64(time.Now().Unix()),
        Member: string(data),
    }).Err(); err != nil {
        return fmt.Errorf("failed to accumulate digest event: %w", err)
    }

    // Schedule flush if not already scheduled
    exists, _ := dm.redisClient.ZScore(ctx, digestFlushKey, redisKey).Result()
    if exists == 0 {
        window, err := time.ParseDuration(rule.Window)
        if err != nil {
            window = 1 * time.Hour // Safe default
        }
        flushAt := time.Now().Add(window)
        dm.redisClient.ZAdd(ctx, digestFlushKey, &redis.Z{
            Score:  float64(flushAt.Unix()),
            Member: redisKey,
        })
    }

    // Update notification status to "digesting" (a conceptual status — we use "queued"
    // to keep backward compatibility with the Status enum)
    // Actually, we mark as "sent" since it's "accepted into the digest pipeline"
    dm.logger.Info("Notification accumulated into digest",
        zap.String("notification_id", notif.NotificationID),
        zap.String("digest_key", redisKey))

    return nil
}

// StartFlushPoller starts the background goroutine that flushes mature digests.
func (dm *DigestManager) StartFlushPoller(ctx context.Context) {
    dm.wg.Add(1)
    go dm.flushPoller(ctx)
}

func (dm *DigestManager) flushPoller(ctx context.Context) {
    defer dm.wg.Done()
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-dm.stopChan:
            return
        case <-ticker.C:
            dm.flushReady(ctx)
        }
    }
}

func (dm *DigestManager) flushReady(ctx context.Context) {
    now := float64(time.Now().Unix())
    results, err := dm.redisClient.ZRangeByScore(ctx, digestFlushKey, &redis.ZRangeBy{
        Min:   "-inf",
        Max:   fmt.Sprintf("%f", now),
        Count: 50,
    }).Result()
    if err != nil {
        return
    }

    for _, redisKey := range results {
        if err := dm.flushOneDigest(ctx, redisKey); err != nil {
            dm.logger.Error("Failed to flush digest",
                zap.String("key", redisKey),
                zap.Error(err))
            continue
        }
        // Remove from flush schedule
        dm.redisClient.ZRem(ctx, digestFlushKey, redisKey)
    }
}

func (dm *DigestManager) flushOneDigest(ctx context.Context, redisKey string) error {
    // Get all accumulated events
    results, err := dm.redisClient.ZRangeByScore(ctx, redisKey, &redis.ZRangeBy{
        Min: "-inf",
        Max: "+inf",
    }).Result()
    if err != nil {
        return err
    }

    if len(results) == 0 {
        return nil // Nothing to flush
    }

    // Parse events
    var events []map[string]interface{}
    for _, r := range results {
        var event map[string]interface{}
        if err := json.Unmarshal([]byte(r), &event); err != nil {
            continue
        }
        events = append(events, event)
    }

    // TODO: Parse the redis key to extract app_id, user_id, groupVal
    // Then look up the digest rule, render the digest template, and send
    // a single consolidated notification using notifService.Send()

    dm.logger.Info("Digest flushed",
        zap.String("key", redisKey),
        zap.Int("event_count", len(events)))

    // Clear the accumulator
    dm.redisClient.Del(ctx, redisKey)

    return nil
}

func (dm *DigestManager) Shutdown() {
    close(dm.stopChan)
    dm.wg.Wait()
}
```

### 4.5 Processor Modification — The Only Change to Existing Code

**File to modify:** `cmd/worker/processor.go`

This is the **single change** to existing production code. It adds an optional digest check at the top of `processNotification()`.

**Current code (unchanged):**
```go
func (p *NotificationProcessor) processNotification(ctx context.Context, item *queue.NotificationQueueItem, logger *zap.Logger) {
    startTime := time.Now()
    // ... existing code
```

**Modified code:**
```go
// Add a new field to NotificationProcessor struct:
type NotificationProcessor struct {
    // ... existing fields (all unchanged)
    digestManager *orchestrator.DigestManager // Phase 1: optional, nil when disabled
}

// Add setter (NOT constructor change — backward compatible)
func (p *NotificationProcessor) SetDigestManager(dm *orchestrator.DigestManager) {
    p.digestManager = dm
}

// In processNotification, add check AFTER fetching notification from ES,
// BEFORE updating status to processing:
func (p *NotificationProcessor) processNotification(ctx context.Context, item *queue.NotificationQueueItem, logger *zap.Logger) {
    startTime := time.Now()

    // ... existing: log, metrics, get notification from DB ...

    // ── Phase 1: Digest check (optional, no-op when digestManager is nil) ──
    if p.digestManager != nil {
        rule, keyValue := p.digestManager.MatchesDigestRule(ctx, notif)
        if rule != nil {
            if err := p.digestManager.Accumulate(ctx, notif, rule, keyValue); err != nil {
                logger.Error("Failed to accumulate digest, falling through to normal delivery",
                    zap.Error(err))
            } else {
                // Notification was accumulated — mark as "sent" (into digest)
                p.notifRepo.UpdateStatus(ctx, notif.NotificationID, notification.StatusSent)
                p.publishActivity(ctx, notif.NotificationID, string(notif.Channel), "digested")
                return
            }
        }
    }

    // ... rest of existing processNotification code (completely unchanged) ...
```

**Why this is safe:**

1. `digestManager` is `nil` by default (zero value of pointer). The `if p.digestManager != nil` check short-circuits. When digest is disabled, this is a single nil comparison — essentially zero overhead.
2. No existing constructor signature changes. `NewNotificationProcessor()` keeps its exact parameter list. The digestManager is injected via `SetDigestManager()` after construction.
3. If `MatchesDigestRule()` returns nil (no matching rule), the existing code path continues unchanged.
4. If `Accumulate()` fails, it falls through to normal delivery — fail-open, not fail-closed.

### 4.6 Digest API Routes

**Add to** `setupProtectedRoutes()` in routes.go:

```go
    // Digest rules (Phase 1 — gated by feature flag)
    if c.Config.Features.DigestEnabled {
        digestRules := v1.Group("/digest-rules")
        digestRules.Use(auth)
        digestRules.Post("/", c.DigestHandler.Create)
        digestRules.Get("/", c.DigestHandler.List)
        digestRules.Get("/:id", c.DigestHandler.Get)
        digestRules.Put("/:id", c.DigestHandler.Update)
        digestRules.Delete("/:id", c.DigestHandler.Delete)
    }
```

### 4.7 Container Wiring

Add to `Container` struct:
```go
    // Phase 1: Digest
    DigestService digest.Service
    DigestHandler *handlers.DigestHandler
```

Add to `NewContainer()`:
```go
    if cfg.Features.DigestEnabled {
        digestRepo := repository.NewDigestRepository(dbManager.Client.GetClient(), logger)
        container.DigestService = services.NewDigestService(digestRepo, logger)
        container.DigestHandler = handlers.NewDigestHandler(
            container.DigestService, container.Validator, logger,
        )
    }
```

### 4.8 Worker Integration for Digest

**Add to** `cmd/worker/main.go`, AFTER processor creation:

```go
    // Phase 1: Digest manager (only if enabled)
    if cfg.Features.DigestEnabled {
        digestRepo := repository.NewDigestRepository(
            c.DatabaseManager.Client.GetClient(), logger,
        )
        digestMgr := orchestrator.NewDigestManager(
            digestRepo,
            c.NotificationService,
            c.RedisClient,
            logger,
        )
        processor.SetDigestManager(digestMgr)
        digestMgr.StartFlushPoller(processorCtx)
        logger.Info("Digest manager started")
    }
```

---

## 5. Feature 3: HMAC for SSE / Inbox

### 5.1 Design Principle

HMAC is **opt-in, not opt-out**. The default behavior (no hash required) is unchanged. Only apps that explicitly enable HMAC enforcement get the stricter check.

```
Client connects: GET /v1/sse?user_id=xxx&token=frn_yyy

Is SSE HMAC enforced globally?
├── NO  → Existing behavior (token-based auth)
└── YES → Is hash query param present?
    ├── NO  → 401 Unauthorized
    └── YES → Validate HMAC(user_id, api_key_secret) == hash
        ├── VALID  → Proceed to SSE
        └── INVALID → 401 Unauthorized
```

### 5.2 HMAC Utility

**New file:** `pkg/utils/hmac.go`

```go
package utils

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
)

// GenerateSubscriberHash creates an HMAC-SHA256 hash of the user ID
// using the application's API key as the secret.
func GenerateSubscriberHash(userID, apiKey string) string {
    mac := hmac.New(sha256.New, []byte(apiKey))
    mac.Write([]byte(userID))
    return hex.EncodeToString(mac.Sum(nil))
}

// ValidateSubscriberHash verifies that the provided hash matches
// the expected HMAC for the given user ID and API key.
func ValidateSubscriberHash(userID, apiKey, hash string) bool {
    expected := GenerateSubscriberHash(userID, apiKey)
    return hmac.Equal([]byte(expected), []byte(hash))
}
```

### 5.3 SSE Handler Modification

**File to modify:** `internal/interfaces/http/handlers/sse_handler.go`

The change adds an optional HMAC check AFTER the existing token validation. When HMAC is not enforced (default), the behavior is identical to today.

**Current code block (Connect method, after token validation):**
```go
    // ── 2b. Resolve user_id: if not a UUID, try external_id lookup ──
```

**Insert BEFORE this block:**
```go
    // ── 2a. HMAC validation (opt-in via feature flag) ──
    if h.hmacEnforced {
        hash := c.Query("hash")
        if hash == "" {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
                "error": "subscriber hash is required when HMAC is enforced",
            })
        }
        // apiKey is the raw key from the token used above, or from the app lookup
        apiKey := ""
        if token != "" {
            apiKey = token // The token IS the API key
        }
        if apiKey == "" {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
                "error": "token is required when HMAC is enforced",
            })
        }
        if !utils.ValidateSubscriberHash(userID, apiKey, hash) {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
                "error": "invalid subscriber hash",
            })
        }
    }
```

**SSEHandler struct modification:**
```go
type SSEHandler struct {
    broadcaster  *sse.Broadcaster
    appService   usecases.ApplicationService
    notifService notification.Service
    userRepo     user.Repository
    redisClient  *redis.Client
    hmacEnforced bool   // Phase 1: default false
    logger       *zap.Logger
}
```

**Constructor modification:**
```go
func NewSSEHandler(
    broadcaster *sse.Broadcaster,
    appService usecases.ApplicationService,
    notifService notification.Service,
    userRepo user.Repository,
    redisClient *redis.Client,
    logger *zap.Logger,
) *SSEHandler {
    return &SSEHandler{
        broadcaster:  broadcaster,
        appService:   appService,
        notifService: notifService,
        userRepo:     userRepo,
        redisClient:  redisClient,
        hmacEnforced: false, // Default OFF
        logger:       logger,
    }
}

// SetHMACEnforced enables/disables HMAC enforcement. Called from container wiring.
func (h *SSEHandler) SetHMACEnforced(enforced bool) {
    h.hmacEnforced = enforced
}
```

**Backward compatibility:**
- Constructor signature is **unchanged** (6 params, same types, same order).
- `hmacEnforced` defaults to `false` — existing SSE connections work without `hash`.
- The `SetHMACEnforced()` setter is called from `NewContainer()` only when the feature flag is on.
- Clients that already pass `token=` continue to work.
- The `hash` param is optional and ignored when enforcement is off.

### 5.4 Container Wiring

**Add to** `NewContainer()`, AFTER SSEHandler creation:
```go
    if cfg.Features.SSEHMACEnforced {
        container.SSEHandler.SetHMACEnforced(true)
    }
```

### 5.5 SDK Changes

**File to modify:** `sdk/js/src/index.ts`

Add `subscriberHash` to `connectSSE`:
```typescript
// New optional parameter — not required, no breaking change
connectSSE(userId: string, options: SSEConnectionOptions & { subscriberHash?: string }): SSEConnection {
    // ...existing URL building...
    if (options.subscriberHash) {
        url += `&hash=${encodeURIComponent(options.subscriberHash)}`;
    }
    // ...rest unchanged...
}
```

**File to modify:** `sdk/react/src/index.tsx`

Add `subscriberHash` to `NotificationBellProps`:
```typescript
export interface NotificationBellProps {
    // ...existing props unchanged...
    /** HMAC-SHA256 hash of userId for SSE authentication. Optional. */
    subscriberHash?: string;
}
```

In the `useEffect` SSE connection:
```typescript
const url = `${baseURL}/v1/sse?user_id=${encodeURIComponent(userId)}${apiKey ? `&token=${encodeURIComponent(apiKey)}` : ''}${subscriberHash ? `&hash=${encodeURIComponent(subscriberHash)}` : ''}`;
```

**Backward compatibility:** Both are optional props with no default. Existing code does not pass them and behavior is unchanged.

### 5.6 API Endpoint for Generating Hash

**New endpoint** (protected, behind API key auth):

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/v1/users/:id/subscriber-hash` | API Key | Generate HMAC hash for SSE |

This endpoint is always available (not gated) so clients can start generating hashes before HMAC enforcement is turned on.

**Add to** `user_handler.go`:
```go
func (h *UserHandler) GetSubscriberHash(c *fiber.Ctx) error {
    userID := c.Params("id")
    apiKey := c.Locals("api_key").(string)
    hash := utils.GenerateSubscriberHash(userID, apiKey)
    return c.JSON(fiber.Map{
        "user_id": userID,
        "hash":    hash,
    })
}
```

**Add to** routes.go in the users group:
```go
    users.Get("/:id/subscriber-hash", c.UserHandler.GetSubscriberHash)
```

---

## 6. Deployment Sequence

```
Day 0: Pre-deployment
├── Run unit tests: go test -v -short ./...
├── Run integration tests: make test-integration
└── Build new Docker images

Day 1: Deploy with features OFF (zero risk)
├── docker-compose down
├── docker-compose up -d  (new images, old config)
├── Run migrate (creates new ES indices alongside existing ones)
├── Smoke test ALL existing endpoints:
│   ├── POST /v1/notifications (send)
│   ├── POST /v1/quick-send
│   ├── POST /v1/notifications/broadcast
│   ├── GET  /v1/sse?user_id=xxx
│   ├── GET  /v1/notifications (list)
│   ├── POST /v1/users
│   └── GET  /v1/health
├── Verify: new endpoints return 404 (not registered)
└── Verify: worker processes notifications identically

Day 2: Enable Workflow Engine
├── Set FREERANGE_FEATURES_WORKFLOW_ENABLED=true
├── Restart services
├── Test: POST /v1/workflows (create)
├── Test: POST /v1/workflows/trigger (trigger)
├── Test: GET /v1/workflows/executions/:id (check results)
└── Verify: existing notification flow still works

Day 3: Enable Digest Engine
├── Set FREERANGE_FEATURES_DIGEST_ENABLED=true
├── Restart services
├── Test: POST /v1/digest-rules (create rule)
├── Test: Send notification with digest_key in metadata
├── Verify: notification is accumulated, not sent
├── Wait for window → verify digest notification sent
└── Verify: notifications WITHOUT digest_key still deliver instantly

Day 4+: Enable HMAC (when ready)
├── Set FREERANGE_FEATURES_SSE_HMAC_ENFORCED=true
├── Update client apps to pass subscriberHash
├── Restart services
└── Verify: SSE without hash returns 401
```

---

## 7. Rollback Plan

Each feature can be independently disabled by setting its flag to `false` and restarting. No database rollback, no data migration, no schema change.

| Scenario | Action | Data Impact |
|----------|--------|-------------|
| Workflow engine has a bug | Set `workflow_enabled=false`, restart | Existing workflow executions remain in ES but are not processed. Resume by re-enabling. |
| Digest causes unexpected behavior | Set `digest_enabled=false`, restart | Accumulated digests in Redis expire naturally (TTL). Notifications resume instant delivery. |
| HMAC breaks SSE clients | Set `sse_hmac_enforced=false`, restart | SSE connections work without hash again. No data impact. |
| Full Phase 1 rollback | Set all three to false | System behaves identically to pre-Phase-1. New ES indices remain but are unused. |

**Rollback does NOT require:**
- Database reindexing
- Redis key cleanup (keys have implicit expiry or are harmless)
- Client-side changes (SDKs handle missing features gracefully)
- Code revert (all changes are behind flags)

---

## 8. Testing Strategy

### 8.1 Unit Tests

| Package | What to Test | File |
|---------|-------------|------|
| `domain/workflow` | Model validation, step ordering | `models_test.go` |
| `domain/digest` | Rule validation | `models_test.go` |
| `orchestrator` | Condition evaluation, step advancement | `engine_test.go` |
| `orchestrator` | Digest accumulation, flush logic | `digest_test.go` |
| `pkg/utils` | HMAC generation and validation | `hmac_test.go` |
| `handlers` | Workflow handler HTTP parsing | `workflow_handler_test.go` |

### 8.2 Integration Tests

| Test | Description |
|------|-------------|
| **Workflow E2E** | Create workflow → trigger → verify execution steps → check notification was sent via existing pipeline |
| **Digest E2E** | Create digest rule → send 3 notifications with matching `digest_key` → wait for window → verify single digest notification delivered |
| **HMAC SSE** | Enable HMAC → connect SSE without hash → expect 401 → connect with valid hash → expect 200 |
| **Backward compat: features OFF** | Deploy with all flags false → run entire existing test suite → verify zero diff |
| **Backward compat: features ON** | Deploy with all flags true → run entire existing test suite → verify zero diff (existing tests don't use workflows/digests) |

### 8.3 Backward Compatibility Test Matrix

| Existing Feature | With Phase 1 OFF | With Phase 1 ON | Status |
|-----------------|-----------------|----------------|--------|
| `POST /v1/notifications` | Identical | Identical (unless `digest_key` in metadata) | Must pass |
| `POST /v1/notifications/broadcast` | Identical | Identical | Must pass |
| `POST /v1/quick-send` | Identical | Identical | Must pass |
| `GET /v1/sse?user_id=xxx` | Identical | Identical (hash not required unless HMAC enforced) | Must pass |
| `<NotificationBell>` React component | Identical | Identical (no `subscriberHash` prop) | Must pass |
| `FreeRangeNotify.connectSSE()` JS SDK | Identical | Identical (no `subscriberHash` option) | Must pass |
| Worker notification processing | Identical | Identical (digestManager is nil) | Must pass |
| Worker retry/DLQ processing | Identical | Identical | Must pass |
| Redis queue keys | Unchanged | Unchanged + new `frn:queue:workflow`, `frn:digest:*` | Must pass |
| ES indices | Unchanged | Unchanged + new `workflows`, `workflow_executions`, `digest_rules` | Must pass |

---

## Appendix A: Complete File Inventory

### New Files (13)

| File | Description |
|------|-------------|
| `internal/domain/workflow/models.go` | Workflow, Step, WorkflowExecution, StepResult types |
| `internal/domain/workflow/repository.go` | Repository interface |
| `internal/domain/workflow/service.go` | Service interface, CreateRequest, TriggerRequest |
| `internal/domain/digest/models.go` | DigestRule, Repository, Service interfaces |
| `internal/infrastructure/repository/workflow_repository.go` | ES implementation of workflow.Repository |
| `internal/infrastructure/repository/digest_repository.go` | ES implementation of digest.Repository |
| `internal/infrastructure/orchestrator/engine.go` | Workflow step execution state machine |
| `internal/infrastructure/orchestrator/digest.go` | Digest accumulation and flush logic |
| `internal/infrastructure/queue/workflow_queue.go` | WorkflowQueue interface, WorkflowQueueItem type |
| `internal/usecases/services/workflow_service_impl.go` | workflow.Service implementation |
| `internal/usecases/services/digest_service_impl.go` | digest.Service implementation |
| `internal/interfaces/http/handlers/workflow_handler.go` | HTTP handlers for workflow API |
| `internal/interfaces/http/handlers/digest_handler.go` | HTTP handlers for digest rules API |
| `pkg/utils/hmac.go` | HMAC hash generation and validation |

### Modified Files (7)

| File | Change | Risk |
|------|--------|------|
| `internal/config/config.go` | Add `FeaturesConfig` struct | **None** — additive struct, no existing field changed |
| `config/config.yaml` | Add `features:` section (all false) | **None** — new keys with safe defaults |
| `internal/infrastructure/database/index_templates.go` | Add 3 new template methods | **None** — new methods, existing ones untouched |
| `internal/infrastructure/database/index_manager.go` | Add 3 indices to `CreateIndices()` map | **None** — `createIndex` is idempotent |
| `internal/infrastructure/queue/redis_queue.go` | Add 4 new methods for workflow queue | **None** — new methods, existing ones untouched |
| `internal/container/container.go` | Add workflow/digest fields, conditional init | **None** — additive fields, existing init untouched |
| `internal/interfaces/http/routes/routes.go` | Add workflow/digest route groups (gated) | **None** — new groups, existing routes untouched |
| `cmd/worker/processor.go` | Add `digestManager` field + digest check | **Low** — single nil check at top of processNotification; nil = no-op |
| `cmd/worker/main.go` | Start workflow engine + digest manager | **None** — conditional startup after existing processor |
| `internal/interfaces/http/handlers/sse_handler.go` | Add `hmacEnforced` field + HMAC check | **None** — defaults to false, setter pattern |
| `sdk/js/src/index.ts` | Add optional `subscriberHash` param | **None** — optional, no default behavior change |
| `sdk/react/src/index.tsx` | Add optional `subscriberHash` prop | **None** — optional, no default behavior change |

### Files NOT Modified (Backward Compatibility Proof)

These critical files are explicitly **not touched**:

- `internal/domain/notification/models.go` — All types, interfaces unchanged
- `internal/domain/user/models.go` — All types unchanged
- `internal/domain/template/models.go` — All types unchanged
- `internal/domain/application/models.go` — All types unchanged
- `internal/infrastructure/providers/provider.go` — Provider interface unchanged
- `internal/infrastructure/providers/manager.go` — Manager unchanged
- `internal/infrastructure/queue/queue.go` — Queue interface unchanged
- `internal/usecases/notification_service.go` — Service implementation unchanged
- `internal/interfaces/http/dto/notification_dto.go` — All DTOs unchanged
- `internal/interfaces/http/handlers/notification_handler.go` — Handler unchanged
- All existing repository implementations — Unchanged
- All existing provider implementations — Unchanged
