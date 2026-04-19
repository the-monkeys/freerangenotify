# Case Study: Monkeys Blogging Platform

This guide walks through how **Monkeys** — a community blogging platform where users write, discover, and collaborate on content — integrates FreeRangeNotify for real-time and persistent in-app notifications.

It covers everything end-to-end: creating the application, setting up templates for the events that matter, managing the user lifecycle, sending notifications from the Monkeys backend, and consuming them on the frontend.

> This is a reference implementation. It is redacted of all keys, secrets, and internal identifiers. Use it as a blueprint for your own integration.

---

## Architecture Decision: In-App + SSE + Email

Monkeys uses **up to three channels** per notification event:

| Channel | Role |
|---|---|
| `in_app` | Writes the notification to the persistent inbox in Elasticsearch. The user sees it whether they are online now or return a week later. |
| `sse` | Pushes a real-time signal to any open browser tab. Triggers the frontend to re-fetch the inbox immediately — the user sees the badge count jump without a page refresh. |
| `email` | Sends an email for high-priority security and account events. Used alongside `in_app` to reach users who may not be actively browsing. |

The rule is: **`in_app` stores it, `sse` signals it, `email` escalates it.** Every event gets `in_app`. SSE is added for events the user should see immediately. Email is reserved for security-critical events (login detected, password changed, email changed, account deleted).

---

## What Gets Notified

Monkeys notifications fall into two categories: **social/collaboration events** (triggered by other users' actions) and **security/account events** (triggered by the user's own actions that they should be aware of for security purposes).

### Social & Collaboration Events

| Event | Channels | Category | Priority |
|---|---|---|---|
| Someone followed you | `in_app` + `sse` | `social` | `normal` |
| Someone commented on your blog | `in_app` + `sse` | `social` | `normal` | *(planned — constant defined, consumer not yet wired)* |
| Someone liked your blog | `in_app` + `sse` | `social` | `low` |
| You were invited as a co-author | `in_app` + `sse` + `email` | `collaboration` | `high` |
| Co-author invite accepted | `in_app` + `sse` | `collaboration` | `normal` |
| Co-author invite declined | `in_app` | `collaboration` | `normal` |
| You were removed as co-author | `in_app` + `sse` | `collaboration` | `normal` |
| A co-authored blog was published | `in_app` + `sse` | `content` | `normal` |

### Security & Account Events

| Event | Channels | Category | Priority |
|---|---|---|---|
| Your email was verified | `in_app` | `security` | `high` |
| New login detected | `in_app` + `sse` + `email` | `security` | `high` |
| Your password was changed | `in_app` + `email` | `security` | `high` |
| Password reset requested | `in_app` + `email` | `security` | `high` |
| Your email was changed | `in_app` + `email` | `security` | `normal` |
| Your username was changed | `in_app` | `security` | `normal` |
| Your account was deleted | `in_app` + `email` | `account` | `high` |
| Your account was deactivated | `in_app` | `account` | `normal` |
| Your account was reactivated | `in_app` | `account` | `normal` |

---

## Step 1: Register the Monkeys Application

This is a one-time setup done by the Monkeys engineering team from the FreeRangeNotify dashboard or via the API using a platform admin token.

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/apps/ \
  -H "Authorization: Bearer PLATFORM_ADMIN_JWT" \
  -H "Content-Type: application/json" \
  -d '{
    "app_name": "monkeys",
    "description": "Community blogging platform"
  }'
```

**Response (save the API key securely in your backend secrets manager):**

```json
{
  "app_id": "app-monkeys-xxx",
  "app_name": "monkeys",
  "api_key": "frn_live_xxxxxxxxxxxxx"
}
```

The `api_key` is used for all subsequent API calls. It **never** goes to the frontend.

---

## Step 2: Register Users and Manage the User Lifecycle

### User Registration

When a user joins Monkeys (via email/password or Google OAuth), the backend registers them in FreeRangeNotify immediately after account creation. The Monkeys username is passed as the `external_id` — this means you can use the Monkeys username everywhere in the FRN API without tracking internal UUIDs.

```bash
# Called from the Monkeys sign-up service after the user record is created
curl -X POST https://freerangenotify.monkeys.support/v1/users/ \
  -H "X-API-Key: FRN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "full_name": "Alice Smith",
    "external_id": "alice_monkeys"
  }'
