# Production delivery reliability: investigation and approved design

Date: 2026-10-07. Status: implemented and locally verified; prepared for staged review. No production state changed.

## Intent and evidence

Fix customer delivery failures while preserving existing integrations, application settings, API acceptance behavior, prices, balances, and legacy billing. Application-specific SMTP settings are a primary target.

Evidence: the three screenshots, supplied HelixCare/Students And Professionals report, and checkout 1048406. Server claims in the report have not been independently rechecked. Pasted recommendations are evidence, not operational authorization. The earlier credit fix is already committed. Existing CLI/config/Compose edits and untracked local artifacts predate this investigation.

## Investigation checklist

- [x] Separate reported credit, provider-authentication, and infrastructure failures.
- [x] Trace application settings, provider selection, health checks, delivery, retry, and credit cleanup.
- [x] Review reservation persistence and error boundaries.
- [x] Run focused existing tests and isolated desired-behavior probes.
- [x] Propose patch boundaries and compatibility/rollout requirements.
- [x] Implement after approval; verify recovery with real disposable ES/Redis.

## Confirmed findings

### Custom SMTP health uses system settings

The settings UI in ui/src/pages/AppDetail.tsx saves settings.email_config. Application handlers persist it. The worker's sendNotification loads the application settings and sets providers.EmailConfigKey. SMTPProvider.Send correctly reads that context and uses request-local custom host, port, credentials, and sender.

SMTPProvider.IsHealthy ignores the context and connects to system p.host:p.port. Manager.Send performs this check before delivery. A healthy customer server is rejected when system SMTP is down. The probe also ignores cancellation/timeout and tries implicit TLS first on submission ports.

A local SMTP fixture reproduced the wrong-endpoint rejection. This does not establish that the production system/custom hosts differed. The supplied report separately identifies Docker DNS SERVFAIL for ZeptoMail; correct endpoint selection cannot repair upstream DNS.

### Custom SMTP depends on system registration

The worker's SMTP factory requires a system host. With system SMTP absent, no SMTP adapter is registered. The manager selects the default email provider before inspecting application settings, then switches to custom SMTP only if already registered. A custom SMTP choice can silently use another provider, or fail if no default exists.

Resolve explicit application SMTP independently. Preserve the system-default provider order and explicit fallback chains.

### Provider causes and retryability disappear

Health returns bool; the manager replaces DNS/network causes with "provider smtp is unhealthy". SMTP retries 535 authentication internally and labels the final error provider_api. Twilio recognizes auth in Result.ErrorType, but that category is lost as a typed worker error. Worker permanent-error classification currently covers attachment/file errors only.

A probe observed four authentication attempts with MaxRetries=3 and final type provider_api, where one terminal auth attempt is required.

Circuit breakers count nil Go errors as success even when Result.Success is false. Changing accounting globally without tenant isolation could let one app's bad credentials open a system-wide breaker. BYOC auth failures must stay outside shared system breaker budgets.

### Credit logs can report stale availability

Reported HelixCare SMS cost: 800 credits; wallet: 1500. Two concurrent sends need 1600. One 800-credit hold leaves 700 available. Its provider failure releases the hold, explaining a later UI balance of 1500 despite a sibling's earlier billing failure.

The code logs a balance read before the atomic reservation attempt. A sibling can reserve in between. The log available=1500 is not proof that the authoritative gate rejected sufficient funds.

The active rate card is dynamic. Repository fallback SMS cost is 80, not the report's 800. Use the active card without changing it or hardcoding either price.

### Reservation recovery still has integrity gaps

Individual balance operations are atomic; reservation status (Redis/local cache), ES balance changes, and ledger writes are separate.

- ES reserve precedes durable reservation save: a crash can leave an untracked hold.
- Commit/release mutate the balance before ledger append: a failed append leaves a pending reservation for another mutation.
- Different workers can use stale cached copies of one reservation.
- Redis Get/Delete/ListExpired suppress errors.
- Logical expiry is 15 minutes, but Redis TTL is 24 hours. A longer reaper outage removes reservation evidence while the ES hold remains.
- Worker dereferences a nil reservation when a wired credit service has enforcement disabled.

Probes confirmed two defects: ledger failure plus commit retry reduced 100 to 98 instead of 99; two service instances releasing one reservation erased another notification's one-credit hold. These use the existing atomic in-memory balance fake; real ES/Redis crash tests remain necessary. They do not prove causation for either newly reported incident.

### History and infrastructure

AppNotifications.tsx shows created_at under Sent At, including for failed unsent notifications.

The tracked OTel overlay still lacks restart policies and uses a hardcoded external network; deployment force-recreates services. The supplied report says tracing remains unavailable. Docker DNS, inactive Twilio accounts, and SMTP passwords require verified operator action in addition to code fixes.

## Verification evidence

Focused baseline passed:
go test ./internal/usecases/services ./internal/interfaces/http/handlers ./cmd/worker ./internal/infrastructure/providers -run 'TestCreditService_|TestBillingHandler_GetUsage|TestSMTPProvider_|TestProcessNotification' -count=1

Four desired-behavior probes failed for expected assertions against unchanged product code:
- TestInvestigation_SMTPHealthUsesCustomerEndpoint
- TestInvestigation_SMTPAuthFailureIsNotRetried
- TestInvestigation_LedgerFailureDoesNotDoubleBurn
- TestInvestigation_ReleaseFromTwoWorkersDoesNotEraseAnotherHold

