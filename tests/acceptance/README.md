# ALEX Acceptance Test Suite

Comprehensive acceptance tests for ALEX backend server and frontend integration.

## Overview

This test suite validates:
- **Backend API** - All REST endpoints and CRUD operations
- **SSE Streaming** - Real-time event delivery and session isolation
- **Integration** - End-to-end workflows and system behavior

## Test Structure

```
tests/acceptance/
├── api_test.sh              # Backend API acceptance tests (17 tests)
├── sse_test.sh              # SSE streaming tests (8 tests)
├── integration_test.sh      # Integration & E2E tests (10 tests)
├── run_all_tests.sh         # Master test runner
├── results/                 # Test execution results
│   ├── api_test_*.txt
│   ├── sse_test_*.txt
│   ├── integration_test_*.txt
│   └── test_summary_*.txt
└── README.md                # This file
```

## Prerequisites

### 1. Server Running

Start the ALEX server before running tests:

```bash
# From project root
./alex-server
```

Or specify custom URL:

```bash
export BASE_URL=http://your-server:port
```

### 2. Environment Variables

Ensure runtime configuration exists at `~/.alex/config.yaml` (see `examples/config/runtime-config.yaml`).
If you reference `${OPENAI_API_KEY}`, export it:

```bash
export OPENAI_API_KEY=your-key
```

### 3. Dependencies

Tests use standard Unix tools:
- `curl` - HTTP requests
- `grep` - Text processing
- `awk` - Calculations
- `jq` - JSON parsing (optional, for manual testing)

## Running Tests

### Run All Tests

Execute the complete test suite:

```bash
cd tests/acceptance
./run_all_tests.sh
```

### Run Individual Suites

Run specific test suites:

```bash
# Backend API tests only
./api_test.sh

# SSE streaming tests only
./sse_test.sh

# Integration tests only
./integration_test.sh
```

### Custom Base URL

Test against a different server:

```bash
BASE_URL=http://staging-server:8080 ./run_all_tests.sh
```

## Test Coverage

### Backend API Tests (17 tests)

| Test | Endpoint | Validates |
|------|----------|-----------|
| Health Check | GET /health | Server is running |
| Create Task | POST /api/tasks | Task creation |
| Create Task w/ Session | POST /api/tasks | Session continuity |
| Get Task Status | GET /api/tasks/:id | Task retrieval |
| List Tasks | GET /api/tasks | Task listing |
| Pagination | GET /api/tasks?limit&offset | Pagination params |
| Cancel Task | POST /api/tasks/:id/cancel | Task cancellation |
| Session Creation | Implicit via tasks | Session generation |
| Get Session | GET /api/sessions/:id | Session retrieval |
| List Sessions | GET /api/sessions | Session listing |
| Fork Session | POST /api/sessions/:id/fork | Session forking |
| Delete Session | DELETE /api/sessions/:id | Session deletion |
| Invalid Task | POST /api/tasks | Error handling (empty task) |
| Invalid Task ID | GET /api/tasks/invalid | Error handling (404) |
| Invalid Session ID | GET /api/sessions/invalid | Error handling (404) |

### SSE Streaming Tests (8 tests)

| Test | Validates |
|------|-----------|
| Connection Establishment | SSE connection opens |
| Event Format | Proper SSE data format |
| Session Isolation | Events per session only |
| Event Types | All event types received |
| Connection Persistence | Long-lived connections |
| Reconnection | Reconnect capability |
| Heartbeat | Keep-alive signals |
| Error Handling | Error event propagation |

### Integration Tests (10 tests)

| Test | Validates |
|------|-----------|
| E2E Task Execution | Complete task workflow |
| Multi-step Workflow | Session state continuity |
| Concurrent Sessions | Session isolation under load |
| Session Persistence | Data durability |
| Task Lifecycle | Status transitions |
| Session Fork | Fork workflow |
| Error Recovery | Graceful error handling |
| Pagination | List pagination |
| Agent Preset | Preset application |
| Performance | Response time benchmarks |

## Understanding Results

### Exit Codes

- `0` - All tests passed
- `1` - One or more tests failed

