# FreeRangeNotify — Simplification Low-Level Design

> **Date**: March 2026  
> **Prerequisite**: Read `SIMPLIFICATION_ROADMAP.md` for problem context and priority matrix.  
> **Scope**: Implementation-level specifications for every fix identified in the roadmap.

---

## Table of Contents

- [Phase 1 — Quick Wins](#phase-1--quick-wins)
  - [1.1 Enable Template Editing in UI](#11-enable-template-editing-in-ui)
  - [1.2 Default Locale to "en"](#12-default-locale-to-en)
  - [1.3 Auto-Detect Template Variables](#13-auto-detect-template-variables)
  - [1.4 Fix Multi-Send to Use Bulk API](#14-fix-multi-send-to-use-bulk-api)
  - [1.5 Add Phone Field to User UI](#15-add-phone-field-to-user-ui)
  - [1.6 Remove Debug fmt.Printf Statements](#16-remove-debug-fmtprintf-statements)
  - [1.7 Add Pagination to All List Views](#17-add-pagination-to-all-list-views)
- [Phase 2 — Core Simplifications](#phase-2--core-simplifications)
  - [2.1 Quick-Send API Endpoint](#21-quick-send-api-endpoint)
  - [2.2 Send by Email / External ID](#22-send-by-email--external-id)
  - [2.3 Template Reference by Name](#23-template-reference-by-name)
  - [2.4 Infer Channel from Template](#24-infer-channel-from-template)
  - [2.5 Simplified Send Form in UI](#25-simplified-send-form-in-ui)
  - [2.6 Timezone / Language Dropdowns](#26-timezone--language-dropdowns)
  - [2.7 Unified Auth Documentation](#27-unified-auth-documentation)
  - [2.8 Bulk User Import Endpoint](#28-bulk-user-import-endpoint)
- [Phase 3 — Premium Experience](#phase-3--premium-experience)
  - [3.1 Rich HTML Template Editor](#31-rich-html-template-editor)
  - [3.2 Setup Wizard in UI](#32-setup-wizard-in-ui)
  - [3.3 Pre-Built Template Library](#33-pre-built-template-library)
  - [3.4 Notification Activity Feed](#34-notification-activity-feed)
  - [3.5 Provider Health Dashboard](#35-provider-health-dashboard)
  - [3.6 Dead Letter Queue Viewer](#36-dead-letter-queue-viewer)
  - [3.7 Multi-Channel Template Groups](#37-multi-channel-template-groups)
  - [3.8 Client SDKs](#38-client-sdks)
- [Phase 4 — Competitive Edge](#phase-4--competitive-edge)
  - [4.1 Webhook Playground](#41-webhook-playground)
  - [4.2 Notification Search & Filtering](#42-notification-search--filtering)
  - [4.3 React Notification Bell Component](#43-react-notification-bell-component)
  - [4.4 Template Versioning UI](#44-template-versioning-ui)
  - [4.5 Analytics Dashboard](#45-analytics-dashboard)
  - [4.6 Provider Fallback Chains](#46-provider-fallback-chains)

---

## Phase 1 — Quick Wins

### 1.1 Enable Template Editing in UI

**Problem**: `AppTemplates.tsx` has Create and Delete but no Edit. Users must delete and re-create templates to fix a typo, losing the template UUID and breaking all notification references.

**Backend Status**: `PUT /v1/templates/:id` already works. `TemplateHandler.UpdateTemplate` parses `dto.UpdateTemplateRequest` and calls `TemplateService.Update`. No backend changes needed.

**Files to Modify**:
- `ui/src/components/AppTemplates.tsx`

**Implementation**:

1. Add editing state alongside existing `showAddForm`:

```tsx
// New state
const [editingTemplate, setEditingTemplate] = useState<Template | null>(null);
```

2. When Edit is clicked, pre-populate `formData` from the selected template:

```tsx
const handleEdit = (template: Template) => {
  setEditingTemplate(template);
  setFormData({
    app_id: appId,
    name: template.name,
    channel: template.channel,
    webhook_target: template.webhook_target || '',
    subject: template.subject || '',
    body: template.body,
    description: template.description || '',
    variables: template.variables || [],
  });
  setShowAddForm(true);
};
```

3. Modify the submit handler to call `update` when `editingTemplate` is set:

```tsx
const handleSubmit = async () => {
  try {
    if (editingTemplate) {
      await templatesAPI.update(apiKey, editingTemplate.id, {
        description: formData.description,
        webhook_target: formData.webhook_target,
        subject: formData.subject,
        body: formData.body,
        variables: formData.variables,
      });
    } else {
      await templatesAPI.create(apiKey, formData);
    }
    setShowAddForm(false);
    setEditingTemplate(null);
    resetForm();
    fetchTemplates();
  } catch (err) { /* handle error */ }
};
```

4. Add Edit button to each template row (next to Delete):

```tsx
<Button variant="outline" size="sm" onClick={() => handleEdit(template)}>
  Edit
</Button>
```

5. Clear `editingTemplate` when form is cancelled:

```tsx
const handleCancel = () => {
  setShowAddForm(false);
  setEditingTemplate(null);
  resetForm();
};
```

6. Disable the `name` and `channel` fields when editing (these are immutable to avoid breaking references):

```tsx
<Input
  value={formData.name}
  onChange={...}
  disabled={!!editingTemplate}
/>
<Select
  value={formData.channel}
  onValueChange={...}
  disabled={!!editingTemplate}
/>
```

**Testing**: Create template → Edit body → Verify template ID unchanged → Send notification with same template ID → Verify new body used.

---

### 1.2 Default Locale to "en"

**Problem**: `locale` is `validate:"required"` in `CreateTemplateRequest`, forcing every template to specify a locale even for single-language apps.

**Files to Modify**:
- `internal/interfaces/http/dto/template_dto.go`
- `internal/usecases/template_service.go`

**Changes**:

1. **DTO**: Change validation tag from `required` to `omitempty`:

```go
// In dto.CreateTemplateRequest
Locale string `json:"locale" validate:"omitempty,min=2,max=10"`
```

2. **Service**: Apply default in `TemplateService.Create` before saving:

```go
func (s *TemplateService) Create(ctx context.Context, req *templateDomain.CreateRequest) (*templateDomain.Template, error) {
    // Apply default locale
    if req.Locale == "" {
        req.Locale = "en"
    }

    // ... existing validation and creation logic
}
```

3. **Handler**: The `CreateTemplateVersion` handler fetches locale from query with default `"en-US"` — standardize this to `"en"`:

```go
locale := c.Query("locale", "en")
```

**Impact**: Existing templates with explicit locale are unaffected. New templates without `locale` default to `"en"`.

---

### 1.3 Auto-Detect Template Variables

**Problem**: Users must manually list `variables: ["name", "product"]` in the template body AND body must use `{{.name}}`, `{{.product}}`. These desync easily. The UI auto-detects and adds but never removes stale variables.

**Files to Modify**:
- `internal/usecases/template_service.go` — `Create` and `Update` methods
- `internal/interfaces/http/dto/template_dto.go` — `CreateTemplateRequest`
- `ui/src/components/AppTemplates.tsx` — remove manual variable input

**Backend Changes**:

1. Add `extractVariables` helper to `TemplateService`:

```go
// extractVariables parses Go template variables ({{.varName}}) from body text.
// Returns a deduplicated, sorted slice of variable names.
func extractVariables(body string) []string {
    re := regexp.MustCompile(`\{\{\s*\.(\w+)\s*\}\}`)
    matches := re.FindAllStringSubmatch(body, -1)

    seen := make(map[string]struct{})
    var vars []string
    for _, match := range matches {
        if len(match) > 1 {
            name := match[1]
            if _, exists := seen[name]; !exists {
                seen[name] = struct{}{}
                vars = append(vars, name)
            }
        }
    }
    sort.Strings(vars)
    return vars
}
```

2. In `Create`, auto-detect when `variables` is empty or nil:

```go
func (s *TemplateService) Create(ctx context.Context, req *templateDomain.CreateRequest) (*templateDomain.Template, error) {
    if req.Locale == "" {
        req.Locale = "en"
    }

    // Auto-detect variables from body if not explicitly provided
    if len(req.Variables) == 0 {
        req.Variables = extractVariables(req.Body)
    }

    // ... existing validation
}
```

3. In `Update`, re-detect when body changes and `variables` isn't explicitly provided:

```go
if req.Body != nil && *req.Body != "" {
    tmpl.Body = *req.Body
    // Auto-detect variables from new body if variables not explicitly overridden
    if req.Variables == nil || len(*req.Variables) == 0 {
        detected := extractVariables(tmpl.Body)
        tmpl.Variables = detected
    }
    // Validate (existing call)
    if err := s.validateTemplateVariables(tmpl.Body, tmpl.Variables); err != nil {
        return nil, fmt.Errorf("template validation failed: %w", err)
    }
}
```

4. Make `variables` optional in DTO:

```go
// In dto.CreateTemplateRequest — no change needed, already has no validate tag
Variables []string `json:"variables"`
```

**UI Changes** (in `AppTemplates.tsx`):

1. Remove the manual variable input (`varInput` state, the "Add Variable" button, the variable chips).
2. Show auto-detected variables as read-only chips below the body textarea:

```tsx
const detectedVars = useMemo(() => {
  const matches = formData.body.matchAll(/{{\s*\.?(\w+)\s*}}/g);
  return [...new Set(Array.from(matches, m => m[1]))];
}, [formData.body]);

// In the form:
{detectedVars.length > 0 && (
  <div>
    <Label>Detected Variables (auto)</Label>
    <div className="flex gap-1 flex-wrap">
      {detectedVars.map(v => (
        <Badge key={v} variant="secondary">{v}</Badge>
      ))}
    </div>
  </div>
)}
```

3. Remove `variables` from `formData` state and the submit payload — let the backend handle it.

**Backward Compatibility**: Templates with explicitly provided `variables` still work. The auto-detect only activates when `variables` is empty/nil.

---

### 1.4 Fix Multi-Send to Use Bulk API

**Problem**: `AppNotifications.tsx` multi-user send fires parallel individual `POST /v1/notifications` per user via `Promise.all`. This is inefficient, causes partial failures without clear feedback, and ignores the existing `POST /v1/notifications/bulk` endpoint.

**Files to Modify**:
- `ui/src/components/AppNotifications.tsx`

**Current Code** (approximate):
```tsx
// For multi-user send:
const promises = selectedUsers.map(userId =>
  notificationsAPI.send(apiKey, { ...form, user_id: userId })
);
await Promise.all(promises);
```

**Replace With**:
```tsx
// For multi-user send:
if (selectedUsers.length > 1) {
  const bulkPayload = {
    user_ids: selectedUsers,
    channel: form.channel,
    priority: form.priority,
    title: form.title,
    body: form.body,
    template_id: form.template_id,
    data: form.data,
    category: form.category,
    scheduled_at: form.scheduled_at,
  };
  const response = await notificationsAPI.sendBulk(apiKey, bulkPayload);
  toast.success(`Sent ${response.data.sent} of ${response.data.total} notifications`);
} else {
  await notificationsAPI.send(apiKey, { ...form, user_id: selectedUsers[0] });
  toast.success('Notification sent');
}
```

**API service** (`api.ts`): `notificationsAPI.sendBulk` already exists — it calls `POST /v1/notifications/bulk`.

**Testing**: Select 5 users → Send → Verify single API call in network tab → Verify `BulkSendResponse` with `sent` and `total` counts.

---

### 1.5 Add Phone Field to User UI

**Problem**: Backend supports `phone` field on `User`, and the SMS channel needs it, but `AppUsers.tsx` doesn't show a phone input.

**Files to Modify**:
- `ui/src/components/AppUsers.tsx`

**Changes**:

1. Add `phone` to form state:

```tsx
const [formData, setFormData] = useState<CreateUserRequest>({
  email: '',
  phone: '',      // NEW
  timezone: 'UTC',
  language: 'en',
  // ... existing preferences
});
```

2. Add phone input field after email, before timezone:

```tsx
<div className="space-y-1">
  <Label htmlFor="phone">Phone Number</Label>
  <Input
    id="phone"
    type="tel"
    placeholder="+1 555 0100"
    value={formData.phone}
    onChange={(e) => setFormData(prev => ({ ...prev, phone: e.target.value }))}
  />
</div>
```

3. Include `phone` in the submit payload — already handled since `CreateUserRequest` is spread into the API call.

4. Show phone in the user table:

```tsx
<TableHead>Phone</TableHead>
// ...
<TableCell>{user.phone || '—'}</TableCell>
```

5. Pre-fill phone when editing:

```tsx
const handleEdit = (user: User) => {
  setFormData({
    email: user.email || '',
    phone: user.phone || '',  // NEW
    timezone: user.timezone || 'UTC',
    // ...
  });
};
```

**Backend**: No changes needed. `dto.CreateUserRequest` already has `Phone string` and `UserHandler.Create` already maps it to `user.User`.

---

### 1.6 Remove Debug fmt.Printf Statements

**Problem**: `notification_handler.go` has `fmt.Printf("DEBUG: ...")` calls that leak raw request bodies (potentially containing sensitive data) to stdout in production.

**Files to Modify**:
- `internal/interfaces/http/handlers/notification_handler.go`

**Changes**: In the `Send` method, remove:

```go
// DELETE these lines:
fmt.Printf("DEBUG: Raw request body: %s, app_id: %s\n", string(body), appID)
// ...
fmt.Printf("DEBUG: BodyParser error: %v\n", err)
```

The existing `h.logger.Debug(...)` calls with zap fields are correct and should remain — they're only emitted at debug log level.

**Verify**: Search the entire codebase for `fmt.Printf` and `fmt.Println` in handler files. Replace any remaining instances with `logger.Debug()`.

```bash
grep -rn "fmt.Printf\|fmt.Println" internal/interfaces/ cmd/
```

---

### 1.7 Add Pagination to All List Views

**Problem**: `AppUsers.tsx`, `AppTemplates.tsx`, and `AppNotifications.tsx` fetch all records at once with no pagination. This breaks at scale.

**Files to Modify**:
- `ui/src/components/AppUsers.tsx`
- `ui/src/components/AppTemplates.tsx`
- `ui/src/components/AppNotifications.tsx`
- `ui/src/services/api.ts` — add pagination params to list calls

#### 1.7.1 — Backend (No changes needed)

The backend already supports pagination:
- `GET /v1/users?page=1&page_size=20` → returns `{ users, total_count, page, page_size }`
- `GET /v1/templates?limit=50&offset=0` → returns `{ templates, total, limit, offset }`
- `GET /v1/notifications?page=1&page_size=50` → returns `{ notifications, total, page, page_size }`

#### 1.7.2 — API Service Changes

Add pagination params to `api.ts`:

```typescript
// usersAPI
list: (apiKey: string, page = 1, pageSize = 20) =>
  api.get(`/users?page=${page}&page_size=${pageSize}`, { headers: getAuthHeaders(apiKey) }),

// templatesAPI
list: (apiKey: string, limit = 20, offset = 0) =>
  api.get(`/templates?limit=${limit}&offset=${offset}`, { headers: getAuthHeaders(apiKey) }),

// notificationsAPI
list: (apiKey: string, page = 1, pageSize = 20) =>
  api.get(`/notifications?page=${page}&page_size=${pageSize}`, { headers: getAuthHeaders(apiKey) }),
```

#### 1.7.3 — Shared Pagination Component

Create `ui/src/components/Pagination.tsx`:

```tsx
interface PaginationProps {
  currentPage: number;
  totalItems: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  onPageSizeChange?: (size: number) => void;
}

export function Pagination({ currentPage, totalItems, pageSize, onPageChange, onPageSizeChange }: PaginationProps) {
  const totalPages = Math.ceil(totalItems / pageSize);

  return (
    <div className="flex items-center justify-between py-4">
      <span className="text-sm text-muted-foreground">
        {totalItems} total · Page {currentPage} of {totalPages}
      </span>
      <div className="flex gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={currentPage <= 1}
          onClick={() => onPageChange(currentPage - 1)}
        >
          Previous
        </Button>
        <Button
          variant="outline"
          size="sm"
          disabled={currentPage >= totalPages}
          onClick={() => onPageChange(currentPage + 1)}
        >
          Next
        </Button>
      </div>
    </div>
  );
}
```

#### 1.7.4 — Per-Component Wiring (AppUsers example)

```tsx
const [page, setPage] = useState(1);
const [pageSize] = useState(20);
const [totalCount, setTotalCount] = useState(0);

const fetchUsers = async () => {
  const res = await usersAPI.list(apiKey, page, pageSize);
  setUsers(res.data.data.users);
  setTotalCount(res.data.data.total_count);
};

useEffect(() => { fetchUsers(); }, [page]);

// After the table:
<Pagination
  currentPage={page}
  totalItems={totalCount}
  pageSize={pageSize}
  onPageChange={setPage}
/>
```

Apply the same pattern to `AppTemplates.tsx` (using `limit`/`offset`) and `AppNotifications.tsx` (using `page`/`page_size`).

---

## Phase 2 — Core Simplifications

### 2.1 Quick-Send API Endpoint

**Problem**: Sending a notification requires knowing internal UUIDs for both user and template. A new developer needs 6 API calls before their first notification.

**Goal**: A single `POST /v1/quick-send` that accepts human-readable identifiers and inline content.

#### New Files

| File | Purpose |
|------|---------|
| `internal/interfaces/http/dto/quick_send_dto.go` | Request/response DTOs |
| `internal/interfaces/http/handlers/quick_send_handler.go` | Handler logic |
| `internal/usecases/quick_send_service.go` | Orchestration: resolve user, resolve template, delegate to notification service |

#### DTO: `quick_send_dto.go`

```go
package dto

import "time"

// QuickSendRequest is a simplified notification request that accepts
// human-readable identifiers instead of internal UUIDs.
type QuickSendRequest struct {
    // Recipient: email address, external user ID, or internal UUID.
    // If recipient doesn't exist as a user, auto-creates one (email only).
    To string `json:"to" validate:"required"`

    // Channel: push, email, sms, webhook, in_app, sse.
    // Optional if Template is specified (inferred from template).
    Channel string `json:"channel" validate:"omitempty,oneof=push email sms webhook in_app sse"`

    // Template reference: name (string) or UUID.
    // Optional if Subject+Body are provided (inline content).
    Template string `json:"template,omitempty"`

    // Inline content (used when Template is empty).
    Subject string `json:"subject,omitempty"`
    Body    string `json:"body,omitempty"`

    // Template variables (used when Template is specified).
    Data map[string]interface{} `json:"data,omitempty"`

    // Priority: low, normal, high, critical. Defaults to "normal".
    Priority string `json:"priority,omitempty"`

    // Optional scheduling
    ScheduledAt *time.Time `json:"scheduled_at,omitempty"`

    // Optional: explicit webhook URL (for webhook channel without user)
    WebhookURL string `json:"webhook_url,omitempty"`
}

// QuickSendResponse is the response for quick-send.
type QuickSendResponse struct {
    NotificationID string `json:"notification_id"`
    Status         string `json:"status"`
    UserID         string `json:"user_id"`
    Channel        string `json:"channel"`
    Message        string `json:"message"`
}
```

#### Handler: `quick_send_handler.go`

```go
package handlers

import (
    "github.com/gofiber/fiber/v2"
    "github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
    "github.com/the-monkeys/freerangenotify/internal/usecases"
    "github.com/the-monkeys/freerangenotify/pkg/validator"
    "go.uber.org/zap"
)

type QuickSendHandler struct {
    service   *usecases.QuickSendService
    validator *validator.Validator
    logger    *zap.Logger
}

func NewQuickSendHandler(service *usecases.QuickSendService, v *validator.Validator, logger *zap.Logger) *QuickSendHandler {
    return &QuickSendHandler{service: service, validator: v, logger: logger}
}

// Send handles POST /v1/quick-send
func (h *QuickSendHandler) Send(c *fiber.Ctx) error {
    appID := c.Locals("app_id").(string)

    var req dto.QuickSendRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
    }

    if err := h.validator.Validate(req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
    }

    result, err := h.service.Send(c.Context(), appID, &req)
    if err != nil {
        h.logger.Error("Quick-send failed", zap.Error(err))
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
    }

    return c.Status(fiber.StatusAccepted).JSON(result)
}
```

#### Service: `quick_send_service.go`

```go
package usecases

import (
    "context"
    "fmt"
    "regexp"
    "strings"

    "github.com/google/uuid"
    "github.com/the-monkeys/freerangenotify/internal/domain/notification"
    templateDomain "github.com/the-monkeys/freerangenotify/internal/domain/template"
    "github.com/the-monkeys/freerangenotify/internal/domain/user"
    "github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
    "go.uber.org/zap"
)

type QuickSendService struct {
    notificationService notification.Service
    userRepo            user.Repository
    templateRepo        templateDomain.Repository
    templateService     *TemplateService
    logger              *zap.Logger
}

func NewQuickSendService(
    notifSvc notification.Service,
    userRepo user.Repository,
    tmplRepo templateDomain.Repository,
    tmplSvc *TemplateService,
    logger *zap.Logger,
) *QuickSendService {
    return &QuickSendService{
        notificationService: notifSvc,
        userRepo:            userRepo,
        templateRepo:        tmplRepo,
        templateService:     tmplSvc,
        logger:              logger,
    }
}

func (s *QuickSendService) Send(ctx context.Context, appID string, req *dto.QuickSendRequest) (*dto.QuickSendResponse, error) {
    // 1. Resolve recipient
    userID, err := s.resolveRecipient(ctx, appID, req.To)
    if err != nil {
        return nil, fmt.Errorf("failed to resolve recipient %q: %w", req.To, err)
    }

    // 2. Resolve template (or use inline content)
    var templateID string
    var channel notification.Channel

    if req.Template != "" {
        tmpl, err := s.resolveTemplate(ctx, appID, req.Template)
        if err != nil {
            return nil, fmt.Errorf("failed to resolve template %q: %w", req.Template, err)
        }
        templateID = tmpl.ID
        channel = notification.Channel(tmpl.Channel)
    } else if req.Body != "" {
        // Inline content: create a transient template
        tmpl, err := s.createTransientTemplate(ctx, appID, req)
        if err != nil {
            return nil, fmt.Errorf("failed to create inline template: %w", err)
        }
        templateID = tmpl.ID
        channel = notification.Channel(tmpl.Channel)
    } else {
        return nil, fmt.Errorf("either 'template' or 'body' must be provided")
    }

    // 3. Channel: explicit > inferred from template
    if req.Channel != "" {
        channel = notification.Channel(req.Channel)
    }

    // 4. Priority: default to "normal"
    priority := notification.PriorityNormal
    if req.Priority != "" {
        priority = notification.Priority(req.Priority)
    }

    // 5. Build and send
    sendReq := notification.SendRequest{
        AppID:       appID,
        UserID:      userID,
        Channel:     channel,
        Priority:    priority,
        TemplateID:  templateID,
        Data:        req.Data,
        ScheduledAt: req.ScheduledAt,
    }

    notif, err := s.notificationService.Send(ctx, sendReq)
    if err != nil {
        return nil, err
    }

    return &dto.QuickSendResponse{
        NotificationID: notif.NotificationID,
        Status:         string(notif.Status),
        UserID:         userID,
        Channel:        string(channel),
        Message:        "Notification accepted for delivery",
    }, nil
}

// resolveRecipient resolves a "to" value to an internal user UUID.
// Accepts: email address, UUID, or external user ID.
// If email and user doesn't exist, auto-creates.
func (s *QuickSendService) resolveRecipient(ctx context.Context, appID, to string) (string, error) {
    // Check if it's a UUID (existing internal ID)
    if _, err := uuid.Parse(to); err == nil {
        if _, err := s.userRepo.GetByID(ctx, to); err == nil {
            return to, nil
        }
    }

    // Check if it's an email
    emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
    if emailRegex.MatchString(to) {
        // Try to find existing user by email
        existing, err := s.userRepo.GetByEmail(ctx, appID, to)
        if err == nil && existing != nil {
            return existing.UserID, nil
        }

        // Auto-create user
        newUser := &user.User{
            UserID: uuid.New().String(),
            AppID:  appID,
            Email:  to,
            Preferences: user.Preferences{
                EmailEnabled: boolPtr(true),
                PushEnabled:  boolPtr(true),
                SMSEnabled:   boolPtr(true),
            },
        }
        if err := s.userRepo.Create(ctx, newUser); err != nil {
            return "", fmt.Errorf("failed to auto-create user: %w", err)
        }
        s.logger.Info("Auto-created user for quick-send",
            zap.String("email", to),
            zap.String("user_id", newUser.UserID))
        return newUser.UserID, nil
    }

    // Treat as external user ID — scan users (future: add indexed lookup)
    // For now, return error suggesting to use email or UUID
    return "", fmt.Errorf("recipient %q not found. Use an email address (auto-creates user) or internal UUID", to)
}

// resolveTemplate resolves a template reference by name or UUID.
func (s *QuickSendService) resolveTemplate(ctx context.Context, appID, ref string) (*templateDomain.Template, error) {
    // Try UUID first
    if _, err := uuid.Parse(ref); err == nil {
        tmpl, err := s.templateRepo.GetByID(ctx, ref)
        if err == nil && tmpl.AppID == appID {
            return tmpl, nil
        }
    }

    // Try by name (latest active, default locale "en")
    tmpl, err := s.templateRepo.GetByAppAndName(ctx, appID, ref, "en")
    if err == nil {
        return tmpl, nil
    }

    // Try with empty locale (catch-all)
    tmpl, err = s.templateRepo.GetByAppAndName(ctx, appID, ref, "")
    if err == nil {
        return tmpl, nil
    }

    return nil, fmt.Errorf("template %q not found", ref)
}

// createTransientTemplate creates a system-managed template for inline content.
func (s *QuickSendService) createTransientTemplate(ctx context.Context, appID string, req *dto.QuickSendRequest) (*templateDomain.Template, error) {
    ch := req.Channel
    if ch == "" {
        ch = "email" // Default channel for inline content
    }

    // Generate a deterministic name based on content hash to deduplicate
    name := fmt.Sprintf("_inline_%s", uuid.New().String()[:8])

    createReq := &templateDomain.CreateRequest{
        AppID:     appID,
        Name:      name,
        Channel:   ch,
        Subject:   req.Subject,
        Body:      req.Body,
        Locale:    "en",
        CreatedBy: "system:quick-send",
    }

    return s.templateService.Create(ctx, createReq)
}

func boolPtr(b bool) *bool { return &b }
```

#### Container Wiring

In `internal/container/container.go`, add:

```go
// Add to Container struct
QuickSendService *usecases.QuickSendService
QuickSendHandler *handlers.QuickSendHandler

// In NewContainer, after existing services:
c.QuickSendService = usecases.NewQuickSendService(
    c.NotificationService,
    repos.User,
    repos.Template,
    c.TemplateService,
    c.Logger,
)
c.QuickSendHandler = handlers.NewQuickSendHandler(
    c.QuickSendService,
    c.Validator,
    c.Logger,
)
```

#### Route Registration

In `internal/interfaces/http/routes/routes.go`, inside `setupProtectedRoutes`:

```go
// Quick-send (simplified notification endpoint)
v1.Post("/quick-send", auth, c.QuickSendHandler.Send)
```

#### Sequence Diagram

```
Client                     QuickSendHandler    QuickSendService    UserRepo    TemplateRepo    NotificationService
  |                              |                   |                |             |                  |
  | POST /v1/quick-send         |                   |                |             |                  |
  | { to: "j@x.com",           |                   |                |             |                  |
  |   template: "welcome",     |                   |                |             |                  |
  |   data: {name: "John"} }   |                   |                |             |                  |
  |----------------------------->                   |                |             |                  |
  |                              | Send(ctx, appID, req)             |             |                  |
  |                              |------------------>                |             |                  |
  |                              |                   | resolveRecipient("j@x.com") |                  |
  |                              |                   |--------------->             |                  |
  |                              |                   |  GetByEmail(appID, "j@x.com")                  |
  |                              |                   |<-- user.UserID -------------|                  |
  |                              |                   |                |             |                  |
  |                              |                   | resolveTemplate("welcome")  |                  |
  |                              |                   |------------------------------>                 |
  |                              |                   |  GetByAppAndName(appID, "welcome", "en")       |
  |                              |                   |<-- template.ID, Channel ----|                  |
  |                              |                   |                |             |                  |
  |                              |                   | notificationService.Send(sendReq)              |
  |                              |                   |------------------------------------------------>
  |                              |                   |<-- *Notification ----------------------------- |
  |                              |<-- QuickSendResponse              |             |                  |
  |<---- 202 Accepted -----------|                   |                |             |                  |
```

---

### 2.2 Send by Email / External ID

**Problem**: `POST /v1/notifications` requires `user_id` as an internal UUID. Users must perform a lookup before every send.

**Goal**: Allow `user_id` to accept email addresses or external IDs alongside UUIDs.

**Files to Modify**:
- `internal/usecases/notification_service.go` — `Send` method
- `internal/domain/notification/models.go` — `SendRequest.Validate`

**Approach**: Add a resolution layer at the start of `NotificationService.Send` — before the existing user lookup.

```go
// In NotificationService.Send, before existing user fetch:

func (s *NotificationService) Send(ctx context.Context, req notification.SendRequest) (*notification.Notification, error) {
    // ...existing validation...

    // Resolve user_id if it's not a UUID
    if req.UserID != "" {
        resolvedID, err := s.resolveUserID(ctx, req.AppID, req.UserID)
        if err != nil {
            return nil, fmt.Errorf("failed to resolve user: %w", err)
        }
        req.UserID = resolvedID
    }

    // ...existing logic continues unchanged...
}

// resolveUserID converts email/external ID to internal UUID.
func (s *NotificationService) resolveUserID(ctx context.Context, appID, identifier string) (string, error) {
    // If it parses as UUID, use directly
    if _, err := uuid.Parse(identifier); err == nil {
        return identifier, nil
    }

    // Try email lookup
    if strings.Contains(identifier, "@") {
        u, err := s.userRepo.GetByEmail(ctx, appID, identifier)
        if err == nil {
            return u.UserID, nil
        }
    }

    // Identifier doesn't resolve — return descriptive error
    return "", fmt.Errorf("user %q not found; use a valid email or internal UUID", identifier)
}
```

**Validation Change**: In `SendRequest.Validate`, relax the UUID format check on `UserID`:

```go
// Current: ErrInvalidUserID if UserID is empty (for non-webhook)
// Change: Keep the same check (non-empty), but remove any UUID format validation.
// The resolution layer in Send() handles format conversion.
```

**Impact**: Fully backward compatible. Existing UUID-based calls work unchanged. New email-based calls resolve before hitting the user repo.

---

### 2.3 Template Reference by Name

**Problem**: `template_id` in send requests requires an internal UUID. Users must look up the template UUID after creating it by name.

**Goal**: Allow `template_id` to accept template names as well as UUIDs.

**Files to Modify**:
- `internal/usecases/notification_service.go` — `Send` method

**Implementation**: Add template resolution before the existing `templateRepo.GetByID` call.

```go
// In NotificationService.Send, replace:
//   tmpl, err := s.templateRepo.GetByID(ctx, req.TemplateID)
// With:

tmpl, err := s.resolveTemplate(ctx, req.AppID, req.TemplateID)

// New method:
func (s *NotificationService) resolveTemplate(ctx context.Context, appID, ref string) (*template.Template, error) {
    // Try UUID first
    if _, err := uuid.Parse(ref); err == nil {
        tmpl, err := s.templateRepo.GetByID(ctx, ref)
        if err == nil {
            return tmpl, nil
        }
    }

    // Try by name (default locale "en")
    tmpl, err := s.templateRepo.GetByAppAndName(ctx, appID, ref, "en")
    if err == nil {
        return tmpl, nil
    }

    // Try by name (empty locale — any locale)
    tmpl, err = s.templateRepo.GetByAppAndName(ctx, appID, ref, "")
    if err == nil {
        return tmpl, nil
    }

    return nil, notification.ErrTemplateNotFound
}
```

**Backward Compatibility**: UUID references continue to work. New name-based references are additive.

---

### 2.4 Infer Channel from Template

**Problem**: When sending a notification with a `template_id`, the user must also specify `channel`, which is redundant since the template already declares its channel.

**Files to Modify**:
- `internal/interfaces/http/dto/notification_dto.go` — `SendNotificationRequest`
- `internal/domain/notification/models.go` — `SendRequest.Validate`
- `internal/usecases/notification_service.go` — `Send` method

**Changes**:

1. **DTO**: Make `channel` optional:

```go
// In dto.SendNotificationRequest
Channel string `json:"channel" validate:"omitempty,oneof=push email sms webhook in_app sse"`
```

2. **Domain Validate**: Allow empty channel if template is provided:

```go
// In SendRequest.Validate
func (r *SendRequest) Validate() error {
    if r.AppID == "" {
        return ErrInvalidAppID
    }
    // Channel is optional when TemplateID is present (will be inferred)
    if r.Channel != "" && !r.Channel.Valid() {
        return ErrInvalidChannel
    }
    if r.Channel == "" && r.TemplateID == "" {
        return ErrInvalidChannel // Must have one or the other
    }
    // ...rest unchanged
}
```

3. **Service**: Infer channel after template resolution:

```go
// In NotificationService.Send, after resolving template:
tmpl, err := s.resolveTemplate(ctx, req.AppID, req.TemplateID)
if err != nil {
    return nil, notification.ErrTemplateNotFound
}

// Infer channel from template if not explicitly set
if req.Channel == "" {
    req.Channel = notification.Channel(tmpl.Channel)
    s.logger.Debug("Inferred channel from template",
        zap.String("template_id", req.TemplateID),
        zap.String("channel", string(req.Channel)))
}

title := tmpl.Subject
body := tmpl.Body
```

**API Examples**:

```json
// Before (required both):
{ "channel": "email", "template_id": "uuid-123", ... }

// After (channel optional when template specified):
{ "template_id": "uuid-123", ... }
// Channel inferred from template

// Still works if explicit:
{ "channel": "sms", "template_id": "uuid-123", ... }
// Explicit channel wins (useful for cross-channel sends)
```

---

### 2.5 Simplified Send Form in UI

**Problem**: `AppNotifications.tsx` crams everything into one form: single send, multi-user, broadcast, webhook targets, scheduling, recurrence. High cognitive load.

**Files to Modify**:
- `ui/src/components/AppNotifications.tsx`

**Design**: Split into 3 sub-views with tabs:

```
┌──────────────────────────────────────────────────────┐
│  [Quick Send]    [Advanced]    [Broadcast]            │
├──────────────────────────────────────────────────────┤
│                                                      │
│  Quick Send:                                         │
│  ┌──────────────────────────────────────────┐       │
│  │ To:      [john@example.com          ]     │       │
│  │ Template:[Welcome Email         ▼   ]     │       │
│  │                                           │       │
│  │ Variables:                                │       │
│  │ name:    [John                      ]     │       │
│  │ product: [Acme                      ]     │       │
│  │                                           │       │
│  │          [Send Notification →]            │       │
│  └──────────────────────────────────────────┘       │
│                                                      │
│  ─── Notification History ───                        │
│  (same table, now with pagination)                   │
└──────────────────────────────────────────────────────┘
```

**Implementation**:

1. Extract the current form into `<AdvancedSendForm>` component (unchanged logic).

2. Create `<QuickSendForm>` component:

```tsx
interface QuickSendFormProps {
  apiKey: string;
  templates: Template[];
  onSent: () => void;
}

function QuickSendForm({ apiKey, templates, onSent }: QuickSendFormProps) {
  const [to, setTo] = useState('');
  const [templateId, setTemplateId] = useState('');
  const [data, setData] = useState<Record<string, string>>({});
  const [sending, setSending] = useState(false);

  // When template changes, populate variable fields from template.variables
  const selectedTemplate = templates.find(t => t.id === templateId);
  const variables = selectedTemplate?.variables || [];

  const handleSend = async () => {
    setSending(true);
    try {
      // Use quick-send endpoint
      await api.post('/quick-send', {
        to,
        template: selectedTemplate?.name || templateId,
        data,
      }, { headers: getAuthHeaders(apiKey) });
      toast.success('Notification sent!');
      onSent();
    } catch (err) {
      toast.error('Send failed');
    }
    setSending(false);
  };

  return (
    <div className="space-y-4">
      <div>
        <Label>To (email or user ID)</Label>
        <Input value={to} onChange={e => setTo(e.target.value)} placeholder="john@example.com" />
      </div>
      <div>
        <Label>Template</Label>
        <Select value={templateId} onValueChange={setTemplateId}>
          {templates.map(t => <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>)}
        </Select>
      </div>
      {variables.map(v => (
        <div key={v}>
          <Label>{v}</Label>
          <Input value={data[v] || ''} onChange={e => setData(d => ({ ...d, [v]: e.target.value }))} />
        </div>
      ))}
      <Button onClick={handleSend} disabled={sending || !to || !templateId}>
        Send Notification
      </Button>
    </div>
  );
}
```

3. Create `<BroadcastForm>` — simplified from existing broadcast logic, just template + data + confirm.

4. Wire with `Tabs` component:

```tsx
<Tabs defaultValue="quick">
  <TabsList>
    <TabsTrigger value="quick">Quick Send</TabsTrigger>
    <TabsTrigger value="advanced">Advanced</TabsTrigger>
    <TabsTrigger value="broadcast">Broadcast</TabsTrigger>
  </TabsList>
  <TabsContent value="quick"><QuickSendForm ... /></TabsContent>
  <TabsContent value="advanced"><AdvancedSendForm ... /></TabsContent>
  <TabsContent value="broadcast"><BroadcastForm ... /></TabsContent>
</Tabs>
```

---

### 2.6 Timezone / Language Dropdowns

**Problem**: `AppUsers.tsx` uses free-text inputs for timezone and language, inviting typos.

**Files to Modify**:
- `ui/src/components/AppUsers.tsx`

**Implementation**:

1. Define constant arrays (or import from a utility):

```tsx
const TIMEZONES = [
  'UTC', 'America/New_York', 'America/Chicago', 'America/Denver',
  'America/Los_Angeles', 'America/Toronto', 'Europe/London',
  'Europe/Paris', 'Europe/Berlin', 'Asia/Tokyo', 'Asia/Shanghai',
  'Asia/Kolkata', 'Asia/Dubai', 'Australia/Sydney', 'Pacific/Auckland',
];

const LANGUAGES = [
  { code: 'en', label: 'English' },
  { code: 'es', label: 'Spanish' },
  { code: 'fr', label: 'French' },
  { code: 'de', label: 'German' },
  { code: 'pt', label: 'Portuguese' },
  { code: 'zh', label: 'Chinese' },
  { code: 'ja', label: 'Japanese' },
  { code: 'ko', label: 'Korean' },
  { code: 'ar', label: 'Arabic' },
  { code: 'hi', label: 'Hindi' },
];
```

2. Replace `<Input>` with `<Select>`:

```tsx
<div>
  <Label>Timezone</Label>
  <Select value={formData.timezone} onValueChange={v => setFormData(p => ({ ...p, timezone: v }))}>
    <SelectTrigger><SelectValue /></SelectTrigger>
    <SelectContent>
      {TIMEZONES.map(tz => <SelectItem key={tz} value={tz}>{tz}</SelectItem>)}
    </SelectContent>
  </Select>
</div>

<div>
  <Label>Language</Label>
  <Select value={formData.language} onValueChange={v => setFormData(p => ({ ...p, language: v }))}>
    <SelectTrigger><SelectValue /></SelectTrigger>
    <SelectContent>
      {LANGUAGES.map(l => <SelectItem key={l.code} value={l.code}>{l.label} ({l.code})</SelectItem>)}
    </SelectContent>
  </Select>
</div>
```

**Backend**: No validation changes needed initially. The dropdown constrains input at the UI layer.

---

### 2.7 Unified Auth Documentation

**Problem**: JWT and API Key auth are documented in separate files (`AUTH_QUICKSTART.md` and `API_DOCUMENTATION.md`) with no cross-referencing. Users don't know when to use which.

**File to Create**:
- `documents/AUTHENTICATION_GUIDE.md`

**Content Structure**:

```markdown
# Authentication Guide

## Overview
FreeRangeNotify uses TWO authentication mechanisms:

| Mechanism | Format | Used For | Where Set |
|-----------|--------|----------|-----------|
| JWT Token | `Bearer eyJ...` | Admin dashboard, app CRUD | `/v1/auth/login` |
| API Key   | `Bearer frn_xxx` | Notifications, users, templates | App creation response |

## Decision Table
| I want to... | Use |
|--------------|-----|
| Create/manage apps | JWT |
| Send notifications | API Key |
| Manage users | API Key |
| Manage templates | API Key |
| View dashboard | JWT |
| Use quick-send API | API Key |

## JWT Flow (Admin)
1. Register: POST /v1/auth/register
2. Login: POST /v1/auth/login → { access_token, refresh_token }
3. Use access_token for /v1/apps/* and /v1/admin/* routes
4. Refresh: POST /v1/auth/refresh

## API Key Flow (Programmatic)
1. Create app (with JWT) → response includes api_key (frn_xxx)
2. Use api_key for all /v1/users/*, /v1/templates/*, /v1/notifications/*
3. API key doesn't expire (but can be regenerated)

## Both in One Session (Dashboard)
The web UI handles this automatically:
- Login → stores JWT in localStorage
- When you select an app → uses that app's API key for sub-resource calls
```

---

### 2.8 Bulk User Import Endpoint

**Problem**: Adding many users requires individual API calls. The backend has `user.Repository.BulkCreate` and `UserService.BulkCreate` but no HTTP handler.

**Files to Modify**:
- `internal/interfaces/http/dto/user_dto.go` — new DTO
- `internal/interfaces/http/handlers/user_handler.go` — new handler method
- `internal/interfaces/http/routes/routes.go` — new route

#### DTO

```go
// In user_dto.go
type BulkCreateUserRequest struct {
    Users []CreateUserRequest `json:"users" validate:"required,min=1,max=1000,dive"`
}

type BulkCreateUserResponse struct {
    Created int            `json:"created"`
    Total   int            `json:"total"`
    Errors  []BulkUserError `json:"errors,omitempty"`
}

type BulkUserError struct {
    Index   int    `json:"index"`
    Email   string `json:"email,omitempty"`
    Message string `json:"message"`
}
```

#### Handler

```go
// In user_handler.go
func (h *UserHandler) BulkCreate(c *fiber.Ctx) error {
    appID, ok := c.Locals("app_id").(string)
    if !ok || appID == "" {
        return errors.Unauthorized("Application not authenticated")
    }

    var req dto.BulkCreateUserRequest
    if err := c.BodyParser(&req); err != nil {
        return errors.BadRequest("Invalid request body")
    }

    if err := h.validator.Validate(req); err != nil {
        return errors.Validation("Validation failed", validator.FormatValidationErrors(err))
    }

    var users []*user.User
    var bulkErrors []dto.BulkUserError

    for i, ur := range req.Users {
        u := &user.User{
            UserID:     ur.UserID,
            AppID:      appID,
            Email:      ur.Email,
            Phone:      ur.Phone,
            Timezone:   ur.Timezone,
            Language:   ur.Language,
            WebhookURL: ur.WebhookURL,
        }
        if ur.Preferences != nil {
            u.Preferences = *ur.Preferences
        }

        // Validate individually
        if u.Email == "" && u.Phone == "" {
            bulkErrors = append(bulkErrors, dto.BulkUserError{
                Index: i, Email: ur.Email, Message: "email or phone required",
            })
            continue
        }
        users = append(users, u)
    }

    if len(users) > 0 {
        if err := h.service.BulkCreate(c.Context(), users); err != nil {
            return err
        }
    }

    return c.Status(fiber.StatusCreated).JSON(dto.BulkCreateUserResponse{
        Created: len(users),
        Total:   len(req.Users),
        Errors:  bulkErrors,
    })
}
```

#### Route

```go
// In routes.go, inside setupProtectedRoutes, users group:
users.Post("/bulk", c.UserHandler.BulkCreate)
```

---

## Phase 3 — Premium Experience

### 3.1 Rich HTML Template Editor

**Problem**: HTML email templates are pasted as raw strings into a JSON body field or a plain textarea. No preview, no formatting, no drag-and-drop.

**Approach**: Integrate [TipTap](https://tiptap.dev/) (MIT license) as a rich-text editor for email templates, with raw HTML fallback for advanced users.

**Files to Create/Modify**:
- `ui/src/components/TemplateEditor.tsx` — new component
- `ui/src/components/AppTemplates.tsx` — swap textarea for editor
- `ui/package.json` — add dependencies

#### Dependencies

```bash
npm install @tiptap/react @tiptap/starter-kit @tiptap/extension-color \
  @tiptap/extension-text-style @tiptap/extension-link @tiptap/extension-image \
  @tiptap/extension-text-align @tiptap/extension-placeholder
```

#### TemplateEditor Component

```tsx
// ui/src/components/TemplateEditor.tsx

import { useEditor, EditorContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Link from '@tiptap/extension-link';
import Image from '@tiptap/extension-image';
import TextAlign from '@tiptap/extension-text-align';
import Placeholder from '@tiptap/extension-placeholder';

interface TemplateEditorProps {
  content: string;
  onChange: (html: string) => void;
  channel: string;  // show rich editor only for 'email'
  placeholder?: string;
}

export function TemplateEditor({ content, onChange, channel, placeholder }: TemplateEditorProps) {
  const [mode, setMode] = useState<'visual' | 'html'>('visual');

  const editor = useEditor({
    extensions: [
      StarterKit,
      Link.configure({ openOnClick: false }),
      Image,
      TextAlign.configure({ types: ['heading', 'paragraph'] }),
      Placeholder.configure({ placeholder: placeholder || 'Write your template...' }),
    ],
    content,
    onUpdate: ({ editor }) => {
      onChange(editor.getHTML());
    },
  });

  // For non-email channels, show plain textarea
  if (channel !== 'email') {
    return (
      <textarea
        className="w-full min-h-[200px] font-mono text-sm p-3 border rounded"
        value={content}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
      />
    );
  }

  return (
    <div className="border rounded">
      <div className="flex items-center gap-2 p-2 border-b bg-muted/30">
        <Button variant={mode === 'visual' ? 'default' : 'ghost'} size="sm"
          onClick={() => setMode('visual')}>Visual</Button>
        <Button variant={mode === 'html' ? 'default' : 'ghost'} size="sm"
          onClick={() => setMode('html')}>HTML</Button>
        {mode === 'visual' && editor && (
          <>
            <Separator orientation="vertical" className="h-6" />
            <Button variant="ghost" size="sm"
              onClick={() => editor.chain().focus().toggleBold().run()}
              data-active={editor.isActive('bold')}>B</Button>
            <Button variant="ghost" size="sm"
              onClick={() => editor.chain().focus().toggleItalic().run()}
              data-active={editor.isActive('italic')}>I</Button>
            <Button variant="ghost" size="sm"
              onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}>H2</Button>
            <Button variant="ghost" size="sm"
              onClick={() => editor.chain().focus().toggleBulletList().run()}>List</Button>
          </>
        )}
      </div>
      {mode === 'visual' ? (
        <EditorContent editor={editor} className="p-4 min-h-[300px] prose max-w-none" />
      ) : (
        <textarea
          className="w-full min-h-[300px] font-mono text-sm p-3"
          value={content}
          onChange={(e) => {
            onChange(e.target.value);
            editor?.commands.setContent(e.target.value);
          }}
        />
      )}
    </div>
  );
}
```

#### Integration in AppTemplates.tsx

Replace the body `<Textarea>` with:

```tsx
<TemplateEditor
  content={formData.body}
  onChange={(html) => setFormData(p => ({ ...p, body: html }))}
  channel={formData.channel}
  placeholder="Write your notification template..."
/>
```

#### HTML Preview

Add a preview iframe below the editor for email templates:

```tsx
{formData.channel === 'email' && formData.body && (
  <div>
    <Label>Live Preview</Label>
    <iframe
      srcDoc={formData.body}
      className="w-full h-[400px] border rounded"
      sandbox=""
      title="Template Preview"
    />
  </div>
)}
```

**Backend**: Switch from `text/template` to `html/template` for XSS safety:

```go
// In template_service.go, renderTemplate method:
import "html/template"

func (s *TemplateService) renderTemplate(body string, data map[string]interface{}) (string, error) {
    tmpl, err := template.New("notification").Parse(body)
    // ... rest unchanged
}
```

Same change in `cmd/worker/processor.go` `renderTemplate` method.

---

### 3.2 Setup Wizard in UI

**Problem**: After creating an app, users face a blank dashboard with tabs they don't understand. No guided path to "first notification."

**Files to Create**:
- `ui/src/components/SetupWizard.tsx`

**Files to Modify**:
- `ui/src/pages/AppDetail.tsx` — show wizard for new apps

**Design**: A 4-step wizard shown when an app has 0 templates and 0 users.

```tsx
interface SetupWizardProps {
  appId: string;
  apiKey: string;
  onComplete: () => void;
}

type WizardStep = 'channel' | 'template' | 'recipient' | 'send';

export function SetupWizard({ appId, apiKey, onComplete }: SetupWizardProps) {
  const [step, setStep] = useState<WizardStep>('channel');
  const [channel, setChannel] = useState('email');
  const [templateData, setTemplateData] = useState({
    name: '', subject: '', body: '', variables: [] as string[]
  });
  const [recipient, setRecipient] = useState('');
  const [sendData, setSendData] = useState<Record<string, string>>({});

  // Step 1: Choose channel
  // Step 2: Create template (with TemplateEditor)
  // Step 3: Add recipient (email input → auto-creates user via quick-send)
  // Step 4: Preview and send (uses POST /v1/quick-send)

  return (
    <Card>
      <CardHeader>
        <CardTitle>Get Started</CardTitle>
        <Progress value={stepProgress} />
      </CardHeader>
      <CardContent>
        {step === 'channel' && <ChannelStep ... />}
        {step === 'template' && <TemplateStep ... />}
        {step === 'recipient' && <RecipientStep ... />}
        {step === 'send' && <SendStep ... />}
      </CardContent>
    </Card>
  );
}
```

**In AppDetail.tsx**:

```tsx
const [showWizard, setShowWizard] = useState(false);

// On load, check if app has templates/users:
useEffect(() => {
  const templates = await templatesAPI.list(app.api_key);
  const users = await usersAPI.list(app.api_key);
  if (templates.data.total === 0 && users.data.total_count === 0) {
    setShowWizard(true);
  }
}, []);

// Render:
{showWizard ? (
  <SetupWizard appId={app.app_id} apiKey={app.api_key} onComplete={() => setShowWizard(false)} />
) : (
  <Tabs ...> {/* existing tabs */} </Tabs>
)}
```

---

### 3.3 Pre-Built Template Library

**Problem**: Users start from a blank textarea. No reference for good template structure.

**Implementation**: Ship seed templates as JSON fixtures and expose a "Template Library" in the UI.

#### Seed Data File

Create `internal/seed/templates.go`:

```go
package seed

import "github.com/the-monkeys/freerangenotify/internal/domain/template"

// LibraryTemplates are pre-built templates users can clone into their apps.
var LibraryTemplates = []template.Template{
    {
        Name:        "welcome_email",
        Description: "Welcome email for new user signups",
        Channel:     "email",
        Subject:     "Welcome to {{.product}}, {{.name}}!",
        Body: `<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
  <h1 style="color: #333;">Welcome, {{.name}}!</h1>
  <p>Thank you for joining {{.product}}. We're excited to have you on board.</p>
  <a href="{{.cta_url}}" style="display: inline-block; padding: 12px 24px; background: #4F46E5; color: white; text-decoration: none; border-radius: 6px;">Get Started</a>
</div>`,
        Variables: []string{"name", "product", "cta_url"},
        Locale:    "en",
        Status:    "active",
    },
    {
        Name:        "password_reset",
        Description: "Password reset request with OTP code",
        Channel:     "email",
        Subject:     "Reset Your Password",
        Body: `<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
  <h2>Password Reset</h2>
  <p>Hi {{.name}}, you requested a password reset. Use this code:</p>
  <div style="font-size: 32px; font-weight: bold; text-align: center; padding: 20px; background: #f5f5f5; border-radius: 8px; letter-spacing: 8px;">{{.code}}</div>
  <p style="color: #666; font-size: 14px;">This code expires in {{.expiry_minutes}} minutes.</p>
</div>`,
        Variables: []string{"name", "code", "expiry_minutes"},
        Locale:    "en",
        Status:    "active",
    },
    {
        Name:        "order_confirmation",
        Description: "Order confirmation with order details",
        Channel:     "email",
        Subject:     "Order Confirmed: #{{.order_id}}",
        Body: `<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
  <h2>Order Confirmed</h2>
  <p>Hi {{.name}}, your order <strong>#{{.order_id}}</strong> has been confirmed.</p>
  <p>Total: <strong>{{.total}}</strong></p>
  <p>Estimated delivery: {{.delivery_date}}</p>
</div>`,
        Variables: []string{"name", "order_id", "total", "delivery_date"},
        Locale:    "en",
        Status:    "active",
    },
    {
        Name:        "push_alert",
        Description: "Generic push notification alert",
        Channel:     "push",
        Subject:     "{{.title}}",
        Body:        "{{.message}}",
        Variables:   []string{"title", "message"},
        Locale:      "en",
        Status:      "active",
    },
    {
        Name:        "sms_verification",
        Description: "SMS verification code",
        Channel:     "sms",
        Body:        "Your verification code is {{.code}}. Expires in {{.expiry}} minutes.",
        Variables:   []string{"code", "expiry"},
        Locale:      "en",
        Status:      "active",
    },
    {
        Name:        "webhook_event",
        Description: "Generic webhook event notification",
        Channel:     "webhook",
        Subject:     "{{.event_type}}",
        Body:        `{"event": "{{.event_type}}", "message": "{{.message}}", "timestamp": "{{.timestamp}}"}`,
        Variables:   []string{"event_type", "message", "timestamp"},
        Locale:      "en",
        Status:      "active",
    },
}
```

#### API Endpoint

Add `GET /v1/templates/library` (public within protected routes):

```go
// Handler
func (h *TemplateHandler) GetLibrary(c *fiber.Ctx) error {
    return c.JSON(fiber.Map{
        "templates": seed.LibraryTemplates,
    })
}

// Route (in setupProtectedRoutes, BEFORE /:id routes to avoid conflict)
templates.Get("/library", c.TemplateHandler.GetLibrary)
```

#### Clone Endpoint

Add `POST /v1/templates/library/:name/clone`:

```go
func (h *TemplateHandler) CloneFromLibrary(c *fiber.Ctx) error {
    appID := c.Locals("app_id").(string)
    name := c.Params("name")

    // Find in library
    var source *template.Template
    for _, t := range seed.LibraryTemplates {
        if t.Name == name {
            source = &t
            break
        }
    }
    if source == nil {
        return c.Status(404).JSON(fiber.Map{"error": "library template not found"})
    }

    // Clone into user's app
    createReq := &template.CreateRequest{
        AppID:     appID,
        Name:      source.Name,
        Description: source.Description,
        Channel:   source.Channel,
        Subject:   source.Subject,
        Body:      source.Body,
        Variables: source.Variables,
        Locale:    source.Locale,
        CreatedBy: "library:clone",
    }

    tmpl, err := h.service.Create(c.Context(), createReq)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": err.Error()})
    }

    return c.Status(201).JSON(toTemplateResponse(tmpl))
}
```

#### UI: Library Browser

In `AppTemplates.tsx`, add a "Browse Library" button that shows a dialog:

```tsx
function TemplateLibrary({ apiKey, onClone }: { apiKey: string, onClone: () => void }) {
  const [library, setLibrary] = useState<Template[]>([]);

  useEffect(() => {
    templatesAPI.getLibrary(apiKey).then(res => setLibrary(res.data.templates));
  }, []);

  const handleClone = async (name: string) => {
    await templatesAPI.cloneFromLibrary(apiKey, name);
    toast.success('Template cloned!');
    onClone();
  };

  return (
    <Dialog>
      <DialogContent>
        <DialogHeader><DialogTitle>Template Library</DialogTitle></DialogHeader>
        <div className="space-y-3">
          {library.map(t => (
            <Card key={t.name}>
              <CardContent className="flex justify-between items-center p-4">
                <div>
                  <p className="font-medium">{t.name}</p>
                  <p className="text-sm text-muted-foreground">{t.description} · {t.channel}</p>
                </div>
                <Button size="sm" onClick={() => handleClone(t.name)}>Clone</Button>
              </CardContent>
            </Card>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  );
}
```

---

### 3.4 Notification Activity Feed

**Problem**: No real-time visibility into notification delivery status. Users must manually refresh.

**Approach**: Use the existing SSE infrastructure to push status updates to the dashboard.

#### Backend: Publish Status Changes to Redis Pub/Sub

In `NotificationService.UpdateStatus` (or at the repository level), publish an event after status changes:

```go
// In notification_service.go, after successful status update:
func (s *NotificationService) publishStatusEvent(ctx context.Context, notificationID string, newStatus notification.Status) {
    event := map[string]interface{}{
        "notification_id": notificationID,
        "status":          string(newStatus),
        "timestamp":       time.Now().Format(time.RFC3339),
    }
    data, _ := json.Marshal(event)
    s.redisClient.Publish(ctx, "notification:status_updates", string(data))
}
```

Also publish from the worker after `UpdateStatus`:

```go
// In processor.go, after notifRepo.UpdateStatus:
p.publishStatusUpdate(ctx, notif.NotificationID, notification.StatusSent)
```

#### Backend: SSE Stream for Admin Dashboard

Add a new SSE endpoint for admin activity feed (JWT-protected):

```go
// Route
adminAuth.Get("/activity-feed", c.SSEHandler.AdminActivityFeed)

// Handler: subscribes to "notification:status_updates" Redis channel
// and streams events to the browser as SSE
func (h *SSEHandler) AdminActivityFeed(c *fiber.Ctx) error {
    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("Connection", "keep-alive")
    // Subscribe to Redis and flush events to response
}
```

#### UI: Activity Feed Component

```tsx
// ui/src/components/ActivityFeed.tsx
export function ActivityFeed() {
  const [events, setEvents] = useState<StatusEvent[]>([]);

  useEffect(() => {
    const es = new EventSource('/v1/admin/activity-feed');
    es.onmessage = (e) => {
      const event = JSON.parse(e.data);
      setEvents(prev => [event, ...prev].slice(0, 50)); // Keep last 50
    };
    return () => es.close();
  }, []);

  return (
    <div className="space-y-2">
      {events.map(e => (
        <div key={e.notification_id + e.timestamp} className="flex items-center gap-2 text-sm">
          <StatusBadge status={e.status} />
          <span className="text-muted-foreground">{formatTime(e.timestamp)}</span>
          <span>{e.notification_id.slice(0, 8)}...</span>
        </div>
      ))}
    </div>
  );
}
```

---

### 3.5 Provider Health Dashboard

**Problem**: No visibility into whether SMTP, APNS, FCM, etc. are configured and healthy.

#### Backend: Health Endpoint

Add `GET /v1/admin/providers/health`:

```go
// In provider Manager, add:
func (m *Manager) HealthStatus() map[string]ProviderHealth {
    m.mu.RLock()
    defer m.mu.RUnlock()

    result := make(map[string]ProviderHealth)
    for channel, provider := range m.providers {
        healthy := provider.IsHealthy(context.Background())
        breakerName := provider.GetName() + "-" + string(channel)
        breakerState := "closed"
        if b, ok := m.breakers[breakerName]; ok {
            breakerState = b.State()
        }
        result[string(channel)] = ProviderHealth{
            Name:         provider.GetName(),
            Channel:      string(channel),
            Healthy:      healthy,
            BreakerState: breakerState,
        }
    }
    return result
}

type ProviderHealth struct {
    Name         string `json:"name"`
    Channel      string `json:"channel"`
    Healthy      bool   `json:"healthy"`
    BreakerState string `json:"breaker_state"` // closed, open, half-open
}
```

Add handler in `AdminHandler`:

```go
func (h *AdminHandler) GetProviderHealth(c *fiber.Ctx) error {
    health := h.providerManager.HealthStatus()
    return c.JSON(fiber.Map{"providers": health})
}
```

Route:
```go
admin.Get("/providers/health", c.AdminHandler.GetProviderHealth)
```

#### Container Wiring

Pass `providerManager` to `AdminHandler`. Currently `AdminHandler` only has queue access — add:

```go
type AdminHandler struct {
    queue           queue.Queue
    providerManager *providers.Manager  // NEW
    logger          *zap.Logger
}
```

#### UI Component

```tsx
// In Dashboard.tsx or a new ProviderHealth.tsx
function ProviderHealth() {
  const [health, setHealth] = useState<Record<string, ProviderStatus>>({});

  useEffect(() => {
    adminAPI.getProviderHealth().then(res => setHealth(res.data.providers));
    const interval = setInterval(() => {
      adminAPI.getProviderHealth().then(res => setHealth(res.data.providers));
    }, 30000); // Poll every 30s
    return () => clearInterval(interval);
  }, []);

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Provider</TableHead>
          <TableHead>Channel</TableHead>
          <TableHead>Status</TableHead>
          <TableHead>Circuit Breaker</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {Object.values(health).map(p => (
          <TableRow key={p.channel}>
            <TableCell>{p.name}</TableCell>
            <TableCell><Badge variant="outline">{p.channel}</Badge></TableCell>
            <TableCell>{p.healthy ? '✅ Healthy' : '❌ Down'}</TableCell>
            <TableCell><Badge variant={p.breaker_state === 'closed' ? 'default' : 'destructive'}>{p.breaker_state}</Badge></TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
```

---

### 3.6 Dead Letter Queue Viewer

**Problem**: Failed notifications silently enter the DLQ. The admin API exists (`GET /v1/admin/queues/dlq`, `POST /v1/admin/queues/dlq/replay`) but has no UI.

**Files to Create**:
- `ui/src/components/DLQViewer.tsx`

**Files to Modify**:
- `ui/src/pages/Dashboard.tsx` — add DLQ tab
- `ui/src/services/api.ts` — `adminAPI` already has `listDLQ()

#### UI Component

```tsx
function DLQViewer() {
  const [items, setItems] = useState<DLQItem[]>([]);
  const [replaying, setReplaying] = useState(false);

  const fetchDLQ = async () => {
    const res = await adminAPI.listDLQ();
    setItems(res.data.items || []);
  };

  const handleReplay = async () => {
    setReplaying(true);
    const res = await adminAPI.replayDLQ();
    toast.success(`Replayed ${res.data.replayed} items`);
    await fetchDLQ();
    setReplaying(false);
  };

  useEffect(() => { fetchDLQ(); }, []);

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h3>{items.length} Failed Notifications</h3>
        <Button onClick={handleReplay} disabled={replaying || items.length === 0}>
          Replay All
        </Button>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Notification ID</TableHead>
            <TableHead>Priority</TableHead>
            <TableHead>Reason</TableHead>
            <TableHead>Failed At</TableHead>
            <TableHead>Retries</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map(item => (
            <TableRow key={item.notification_id}>
              <TableCell className="font-mono text-xs">{item.notification_id.slice(0, 12)}...</TableCell>
              <TableCell><Badge>{item.priority}</Badge></TableCell>
              <TableCell className="text-red-600 text-sm">{item.reason}</TableCell>
              <TableCell>{formatDate(item.timestamp)}</TableCell>
              <TableCell>{item.retry_count}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
```

---

### 3.7 Multi-Channel Template Groups

**Problem**: Templates are channel-locked. A "welcome" message needs separate templates for email, push, SMS. Sending on multiple channels requires multiple API calls with different template IDs.

#### New Domain Model

Create `internal/domain/template/group.go`:

```go
package template

// TemplateGroup represents a named collection of channel-specific template variants.
type TemplateGroup struct {
    Name        string                       `json:"name"`
    AppID       string                       `json:"app_id"`
    Description string                       `json:"description"`
    Variants    map[string]TemplateVariantRef `json:"variants"` // channel → template ID
    CreatedAt   time.Time                    `json:"created_at"`
    UpdatedAt   time.Time                    `json:"updated_at"`
}

// TemplateVariantRef links a channel to a specific template.
type TemplateVariantRef struct {
    TemplateID string `json:"template_id"`
    Channel    string `json:"channel"`
}
```

#### Storage

Store in ES index `"template_groups"` as a flat document:

```json
{
    "name": "welcome",
    "app_id": "uuid",
    "description": "Welcome message across all channels",
    "variants": {
        "email": { "template_id": "uuid-1", "channel": "email" },
        "push":  { "template_id": "uuid-2", "channel": "push" },
        "sms":   { "template_id": "uuid-3", "channel": "sms" }
    }
}
```

#### API

```
POST   /v1/template-groups          — Create a group with variant template IDs
GET    /v1/template-groups           — List groups
GET    /v1/template-groups/:name     — Get group with variant details
PUT    /v1/template-groups/:name     — Update group variants
DELETE /v1/template-groups/:name     — Delete group
```

#### Send Integration

When a notification references a template group name:
1. `resolveTemplate` checks the `template_groups` index if not found in `templates`
2. If found, selects the variant matching the notification's channel
3. Falls back to `email` variant if no channel-specific variant exists

#### Multi-Channel Send

New DTO for "send on all channels":

```go
type MultiChannelSendRequest struct {
    To            string                 `json:"to" validate:"required"`
    TemplateGroup string                 `json:"template_group" validate:"required"`
    Data          map[string]interface{} `json:"data,omitempty"`
    Priority      string                 `json:"priority,omitempty"`
    Channels      []string               `json:"channels,omitempty"` // Subset; empty = all variants
}
```

The handler iterates over the group's variants and fires a notification per channel.

---

### 3.8 Client SDKs

#### JavaScript/TypeScript SDK

Create `sdk/js/` or publish as `@freerangenotify/sdk`:

```typescript
// sdk/js/src/index.ts

export class FreeRangeNotify {
  private apiKey: string;
  private baseURL: string;

  constructor(apiKey: string, options?: { baseURL?: string }) {
    this.apiKey = apiKey;
    this.baseURL = options?.baseURL || 'https://api.freerangenotify.com/v1';
  }

  async send(params: {
    to: string;
    template?: string;
    subject?: string;
    body?: string;
    data?: Record<string, any>;
    channel?: string;
    priority?: 'low' | 'normal' | 'high' | 'critical';
    scheduledAt?: Date;
  }) {
    const res = await fetch(`${this.baseURL}/quick-send`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${this.apiKey}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        to: params.to,
        template: params.template,
        subject: params.subject,
        body: params.body,
        data: params.data,
        channel: params.channel,
        priority: params.priority,
        scheduled_at: params.scheduledAt?.toISOString(),
      }),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async broadcast(params: {
    template: string;
    data?: Record<string, any>;
    channel?: string;
    priority?: string;
  }) {
    const res = await fetch(`${this.baseURL}/notifications/broadcast`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${this.apiKey}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(params),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async createUser(params: {
    email?: string;
    phone?: string;
    timezone?: string;
    language?: string;
  }) {
    const res = await fetch(`${this.baseURL}/users`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${this.apiKey}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(params),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }
}
```

#### Go SDK

Create `sdk/go/`:

```go
// sdk/go/freerangenotify/client.go

package freerangenotify

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
)

type Client struct {
    apiKey  string
    baseURL string
    http    *http.Client
}

func New(apiKey string, opts ...Option) *Client {
    c := &Client{
        apiKey:  apiKey,
        baseURL: "http://localhost:8080/v1",
        http:    http.DefaultClient,
    }
    for _, opt := range opts {
        opt(c)
    }
    return c
}

type Option func(*Client)

func WithBaseURL(url string) Option {
    return func(c *Client) { c.baseURL = url }
}

type SendParams struct {
    To       string                 `json:"to"`
    Template string                 `json:"template,omitempty"`
    Subject  string                 `json:"subject,omitempty"`
    Body     string                 `json:"body,omitempty"`
    Data     map[string]interface{} `json:"data,omitempty"`
    Channel  string                 `json:"channel,omitempty"`
    Priority string                 `json:"priority,omitempty"`
}

type SendResult struct {
    NotificationID string `json:"notification_id"`
    Status         string `json:"status"`
    UserID         string `json:"user_id"`
    Channel        string `json:"channel"`
}

func (c *Client) Send(ctx context.Context, params SendParams) (*SendResult, error) {
    body, _ := json.Marshal(params)
    req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/quick-send", bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+c.apiKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.http.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return nil, fmt.Errorf("API error: %d", resp.StatusCode)
    }

    var result SendResult
    json.NewDecoder(resp.Body).Decode(&result)
    return &result, nil
}
```

---

## Phase 4 — Competitive Edge

### 4.1 Webhook Playground

**Goal**: In-dashboard temporary webhook URL to test delivery without external tools.

#### Backend

Add `POST /v1/admin/playground/webhook` — generates a temporary webhook receiver:

```go
// Generate a unique playground ID
playgroundID := uuid.New().String()[:8]
url := fmt.Sprintf("http://localhost:8080/v1/playground/%s", playgroundID)

// Store in Redis with 30-minute TTL
redisClient.Set(ctx, "playground:"+playgroundID, "[]", 30*time.Minute)

// Return the URL + playground ID
```

Add `GET /v1/playground/:id` — returns received payloads (stored in Redis list).

Add `POST /v1/playground/:id` — receives webhook (public, no auth). Appends payload to Redis list.

#### UI

```tsx
function WebhookPlayground({ apiKey }: { apiKey: string }) {
  const [playgroundURL, setPlaygroundURL] = useState('');
  const [payloads, setPayloads] = useState<any[]>([]);

  const createPlayground = async () => {
    const res = await adminAPI.createPlayground();
    setPlaygroundURL(res.data.url);
    // Start polling for received payloads
    pollPayloads(res.data.id);
  };

  return (
    <div>
      <Button onClick={createPlayground}>Create Test Webhook</Button>
      {playgroundURL && (
        <div>
          <Label>Your test webhook URL (expires in 30 min):</Label>
          <Input value={playgroundURL} readOnly />
          <CopyButton text={playgroundURL} />
          <h4>Received Payloads:</h4>
          {payloads.map((p, i) => (
            <pre key={i} className="bg-muted p-3 rounded text-xs overflow-auto">
              {JSON.stringify(p, null, 2)}
            </pre>
          ))}
        </div>
      )}
    </div>
  );
}
```

---

### 4.2 Notification Search & Filtering

**Problem**: `GET /v1/notifications` supports filters, but the UI doesn't expose them.

**Files to Modify**:
- `ui/src/components/AppNotifications.tsx`

**Add Filter Bar** above the notification history table:

```tsx
function NotificationFilters({ onFilter }: { onFilter: (filters: FilterParams) => void }) {
  const [status, setStatus] = useState('');
  const [channel, setChannel] = useState('');
  const [dateRange, setDateRange] = useState({ from: '', to: '' });

  return (
    <div className="flex gap-3 items-end mb-4">
      <div>
        <Label>Status</Label>
        <Select value={status} onValueChange={v => { setStatus(v); onFilter({ status: v, channel, ...dateRange }); }}>
          <SelectItem value="">All</SelectItem>
          <SelectItem value="pending">Pending</SelectItem>
          <SelectItem value="queued">Queued</SelectItem>
          <SelectItem value="sent">Sent</SelectItem>
          <SelectItem value="delivered">Delivered</SelectItem>
          <SelectItem value="failed">Failed</SelectItem>
        </Select>
      </div>
      <div>
        <Label>Channel</Label>
        <Select value={channel} onValueChange={v => { setChannel(v); onFilter({ status, channel: v, ...dateRange }); }}>
          <SelectItem value="">All</SelectItem>
          <SelectItem value="email">Email</SelectItem>
          <SelectItem value="push">Push</SelectItem>
          <SelectItem value="sms">SMS</SelectItem>
          <SelectItem value="webhook">Webhook</SelectItem>
          <SelectItem value="sse">SSE</SelectItem>
        </Select>
      </div>
      <div>
        <Label>From</Label>
        <Input type="date" value={dateRange.from} onChange={e => setDateRange(p => ({ ...p, from: e.target.value }))} />
      </div>
      <div>
        <Label>To</Label>
        <Input type="date" value={dateRange.to} onChange={e => setDateRange(p => ({ ...p, to: e.target.value }))} />
      </div>
    </div>
  );
}
```

**Wire filters to API call**:

```tsx
const fetchNotifications = async (filters: FilterParams) => {
  const params = new URLSearchParams();
  if (filters.status) params.set('status', filters.status);
  if (filters.channel) params.set('channel', filters.channel);
  if (filters.from) params.set('from_date', new Date(filters.from).toISOString());
  if (filters.to) params.set('to_date', new Date(filters.to).toISOString());
  params.set('page', String(page));
  params.set('page_size', '20');

  const res = await api.get(`/notifications?${params}`, { headers: getAuthHeaders(apiKey) });
  setNotifications(res.data.notifications);
  setTotal(res.data.total);
};
```

---

### 4.3 React Notification Bell Component

**Goal**: A drop-in React component for in-app notifications using SSE.

Create `sdk/react/`:

```tsx
// sdk/react/src/NotificationBell.tsx

import React, { useState, useEffect } from 'react';

interface NotificationBellProps {
  userId: string;         // Internal UUID
  apiBaseURL?: string;    // Default: window.location.origin
  onNotification?: (notification: any) => void;
}

export function NotificationBell({ userId, apiBaseURL, onNotification }: NotificationBellProps) {
  const [unreadCount, setUnreadCount] = useState(0);
  const [notifications, setNotifications] = useState<any[]>([]);
  const [open, setOpen] = useState(false);

  const baseURL = apiBaseURL || window.location.origin;

  // Connect to SSE
  useEffect(() => {
    const es = new EventSource(`${baseURL}/v1/sse?user_id=${userId}`);

    es.addEventListener('notification', (e) => {
      const notif = JSON.parse(e.data);
      setNotifications(prev => [notif, ...prev]);
      setUnreadCount(prev => prev + 1);
      onNotification?.(notif);
    });

    return () => es.close();
  }, [userId, baseURL]);

  return (
    <div style={{ position: 'relative', display: 'inline-block' }}>
      <button onClick={() => setOpen(!open)} style={{ position: 'relative' }}>
        🔔
        {unreadCount > 0 && (
          <span style={{
            position: 'absolute', top: -4, right: -4,
            background: 'red', color: 'white', borderRadius: '50%',
            width: 18, height: 18, fontSize: 11, display: 'flex',
            alignItems: 'center', justifyContent: 'center'
          }}>
            {unreadCount > 9 ? '9+' : unreadCount}
          </span>
        )}
      </button>
      {open && (
        <div style={{
          position: 'absolute', right: 0, top: '100%', width: 320,
          maxHeight: 400, overflowY: 'auto', background: 'white',
          border: '1px solid #ddd', borderRadius: 8, boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
          zIndex: 1000
        }}>
          {notifications.length === 0 ? (
            <p style={{ padding: 16, textAlign: 'center', color: '#999' }}>No notifications</p>
          ) : (
            notifications.map((n, i) => (
              <div key={i} style={{ padding: '12px 16px', borderBottom: '1px solid #eee' }}>
                <strong>{n.title}</strong>
                <p style={{ margin: '4px 0 0', fontSize: 13, color: '#666' }}>{n.body}</p>
              </div>
            ))
          )}
        </div>
      {...}
    </div>
  );
}
```

---

### 4.4 Template Versioning UI

**Problem**: Backend supports `CreateVersion` and `GetVersions` but the UI doesn't expose them.

**Files to Modify**:
- `ui/src/components/AppTemplates.tsx`

**Add to each template row**:

```tsx
<Button variant="ghost" size="sm" onClick={() => showVersions(template)}>
  v{template.version} · History
</Button>
```

**Version History Dialog**:

```tsx
function VersionHistory({ template, apiKey, onRestore }: VersionHistoryProps) {
  const [versions, setVersions] = useState<Template[]>([]);

  useEffect(() => {
    templatesAPI.getVersions(apiKey, template.app_id, template.name)
      .then(res => setVersions(res.data.templates));
  }, []);

  return (
    <Dialog>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Version History: {template.name}</DialogTitle>
        </DialogHeader>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Version</TableHead>
              <TableHead>Updated By</TableHead>
              <TableHead>Updated At</TableHead>
              <TableHead>Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {versions.map(v => (
              <TableRow key={v.id}>
                <TableCell>v{v.version}</TableCell>
                <TableCell>{v.updated_by || '—'}</TableCell>
                <TableCell>{formatDate(v.updated_at)}</TableCell>
                <TableCell>
                  <Button variant="ghost" size="sm" onClick={() => previewVersion(v)}>Preview</Button>
                  <Button variant="ghost" size="sm" onClick={() => onRestore(v)}>Restore</Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </DialogContent>
    </Dialog>
  );
}
```

**Create Version Button**: Add a "Save as New Version" button in the edit form that calls:

```tsx
await templatesAPI.createVersion(apiKey, template.app_id, template.name, {
  body: formData.body,
  subject: formData.subject,
  description: formData.description,
  variables: formData.variables,
  created_by: 'dashboard',
});
```

---

### 4.5 Analytics Dashboard

**Problem**: `internal/domain/analytics/models.go` exists but no data is collected or displayed.

#### Data Collection

In the worker `processNotification`, after successful delivery, index an analytics event:

```go
type DeliveryEvent struct {
    NotificationID string  `json:"notification_id"`
    AppID          string  `json:"app_id"`
    Channel        string  `json:"channel"`
    Provider       string  `json:"provider"`
    Status         string  `json:"status"`
    Latency        float64 `json:"latency_ms"`
    Timestamp      string  `json:"timestamp"`
}

// Index to "analytics_events" ES index
```

#### Aggregation API

Add `GET /v1/admin/analytics/summary?period=7d`:

```go
// Elasticsearch aggregation query:
// - Group by channel → count by status
// - Average latency per channel
// - Delivery success rate per day
// - Top failing templates

type AnalyticsSummary struct {
    Period         string              `json:"period"`
    TotalSent      int64               `json:"total_sent"`
    TotalDelivered int64               `json:"total_delivered"`
    TotalFailed    int64               `json:"total_failed"`
    SuccessRate    float64             `json:"success_rate"`
    AvgLatency     float64             `json:"avg_latency_ms"`
    ByChannel      []ChannelAnalytics  `json:"by_channel"`
    DailyBreakdown []DailyAnalytics    `json:"daily_breakdown"`
}
```

#### UI

Add "Analytics" tab to Dashboard:

```tsx
function AnalyticsDashboard() {
  // Fetch summary → render:
  // - Stat cards: Total Sent, Delivered, Failed, Success Rate
  // - Bar chart: Notifications per channel (use recharts or similar)
  // - Line chart: Daily sends over time
  // - Table: Top failing templates with failure reasons
}
```

---

### 4.6 Provider Fallback Chains

**Problem**: If the primary provider for a channel fails, the notification fails. No automatic fallback to a secondary provider.

#### Configuration

Add to `application.Settings`:

```go
type ProviderFallback struct {
    Channel  string   `json:"channel"`
    Providers []string `json:"providers"` // Ordered list: ["sendgrid", "smtp"]
}
```

#### Manager Changes

In `providers.Manager.Send`, implement ordered retry:

```go
func (m *Manager) SendWithFallback(ctx context.Context, notif *notification.Notification, usr *user.User, fallbacks []string) (*Result, error) {
    var lastErr error
    for _, providerName := range fallbacks {
        key := providerName + "-" + string(notif.Channel)
        provider, ok := m.namedProviders[key]
        if !ok {
            continue
        }

        breaker := m.breakers[key]
        result, err := breaker.Execute(func() (*Result, error) {
            return provider.Send(ctx, notif, usr)
        })

        if err == nil && result.Success {
            m.logger.Info("Delivery succeeded via fallback",
                zap.String("provider", providerName))
            return result, nil
        }
        lastErr = err
        m.logger.Warn("Provider failed, trying next",
            zap.String("provider", providerName),
            zap.Error(err))
    }
    return nil, fmt.Errorf("all providers failed: %w", lastErr)
}
```

#### Worker Integration

In `processor.sendNotification`, check app settings for fallback config:

```go
app, _ := p.appRepo.GetByID(ctx, notif.AppID)
if app != nil && len(app.Settings.ProviderFallbacks) > 0 {
    for _, fb := range app.Settings.ProviderFallbacks {
        if fb.Channel == string(notif.Channel) {
            return p.providerManager.SendWithFallback(ctx, notif, usr, fb.Providers)
        }
    }
}
// Default: single provider
return p.providerManager.Send(ctx, notif, usr)
```

---

## Cross-Cutting Concerns

### Duplicate Code: `isQuietHours`

**Problem**: `isQuietHours` exists in both `notification_service.go` and `processor.go` with identical logic.

**Fix**: Extract to a shared package.

Create `pkg/utils/quiet_hours.go`:

```go
package utils

import (
    "time"
    "github.com/the-monkeys/freerangenotify/internal/domain/user"
)

// IsQuietHours checks if the current time falls within a user's quiet hours.
func IsQuietHours(u *user.User) bool {
    if u.Preferences.QuietHours.Start == "" || u.Preferences.QuietHours.End == "" {
        return false
    }

    loc := time.UTC
    if u.Timezone != "" {
        if l, err := time.LoadLocation(u.Timezone); err == nil {
            loc = l
        }
    }

    now := time.Now().In(loc)
    startStr := u.Preferences.QuietHours.Start // "HH:MM"
    endStr := u.Preferences.QuietHours.End     // "HH:MM"

    start, err1 := time.Parse("15:04", startStr)
    end, err2 := time.Parse("15:04", endStr)
    if err1 != nil || err2 != nil {
        return false
    }

    currentMinutes := now.Hour()*60 + now.Minute()
    startMinutes := start.Hour()*60 + start.Minute()
    endMinutes := end.Hour()*60 + end.Minute()

    if startMinutes <= endMinutes {
        return currentMinutes >= startMinutes && currentMinutes < endMinutes
    }
    // Overnight quiet hours (e.g., 22:00 - 07:00)
    return currentMinutes >= startMinutes || currentMinutes < endMinutes
}
```

Then in both `notification_service.go` and `processor.go`, replace `s.isQuietHours(u)` / `p.isQuietHours(usr, notif)` with `utils.IsQuietHours(u)`.

### Application Caching in NotificationService

**Problem**: `isChannelEnabled` fetches the application from ES on every call. In a broadcast to 10K users, this is 10K identical ES queries.

**Fix**: Add a simple in-memory cache with short TTL.

```go
// In NotificationService, add:
type appCache struct {
    mu    sync.RWMutex
    items map[string]*appCacheEntry
}

type appCacheEntry struct {
    app       *application.Application
    expiresAt time.Time
}

func (s *NotificationService) getCachedApp(ctx context.Context, appID string) (*application.Application, error) {
    s.appCacheMu.RLock()
    if entry, ok := s.appCache[appID]; ok && time.Now().Before(entry.expiresAt) {
        s.appCacheMu.RUnlock()
        return entry.app, nil
    }
    s.appCacheMu.RUnlock()

    app, err := s.appRepo.GetByID(ctx, appID)
    if err != nil {
        return nil, err
    }

    s.appCacheMu.Lock()
    s.appCache[appID] = &appCacheEntry{app: app, expiresAt: time.Now().Add(30 * time.Second)}
    s.appCacheMu.Unlock()

    return app, nil
}
```

Use `getCachedApp` in `isChannelEnabled` instead of `s.appRepo.GetByID`.

### Security: MarkRead Ownership Check

**Problem**: `MarkRead` skips ownership verification — any user can mark another user's notifications as read.

**Fix**: In `NotificationService.MarkRead`, verify each notification belongs to the caller:

```go
func (s *NotificationService) MarkRead(ctx context.Context, notificationIDs []string, appID, userID string) error {
    for _, id := range notificationIDs {
        notif, err := s.notificationRepo.GetByID(ctx, id)
        if err != nil {
            continue
        }
        if notif.AppID != appID || notif.UserID != userID {
            s.logger.Warn("MarkRead ownership check failed",
                zap.String("notification_id", id),
                zap.String("claimed_user", userID),
                zap.String("actual_user", notif.UserID))
            return fmt.Errorf("notification %s does not belong to user %s", id, userID)
        }
    }
    return s.notificationRepo.BulkUpdateStatus(ctx, notificationIDs, notification.StatusRead)
}
```

---

## File Change Summary

| Phase | New Files | Modified Files |
|-------|-----------|----------------|
| **1.1** Template Editing | — | `ui/src/components/AppTemplates.tsx` |
| **1.2** Default Locale | — | `internal/interfaces/http/dto/template_dto.go`, `internal/usecases/template_service.go` |
| **1.3** Auto-Detect Vars | — | `internal/usecases/template_service.go`, `ui/src/components/AppTemplates.tsx` |
| **1.4** Bulk Send Fix | — | `ui/src/components/AppNotifications.tsx` |
| **1.5** Phone Field | — | `ui/src/components/AppUsers.tsx` |
| **1.6** Debug Cleanup | — | `internal/interfaces/http/handlers/notification_handler.go` |
| **1.7** Pagination | `ui/src/components/Pagination.tsx` | `ui/src/components/App{Users,Templates,Notifications}.tsx`, `ui/src/services/api.ts` |
| **2.1** Quick-Send | `internal/interfaces/http/dto/quick_send_dto.go`, `internal/interfaces/http/handlers/quick_send_handler.go`, `internal/usecases/quick_send_service.go` | `internal/container/container.go`, `internal/interfaces/http/routes/routes.go` |
| **2.2** Send by Email | — | `internal/usecases/notification_service.go` |
| **2.3** Template by Name | — | `internal/usecases/notification_service.go` |
| **2.4** Infer Channel | — | `internal/interfaces/http/dto/notification_dto.go`, `internal/domain/notification/models.go`, `internal/usecases/notification_service.go` |
| **2.5** Simplified UI | — | `ui/src/components/AppNotifications.tsx` |
| **2.6** Dropdowns | — | `ui/src/components/AppUsers.tsx` |
| **2.7** Auth Docs | `documents/AUTHENTICATION_GUIDE.md` | — |
| **2.8** Bulk Import | — | `internal/interfaces/http/dto/user_dto.go`, `internal/interfaces/http/handlers/user_handler.go`, `internal/interfaces/http/routes/routes.go` |
| **3.1** HTML Editor | `ui/src/components/TemplateEditor.tsx` | `ui/src/components/AppTemplates.tsx`, `ui/package.json`, `internal/usecases/template_service.go`, `cmd/worker/processor.go` |
| **3.2** Setup Wizard | `ui/src/components/SetupWizard.tsx` | `ui/src/pages/AppDetail.tsx` |
| **3.3** Template Library | `internal/seed/templates.go` | `internal/interfaces/http/handlers/template_handler.go`, `internal/interfaces/http/routes/routes.go` |
| **3.4** Activity Feed | `ui/src/components/ActivityFeed.tsx` | `internal/usecases/notification_service.go`, `cmd/worker/processor.go`, `internal/interfaces/http/handlers/sse_handler.go`, routes |
| **3.5** Provider Health | `ui/src/components/ProviderHealth.tsx` | `internal/infrastructure/providers/manager.go`, `internal/interfaces/http/handlers/admin_handler.go`, routes |
| **3.6** DLQ Viewer | `ui/src/components/DLQViewer.tsx` | `ui/src/pages/Dashboard.tsx` |
| **3.7** Multi-Channel | `internal/domain/template/group.go`, handler, repo | `internal/usecases/notification_service.go`, routes |
| **3.8** SDKs | `sdk/js/`, `sdk/go/` | — |
| **4.1** Playground | `ui/src/components/WebhookPlayground.tsx` | routes, new handler |
| **4.2** Search/Filter | — | `ui/src/components/AppNotifications.tsx` |
| **4.3** Bell Component | `sdk/react/` | — |
| **4.4** Versioning UI | — | `ui/src/components/AppTemplates.tsx` |
| **4.5** Analytics | `ui/src/components/AnalyticsDashboard.tsx` | worker processor, new handler, new repo |
| **4.6** Fallback Chains | — | `internal/infrastructure/providers/manager.go`, `internal/domain/application/models.go`, worker processor |
| **Cross-cutting** | `pkg/utils/quiet_hours.go` | `internal/usecases/notification_service.go`, `cmd/worker/processor.go` |
