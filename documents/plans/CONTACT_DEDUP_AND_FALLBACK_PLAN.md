# Contact-Aware Delivery Guardrails (Skip Missing, De-duplicate)  

## Goals  
- Skip WhatsApp/SMS sends when no mobile is available; skip email sends when no email is available.  
- For bulk/broadcast/subscriber sends, deliver **at most one message per unique contact** (per channel) even if duplicated user/contact entries exist.  
- Keep queues fast for 100k+ recipients and avoid delaying scheduled notifications.  
- Backward compatible: no API contract changes; producers keep calling existing endpoints.  

## Scope  
In-scope: notification pipeline (broadcast, bulk, workflow/schedule, topic/subscriber sends), worker delivery path, queue payload shape, observability.  
Out-of-scope: contact enrichment/validation (we only normalize), campaign-level throttling (unchanged).  

## Functional Requirements  
- Channel gating:  
  - `sms`/`whatsapp`: require normalized E.164 phone. If missing, mark as skipped (reason `missing_phone`).  
  - `email`: require normalized email. If missing, mark as skipped (reason `missing_email`).  
- Dedup per delivery batch: for a given job/run, only one message per `(channel, normalized_contact)` is enqueued/sent.  
- Scheduled/queued messages must dedup **before enqueue** to avoid clogging the queue.  
- Idempotent across worker restarts; duplicate queue messages should still coalesce via idempotency keys.  
- Audit/metrics: count skipped, deduped, sent.  

## Non-Functional Requirements  
- Handle 100k recipients with O(n) memory bounded by chunking (no full in-memory set).  
- No impact to existing clients or SDKs.  
- No additional DB writes on the hot path for dedup (in-memory/Bloom + per-job cache).  

## High-Level Design  
1) **Normalization layer (shared lib)**  
   - `NormalizePhone(msisdn)` → E.164, drop punctuation; returns `""` on failure.  
   - `NormalizeEmail(email)` → lower-case trimmed.  
   - Lives in `internal/domain/notification/normalizer.go` (new), imported by orchestrator/broadcast/services.  
2) **Preparation phase (orchestrator/bulk builder)**  
   - For each target user/contact:  
     - Compute `contact_key = channel + ":" + normalized_contact`.  
     - If normalized is empty → mark skipped (reason missing).  
     - If `contact_key` already seen in this job → mark deduped.  
     - Else enqueue payload.  
   - Maintain `SeenSet` per job using:  
     - In-memory hash set for chunks of 10k;  
     - Optional Bloom filter to cap memory when >100k (false positives acceptable → safe extra dedup).  
3) **Queue payload**  
   - Add `contact_key` and `normalized_contact` to message metadata (optional fields; default empty for backward compatibility).  
   - Idempotency key becomes `job_id + contact_key + template_id + schedule_time`.  
4) **Worker send path**  
   - Re-check contact presence (defensive). If missing, log skip and ack.  
   - Rebuild idempotency key from metadata; rely on existing idempotency repository to drop duplicates if any slipped through.  
5) **Scheduled / delayed flows**  
   - Dedup performed at scheduling time; the scheduled items already unique.  
   - For workflow “delay” nodes, maintain `contact_key` in execution context to avoid re-enqueue duplicates after resume.  
6) **Observability**  
   - Emit counters: `notifications_skipped_missing_contact{channel}`, `notifications_skipped_dedup{channel}`, `notifications_enqueued{channel}`.  
   - Audit log entry reason when a send is skipped.  

## Low-Level Design / Code Touch Points  
- `internal/infrastructure/orchestrator`  
  - Build stage: call normalizers, construct `contact_key`, apply per-job dedup set.  
  - Attach `contact_key` to queue message metadata.  
- `internal/usecases/services/notification_service.go` (or broadcast service)  
  - Integrate normalization/dedup helper for bulk/topic workflows.  
- `internal/domain/notification/normalizer.go` (new)  
  - Export `NormalizePhone`, `NormalizeEmail`.  
- `internal/infrastructure/queue/message.go` (if exists)  
  - Add optional fields `ContactKey`, `NormalizedContact` (JSON omitempty).  
- `internal/infrastructure/idempotency`  
  - Ensure idempotency key builder can use `ContactKey` when present; fallback to previous key if absent.  
- `internal/interfaces/http/handlers` (only if validation added)  
  - Do **not** reject requests; let pipeline skip to stay backward compatible.  
- `tests/integration`  
  - Add bulk send test that includes missing phone/email and duplicates; expect correct counts.  

## Performance Strategy (100k recipients)  
- Chunk processing (e.g., 5–10k per chunk) with a fresh in-memory set per chunk plus a Bloom filter per job for cross-chunk detection.  
- Avoid DB lookups per recipient; load audience with projection of only `user_id`, `email`, `phone`.  
- Keep queue fan-out similar; dedup reduces total messages.  

## Backward Compatibility  
- No schema migrations required; new metadata fields are optional.  
- Old workers still function (idempotency uses old keys); new workers provide stronger dedup.  
- Skipping missing contacts replaces previous hard errors; behavior change is safer and requested.  

## Rollout Plan  
1) Ship normalizer + orchestrator dedup behind flag `FEATURE_DEDUP_CONTACTS` (default true).  
2) Deploy worker first (understands new metadata), then orchestrator.  
3) Monitor skip/dedup metrics; compare send volume vs baseline.  

## Testing Plan  
- Unit: normalization functions; dedup set behavior; idempotency key generation with/without contact_key.  
- Integration: bulk send with 10 recipients including missing phone/email and duplicates across email/SMS → expect skip counts and single delivery per contact.  
- Workflow delay resume: enqueue same contact twice after crash → only one send.  
- Load test: 100k recipients, measure prep time < acceptable SLA and queue size == unique contacts.  

## Open Questions / To Refine  
- Choose Bloom filter parameters (expected n=200k, fp~1%).  
- Whether to persist skip/dedup reasons to analytics index for UI surfacing.  

