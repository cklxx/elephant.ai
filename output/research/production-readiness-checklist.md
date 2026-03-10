# Leader Agent MVP Production Readiness Checklist

Date: 2026-03-10

Scope:
- Leader-agent MVP launch readiness based on current `main`
- Verified against implementation in:
  - [internal/app](/Users/bytedance/code/elephant.ai/internal/app)
  - [internal/runtime/leader](/Users/bytedance/code/elephant.ai/internal/runtime/leader)
  - [internal/delivery/server/http](/Users/bytedance/code/elephant.ai/internal/delivery/server/http)
  - [internal/delivery/server/bootstrap](/Users/bytedance/code/elephant.ai/internal/delivery/server/bootstrap)
  - [internal/delivery/channels/lark](/Users/bytedance/code/elephant.ai/internal/delivery/channels/lark)
- Cross-checked with recent commits:
  - `c6c93f02` scheduler wiring
  - `b12be487` leader config validation
  - `8b1fe5ad` leader OpenAPI spec
  - `964afe3e` health-checker fix
  - `6faf379c` leader CLI
  - `8ae9f51b` Lark notification rate limiter

Verification run:

```bash
go test ./internal/app/blocker ./internal/app/pulse ./internal/app/prepbrief ./internal/app/milestone ./internal/app/decision ./internal/app/scheduler ./internal/runtime/leader ./internal/delivery/channels/lark ./internal/delivery/server/http ./internal/shared/config
```

Result: all passed.

## Overall Verdict

Overall status: `Partial`

Launch recommendation: `No-Go` for unattended production launch until the security and operations gaps below are closed.

Reason:
- The core MVP features now exist, are wired, and have meaningful test coverage.
- The main blockers are not feature absence. They are production-hardening gaps:
  - public HTTP routes still run without auth middleware
  - debug server explicitly runs without auth or rate limiting
  - the new Lark notification rate limiter exists but does not appear wired into live delivery
  - leader OpenAPI docs advertise endpoints that are not actually routed
  - no clear rollback / monitoring-alert playbook exists for leader-specific failures

## 1. Feature Completeness

| Item | Status | Evidence | Notes |
|---|---|---|---|
| Blocker Radar implemented | `Done` | [radar.go](/Users/bytedance/code/elephant.ai/internal/app/blocker/radar.go), [radar_test.go](/Users/bytedance/code/elephant.ai/internal/app/blocker/radar_test.go), commit `c6c93f02` | Core stuck-task detection and notification path exists and tests passed. |
| Weekly Pulse implemented | `Done` | [weekly.go](/Users/bytedance/code/elephant.ai/internal/app/pulse/weekly.go), [weekly_test.go](/Users/bytedance/code/elephant.ai/internal/app/pulse/weekly_test.go) | Digest generation is in place and tested. |
| Milestone Check-ins implemented | `Done` | [checkin.go](/Users/bytedance/code/elephant.ai/internal/app/milestone/checkin.go), [checkin_test.go](/Users/bytedance/code/elephant.ai/internal/app/milestone/checkin_test.go) | Hourly summary path exists and is tested. |
| Prep Brief generation implemented | `Done` | [brief.go](/Users/bytedance/code/elephant.ai/internal/app/prepbrief/brief.go), [brief_test.go](/Users/bytedance/code/elephant.ai/internal/app/prepbrief/brief_test.go) | Core brief generation exists and is tested. |
| All Phase 1 jobs wired into scheduler | `Done` | [scheduler.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/bootstrap/scheduler.go), [blocker_trigger.go](/Users/bytedance/code/elephant.ai/internal/app/scheduler/blocker_trigger.go), [prepbrief_trigger.go](/Users/bytedance/code/elephant.ai/internal/app/scheduler/prepbrief_trigger.go), commit `c6c93f02` | Earlier dark-code gap is closed: milestone, weekly pulse, blocker radar, and prep brief are now registered by the scheduler. |
| Prep Brief semantics match “calendar-triggered before 1:1” MVP claim | `Partial` | [prepbrief_trigger.go](/Users/bytedance/code/elephant.ai/internal/app/scheduler/prepbrief_trigger.go), [types.go](/Users/bytedance/code/elephant.ai/internal/shared/config/types.go) | The live trigger is still cron-based and keyed by configured `member_id`, not actual calendar 1:1 detection. Good enough for internal dogfood, weaker than the product copy. |
| Attention Gate fully matches launch claims | `Partial` | [attention_gate.go](/Users/bytedance/code/elephant.ai/internal/delivery/channels/lark/attention_gate.go), [leader_config.go](/Users/bytedance/code/elephant.ai/internal/shared/config/leader_config.go), commit `b12be487` | Budgeting and urgency classification exist, but the config exposes `quiet_hours_start/end` and `priority_threshold` without a clear enforcement path in the current gate implementation. |
| Leader feature test surface exists | `Done` | [Makefile](/Users/bytedance/code/elephant.ai/Makefile), target `leader-test`; package tests above passed | There is a dedicated leader test target and the core leader-related packages passed locally. |

