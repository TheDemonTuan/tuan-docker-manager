# Docker Compose Management Panel — Deployment & Implementation Plan

> Mục tiêu: xây một panel quản trị Docker/Compose dành cho VPS, chạy hoàn toàn bằng Docker Compose, nhẹ, dễ triển khai, ưu tiên bảo mật, không public port quản trị trực tiếp, tận dụng Cloudflare Tunnel + Access.

---

## 1. Mục tiêu hệ thống

Panel cần quản lý toàn bộ workload Docker trên một VPS:

- Xem trạng thái VPS
- Xem CPU / RAM / Swap / Disk / Network / Load / Uptime
- Xem GPU NVIDIA: utilization, VRAM, temperature, power
- Xem toàn bộ Docker containers
- Quản lý Docker Compose stacks
- Start / Stop / Restart / Kill container
- Pull image
- Build image
- `compose up`
- `compose down`
- `compose restart`
- `compose pull`
- Xem logs realtime
- Terminal vào container
- Xem container stats realtime
- Lưu history metrics
- Quản lý images
- Quản lý volumes
- Quản lý networks
- Xem Docker events
- Audit log
- Security scan cho Compose
- Backup cấu hình panel và Compose
- Hỗ trợ Cloudflare Access làm lớp đăng nhập ngoài
- Không expose Docker socket cho frontend/API chính

Không triển khai trong MVP:

- Kubernetes
- Prometheus
- Grafana
- Redis
- PostgreSQL
- RabbitMQ
- Elasticsearch
- CI/CD phức tạp
- Multi-host ngay từ đầu

---

# 2. Nguyên tắc kiến trúc

## 2.1 Security-first

Nguyên tắc quan trọng nhất:

> Docker socket tương đương quyền rất cao trên host. Không được mount trực tiếp vào UI/API container.

Không được:

```text
browser -> panel-core -> /var/run/docker.sock
```

Phải đi qua:

```text
browser
  -> Cloudflare
  -> panel-core
  -> Unix socket
  -> panel-agent
  -> docker.sock
```

---

## 2.2 Zero public port

Panel không publish port ra Internet.

Không dùng:

```yaml
ports:
  - "8080:8080"
```

Nếu `cloudflared` chạy bằng Docker, traffic đi qua internal Docker network.

Nếu `cloudflared` chạy trên host, chỉ bind localhost:

```yaml
ports:
  - "127.0.0.1:8080:8080"
```

---

## 2.3 Lightweight

Panel chỉ cần:

```text
cloudflared    # nếu VPS chưa có
panel-core
panel-agent
```

Không cần database server riêng.

Database:

```text
SQLite
```

Frontend:

```text
React static build
```

được embed vào binary Go.

---

# 3. Kiến trúc production

```text
                         INTERNET
                             |
                             v
                +-------------------------+
                |       CLOUDFLARE        |
                |                         |
                | DNS                     |
                | Zero Trust Access       |
                | Tunnel                  |
                | WAF / DDoS              |
                +------------+------------+
                             |
                             | outbound tunnel
                             v
+---------------------------- VPS ----------------------------+
|                                                             |
|                    +----------------+                       |
|                    |  cloudflared   |                       |
|                    +-------+--------+                       |
|                            |                                |
|                       panel-edge                            |
|                            |                                |
|                            v                                |
|                    +----------------+                       |
|                    |   panel-core   |                       |
|                    |                |                       |
|                    | Go API         |                       |
|                    | React UI       |                       |
|                    | SQLite         |                       |
|                    | Auth/RBAC      |                       |
|                    | Audit          |                       |
|                    | Metrics store  |                       |
|                    +-------+--------+                       |
|                            |                                |
|                       Unix Socket                           |
|                            |                                |
|                            v                                |
|                    +----------------+                       |
|                    |  panel-agent   |                       |
|                    |                |                       |
|                    | Docker SDK     |                       |
|                    | Compose SDK    |                       |
|                    | Host metrics   |                       |
|                    | GPU metrics    |                       |
|                    +-------+--------+                       |
|                            |                                |
|                      docker.sock                            |
|                            |                                |
|                            v                                |
|                  +--------------------+                     |
|                  |   Docker Engine    |                     |
|                  +--------------------+                     |
|                                                             |
+-------------------------------------------------------------+
```

