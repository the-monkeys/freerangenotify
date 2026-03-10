Procvkflow Integration Plan

A stepwise plan to add all workflow trigger integrations: trigger-by-topic, broadcast→workflow, send→workflow, topic-subscribe, user-created, scheduled, and inbound webhooks. Designed to be channel-agnostic (email, push, SMS, webhook, SSE) and incremental.

---

## Design Principles

1. **Single primitive**: All integrations ultimately call `workflow.Service.Trigger(appID, TriggerRequest)`. No changes to the workflow engine itself.

2. **Channel-agnostic**: Workflows already support all channels via channel steps. New integrations only control *who* gets triggered; the workflow steps determine *what* is sent (email, push, etc.).

3. **Incremental**: Each phase is independently shippable, testable, and backward-compatible.

4. **Shared helper**: Introduce `TriggerForUsers(ctx, appID, triggerID, userIDs []string, payload)` internally to avoid duplication when fanning out.

---

## Shared Helper (Internal)

**Location**: `internal/usecases/services/workflow_service_impl.go`

```go
// TriggerForUsers triggers a workflow for each user. Returns execution IDs and first error if any.
func (s *workflowService) TriggerForUsers(ctx context.Context, appID, triggerID string, userIDs []string, payload map[string]any) ([]string, error)
```

- Loops over `userIDs`, calls `Trigger` for each.
- Returns slice of execution IDs (one per user) and the first error encountered.
- Continues on per-user errors (log and skip) or aborts on workflow-not-found (return early).
- Used by Phase 1 (trigger-by-topic), Phase 2 (broadcast→workflow), Phase 4 (topic subscribe).

---

## Phase 1: Trigger-by-Topic ✅ Implemented

**Goal**: `POST /v1/workflows/trigger-by-topic` — trigger workflow for all subscribers of a topic.

**Request**:
```json
{
  "trigger_id": "product_launch",
  "topic_id": "uuid",
  "payload": { "product": "v2", "launch_date": "2025-04-01" }
}
```

**Response**:
```json
{
  "success": true,
  "triggered": 42,
  "execution_ids": ["exec-1", "exec-2", ...]
}
```

**Implementation**:
- New handler `WorkflowHandler.TriggerByTopic`
- New route `POST /workflows/trigger-by-topic`
- Workflow service: `TriggerByTopic(ctx, appID, triggerID, topicID, payload)` → get topic subscribers via `topicService.GetSubscriberUserIDs`, validate topic belongs to app → call `TriggerForUsers`

**Files**:
- `internal/domain/workflow/service.go` — add `TriggerByTopic` to interface
- `internal/usecases/services/workflow_service_impl.go` — add `TriggerForUsers`, `TriggerByTopic`
- `internal/interfaces/http/handlers/workflow_handler.go` — `TriggerByTopic`
- `internal/interfaces/http/routes/routes.go` — new route
- `ui/src/services/api.ts` — `workflowsAPI.triggerByTopic`
- Workflow Builder: "Test Trigger by Topic" section (optional, Phase 1.1)

**Estimate**: 0.5 day

**Implementation** (done):
- `TriggerForUsers` helper in workflow service
- `TriggerByTopic` on workflow service with setter-injected topic service
- Handler `POST /workflows/trigger-by-topic`
- UI: "Test Trigger by Topic" section in Workflow Builder with topic picker

---

## Phase 2: Broadcast → Workflow

**Goal**: When broadcasting, optionally trigger a workflow for each recipient instead of sending a single notification.

**Request change** (`POST /v1/notifications/broadcast`):
```json
{
  "template_id": "product_announcement",
  "channel": "email",
  "topic_key": "product-updates",
  "workflow_trigger_id": "product_launch_series"
}
```

- If `workflow_trigger_id` is set: for each recipient (from topic if `topic_key` set, else all app users), **trigger workflow** instead of sending one notification.
- If not set: current behavior (single notification to all).
- Supports `topic_key` for broadcast: send/trigger only to topic subscribers. (Currently broadcast sends to all users; this phase can add topic support to broadcast as well.)