## 2. Observability

| Item | Status | Evidence | Notes |
|---|---|---|---|
| HTTP request metrics and tracing | `Done` | [middleware_observability.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/middleware_observability.go), [metrics.go](/Users/bytedance/code/elephant.ai/internal/infra/observability/metrics.go), [observability.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/bootstrap/observability.go) | Request latency, status, bytes, tracing spans, and OTel metrics are wired. |
| SSE / task / web-vitals metrics | `Done` | [metrics.go](/Users/bytedance/code/elephant.ai/internal/infra/observability/metrics.go), [sse_handler_stream.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/sse_handler_stream.go), [api_handler_misc.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/api_handler_misc.go) | Good base telemetry coverage for runtime delivery surfaces. |
| Health endpoint for public server | `Done` | [health.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/app/health.go), [api_handler_misc.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/api_handler_misc.go), [health_integration_test.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/health_integration_test.go), commit `964afe3e` | `/health` exists, aggregates probes, and has integration coverage. |
| Debug health visibility for model-level details | `Done` | [router_debug.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/router_debug.go), [api_handler_misc.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/api_handler_misc.go) | Per-model health is split to the debug endpoint rather than the public health route. |
| Leader operational dashboard | `Done` | [leader_dashboard_handler.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/leader_dashboard_handler.go), [leader_dashboard_handler_test.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/leader_dashboard_handler_test.go), commit `6faf379c` | Good operator visibility for tasks, blockers, daily summary, and jobs. |
| Leader-specific alert outcome telemetry | `TODO` | no concrete sent/opened/dismissed/acted-on instrumentation found in leader paths | This is the main observability gap for launch tuning. The system measures transport/runtime health better than product usefulness. |
| SLO dashboards / paging alerts for leader failures | `TODO` | no leader-specific alerting rules or runbook-backed SLO docs found | Metrics exist, but I did not find shipped monitoring-alert definitions for leader-job failures, missed pulses, or stuck blocker scans. |

## 3. Security

| Item | Status | Evidence | Notes |
|---|---|---|---|
| HTTP rate limiting middleware exists | `Partial` | [middleware_rate_limit.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/middleware_rate_limit.go), [router.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/router.go) | Public server requests are rate-limited, but the key function trusts `X-Forwarded-For` / `X-Real-IP` directly via [http_util.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/http_util.go), which is weak unless front-proxy trust is controlled. |
| Lark leader-notification rate limiting | `TODO` | [rate_limiter.go](/Users/bytedance/code/elephant.ai/internal/delivery/channels/lark/rate_limiter.go), [rate_limiter_test.go](/Users/bytedance/code/elephant.ai/internal/delivery/channels/lark/rate_limiter_test.go), commit `8ae9f51b` | The rate limiter exists and is tested, but I did not find it wired into live notification sending paths. |
| Input/config validation for leader features | `Done` | [leader_config.go](/Users/bytedance/code/elephant.ai/internal/shared/config/leader_config.go), [leader_config_test.go](/Users/bytedance/code/elephant.ai/internal/shared/config/leader_config_test.go), commit `b12be487` | Cron syntax, thresholds, and conflicting config are validated. |
| Public API auth / authorization | `TODO` | [router.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/router.go) | The router still contains the explicit comment `Identity function — auth middleware removed.` That is not production-launch quality for leader endpoints. |
| Debug server auth / network hardening | `TODO` | [lark_debug.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/bootstrap/lark_debug.go), [router_debug.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/router_debug.go) | The debug server explicitly states `no auth, no rate limiting` and exposes `/metrics`, `/debug/pprof/*`, config mutation, memory, hooks, and runtime control surfaces. This must not be internet-reachable in production. |
| Public health output sanitization | `Done` | [health.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/app/health.go), [api_handler_misc.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/api_handler_misc.go) | Aggregate model health is exposed publicly; detailed per-model telemetry is limited to the debug endpoint. |

