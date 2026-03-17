# Agent Self-Verification Workflow

Updated: 2026-03-17

A structured protocol for coding agents to verify their own work, debug failures, and confirm feature correctness before delivery.

See also: [Engineering Workflow](engineering-workflow.md) | [Integration Testing](integration-testing.md) | [Agents Teams Testing](agents-teams-testing.md)

---

## 1. Core Verification Loop

Every implementation task follows this loop:

```
PRE-CHECK → IMPLEMENT → VERIFY → REGRESSION → DELIVER
```

### 1.1 Pre-Implementation Checklist

Before writing any code:

```bash
# 1. Understand the scope — read the target files
cat <target_file>

# 2. Capture baseline state
make dev-test 2>&1 | tail -20    # record pass/fail counts
make dev-lint 2>&1 | tail -10    # record lint status

# 3. Identify existing test files
find . -name '*_test.go' -path '*/<package>/*'

# 4. Check for related eval cases
grep -r '<feature_keyword>' evaluation/agent_eval/datasets/
```

Capture these baselines before any edit. You need them to confirm no regressions.

### 1.2 Implementation Verification Steps

After each meaningful change:

```bash
# Compile check — fast feedback
go build ./...

# Run targeted tests for the package you changed
go test -v ./<changed_package>/...

# Run lint on changed files
alex dev lint
```

Do not batch all verification to the end. Verify incrementally.

### 1.3 Post-Implementation Validation

After all changes are complete:

```bash
# Full test suite
make dev-test

# Full lint
make dev-lint

# Compare against baseline — pass count should be >= baseline
```

### 1.4 Regression Check Protocol

Compare post-implementation results against the pre-implementation baseline:

| Check | Criteria |
|-------|----------|
| Test count | Must be >= baseline (new tests added, none removed) |
| Pass rate | Must be 100% for previously passing tests |
| Lint errors | Must be <= baseline (no new violations) |
| Build | Must succeed with zero errors |

If any regression is detected, stop delivery and enter the Debug Protocol (§3).

---

## 2. Feature Verification Protocol

### 2.1 Unit Tests

**When to write:** Every logic change requires a unit test. Skip only for pure config/doc changes.

**How to write:**

```go
func TestFeatureName_Scenario(t *testing.T) {
    // Arrange — set up inputs and expected outputs
    input := &domain.Request{Intent: "test-intent"}
    want := &domain.Response{Action: "expected-action"}

    // Act — call the function under test
    got, err := handler.Process(input)

    // Assert — verify results
    require.NoError(t, err)
    assert.Equal(t, want.Action, got.Action)
}
```

Rules:
- One test function per behavior, not per method.
- Name pattern: `Test<Unit>_<Scenario>`.
- Use `testify/require` for fatal checks, `testify/assert` for soft checks.
- Use mocks only in unit tests. Integration tests use real dependencies.

### 2.2 Integration Tests (Inject API)

For features that touch the conversation pipeline, use the inject API:

```bash
#!/usr/bin/env bash
set -euo pipefail

INJECT_URL="${INJECT_URL:-http://localhost:9090/api/dev/inject}"
TIMEOUT="${TIMEOUT:-360}"

inject() {
  local chat_id="$1" message="$2"
  curl -s -X POST "$INJECT_URL" \
    -H 'Content-Type: application/json' \
    -d "$(jq -n \
      --arg cid "$chat_id" \
      --arg msg "$message" \
      '{chat_id: $cid, sender_id: "test-agent", message: $msg}'
    )"
}

# Test case: feature behavior
tc_1_feature_name() {
  local resp
  resp=$(inject "test-chat-001" "trigger message for the feature")

  # Layer 1: HTTP success
  echo "$resp" | jq -e '.code == 0' || { echo "FAIL: bad response code"; return 1; }

  # Layer 2: Expected content
  echo "$resp" | jq -e '.data.reply | test("expected_keyword")' \
    || { echo "FAIL: missing expected keyword"; return 1; }

  echo "PASS: tc_1_feature_name"
}

tc_1_feature_name
```

Place integration test scripts at `scripts/test/<feature>-e2e.sh`.

### 2.3 E2E Verification for Behavioral Changes

For changes to agent behavior (routing, intent, tool selection):

1. Add or update eval cases in `evaluation/agent_eval/datasets/foundation_eval_cases.yaml`.
2. Run the foundation eval to confirm routing correctness.
3. Verify no existing eval case regresses.

### 2.4 Eval Case Definitions

Define expected behaviors in YAML:

```yaml
# evaluation/agent_eval/datasets/foundation_eval_cases.yaml
scenarios:
  - id: intent-new-feature-keyword
    category: execution
    intent: "user message that should trigger the new feature"
    expected_tools:
      - "tool_name_expected"

  - id: intent-new-feature-edge-case
    category: execution
    intent: "edge case message that should still route correctly"
    expected_tools:
      - "tool_name_expected"
```

