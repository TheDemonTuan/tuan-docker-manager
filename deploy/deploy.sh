#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="${APP_DIR:-$SCRIPT_DIR}"
COMPOSE="$APP_DIR/compose.prod.yaml"
APP_ENV="$APP_DIR/.env"
DEPLOY_ENV="$APP_DIR/.deploy.env"
LOCK_FILE="$APP_DIR/.deploy.lock"
IMAGES_STATE="$APP_DIR/.deployed-images"
PREVIOUS_IMAGES_STATE="$APP_DIR/.previous-images"
READY_TIMEOUT="${READY_TIMEOUT:-120}"

log() { printf '%s  %s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$*"; }
die() { log "ERROR: $*" >&2; exit 1; }

# shellcheck source=deploy/image-retention.sh
source "$APP_DIR/image-retention.sh"

dc() {
  local env_args=(--env-file "$APP_ENV")
  if [[ -f "$DEPLOY_ENV" ]]; then
    env_args+=(--env-file "$DEPLOY_ENV")
  fi
  docker compose "${env_args[@]}" -f "$COMPOSE" "$@"
}

validate_digest() {
  local image="$1"
  [[ "$image" =~ ^ghcr\.io/[a-z0-9._/-]+@sha256:[a-f0-9]{64}$ ]] || \
    die "image must be an immutable GHCR digest: $image"
}

health_status() {
  local service="$1" container_id
  container_id="$(dc ps -q "$service" 2>/dev/null || true)"
  [[ -n "$container_id" ]] || { printf 'missing\n'; return; }
  docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' \
    "$container_id" 2>/dev/null || true
}

status() {
  printf '=== Docker Panel Deployment Status ===\n'
  printf 'Active images:\n%s\n' "$(cat "$IMAGES_STATE" 2>/dev/null || echo none)"
  printf 'Previous images:\n%s\n' "$(cat "$PREVIOUS_IMAGES_STATE" 2>/dev/null || echo none)"
  for service in panel-agent panel-core cloudflared; do
    printf '%s: %s\n' "$service" "$(health_status "$service")"
  done
  dc ps
}

if [[ "${1:-}" == "--status" ]]; then
  status
  exit 0
fi

[[ $# -eq 2 ]] || die "usage: $0 <core-image@sha256:digest> <agent-image@sha256:digest> | --status"
NEW_CORE="$1"
NEW_AGENT="$2"
validate_digest "$NEW_CORE"
validate_digest "$NEW_AGENT"

command -v docker >/dev/null || die "docker is required"
docker compose version >/dev/null || die "Docker Compose v2 is required"
command -v flock >/dev/null || die "flock is required"
[[ -f "$COMPOSE" ]] || die "$COMPOSE is missing"
[[ -f "$APP_ENV" ]] || die "$APP_ENV is missing"
[[ -s "$APP_DIR/.tunnel-token" ]] || die "$APP_DIR/.tunnel-token is missing or empty"
chmod 600 "$APP_ENV" "$APP_DIR/.tunnel-token"
mkdir -p "$APP_DIR"

exec 9>"$LOCK_FILE"
flock -n 9 || die "another docker-panel deployment is already running"

PREVIOUS_CONFIG="$(cat "$DEPLOY_ENV" 2>/dev/null || true)"
PREVIOUS_STATE="$(cat "$IMAGES_STATE" 2>/dev/null || true)"
DEPLOY_SUCCEEDED=false

rollback() {
  local exit_code=$?
  trap - ERR INT TERM HUP
  if [[ "$DEPLOY_SUCCEEDED" != true ]]; then
    log "Deployment failed; rolling back to the previous image pair."
    if [[ -n "$PREVIOUS_CONFIG" ]]; then
      printf '%s\n' "$PREVIOUS_CONFIG" > "$DEPLOY_ENV"
      dc up -d --no-deps panel-agent panel-core || true
      if [[ -s "$APP_DIR/.tunnel-token" ]]; then
        dc up -d --no-deps cloudflared || true
      fi
    else
      rm -f "$DEPLOY_ENV"
      log "No previous deployment state exists; inspect the containers manually."
    fi
  fi
  exit "$exit_code"
}
trap rollback ERR INT TERM HUP

log "Pulling immutable images."
docker pull "$NEW_CORE"
docker pull "$NEW_AGENT"

cat > "$DEPLOY_ENV" <<EOF
PANEL_CORE_IMAGE=$NEW_CORE
PANEL_AGENT_IMAGE=$NEW_AGENT
EOF
chmod 600 "$DEPLOY_ENV"

log "Starting production services and waiting for health checks."
dc config --quiet
dc up -d --remove-orphans --wait --wait-timeout "$READY_TIMEOUT" panel-init panel-agent panel-core

[[ "$(health_status panel-agent)" == "healthy" ]] || die "panel-agent failed its health check"
[[ "$(health_status panel-core)" == "healthy" ]] || die "panel-core failed its health check"

if [[ -s "$APP_DIR/.tunnel-token" ]]; then
  log "Starting cloudflared ingress tunnel."
  dc up -d --no-deps cloudflared || true
else
  log "Notice: $APP_DIR/.tunnel-token is empty. Add Cloudflare Tunnel token to start ingress."
fi

if [[ -n "$PREVIOUS_STATE" ]]; then
  printf '%s\n' "$PREVIOUS_STATE" > "$PREVIOUS_IMAGES_STATE"
fi
printf 'PANEL_CORE_IMAGE=%s\nPANEL_AGENT_IMAGE=%s\n' "$NEW_CORE" "$NEW_AGENT" > "$IMAGES_STATE"
chmod 600 "$IMAGES_STATE" "$PREVIOUS_IMAGES_STATE" 2>/dev/null || true
DEPLOY_SUCCEEDED=true
trap - ERR INT TERM HUP

log "Deployment succeeded."
status

previous_core="$(printf '%s\n' "$PREVIOUS_STATE" | sed -n 's/^PANEL_CORE_IMAGE=//p')"
previous_agent="$(printf '%s\n' "$PREVIOUS_STATE" | sed -n 's/^PANEL_AGENT_IMAGE=//p')"
prune_repository_images "${NEW_CORE%@sha256:*}" "$NEW_CORE" "$previous_core"
prune_repository_images "${NEW_AGENT%@sha256:*}" "$NEW_AGENT" "$previous_agent"
docker image prune -f >/dev/null 2>&1 || true
