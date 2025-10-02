#!/bin/bash

# ALEX SSE System Integration Test
# Tests the full stack: Server + Web + SSE

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
API_URL="${1:-http://localhost:8080}"
WEB_URL="${2:-http://localhost:3000}"
TEST_SESSION_ID="integration-test-$(date +%s)"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  ALEX SSE System Integration Test"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "API URL: $API_URL"
echo "Web URL: $WEB_URL"
echo "Test Session ID: $TEST_SESSION_ID"
echo ""

# Test counter
total_tests=0
passed_tests=0

# Test function
run_test() {
    local test_name="$1"
    local test_command="$2"

    total_tests=$((total_tests + 1))
    echo -n "Test $total_tests: $test_name... "

    if eval "$test_command" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ PASSED${NC}"
        passed_tests=$((passed_tests + 1))
        return 0
    else
        echo -e "${RED}✗ FAILED${NC}"
        return 1
    fi
}

# Detailed test with output
run_detailed_test() {
    local test_name="$1"
    local test_command="$2"

    total_tests=$((total_tests + 1))
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Test $total_tests: $test_name"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    if eval "$test_command"; then
        echo -e "${GREEN}✓ PASSED${NC}"
        passed_tests=$((passed_tests + 1))
        return 0
    else
        echo -e "${RED}✗ FAILED${NC}"
        return 1
    fi
}

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Phase 1: Service Health Checks"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Test 1: Server health
run_test "Server health check" \
    "curl -f -s $API_URL/health"

# Test 2: Web accessibility
run_test "Web frontend accessibility" \
    "curl -f -s -o /dev/null $WEB_URL"

# Test 3: CORS headers
run_test "CORS headers present" \
    "curl -s -I $API_URL/api/sessions | grep -i 'access-control-allow-origin'"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Phase 2: REST API Tests"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Test 4: List sessions
run_test "List sessions" \
    "curl -f -s $API_URL/api/sessions | jq -e '.sessions'"

# Test 5: Create task
echo ""
echo "Test 5: Create and execute task"
TASK_RESPONSE=$(curl -s -X POST $API_URL/api/tasks \
    -H "Content-Type: application/json" \
    -d "{\"task\": \"What is 2+2?\", \"session_id\": \"$TEST_SESSION_ID\"}")

if echo "$TASK_RESPONSE" | jq -e '.task_id' > /dev/null 2>&1; then
    TASK_ID=$(echo "$TASK_RESPONSE" | jq -r '.task_id')
    echo -e "${GREEN}✓ PASSED${NC} - Task ID: $TASK_ID"
    passed_tests=$((passed_tests + 1))
else
    echo -e "${RED}✗ FAILED${NC}"
fi
total_tests=$((total_tests + 1))

# Test 6: Get session details
run_test "Get session details" \
    "curl -f -s $API_URL/api/sessions/$TEST_SESSION_ID | jq -e '.session_id'"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Phase 3: SSE Connection Test"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Test 7: SSE connection
echo "Test 7: SSE event stream connection"
echo "Connecting to SSE endpoint for 5 seconds..."

SSE_OUTPUT=$(timeout 5s curl -N -H "Accept: text/event-stream" \
    "$API_URL/api/sse?session_id=$TEST_SESSION_ID" 2>/dev/null || true)

if echo "$SSE_OUTPUT" | grep -q "event:"; then
    echo -e "${GREEN}✓ PASSED${NC} - Received SSE events:"
    echo "$SSE_OUTPUT" | head -10
    passed_tests=$((passed_tests + 1))
else
    echo -e "${RED}✗ FAILED${NC} - No events received"
fi
total_tests=$((total_tests + 1))

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Phase 4: Full Workflow Test"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Test 8: Complete workflow
echo "Test 8: Complete workflow (SSE + Task execution)"
echo ""

# Start SSE listener in background
echo "Starting SSE listener..."
timeout 30s curl -N -H "Accept: text/event-stream" \
    "$API_URL/api/sse?session_id=workflow-test" > /tmp/sse-output.txt 2>&1 &
SSE_PID=$!

sleep 2

# Submit task
echo "Submitting task..."
TASK_RESP=$(curl -s -X POST $API_URL/api/tasks \
    -H "Content-Type: application/json" \
    -d '{"task": "Calculate 10+20", "session_id": "workflow-test"}')

echo "Task response: $TASK_RESP"

# Wait for SSE events
sleep 5

# Kill SSE listener
kill $SSE_PID 2>/dev/null || true

# Check if events were received
if [ -s /tmp/sse-output.txt ]; then
    EVENT_COUNT=$(grep -c "event:" /tmp/sse-output.txt || echo "0")
    echo "Received $EVENT_COUNT SSE events"

    if [ "$EVENT_COUNT" -gt 0 ]; then
        echo -e "${GREEN}✓ PASSED${NC} - Full workflow completed"
        echo ""
        echo "Sample events received:"
        cat /tmp/sse-output.txt | head -20
        passed_tests=$((passed_tests + 1))
    else
        echo -e "${RED}✗ FAILED${NC} - No events captured"
    fi
else
    echo -e "${RED}✗ FAILED${NC} - SSE listener produced no output"
fi
total_tests=$((total_tests + 1))

# Cleanup
rm -f /tmp/sse-output.txt

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Phase 5: Cleanup"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Test 9: Delete test session
run_test "Delete test session" \
    "curl -f -s -X DELETE $API_URL/api/sessions/$TEST_SESSION_ID"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Test Summary"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Total Tests: $total_tests"
echo -e "Passed: ${GREEN}$passed_tests${NC}"
echo -e "Failed: ${RED}$((total_tests - passed_tests))${NC}"
echo ""

if [ $passed_tests -eq $total_tests ]; then
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}  ✓ ALL TESTS PASSED!${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    exit 0
else
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${RED}  ✗ SOME TESTS FAILED${NC}"
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    exit 1
fi
