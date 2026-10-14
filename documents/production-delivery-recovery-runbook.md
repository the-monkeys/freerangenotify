# Production delivery recovery — Task 6

## WhatsApp/SMS Phase 1 — October 2026

Ordinary application, settings, provider-list and environment reads redact
credentials. The `api_key` field in dashboard reads is a public `dashboard:<app>`
or `dashboard:<app>:<environment>` reference requiring a valid dashboard JWT and
current app permissions. Existing API-key-only SDK authentication remains valid.
Creation and explicit owner-only reveal return a real key; reveal uses
`Cache-Control: no-store`. Code samples use placeholders. Previously copied keys
remain valid until separately authorized rotation. Release the matching frontend
and API together. Do not roll back to secret-bearing ordinary responses.

Settings responses include `secret_configured`. Empty, masked and omitted secrets
preserve stored credentials; explicit `clear_secrets` paths remove them. Dashboard
patches carry `settings_revision`; stale saves return 409 and require reload.
Legacy request shapes remain accepted. Unchanged browser settings cannot replace
a newly connected account. Automatic replies are explicitly unavailable until
shared dispatch/accounting is implemented.

Configure `server.public_url` to the real public callback origin (including any
mount prefix); provider callback URLs must match exactly. Forwarded host/protocol
headers cannot select signing identity. Meta requires the owning application's
signing secret; Twilio uses the owning account's token. Missing signing credentials
fail closed. Verify sender/WABA ownership and remove ambiguous legacy assignments
before a separately authorized canary. Unresolved senders are rejected for vendor
retry instead of storing an unauthenticated payload or assigning it arbitrarily;
operators must repair mappings before vendor retries expire.

Elasticsearch needs read/create/update/search access to `channel_sender_bindings`
and `channel_events`, plus existing app/message/notification indices. Claims are
immutable; a sender transfer requires audited migration rather than a settings
change. Encrypted connection migration is Phase 2.

Callback acknowledgements follow durable acceptance. Recovery runs in compatible
API/worker processes with CAS leases. Pending/expired processing receipts retry
with bounded delays; five failures produce `dead_letter`. `unknown` means a
workflow or forwarded webhook might have run, and is never automatically replayed.
Inspect the receiver using the stable `channel-event:<receipt-id>` transaction/
idempotency key before an explicitly authorized resolution. Receiver idempotency
is necessary for exactly-once external effects. SSE refresh signals can repeat;
consumers refresh stored state instead of performing business actions from them.

Monitor receipt counts by state, pending age and attempts without dumping customer
payloads. Keep recovery readers running during an admissions rollback. Do not
delete claims/receipts, reset unknown events blindly, replay production messages,
or alter balances as part of deploying these code changes.