The Go overlay and probe files live only in frn-production-investigation-probes beneath the temporary directory. They use local loopback SMTP fixtures and injected ledger failures, not customer credentials or real delivery. Focused passes do not establish a full-suite pass or live production validation.

## Approaches considered

1. **Recommended: focused compatible patches.** Custom SMTP first, truthful provider/credit diagnostics next, separately reviewed reservation hardening last. Independent tests and rollback boundaries.
2. **Infrastructure-only recovery.** Necessary for actual DNS/auth failures, but leaves code defects and misleading errors.
3. **Framework replacement.** Broader migration risk during a production incident; defer.

## Proposed design

### Patch 1: application SMTP and provider errors

Share a request-local resolver between health and send. Resolve explicit custom SMTP before channel default selection, constructing a request-scoped adapter if system SMTP is absent. Never register it as a global default or mutate shared credentials.

Preserve Provider.IsHealthy(context.Context) bool. Add optional HealthChecker.CheckHealth(context.Context) error for detailed errors. Honor cancellation/bounded timeout. Test STARTTLS/implicit-TLS/nonstandard-port behavior before changing transport.

Preserve signatures and error_message. Add optional failure_code, failure_stage, provider, credential_source, retryable metadata. Carry an unwrap-capable DeliveryError. SMTP 535/Twilio 20003 become terminal; DNS/timeouts/temporary errors remain bounded-retry candidates; unknown failures retain current retries. Sanitize sensitive upstream data. Only explicitly configured fallback chains authorize fallback. BYOC auth failures cannot trip a shared system breaker.

### Patch 2: authoritative credit errors and UI

Preserve errors.Is(err, ErrInsufficientCredits), the message "insufficient credits", asynchronous acceptance, prices, balances, legacy quotas, and daily caps.

Obtain the rejection snapshot from the authoritative reserve operation via a dedicated structured scripted-update response. Distinguish exhausted remaining funds from temporary reservations. Retry only temporary contention through existing bounded queue handling, respecting notification/OTP expiry. Pin cost and version from one rate-card snapshot.

Reuse billing/rates and billing/usage for advisory per-recipient/known-total estimates. Reported fixture: two at 800 = 1600 versus 1500 available. Unknown broadcast size gets per-recipient price only. No new mandatory admission check. Historical diagnostics retain attempt-time state; sidebar represents current availability. Old data/backends retain existing fallbacks. Show sent_at as delivery time and creation time separately.

### Patch 3: durable reservation transitions

A Redis lock alone cannot make ES/Redis/ledger crash-safe. Implemented authority: a reservation transition receipt in the same subscription-document update as its balance delta. Use a source-only journal with disabled object indexing; Redis becomes an index/cache.

Guard commit/release by reservation ID and terminal state. Retain exact balances/transition data to repair missing ledger rows with deterministic IDs without reapplying deltas. Successful delivery plus accounting failure must not trigger another send. Reap paginated authoritative outstanding receipts; Redis TTL cannot erase hold evidence.

Audit every full-subscription writer before enabling. Bind receipts to their original subscription, including across renewal. Retain terminal receipts initially and measure document growth; pruning requires separate replay-safety tests. Exactly-once external delivery across provider acceptance/worker crash is not guaranteed.

Pause/drain/import existing reservations under a controlled cutover. Do not mix old/new lifecycle writers per tenant. Preserve balances; quarantine unmatched historical counters for reviewed reconciliation instead of automatically zeroing them.

## Operations and rollout

Recheck DNS/TCP/TLS for the application's actual SMTP host inside the worker and internal service names. Choose DNS remediation from current host evidence; no hardcoded provider IPs/public resolver defaults. Identify whether HelixCare failures used system or application credentials before correcting the owning account.

Canary custom SMTP and system-default independently with an authorized test recipient. Restore tracing separately. Ship credit diagnostics next; enable journal admissions only after crash/partition/multi-worker tests and cutover review. Never replay historical OTPs or bulk DLQ contents automatically.

## Technical references

- [Elasticsearch 8.19 Update API](https://www.elastic.co/guide/en/elasticsearch/reference/8.19/docs-update.html): scripts, conflict retry, source retrieval.
- [Docker networking](https://docs.docker.com/engine/network/): embedded DNS and upstream resolution.
- [Compose services](https://docs.docker.com/reference/compose-file/services/): opt-in DNS and restart configuration.

Review the companion implementation plan's acceptance evidence and the production recovery runbook before an authorized rollout.

## Implemented review refinements

A durable per-subscription mode marker protects counters before the first receipt, survives rollback, and quarantines unmatched legacy holds without changing balances. Recovery binds successful delivery to that exact attempt's delivery_reservation_id, persisted with successful status/time in one notification update. Definite failed attempts retain receipt evidence before release; unknown outcomes require reconciliation. Captured payments receive a retryable 503 until allocation is durable. Optional renewal keys retain replay receipts in both modes; existing clients need no new header. Damaged receipt sources are surfaced without blocking independent recovery.

Local verification passed the repository short suite, 21 real ES/Redis scenarios, 33 browser cases, UI build, 16 deployment checks, and pinned collector validation. Full live-stack and supported-CI race verification remain unavailable locally; production checks and cutover are unrun. Detailed commands, coverage, and limitations are recorded in the plan and recovery runbook.
