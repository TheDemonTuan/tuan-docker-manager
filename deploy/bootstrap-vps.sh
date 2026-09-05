#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="${APP_DIR:-/opt/tuan-docker-manager}"
DEPLOY_USER="${SUDO_USER:-$(id -un)}"
STACKS_DIR="${STACKS_HOST_PATH:-/srv/docker-panel/stacks}"

[[ "$(id -u)" -eq 0 ]] || { echo "Run this script with sudo." >&2; exit 1; }
command -v docker >/dev/null || { echo "Docker Engine is required." >&2; exit 1; }
docker compose version >/dev/null || { echo "Docker Compose v2 is required." >&2; exit 1; }
command -v flock >/dev/null || { echo "flock is required." >&2; exit 1; }

install -d -o "$DEPLOY_USER" -g "$DEPLOY_USER" -m 0750 "$APP_DIR"
install -d -o "$DEPLOY_USER" -g "$DEPLOY_USER" -m 0750 "$STACKS_DIR"
install -o "$DEPLOY_USER" -g "$DEPLOY_USER" -m 0644 "$SCRIPT_DIR/compose.prod.yaml" "$APP_DIR/compose.prod.yaml"
install -o "$DEPLOY_USER" -g "$DEPLOY_USER" -m 0755 "$SCRIPT_DIR/deploy.sh" "$APP_DIR/deploy.sh"
install -o "$DEPLOY_USER" -g "$DEPLOY_USER" -m 0755 "$SCRIPT_DIR/image-retention.sh" "$APP_DIR/image-retention.sh"

if [[ ! -f "$APP_DIR/.env" ]]; then
  install -o "$DEPLOY_USER" -g "$DEPLOY_USER" -m 0600 "$SCRIPT_DIR/.env.production.example" "$APP_DIR/.env"
  echo "Created $APP_DIR/.env. Replace every placeholder before the first deployment."
fi
if [[ ! -f "$APP_DIR/.tunnel-token" ]]; then
  install -o "$DEPLOY_USER" -g "$DEPLOY_USER" -m 0644 /dev/null "$APP_DIR/.tunnel-token"
  echo "Created $APP_DIR/.tunnel-token. Put only the Cloudflare Tunnel token in this file."
fi

usermod -aG docker "$DEPLOY_USER" 2>/dev/null || true

cat <<EOF
VPS bootstrap complete.

Next:
1. Edit $APP_DIR/.env and replace ALLOWED_EMAILS and ADMIN_EMAIL.
2. Put only the Cloudflare Tunnel token in $APP_DIR/.tunnel-token and ensure the tunnel routes to http://panel-core:8080.
3. Log out and back in if $DEPLOY_USER was newly added to the docker group.
4. Add the GitHub Actions production secrets documented in deploy/README.md.
5. Push to main. The workflow will build immutable GHCR images and run $APP_DIR/deploy.sh.
EOF