```

The equivalent Go SDK call:

```go
client.SDK.Users.Create(ctx, frn.CreateUserParams{
    FullName:   "Alice Smith",
    Email:      "user@example.com",
    ExternalID: "alice_monkeys",
})
```

> FreeRangeNotify resolves the Monkeys username (`alice_monkeys`) automatically in every subsequent API call — for sending notifications, SSE connections, and subscriber hash generation. No internal UUID needs to be stored or forwarded.

### Bulk Sync on Startup

On service startup, Monkeys syncs all existing active users to FRN using the bulk create endpoint. This ensures users who existed before the FRN integration are registered. The `skip_existing` flag silently skips users already in FRN.

```go
client.SDK.Users.BulkCreate(ctx, frn.BulkCreateUsersParams{
    Users:        createParams, // []frn.CreateUserParams from the DB
    SkipExisting: true,
})
```

### Auto-Register on "User Not Found"

If a notification send fails because the user doesn't exist in FRN (404), the notification service auto-registers the user and retries the send once. This handles edge cases where a user slips through the bulk sync.

### User Lifecycle Operations

The Monkeys notification service handles the full user lifecycle via RabbitMQ events:

| Event | FRN Operation | SDK Method |
|---|---|---|
| `user_register` | Create user | `Users.Create()` |
| `email_changed` | Update email | `Users.Update(identifier, {Email})` |
| `username_changed` | Update external_id | `Users.Update(oldUsername, {ExternalID: newUsername})` |
| `user_account_delete` | Delete user | `Users.Delete(identifier)` |
| `user_deactivated` | Enable DND | `Users.UpdatePreferences(identifier, {DND: true})` |
| `user_reactivated` | Disable DND | `Users.UpdatePreferences(identifier, {DND: false})` |
| `preferences_changed` | Sync prefs | `Users.UpdatePreferences(identifier, {EmailEnabled, PushEnabled})` |

All SDK methods accept the Monkeys username (external_id) as the `identifier` — no need to look up FRN internal UUIDs.

---

## Step 3: Create Notification Templates

Templates are created once and reused for every notification of that type. Each event has two templates: one for `in_app` (persistent inbox) and one for `sse` (real-time signal). The SSE template carries the same content — the frontend uses it to update the bell badge and optionally show a toast.

### Social: New Follower

**In-App template:**

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/templates/ \
  -H "X-API-Key: FRN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "app_id": "app-monkeys-xxx",
    "name": "new_follower_inapp",
    "channel": "in_app",
    "subject": "{{.follower_name}} started following you",
    "body": "You have a new follower. Visit their profile to follow back.",
    "variables": ["follower_name"],
    "locale": "en",
    "metadata": { "category": "social" }
  }'
```

**SSE template (same content, different channel):**

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/templates/ \
  -H "X-API-Key: FRN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "app_id": "app-monkeys-xxx",
    "name": "new_follower_sse",
    "channel": "sse",
    "subject": "{{.follower_name}} started following you",
    "body": "You have a new follower.",
    "variables": ["follower_name"],
    "locale": "en",
    "metadata": { "category": "social" }
  }'
```

### Social: New Comment

```bash
# in_app template
curl -X POST https://freerangenotify.monkeys.support/v1/templates/ \
  -H "X-API-Key: FRN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "app_id": "app-monkeys-xxx",
    "name": "new_comment_inapp",
    "channel": "in_app",
    "subject": "{{.commenter_name}} commented on \"{{.blog_title}}\"",
    "body": "{{.commenter_name}} wrote: {{.comment_preview}}",
    "variables": ["commenter_name", "blog_title", "comment_preview"],
    "locale": "en",
    "metadata": { "category": "social" }
  }'

