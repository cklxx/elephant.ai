# Agent 自主测试与迭代方案 — Lark 完整体验

> Status: **P1-P5 All Done**
> Created: 2026-02-01
> Updated: 2026-02-01 19:00
> Author: cklxx + Claude

## Progress

### P1: LarkMessenger Interface Extraction [DONE]
- **commit**: `b60ba45a` — Extract LarkMessenger interface for testable outbound Lark API calls
- `messenger.go`: 7-method LarkMessenger interface
- `sdk_messenger.go`: production implementation wrapping `*lark.Client`
- `recording_messenger.go`: test double with call capture, configurable responses
- Gateway refactored: all SDK calls route through messenger field
- `chat_context.go`: `fetchRecentChatMessages` now a Gateway method using messenger
- `InjectMessage()`: public entry point for scenario tests, bypasses WebSocket
- 16 new unit tests (RecordingMessenger + InjectMessage)
- All existing 55 tests continue to pass

### P2: YAML Scenario Framework [DONE]
- **commit**: `d3ee065f` — Add YAML scenario test framework with 5 initial scenarios
- `internal/channels/lark/testing/scenario.go`: YAML schema + loader
- `internal/channels/lark/testing/runner.go`: ScenarioRunner with single-gateway lifecycle,
  multi-turn mock executor
- `internal/channels/lark/testing/assertions.go`: deterministic assertion engine
  (messenger calls, executor, no-call, timing)
- 5 initial scenarios: basic_p2p, basic_group, dedup, reset_command, error_handling
- 23 assertion unit tests

### P3: Full Scenario Coverage [DONE]
- **commit**: `e2c173a4` — Add 6 more scenarios for complete 11-category coverage
- emoji_reactions, thinking_fallback, empty_message_filter, plan_review,
  group_not_allowed, direct_not_allowed
- **11 total scenarios** covering all identified Lark gateway functional paths

### P4: Self-Test Skill + Iteration Loop [DONE]
- **commit**: `0395f58f` — Add self-test skill, JSON/Markdown report generation, and fix classification
- `skills/self-test/SKILL.md`: self-test skill with 6-phase workflow
  (execute → analyze → classify → fix-tier → iterate → report)
- `internal/channels/lark/testing/report.go`: TestReport, ReportSummary, ScenarioReport types
  with JSON/Markdown output, fix tier classification (1-4), allowed file patterns per tier
- `internal/channels/lark/testing/report_test.go`: tests for BuildReport, ToJSON, ToMarkdown,
  ClassifyFixTier, FilesForTier
- `runner.go`: added `RunAll(ctx, scenarioDir)` for batch execution

### P5: Conversation Recorder + Scenario Mining [DONE]
- **commit**: `e17c16d1` — Add ConversationRecorder and Trace-to-Scenario converter
- `internal/channels/lark/testing/recorder.go`:
  - `ConversationTrace`: thread-safe trace buffer (Append/Entries/Reset)
  - `TracingMessenger`: LarkMessenger decorator that records all outbound calls
  - `RecordingEventListener`: EventListener decorator that records domain events
- `internal/channels/lark/testing/converter.go`:
  - `TraceToScenario()`: converts recorded traces to Scenario with ID anonymization
  - `ScenarioToYAML()`: serializes scenarios to YAML
  - `AnonymizeID()`: deterministic hash-based ID anonymization
  - `extractKeywords()` / `deduplicateAssertions()`: smart assertion generation
- `internal/channels/lark/testing/recorder_test.go`: trace, tracing messenger, event listener tests
- `internal/channels/lark/testing/converter_test.go`: 12 tests covering TraceToScenario
  (basic, anonymization, multi-turn, orphaned outbound, assertions, mock responses),
  ScenarioToYAML, AnonymizeID, extractTextFromContent, extractKeywords, deduplicateAssertions

## Architecture

```
Production conversation recording:
    TracingMessenger → ConversationTrace → TraceToScenario → YAML
    RecordingEventListener ↗

tests/scenarios/lark/*.yaml          ← 11 declarative scenarios
    ↓
internal/channels/lark/testing/      ← Runner + Assertions + Reports
    ↓
internal/channels/lark/              ← Gateway + LarkMessenger
    messenger.go                     ← interface
    sdk_messenger.go                 ← production (real SDK)
    recording_messenger.go           ← test double
    gateway.go                       ← InjectMessage() entry point

skills/self-test/SKILL.md            ← Agent self-test workflow
```

## File Map

| File | Purpose |
|---|---|
| `internal/channels/lark/messenger.go` | `LarkMessenger` interface (7 methods) |
| `internal/channels/lark/sdk_messenger.go` | Production SDK wrapper |
| `internal/channels/lark/recording_messenger.go` | Test recording messenger |
| `internal/channels/lark/recording_messenger_test.go` | RecordingMessenger unit tests |
| `internal/channels/lark/inject_message_test.go` | InjectMessage integration tests |
| `internal/channels/lark/testing/scenario.go` | YAML schema + loader |
| `internal/channels/lark/testing/runner.go` | ScenarioRunner + RunAll + multiMockExecutor |
| `internal/channels/lark/testing/assertions.go` | Assertion engine |
| `internal/channels/lark/testing/assertions_test.go` | Assertion unit tests |
| `internal/channels/lark/testing/report.go` | TestReport + fix tier classification |
| `internal/channels/lark/testing/report_test.go` | Report unit tests |
| `internal/channels/lark/testing/recorder.go` | ConversationTrace + TracingMessenger + RecordingEventListener |
| `internal/channels/lark/testing/recorder_test.go` | Recorder unit tests |
| `internal/channels/lark/testing/converter.go` | TraceToScenario + anonymization |
| `internal/channels/lark/testing/converter_test.go` | Converter unit tests |
| `internal/channels/lark/testing/scenario_test.go` | Test entry point (loads YAML scenarios) |
| `skills/self-test/SKILL.md` | Self-test skill definition |
| `tests/scenarios/lark/*.yaml` | 11 scenario files |