---

# 4. Technology stack

## Backend

```text
Go
```

Thành phần:

- Docker Go SDK
- Docker Compose SDK
- SQLite
- HTTP API
- SSE
- WebSocket
- Unix socket RPC/API nội bộ
- host metrics collector
- GPU collector

---

## Frontend

```text
React
Vite
TypeScript
Tailwind CSS
CodeMirror 6
xterm.js
uPlot
```

### Vai trò

- React: UI
- Vite: build
- TypeScript: type safety
- Tailwind: styling
- CodeMirror 6: compose editor
- xterm.js: terminal
- uPlot: metrics chart

---

## Database

```text
SQLite
```

Không dùng PostgreSQL trong MVP.

---

# 5. Cấu trúc filesystem trên VPS

Khuyến nghị:

```text
/opt/docker-panel/
├── compose.yaml
├── .env
└── secrets/

/srv/docker-panel/
├── stacks/
│   ├── immich/
│   │   ├── compose.yaml
│   │   ├── .env
│   │   └── data/
│   │
│   ├── postgres/
│   │   ├── compose.yaml
│   │   └── data/
│   │
│   └── ollama/
│       ├── compose.yaml
│       └── data/
│
├── backups/
└── secrets/

/var/lib/docker-panel/
└── panel.db
```

Panel chỉ được quản lý stack trong:

```text
/srv/docker-panel/stacks/
```

---

# 6. Docker Compose của chính panel

Skeleton:

```yaml
services:

  panel-core:
    image: ghcr.io/your-org/docker-panel-core:latest

    expose:
      - "8080"

    networks:
      - panel-edge

    volumes:
      - panel-data:/data
      - agent-socket:/run/panel-agent

    read_only: true

    tmpfs:
      - /tmp

    security_opt:
      - no-new-privileges:true

    cap_drop:
      - ALL

    restart: unless-stopped


  panel-agent:
    image: ghcr.io/your-org/docker-panel-agent:latest

    network_mode: none

    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /srv/docker-panel:/srv/docker-panel
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - agent-socket:/run/panel-agent

    read_only: true

    tmpfs:
      - /tmp

    security_opt:
      - no-new-privileges:true

    cap_drop:
      - ALL

    restart: unless-stopped


volumes:
  panel-data:
  agent-socket:


networks:
  panel-edge:
    external: true
```

Nếu `cloudflared` đã tồn tại, cho nó join `panel-edge`.

---

# 7. Cloudflare Tunnel

Hostname ví dụ:

```text
panel.example.com
```

Route:

```text
panel.example.com
        |
        v
http://panel-core:8080
```

Không publish `panel-core` ra Internet.

---

# 8. Cloudflare Access

Tạo Access Application:

```text
panel.example.com
```

Policy mặc định:

```text
DENY
```

Allow:

```text
admin@example.com
```

Hoặc danh sách email admin.

Khuyến nghị:

```text
Cloudflare Access
        |
        v
exact email allowlist
        |
        v
panel
```

---

# 9. Authentication bên trong panel

Cloudflare Access quyết định:

```text
User có được vào panel hay không?
```

Panel quyết định:

```text
User được phép làm gì?
```

Không cần login password lần hai trong MVP.

Panel lấy identity từ Access JWT/header đã verify.

---

# 10. RBAC

Các role:

```text
VIEWER
OPERATOR
ADMIN
OWNER
```

## VIEWER

- xem dashboard
- xem stats
- xem logs
- xem inspect
- xem compose
- xem events

## OPERATOR

Bao gồm VIEWER:

- start
- stop
- restart
- pull
- compose up
- compose restart

## ADMIN

Bao gồm OPERATOR:

- edit compose
- create stack
- import stack
- build
- quản lý images
- quản lý networks
- quản lý non-critical volumes

## OWNER

Bao gồm ADMIN:

- delete volume
- `compose down -v`
- unsafe deploy
- secrets
- user/role management
- system prune
- panel settings

---

# 11. Critical action re-auth

Các thao tác critical nên yêu cầu Passkey/WebAuthn:

```text
delete volume
down -v
prune volumes
unsafe compose
privileged deployment
mount docker.sock
mount /
delete stack data
```

Flow:

```text
User click critical action
        |
        v
Confirm
        |
        v
Passkey
        |
        v
Execute
```

---

# 12. Agent isolation

`panel-agent` là service duy nhất có:

```text
/var/run/docker.sock
```

Không có:

```text
TCP listener
public port
Docker network
Internet access
```

Dùng:

```yaml
network_mode: none
```

Communication:

```text
panel-core
   |
   v
/run/panel-agent/agent.sock
   |
   v
panel-agent
```

---

# 13. Không xây raw Docker API proxy

Không được implement kiểu:

```text
/api/docker/*
```

Agent phải expose action explicit.

Ví dụ:

```text
ListContainers
InspectContainer
StartContainer
StopContainer
RestartContainer
KillContainer

ReadLogs

ComposeValidate
ComposeUp
ComposeDown
ComposePull
ComposeBuild
ComposeRestart

ListImages
PullImage
DeleteImage

ListVolumes
DeleteVolume

ListNetworks
DeleteNetwork
```

---

# 14. Stack filesystem policy

Mỗi stack:

```text
/srv/docker-panel/stacks/<stack-name>/
```

Ví dụ:

```text
/srv/docker-panel/stacks/immich/
├── compose.yaml
├── .env
└── data/
```

Không nhận arbitrary host path trực tiếp từ frontend.

Backend resolve bằng stack ID.

---

# 15. Compose management

Trang stack:

```text
Stack
├── Overview
├── Compose
├── Containers
├── Logs
├── Metrics
├── Volumes
├── Networks
├── Secrets
├── Security
└── History
```

Action:

```text
Up
Down
Restart
Pull
Build
Redeploy
```

---

# 16. Compose editor

Editor:

```text
CodeMirror 6
```

Flow Save:

```text
user edit
   |
   v
parse YAML
   |
   v
Compose SDK validation
   |
   v
security scanner
   |
   v
create revision
   |
   v
write temp file
   |
   v
atomic rename
```

---

# 17. Compose revision history

Schema:

```text
stack_revisions

id
stack_id
created_at
user_id
content
sha256
message
```

UI:

```text
v18    updated image
v17    added redis
v16    changed env

View Diff
Restore
```

---

# 18. Security scanner cho Compose

Scanner phải phát hiện:

## Critical

```yaml
privileged: true
```

```yaml
volumes:
  - /:/host
```

```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock
```

```yaml
cap_add:
  - SYS_ADMIN
```

---

## High Risk

```yaml
pid: host
```

```yaml
ipc: host
```

```yaml
network_mode: host
```

Host device:

```yaml
devices:
  - /dev/...
```

---

## Medium Risk

Public ports:

```yaml
ports:
  - "5432:5432"
```

Image latest:

```yaml
image: nginx:latest
```

No resource limits.

Writable root filesystem.

---

# 19. Allowed host paths

Mặc định chỉ cho:

```text
/srv/docker-panel/stacks/<stack>/*
```

Nếu cần thêm:

```text
/mnt/media
/mnt/storage
```

admin phải allow trong settings:

```text
Allowed Host Paths

/srv/docker-panel
/mnt/media
/mnt/storage
```

---

# 20. Public port policy

Mặc định block hoặc cảnh báo:

```yaml
ports:
  - "5432:5432"
```

Cho phép:

```yaml
ports:
  - "127.0.0.1:5432:5432"
```

hoặc không publish port và chỉ dùng Docker network.

UI:

```text
Network Exposure

nginx
443       PUBLIC

postgres
5432      INTERNAL

redis
6379      INTERNAL

ollama
11434     LOCALHOST
```

---

# 21. Security score

Mỗi stack:

```text
Security Score: 94 / 100
```

Ví dụ:

```text
+ No privileged mode
+ No Docker socket
+ No host filesystem
+ No dangerous capabilities
+ No public DB port
+ Resource limit configured

- image uses :latest
```

---

# 22. Docker discovery

Backend lấy:

- containers
- images
- volumes
- networks
- labels
- ports
- health
- state

Mapping stack/container qua labels:

```text
com.docker.compose.project
com.docker.compose.service
```

Không map dựa vào container name.

---

# 23. Container page

```text
Container / ollama

Status
Image
Uptime
Restart policy
Network
IP

CPU
RAM
GPU
VRAM

Tabs:
Overview
Logs
Terminal
Stats
Inspect
```

Actions:

```text
Start
Stop
Restart
Kill
```

---

# 24. Logs realtime

Dùng:

```text
Docker Logs API
        |
        v
panel-agent
        |
        v
panel-core
        |
        v
SSE
        |
        v
Browser
```

Frontend options:

```text
Follow
Timestamp
Search
Filter
```

---

# 25. Terminal

Frontend:

```text
xterm.js
```

Flow:

```text
Browser
   |
 WebSocket
   |
panel-core
   |
 Unix socket
   |
panel-agent
   |
Docker Exec API
   |
container shell
```

Shell fallback:

```text
/bin/bash
/bin/sh
```

---

# 26. Host metrics

Agent đọc trực tiếp:

## CPU

```text
/host/proc/stat
```

## RAM

```text
/host/proc/meminfo
```

## Load

```text
/host/proc/loadavg
```

## Uptime

```text
/host/proc/uptime
```

## Network

```text
/host/proc/net/dev
```

## Disk

```text
statfs
```

## Disk IO

```text
/host/proc/diskstats
```

---

# 27. Container metrics

Dùng Docker stats API.

Metrics:

```text
CPU %
Memory
Memory %
PIDs
Network RX
Network TX
Block Read
Block Write
```

Sampling:

```text
realtime read: 2-3 sec
database write: 30 sec
```

---

# 28. GPU metrics

Nếu VPS có NVIDIA:

```text
NVIDIA NVML
```

Metrics:

```text
GPU utilization
VRAM used
VRAM total
Temperature
Fan speed
Power draw
Power limit
Clock
```

V2:

```text
PID -> cgroup -> container ID
```

để map GPU process về container.

---

# 29. Metrics retention

Raw:

```text
30 sec sample
7 days
```

Rollup:

```text
5 minute
90 days
```

Long-term:

```text
1 hour
1 year
```

Không lưu raw mỗi 2 giây vào SQLite.

---

# 30. Database schema

Các bảng:

```text
users
sessions

hosts

stacks
stack_revisions

jobs

host_metrics
container_metrics
gpu_metrics

docker_events

audit_logs

alert_rules
alert_events

settings

secrets_metadata
```

---

# 31. Hosts table

Ngay cả khi MVP chỉ 1 VPS:

```text
hosts

id
name
hostname
status
created_at
last_seen_at
```

Mọi metric/resource có:

```text
host_id
```

để sau này mở rộng multi-host.

---

# 32. Job system

Các thao tác dài:

```text
pull
build
compose up
compose down
prune
```

không giữ HTTP request treo.

Flow:

```text
POST action
   |
   v
create job
   |
   v
worker
   |
   v
agent
```

Frontend nhận progress qua SSE/WebSocket.

---

# 33. Stack lock

Một stack chỉ cho phép một mutation job cùng lúc.

Ví dụ không được xảy ra:

```text
compose down
compose up
compose pull
```

đồng thời.

Cần:

```text
stack mutex / job lock
```

---

# 34. Docker events

Subscribe Docker Events:

```text
create
start
stop
restart
die
destroy
oom
health_status
exec_start
exec_die
```

Dùng để update dashboard realtime.

---

# 35. Internal event bus

```text
Docker Events
Metrics
Jobs
Alerts
     |
     v
Internal Event Bus
     |
     +-> WebSocket/SSE
     +-> Audit
     +-> Alert engine
     +-> Database
```

---

# 36. Audit logging

Mọi action quan trọng:

```text
user
timestamp
IP
action
resource
metadata
result
```