Rules:
- One scenario per distinct routing path.
- `id` pattern: `intent-<category>-<short-description>`.
- `expected_tools` is the minimum set; order does not matter.
- Add negative cases where a tool should NOT be selected.

---

## 3. Debug Protocol

When tests fail or behavior is unexpected, follow this structured flow:

```
REPRODUCE → ISOLATE → TRACE → FIX → VERIFY
```

### 3.1 Reproduce

Make the failure deterministic and repeatable:

```bash
# Run the specific failing test in verbose mode
go test -v -run 'TestName' ./<package>/...

# For inject-based failures, replay the exact payload
curl -s -X POST http://localhost:9090/api/dev/inject \
  -H 'Content-Type: application/json' \
  -d '{"chat_id":"debug-001","sender_id":"test","message":"exact trigger"}'
```

If the failure is intermittent, run the test multiple times:

```bash
go test -v -count=5 -run 'TestName' ./<package>/...
```

### 3.2 Isolate

Narrow down the failure surface:

```bash
# Check if the failure is in your changes
git stash && go test -v -run 'TestName' ./<package>/... && git stash pop

# If the test passes without your changes, the regression is yours
# If it still fails, the issue predates your work
```

### 3.3 Trace

Follow the data path from input to failure:

```bash
# Add targeted debug output (remove before commit)
go test -v -run 'TestName' ./<package>/... 2>&1 | grep -E '(FAIL|ERROR|panic)'

# Check logs for runtime failures
tail -100 /tmp/alex-dev.log | grep -i error

# For routing issues, check the intent resolution path
grep -rn 'func.*Route\|func.*Dispatch\|func.*Resolve' internal/
```

### 3.4 Bisection Strategy

When the root cause is unclear across multiple changes:

```bash
# List your commits since branching
git log --oneline main..HEAD

# Test at midpoint
git stash
git checkout <midpoint_sha>
go test -v -run 'TestName' ./<package>/...
git checkout -    # return to branch
git stash pop
```

Binary search until you find the first breaking commit.

### 3.5 Common Failure Modes

| Symptom | Likely cause | Remediation |
|---------|-------------|-------------|
| `undefined` in test output | Missing struct field or nil pointer | Check struct initialization, add nil guards at boundaries |
| Test timeout | Blocking channel or HTTP call | Add context with deadline, check for missing goroutine cleanup |
| `interface conversion` panic | Wrong type assertion | Use comma-ok pattern: `v, ok := x.(Type)` |
| Inject returns empty reply | Server not running or wrong port | `curl http://localhost:9090/health` to verify |
| Eval case mismatch | Intent routing changed | Review router/dispatcher changes, update eval cases |
| Race condition (`-race` flag) | Shared state without synchronization | Use mutex or channel; on macOS add `CGO_ENABLED=0` |
| Lint failure on unchanged file | Pre-existing violation surfaced by import change | Fix it if trivial, otherwise note as follow-up |

### 3.6 Fix and Verify

After identifying the root cause:

1. Write a test that captures the bug (red).
2. Apply the minimal fix (green).
3. Run the full verification loop (§1.3, §1.4).
4. Confirm the fix does not introduce new failures.

---

## 4. Deliverable Contracts

### 4.1 Required Artifacts

| Change type | Required artifacts |
|-------------|-------------------|
| Logic change | Unit test(s) covering new/changed behavior |
| Pipeline change | Integration test script in `scripts/test/` |
| Routing/intent change | Updated eval cases in `foundation_eval_cases.yaml` |
| Config change | Example in YAML format |
| Bug fix | Regression test that fails without the fix |

### 4.2 Definition of Done

A feature is done when ALL of the following are true:

- [ ] `make dev-test` passes with no regressions.
- [ ] `make dev-lint` passes with no new violations.
- [ ] New tests cover the changed behavior.
- [ ] Eval cases updated if routing/intent changed.
- [ ] Code review (`python3 skills/code-review/run.py review`) shows no P0/P1 findings.
- [ ] Commit message describes the change clearly.

### 4.3 Documentation Requirements

- New features: update relevant guide in `docs/guides/` or add a section.
- New config options: add YAML examples in the guide or config reference.
- New eval patterns: document the scenario rationale in a comment above the case.

---

## 5. Self-Check Checklist

Quick reference — run through this before every commit.

### Pre-Commit Gate

```bash
# 1. Build
go build ./...

# 2. Test
make dev-test

# 3. Lint
make dev-lint

# 4. Code review
python3 skills/code-review/run.py review
```

### Pass Criteria

| Step | Must pass |
|------|-----------|
| Build | Zero errors |
| Test | All tests green, count >= baseline |
| Lint | No new violations |
| Review | No P0 or P1 findings |

### Final Checks

- [ ] No debug prints or temporary code left in.
- [ ] No commented-out code added.
- [ ] No unrelated files changed.
- [ ] Test names follow `Test<Unit>_<Scenario>` convention.
- [ ] YAML config examples are valid YAML.
- [ ] Commit is scoped to the task — one logical change per commit.
