# ALEX Acceptance & Verification Plan
> Last updated: 2025-11-18


**Version**: 2025-10-05  
**Owners**: Platform Team (Go backend), Web Experience Team (Next.js UI)  
**Purpose**: Define the end-to-end acceptance strategy that certifies the ALEX agent platform (CLI, HTTP server, Web UI) as production ready after the current refactor and backend alignment work.

---

## 1. Scope

| Component | Surfaces | Key Capabilities | Acceptance Focus |
|-----------|----------|------------------|------------------|
| Agent Core | Go domain + application layers | ReAct loop, tool execution, session persistence | Stability under long-running tasks, cost tracking, retry behaviour |
| HTTP Server | `cmd/alex-server` APIs + SSE | Task lifecycle, session CRUD, real-time streaming | Contract parity with web client, context isolation, error semantics |
| CLI | `alex` binary | Interactive TUI, command mode | Regression coverage vs server APIs, shared session store |
| Web UI | Next.js app | Task submission, streaming display, session management | UX alignment, SSE consumption, error surfaces |
| Observability & Ops | Prometheus, logs, cost store | Token/cost reporting, request tracing | Alerting hooks, operational dashboards |

Out of scope: third-party MCP services, enterprise SSO, custom deployment scripts (validated separately).

---

## 2. Acceptance Goals

1. **Functional completeness** – Every contract documented in `web/lib/types.ts` and API docs is satisfied by the Go server.  
2. **Streaming reliability** – SSE stays connected for ≥30 minutes per task, with strict session isolation and resumable listeners.  
3. **Data integrity** – Sessions, tasks, and cost records persist with no ID collisions or partial writes under concurrent load.  
4. **Operational readiness** – Metrics, structured logs, and health endpoints support 24/7 operations and alerting.  
5. **Security posture** – CORS, credential handling, env separation, and storage paths meet deployment checklist requirements.  
6. **User experience** – CLI and Web flows deliver consistent behaviour, surface errors clearly, and recover from transient faults.

---

## 3. Test Environments

| ID | Description | Runtime | Data Roots | Notes |
|----|-------------|---------|------------|-------|
| DEV | Local developer workstation | Go 1.25.1, Node 20 | `~/.alex-*` (isolated per tester) | Fast iteration, mock providers permitted |
| STAGE | Shared staging cluster | Kubernetes (3 replicas) | `s3://alex-stage-sessions`, `postgres://alex_stage_costs` | Mirrors production topology, no mock LLM |
| PROD-CAND | Pre-production sandbox | Same as prod | TBD (final) | One-shot smoke prior to cutover |

All acceptance suites must pass in DEV and STAGE. PROD-CAND executes a reduced smoke checklist.

---

## 4. Entry & Exit Criteria

**Entry**
- All P0 bugs closed; CI green (unit + integration + lint).  
- Task store + cost tracking features merged behind config flags.  
- Updated architecture docs (refactor plan, API contracts) published.

**Exit**
- 100% of acceptance scenarios below marked PASS with evidence in TestRail (or shared spreadsheet).  
- No open Sev-1/Sev-2 bugs; Sev-3 accepted by product.  
- Metrics dashboard + alert runbooks reviewed with Ops.  
- Rollback plan validated (CLI + server binaries packaged, migrations reversible).

---

## 4A. Current Readiness Gaps (2025-10-05 Audit)

| Gap | Impact on Acceptance | Required Fix Before Suite Execution |
|-----|----------------------|--------------------------------------|
| Task execution still blocks inside the HTTP handler | `POST /api/tasks` can’t return real `task_id`/`session_id`; Suite A/B consumers have nothing to track | Refactor `ServerCoordinator.ExecuteTaskAsync` to spawn background workers, resolve the session immediately, and return identifiers synchronously |
| Task store is purely in-memory and drops new session IDs | Restart wipes task history; responses keep blank `session_id`, breaking Suites A, C, and E | Persist tasks (file/DB) or, at minimum, update the store to copy `result.SessionID` and back it with durable storage before acceptance |
| Storage path expansion strips `$HOME` incorrectly | Sessions/costs try to write to `/.alex-*`; Suite C fails in multi-user envs | Fix `resolveStorageDir` to trim the `~/` prefix safely and add regression tests |
| Live progress never updates | Dashboard checks in Suites B/E can’t validate iterations/tokens | Emit iteration/tokens via `taskStore.UpdateProgress` from event callbacks |
| CLI session list still stubbed | CLI regression Suite F cannot pass; CLI/server become inconsistent | Wire `Container.ListSessions` to `SessionStore.List` so CLI surfaces real sessions |

All items are **P0 gating issues**. Acceptance execution may start only after responsible owners close them and the readiness rerun shows green.

## 5. Deliverables

- Completed acceptance run log with timestamps, environment, and artefacts.  
- API contract report (OpenAPI diff vs implementation).  
- SSE transcript samples per scenario stored in `docs/validation/streams/`.  
- Cost tracking audit (CSV export) for staged runs.  
- Incident simulation report (network flap, provider outage).  
- Sign-off memo co-signed by Backend, Web, Ops leads.

---

## 6. Acceptance Phases & Timeline

| Phase | Duration | Owner | Focus |
|-------|----------|-------|-------|
| **P0 – Readiness** | 2 days | Backend | Fix flaky tests, stabilise session/task store, add retry cost callbacks |
| **P1 – Backend Contract** | 3 days | Backend QA | API + SSE suites, persistence, cost tracking |
| **P2 – Frontend Alignment** | 3 days | Web QA | Web UI end-to-end flows, error UX, SSE resilience |
| **P3 – Ops & Security** | 2 days | Platform Ops | Observability, CORS, rate limits, auth stubs |
| **P4 – Resilience Drill** | 1 day | Joint | Chaos exercises, rollback confirmation |