Ví dụ:

```text
admin restarted container ollama

admin edited compose stack ai

admin enabled unsafe deployment

admin deleted volume postgres_data
```

---

# 37. Secrets

Không lưu plaintext secrets vào DB.

Dùng:

```text
XChaCha20-Poly1305
```

Database:

```text
ciphertext
nonce
metadata
```

Master key ngoài SQLite:

```text
/opt/docker-panel/secrets/master.key
```

Permission:

```text
0600
```

---

# 38. Backup

Backup quan trọng:

```text
panel.db
compose files
.env
panel settings
encrypted secrets
```

Không ưu tiên backup metrics.

---

# 39. Cloudflare R2 backup

Pipeline:

```text
data
 |
 v
tar
 |
 v
zstd
 |
 v
encrypt
 |
 v
R2
```

R2 chỉ nhận:

```text
*.enc
```

Không upload plaintext secret.

---

# 40. Backup retention

Đề xuất:

```text
7 daily
4 weekly
3 monthly
```

Panel DB:

```text
every 6h
```

Compose config:

```text
backup on every modification
```

R2:

```text
daily
```

---

# 41. Alert engine

Rules:

```text
CPU > 90% for 5m
RAM > 90% for 5m
Disk > 85%
GPU temp > 85 C
VRAM > 95%
Container unhealthy
Container OOM
Container restart loop
Container unexpectedly stopped
```

Notification phase sau:

```text
Telegram
Discord
Webhook
Email
```

---

# 42. Panel self-protection

Panel stack phải có flag:

```text
SYSTEM STACK
```

Không cho UI:

```text
Down
Delete
Prune
Edit critical mount
```

Panel update bằng SSH hoặc updater riêng.

---

# 43. API structure

Base:

```text
/api/v1
```

## Session / identity

```text
GET /auth/me
POST /auth/passkey/challenge
POST /auth/passkey/verify
```

## Dashboard

```text
GET /dashboard
```

## Stacks

```text
GET    /stacks
POST   /stacks
GET    /stacks/:id
DELETE /stacks/:id

GET /stacks/:id/compose
PUT /stacks/:id/compose

POST /stacks/:id/up
POST /stacks/:id/down
POST /stacks/:id/restart
POST /stacks/:id/pull
POST /stacks/:id/build

GET /stacks/:id/security
GET /stacks/:id/revisions
POST /stacks/:id/revisions/:revision/restore
```

## Containers

```text
GET /containers
GET /containers/:id

POST /containers/:id/start
POST /containers/:id/stop
POST /containers/:id/restart
POST /containers/:id/kill

GET /containers/:id/stats
GET /containers/:id/logs
```

## Docker resources

```text
GET /images
POST /images/pull
DELETE /images/:id

GET /volumes
DELETE /volumes/:id

GET /networks
DELETE /networks/:id
```

## Metrics

```text
GET /metrics/host
GET /metrics/container/:id
GET /metrics/gpu
```

## Events

```text
GET /events
```

## Audit

```text
GET /audit
```

---

# 44. Project structure

```text
docker-panel/
│
├── cmd/
│   ├── panel-core/
│   │   └── main.go
│   │
│   └── panel-agent/
│       └── main.go
│
├── internal/
│   ├── api/
│   ├── auth/
│   ├── rbac/
│   ├── docker/
│   ├── compose/
│   ├── agent/
│   ├── metrics/
│   ├── gpu/
│   ├── jobs/
│   ├── events/
│   ├── alerts/
│   ├── audit/
│   ├── security/
│   ├── secrets/
│   ├── backup/
│   ├── database/
│   └── config/
│
├── web/
│   ├── src/
│   ├── package.json
│   └── vite.config.ts
│
├── migrations/
│
├── deploy/
│   ├── compose.yaml
│   └── example.env
│
├── go.mod
├── Makefile
└── README.md
```

---

# 45. Frontend navigation

```text
Dashboard

Stacks
Containers

Images
Volumes
Networks

Host
GPU

Events
Alerts

Security
Audit

Settings
```

---

