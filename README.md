# Docker Compose Management Panel (MVP)

A lightweight, security-first Docker Compose management panel designed for single-VPS production deployments, protected by Cloudflare Zero Trust with zero public ports.

---

## 1. Architectural Highlights & Security Model

```
                    INTERNET
                       │
                       ▼
          ┌─────────────────────────┐
          │     CLOUDFLARE EDGE     │
          │  • Zero Trust Access    │
          │  • WAF / DDoS           │
          │  • Tunnel Ingress       │
          └────────────┬────────────┘
                       │ Outbound Tunnel
                       ▼
┌───────────────── HOST VPS ─────────────────┐
│                                            │
│   ┌─────────────────────────────────────┐  │
│   │             cloudflared             │  │
│   └──────────────────┬──────────────────┘  │
│                      │ HTTP                │
│                      ▼                     │
│   ┌─────────────────────────────────────┐  │
│   │             panel-core              │  │
│   │  • Pure Go REST & WebSocket API     │  │
│   │  • Embedded React UI (Single Binary)│  │
│   │  • SQLite + WAL Mode                │  │
│   │  • Secrets Crypto (XChaCha20)       │  │
│   │  • RBAC & Origin Protection         │  │
│   │  • NO DOCKER SOCKET MOUNT           │  │
│   └──────────────────┬──────────────────┘  │
│                      │ Unix Domain Socket  │
│                      ▼ (/run/panel-agent)  │
│   ┌─────────────────────────────────────┐  │
│   │             panel-agent             │  │
│   │  • network_mode: none               │  │
│   │  • Docker Engine Communication      │  │
│   │  • Strict RPC Actions Only          │  │
│   │  • Path Confinement (/srv/..)       │  │
│   └──────────────────┬──────────────────┘  │
│                      │                     │
│                      ▼                     │
│              /var/run/docker.sock          │
└────────────────────────────────────────────┘
```

### Core Security Guarantees
1. **Core Never Touches Docker Socket**: `panel-core` is physically isolated from `/var/run/docker.sock`. Only `panel-agent` mounts the Docker socket.
2. **Zero Public Ports**: The panel does not publish any port (`8080:8080`) to the Internet. Ingress is routed through an outbound Cloudflare Tunnel (`cloudflared`).
3. **Agent Network Isolation**: `panel-agent` runs with `network_mode: none`, preventing inbound and outbound internet traffic. It communicates with `panel-core` solely via a shared Unix domain socket.
4. **No Raw Docker API Proxy**: The agent exposes strictly validated, typed RPC actions (ListContainers, ComposeUp, PruneImages, etc.) with strict path confinement to `/srv/docker-panel/stacks/<stack>/`.
5. **Least Privilege Runtime**: Both core and agent run with `read_only: true`, `security_opt: [no-new-privileges:true]`, and `cap_drop: [ALL]`.
6. **Cloudflare Zero Trust Identity**: Identity is verified via `Cf-Access-Authenticated-User-Email` header, with allowlist enforcement, Origin/CSRF validation, and RBAC roles: `VIEWER`, `OPERATOR`, `ADMIN`, `OWNER`.
7. **Encrypted Secrets**: Sensitive environment files (`.env`) and database backups are encrypted with **XChaCha20-Poly1305** using a 256-bit external master key (`/data/secrets/master.key`).

---

## 2. Feature Matrix

- **Compose Stack Management**:
  - Auto-discovery of existing Compose stacks in `/srv/docker-panel/stacks/`.
  - In-browser YAML editor with live syntax validation and syntax highlighting.
  - Automated Compose Security Scanner (0–100 security score) checking for privileged mode, root/socket mounts, dangerous capabilities, host namespaces, and public port exposures.
  - Revision history with snapshot storage, author metadata, and one-click rollback.
  - Asynchronous background job worker with per-stack mutex locks preventing concurrent conflicting operations.
- **Docker Inventory & Operations**:
  - Live container table with state badges, CPU/RAM stats, port bindings, and quick controls (start, stop, restart, kill).
  - Streaming Server-Sent Events (SSE) logs with tailing and timestamps.
  - Interactive WebSocket container terminal (`/bin/sh` / `/bin/bash`).
  - Image management (list, pull, remove, system prune).
  - Volume management with critical action confirmation modals and prune support.
  - Network inventory.
- **Host & Hardware Telemetry**:
  - Host CPU utilization, memory, swap, disk usage, and 1/5/15-minute load averages.
  - Realtime NVIDIA GPU telemetry (driver version, GPU temperature, VRAM usage, utilization %, power draw, and fan speed).
  - SVG sparkline time-series metrics.
- **Auditing, Alerts & Backups**:
  - Tamper-evident SQLite audit log recording every user action, IP, target resource, and status.
  - Alert rules engine monitoring thresholds (CPU, RAM, container health).
  - Automated or manual XChaCha20-Poly1305 encrypted `.tar.gz` backups containing database and stack definitions.

---

## 3. Directory Structure