Each phase requires daily sync + update in shared tracking doc. Exit to next phase only if blocking issues resolved.

---

## 7. Detailed Test Suites

### Suite A – Backend API & Task Lifecycle

| ID | Scenario | Steps | Expected |
|----|----------|-------|----------|
| A1 | Create task without session | POST `/api/tasks` (no `session_id`) | 200, body includes generated `task_id`, `session_id`, `status="pending"` |
| A2 | Create task with existing session | POST with valid `session_id` | Response echoes same session, new `task_id` linked |
| A3 | Fetch task status | GET `/api/tasks/{task_id}` | Status transitions `pending→running→completed`, timestamps populated |
| A4 | Cancel task | POST `/api/tasks/{task_id}/cancel` mid-execution | Task stops gracefully, SSE emits cancel event, session remains intact |
| A5 | Session CRUD | GET/DELETE `/api/sessions/{id}` | List includes metadata, delete purges files + cost rows |

Automation: Go integration tests (`internal/server/http`), Postman collections, `ghz` for load.

### Suite B – SSE Streaming & Event Isolation

| ID | Scenario | Validation |
|----|----------|------------|
| B1 | Long-lived stream | Execute 20-minute task (mock provider). No disconnects; heartbeat every 30s |
| B2 | Multi-session isolation | 3 concurrent sessions, ensure events tagged with correct `session_id`; subscribers receive only their stream |
| B3 | Reconnect after drop | Kill SSE connection mid-task, reconnect within 5s, events resume without loss |
| B4 | Tool streaming | Streaming tool emits partial chunks; ensure order preserved, final chunk flagged |
| B5 | Error propagation | Inject tool failure; SSE emits `error` event with `recoverable=false` |

Automation: Go tests + integration harness storing raw SSE logs for review.

### Suite C – Persistence & Data Integrity

| ID | Scenario | Validation |
|----|----------|------------|
| C1 | Concurrent session creation | 100 sessions/min (stage). All IDs unique, no file collisions |
| C2 | Task store durability | Crash server mid-task, restart; pending tasks reconciled or marked failed |
| C3 | Cost tracking accuracy | Compare recorded costs vs provider usage for 3 models (OpenRouter, DeepSeek, Mock) |
| C4 | Session export/import | Export session (JSON), re-import using CLI; history preserved |

### Suite D – Observability & Ops

| ID | Scenario | Validation |
|----|----------|------------|
| D1 | Metrics scrape | `/metrics` exposes LLM latency, SSE connections, task status gauges |
| D2 | Structured logging | Sample log contains `session_id`, `task_id`, severity, correlation IDs |
| D3 | Alert simulation | Trigger high error rate; alert fires in staging ops channel within 5 min |
| D4 | Cost reporting | Generate monthly CSV, totals reconcile with Test Suite C3 |

### Suite E – Web UI End-to-End

| ID | Scenario | Validation |
|----|----------|------------|
| E1 | Task submission | User flow from input→stream; UI displays session + status badges |
| E2 | Event rendering | All event types mapped to UI components (thinking spinners, tool cards) |
| E3 | Error UX | Backend error surfaces as toast + inline state, no silent failures |
| E4 | Session dashboard | List view paginated, filter by preset, delete works |
| E5 | Preset switch | Changing preset updates system prompt + tool set, confirmed via SSE event |

Automation: Playwright suite running against staged server; manual visual QA for styling.

### Suite F – CLI Regression

| ID | Scenario | Validation |
|----|----------|------------|
| F1 | Interactive chat | `alex` TUI completes multi-step task; transcripts stored |
| F2 | Command mode | `alex "task"` uses shared session store, matches server behaviour |
| F3 | Session resume | `alex -r <session>` shows history identical to server session GET |

### Suite G – Security & Resilience

| ID | Scenario | Validation |
|----|----------|------------|
| G1 | CORS policy | Allowed origin succeeds with credentials; disallowed origin blocked when `ALEX_ENV=production` |
| G2 | Rate limiting | Burst of 100 tasks/min; server throttles with `429` and retry headers |
| G3 | Secret handling | Verify API keys never logged or returned in responses |
| G4 | Dependency failure | Force LLM provider outage; retry logic triggers, cost callback fires once |
| G5 | Rollback drill | Deploy previous release binary; migration rollback executes cleanly |

---

## 8. Evidence Collection & Reporting

- **Automation pipelines**: nightly staged run posts coverage + pass/fail to Slack (#alex-release).  
- **Manual checklists**: stored under `docs/validation/YYYY-MM-DD-acceptance.md`.  
- **SSE transcripts**: saved as NDJSON for Suites B/E.  
- **Cost audits**: CSV exports archived in S3 and attached to acceptance report.  
- **Issue tracking**: All failures logged in Linear with label `acceptance-blocker`.

---

## 9. Risk Mitigation

- Parallelise suites where possible (see timeline) but gate prod readiness on Suites A–C.  
- If critical regression found, freeze feature merges until root cause fixed + regression test added.  
- Maintain mock LLM capability for deterministic runs; final sign-off must use real providers.  
- Document variances immediately—no verbal exemptions.

---

## 10. Sign-off Checklist

- [ ] Suite A passes in DEV & STAGE  
- [ ] Suite B passes with ≥3 concurrent sessions  
- [ ] Suite C cost delta < 1% vs provider report  
- [ ] Suite D dashboards + alerts reviewed by Ops  
- [ ] Suite E Playwright run recorded  
- [ ] Suite F CLI regressions completed  
- [ ] Suite G security items approved by security champion  
- [ ] Rollback drill documented  
- [ ] Acceptance packet archived & sign-off memo circulated

Once all boxes are checked and leads sign off, ALEX is cleared for production deployment of the aligned architecture.