# 46. Dashboard layout

```text
+-------------------------------------------------------+
| VPS-01                                  Healthy       |
+-------------------------------------------------------+
| CPU        RAM        DISK       LOAD                 |
| 32%        54%        41%        0.84                 |
+-------------------------------------------------------+
| GPU        VRAM       TEMP       POWER                |
| 72%        14/24 GB   68 C       240 W                |
+-------------------------------------------------------+
| STACK       STATUS      CPU      RAM      SECURITY     |
| immich      healthy     12%      2.3G     94/100       |
| postgres    healthy      3%      1.1G     98/100       |
| ollama      healthy     67%      8.2G     91/100       |
+-------------------------------------------------------+
```

---

# 47. Deployment preview

Trước khi deploy:

```text
Deploy Preview

Images
+ nginx:1.29

Containers
+ web

Networks
+ app_default

Volumes
+ app_data

Ports
+ 443 public

Security
LOW

Confirm Deploy
```

---

# 48. Failure strategy

Deploy:

```text
validate
  |
security scan
  |
revision backup
  |
pull/build
  |
compose up
  |
health check
```

Nếu fail:

```text
show exact error
offer rollback
```

Không auto rollback database/data volume một cách mù quáng.

---

# 49. MVP phases

## Phase 1 — Foundation

Build:

- Go core
- Go agent
- SQLite
- React
- Unix socket communication
- Docker Compose deployment
- Cloudflare Tunnel route

Deliverable:

```text
panel reachable through Cloudflare Access
```

---

## Phase 2 — Identity & Security

Build:

- Cloudflare identity integration
- users table
- RBAC
- audit logging
- CSRF/origin protection
- critical-action confirmation

Deliverable:

```text
secure authenticated panel
```

---

## Phase 3 — Docker Discovery

Build:

- list containers
- images
- volumes
- networks
- Docker info
- Docker events

Deliverable:

```text
full Docker inventory
```

---

## Phase 4 — Monitoring

Build:

- host CPU
- RAM
- disk
- network
- load
- uptime
- container CPU/RAM
- realtime stream

Deliverable:

```text
live dashboard
```

---

## Phase 5 — Compose Discovery

Build:

- scan `/srv/docker-panel/stacks`
- parse Compose
- map project labels
- import existing stacks
- stack overview

Deliverable:

```text
panel understands Compose stacks
```

---

## Phase 6 — Compose Management

Build:

- editor
- validation
- revision history
- up
- down
- restart
- pull
- build

Deliverable:

```text
usable Compose manager
```

---

## Phase 7 — Logs & Terminal

Build:

- SSE logs
- xterm.js
- WebSocket terminal
- Docker exec

Deliverable:

```text
container operations from browser
```

---

## Phase 8 — Compose Security Scanner

Build:

- privileged detection
- docker.sock detection
- host filesystem detection
- capability detection
- public ports
- host network
- device mounts
- security score

Deliverable:

```text
security-first deployment workflow
```

---

## Phase 9 — Metrics History

Build:

- SQLite metrics
- rollup
- retention
- uPlot charts

Deliverable:

```text
1h / 6h / 24h / 7d / 30d history
```

---

## Phase 10 — GPU

Build:

- NVIDIA NVML
- utilization
- VRAM
- temperature
- power
- process mapping

Deliverable:

```text
GPU monitoring
```

---

## Phase 11 — Storage & Cleanup

Build:

- image management
- volumes
- networks
- disk usage
- safe prune

Deliverable:

```text
Docker storage administration
```

---

## Phase 12 — Backup & Alerts

Build:

- encrypted backups
- R2 upload
- alert engine
- Telegram/webhook

Deliverable:

```text
production operations
```

---

# 50. Security checklist trước production

## Cloudflare

- [ ] Tunnel hoạt động
- [ ] Access Application tạo trước khi public hostname
- [ ] Default deny
- [ ] Exact email allowlist
- [ ] Access JWT validation
- [ ] Không bypass Access

## VPS