```
.
├── cmd/
│   ├── panel-agent/            # Agent daemon (network_mode: none, communicates with Docker)
│   │   └── main.go
│   └── panel-core/             # Core backend API & embedded React UI
│       ├── main.go
│       └── webdist/            # Production React assets embedded via //go:embed
├── internal/
│   ├── agent/                  # Agent RPC protocol & HTTP client over Unix socket
│   ├── alerts/                 # System alerting rule engine
│   ├── api/                    # REST, SSE, and WebSocket HTTP handlers
│   ├── audit/                  # Audit logging service
│   ├── auth/                   # Cloudflare Access authentication & Origin/CSRF validation
│   ├── backup/                 # Encrypted backup creation and management
│   ├── compose/                # Stack discovery and Compose CLI runner
│   ├── database/               # Pure-Go SQLite (WAL mode) schema & repositories
│   ├── docker/                 # Unix socket Docker client
│   ├── events/                 # Realtime pub/sub event bus
│   ├── gpu/                    # NVIDIA SMI metrics collector
│   ├── jobs/                   # Background async job runner with per-stack locks
│   ├── metrics/                # Host procfs/sysfs metrics collector
│   ├── models/                 # Shared domain data models
│   ├── rbac/                   # Role-Based Access Control matrix
│   ├── secrets/                # XChaCha20-Poly1305 encryption manager
│   └── security/               # Compose YAML AST security scanner
├── web/                        # React 18 + Vite + TypeScript + Tailwind CSS UI
│   ├── src/
│   │   ├── components/         # Navbar, Sidebar, Modal, SVG Icons
│   │   ├── pages/              # Dashboard, Stacks, Containers, Resources,
│   │   │                       # Metrics, Events/Audit, Alerts/Backups, Settings
│   │   ├── api.ts              # Typed API client
│   │   └── types.ts            # TypeScript data contracts
│   └── package.json
├── deploy/
│   ├── Dockerfile.core         # Multi-stage build for panel-core
│   ├── Dockerfile.agent        # Multi-stage build for panel-agent
│   ├── cloudflared-config.yaml # Sample Cloudflare Tunnel configuration
│   └── .env.example            # Environment template
├── docker-compose.yml          # Production Compose specification
├── Makefile                    # Build & test automation
└── README.md
```

---

## 4. Quickstart Deployment

### Step 1: Clone and Configure Environment
```bash
cp deploy/.env.example .env
```
Edit `.env` to configure your admin email and Cloudflare credentials:
```env
DEV_MODE=false
ALLOWED_EMAILS=admin@example.com
ADMIN_EMAIL=admin@example.com
TUNNEL_TOKEN=eyJhIjoi... (Your Cloudflare Tunnel Token)
```

### Step 2: Initialize Directories on Host
```bash
sudo mkdir -p /srv/docker-panel/stacks
sudo mkdir -p /srv/docker-panel/backups
sudo chown -R 10001:10001 /srv/docker-panel
```

### Step 3: Launch with Docker Compose
```bash
docker compose up -d
```

### Step 4: Configure Cloudflare Zero Trust
1. Navigate to **Cloudflare Zero Trust** -> **Networks** -> **Tunnels**.
2. Create or configure your tunnel to route `panel.yourdomain.com` to `http://panel-core:8080`.
3. Navigate to **Access** -> **Applications** -> **Add an Application** (Self-hosted).
4. Set the domain to `panel.yourdomain.com`.
5. Add an Access Policy allowing your email address (`admin@example.com`).

---

## 5. Local Development & Testing

### Prerequisites
- Go 1.24+
- Node.js 20+ and npm

### Build All Binaries & Frontend
```bash
make build
```
Or build components individually:
```bash
# Build React frontend
make build-web

# Build backend binaries
make build-core
make build-agent
```

### Run Full Test Suite
```bash
make test
# Or directly with Go:
go test -v -race ./...
```

The test suite validates:
- Compose security AST scanner (privileged, root mounts, socket mounts, port exposures).
- XChaCha20-Poly1305 encryption, decryption, and tamper detection.
- RBAC permissions matrix and role enforcement.
- Agent IPC client/server RPC actions over mock transport.
- SQLite repositories with WAL mode, jobs, stack revisions, and audit logs.
- Cloudflare Access header parsing, dev authentication, and Origin/CSRF validation.
- Core REST API endpoints (`/api/v1/auth/me`, `/api/v1/dashboard`, `/api/v1/stacks`).

---

## 6. Security Analysis & Tradeoffs

| Mechanism | Implementation | Benefit |
| :--- | :--- | :--- |
| **Core / Agent Boundary** | Unix socket RPC over shared volume | Even if `panel-core` is compromised via RCE, the attacker has no access to the Docker socket. |
| **Agent Isolation** | `network_mode: none` | Agent cannot initiate or receive network connections; cannot be port-scanned or exploited from network. |
| **Path Confinement** | Whitelist confined to `/srv/docker-panel/stacks/` | Path traversal attacks (`../../etc/passwd`) are caught and blocked before execution. |
| **Zero Public Ingress** | Cloudflare Tunnel outbound connection | Server IP address is completely concealed; DDoS and volumetric attacks are absorbed by Cloudflare Edge. |
| **Origin & CSRF** | Origin / Host header matching on state-changing calls | Cross-origin requests from third-party sites are blocked. |
| **Critical Confirmations** | Dual-modal confirmation required | Irreversible actions (`docker system prune`, volume deletion) require typing the resource identifier. |
| **Crypto at Rest** | XChaCha20-Poly1305 with random nonces | Stack environment secrets and backup archives cannot be read from raw disk without the master key. |

---

## 7. Automated production deployment

Production deployment follows the same GHCR + immutable digest + SSH model used by the owner's other VPS applications. A push to `main` tests the project, builds both images, publishes them to GHCR, and deploys them as one versioned pair with health checks and automatic rollback.

See [`deploy/README.md`](deploy/README.md) for first-time VPS bootstrap, GitHub Environment secrets, Cloudflare setup, status checks, and manual rollback.

## 8. License

MIT License.