## 4. Performance

| Item | Status | Evidence | Notes |
|---|---|---|---|
| Benchmark harness exists | `Partial` | [Makefile](/Users/bytedance/code/elephant.ai/Makefile) `bench`, [event_broadcaster_benchmark_test.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/app/event_broadcaster_benchmark_test.go), [sse_render_benchmark_test.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/sse_render_benchmark_test.go) | There is generic benchmark support, but it is not leader-MVP specific. |
| Leader scheduler concurrency / timeout controls | `Partial` | [scheduler.go](/Users/bytedance/code/elephant.ai/internal/app/scheduler/scheduler.go), [bootstrap/scheduler.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/bootstrap/scheduler.go) | `MaxConcurrent`, cooldowns, trigger timeouts, and recovery settings exist, which is good. I did not find a leader-specific load envelope or proven capacity target. |
| Load testing for leader workflows | `TODO` | no leader-specific load tests found for blocker scans, concurrent digests, or Lark notification bursts | Generic HTTP/SSE benchmarks are not enough to prove MVP capacity under real proactive traffic. |
| Resource usage / memory budgets for leader jobs | `TODO` | no explicit leader-specific memory/CPU budget docs or tests found | The implementation has operational controls, but I did not find a defined budget or acceptance threshold for leader workloads. |
| Latency budgets / SLOs for leader user-facing loops | `TODO` | no concrete leader p95/p99 targets found in code or leader docs | This matters especially for blocker alerts, dashboard freshness, and prep brief delivery. |

## 5. Operations

| Item | Status | Evidence | Notes |
|---|---|---|---|
| Local deployment / start / status commands | `Done` | [docs/operations/DEPLOYMENT.md](/Users/bytedance/code/elephant.ai/docs/operations/DEPLOYMENT.md), [cmd/alex/health_cmd.go](/Users/bytedance/code/elephant.ai/cmd/alex/health_cmd.go), [cmd/alex/leader_cmd.go](/Users/bytedance/code/elephant.ai/cmd/alex/leader_cmd.go), [Makefile](/Users/bytedance/code/elephant.ai/Makefile) | Good local/dev operational story. |
| Generic deployment guide exists | `Partial` | [DEPLOYMENT.md](/Users/bytedance/code/elephant.ai/docs/operations/DEPLOYMENT.md) | There is a repo-level deployment guide, but it is not leader-MVP specific and does not include launch gating or exposure controls for the leader surfaces. |
| Graceful shutdown / recovery basics | `Partial` | [bootstrap/server.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/bootstrap/server.go), [scheduler/jobstore_file.go](/Users/bytedance/code/elephant.ai/internal/app/scheduler/jobstore_file.go) | There is job persistence and server shutdown logic, but I did not find a leader-specific rollback / degraded-mode checklist. |
| Rollback plan for bad leader rollout | `TODO` | no explicit rollback runbook found in docs/operations | Need a concrete “disable leader jobs / revert config / isolate debug server / revert deployment” procedure. |
| Monitoring alerts for production incidents | `TODO` | no shipped alert rules or on-call thresholds found | This is the biggest ops gap after auth/debug exposure. |
| Multi-instance scheduler safety | `Partial` | [bootstrap/scheduler.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/bootstrap/scheduler.go), [scheduler.go](/Users/bytedance/code/elephant.ai/internal/app/scheduler/scheduler.go) | The scheduler supports `LeaderLock`, but bootstrap currently passes `nil`, so production multi-instance deployment remains risky. |