- [ ] Panel không publish public port
- [ ] Docker TCP 2375 disabled
- [ ] SSH key only
- [ ] Root login SSH disabled nếu phù hợp
- [ ] Firewall deny unnecessary inbound
- [ ] Docker daemon up to date

## panel-core

- [ ] Không mount docker.sock
- [ ] read_only filesystem
- [ ] cap_drop ALL
- [ ] no-new-privileges
- [ ] secure cookies
- [ ] CSRF protection
- [ ] origin validation
- [ ] rate limit
- [ ] audit enabled

## panel-agent

- [ ] Docker socket chỉ ở agent
- [ ] network_mode none
- [ ] no TCP listener
- [ ] read_only filesystem
- [ ] cap_drop ALL
- [ ] no-new-privileges
- [ ] filesystem allowlist
- [ ] strict API allowlist

## Compose

- [ ] block privileged by default
- [ ] block docker.sock by default
- [ ] block `/` mount by default
- [ ] dangerous caps warning
- [ ] public ports warning/block
- [ ] host network warning
- [ ] revision before deploy
- [ ] deploy preview
- [ ] rollback available

## Secrets

- [ ] master key outside SQLite
- [ ] master key chmod 0600
- [ ] secrets encrypted at rest
- [ ] no plaintext secrets in logs
- [ ] no secrets committed to Git

## Backup

- [ ] panel DB backup
- [ ] Compose backup
- [ ] encrypted off-site backup
- [ ] restore tested

---

# 51. Definition of Done cho MVP

MVP được xem là hoàn thành khi:

- [ ] Panel deploy bằng Docker Compose
- [ ] Không cần mở port public
- [ ] Truy cập qua Cloudflare Access
- [ ] Agent là service duy nhất có Docker socket
- [ ] Dashboard xem CPU/RAM/Disk/Network
- [ ] Xem container list và stats
- [ ] Xem Compose stacks
- [ ] Edit compose
- [ ] Validate compose
- [ ] Security scan compose
- [ ] Up/down/restart/pull/build
- [ ] Logs realtime
- [ ] Container terminal
- [ ] Revision history
- [ ] Audit log
- [ ] Role-based access
- [ ] Metrics history
- [ ] NVIDIA GPU monitoring
- [ ] Backup config
- [ ] Critical action confirmation

---

# 52. Mục tiêu cuối cùng

Hệ thống production lý tưởng:

```text
Cloudflare
   |
   v
cloudflared
   |
   v
panel-core
   |
 Unix socket
   |
   v
panel-agent
   |
   v
Docker Engine
```

Panel chỉ dùng:

```text
Go
React
SQLite
Docker SDK
Compose SDK
NVML
Cloudflare
```

Không cần:

```text
Redis
PostgreSQL
Prometheus
Grafana
RabbitMQ
Kubernetes
```

Ưu tiên theo thứ tự:

```text
1. Security
2. Reliability
3. Convenience
4. Lightweight
5. Extensibility
```

---

# 53. Hướng triển khai thực tế đề xuất

Bắt đầu code theo đúng thứ tự:

```text
1. docker-compose skeleton
2. panel-agent
3. panel-core
4. Unix socket protocol
5. Docker discovery
6. React dashboard
7. Cloudflare identity
8. Compose discovery
9. Compose editor
10. Compose operations
11. Logs
12. Terminal
13. Security scanner
14. Metrics history
15. GPU
16. Backup
17. Alerts
```

Không build toàn bộ cùng lúc.

Mỗi phase phải có version chạy được và deploy được lên VPS.

---

# 54. Release strategy

Version gợi ý:

```text
v0.1
foundation + Docker discovery

v0.2
Compose management

v0.3
logs + terminal

v0.4
security scanner

v0.5
metrics history + GPU

v0.6
backup + alerts

v1.0
production stable
```

---

# 55. Nguyên tắc cuối

Luôn coi:

```text
Docker control == host-level privilege
```

Vì vậy:

```text
Cloudflare Access
        +
zero public port
        +
isolated agent
        +
explicit Docker API
        +
Compose security scanner
        +
RBAC
        +
audit
        +
backup
```

là bộ khung bắt buộc của project, không phải tính năng thêm sau.