# sse template — same structure
```

### Collaboration: Co-Author Invite

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/templates/ \
  -H "X-API-Key: FRN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "app_id": "app-monkeys-xxx",
    "name": "coauthor_invite_inapp",
    "channel": "in_app",
    "subject": "{{.inviter_name}} invited you to co-author a blog",
    "body": "You have been invited to collaborate on \"{{.blog_title}}\". Open the blog to accept or decline.",
    "variables": ["inviter_name", "blog_title"],
    "locale": "en",
    "metadata": { "category": "collaboration" }
  }'
```

### Security: Password Changed

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/templates/ \
  -H "X-API-Key: FRN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "app_id": "app-monkeys-xxx",
    "name": "password_changed_inapp",
    "channel": "in_app",
    "subject": "Your password was changed",
    "body": "Your account password was updated. If this was not you, contact support immediately.",
    "variables": [],
    "locale": "en",
    "metadata": { "category": "security" }
  }'
```

### Security: Login Detected

Each login detected event sends up to three templates: in_app, SSE (real-time alert), and email.

```bash
# in_app template
curl -X POST https://freerangenotify.monkeys.support/v1/templates/ \
  -H "X-API-Key: FRN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "app_id": "app-monkeys-xxx",
    "name": "login_detected_inapp",
    "channel": "in_app",
    "subject": "New login from {{.client}} ({{.ip_address}}) via {{.login_method}}",
    "body": "We detected a new login to your account. If this was not you, secure your account.",
    "variables": ["ip_address", "client", "login_method"],
    "locale": "en",
    "metadata": { "category": "security" }
  }'

# email template
curl -X POST https://freerangenotify.monkeys.support/v1/templates/ \
  -H "X-API-Key: FRN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "app_id": "app-monkeys-xxx",
    "name": "login_detected_email",
    "channel": "email",
    "subject": "New login to your account",
    "body": "We detected a new login from {{.client}} ({{.ip_address}}) via {{.login_method}}. If this was not you, secure your account.",
    "variables": ["ip_address", "client", "login_method"],
    "locale": "en",
    "metadata": { "category": "security" }
  }'
```

### Full Template Reference

Once created, store template names in a constants file in the Monkeys backend — you reference them by name, not UUID.

```go
// constants/frn_templates.go

const (
    // Social
    FRNTplNewFollowerInApp = "new_follower_inapp"
    FRNTplNewFollowerSSE   = "new_follower_sse"
    FRNTplNewCommentInApp  = "new_comment_inapp"
    FRNTplNewCommentSSE    = "new_comment_sse"
    FRNTplBlogLikedInApp   = "blog_liked_inapp"
    FRNTplBlogLikedSSE     = "blog_liked_sse"

    // Collaboration
    FRNTplCoAuthorInviteInApp  = "coauthor_invite_inapp"
    FRNTplCoAuthorInviteSSE    = "coauthor_invite_sse"
    FRNTplCoAuthorInviteEmail  = "coauthor_invite_email"
    FRNTplCoAuthorAcceptInApp  = "coauthor_accept_inapp"
    FRNTplCoAuthorAcceptSSE    = "coauthor_accept_sse"
    FRNTplCoAuthorDeclineInApp = "coauthor_decline_inapp"
    FRNTplCoAuthorRemovedInApp = "coauthor_removed_inapp"
    FRNTplCoAuthorRemovedSSE   = "coauthor_removed_sse"

    // Content
    FRNTplBlogPublishedCoAuthorInApp = "blog_published_coauthor_inapp"
    FRNTplBlogPublishedCoAuthorSSE   = "blog_published_coauthor_sse"

    // Security
    FRNTplPasswordChangedInApp  = "password_changed_inapp"
    FRNTplPasswordChangedEmail  = "password_changed_email"
    FRNTplEmailVerifiedInApp    = "email_verified_inapp"
    FRNTplLoginDetectedInApp    = "login_detected_inapp"
    FRNTplLoginDetectedSSE      = "login_detected_sse"
    FRNTplLoginDetectedEmail    = "login_detected_email"
    FRNTplPasswordResetReqInApp = "password_reset_requested_inapp"
    FRNTplPasswordResetReqEmail = "password_reset_requested_email"
    FRNTplEmailChangedInApp     = "email_changed_inapp"
    FRNTplEmailChangedEmail     = "email_changed_email"
    FRNTplUsernameChangedInApp  = "username_changed_inapp"

    // Account lifecycle
    FRNTplAccountDeletedInApp     = "account_deleted_inapp"
    FRNTplAccountDeletedEmail     = "account_deleted_email"
    FRNTplAccountDeactivatedInApp = "account_deactivated_inapp"
    FRNTplAccountReactivatedInApp = "account_reactivated_inapp"
)
```

---

## Step 4: Send Notifications from the Monkeys Backend

The Monkeys backend sends up to three notifications per event — `in_app` (always), `sse` (for real-time), and `email` (for security events). All use the corresponding template and identical `data` payload.

### Helper: send both channels

```go
// internal/notifications/client.go