**Implementation**:
- Extend `BroadcastRequest` with `WorkflowTriggerID`, `TopicKey` (optional)
- In `NotificationService.Broadcast`:
  - If `WorkflowTriggerID != ""`: resolve recipients (topic subscribers or all users), inject `WorkflowService`, call `TriggerForUsers`. Return summary (no per-notification IDs).
  - Else: current logic.
- Broadcast stays channel-agnostic: workflow steps define channels.

**Files**:
- `internal/domain/notification/models.go` — extend `BroadcastRequest`
- `internal/usecases/notification_service.go` — inject `WorkflowService`, branch on `WorkflowTriggerID`
- `internal/container/container.go` — wire `WorkflowService` into `NotificationService` if not already
- `ui/src/components/AppNotifications.tsx` — add workflow trigger dropdown to broadcast form

**Estimate**: 0.5 day

**Implementation** (done):
- `BroadcastRequest`: added `WorkflowTriggerID`, `TopicKey`; `BroadcastResult` for workflow path
- `workflow.Service`: added `TriggerForUserIDs(ctx, appID, triggerID, userIDs, payload)`
- `NotificationService.Broadcast`: when `WorkflowTriggerID` set, resolve recipients (topic or all users), call `TriggerForUserIDs`; when `TopicKey` set, use topic subscribers
- DTO + handler: map new fields; response uses `result.Triggered` for workflow path
- UI: optional workflow picker and topic picker in broadcast form

---

## Phase 3: Send → Workflow (After-Send Hook) ✅ Implemented

**Goal**: Optional workflow trigger when a single notification is successfully sent.

**Request change** (`POST /v1/notifications`):
```json
{
  "user_id": "uuid",
  "channel": "email",
  "template_id": "welcome_email",
  "workflow_trigger_id": "onboarding_series"
}
```

- After `Send` succeeds, if `workflow_trigger_id` is set, trigger that workflow for the same user with the same payload (or minimal payload).

**Implementation**:
- Extend `SendRequest` with `WorkflowTriggerId` (optional)
- In `NotificationService.Send`, after enqueue/success: if set, call `WorkflowService.Trigger` (fire-and-forget, don’t fail send on trigger error)

**Files**:
- `internal/domain/notification/models.go` — extend `SendRequest`
- `internal/usecases/notification_service.go` — inject workflow service, call trigger after send
- API docs, SDK types

**Estimate**: 0.25 day

**Implementation** (done):
- SendRequest: added `WorkflowTriggerID`; DTO + handler map it
- NotificationService.Send: after enqueue success, fire-and-forget goroutine calls Trigger for same user
- UI: optional workflow picker in Send form (AppNotifications)

---

## Phase 4: Topic Subscribe Event ✅ Implemented

**Goal**: Auto-trigger a workflow when a user subscribes to a topic.

**New field on Topic**: `on_subscribe_trigger_id` (optional). When adding subscribers, if set, trigger that workflow for each **new** subscriber.

**Implementation**:
- Add `OnSubscribeTriggerID string` to `topic.Topic` and `topic.UpdateRequest`
- In `topicService.AddSubscribers`: after adding, for each userID, call `WorkflowService.Trigger` if `OnSubscribeTriggerID != ""`. Validate topic belongs to app; workflow must exist and be active.
- Topic service needs `WorkflowService` injected.

**Files**:
- `internal/domain/topic/models.go` — add `OnSubscribeTriggerID`
- `internal/usecases/services/topic_service_impl.go` — inject workflow service, trigger on add
- `internal/container/container.go` — wire
- UI: Topics list/detail — add optional "On Subscribe Workflow" picker

**Estimate**: 0.5 day

---

## Phase 5: User Lifecycle (On User Created)

**Goal**: Auto-trigger a workflow when a new user is created.

**New field on App Settings**: `on_user_created_trigger_id` (optional).

**Implementation**:
- Add `OnUserCreatedTriggerID string` to `application.Settings`
- In `UserService.Create` (or equivalent): after creating user, if app settings have `OnUserCreatedTriggerID`, call `WorkflowService.Trigger` for the new user.
- User service needs `WorkflowService` injected.

