#!/usr/bin/env bash
set -euo pipefail

bash -n deploy/deploy.sh deploy/bootstrap-vps.sh deploy/image-retention.sh
grep -q 'flock -n' deploy/deploy.sh
grep -q 'source "$APP_DIR/image-retention.sh"' deploy/deploy.sh
grep -q 'prune_repository_images' deploy/deploy.sh
grep -q 'sha256:\[a-f0-9\]{64}' deploy/deploy.sh
grep -q 'Deployment failed; rolling back' deploy/deploy.sh
grep -q -- '--wait-timeout' deploy/deploy.sh
grep -q 'trap rollback ERR INT TERM HUP' deploy/deploy.sh
grep -q 'docker image prune -f' deploy/deploy.sh
grep -q 'PANEL_CORE_IMAGE' deploy/compose.prod.yaml
grep -q 'PANEL_AGENT_IMAGE' deploy/compose.prod.yaml
grep -q '/var/run/docker.sock:/var/run/docker.sock' deploy/compose.prod.yaml
if grep -A45 '^  panel-core:' deploy/compose.prod.yaml | grep -q '/var/run/docker.sock'; then
  echo 'panel-core must never mount the Docker socket' >&2
  exit 1
fi

echo 'deployment checks passed'
