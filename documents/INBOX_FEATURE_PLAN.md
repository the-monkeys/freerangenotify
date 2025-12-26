# Inbox Feature Plan (Unread Badge, Mark Read, Infinite Scroll)

## Goals
- Show unread count fast.
- Mark-as-read on click; optional mark-all-read.
- Persist recent history and load older pages with infinite scroll.
- Fit existing stack: Fiber API, Redis, Elasticsearch, workers.

## Data Model
- Use existing `status` and `read_at` in notifications index.
  - Unread: `status != "read"`.
  - Read: `status == "read"` and `read_at` set.
- Sorting for pagination: `created_at desc`, `notification_id desc` (search_after cursor).
- No schema change unless `read_at` is missing (add if needed).

## Redis Unread Counter
- Key: `frn:unread:<user_id>`.
- Increment once when a notification is enqueued.
- Decrement on successful mark-read (only if status changed unread→read).
- Reset to 0 on mark-all-read.
- On cache miss/drift: recompute from ES (`status != read`), then set counter.
- Clamp counter at zero to avoid negatives.

## APIs (all API-key protected)
- `GET /v1/notifications/unread/count` → `{count:int}` (Redis-first, ES fallback).
- `GET /v1/notifications?cursor=<created_at,notification_id>&page_size=20&unread_only=true|false`
  - Returns notifications plus `next_cursor` (search_after token); sorted by `created_at desc, notification_id desc`.
- `POST /v1/notifications/:id/read` → marks one as read; returns updated notification.
- `POST /v1/notifications/read-all` (optional `before=<timestamp>` to bound scope) → marks all unread as read and resets counter.

## Server Logic
- Mark single read:
  - Fetch doc; if already read, no-op.
  - Update ES: `status=read`, `read_at=now`.
  - Decrement Redis counter only if it transitioned from unread.
- Mark all read:
  - ES bulk update: filter `user_id` AND `status != read` (optionally `created_at <= now`).
  - Reset Redis unread counter to 0.
- List with cursor:
  - ES query with filters: `user_id`, optional `status != read` when `unread_only=true`.
  - Sort: `created_at desc`, `notification_id desc`.
  - Use search_after from `cursor`; return `next_cursor`.

## Client Behavior (Receiver/UI)
- On load: fetch unread count for badge; fetch first page.
- On click: call mark-read; optimistically decrement badge.
- Infinite scroll: request with `next_cursor` until empty.
- (Future) SSE/WebSocket for live badge updates; not required initially.

## Backfill / Consistency
- Admin job/API to recompute unread counters from ES and repopulate Redis (rarely needed).

## Testing
- Unit: counter inc/dec/reset; mark-read idempotency; search_after pagination; read-all reset; clamp at zero.
- Integration: enqueue N, verify count, mark some read, verify count; ensure DLQ/replay does not double-count.
- Concurrency: concurrent mark-read should not double-decrement (check ES state before decrementing counter).

## Minimal Code Changes
- Repository: list with search_after + unread filter; bulk mark read; single mark read returning prior status.
- Service layer: methods `GetUnreadCount`, `ListWithCursor`, `MarkRead`, `MarkAllRead` using Redis counter.
- Redis helper: get/inc/dec/reset unread count with clamp.
- HTTP handlers/routes: add endpoints above; wire API key auth.

## Decisions (confirmed)
- Add `before` parameter to mark-all-read to bound bulk updates.
- Max page size: default 20, hard cap 100.
- Expose `unread_only` filter on list.
- Rate-limit mark-all-read per user/app.

## Rollout Steps
1) Pagination + filters: implement search_after list with `unread_only`, default 20, cap 100.
2) Mark single read + unread counter: ensure decrement is guarded by state transition.
3) Mark-all-read with `before` and per-user/app rate limit; reset counter to 0.
4) Unread count endpoint wired to Redis with ES fallback and clamp at zero.
5) Admin backfill to recompute counters (optional if time permits).