**Files**:
- `internal/domain/application/models.go` — add to `Settings`
- `internal/usecases/services/user_service_impl.go` (or auth/user creation path) — inject workflow, trigger on create
- `internal/container/container.go` — wire
- UI: App Settings — add "On User Created Workflow" picker

**Estimate**: 0.5 day

**Implementation** (done):
- application.Settings: added `OnUserCreatedTriggerID`
- UserServiceImpl: SetAppRepo, SetWorkflowService; triggerOnUserCreated after Create/BulkCreate
- UpdateSettingsRequest + handler: map on_user_created_trigger_id
- Container: wire AppRepo + WorkflowService into UserService when Workflow enabled
- UI: "On User Created Workflow" picker in App Settings (AppDetail)

---

## Phase 6: Scheduled Triggers

**Goal**: Run workflows on a schedule (cron or interval) for users matching criteria (e.g. topic, or all).

**New resource**: `Schedule` (or `WorkflowSchedule`).

```json
{
  "name": "Weekly digest",
  "workflow_trigger_id": "weekly_digest",
  "cron": "0 9 * * 1",
  "target": { "type": "topic", "topic_id": "uuid" }
}
```

**Implementation**:
- New `schedule` package: model, repository, service
- Scheduler goroutine (in worker or API): every minute, evaluate cron expressions, for matching schedules resolve target users, call `TriggerForUsers`
- Store schedules in ES or DB
- API: CRUD for schedules
- UI: Schedules tab under Workflows

**Estimate**: 2 days (new domain, persistence, cron parsing, UI)

---

## Phase 7: Inbound Webhooks

**Goal**: External systems trigger workflows via HTTP (e.g. Stripe, CRM).

**New endpoint**: `POST /v1/webhooks/inbound` (or per-app `POST /v1/webhooks/:id`).

**Request** (from external system):
```json
{
  "event": "payment.received",
  "user_id": "external-id-or-email",
  "payload": { "amount": 99, "plan": "pro" }
}
```

**Implementation**:
- Webhook config per app: URL path, secret, event→trigger_id mapping
- Handler verifies signature (HMAC), resolves `user_id` (external_id lookup), calls `Trigger`
- Event mapping: `{"payment.received": "payment_received_workflow"}`

**Estimate**: 1 day

---

## Dependency Graph

```
Phase 1 (TriggerByTopic)     → standalone
Phase 2 (Broadcast→Workflow) → uses TriggerForUsers (from Phase 1)
Phase 3 (Send→Workflow)      → standalone
Phase 4 (Topic Subscribe)    → uses Trigger (Topic service + Workflow service)
Phase 5 (User Created)       → uses Trigger (User service + Workflow service)
Phase 6 (Scheduled)         → uses TriggerForUsers
Phase 7 (Webhooks)           → uses Trigger
```

**Recommended order**: 1 → 2 → 3 → 4 → 5 → 6 → 7 (or 6 and 7 in parallel after 5).

---

## Summary Table

| Phase | Integration          | Trigger source         | Recipients        | Est.  |
|-------|----------------------|------------------------|-------------------|-------|
| 1     | Trigger-by-topic     | API `trigger-by-topic` | Topic subscribers | 0.5d  |
| 2     | Broadcast→Workflow   | API `broadcast`        | Topic or all      | 0.5d  |
| 3     | Send→Workflow        | API `send`             | Single user       | 0.25d |
| 4     | Topic subscribe      | `AddSubscribers`       | New subscribers   | 0.5d  |
| 5     | User created         | `UserService.Create`   | New user          | 0.5d  |
| 6     | Scheduled            | Cron                   | Configurable      | 2d    |
| 7     | Inbound webhooks     | External HTTP          | Per event         | 1d    |

**Total**: ~5.25 days.

---

## Channel Compatibility

All phases are channel-agnostic. Workflows define steps (email, push, SMS, webhook, SSE). The integration only decides *which users* to trigger; the workflow steps decide *what* to send. No channel-specific changes required in this plan.