### Output Format

Tests produce color-coded output:

- 🔵 `[TEST]` - Test starting
- ✅ `[PASS]` - Test passed
- ❌ `[FAIL]` - Test failed
- ⚠️ `[INFO]` - Informational message

### Result Files

Each test suite generates a detailed result file:

```
results/api_test_YYYYMMDD_HHMMSS.txt
```

The master runner creates a summary:

```
results/test_summary_YYYYMMDD_HHMMSS.txt
```

### Sample Output

```bash
$ ./run_all_tests.sh

========================================
  ALEX Acceptance Test Suite Runner
========================================

Base URL: http://localhost:8080
Results Directory: /path/to/results
Timestamp: Fri Oct  3 22:58:08 CST 2025

[CHECK] Verifying server is running...
[OK] Server is healthy and responsive

================================================
Running: Backend API Tests
================================================

[TEST] Health Check Endpoint
[PASS] Health check returned 200 OK with correct status

[TEST] Create Task - Basic
[PASS] Task created successfully: task_id=abc123

...

========================================
  Test Execution Summary
========================================

✓ Backend API Tests - PASSED (15s)
✓ SSE Streaming Tests - PASSED (25s)
✓ Integration & E2E Tests - PASSED (40s)

Total Suites: 3
Passed:       3
Failed:       0
Success Rate: 100.00%

════════════════════════════════════════
  ✓ ALL ACCEPTANCE TESTS PASSED  ✓
════════════════════════════════════════
```

## Troubleshooting

### Server Not Running

```
[ERROR] Server is not responding at http://localhost:8080
```

**Solution**: Start the server with `./alex-server`

### Tests Failing

Check the detailed result files in `results/` directory:

```bash
cat results/api_test_*.txt
```

Common issues:
- LLM authentication failures (check `.env`)
- Empty task_id or session_id (server bug)
- Timeout errors (server overloaded)

### Permission Denied

```
bash: ./api_test.sh: Permission denied
```

**Solution**: Make scripts executable:

```bash
chmod +x *.sh
```

## Test Development

### Adding New Tests

1. Choose the appropriate test suite file
2. Add test function following naming convention: `test_description()`
3. Use helper functions: `log_test()`, `log_pass()`, `log_fail()`, `log_info()`
4. Call new test in main execution section

Example:

```bash
test_my_new_feature() {
    log_test "My New Feature Test"

    response=$(curl -s "$BASE_URL/api/endpoint")

    if [ ... ]; then
        log_pass "Feature works correctly"
        return 0
    else
        log_fail "Feature failed"
        return 1
    fi
}

# Add to execution section
test_my_new_feature
```

### Test Best Practices

1. **Idempotent**: Tests should be repeatable without side effects
2. **Isolated**: Each test should be independent
3. **Clear**: Use descriptive test names and log messages
4. **Fast**: Keep individual tests under 10 seconds when possible
5. **Cleanup**: Clean up test data when needed

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Acceptance Tests

on: [push, pull_request]

jobs:
  acceptance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2

      - name: Start Server
        run: |
          ./alex-server &
          sleep 5
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}

      - name: Run Tests
        run: |
          cd tests/acceptance
          ./run_all_tests.sh

      - name: Upload Results
        if: always()
        uses: actions/upload-artifact@v2
        with:
          name: test-results
          path: tests/acceptance/results/
```

## Maintenance

### Regular Tasks

- Review test coverage monthly
- Update tests when API changes
- Archive old result files quarterly
- Benchmark performance quarterly

### Version History

- **v1.0** (Oct 2025) - Initial comprehensive test suite
  - 35 test cases across 3 suites
  - Full API coverage
  - SSE streaming validation
  - Integration workflows

## Support

For issues or questions:
- Review detailed results in `results/` directory
- Check server logs in `logs/alex-server.log`
- Refer to main project `ACCEPTANCE_TEST_RESULTS.md`
- Consult `ACCEPTANCE_TEST_PLAN.md` for test strategy

---

**Last Updated**: October 3, 2025
**Test Framework Version**: 1.0
