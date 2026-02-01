# Agent 自主测试与迭代方案 — Lark 完整体验

> Status: **P1-P3 Done** (P4-P5 Planned)
> Created: 2026-02-01
> Updated: 2026-02-01 17:00
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

### P4: Self-Test Skill + Iteration Loop [PLANNED]
- `skills/self-test.md` skill definition
- Auto-categorized fix tiers (test → skill → prod)
- Max 3 auto-iteration rounds
- Error experience integration

### P5: Conversation Recorder + Scenario Mining [PLANNED]
- `ConversationRecorder` wrapper
- Trace → Scenario YAML converter
- Coverage analysis integration

## Architecture

```
tests/scenarios/lark/*.yaml          ← 11 declarative scenarios
    ↓
internal/channels/lark/testing/      ← Runner + Assertions
    ↓
internal/channels/lark/              ← Gateway + LarkMessenger
    messenger.go                     ← interface
    sdk_messenger.go                 ← production (real SDK)
    recording_messenger.go           ← test double
    gateway.go                       ← InjectMessage() entry point
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
| `internal/channels/lark/testing/runner.go` | ScenarioRunner |
| `internal/channels/lark/testing/assertions.go` | Assertion engine |
| `internal/channels/lark/testing/assertions_test.go` | Assertion unit tests |
| `internal/channels/lark/testing/scenario_test.go` | Test entry point (loads YAML) |
| `tests/scenarios/lark/*.yaml` | 11 scenario files |
