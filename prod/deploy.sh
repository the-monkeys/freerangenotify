#!/usr/bin/env bash
set -euo pipefail

# Usage: ./deploy.sh [--check] [SERVICE ...]
# --check validates without printing resolved secrets or deploying.
# Named services are reconciled with --no-deps; no blanket recreation.
DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$DIR/.." && pwd)"
cd "$DIR"

CHECK_ONLY=false
if [[ "${1:-}" == --check ]]; then
  CHECK_ONLY=true
  shift
fi
SERVICES=("$@")
for svc in "${SERVICES[@]}"; do
  if [[ ! "$svc" =~ ^[a-zA-Z0-9][a-zA-Z0-9_.-]*$ ]]; then
    echo "ERROR: expected a service name, got an option or invalid name" >&2
    exit 1
  fi
done

if [[ -f docker-compose.yaml ]]; then
  COMPOSE_FILE="$DIR/docker-compose.yaml"
elif [[ -f docker-compose.yml ]]; then
  COMPOSE_FILE="$DIR/docker-compose.yml"
else
  echo "ERROR: no docker-compose.yaml/.yml found in $DIR" >&2
  exit 1
fi
COMPOSE_ARGS=(--project-directory "$DIR" -f "$COMPOSE_FILE")

# Keep prod/.env and project identity; Compose rebases all overlay paths to prod.
OTEL_ENABLED=false
case "${FREERANGE_ENABLE_OTEL:-1}" in
  1)
    if [[ -n "${OTEL_COMPOSE_FILE:-}" ]]; then
      OTEL_FILE="$OTEL_COMPOSE_FILE"
    elif [[ -f "$DIR/docker-compose.otel.yml" ]]; then
      OTEL_FILE="$DIR/docker-compose.otel.yml"
    elif [[ -f "$DIR/docker-compose.otel.yaml" ]]; then
      OTEL_FILE="$DIR/docker-compose.otel.yaml"
    else
      OTEL_FILE="$ROOT/docker-compose.otel.yml"
    fi
    if [[ ! -f "$OTEL_FILE" ]]; then
      echo "ERROR: OTel overlay missing; set FREERANGE_ENABLE_OTEL=0 to deploy without tracing" >&2
      exit 1
    fi
    export FREERANGE_OTEL_CONFIG_DIR="${FREERANGE_OTEL_CONFIG_DIR:-$ROOT/config}"
    for cfg in otel-collector.yaml tempo.yaml grafana-datasources.yaml; do
      if [[ ! -f "$FREERANGE_OTEL_CONFIG_DIR/$cfg" ]]; then
        echo "ERROR: missing OTel config file: $cfg" >&2
        exit 1
      fi
    done
    COMPOSE_ARGS+=(-f "$OTEL_FILE")
    OTEL_ENABLED=true
    ;;
  0) ;;
  *) echo "ERROR: FREERANGE_ENABLE_OTEL must be 0 or 1" >&2; exit 1 ;;
esac

# config.Load reads ./config/config.yaml with FREERANGE_* env overrides.
# config.prod.yaml is not automatically loaded and has unresolved placeholders.
# Keep the shipped prod base's staging process mode: app.environment=production
# currently enables prefork. Production trace tags are independent of that mode.
# Explicit app env overrides also default the trace tag unless OTEL_ENV is set.
DEFAULTS_FILE="$(mktemp "${TMPDIR:-/tmp}/frn-production.XXXXXX.yaml")"
trap 'rm -f -- "$DEFAULTS_FILE"' EXIT
cat > "$DEFAULTS_FILE" <<'YAML'
services:
  notification-service:
    environment: &production_environment
      FREERANGE_APP_ENVIRONMENT: "${FREERANGE_APP_ENVIRONMENT:-staging}"
      FREERANGE_APP_DEBUG: "false"
      DEBUG: "false"
      FREERANGE_APP_LOG_LEVEL: "${FREERANGE_APP_LOG_LEVEL:-info}"
      FREERANGE_OTEL_ENV: "${FREERANGE_OTEL_ENV:-${FREERANGE_APP_ENVIRONMENT:-production}}"
      FREERANGE_OTEL_EXPORTER_OTLP_ENDPOINT: "${FREERANGE_OTEL_EXPORTER_OTLP_ENDPOINT:-otel-collector:4317}"
      FREERANGE_OTEL_SAMPLE_RATIO: "${FREERANGE_OTEL_SAMPLE_RATIO:-0.05}"
