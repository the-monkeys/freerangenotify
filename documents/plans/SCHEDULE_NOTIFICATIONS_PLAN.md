# Schedule Notifications – Implementation Plan

Date: 2026-04-05  
Owner: Notifications/Workflow team  
Scope: Scheduling of notifications created via Notification API, Workflow delay steps, and Workflow cron schedules. Preserve backward compatibility and at-least-once delivery when the hub/worker are down past the scheduled fire time.

## Desired Behaviour (including outage scenario)
- If a notification is scheduled for 10:00 and the hub/worker are down from 09:55 to 11:00, it **must be delivered shortly after recovery** (catch-up) without caller replays.
- No duplicate sends for the same `notification_id`; status transitions remain valid.
- Workflows with delay steps and cron schedules also catch up after downtime, bounded to a safe window to avoid storms.
- Workflow delay steps are wall-clock based: a 6h delay should still complete 6h after it started, even if the system was down for 1h in the middle (so only 5h remain after recovery).

## Current State (code read)
- Notification scheduling persists `scheduled_at` in Elasticsearch and status `pending`; it is also enqueued in Redis ZSET `frn:queue:scheduled`. Worker scheduler moves ready items to live queues and flips status to `queued` (cmd/worker/processor.go).
- Fallback: scheduler also queries ES for `status=pending` with `scheduled_at <= now` (repository.GetPending) to rescue items if Redis data is missing.
- Workflow delays use Redis sorted set `frn:workflow:delayed`; the workflow engine’s `delayedPoller` re-enqueues due steps (internal/infrastructure/orchestrator/engine.go).
- Workflow cron schedules are polled each minute by `SchedulePoller` (internal/infrastructure/orchestrator/schedule_poller.go) but it fires **only when the current minute matches**, so missed minutes during downtime are skipped.

## Gaps
- Cron-based workflow schedules do not catch up after downtime; missed ticks are silently dropped.
- No explicit “late delivery” marker for observability when a scheduled notification fired after its target time.
- Retry/backfill behaviour is not configurable; heavy downtime could enqueue many items at once without back-pressure.

## High-Level Design
- **Source of truth = Elasticsearch** for scheduled_at/status; **Redis = fast path**. Keep existing schema to stay backward compatible.
- **Catch-up semantics:** on service restart, process every item whose scheduled time is ≤ now, limited by configurable windows to avoid storms. Prefer “send once, even if late” over “drop”.
- **Late marker:** when a notification or workflow step fires after its scheduled_at by more than a threshold (config), add `metadata["delayed_by_ms"]` in memory for logging/metrics (no API schema change).
- **Back-pressure:** cap catch-up batch sizes and pace using existing queue priorities plus a new optional rate limiter for catch-up batches.

## Detailed Implementation Steps

1) **Notification Scheduler Hardening** (cmd/worker/processor.go, internal/infrastructure/queue/redis_queue.go)  
   - Keep existing Redis + ES fallback.  
   - Add an optional config `scheduler.catchup_batch_size` (default 200) and `scheduler.late_threshold_ms` (default 60000). Apply when moving items from Redis and ES.  
   - When enqueueing late items, log `delayed_by_ms` and include it in activity publish; do **not** change API surface or stored document.

2) **Workflow Cron Catch-Up** (internal/infrastructure/orchestrator/schedule_poller.go)  
   - Replace `cronMatchesNow` check with a catch-up loop that computes missed fire times between `max(last_run_at, created_at)` and `now`, up to `schedule.catchup_window_minutes` (default 1440, i.e., 24h).  
   - For each missed tick, trigger once per tick (preserves semantics) but cap total executions per schedule per tick loop with `schedule.catchup_max_runs` (default 100).  
   - Update `LastRunAt` to the latest executed tick.  
   - Keep existing timezone handling; reuse the robfig/cron parser already validated in service layer.

3) **Workflow Delay Catch-Up Verification & Semantics** (internal/infrastructure/orchestrator/engine.go, internal/infrastructure/queue/redis_queue.go)  
   - Keep absolute `execute_at` timestamps (already used) so wall-clock time keeps running during downtime; no delay is restarted.  
   - Ensure delayedPoller runs once at startup (already does). Add a startup log and a metrics increment when delayed items are drained to confirm catch-up worked after outages.  
   - Add guardrail `workflow.delayed.catchup_batch_size` (default 500).  
   - When a delayed item is late, log `delayed_by_ms` and tag metric to show downtime was absorbed (e.g., 6h delay, 1h outage -> ~1h late).

4) **Configuration Surface** (internal/config/config.go, config files)  
   - Add new optional fields under `queue` or `scheduler` for the batch sizes and catch-up windows noted above; defaults chosen to maintain current behaviour if unset.

5) **Observability**  
   - Emit structured logs with `delayed_by_ms`, `scheduled_at`, and `recovered_from_outage=true` when catch-up paths run.  
   - Add metrics: `scheduler_catchup_count`, `workflow_schedule_catchup_count`, `delayed_step_catchup_count`, with tags `app_id`, `env`, `late_bucket`.

6) **Backwards Compatibility**  
   - No API contract changes.  
   - ES mappings unchanged; only additional metadata injected into logs/metrics.  
   - Defaults keep current behaviour: notifications already catch up; workflow schedules will now catch up (additive improvement).

## Code Touch Points
- `cmd/worker/processor.go`: scheduler loop config + late logging.  
- `internal/infrastructure/queue/redis_queue.go`: optional batch size parameterization (non-breaking default).  
- `internal/infrastructure/orchestrator/schedule_poller.go`: catch-up loop using `LastRunAt`.  
- `internal/config/config.go` and sample configs/env: new optional fields.  
- Tests under `internal/infrastructure/orchestrator` and `cmd/worker` packages.

## Test Plan
- **Unit tests**
  - `schedule_poller_test.go`: simulate `LastRunAt` 90 minutes ago with cron `*/30 * * * *`; assert 3 triggers executed and `LastRunAt` advanced.  
  - `processor_scheduler_test.go`: seed Redis ZSET with past scheduled items and ensure they move to queue and statuses set to `queued`; verify late metric/log flag.  
  - `workflow_delayed_test.go`: ensure delayedPoller drains past-due items on startup and respects batch size; include case: delay 6h, advance clock 5h with 1h simulated downtime, expect item enqueued immediately and `delayed_by_ms` ≈ 0 (wall-clock preserved).
  - `notification_repository_test.go`: verify `GetPending` returns items with `scheduled_at <= now`.
- **Integration tests** (tests/ or new in tests/):  
  - Spin up docker-compose, schedule notifications for near-future, stop worker container before fire time, restart after time passes, assert delivery via status = `sent` and provider mock received payload.  
  - Cron schedule test: create workflow schedule for `* * * * *`, stop worker for 3 minutes, restart, assert three workflow triggers executed.  
  - Workflow delay test: enqueue delay step of 2 minutes, stop engine for 5 minutes, restart, assert step continues and notification delivered once.

## Rollout & Ops
- Ship behind config defaults; monitor new metrics for spikes.  
- If backlog is large, operators can temporarily reduce worker_count or increase batch sizes safely—no data migration required.  
- Document recovery expectations in `SMART_DELIVERY_GUIDE.md` follow-up.