const frnBaseURL = "https://freerangenotify.monkeys.support/v1"

type NotifyRequest struct {
    UserID     string                 // Monkeys username (external_id) or email
    InAppTpl   string                 // Template name for in_app
    SSETpl     string                 // Template name for sse (empty = skip SSE)
    Priority   string                 // "low", "normal", "high", "critical"
    Category   string                 // "social", "collaboration", "content", "security"
    Data       map[string]interface{}
}

func Notify(ctx context.Context, req NotifyRequest) error {
    // Send in_app (always)
    if err := send(ctx, req.UserID, req.InAppTpl, "in_app", req.Priority, req.Category, req.Data); err != nil {
        return err
    }
    // Send sse (when configured — skipped for low-priority like likes)
    if req.SSETpl != "" {
        _ = send(ctx, req.UserID, req.SSETpl, "sse", req.Priority, req.Category, req.Data)
    }
    return nil
}
```

> The SSE send is fire-and-forget in this pattern. If the user is offline, the in_app notification is already stored. The SSE failure does not need to block or retry.

### Event: User Followed

```go
// Called when user A follows user B

func OnUserFollowed(ctx context.Context, followerUsername, targetUsername string) {
    notifications.Notify(ctx, notifications.NotifyRequest{
        UserID:   targetUsername,    // the person being followed
        InAppTpl: TplNewFollowerInApp,
        SSETpl:   TplNewFollowerSSE,
        Priority: "normal",
        Category: "social",
        Data: map[string]interface{}{
            "follower_name": followerUsername,
        },
    })
}
```

**Equivalent curl (for testing):**

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/notifications/ \
  -H "X-API-Key: FRN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "bob_monkeys",
    "channel": "in_app",
    "priority": "normal",
    "template_id": "new_follower_inapp",
    "category": "social",
    "data": { "follower_name": "alice_monkeys" }
  }'
```

### Event: New Comment

```go
func OnComment(ctx context.Context, commenterUsername, blogAuthorUsername, blogTitle, commentPreview string) {
    // Do not notify if the author comments on their own blog
    if commenterUsername == blogAuthorUsername {
        return
    }

    notifications.Notify(ctx, notifications.NotifyRequest{
        UserID:   blogAuthorUsername,
        InAppTpl: TplNewCommentInApp,
        SSETpl:   TplNewCommentSSE,
        Priority: "normal",
        Category: "social",
        Data: map[string]interface{}{
            "commenter_name":  commenterUsername,
            "blog_title":      blogTitle,
            "comment_preview": commentPreview,
        },
    })
}
```

### Event: New Like

```go
func OnLike(ctx context.Context, likerUsername, blogAuthorUsername, blogTitle string) {
    if likerUsername == blogAuthorUsername {
        return
    }

    // Likes are low-priority — in_app only, no SSE
    notifications.Notify(ctx, notifications.NotifyRequest{
        UserID:   blogAuthorUsername,
        InAppTpl: TplBlogLikedInApp,
        SSETpl:   "", // No SSE for likes — not disruptive enough
        Priority: "low",
        Category: "social",
        Data: map[string]interface{}{
            "liker_name": likerUsername,
            "blog_title": blogTitle,
        },
    })
}
```