## 6. Documentation

| Item | Status | Evidence | Notes |
|---|---|---|---|
| Product / launch-level feature overview | `Done` | [README.md](/Users/bytedance/code/elephant.ai/README.md), commit `54004c36` | The README now explains the leader-agent positioning and MVP feature set well. |
| API documentation for leader endpoints | `Done` | [openapi_leader.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/openapi_leader.go), [router.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/router.go), [openapi_leader_test.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/openapi_leader_test.go) | All 4 spec paths (`/dashboard`, `/tasks`, `/tasks/{id}/unblock`, `/openapi.json`) are routed and implemented. `TestLeaderOpenAPISpec_NoSpecDrift` prevents future spec/route mismatch. |
| Config guide for leader features | `Partial` | [README.md](/Users/bytedance/code/elephant.ai/README.md), [leader_config.go](/Users/bytedance/code/elephant.ai/internal/shared/config/leader_config.go), [docs/reference/CONFIG.md](/Users/bytedance/code/elephant.ai/docs/reference/CONFIG.md) | README has a quick enable snippet, but I did not find leader-specific config coverage in `docs/reference/CONFIG.md`. |
| Troubleshooting guide for leader failures | `Partial` | [docs/operations/DEPLOYMENT.md](/Users/bytedance/code/elephant.ai/docs/operations/DEPLOYMENT.md), [docs/guides/incident-response.md](/Users/bytedance/code/elephant.ai/docs/guides/incident-response.md), [docs/reference/LOG_FILES.md](/Users/bytedance/code/elephant.ai/docs/reference/LOG_FILES.md) | There is generic incident-response and log guidance, but not a focused leader-MVP troubleshooting doc covering missed pulses, noisy alerts, or stuck prep briefs. |
| Operator-facing test / status commands documented | `Done` | [Makefile](/Users/bytedance/code/elephant.ai/Makefile), [cmd/alex/cli.go](/Users/bytedance/code/elephant.ai/cmd/alex/cli.go), [cmd/alex/leader_cmd.go](/Users/bytedance/code/elephant.ai/cmd/alex/leader_cmd.go), commit `6faf379c` | CLI status/dashboard/config surfaces exist and are usable. |

## Launch Blockers

These should be treated as MVP launch blockers:

1. Add auth / authorization back to public leader and server APIs.
   - Evidence: [router.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/router.go)

2. Ensure the debug server is never publicly reachable, or add auth + rate limiting.
   - Evidence: [lark_debug.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/bootstrap/lark_debug.go), [router_debug.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/router_debug.go)

3. Either wire the Lark notification rate limiter into production delivery or remove it from the launch story.
   - Evidence: [rate_limiter.go](/Users/bytedance/code/elephant.ai/internal/delivery/channels/lark/rate_limiter.go), commit `8ae9f51b`

4. ~~Fix the leader OpenAPI spec so it only documents live routes, or implement the missing routes.~~ **CLOSED** — All spec paths are routed and implemented. Drift-detection test added.
   - Evidence: [openapi_leader.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/openapi_leader.go), [router.go](/Users/bytedance/code/elephant.ai/internal/delivery/server/http/router.go)

5. Write a minimal production ops package:
   - rollback procedure
   - monitoring / alert thresholds
   - leader-specific troubleshooting flow

## Recommended Pre-Launch Exit Criteria

- `Auth`: public leader routes and server routes protected appropriately.
- `Debug isolation`: debug server restricted to trusted network or authenticated.
- `Rate limiting`: both HTTP ingress and Lark notification egress enforced in live paths.
- `Docs accuracy`: README, config docs, and OpenAPI match the actual shipped behavior.
- `Ops`: rollback and monitoring-alert runbooks written and reviewed.
- `Performance`: at least one leader-specific load pass completed for scheduler + notification bursts.
