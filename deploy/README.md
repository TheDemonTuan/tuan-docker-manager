# Production deployment to the VPS

This repository uses the same deployment model as the owner's `facebook-messenger-ai-rep` and `tuan-portfolio` repositories:

1. A push to `main` runs tests and builds the React application.
2. GitHub Actions builds `panel-core` and `panel-agent` images.
3. Both images are published to GitHub Container Registry (GHCR) with `latest` and commit-SHA tags.
4. The build output digest is passed to deployment, so the VPS always runs immutable `ghcr.io/...@sha256:...` references.
5. GitHub Actions copies only `compose.prod.yaml` and `deploy.sh` to the VPS over SSH.
6. `deploy.sh` uses `flock`, pulls both images, runs Docker Compose with `--wait`, checks core and agent health, records the active pair, and rolls back both services if deployment fails.

After one-time bootstrap, the VPS never clones or pulls the repository and never builds production images. Application secrets remain on the VPS.

## First-time VPS setup

Use Ubuntu/Debian with Docker Engine, Docker Compose v2, OpenSSH, and `flock` installed. Create a non-root deployment user with SSH key authentication. Copy the repository's `deploy` directory to the VPS once, then run:

```bash
sudo /path/to/deploy/bootstrap-vps.sh
```

Then configure:

```bash
sudoedit /opt/tuan-docker-manager/.env
sudoedit /opt/tuan-docker-manager/.tunnel-token
sudo chmod 600 /opt/tuan-docker-manager/.env /opt/tuan-docker-manager/.tunnel-token
```

`/opt/tuan-docker-manager/.env`:

```dotenv
ALLOWED_EMAILS=admin@example.com
ADMIN_EMAIL=admin@example.com
STACKS_HOST_PATH=/srv/docker-panel/stacks
```

`/opt/tuan-docker-manager/.tunnel-token` contains only the Cloudflare Tunnel token, with no variable name or quotes.

Before the first deploy, configure the Cloudflare Tunnel public hostname to route to:

```text
http://panel-core:8080
```

Create a Cloudflare Access application for that hostname before allowing DNS traffic.

## GitHub repository setup

Create a GitHub Environment named `production`. Add these Environment secrets:

| Secret | Value |
|---|---|
| `VPS_HOST` | VPS IP address or SSH hostname |
| `VPS_USER` | Non-root deployment user |
| `VPS_PORT` | SSH port; optional, defaults to `22` |
| `VPS_SSH_KEY` | Private Ed25519 key for the deployment user |
| `VPS_KNOWN_HOSTS` | Pinned output from `ssh-keyscan -H -p <port> <host>` verified against the VPS host key |

Optional repository variables:

| Variable | Default | Purpose |
|---|---|---|
| `DEPLOY_PLATFORM` | `linux/arm64` | Set to `linux/amd64` for an x86-64 VPS |
| `BUILD_RUNNER` | `ubuntu-24.04-arm` | Set to `ubuntu-latest` when building amd64 or when the ARM runner is unavailable |

The workflow uses the built-in `GITHUB_TOKEN` to publish and temporarily authenticate the VPS to GHCR, then logs the VPS out after deployment. If the VPS cannot pull a private package using that token, grant the repository package access in GHCR settings or make the package public.

## Normal operation

Every push to `main` automatically deploys after tests and image builds pass. To build without deployment, run **Build and Deploy** manually and set `skip_deploy=true`.

Check the VPS state:

```bash
/opt/tuan-docker-manager/deploy.sh --status
```

Inspect logs:

```bash
cd /opt/tuan-docker-manager
docker compose --env-file .env --env-file .deploy.env -f compose.prod.yaml logs --tail=200 panel-core panel-agent cloudflared
```

The application publishes no host port. User traffic reaches `panel-core` only through `cloudflared` on the internal `panel-edge` network.

## Rollback behavior

A deployment updates core and agent as one immutable pair. If image pull, Compose startup, or either health check fails, `deploy.sh` restores the previous `.deploy.env` and recreates the previous pair. Docker keeps the active and previous layers; dangling layers are pruned after success.

To perform a manual rollback, retrieve the two references from `.previous-images` and run:

```bash
core_image="$(grep '^PANEL_CORE_IMAGE=' /opt/tuan-docker-manager/.previous-images | cut -d= -f2-)"
agent_image="$(grep '^PANEL_AGENT_IMAGE=' /opt/tuan-docker-manager/.previous-images | cut -d= -f2-)"
/opt/tuan-docker-manager/deploy.sh "$core_image" "$agent_image"
```

## Production checks

```bash
# Repository checks
bash deploy/test-deploy.sh
go test ./...
npm --prefix web ci
npm --prefix web run build

# VPS checks
/opt/tuan-docker-manager/deploy.sh --status
docker compose --env-file /opt/tuan-docker-manager/.env \
  --env-file /opt/tuan-docker-manager/.deploy.env \
  -f /opt/tuan-docker-manager/compose.prod.yaml config --quiet
```