Inbox rows deduplicate provider message identities; read markers reduce unread
counts. Conditional delivery transitions protect app ownership and prevent late
failure from overwriting delivered/read. Meta service windows expire from validated
provider time. Twilio's default incoming webhook omits creation timestamps
([Twilio documentation](https://help.twilio.com/articles/46569585455003)); receipt
time cannot open a window. Phase 3 needs an account-bound timestamp resolver before
admitting free-form Twilio replies through shared dispatch. Legacy ingestion
continues. Tests use mock providers and disposable local ES/Redis; no production
rollout, live sends, credential changes or balance repair have been performed.

Later phases still cover encrypted connections, onboarding, outbound dispatch,
pricing, OTP, domestic SMS and environment simplification.

The Phase 1 review found eight release blockers. Regressions now cover spoofed
public SSE identities, nested configuration loss, overview-induced stale forms,
disconnect concurrency, definite pre-dispatch workflow failures, sender limits
including disconnected records, shared-WABA signing contexts and status refresh
recovery. Workflow `Trigger` retains its public return/error contract; the callback
processor uses an optional outcome method to distinguish safe retries from
uncertain writes/enqueues. Named webhook display URLs also round-trip without
erasing the stored path/token. No minor findings were deferred.

Private inbox events require a server-issued dashboard principal on each SSE
connection, with expiry and current application authorization. Subscriber and
legacy public connections cannot become dashboard connections by supplying an
owner UUID. Reload dashboards to obtain the new token format. Ensure every API
instance consuming SSE events has the principal gate before starting compatible
callback producers; an old reader can broadcast private events globally. Use a
coordinated cutover and retain compatible recovery readers during rollback.

Local phase decisions: reuse the user's checkout on one branch, preserve existing
edits, adapt bookkeeping to PowerShell, put common authorization in the auth
domain, use public dashboard references for both app/environment keys, and keep
owner key reveal explicit. Routing and receipt processing share one task commit
to avoid incomplete bootstrap. Sender migration, key rotation and live recovery
remain operator work. External outcome quarantine, repeatable SSE refreshes and
Twilio timestamp requirements described above are deliberate limits; automatic
replay cannot replace receiver idempotency or a provider creation timestamp.

This is an operator procedure, not evidence of live recovery. Implementing Task 6
does not authorize production deployment, credential/account changes, messages,
balance adjustments, migration, or replay. Obtain incident-specific authorization
for those actions and record the owner, affected application, patch/image digest,
time window, and approved recipients before executing them. Keep credentials and
customer payloads out of terminal output, incident notes, and traces.

## Preflight and deployment configuration

1. Identify the current Compose project from the running application's
   `com.docker.compose.project` label, and the actual network and volume names.
   Preserve that project name throughout recovery. Do not guess `freerangenotify`
   or switch a running `prod` project to a different project/volume namespace.
2. Save the currently deployed image digests, Compose files, redacted environment
   settings, provider record IDs, and baseline delivery/credit/queue observations.
   Confirm that the release image contains all approved application patches.
3. Use Bash 4+ and Docker Compose v2+ on the deployment host. The checked-in
   `prod/deploy.sh` uses `prod/docker-compose.yaml` (or `.yml`), its `prod/.env`,
   and the root OTel overlay by default. It preserves the base project directory
   and exports an absolute `FREERANGE_OTEL_CONFIG_DIR` for overlay bind mounts.
   Deploy the root `config/` directory alongside `prod/`; missing files fail fast.
4. Grafana keeps the legacy `admin` provisioning default when no
   `FREERANGE_GRAFANA_ADMIN_PASSWORD` is supplied, so application-only deployment
   does not require a new Grafana secret. Grafana binds to loopback and anonymous
   access remains disabled. An authorized operator can opt into a strong password
   through protected deployment configuration when provisioning a new volume.
   Existing Grafana data volumes keep their current admin password; changing this
   variable does not rotate an existing password. Rotation is a separate,
   explicitly authorized Grafana password-reset action. Do not publish Grafana or
   open a broadly accessible proxy with default credentials; use an approved,
   access-controlled tunnel/proxy. The deploy script never generates, replaces,
   or resets Grafana credentials.

From the repository root, config-only validation is:

```bash
# Use the already-running project's name, not this literal placeholder.
export COMPOSE_PROJECT_NAME="$EXISTING_PROJECT_NAME"
bash prod/deploy.sh --check
```

The script creates and removes a temporary Compose defaults file; it does not
edit the base Compose or `.env`. Both API and worker preserve the shipped prod
base's `staging` application/process mode by default, while labeling traces
`production` independently. They receive debug disabled (including worker
`DEBUG=false`), `info` logging, `otel-collector:4317`, and a 0.05 trace sample ratio.
An explicit `FREERANGE_APP_ENVIRONMENT` from the shell or `prod/.env` overrides
the app mode and supplies the default trace tag. `FREERANGE_OTEL_ENV` takes
precedence when explicitly set; endpoint and sampling remain configurable.
Thus no override means app `staging` / trace `production`; an explicit app
`staging` means trace `staging` unless its trace tag is also explicitly set.
`FREERANGE_OTEL_ENABLED=false` keeps tracing off even when the stack is included.

The application loader reads `config/config.yaml` and overlays `FREERANGE_*`
variables. It does **not** automatically select `config/config.prod.yaml`; that
file has literal placeholders and a Kafka queue default. Do not mount it blindly
or assume its values are effective. The current API enables Fiber prefork from
`app.environment == production` and does not read `FREERANGE_SERVER_PREFORK=false`.
Task 6 does not enable prefork by default or add a prefork feature. Explicitly
switching the app mode to production is a separate application behavior change:
it multiplies exporters, pools, initialization and background jobs, including
uncoordinated billing schedulers. Such a change needs independent lifecycle
review; a production trace tag alone must not trigger it. The `prod` base also
expects an external Elasticsearch deployment unless the operator provisions one;
verify its real host rather than assuming a local `elasticsearch` service exists.

Script options (set selectors in the calling shell, not only in `.env`):

- `FREERANGE_ENABLE_OTEL=0`: omit the OTel stack and force application tracing off.
  This does not remove existing tracing containers or volumes.
- `OTEL_COMPOSE_FILE=/absolute/path/overlay.yml`: explicit alternate tracing file;
  otherwise a `prod/` tracing file takes precedence over the root overlay.
- `FREERANGE_OTEL_CONFIG_DIR=/absolute/path/config`: alternate collector/Tempo/
  Grafana config source directory.
- `DEPLOY_OVERRIDE_FILE=/absolute/path/operator.yml`: optional final overlay,
  for reviewed deployment-specific settings such as DNS. It takes precedence over
  generated defaults; audit it accordingly. Relative mount paths resolve to `prod/`.
- `PULL_RETRIES` (positive integer, default 5) and `PULL_BACKOFF` (seconds, default
  10): finite parallel attempts, then finite sequential attempts per selected image.
- `--check [SERVICE ...]`: config and service-name validation only; no pulls/up.
- `[SERVICE ...]`: pull and reconcile only these services with `--no-deps`.
  Dependencies must already be running. No arguments reconciles the combined
  project, recreating services only when their image/config differs. There is no
  blanket `--force-recreate`, orphan removal, or image pruning. `up --pull never`
  uses the already-pulled images instead of bypassing bounded pull retries.

## DNS, TCP, TLS, and provider ownership

For HelixCare's failing rows, record notification/application IDs, failure time,
provider, failure category/stage, and `credential_source` when available. Older
rows may only have `error_message`; do not infer their credential source from a
current global setting. Inspect the authorized application's provider settings
and the effective provider-selection path for that attempt:

- **System SMTP/Twilio:** correct the platform-owned account/settings through the
  authorized platform operator path. Verify active account, sender/region/endpoint,
  credential version, and configured system fallback.
- **BYOC/application SMTP/Twilio:** the application owner verifies its specific
  host, account/subaccount, credentials, sender permissions, and restrictions.
  Global credentials are not a remedy for application-owned authentication.

SMTP 535 and Twilio 20003/auth/inactive-account failures require the owning
operator's intervention; code cannot activate an account or infer a password.
DNS/TCP/TLS fixes cannot resolve those failures. Do not log passwords, API tokens,
AUTH exchanges, or entire provider request/response payloads. Never test SMTP AUTH
or send an email/SMS merely to diagnose DNS or TLS.

For a `prod` deployment, define an inspection helper using its existing project:

```bash
REPO="$(pwd)"
: "${COMPOSE_PROJECT_NAME:?Set the existing application Compose project}"
export FREERANGE_OTEL_CONFIG_DIR="$REPO/config"
dc() {
  docker compose --project-directory "$REPO/prod" --env-file "$REPO/prod/.env" \
    -p "$COMPOSE_PROJECT_NAME" -f "$REPO/prod/docker-compose.yaml" \
    -f "$REPO/docker-compose.otel.yml" "$@"
}
```

This helper is for inspection and tracing-only service management. Application
deployment uses `prod/deploy.sh` so its production defaults are included. Adapt
the helper's base file for a root/deploy deployment; keep project identity and
base directory consistent.

Set `SMTP_HOST` and `SMTP_PORT` from the **actual selected** application or system
provider; printing the global SMTP host alone does not identify a BYOC endpoint.
From inside the worker, inspect resolver configuration and resolve that host,
another relevant external provider, and internal dependencies:

```bash
: "${SMTP_HOST:?Set the selected provider hostname}"
: "${SMTP_PORT:?Set the selected provider port}"
dc exec -T notification-worker cat /etc/resolv.conf
dc exec -T notification-worker sh -c 'nslookup "$1"' sh "$SMTP_HOST"
dc exec -T notification-worker nslookup api.twilio.com
dc exec -T notification-worker nslookup redis
dc exec -T notification-worker nslookup otel-collector
# Also resolve the deployment's actual external Elasticsearch host.
```

Check the host resolver and Docker upstream resolver path, compare timeout versus
SERVFAIL versus NXDOMAIN, and check firewall/routing. Docker's embedded DNS on
user-defined networks preserves service discovery and forwards external queries.
If diagnostic utilities are unavailable, use an operator-approved, pinned
diagnostic image in the worker's network namespace; do not install tooling into
the running worker. The following opens TCP/TLS only, without AUTH or mail:

```bash
WORKER_ID="$(dc ps -q notification-worker)"
: "${WORKER_ID:?Worker must be running}"
: "${DIAGNOSTIC_IMAGE:?Set an approved pinned image with nc, timeout and openssl}"
# TCP only, any selected SMTP port:
docker run --rm --read-only --cap-drop ALL --network "container:$WORKER_ID" \
  "$DIAGNOSTIC_IMAGE" sh -c 'nc -z -w 5 "$1" "$2"' sh "$SMTP_HOST" "$SMTP_PORT"
# STARTTLS SMTP (typically 587; use only if this provider uses STARTTLS):
docker run --rm --read-only --cap-drop ALL --network "container:$WORKER_ID" \
  "$DIAGNOSTIC_IMAGE" sh -c \
  'timeout 15 openssl s_client -starttls smtp -connect "$1:$2" -servername "$1" -verify_hostname "$1" -verify_return_error </dev/null' \
  sh "$SMTP_HOST" "$SMTP_PORT"
# For implicit TLS (typically 465), omit -starttls smtp; retain certificate checks.
```

Choose any DNS remedy only from current host evidence and deployment policy. An
approved reachable internal/corporate resolver must resolve external provider
names and any private zones. No public resolver defaults, static SMTP IPs,
`extra_hosts` SMTP pinning, or replacement of Docker service discovery.

If evidence warrants per-service DNS, create an **operator-owned**, untracked
overlay, applying it only to services whose external resolution is faulty:

```yaml
services:
  notification-worker:
    dns:
      - "${FREERANGE_APPROVED_DNS:?Set the evidence-validated resolver address}"
  # Add notification-service only if its DNS also needs the same remedy.
```

Validate before applying; then reconcile exactly the affected service(s):

```bash
export DEPLOY_OVERRIDE_FILE=/absolute/path/operator-dns.yml
# FREERANGE_APPROVED_DNS comes from authorized host/network evidence.
bash prod/deploy.sh --check notification-worker
# Only after incident-specific deployment authorization:
bash prod/deploy.sh notification-worker
```

Compose detects the DNS change and recreates that service. Repeat actual SMTP,
external DB, Redis, and collector lookups plus TCP/TLS checks after the change.
For rollback, remove the opt-in override and reconcile those same services with
the previously approved image; do not recreate Redis/DB or reset balances.

## Restore tracing independently

The OTel overlay pins collector 0.130.0, Tempo 2.8.2, and Grafana 12.0.0 and sets
`restart: unless-stopped` on all three. The collector has a 256 MiB container
limit, 160 MiB Go memory target, a first-position memory limiter (200 MiB hard /
160 MiB soft heap thresholds, checked each second), batches capped at 512 spans,
a 128-request in-memory exporter queue with two consumers, five-second export
attempts, and retry backoff of 1–10 seconds limited to 60 seconds per export.
Queues are measured in batches, not bytes. Large spans and process overhead can
still trigger the container limit; tune based on measured load. Full queues,
retry exhaustion, and restarts can drop traces. This telemetry queue is not the
business notification queue and does not grant notification replay authority.

No debug exporter logs span payloads, and OTLP 4317/4318 is not published on the
host. Application containers use `otel-collector:4317` on their shared network.
Tempo HTTP and Grafana bind to loopback (3200 and 3002, respectively, overridable
with `FREERANGE_TEMPO_PORT` / `FREERANGE_GRAFANA_PORT`). Grafana 3002 avoids the
existing application UI on 3001. Use an approved SSH tunnel/reverse proxy for
operator access; configure Grafana's Tempo source as `http://tempo:3200`. Tempo's
existing 24-hour local retention and persistent volumes remain in effect. This
single-host setup has no trace-storage HA.

Supported Compose configurations (Grafana password override is optional):

1. **Root base + overlay:** `docker compose -p "$PROJECT" -f docker-compose.yml
   -f docker-compose.otel.yml config --quiet`. Uses root-relative config paths.
2. **Prod base + overlay:** `bash prod/deploy.sh --check` supplies trace defaults,
   preserves the default process mode, and uses absolute paths. Direct Compose
   validation is also supported:
   `FREERANGE_OTEL_CONFIG_DIR="$REPO/config" docker compose -p "$PROJECT"
   -f prod/docker-compose.yaml -f docker-compose.otel.yml config --quiet`.
3. **Deploy base + overlay:** export `FREERANGE_OTEL_CONFIG_DIR="$REPO/config"`,
   supply its required image/secret environment settings, and validate with
   `docker compose -p "$PROJECT" -f deploy/docker-compose.prod.yml
   -f docker-compose.otel.yml config --quiet`.
4. **New isolated standalone tracing project:**
   `docker compose -p "$PROJECT" -f docker-compose.otel.yml config --quiet`.
   The default network is project-managed (`<project>_freerange-network`) and
   explicitly declares `driver: bridge`, matching the supported base declarations.
   This mode creates a new network; it cannot receive application OTLP until the
   applications are intentionally attached. Do not use it to adopt an existing
   application network based only on matching names: Compose also reconciles
   network declarations/hashes, including options set by operator overlays.
5. **Standalone tracing attached to an existing application network, including
   the same Compose project:** append an operator-owned external-network override
   with the inspected existing network name. Matching project names alone do not
   authorize network reconciliation or prove compatibility:

   ```yaml
   networks:
     freerange-network:
       driver: !reset null
       external: true
       name: "${FREERANGE_EXISTING_NETWORK:?Set the inspected application network name}"
   ```

   Validate using `-p "$TRACING_PROJECT" -f docker-compose.otel.yml
   -f /absolute/path/tracing-network.yml config --quiet`. Use the existing project
   name when preserving its tracing volume namespace, or an intentional separate
   tracing project when new volumes are appropriate. `!reset` removes the bridge
   driver because external network declarations must not configure a driver.
   Use a Compose version supporting `!reset`; verify the resolved network is
   external with the inspected name and no driver (without exposing secrets).
   This override is for standalone tracing attachment only. Keep merged base
   deployments on their existing managed network. Never use standalone `down` or
   orphan removal to manage the application network. Fixed container names permit
   only one tracing instance on a host; do not run merged and standalone copies.

Use `config --quiet` on real environments; full resolved Compose/`--environment`
output can expose secrets. Offline tests use synthetic values and skip service
env-file resolution. Schema validation alone does not prove that mounts exist,
network access works, images start, or data exports successfully.

Before startup on an authorized Docker host, validate the collector with its
actual pinned binary (this requires Docker daemon/image access):

```bash
docker run --rm --network none --read-only \
  --mount "type=bind,source=$REPO/config/otel-collector.yaml,target=/etc/otelcol/config.yaml,readonly" \
  otel/opentelemetry-collector-contrib:0.130.0 validate --config=/etc/otelcol/config.yaml
```

For the `prod` deployment, independent recovery is targeted:

```bash
bash prod/deploy.sh --check tempo otel-collector grafana
# After authorization; dependencies are explicitly selected because --no-deps:
bash prod/deploy.sh tempo otel-collector grafana
dc ps tempo otel-collector grafana
dc logs --tail 100 otel-collector tempo
curl --fail --max-time 5 http://127.0.0.1:3200/ready
```

Bound-mounted collector config content changes do not change Compose's service
hash. For later config-only changes, validate the binary config, then explicitly
`dc restart otel-collector`; reconcile with `up` when mount/image/service settings
change. Restarting the collector loses its in-memory buffered traces. Logs should
show startup/export status, not detailed payloads. Sanitize incidental error
details before sharing. Enable/reconcile API and worker only in the authorized
application canary; tracing service recovery alone does not change their env.
Use a harmless approved health request to verify API traces, then use the approved
delivery canary to verify worker linkage, environment tags and Tempo/Grafana
visibility. Missing sampled traces are not proof that no delivery occurred.

## Canary gates, accounting, and selective retry

**Patch 1:** run the approved local provider/fault tests first. Once DNS/TCP/TLS
and account ownership are verified, use explicitly authorized recipients for
one custom-SMTP app, one system-default app, and an explicitly configured fallback
chain. Verify the delivered sender and selected host/source, preserved MIME and
attachments, failure stage/category, bounded transient retry, terminal auth
handling, and released credit holds after failed delivery. An unconfigured
fallback must not silently route around BYOC authentication failure.

**Patch 2:** run the **synthetic fixture** with wallet 1500 and SMS unit cost 800:
one recipient estimates/reserves 800, two known recipients estimate 1600 and
cannot both reserve against that wallet. Verify advisory UI estimates, the exact
attempt-time credit observation, contention versus true depletion, daily-cap
handling, and successful release. Do not modify a real wallet to create this
fixture. Later wallet values cannot reconstruct the earlier failure snapshot.

**Patch 3:** require real ES/Redis fault tests, reviewed cutover, journal mapping,
and all subscription writers audited before canary. Stop admissions/pause worker
processing as reviewed, drain/import or quarantine legacy holds, preserve balances,
and do not mix legacy and journal lifecycle writers for the same tenant. Verify
crashes/outages before and after ES/Redis/ledger writes, renewal, missed reaper,
more-than-24-hour expiry, and success-plus-accounting-failure without a resend.
Deterministic ledger IDs or Redis locks alone do not establish crash safety.

Observe baseline versus canary queue depth/oldest age, retry/DLQ growth, reserved
credit age and orphan holds, delivery errors by category/provider/source, ledger
repair backlog/oldest age, subscription document size/journal growth, and tracing
queue/refusal/export errors and memory. Use existing authorized read-only metrics
or redacted operator queries; do not introduce balance-reset or replay commands.
Set incident-specific stop thresholds and an observation window before rollout.
Stop expansion for incorrect sender, repeated terminal auth retries, lost holds,
unexpected delta, duplicate delivery, rising repair backlog or sustained telemetry
failure. Record evidence separately for each gate; do not mark unrun gates passed.

Rollback patch 1/2 to recorded image digests with only the affected services after
reviewing compatibility. Patch 3 rollback first stops new admissions, preserves
the durable recovery path, drains journal-backed reservations, and only then
restores old writers. Never clear journals/counters or remove data volumes to
make rollback appear healthy. Keep OTel running independently for diagnosis.

Retry only an explicitly approved list of still-valid business notifications.
Check tenant/recipient, business intent, validity/deadline, delivery/idempotency
state, corrected failure cause, and reservation/accounting state before selection.
Exclude expired OTPs and already-delivered rows, including deliveries whose
accounting needs repair. No blanket DLQ replay, bulk reset, or accounting-induced
resend. Record the selection and owner; Task 6 performs no replay.

## Local verification and references

`python -m unittest discover -s tests/operations -v` requires PyYAML, Docker
Compose CLI, and Bash for deployment-script checks. It exercises the real Compose
parser with synthetic env values, validates standalone/merged/external models,
and runs the script against a Docker boundary stub for pull/up/ps. No containers,
provider messages, account operations, or DB writes are performed. Bash syntax:
`bash -n prod/deploy.sh`. Consult the companion operations report for the actual
results and unavailable runtime checks from this implementation session.

References: [Compose merging and base-relative paths](https://docs.docker.com/compose/how-tos/multiple-compose-files/merge/),
[Compose reset-tag semantics](https://docs.docker.com/reference/compose-file/merge/#reset-value),
[Docker networking and DNS](https://docs.docker.com/engine/network/),
[collector 0.130.0 memory limiter](https://github.com/open-telemetry/opentelemetry-collector/blob/v0.130.0/processor/memorylimiterprocessor/README.md),
[collector 0.130.0 batching](https://github.com/open-telemetry/opentelemetry-collector/blob/v0.130.0/processor/batchprocessor/README.md),
[collector 0.130.0 bounded exporter queue/retry](https://github.com/open-telemetry/opentelemetry-collector/blob/v0.130.0/exporter/exporterhelper/README.md).

## Billing journal cutover and rollback gates (Tasks 3 + 5)

The admission flag is **disabled by default** in `internal/config/config.go`:

```yaml
billing:
  enforce_credit_checks: true
  reservation_journal_enabled: false
```

The corresponding environment variable is
`FREERANGE_BILLING_RESERVATION_JOURNAL_ENABLED=false`. Use `true` only in the
reviewed cutover. The flag is process-wide; it does not itself select tenants.
API/worker instances that admit work for one tenant must agree on its lifecycle
writer. A tenant canary requires routing that actually isolates its writers;
otherwise keep the flag off until the approved fleet cutover. Confirm the effective
configuration on every admitting instance before resuming its queue.

Before enabling, pause admissions and old worker processing for the affected
scope. Record subscription IDs, total/remaining/reserved counters, outstanding
reservation IDs, original subscription bindings, and notification delivery states
through authorized read-only inspection. Drain known old reservations with their
existing lifecycle, or use a separately reviewed paused-worker import that proves
receipt identity and the matched sum. This patch contains no automatic importer.
The sum of pending journal receipts must equal the subscription reserved counter
before new journal admission. A mismatch is quarantined unchanged and blocks
admission; it is not permission to zero, replenish, or guess a balance. Missing
Redis entries are not evidence that a hold is safe to clear.

Verify acknowledged `enabled:false` object mappings for
`credit_reservation_journal` and `credit_allocation_journal` before journal writes.
The repository installs these on existing indices and fails on an incompatible
mapping. A previously dynamically indexed journal needs a separately reviewed
migration/reindex. Do not work around that error by enabling dynamic ID fields.

Run the opt-in tests only against disposable services:

```powershell
$env:FRN_CREDIT_INTEGRATION = '1'
$env:FRN_CREDIT_ES_URL = 'http://127.0.0.1:19200'
$env:FRN_CREDIT_REDIS_ADDR = '127.0.0.1:16379'
go test ./tests/integration -run 'TestCreditJournal|TestDeliveryHistoryRealES' -count=1 -v
```

Explicit opt-in with missing/unavailable services fails the fixture. An unrequested
fixture run skips clearly. Each fixture uses its own physical ES index namespace
and generated Redis IDs; no production wallet/rate-card mutation is a test setup.
Run billing race tests on supported CI with cgo and a C compiler; the local Windows
`CGO_ENABLED=0` environment has no GCC and cannot provide race evidence.

Canary checks must establish: atomic rejection snapshots and correct contention
classification; one terminal delta under duplicate/competing outcomes; repair of
lost ES/Redis/ledger responses; old/new purchase, payment, and admin-renewal HTTP
contracts; original-subscription binding; pagination beyond 100 subscription
documents; recovery after more than 24 hours without Redis; and successful send
plus accounting outage with no provider resend. Purchase and admin-renewal grants
add to the current unexpired wallet atomically and preserve active holds.
Replacement allocations from older internal adapters still require draining holds.
Generic metadata updates cannot overwrite journal counters. Atomic grant/bootstrap
is the credit-writer API; uncertain manual grants still require operator
reconciliation before retrying because they lack a durable grant operation ID.

The durable per-subscription boundary is `credit_reservation_mode: journal`.
The repository installs this marker before the first reservation and, with the
flag enabled, before full subscription updates/allocations; newly created
subscriptions also carry it. Marking preserves every counter, including unmatched
old holds. Verify the marker on the canary before admitting work. Protection is
active even when the reservation map is empty. An absent marker retains old-mode
full-update behavior; disabling admissions never removes an existing marker.

Captured payments use the stored order snapshot, including credits, amount,
currency and price version. Paid customers can recharge without replacing their
remaining credits: 35,000 purchased plus 400 unexpired credits leaves 35,400.
Active reservations survive recharge, and recharge never shortens expiry. Retain
each checkout order snapshot so a newer checkout does not change an older order.
Razorpay callback verification checks capture status, amount and currency against
the gateway; signed capture webhooks are checked against the stored snapshot.
Do not acknowledge capture until allocation is durable. Replay receipts keyed by
payment and order prevent a repeated capture from granting credits again.

Admin renewal accepts the existing request without a header. An optional
`Idempotency-Key` (maximum 200 characters) supplies a stable renewal identity:
reuse it for retries of the same intent, and use a new key for a new renewal.
A keyed lost response can be retried after an intervening burn without refilling
again in either old or journal mode. Renewal adds credits to the usable wallet.
Old-mode keyed allocation receipts survive
later journal adoption and do not by themselves mark the wallet journal-backed.
Unkeyed requests retain the old renewal semantics and have no guarantee
against an ambiguous rebuilt retry; inspect allocation/cycle state before a
manual retry. No new CLI/header requirement is imposed by this rollout.

Keep recovery running after admission is stopped. Set
`reservation_journal_enabled: false` while retaining `enforce_credit_checks: true`
and the new recovery-capable binary/wiring. Durable `cr1.` IDs are still committed,
released, and repaired with admission off. A journal-backed wallet rejects old
lifecycle admissions/writes, rather than silently mixing them. The authoritative
reaper scans subscription source, including expired subscriptions; Redis TTL is
only terminal cache retention: pending records have no Redis TTL, and terminal
records expire after 24 hours and leave the expiry index. Missing legacy records
are reported for audited reconciliation; absence does not authorize a refund.
Legacy recovery with notification storage wired commits a bound successful
delivery, releases a definitively failed attempt, and retains unknown outcomes.
Terminal ledger failures are repaired without a second
balance delta. Sent/delivered/read evidence can commit an expired pending hold
only when `metadata.delivery_reservation_id` equals that exact reservation ID.
The worker writes successful status, timestamp and this binding together in one
notification update. A later successful attempt cannot charge an earlier hold.
`RecordReservationDeliveryFailure(ctx, reservationID)` retains definitive failed
attempt evidence in its receipt before release. Recovery can refund that hold
after a cleanup outage even if a later attempt succeeded. Do not mark ambiguous
network timeouts as definitive failure. Missing/erroring delivery source and
unproven outcomes stay held with surfaced errors; a metadata mismatch alone is
neither permission to charge nor to refund. Reconcile quarantined attempts using
authorized delivery/provider evidence, without resending or resetting balances.

Malformed receipt sources quarantine their subscription and surface errors while
independent valid subscriptions and later pages continue recovery. Review each
quarantine separately. A damaged source is not evidence of absent reservations.

For rollback, first stop admissions, preserve this recovery path, and drain/repair
journal-backed reservations. Check pending holds, ledger acknowledgements,
delivery reconciliation, exact balances, and repair backlog before selecting any
older writer. Restoring old writers requires an explicit reviewed migration of
retained history; disabling the flag alone does not authorize it. Never delete
receipts, clear counters, or run a blanket ops reset to bypass that gate.
Terminal reservation/allocation receipts remain retained; no replay-safe pruning
policy has been approved. Track `journal_entries` and `expired_released` recovery
logs together with document sizes and oldest pending/unrepaired receipt age, and
stop expansion if growth or accounting invariants breach the agreed thresholds.

## Customer billing and throughput rollout

Recharge carries unexpired spendable credits and preserves outstanding holds.
Expired spendable credits do not carry. `credits_total` includes cumulative grants
while credits carry; consumption is total minus remaining. The usage response
exposes reserved and available separately. A hold with an unknown delivery outcome
stays reserved until evidence permits settlement. Do not clear all reserved credits
or reduce a customer's balance to match a historically divergent ledger. Record
the approved balance correction and its reason through the audited recovery path.
These code changes do not repair historical production balances automatically.

Billing continues to charge an application's existing admin wallet. An organization
`tenant_id` is also an authorization identity; it does not redirect credits already
purchased into a personal wallet. Usage events record the actual charged wallet.
Price edits create immutable versions; notification reservations retain the price
they were created with. The runtime active pointer determines the current card,
including disabled checkout plans. Existing historical attribution and rate-card
data require a separately reviewed backfill.

A subscriber CSV can contain up to 10,000 records and 10 MiB. The dashboard parses
quoted fields, validates before writing, and submits sequential chunks of at most
500 records and 1 MiB under the unchanged 1,000-record public API cap. Progress and
errors refer to original CSV record numbers. A transport failure stops subsequent
chunks because the current chunk may already have been committed; inspect its
results before resubmitting. Successful rows are never automatically replayed.

Broadcast preparation reuses loaded profiles and persists queue-ready notifications
in bounded batches before publication. An ambiguous publication response stays
queued for operational review; automatic republication could duplicate delivery.
The default is 20 workers; explicitly configured `queue.workers` values remain in
effect. Tune concurrency to the SMTP provider's connection and send limits. Local
fixtures with a 2 ms provider measured about 2.98 / 1.54 / 0.77 seconds for 6,001
notifications at 5 / 10 / 20 workers. These are scheduling measurements, not a live
SMTP delivery guarantee. No provider throttle can be bypassed by adding workers.

Request quotas are separate from delivery throughput. Enable them after verifying
Redis availability, reverse-proxy client-IP behavior and a customer canary:

```yaml
security:
  app_rate_limit_enabled: true
  rate_limit_enabled: true
```

Equivalent environment overrides are
`FREERANGE_SECURITY_APP_RATE_LIMIT_ENABLED=true` and
`FREERANGE_SECURITY_RATE_LIMIT_ENABLED=true`. Both default to false for a compatible
rollout. The first enforces each application's configured requests/hour across its
API keys and environments on app-scoped routes. Limits of zero or less retain the
existing unlimited interpretation. One broadcast is one HTTP admission regardless
of recipient count; each CSV chunk is one admission. Authentication/authorization
runs first so invalid credentials and forbidden writes do not spend the app budget.
The second applies `security.rate_limit` per `security.rate_limit_window` seconds to
public authentication routes by client IP. Health checks and signed payment callbacks
are outside this bucket. The existing ops quota remains independently configured.

Denied requests return `429`, `Retry-After` and `X-RateLimit-*` headers without
extending the window; browser CORS exposes these headers. Redis failure returns
`503` when enforcement is enabled. Settings display the actual deployment state.
Verify enabled/disabled settings, shared keys, expiry, concurrent admissions and
callback delivery before expanding the canary. Use a coordinated binary rollout:
the new limiter converts old nanosecond scores to milliseconds in Redis while
retaining their admission and expiry history.

## Delivery and broadcast uncertainty

The worker persists `metadata.delivery_attempt_state: dispatching` before calling
a provider. A lost acknowledgement, send timeout, or crash leaves that attempt
quarantined as `processing`; recovery does not resend it or automatically refund
its credits. `unknown` makes that quarantine explicit when the worker can persist
the result. Reconcile the exact reservation and provider evidence before marking
an outcome. A timeout after an OTP deadline does not prove the message was rejected.
Definitive rejection evidence permits normal refund/retry. SMTP connection/auth/
recipient failures occur before acceptance, and explicit 4xx/5xx replies prove
rejection; loss of the final DATA reply does not. SMTP retries and provider fallback
both stop on an unknown outcome. Successful DATA acceptance remains successful even
when later connection cleanup fails.

Compatible credit wallets now retain `credit_legacy_settlement_journal` as another
source-only object mapping. Its reservation identity and terminal wallet delta are
written atomically, even with journal admissions disabled. A lost Redis terminal
write can replay the receipt without debiting again or consuming a sibling hold.
The deterministic ledger write precedes terminal-cache removal from recovery. This
does not reconstruct missing pre-deployment settlement evidence. Keep historical
unknown reservations in the audited recovery process. Retain these settlement
receipts until a replay-safe pruning policy is approved; monitor document growth.

Scheduled publication claims each eligible pending/snoozed notification atomically
before enqueueing. Redis-ready and Elasticsearch fallback schedulers share that
claim; stale search results cannot republish it or overwrite a completed delivery.
An ambiguous claim/publication response stays queued for reconciliation. Custom
repository adapters must implement the optional atomic claim capability before
scheduled dispatch; the shipped Elasticsearch repository implements it.

Broadcast replies preserve `sent` and `total` and add rejected, skipped, unknown,
not-attempted counts and an operation ID. Partial acceptance returns `202` with
`partial:true` and `retryable:false`; it is not permission to repeat the full fan-out.
Recipient outcomes are capped at 1,000 entries with an explicit truncation flag.
Notifications retain `metadata.broadcast_operation_id` for source-level auditing.
Use a stable `Idempotency-Key` for retries of the same intent. An atomic receipt is
claimed before side effects, including concurrent requests. Interrupted/pending
receipts have no TTL and return `409`; reconcile them instead of deleting the claim.
Completed responses retain the existing 24-hour idempotency window. A different
request with the same key returns `409`. Without a key, a fresh request is a new
intent and cannot be deduplicated. The dashboard preserves its key and form after
partial/unknown responses and explains the partial result; keep the operation/key
when investigating across a page reload. CORS permits both idempotency headers.
Forbidden role-specific app operations are checked before spending shared quota.
Broadcast receipts retain the existing application/key Redis namespace so older
readers see the pending/completed response. New request fingerprints reject reuse
for another intent or environment. Complete the API fleet rollout before enabling
new clients; an older request already in flight before a claim cannot be locked
retroactively. Quarantined delivery history explicitly reports `retryable:false`.

## WhatsApp/SMS Phase 2 — local connections and onboarding

Phase 2 adds `channel_connections`, redacted JWT/app-authorized connection reads,
CAS updates, retained encrypted revisions, provider validation and scoped authoring.
A connection update resets readiness to pending. Disconnect creates an explicit
block against legacy/global fallback. The dashboard preserves its selected
messaging environment across navigation and reload, refreshes parent settings after
connection changes, and separates local draft save/edit from provider submission.
Local draft removal leaves the provider resource intact. Imported providers expose
only the specifically linked resources and report origin/action permissions.

### Rollout gate and key rotation

`FREERANGE_CONNECTION_WRITES_ENABLED` defaults to false. Keep it false until Phase 3
has wired delivery admission, workers and charged replies to immutable connection
snapshots and **all** API/worker readers have been upgraded. Phase 2 alone is not a
production enablement checkpoint. No real environment values are changed by this
implementation.

Set `FREERANGE_CONNECTION_ENCRYPTION_KEYS` through the deployment secret manager.
Its format is an ordered comma-separated `key-id:base64-32-byte-key` keyring. The
first key encrypts; retained IDs decrypt old revisions. Keep old keys while queued
sends, bindings and historical revisions still reference them. AES-256-GCM binds
connection/app/environment/provider/field identity. Do not rename a key ID during
rotation or remove a decrypt key while it remains in use.

### Scoped, additive migration

`frn channel-connections --app <app-id>` is a dry run. It emits only the application,
channel, action and revision, and sends no provider requests. After reader rollout,
a maintenance window and provisioning the keyring, operators can add `--apply
--readers-upgraded` with encrypted writes enabled. Pause connection settings writes
and sending during apply/validation; resume only after readiness checks. Each
created connection is its own checkpoint: rerunning skips explicit connections,
including customer edits, rather than overwriting them. Legacy plaintext settings
are retained for compatible recovery; migration never deletes them. No customer
migration was executed during local development.

Manual Meta onboarding needs the customer's token, WABA, selected phone and Meta
app signing secret; it does not need platform Facebook SDK keys. Empty/mismatched
phone lists are rejected. The connection panel can discover a paginated owned phone
list by saving the account/token and validating before selecting the sender.
Embedded Signup does not seed customer business assets from frontend environment
values. Saving credentials is distinct from confirming provider readiness.

Validation uses authenticated read requests, never a paid test message. It checks
account access, sender ownership/registration, token permissions and callback
subscription. Meta callback URL/fields are checked with the customer's app context;
unknown readiness remains pending. Billing state that the provider does not expose
is labelled `not_exposed`; operators still need provider billing/compliance setup.
Twilio validation uses the current v2 WhatsApp Senders API, including the required
channel filter and trusted pagination. References: [Twilio Senders API](https://www.twilio.com/docs/whatsapp/api/senders),
[Twilio phone-number capabilities](https://www.twilio.com/docs/phone-numbers/api/incomingphonenumber-resource).

Provider account database failures never borrow global credentials. Encrypted
Twilio template callbacks resolve their pinned connection revision and preserve
the owning environment in durable receipts. Meta template callbacks read explicit
connections before the legacy adapter. New unvalidated sender assignments do not
receive inbound traffic. Ambiguous vendor submission outcomes become
`submission_unknown` and require reconciliation before another submission.

### Decisions and limits

- Keep the existing local checkout/branch for phase monitoring. There is no extra
  worktree isolation; unrelated existing edits are excluded from task commits.
- Deny vendor template mutations on shared accounts until an explicit ownership
  grant exists. Historical ownership cannot safely be guessed. Shared-account
  users need a scoped grant before those mutations can resume.
- Leave encrypted writers disabled until Phase 3 and the fleet reader rollout.
  Enabling early would leave legacy delivery credentials in use. Existing tariffs,
  historical balances and production credentials are untouched.

### Local verification

Task 4 encryption, legacy/BYOC compatibility, environment precedence, revision
conflicts, immutable old revisions and redacted/resumable migration tests passed.
Disposable Elasticsearch verified concurrent connection updates retain one winner.
Phase 2 browser coverage includes connect/save, disconnect/save, environment
persistence and local-save versus explicit-submission messages. Final suite and
review evidence is recorded with the Phase 2 checkpoint in the implementation plan.

### Phase 2 independent review corrections

One independent whole-branch review identified two critical and six important
issues. All entered a single regression-driven fix pass: shared global Meta and
legacy dual-provider Twilio mutations now require account ownership checks;
legacy onboarding calls the real settings CAS signature and preserves conflicts;
revision-zero legacy drafts do not claim an encrypted historical record; rich Sync
uses its pinned encrypted connection; inbox lists/unread/read markers respect the
selected environment; cached vendor actions disappear while a changed connection
loads; and Embedded Signup carries and validates the SDK-selected WABA and phone.

The review deliberately left worker/queued-price/rotation dispatch integration,
charged replies, OTP and full CSW dispatch to Phase 3, and detailed tariffs/BYOC
fees, Exotel/DLT and runtime config cleanup to Phase 4. Encrypted writers must
remain off while those consumers are incomplete. Historical template owner grants
remain deferred; shared-account vendor writes are denied meanwhile. Production
migration, fleet rollout, approvals, commercial-rate verification and live canaries
remain operator work, so local fixtures do not prove live account readiness.
Previously copied-key rotation and historical data repairs remain separate
operator actions: this work changes no production credentials or balances. Existing
unrelated local files remain excluded and unreviewed. No minor review findings were
deferred.

Final Phase 2 verification passed after the review fix pass: full Go short suite;
three disposable Elasticsearch/Redis integration suites (connections/drafts,
callback recovery, scoped inbox); UI production build; 6 channel UI cases and 43
existing reliability UI cases. The build retains the pre-existing large-chunk
warning. Migration CLI help and the pre-database apply guard passed. No live
provider accounts, paid messages, customer migrations, push or deployment were
used. A final added-line credential scan found no high-confidence secrets.
# Phase 3 local delivery integration

Task 6 binds WhatsApp/SMS to a safe admitted connection reference before enqueue and credit admission. Retained encrypted revisions remain readable; legacy queued sends reject changed account/sender identities rather than selecting a replacement account. All free-form dispatches require the environment/account/sender-scoped customer-service window. Twilio inbound callbacks fetch `date_created` from the signed account's Message resource and verify account, SID and both endpoints before opening a window. Malformed/empty acceptance responses remain unknown outcomes, without synthetic IDs or fallback.

Local tests use mock HTTP endpoints and disposable loopback-only ES/Redis. No production writes, live sends, tariff activation or actual environment edits. Quote/accounting/reply/OTP integration remains the next two Phase 3 tasks. Encrypted writers remain gated until the entire API/worker fleet can read the new records.

Provider references checked 2026-10-14: [Twilio Message resource](https://www.twilio.com/docs/messaging/api/message-resource), [inbound webhook timestamps](https://help.twilio.com/articles/46569585455003), [SMS segment limits](https://www.twilio.com/docs/glossary/what-sms-character-limit).

Task 7 adds immutable quotes and exactly-once journal settlement. Queued content, destination, connection identity, units and tariff version must agree before dispatch. Flat cards retain their per-message fees and legacy phone compatibility; detailed cards require explicit activation with dated provider/account references, verified destination-prefix coverage, currencies/taxes and agreed BYOC fees. No fixture amount is a proposed live price. Country labels come from verified operator rate dimensions; routes with no applicable detailed price fail clearly. Do not activate detailed pricing before journal admission and the complete quote-reader fleet are ready.

GSM-7 counts extension characters as two septets; UCS-2 counts UTF-16 units, including emoji surrogate pairs. Provider acceptance settles FRN processing charges. Estimated vendor costs are separate; delivery callbacks do not retrospectively change a charge. Unknown outcomes retain holds. Ledger acknowledgment follows both immutable ledger and deterministic usage persistence, so recovery repairs accounting without another provider send. A notification-level atomic claim prevents duplicate queue entries from both crossing the provider boundary. Pre-upgrade reservation/ledger formats remain readable.

Client-supplied accounting metadata is reserved for server use. Price/free hints in payload data are advisory and cannot lower the server quote. Older integrations can keep sending their existing payload shapes. Local Task 7 checks passed: Go short suite across all repository roots, quote/segment/category tests, disposable ES/Redis concurrent admission/settlement/claim/legacy-repair cases, and UI production build (existing chunk-size warning). No active tariff or customer balance was changed.