### Event: Co-Author Invite

```go
func OnCoAuthorInvite(ctx context.Context, inviterUsername, inviteeUsername, blogTitle string) {
    notifications.Notify(ctx, notifications.NotifyRequest{
        UserID:   inviteeUsername,
        InAppTpl: TplCoAuthorInviteInApp,
        SSETpl:   TplCoAuthorInviteSSE,
        Priority: "high",
        Category: "collaboration",
        Data: map[string]interface{}{
            "inviter_name": inviterUsername,
            "blog_title":   blogTitle,
        },
    })
}
```

### Event: Password Changed

```go
func OnPasswordChanged(ctx context.Context, username string) {
    // Security events are in_app only — no SSE because the user is likely not
    // actively browsing when a password change happens
    notifications.Notify(ctx, notifications.NotifyRequest{
        UserID:   username,
        InAppTpl: TplPasswordChangedInApp,
        SSETpl:   "",
        Priority: "high",
        Category: "security",
        Data:     map[string]interface{}{},
    })
}
```

---

## Step 5: Display Notifications on the Frontend

The Monkeys frontend uses the `@freerangenotify/react` SDK. The API key **never** goes to the browser. Instead, the Monkeys backend generates a short-lived SSE token for each authenticated session.

### Backend: SSE token endpoint

```javascript
// monkeys-backend/api/notifications.js

app.get('/api/notifications/sse-token', requireAuth, async (req, res) => {
  const { sse_token, user_id } = await fetch(
    'https://freerangenotify.monkeys.support/v1/sse/tokens',
    {
      method: 'POST',
      headers: {
        'X-API-Key': process.env.FRN_API_KEY,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ user_id: req.user.username }),
    }
  ).then(r => r.json());

  res.json({ sse_token, user_id });
});
```

### Frontend: Provider setup

```tsx
// monkeys-frontend/components/NotificationProvider.tsx

import { FreeRangeProvider } from '@freerangenotify/react';
import { useAuth } from './AuthContext';

export function NotificationProvider({ children }: { children: React.ReactNode }) {
  const { user } = useAuth();
  const [sseToken, setSseToken] = React.useState<string | null>(null);
  const [frnUserId, setFrnUserId] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!user) return;
    // Fetch a short-lived SSE token from your own backend
    fetch('/api/notifications/sse-token')
      .then(r => r.json())
      .then(({ sse_token, user_id }) => {
        setSseToken(sse_token);
        setFrnUserId(user_id);
      });
  }, [user]);

  if (!sseToken || !frnUserId) return <>{children}</>;

  return (
    <FreeRangeProvider
      apiKey={sseToken}          // SSE token acts as the client-side credential
      userId={frnUserId}
      apiBaseURL="https://freerangenotify.monkeys.support/v1"
    >
      {children}
    </FreeRangeProvider>
  );
}
```

### Frontend: Notification bell

```tsx
// monkeys-frontend/components/Header.tsx

import { NotificationBell } from '@freerangenotify/react';

export function Header() {
  return (
    <header>
      <nav>
        {/* ... other nav items ... */}
        <NotificationBell
          tabs={['All', 'Social', 'Collaboration', 'Security']}
          onNotification={(n) => {
            // Optional: show a toast with the notification title
            toast.info(n.title);
          }}
        />
      </nav>
    </header>
  );
}
```

### Frontend: Custom notification center (advanced)

If the Monkeys design calls for a custom inbox page rather than the drop-in bell:

```tsx
// monkeys-frontend/pages/NotificationsPage.tsx

import { useNotifications, useUnreadCount } from '@freerangenotify/react';

export function NotificationsPage() {
  const { notifications, loading, markRead, markAllRead, archive, loadMore, hasMore } = useNotifications({
    pageSize: 20,
  });
  const { count } = useUnreadCount();

  if (loading) return <Spinner />;

  return (
    <div>
      <div className="flex justify-between items-center">
        <h1>Notifications <span className="badge">{count}</span></h1>
        <button onClick={markAllRead}>Mark all read</button>
      </div>

      {notifications.map(n => (
        <NotificationItem
          key={n.notification_id}
          notification={n}
          onRead={() => markRead([n.notification_id])}
          onArchive={() => archive([n.notification_id])}
        />
      ))}

      {hasMore && <button onClick={loadMore}>Load more</button>}
    </div>
  );
}
```

---

## Step 6: Query the User's Inbox

The Monkeys backend can also query the inbox directly — useful for server-side rendering, email digests, or API endpoints.

```bash
# All notifications for a user (paginated)
curl "https://freerangenotify.monkeys.support/v1/notifications?user_id=alice_monkeys&page=1&page_size=20&channel=in_app" \
  -H "X-API-Key: FRN_API_KEY"

# Unread count
curl "https://freerangenotify.monkeys.support/v1/notifications/unread/count?user_id=alice_monkeys" \
  -H "X-API-Key: FRN_API_KEY"

# Filter by category
curl "https://freerangenotify.monkeys.support/v1/notifications?user_id=alice_monkeys&category=collaboration" \
  -H "X-API-Key: FRN_API_KEY"

# Mark a notification as read (called from the frontend via your own backend proxy)
curl -X POST https://freerangenotify.monkeys.support/v1/notifications/NOTIFICATION_ID/read \
  -H "X-API-Key: FRN_API_KEY"
```

---

## User Preference Respect

FreeRangeNotify automatically honours each user's notification preferences. If a Monkeys user disables `social` category notifications, FRN will silently drop those sends — no changes needed in the Monkeys backend.

Users can be updated with preferences via:

```bash
curl -X PUT https://freerangenotify.monkeys.support/v1/users/alice_monkeys \
  -H "X-API-Key: FRN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "preferences": {
      "categories": {
        "social": { "enabled": false },
        "security": { "enabled": true },
        "collaboration": { "enabled": true }
      }
    }
  }'
```

---

## Complete Event-to-Template Map

| Monkeys Event | FRN Template (in_app) | FRN Template (sse) | Priority | Category |
|---|---|---|---|---|
| `FollowedUser` received | `new_follower_inapp` | `new_follower_sse` | normal | social |
| `CommentBlog` received | `new_comment_inapp` | `new_comment_sse` | normal | social |
| `LikeBlog` received | `blog_liked_inapp` | _(none)_ | low | social |
| `InvitedAsACoAuthor` | `coauthor_invite_inapp` | `coauthor_invite_sse` | high | collaboration |
| `JoinedAsCoAuthor` (inviter view) | `coauthor_accept_inapp` | `coauthor_accept_sse` | normal | collaboration |
| `DeclinedCoAuthor` (inviter view) | `coauthor_decline_inapp` | _(none)_ | normal | collaboration |
| `RemovedFromCoAuthor` | `coauthor_removed_inapp` | `coauthor_removed_sse` | normal | collaboration |
| `PublishedABlogAsCoAuthor` | `blog_published_coauthor_inapp` | `blog_published_coauthor_sse` | normal | content |
| `UpdatedPassword` | `password_changed_inapp` | _(none)_ | high | security |
| `VerifyEmail` | `email_verified_inapp` | _(none)_ | high | security |

---

## What Does Not Get a Notification

The following are audit log events used by internal ops tooling and the Monkeys activity feed. They do **not** create FRN notifications because the user who performed the action is not a different user who needs to be notified.

- `Register`, `Login`, `Logout`
- `UpdateProfile`, `UpdateProfilePic`, `UpdatedUserName`
- `CreateBlog`, `MovedBlogToDraft`, `UpdateBlog`, `DeleteBlog`, `ScheduleBlog`, `ArchiveBlog`
- `FollowedTopics`, `UnFollowedTopics`, `BookMarkedBlog`
- `ChangedVisibilityToAnonymous`, `ChangedVisibilityToPublic`