YAML
if [[ "$OTEL_ENABLED" == true ]]; then
  printf '%s\n' '      FREERANGE_OTEL_ENABLED: "${FREERANGE_OTEL_ENABLED:-true}"' >> "$DEFAULTS_FILE"
else
  printf '%s\n' '      FREERANGE_OTEL_ENABLED: "false"' >> "$DEFAULTS_FILE"
fi
cat >> "$DEFAULTS_FILE" <<'YAML'
  notification-worker:
    environment: *production_environment
YAML
COMPOSE_ARGS+=(-f "$DEFAULTS_FILE")

# Operator-owned overlays (e.g. evidence-based DNS) are deliberately opt-in.
if [[ -n "${DEPLOY_OVERRIDE_FILE:-}" ]]; then
  if [[ ! -f "$DEPLOY_OVERRIDE_FILE" ]]; then
    echo "ERROR: DEPLOY_OVERRIDE_FILE does not exist" >&2
    exit 1
  fi
  COMPOSE_ARGS+=(-f "$DEPLOY_OVERRIDE_FILE")
fi

PULL_RETRIES="${PULL_RETRIES:-5}"
PULL_BACKOFF="${PULL_BACKOFF:-10}"
if [[ ! "$PULL_RETRIES" =~ ^[1-9][0-9]*$ || ! "$PULL_BACKOFF" =~ ^[0-9]+$ ]]; then
  echo "ERROR: PULL_RETRIES must be positive and PULL_BACKOFF nonnegative integers" >&2
  exit 1
fi

compose() { docker compose "${COMPOSE_ARGS[@]}" "$@"; }
echo "==> Validating Compose configuration..."
compose config --quiet
AVAILABLE_SERVICES="$(compose config --services)"
for svc in "${SERVICES[@]}"; do
  if ! grep -Fxq -- "$svc" <<< "$AVAILABLE_SERVICES"; then
    echo "ERROR: unknown service: $svc" >&2
    exit 1
  fi
done
if [[ "$CHECK_ONLY" == true ]]; then
  echo "==> Configuration valid; no deployment performed."
  exit 0
fi

pull_parallel() { compose pull "${SERVICES[@]}"; }
pull_sequential() {
  local svc attempt
  local targets=("${SERVICES[@]}")
  if (( ${#targets[@]} == 0 )); then
    mapfile -t targets <<< "$AVAILABLE_SERVICES"
  fi
  for svc in "${targets[@]}"; do
    echo "  -> $svc"
    attempt=1
    until compose pull "$svc"; do
      if (( attempt >= PULL_RETRIES )); then
        echo "ERROR: failed to pull $svc after ${PULL_RETRIES} attempts" >&2
        return 1
      fi
      sleep "$PULL_BACKOFF"
      attempt=$((attempt + 1))
    done
  done
}

echo "==> Pulling images..."
attempt=1
until pull_parallel; do
  if (( attempt >= PULL_RETRIES )); then
    echo "WARN: parallel pull failed; falling back to bounded sequential pulls..." >&2
    pull_sequential
    break
  fi
  echo "WARN: pull failed; retrying in ${PULL_BACKOFF}s..." >&2
  sleep "$PULL_BACKOFF"
  attempt=$((attempt + 1))
done

echo "==> Reconciling containers..."
# Images were pulled above; prevent base pull_policy:always from bypassing retries.
UP_ARGS=(up -d --pull never)
if (( ${#SERVICES[@]} > 0 )); then
  UP_ARGS+=(--no-deps "${SERVICES[@]}")
fi
compose "${UP_ARGS[@]}"
echo "==> Current status:"
compose ps "${SERVICES[@]}"
echo "==> Done."
