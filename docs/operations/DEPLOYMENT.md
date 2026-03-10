# Deployment Guide

Updated: 2026-03-10

How to deploy elephant.ai locally, with Docker Compose, or on Kubernetes. Includes leader agent deployment.

---

## 1. Prerequisites

- Go 1.24+
- Node.js 20+
- Docker (optional, recommended for sandbox and service dependencies)

---

## 2. Local Development

```bash
make build
alex setup
alex dev up
```

Default endpoints:
- Web UI: `http://localhost:3000`
- API/SSE: `http://localhost:8080`
- Health: `http://localhost:8080/health`

Common commands:

| Task | Command |
|------|---------|
| Check status | `alex dev status` |
| View server logs | `alex dev logs server` |
| View web logs | `alex dev logs web` |
| Stop all | `alex dev down` |
| Restart backend only | `alex dev restart backend` |

---

## 3. Docker Compose

Two compose files ship in `deploy/docker/`:
- `docker-compose.dev.yml` — hot-reload, local volumes
- `docker-compose.yml` — production-like build

### Dev mode

```bash
export LLM_API_KEY="sk-..."
cp examples/config/runtime-config.yaml ~/.alex/config.yaml
docker compose -f deploy/docker/docker-compose.dev.yml up -d
```

### Production mode

```bash
docker compose -f deploy/docker/docker-compose.yml build
docker compose -f deploy/docker/docker-compose.yml up -d
```

### Logs and teardown

```bash
docker compose -f deploy/docker/docker-compose.yml logs -f
docker compose -f deploy/docker/docker-compose.yml down
```

---

## 4. Kubernetes

No built-in K8s manifests are maintained. Build your own:

1. Build and push images:
   ```bash
   docker build -f deploy/docker/Dockerfile.server -t your-registry/alex-server:latest .
   docker build -f web/Dockerfile -t your-registry/alex-web:latest ./web
   ```
2. Store secrets (API keys, JWT secret, DB URLs) in a Secret resource.
3. Mount `config.yaml` via ConfigMap, or set `ALEX_CONFIG_PATH`.
4. Create Deployment, Service, Ingress, and HPA as needed.

---

## 5. Configuration

Main config: `~/.alex/config.yaml` (or path in `ALEX_CONFIG_PATH`).

Required environment variables:

| Variable | Purpose |
|----------|---------|
| `LLM_API_KEY` | LLM provider API key |
| `AUTH_JWT_SECRET` | JWT signing secret |
| `AUTH_DATABASE_URL` | Auth database connection |
| `ALEX_SESSION_DATABASE_URL` | Session database connection |

Example config snippet:

```yaml
runtime:
  llm_provider: "auto"
  api_key: "${LLM_API_KEY}"

auth:
  jwt_secret: "${AUTH_JWT_SECRET}"
  database_url: "${AUTH_DATABASE_URL}"

session:
  database_url: "${ALEX_SESSION_DATABASE_URL}"
```

Full field reference: `docs/reference/CONFIG.md`.

---

## 6. Leader Agent Deployment

The leader agent runs proactive features (blocker radar, weekly pulse, milestone check-in, prep brief). It runs inside the server process — no separate binary needed.

### Enable leader features

Add to `~/.alex/config.yaml`:

```yaml
proactive:
  scheduler:
    enabled: true

    blocker_radar:
      enabled: true
      schedule: "*/10 * * * *"
      stale_threshold_seconds: 1800
      channel: lark
      chat_id: oc_YOUR_CHAT_ID

    weekly_pulse:
      enabled: true
      schedule: "0 9 * * 1"
      channel: lark
      chat_id: oc_YOUR_CHAT_ID

    milestone_checkin:
      enabled: true
      schedule: "0 */1 * * *"
      channel: lark
      chat_id: oc_YOUR_CHAT_ID

    prep_brief:
      enabled: true
      schedule: "30 8 * * 1-5"
      member_id: ou_TARGET_MEMBER
      channel: lark
      chat_id: oc_YOUR_CHAT_ID
```

### Required Lark credentials

Set these env vars before starting the server:

```bash
export LARK_APP_ID="cli_xxx"
export LARK_APP_SECRET="yyy"
```

The Lark bot must be added to each target chat group.

### Attention gate and rate limiter

Control notification volume separately:

```yaml
channels:
  lark:
    attention_gate:
      enabled: true
      budget_max: 10
      budget_window_seconds: 600
      quiet_hours_start: 22
      quiet_hours_end: 8
    rate_limiter:
      enabled: true
      chat_hourly_limit: 10
      user_daily_limit: 50
```

### Multi-instance warning

The scheduler has no distributed lock by default. Run only one server instance with `proactive.scheduler.enabled: true` to avoid duplicate notifications.

### Verify leader health

```bash
curl -s http://localhost:8080/health | jq '.components[] | select(.name == "scheduler")'
```

Each job shows `registered`, `healthy`, `last_run`, and `next_run`.

---

## 7. Logs and Observability

Structured log files:

| Log file | Content |
|----------|---------|
| `alex-service.log` | Service-level events |
| `alex-llm.log` | LLM request/response traces |
| `alex-latency.log` | Latency measurements |

Log directories:
- `ALEX_LOG_DIR` (default: `$HOME`)
- `ALEX_REQUEST_LOG_DIR` (default: `${PWD}/logs/requests`)
- Dev process logs: `logs/server.log`, `logs/web.log`

Full details: `docs/reference/LOG_FILES.md`.

Leader-specific metrics are exported at `localhost:<prometheus_port>/metrics` (prefix `alex_leader_`). See `docs/runbooks/leader-agent-runbook.md` for alert rules.
