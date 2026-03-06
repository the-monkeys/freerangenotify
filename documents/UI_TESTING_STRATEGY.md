# FreeRangeNotify UI — End-to-End Testing Strategy

> **Date:** March 7, 2026
> **Scope:** Complete manual + scripted testing of every UI feature against a live backend.
> **Prerequisites:** Docker stack running (`docker-compose up -d`), migrations applied (`docker-compose exec notification-service /app/migrate`).

---

## Table of Contents

1. [Environment Setup](#1-environment-setup)
2. [Seed Data Script](#2-seed-data-script)
3. [Test Journeys](#3-test-journeys)
4. [Page-by-Page Verification Matrix](#4-page-by-page-verification-matrix)
5. [API Smoke Tests (PowerShell)](#5-api-smoke-tests-powershell)
6. [Visual / Responsive Checks](#6-visual--responsive-checks)
7. [Build & Performance Verification](#7-build--performance-verification)

---

## 1. Environment Setup

### 1.1 Start the Stack

```powershell
cd C:\Users\Dave\the_monkeys\FreeRangeNotify

# Clean start
docker-compose down -v
docker-compose build
docker-compose up -d

# Wait for Elasticsearch to become healthy (~30s)
Start-Sleep -Seconds 30

# Run migrations (creates indices + seed templates)
docker-compose exec notification-service /app/migrate
```

### 1.2 Service Endpoints

| Service | URL | Purpose |
|---------|-----|---------|
| API Server | `http://localhost:8080` | Backend API |
| UI (Docker) | `http://localhost:3000` | Production-like UI container |
| UI (Dev) | `http://localhost:3000` (via `npm run dev` in `ui/`) | Hot-reload dev server |
| Elasticsearch | `http://localhost:9200` | Index inspection |
| Redis | `localhost:6379` | Queue/presence inspection |
| Health Check | `http://localhost:8080/v1/health` | Verify backend is up |

### 1.3 Verify Stack Health

```powershell
# Backend health
Invoke-RestMethod -Uri "http://localhost:8080/v1/health" -Method Get

# Elasticsearch cluster
Invoke-RestMethod -Uri "http://localhost:9200/_cluster/health" -Method Get

# UI responding
Invoke-RestMethod -Uri "http://localhost:3000" -Method Get -ErrorAction SilentlyContinue
```

**Expected:** Backend returns `200` with `{"status":"ok"}`. Elasticsearch returns `"green"` or `"yellow"`. UI returns HTML.

---

## 2. Seed Data Script

This PowerShell script creates all the data needed for testing. Run it once after `docker-compose up -d` + migrations. It outputs all IDs and tokens needed for subsequent tests.

```powershell
# ═══════════════════════════════════════════════════════════
# FreeRangeNotify — Test Seed Data Script
# ═══════════════════════════════════════════════════════════

$BASE = "http://localhost:8080/v1"
$CONTENT_TYPE = @{ "Content-Type" = "application/json" }

Write-Host "=== Step 1: Register Admin User ===" -ForegroundColor Cyan
$registerBody = @{
    email     = "admin@monkeys.com"
    password  = "TestPassword123!"
    full_name = "Test Admin"
} | ConvertTo-Json

$authResult = Invoke-RestMethod -Uri "$BASE/auth/register" `
    -Method Post -Body $registerBody -ContentType "application/json"
$ACCESS_TOKEN = $authResult.access_token
$REFRESH_TOKEN = $authResult.refresh_token
Write-Host "  access_token: $($ACCESS_TOKEN.Substring(0,20))..."
Write-Host "  refresh_token: $($REFRESH_TOKEN.Substring(0,20))..."

# If register returns 409 (already exists), login instead:
if (-not $ACCESS_TOKEN) {
    $loginBody = @{ email = "admin@monkeys.com"; password = "TestPassword123!" } | ConvertTo-Json
    $authResult = Invoke-RestMethod -Uri "$BASE/auth/login" `
        -Method Post -Body $loginBody -ContentType "application/json"
    $ACCESS_TOKEN = $authResult.access_token
    $REFRESH_TOKEN = $authResult.refresh_token
    Write-Host "  (logged in instead)"
}

$JWT_HEADERS = @{
    "Content-Type"  = "application/json"
    "Authorization" = "Bearer $ACCESS_TOKEN"
}

Write-Host "`n=== Step 2: Create Application ===" -ForegroundColor Cyan
$appBody = @{
    app_name    = "Monkeys Test App"
    webhook_url = "http://host.docker.internal:9999/webhook"
} | ConvertTo-Json

$appResult = Invoke-RestMethod -Uri "$BASE/apps/" `
    -Method Post -Body $appBody -Headers $JWT_HEADERS
$APP_ID = $appResult.data.app_id
$API_KEY = $appResult.data.api_key
Write-Host "  app_id:  $APP_ID"
Write-Host "  api_key: $API_KEY"

$API_HEADERS = @{
    "Content-Type"  = "application/json"
    "Authorization" = "Bearer $API_KEY"
}

Write-Host "`n=== Step 3: Create Test Users ===" -ForegroundColor Cyan
$user1Body = @{
    external_user_id = "user-alice-001"
    email            = "alice@monkeys.com"
    phone            = "+15551234567"
    timezone         = "America/New_York"
    language         = "en-US"
    preferences      = @{
        email_enabled = $true
        push_enabled  = $true
        sms_enabled   = $true
    }
} | ConvertTo-Json -Depth 3

$user1Result = Invoke-RestMethod -Uri "$BASE/users/" `
    -Method Post -Body $user1Body -Headers $API_HEADERS
$USER_1_ID = $user1Result.data.user_id
Write-Host "  user_1 (Alice): $USER_1_ID"

$user2Body = @{
    external_user_id = "user-bob-002"
    email            = "bob@monkeys.com"
    timezone         = "Europe/London"
    language         = "en-GB"
    preferences      = @{
        email_enabled = $true
        push_enabled  = $false
        sms_enabled   = $false
    }
} | ConvertTo-Json -Depth 3

$user2Result = Invoke-RestMethod -Uri "$BASE/users/" `
    -Method Post -Body $user2Body -Headers $API_HEADERS
$USER_2_ID = $user2Result.data.user_id
Write-Host "  user_2 (Bob):   $USER_2_ID"

$user3Body = @{
    external_user_id = "user-carol-003"
    email            = "carol@monkeys.com"
    preferences      = @{ email_enabled = $true }
} | ConvertTo-Json -Depth 3

$user3Result = Invoke-RestMethod -Uri "$BASE/users/" `
    -Method Post -Body $user3Body -Headers $API_HEADERS
$USER_3_ID = $user3Result.data.user_id
Write-Host "  user_3 (Carol): $USER_3_ID"

Write-Host "`n=== Step 4: Create Templates ===" -ForegroundColor Cyan
$template1Body = @{
    app_id    = $APP_ID
    name      = "test_welcome"
    channel   = "webhook"
    subject   = "Welcome {{.user_name}}!"
    body      = "Hello {{.user_name}}, welcome to {{.product_name}}. Your role is {{.role}}."
    variables = @("user_name", "product_name", "role")
    locale    = "en-US"
} | ConvertTo-Json

$t1Result = Invoke-RestMethod -Uri "$BASE/templates/" `
    -Method Post -Body $template1Body -Headers $API_HEADERS
$TEMPLATE_1_ID = $t1Result.data.id
if (-not $TEMPLATE_1_ID) { $TEMPLATE_1_ID = $t1Result.id }
Write-Host "  template_1 (test_welcome): $TEMPLATE_1_ID"

$template2Body = @{
    app_id    = $APP_ID
    name      = "test_alert"
    channel   = "email"
    subject   = "Alert: {{.alert_type}}"
    body      = "Hi {{.user_name}}, alert for {{.alert_type}}: {{.message}}"
    variables = @("user_name", "alert_type", "message")
    locale    = "en-US"
} | ConvertTo-Json

$t2Result = Invoke-RestMethod -Uri "$BASE/templates/" `
    -Method Post -Body $template2Body -Headers $API_HEADERS
$TEMPLATE_2_ID = $t2Result.data.id
if (-not $TEMPLATE_2_ID) { $TEMPLATE_2_ID = $t2Result.id }
Write-Host "  template_2 (test_alert):   $TEMPLATE_2_ID"

$template3Body = @{
    app_id    = $APP_ID
    name      = "test_sms"
    channel   = "sms"
    subject   = "Verification"
    body      = "Your code is {{.code}}. Expires in {{.expiry}} minutes."
    variables = @("code", "expiry")
    locale    = "en-US"
} | ConvertTo-Json

$t3Result = Invoke-RestMethod -Uri "$BASE/templates/" `
    -Method Post -Body $t3Body -Headers $API_HEADERS
$TEMPLATE_3_ID = $t3Result.data.id
if (-not $TEMPLATE_3_ID) { $TEMPLATE_3_ID = $t3Result.id }
Write-Host "  template_3 (test_sms):     $TEMPLATE_3_ID"

Write-Host "`n=== Step 5: Send Test Notifications ===" -ForegroundColor Cyan
$notif1Body = @{
    user_id     = $USER_1_ID
    template_id = $TEMPLATE_1_ID
    channel     = "webhook"
    priority    = "high"
    title       = "Welcome Alice"
    body        = "Fallback body"
    data        = @{
        user_name    = "Alice"
        product_name = "FreeRangeNotify"
        role         = "admin"
    }
} | ConvertTo-Json -Depth 3

$n1Result = Invoke-RestMethod -Uri "$BASE/notifications/" `
    -Method Post -Body $notif1Body -Headers $API_HEADERS
$NOTIF_1_ID = $n1Result.data.notification_id
if (-not $NOTIF_1_ID) { $NOTIF_1_ID = $n1Result.notification_id }
Write-Host "  notification_1: $NOTIF_1_ID"

$notif2Body = @{
    user_id  = $USER_2_ID
    channel  = "email"
    priority = "normal"
    title    = "Bob Update"
    body     = "Your account has been updated."
} | ConvertTo-Json -Depth 3

$n2Result = Invoke-RestMethod -Uri "$BASE/notifications/" `
    -Method Post -Body $notif2Body -Headers $API_HEADERS
$NOTIF_2_ID = $n2Result.data.notification_id
if (-not $NOTIF_2_ID) { $NOTIF_2_ID = $n2Result.notification_id }
Write-Host "  notification_2: $NOTIF_2_ID"

$notif3Body = @{
    user_id  = $USER_1_ID
    channel  = "webhook"
    priority = "low"
    title    = "Low Priority Notice"
    body     = "This is a low-priority test."
} | ConvertTo-Json -Depth 3

$n3Result = Invoke-RestMethod -Uri "$BASE/notifications/" `
    -Method Post -Body $notif3Body -Headers $API_HEADERS
$NOTIF_3_ID = $n3Result.data.notification_id
if (-not $NOTIF_3_ID) { $NOTIF_3_ID = $n3Result.notification_id }
Write-Host "  notification_3: $NOTIF_3_ID"

Write-Host "`n=== Step 6: Create Workflow ===" -ForegroundColor Cyan
$workflowBody = @{
    name        = "Welcome Flow"
    description = "Send welcome + follow-up after 1 hour"
    trigger_id  = "new_signup"
    steps       = @(
        @{
            id     = "step-1"
            name   = "Send Welcome Email"
            type   = "channel"
            order  = 1
            config = @{
                channel     = "webhook"
                template_id = $TEMPLATE_1_ID
            }
        },
        @{
            id     = "step-2"
            name   = "Wait 1 Hour"
            type   = "delay"
            order  = 2
            config = @{
                duration = "1h"
            }
        },
        @{
            id     = "step-3"
            name   = "Check if Read"
            type   = "condition"
            order  = 3
            config = @{
                condition = @{
                    field    = "status"
                    operator = "neq"
                    value    = "read"
                }
            }
        }
    )
    status = "active"
} | ConvertTo-Json -Depth 5

$wfResult = Invoke-RestMethod -Uri "$BASE/workflows/" `
    -Method Post -Body $workflowBody -Headers $API_HEADERS
$WORKFLOW_ID = $wfResult.data.id
if (-not $WORKFLOW_ID) { $WORKFLOW_ID = $wfResult.id }
Write-Host "  workflow_id: $WORKFLOW_ID"

Write-Host "`n=== Step 7: Create Topic ===" -ForegroundColor Cyan
$topicBody = @{
    name        = "Product Updates"
    key         = "product-updates"
    description = "All product update notifications"
} | ConvertTo-Json

$topicResult = Invoke-RestMethod -Uri "$BASE/topics/" `
    -Method Post -Body $topicBody -Headers $API_HEADERS
$TOPIC_ID = $topicResult.data.id
if (-not $TOPIC_ID) { $TOPIC_ID = $topicResult.id }
Write-Host "  topic_id: $TOPIC_ID"

# Add subscribers
$subBody = @{
    user_ids = @($USER_1_ID, $USER_2_ID)
} | ConvertTo-Json

Invoke-RestMethod -Uri "$BASE/topics/$TOPIC_ID/subscribers" `
    -Method Post -Body $subBody -Headers $API_HEADERS | Out-Null
Write-Host "  Added Alice and Bob as subscribers"

Write-Host "`n=== Step 8: Create Digest Rule ===" -ForegroundColor Cyan
$digestBody = @{
    name        = "Hourly Alert Digest"
    digest_key  = "alerts"
    window      = "1h"
    channel     = "email"
    template_id = $TEMPLATE_2_ID
    max_batch   = 50
    status      = "active"
} | ConvertTo-Json

$digestResult = Invoke-RestMethod -Uri "$BASE/digest-rules/" `
    -Method Post -Body $digestBody -Headers $API_HEADERS
$DIGEST_ID = $digestResult.data.id
if (-not $DIGEST_ID) { $DIGEST_ID = $digestResult.id }
Write-Host "  digest_rule_id: $DIGEST_ID"

Write-Host "`n=== SEED DATA COMPLETE ===" -ForegroundColor Green
Write-Host ""
Write-Host "╔══════════════════════════════════════════════════════╗"
Write-Host "║  TEST CREDENTIALS & IDS                             ║"
Write-Host "╠══════════════════════════════════════════════════════╣"
Write-Host "║  Admin Email:    admin@monkeys.com                  ║"
Write-Host "║  Admin Password: TestPassword123!                   ║"
Write-Host "╠══════════════════════════════════════════════════════╣"
Write-Host "║  APP_ID:         $APP_ID"
Write-Host "║  API_KEY:        $API_KEY"
Write-Host "╠══════════════════════════════════════════════════════╣"
Write-Host "║  USER_1 (Alice): $USER_1_ID"
Write-Host "║  USER_2 (Bob):   $USER_2_ID"
Write-Host "║  USER_3 (Carol): $USER_3_ID"
Write-Host "╠══════════════════════════════════════════════════════╣"
Write-Host "║  TEMPLATE_1 (webhook): $TEMPLATE_1_ID"
Write-Host "║  TEMPLATE_2 (email):   $TEMPLATE_2_ID"
Write-Host "║  TEMPLATE_3 (sms):     $TEMPLATE_3_ID"
Write-Host "╠══════════════════════════════════════════════════════╣"
Write-Host "║  NOTIF_1: $NOTIF_1_ID"
Write-Host "║  NOTIF_2: $NOTIF_2_ID"
Write-Host "║  NOTIF_3: $NOTIF_3_ID"
Write-Host "╠══════════════════════════════════════════════════════╣"
Write-Host "║  WORKFLOW_ID:    $WORKFLOW_ID"
Write-Host "║  TOPIC_ID:       $TOPIC_ID"
Write-Host "║  DIGEST_ID:      $DIGEST_ID"
Write-Host "╚══════════════════════════════════════════════════════╝"
```

---

## 3. Test Journeys

Each journey is an end-to-end user flow through the UI. Execute them in order — later journeys depend on data from earlier ones.

### Journey 1: Authentication & Onboarding

| Step | Action | Expected |
|------|--------|----------|
| 1.1 | Open `http://localhost:3000` | Landing page loads with "Monkeys" branding, Inter font, `#FAFAFA` background |
| 1.2 | Click "Get Started" / "Login" | Navigate to `/login` with AuthLayout (centered card) |
| 1.3 | Click "Create account" link | Navigate to `/register` |
| 1.4 | Register: `admin@monkeys.com` / `TestPassword123!` / `Test Admin` | Toast: "Account created". Redirect to `/apps` |
| 1.5 | (If already registered) Login: `admin@monkeys.com` / `TestPassword123!` | Toast: "Welcome back". Redirect to `/apps` |
| 1.6 | Verify sidebar | Shows: Dashboard, Applications, Workflows, Digest Rules, Topics, Audit Logs, Documentation |
| 1.7 | Verify topbar | Shows user email/name, logout dropdown |

### Journey 2: Application CRUD

| Step | Action | Expected |
|------|--------|----------|
| 2.1 | Click "Applications" in sidebar | `/apps` — list page, may show "Monkeys Test App" from seed |
| 2.2 | Click "Create Application" | Dialog/form appears |
| 2.3 | Create: name=`UI Test App`, webhook_url=`http://localhost:9999/hook` | Toast: "Application created". App appears in list |
| 2.4 | Click the app row | Navigate to `/apps/:id` — AppDetail with tabs |
| 2.5 | Verify tabs exist | Overview, Users, Templates, Notifications, Digest Rules, Topics, Team, Providers, Environments, Settings, Integration |
| 2.6 | Overview tab | Shows app name, API key (masked with copy button), webhook URL, created date |
| 2.7 | Settings tab | Can update webhook URL. Toggle feature flags (workflow_enabled, digest_enabled, topics_enabled, audit_enabled) |

### Journey 3: User Management

| Step | Action | Expected |
|------|--------|----------|
| 3.1 | Click "Users" tab | User list table. Seed data: Alice, Bob, Carol |
| 3.2 | Click "Add User" | Form appears with fields: email, external_id, phone, timezone, language |
| 3.3 | Create user: email=`dave@monkeys.com`, external_id=`user-dave-004` | Toast: "User created". User appears in list |
| 3.4 | Click a user row | User detail panel: shows email, phone, external_id, preferences, devices |
| 3.5 | Edit preferences | Toggle email/push/sms enabled. Save. Toast confirms. |
| 3.6 | Delete a user (Dave) | Confirm dialog. Toast: "User deleted". User removed from list |

### Journey 4: Template Management

| Step | Action | Expected |
|------|--------|----------|
| 4.1 | Click "Templates" tab | Template list. Seed: test_welcome, test_alert, test_sms |
| 4.2 | Click "Clone from Library" | Library dialog shows seed templates (welcome_email, password_reset, etc.) |
| 4.3 | Clone `welcome_email` | Toast: "Template cloned". New template appears in list |
| 4.4 | Click "Create Template" | Form: name, channel (dropdown), subject, body (textarea), variables |
| 4.5 | Create: name=`ui_test_tmpl`, channel=`webhook`, subject=`Test {{.name}}`, body=`Hello {{.name}}`, variables=`name` | Toast: "Template created" |
| 4.6 | Click template to edit | Template editor loads with current content |
| 4.7 | Edit body, save | Toast: "Template updated". New version created |
| 4.8 | Click "Version History" | Version list displays. Shows v1, v2 |
| 4.9 | Click "Compare Versions" | TemplateDiffViewer opens: side-by-side diff between v1 and v2 |
| 4.10 | Click "Rollback" on v1 | Confirm dialog. Toast: "Rolled back to v1". Creates v3 with v1 content |
| 4.11 | Click "Test Send" | TemplateTestPanel opens: user picker, variables JSON input, Send Test button |
| 4.12 | Test send: pick Alice, data=`{"name":"Alice"}` | Toast: "Test notification sent" |
| 4.13 | Click "Content Controls" | TemplateControlsPanel opens (may be empty if no controls defined) |

### Journey 5: Notification Send & Inbox

| Step | Action | Expected |
|------|--------|----------|
| 5.1 | Click "Notifications" tab | Notification list. Shows seed notifications. Unread count badge on tab |
| 5.2 | **Quick Send:** select Alice, template=test_welcome, submit | Toast: "Notification sent". Appears in list |
| 5.3 | **Advanced tab:** select channel=webhook, recipient=Bob, no template, title=`Manual Test`, body=`Hello Bob` | Toast: "Notification sent" |
| 5.4 | **Broadcast tab:** channel=webhook, title=`Broadcast Test`, body=`To all users` | Confirm dialog ("send to ALL users?"). Confirm. Toast shows count |
| 5.5 | Verify filter bar | Filter by status (pending/sent/failed), channel (email/webhook), date range |
| 5.6 | Select 2 notifications via checkbox | Bulk actions bar appears: "2 selected" | Mark Read | Archive | Clear |
| 5.7 | Click "Mark Read" | Toast: "Marked as read". Status badges update to `read` |
| 5.8 | Click "Mark All Read" button | Toast: "All notifications marked as read" |
| 5.9 | Click snooze (BellOff) on a notification | Snooze dialog or 1h snooze. Status badge changes to `snoozed` (purple) |
| 5.10 | Click unsnooze (Bell icon, purple) | Toast: "Unsnoozed". Status returns to previous |
| 5.11 | Select notifications → "Archive" bulk action | Toast: "Archived". Status badges update to `archived` (gray) |
| 5.12 | Click "Batch Send" button | Dialog opens with JSON textarea |
| 5.13 | Paste batch JSON, submit: | Toast: "Batch sent" |

**Batch Send input data:**
```json
[
  {
    "user_id": "<USER_1_ID>",
    "channel": "webhook",
    "title": "Batch 1",
    "body": "First batch notification"
  },
  {
    "user_id": "<USER_2_ID>",
    "channel": "webhook",
    "title": "Batch 2",
    "body": "Second batch notification"
  }
]
```

| Step | Action | Expected |
|------|--------|----------|
| 5.14 | Click "Cancel Batch" button | Dialog opens with batch ID input |
| 5.15 | Enter a batch ID (from batch send response), submit | Toast: "Batch cancelled" or error if already processed |
| 5.16 | Click a notification row | Detail panel slides out: full content, metadata, delivery status, timestamps |
| 5.17 | Verify status badges | `pending`=yellow, `sent`=green, `failed`=red, `snoozed`=purple, `archived`=gray, `read`=sky, `dead_letter`=dark red |

### Journey 6: Workflows

| Step | Action | Expected |
|------|--------|----------|
| 6.1 | Click "Workflows" in sidebar | `/workflows` — list page. Seed: "Welcome Flow" |
| 6.2 | Click "Create Workflow" | Navigate to `/workflows/new` — WorkflowBuilder |
| 6.3 | Fill: name=`Test Flow`, trigger_id=`test_trigger`, description=`E2E test` | Fields accept input |
| 6.4 | Click "+ Add Step" → Channel step | Step card appears. Template picker (ResourcePicker) shows templates from API |
| 6.5 | Select template=test_welcome, channel=webhook | Step configured |
| 6.6 | Add Delay step: duration=`30m` | Second step card appears |
| 6.7 | Add Condition step: field=`status`, operator=`neq`, value=`read` | Third step card appears |
| 6.8 | Click "Save" or "Activate" | Toast: "Workflow created/activated". Redirect to list |
| 6.9 | Click "Trigger" on the workflow | Test trigger panel: user picker + JSON payload input |
| 6.10 | Select Alice, payload=`{"user_name":"Alice","product_name":"Test"}` | Toast: "Workflow triggered" |
| 6.11 | Click "Executions" link / tab | WorkflowExecutions page. Shows execution with status |
| 6.12 | Click an execution row | ExecutionTimeline: step-by-step progress with status badges |
| 6.13 | Edit workflow: change step order, save | Toast: "Workflow updated" |
| 6.14 | Delete workflow via dropdown menu | Confirm dialog. Toast: "Workflow deleted" |

### Journey 7: Digest Rules

| Step | Action | Expected |
|------|--------|----------|
| 7.1 | Navigate to app → Digest Rules tab | List page. Seed: "Hourly Alert Digest" |
| 7.2 | Click "Create Rule" | Form: Name, Digest Key, Window, Channel, Template (ResourcePicker), Max Batch, Status |
| 7.3 | Create: name=`Test Digest`, key=`test-events`, window=`30m`, channel=`email`, template=test_alert, max_batch=20 | Toast: "Digest rule created" |
| 7.4 | Edit the rule: change window to `1h` | Toast: "Digest rule updated" |
| 7.5 | Toggle status active/inactive | Status badge updates |
| 7.6 | Delete the rule | Confirm dialog. Toast: "Digest rule deleted" |

### Journey 8: Topics & Subscribers

| Step | Action | Expected |
|------|--------|----------|
| 8.1 | Navigate to app → Topics tab | List. Seed: "Product Updates" with 2 subscribers |
| 8.2 | Click "Create Topic" | Form: Name, Key (auto-slug), Description |
| 8.3 | Create: name=`Engineering Updates`, key=`eng-updates`, description=`Internal eng team` | Toast: "Topic created" |
| 8.4 | Click subscriber count button on "Product Updates" | Subscriber panel shows Alice and Bob |
| 8.5 | Add Carol as subscriber | Search/select Carol via user picker. Click "Add". Toast confirms |
| 8.6 | Remove Bob from subscribers | Click remove on Bob's row. Toast: "Subscriber removed" |
| 8.7 | Edit topic name | Toast: "Topic updated" |
| 8.8 | Delete "Engineering Updates" | Confirm dialog. Toast: "Topic deleted" |

### Journey 9: Team / RBAC

| Step | Action | Expected |
|------|--------|----------|
| 9.1 | Navigate to app → Team tab | Member list. Shows current user as `owner` |
| 9.2 | Click "Invite Member" | Form: Email, Role (admin/editor/viewer dropdown) |
| 9.3 | Invite: email=`editor@monkeys.com`, role=`editor` | Toast: "Member invited" |
| 9.4 | Change role of invited member to `admin` | Dropdown changes. Toast: "Role updated" |
| 9.5 | Remove member | Confirm dialog. Toast: "Member removed" |
| 9.6 | Verify can't remove self or demote last owner | Error messages / disabled buttons |

### Journey 10: Providers

| Step | Action | Expected |
|------|--------|----------|
| 10.1 | Navigate to app → Providers tab | List (may be empty) |
| 10.2 | Click "Register Provider" | Form: Name, Channel, Webhook URL, Headers (JSON) |
| 10.3 | Register: name=`Slack Internal`, channel=`slack`, url=`https://hooks.slack.com/test` | Toast: "Provider registered". Signing key shown (one-time) |
| 10.4 | Copy the signing key | Clipboard copy works |
| 10.5 | Remove the provider | Confirm dialog. Toast: "Provider removed" |

### Journey 11: Environments

| Step | Action | Expected |
|------|--------|----------|
| 11.1 | Navigate to app → Environments tab | List. Default environment shown |
| 11.2 | Click "Create Environment" | Form: Name (development/staging/production dropdown) |
| 11.3 | Create `staging` | Toast: "Environment created". API key shown |
| 11.4 | Verify env switcher in app header | Dropdown with colored dots |
| 11.5 | Switch to staging env | Child tabs (Users, Templates, etc.) reload with staging API key |
| 11.6 | Promote (if available) | Source/target dropdown. Resource checkboxes. Confirm |
| 11.7 | Delete staging env | Confirm. Toast: "Environment deleted" |

### Journey 12: Dashboard & Admin

| Step | Action | Expected |
|------|--------|----------|
| 12.1 | Click "Dashboard" in sidebar | `/dashboard` with tabs: Overview, Analytics, Activity, Tools |
| 12.2 | Overview tab | Stats cards (total apps, users, templates). Queue depth cards with sparklines |
| 12.3 | Queue cards | 3 cards (high/normal/low) with depth bar + SVG sparkline trend |
| 12.4 | DLQ section | Table with checkboxes. Replay selected / Replay All buttons |
| 12.5 | Provider Health table | Shows provider status, latency, last error |
| 12.6 | Analytics tab | AnalyticsDashboard with channel breakdown chart |
| 12.7 | Activity tab | Real-time SSE feed. Events appear as notifications are sent |
| 12.8 | Tools tab | WebhookPlayground + QuickTestPanel |
| 12.9 | Send a test via QuickTestPanel | Select app, user, template. Send. Toast confirms |
| 12.10 | Webhook Playground | Create playground URL. Send webhook to it. Verify payload appears |

### Journey 13: Audit Logs

| Step | Action | Expected |
|------|--------|----------|
| 13.1 | Click "Audit Logs" in sidebar | `/audit` — AuditLogsList |
| 13.2 | Verify entries | Shows create/update/delete actions from previous journeys |
| 13.3 | Filter by action type | Dropdown: create/update/delete/send |
| 13.4 | Filter by resource type | Dropdown: application/user/template/notification |
| 13.5 | Click a log entry | Detail panel: full changes diff, actor, timestamp, IP |

### Journey 14: Documentation Hub

| Step | Action | Expected |
|------|--------|----------|
| 14.1 | Click "Documentation" in sidebar | `/docs` with sidebar nav |
| 14.2 | Getting Started page | Rendered markdown with syntax-highlighted code blocks (Prism oneLight) |
| 14.3 | API Reference page | Swagger/OpenAPI spec rendered (from `public/swagger.json`) |
| 14.4 | Channels page | All channels documented: Webhook, Email, APNS, FCM, SMS, SSE, In-App, Custom (Slack/Discord/Teams/WhatsApp) |
| 14.5 | SDK page | Go, JS, React SDK documentation |
| 14.6 | Workflows page | Workflow creation and step type docs |
| 14.7 | Troubleshooting page | Common issues and solutions |
| 14.8 | Verify code highlighting | Fenced code blocks render with `react-syntax-highlighter` Prism theme |

### Journey 15: Account Management

| Step | Action | Expected |
|------|--------|----------|
| 15.1 | Click user dropdown in topbar | Options: Change Password, Logout |
| 15.2 | Click "Change Password" | ChangePasswordDialog: current password, new password, confirm |
| 15.3 | Enter: current=`TestPassword123!`, new=`NewPassword456!`, confirm=`NewPassword456!` | Toast: "Password changed" |
| 15.4 | Logout | Redirect to `/login` |
| 15.5 | Login with new password | Success |
| 15.6 | (Reset password back for other tests) | Change back to `TestPassword123!` |

---

## 4. Page-by-Page Verification Matrix

Every page and its key UI elements:

| Page | Route | Skeleton Loader | Empty State | Error Boundary | Toast on CRUD | Mobile Responsive |
|------|-------|:---:|:---:|:---:|:---:|:---:|
| Landing | `/` | N/A | N/A | ✓ wraps app | N/A | Verify layout stacks |
| Login | `/login` | N/A | N/A | ✓ | ✓ login | Centered card |
| Register | `/register` | N/A | N/A | ✓ | ✓ register | Centered card |
| Apps List | `/apps` | ✓ | ✓ "No apps" | ✓ | ✓ CRUD | Cards stack |
| App Detail | `/apps/:id` | ✓ | Per-tab | ✓ | ✓ | Tab scroll |
| Dashboard | `/dashboard` | ✓ | N/A | ✓ | ✓ | Cards stack |
| Workflows List | `/workflows` | ✓ | ✓ "No workflows" | ✓ | ✓ CRUD | `hidden md:table-cell` on Trigger ID, Steps, Version, Updated |
| Workflow Builder | `/workflows/new` | N/A | ✓ "Add first step" | ✓ | ✓ save | Steps stack |
| Workflow Executions | `/workflows/executions` | ✓ | ✓ "No executions" | ✓ | N/A | columns hidden |
| Digest Rules | App tab | ✓ | ✓ | ✓ | ✓ CRUD | Table scrolls |
| Topics | App tab | ✓ | ✓ | ✓ | ✓ CRUD | `hidden md:table-cell` on Key, Subscribers, Updated |
| Notifications | App tab | ✓ | ✓ "No notifications" | ✓ | ✓ all ops | `hidden md:table-cell` on ID, User, Scheduled At, Sent At |
| Team | App tab | ✓ | ✓ "No members" | ✓ | ✓ invite/update/remove | Table scrolls |
| Providers | App tab | ✓ | ✓ "No providers" | ✓ | ✓ register/remove | Table scrolls |
| Environments | App tab | ✓ | ✓ | ✓ | ✓ CRUD | Cards stack |
| Audit Logs | `/audit` | ✓ | ✓ "No logs" | ✓ | N/A | Table scrolls |
| Docs | `/docs/*` | ✓ | N/A | ✓ | N/A | Sidebar collapses |
| API Reference | `/docs/api` | ✓ | N/A | ✓ | N/A | Scrollable |

---

## 5. API Smoke Tests (PowerShell)

Run after seed data is created. These verify the API layer independently, confirming the backend is correct before blaming the UI.

```powershell
# ═══════════════════════════════════════════════════════════
# API Smoke Test — Run after seed data script
# ═══════════════════════════════════════════════════════════
# Populate these from seed output:
$BASE = "http://localhost:8080/v1"
$ACCESS_TOKEN = "<JWT_TOKEN>"
$API_KEY = "<API_KEY>"
$APP_ID = "<APP_ID>"
$USER_1_ID = "<USER_1_UUID>"
$NOTIF_1_ID = "<NOTIF_1_UUID>"
$TEMPLATE_1_ID = "<TMPL_1_UUID>"
$WORKFLOW_ID = "<WORKFLOW_UUID>"
$TOPIC_ID = "<TOPIC_UUID>"
$DIGEST_ID = "<DIGEST_UUID>"

$JWT_H = @{ "Authorization" = "Bearer $ACCESS_TOKEN"; "Content-Type" = "application/json" }
$API_H = @{ "Authorization" = "Bearer $API_KEY"; "Content-Type" = "application/json" }

$pass = 0; $fail = 0
function Test-API($name, $method, $uri, $headers, $body) {
    try {
        $params = @{ Uri = $uri; Method = $method; Headers = $headers }
        if ($body) { $params.Body = $body }
        $r = Invoke-RestMethod @params
        Write-Host "  PASS: $name" -ForegroundColor Green
        $script:pass++
        return $r
    } catch {
        Write-Host "  FAIL: $name — $($_.Exception.Message)" -ForegroundColor Red
        $script:fail++
        return $null
    }
}

Write-Host "`n=== Auth ===" -ForegroundColor Cyan
Test-API "Health"               GET  "$BASE/health" @{} $null
Test-API "Get Current User"     GET  "$BASE/admin/me" $JWT_H $null

Write-Host "`n=== Applications ===" -ForegroundColor Cyan
Test-API "List Apps"            GET  "$BASE/apps/" $JWT_H $null
Test-API "Get App"              GET  "$BASE/apps/$APP_ID" $JWT_H $null
Test-API "Get Settings"         GET  "$BASE/apps/$APP_ID/settings" $JWT_H $null

Write-Host "`n=== Users ===" -ForegroundColor Cyan
Test-API "List Users"           GET  "$BASE/users/" $API_H $null
Test-API "Get User"             GET  "$BASE/users/$USER_1_ID" $API_H $null
Test-API "Get Preferences"     GET  "$BASE/users/$USER_1_ID/preferences" $API_H $null
Test-API "Get Devices"          GET  "$BASE/users/$USER_1_ID/devices" $API_H $null

Write-Host "`n=== Templates ===" -ForegroundColor Cyan
Test-API "List Templates"       GET  "$BASE/templates/" $API_H $null
Test-API "Get Template"         GET  "$BASE/templates/$TEMPLATE_1_ID" $API_H $null
Test-API "Get Library"          GET  "$BASE/templates/library" $API_H $null
Test-API "Render Template"      POST "$BASE/templates/$TEMPLATE_1_ID/render" $API_H `
    (@{ data = @{ user_name = "Test"; product_name = "FRN"; role = "tester" } } | ConvertTo-Json -Depth 3)

Write-Host "`n=== Notifications ===" -ForegroundColor Cyan
Test-API "List Notifications"   GET  "$BASE/notifications/" $API_H $null
Test-API "Get Notification"     GET  "$BASE/notifications/$NOTIF_1_ID" $API_H $null
Test-API "Unread Count"         GET  "$BASE/notifications/unread/count?user_id=$USER_1_ID" $API_H $null
Test-API "List Unread"          GET  "$BASE/notifications/unread?user_id=$USER_1_ID" $API_H $null

Write-Host "`n=== Workflows ===" -ForegroundColor Cyan
Test-API "List Workflows"       GET  "$BASE/workflows/" $API_H $null
Test-API "Get Workflow"         GET  "$BASE/workflows/$WORKFLOW_ID" $API_H $null
Test-API "List Executions"      GET  "$BASE/workflows/executions" $API_H $null

Write-Host "`n=== Topics ===" -ForegroundColor Cyan
Test-API "List Topics"          GET  "$BASE/topics/" $API_H $null
Test-API "Get Topic"            GET  "$BASE/topics/$TOPIC_ID" $API_H $null
Test-API "Get Subscribers"      GET  "$BASE/topics/$TOPIC_ID/subscribers" $API_H $null
Test-API "Get by Key"           GET  "$BASE/topics/key/product-updates" $API_H $null

Write-Host "`n=== Digest Rules ===" -ForegroundColor Cyan
Test-API "List Digest Rules"    GET  "$BASE/digest-rules/" $API_H $null
Test-API "Get Digest Rule"      GET  "$BASE/digest-rules/$DIGEST_ID" $API_H $null

Write-Host "`n=== Admin ===" -ForegroundColor Cyan
Test-API "Queue Stats"          GET  "$BASE/admin/queues/stats" @{} $null
Test-API "DLQ List"             GET  "$BASE/admin/queues/dlq" @{} $null
Test-API "Provider Health"      GET  "$BASE/admin/providers/health" @{} $null
Test-API "Analytics Summary"    GET  "$BASE/admin/analytics/summary" $JWT_H $null

Write-Host "`n=== RESULTS ===" -ForegroundColor Cyan
Write-Host "  Passed: $pass" -ForegroundColor Green
Write-Host "  Failed: $fail" -ForegroundColor $(if ($fail -gt 0) { "Red" } else { "Green" })
```

---

## 6. Visual / Responsive Checks

### 6.1 Desktop (> 1024px)

| Check | Where | Expected |
|-------|-------|----------|
| Sidebar fixed 240px | All pages | Fixed left sidebar with nav sections |
| Content area scrolls | All pages | Main area scrolls independently |
| Tables full width | Notifications, Workflows, Topics, Audit | All columns visible |
| Cards in grid | Dashboard overview | 4 cards per row |
| Queue sparklines | Dashboard overview | SVG sparkline below each queue depth bar |

### 6.2 Tablet (768px–1024px)

| Check | Where | Expected |
|-------|-------|----------|
| Sidebar collapses | All pages | Hamburger menu or collapsed icon sidebar |
| Tables | Notifications | ID, Scheduled At, Sent At columns hidden |
| Tables | Workflows | Trigger ID, Steps, Version columns hidden |
| Tables | Topics | Key, Subscribers columns hidden |
| Tab bar | AppDetail | Horizontal scroll with overflow |

### 6.3 Mobile (< 768px)

| Check | Where | Expected |
|-------|-------|----------|
| Sidebar hidden | All pages | Sheet/drawer for navigation |
| Cards stack vertically | Dashboard, Environments | Single column |
| Forms full width | All forms | Inputs span 100% |
| Buttons stack | Notification toolbar | Wrap to next line |
| Tables scroll horizontally | All table pages | `overflow-x-auto` wrapper |
| Additionally hidden | Notifications | User column hidden at `lg` |
| Additionally hidden | Workflows | Updated column hidden at `lg` |
| Additionally hidden | Topics | Updated column hidden at `lg` |

### 6.4 Theme Checks

| Check | Expected |
|-------|----------|
| Background color | `#FAFAFA` (light mode) |
| Card background | `#FFFFFF` |
| Text color | `#121212` |
| Accent (coral) | `#FF5542` — only on primary CTAs, active nav, destructive |
| Font | Inter (body), JetBrains Mono (code/API keys) |
| Border radius | 8px on cards |
| Borders | `#E5E5E5`, 1px |
| No blue/azure remnants | Zero uses of old theme colors |

---

## 7. Build & Performance Verification

### 7.1 TypeScript Check

```powershell
cd C:\Users\Dave\the_monkeys\FreeRangeNotify\ui
npx tsc --noEmit
# Expected: 0 errors
```

### 7.2 Production Build

```powershell
npx vite build
# Expected: ✓ built in <30s, no errors
# Warning about chunk size (DocsPage) is acceptable (react-syntax-highlighter)
```

### 7.3 Docker Container Build

```powershell
cd C:\Users\Dave\the_monkeys\FreeRangeNotify
docker-compose build ui
docker-compose up -d --force-recreate ui
# Wait 10s, then:
Start-Process "http://localhost:3000"
# Expected: UI loads, login works, all pages render
```

### 7.4 Performance Targets

| Metric | Target | How to Measure |
|--------|--------|----------------|
| `tsc --noEmit` | 0 errors | Terminal |
| `vite build` | 0 errors | Terminal |
| Initial bundle (gzipped) | < 200KB (index chunk) | Vite build output |
| Route code-split | All pages lazy-loaded | Vite build output (separate chunks per page) |
| Largest single chunk | < 800KB (DocsPage acceptable) | Vite build output |

### 7.5 Cleanup Script

Run after testing to remove seed data:

```powershell
$BASE = "http://localhost:8080/v1"
# Use the same $JWT_H and $API_H from seed script

# Delete in reverse dependency order
Invoke-RestMethod -Uri "$BASE/digest-rules/$DIGEST_ID" -Method Delete -Headers $API_H
Invoke-RestMethod -Uri "$BASE/topics/$TOPIC_ID" -Method Delete -Headers $API_H
Invoke-RestMethod -Uri "$BASE/workflows/$WORKFLOW_ID" -Method Delete -Headers $API_H
Invoke-RestMethod -Uri "$BASE/notifications/$NOTIF_1_ID" -Method Delete -Headers $API_H
Invoke-RestMethod -Uri "$BASE/notifications/$NOTIF_2_ID" -Method Delete -Headers $API_H
Invoke-RestMethod -Uri "$BASE/notifications/$NOTIF_3_ID" -Method Delete -Headers $API_H
Invoke-RestMethod -Uri "$BASE/templates/$TEMPLATE_1_ID" -Method Delete -Headers $API_H
Invoke-RestMethod -Uri "$BASE/templates/$TEMPLATE_2_ID" -Method Delete -Headers $API_H
Invoke-RestMethod -Uri "$BASE/templates/$TEMPLATE_3_ID" -Method Delete -Headers $API_H
Invoke-RestMethod -Uri "$BASE/users/$USER_1_ID" -Method Delete -Headers $API_H
Invoke-RestMethod -Uri "$BASE/users/$USER_2_ID" -Method Delete -Headers $API_H
Invoke-RestMethod -Uri "$BASE/users/$USER_3_ID" -Method Delete -Headers $API_H
Invoke-RestMethod -Uri "$BASE/apps/$APP_ID" -Method Delete -Headers $JWT_H

Write-Host "Cleanup complete." -ForegroundColor Green
```

---

## Appendix: Test Data Reference

### Users

| Label | Email | External ID | Phone | Preferences |
|-------|-------|-------------|-------|-------------|
| Alice | `alice@monkeys.com` | `user-alice-001` | `+15551234567` | email ✓, push ✓, sms ✓ |
| Bob | `bob@monkeys.com` | `user-bob-002` | — | email ✓, push ✗, sms ✗ |
| Carol | `carol@monkeys.com` | `user-carol-003` | — | email ✓ |

### Templates

| Label | Name | Channel | Variables |
|-------|------|---------|-----------|
| T1 | `test_welcome` | webhook | `user_name`, `product_name`, `role` |
| T2 | `test_alert` | email | `user_name`, `alert_type`, `message` |
| T3 | `test_sms` | sms | `code`, `expiry` |

### Sample Template Variables JSON

```json
// For test_welcome:
{ "user_name": "Alice", "product_name": "FreeRangeNotify", "role": "admin" }

// For test_alert:
{ "user_name": "Bob", "alert_type": "CPU High", "message": "CPU at 95%" }

// For test_sms:
{ "code": "482917", "expiry": "5" }
```

### Admin Account

| Field | Value |
|-------|-------|
| Email | `admin@monkeys.com` |
| Password | `TestPassword123!` |

### Workflow

| Field | Value |
|-------|-------|
| Name | `Welcome Flow` |
| Trigger ID | `new_signup` |
| Steps | Channel (webhook, test_welcome) → Delay (1h) → Condition (status ≠ read) |

### Topic

| Field | Value |
|-------|-------|
| Name | `Product Updates` |
| Key | `product-updates` |
| Subscribers | Alice, Bob |

### Digest Rule

| Field | Value |
|-------|-------|
| Name | `Hourly Alert Digest` |
| Key | `alerts` |
| Window | `1h` |
| Channel | email |
| Template | test_alert |
| Max Batch | 50 |
