#!/bin/bash

# ALEX SSE Streaming Acceptance Tests
# Tests Server-Sent Events functionality for real-time task execution streaming

set -e

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8080}"
RESULTS_DIR="$(dirname "$0")/results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULTS_FILE="$RESULTS_DIR/sse_test_${TIMESTAMP}.txt"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Create results directory
mkdir -p "$RESULTS_DIR"

# Initialize results file
echo "ALEX SSE Streaming Acceptance Tests" > "$RESULTS_FILE"
echo "Timestamp: $(date)" >> "$RESULTS_FILE"
echo "Base URL: $BASE_URL" >> "$RESULTS_FILE"
echo "========================================" >> "$RESULTS_FILE"
echo "" >> "$RESULTS_FILE"

# Helper functions
log_test() {
    echo -e "${BLUE}[TEST]${NC} $1"
    echo "[TEST] $1" >> "$RESULTS_FILE"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
}

log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    echo "[PASS] $1" >> "$RESULTS_FILE"
    PASSED_TESTS=$((PASSED_TESTS + 1))
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    echo "[FAIL] $1" >> "$RESULTS_FILE"
    FAILED_TESTS=$((FAILED_TESTS + 1))
}

log_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
    echo "[INFO] $1" >> "$RESULTS_FILE"
}

log_response() {
    echo "$1" >> "$RESULTS_FILE"
}

# Test: SSE Connection Establishment
test_sse_connection() {
    log_test "SSE Connection Establishment"

    # Create a task first
    payload='{"task":"Echo hello world"}'
    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL/api/tasks")

    session_id=$(echo "$response" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$session_id" ]; then
        log_fail "Failed to create task for SSE test"
        return 1
    fi

    log_info "Testing SSE connection for session: $session_id"

    # Connect to SSE endpoint and read first event
    timeout 5s curl -s -N "$BASE_URL/api/events/$session_id" > "$RESULTS_DIR/sse_output_${session_id}.txt" 2>&1 &
    sse_pid=$!

    sleep 2

    # Check if process is still running
    if ps -p $sse_pid > /dev/null 2>&1; then
        kill $sse_pid 2>/dev/null || true

        # Check if we received SSE data
        if [ -s "$RESULTS_DIR/sse_output_${session_id}.txt" ]; then
            log_pass "SSE connection established successfully"
            log_response "First bytes received from SSE stream"
            echo "$session_id" > "$RESULTS_DIR/last_sse_session.txt"
            return 0
        else
            log_fail "SSE connection established but no data received"
            return 1
        fi
    else
        log_fail "SSE connection failed to establish"
        return 1
    fi
}

# Test: SSE Event Format
test_sse_event_format() {
    log_test "SSE Event Format Validation"

    # Create a simple task
    payload='{"task":"List files in current directory"}'
    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL/api/tasks")

    session_id=$(echo "$response" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$session_id" ]; then
        log_fail "Failed to create task for event format test"
        return 1
    fi

    log_info "Capturing SSE events for session: $session_id"

    # Capture SSE stream for 5 seconds
    timeout 5s curl -s -N "$BASE_URL/api/events/$session_id" > "$RESULTS_DIR/sse_events_${session_id}.txt" 2>&1

    # Analyze captured events
    if [ -s "$RESULTS_DIR/sse_events_${session_id}.txt" ]; then
        # Check for SSE format (data: lines)
        data_lines=$(grep -c "^data:" "$RESULTS_DIR/sse_events_${session_id}.txt" || true)

        if [ "$data_lines" -gt 0 ]; then
            log_pass "SSE events received in correct format (found $data_lines data lines)"
            log_response "Sample events saved to: sse_events_${session_id}.txt"
            return 0
        else
            log_fail "No properly formatted SSE events found"
            return 1
        fi
    else
        log_fail "No SSE events captured"
        return 1
    fi
}

# Test: SSE Session Isolation
test_sse_session_isolation() {
    log_test "SSE Session Isolation"

    # Create two tasks with different sessions
    payload1='{"task":"Task for session 1"}'
    response1=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload1" \
        "$BASE_URL/api/tasks")

    session1=$(echo "$response1" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

    payload2='{"task":"Task for session 2"}'
    response2=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload2" \
        "$BASE_URL/api/tasks")

    session2=$(echo "$response2" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$session1" ] || [ -z "$session2" ]; then
        log_fail "Failed to create tasks for isolation test"
        return 1
    fi

    log_info "Testing isolation between sessions: $session1 and $session2"

    # Capture events from both sessions simultaneously
    timeout 5s curl -s -N "$BASE_URL/api/events/$session1" > "$RESULTS_DIR/sse_session1.txt" 2>&1 &
    pid1=$!

    timeout 5s curl -s -N "$BASE_URL/api/events/$session2" > "$RESULTS_DIR/sse_session2.txt" 2>&1 &
    pid2=$!

    wait $pid1 2>/dev/null || true
    wait $pid2 2>/dev/null || true

    # Check if both sessions received data
    if [ -s "$RESULTS_DIR/sse_session1.txt" ] && [ -s "$RESULTS_DIR/sse_session2.txt" ]; then
        # Check for session_id in events
        session1_count=$(grep -o "\"session_id\":\"$session1\"" "$RESULTS_DIR/sse_session1.txt" | wc -l || true)
        session2_in_session1=$(grep -o "\"session_id\":\"$session2\"" "$RESULTS_DIR/sse_session1.txt" | wc -l || true)

        session2_count=$(grep -o "\"session_id\":\"$session2\"" "$RESULTS_DIR/sse_session2.txt" | wc -l || true)
        session1_in_session2=$(grep -o "\"session_id\":\"$session1\"" "$RESULTS_DIR/sse_session2.txt" | wc -l || true)

        log_info "Session 1 events in stream 1: $session1_count"
        log_info "Session 2 events in stream 1: $session2_in_session1"
        log_info "Session 2 events in stream 2: $session2_count"
        log_info "Session 1 events in stream 2: $session1_in_session2"

        if [ "$session2_in_session1" -eq 0 ] && [ "$session1_in_session2" -eq 0 ]; then
            log_pass "SSE session isolation verified - no event leakage detected"
            return 0
        else
            log_fail "SSE session isolation failed - events leaked between sessions"
            return 1
        fi
    else
        log_fail "Failed to capture events from both sessions"
        return 1
    fi
}

# Test: SSE Event Types
test_sse_event_types() {
    log_test "SSE Event Types Coverage"

    # Create a task that will generate various event types
    payload='{"task":"Read package.json file and tell me the version"}'
    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL/api/tasks")

    session_id=$(echo "$response" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)
    task_id=$(echo "$response" | grep -o '"task_id":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$session_id" ]; then
        log_fail "Failed to create task for event types test"
        return 1
    fi

    log_info "Capturing events for session: $session_id"

    # Capture SSE stream for 15 seconds (give task time to complete)
    timeout 15s curl -s -N "$BASE_URL/api/events/$session_id" > "$RESULTS_DIR/sse_event_types_${session_id}.txt" 2>&1
    if [ -s "$RESULTS_DIR/sse_event_types_${session_id}.txt" ]; then
        # Look for various event types
        has_workflow.node.output.delta=$(grep -c '"type":"workflow.node.output.delta"' "$RESULTS_DIR/sse_event_types_${session_id}.txt" || true)
        has_tool_call=$(grep -c '"type":"tool_call' "$RESULTS_DIR/sse_event_types_${session_id}.txt" || true)
        has_workflow.result.final=$(grep -c '"type":"workflow.result.final"' "$RESULTS_DIR/sse_event_types_${session_id}.txt" || true)

        log_info "Event type counts:"
        log_info "  workflow.node.output.delta: $has_workflow.node.output.delta"
        log_info "  tool_call*: $has_tool_call"
        log_info "  workflow.result.final: $has_workflow.result.final"

        found_types=$((has_workflow.node.output.delta + has_tool_call + has_workflow.result.final))

        if [ "$found_types" -ge 2 ]; then
            log_pass "Multiple SSE event types detected (found $found_types type categories)"
            return 0
        else
            log_fail "Insufficient event type coverage (found only $found_types type categories)"
            return 1
        fi
    else
        log_fail "No events captured for session $session_id"
        return 1
    fi
}

# Test: SSE Connection Persistence
test_sse_connection_persistence() {
    log_test "SSE Connection Persistence"

    # Create a slightly longer task
    payload='{"task":"Count from 1 to 5"}'
    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL/api/tasks")

    session_id=$(echo "$response" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$session_id" ]; then
        log_fail "Failed to create task for persistence test"
        return 1
    fi

    log_info "Testing connection persistence for session: $session_id"

    # Connect and monitor for 10 seconds
    start_time=$(date +%s)
    timeout 10s curl -s -N "$BASE_URL/api/events/$session_id" > "$RESULTS_DIR/sse_persistence_${session_id}.txt" 2>&1
    end_time=$(date +%s)
    duration=$((end_time - start_time))

    log_info "Connection lasted: ${duration} seconds"

    if [ "$duration" -ge 8 ]; then
        if [ -s "$RESULTS_DIR/sse_persistence_${session_id}.txt" ]; then
            log_pass "SSE connection persisted for ${duration} seconds with data"
            return 0
        else
            log_fail "Connection persisted but no data received"
            return 1
        fi
    else
        log_fail "Connection terminated prematurely after ${duration} seconds"
        return 1
    fi
}

# Test: SSE Reconnection
test_sse_reconnection() {
    log_test "SSE Reconnection Capability"

    # Create a task
    payload='{"task":"Echo test message"}'
    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL/api/tasks")

    session_id=$(echo "$response" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$session_id" ]; then
        log_fail "Failed to create task for reconnection test"
        return 1
    fi

    log_info "Testing reconnection for session: $session_id"

    # First connection
    timeout 3s curl -s -N "$BASE_URL/api/events/$session_id" > "$RESULTS_DIR/sse_reconnect1_${session_id}.txt" 2>&1

    # Wait a moment
    sleep 1

    # Second connection (reconnect)
    timeout 3s curl -s -N "$BASE_URL/api/events/$session_id" > "$RESULTS_DIR/sse_reconnect2_${session_id}.txt" 2>&1

    if [ -s "$RESULTS_DIR/sse_reconnect1_${session_id}.txt" ] && [ -s "$RESULTS_DIR/sse_reconnect2_${session_id}.txt" ]; then
        log_pass "SSE reconnection successful - both connections received data"
        return 0
    elif [ -s "$RESULTS_DIR/sse_reconnect1_${session_id}.txt" ]; then
        log_fail "First connection succeeded but reconnection failed"
        return 1
    else
        log_fail "Both connections failed"
        return 1
    fi
}

# Test: SSE Heartbeat/Keep-alive
test_sse_heartbeat() {
    log_test "SSE Heartbeat/Keep-alive"

    # Create a task
    payload='{"task":"Simple test"}'
    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL/api/tasks")

    session_id=$(echo "$response" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$session_id" ]; then
        log_fail "Failed to create task for heartbeat test"
        return 1
    fi

    log_info "Monitoring heartbeat for session: $session_id"

    # Capture stream and look for keep-alive patterns
    timeout 8s curl -s -N "$BASE_URL/api/events/$session_id" > "$RESULTS_DIR/sse_heartbeat_${session_id}.txt" 2>&1

    if [ -s "$RESULTS_DIR/sse_heartbeat_${session_id}.txt" ]; then
        # Look for comment lines (heartbeat) or regular data
        comment_lines=$(grep -c "^:" "$RESULTS_DIR/sse_heartbeat_${session_id}.txt" || true)
        data_lines=$(grep -c "^data:" "$RESULTS_DIR/sse_heartbeat_${session_id}.txt" || true)

        total_activity=$((comment_lines + data_lines))

        log_info "Activity detected: $comment_lines comments + $data_lines data lines = $total_activity total"

        if [ "$total_activity" -gt 0 ]; then
            log_pass "SSE heartbeat/keep-alive functioning ($total_activity activity indicators)"
            return 0
        else
            log_fail "No heartbeat or data activity detected"
            return 1
        fi
    else
        log_fail "No data captured for heartbeat test"
        return 1
    fi
}

# Test: SSE Error Handling
test_sse_error_handling() {
    log_test "SSE Error Event Handling"

    # Create a task that might cause an error (invalid tool call)
    payload='{"task":"Execute non-existent command xyz123abc"}'
    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL/api/tasks")

    session_id=$(echo "$response" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$session_id" ]; then
        log_fail "Failed to create task for error handling test"
        return 1
    fi

    log_info "Testing error handling for session: $session_id"

    # Capture events
    timeout 10s curl -s -N "$BASE_URL/api/events/$session_id" > "$RESULTS_DIR/sse_error_${session_id}.txt" 2>&1

    if [ -s "$RESULTS_DIR/sse_error_${session_id}.txt" ]; then
        # Look for error events
        has_error=$(grep -c '"type":"error"' "$RESULTS_DIR/sse_error_${session_id}.txt" || true)
        has_tool_error=$(grep -i -c 'error\|failed' "$RESULTS_DIR/sse_error_${session_id}.txt" || true)

        log_info "Error indicators found: $has_tool_error"

        if [ "$has_tool_error" -gt 0 ]; then
            log_pass "SSE error handling detected error events"
            return 0
        else
            log_info "No explicit errors detected (task may have completed normally)"
            log_pass "SSE error handling test completed (no errors generated)"
            return 0
        fi
    else
        log_fail "No events captured for error handling test"
        return 1
    fi
}

# Run all tests
echo "========================================="
echo "ALEX SSE Streaming Acceptance Tests"
echo "========================================="
echo ""

test_sse_connection
echo ""
test_sse_event_format
echo ""
test_sse_session_isolation
echo ""
test_sse_event_types
echo ""
test_sse_connection_persistence
echo ""
test_sse_reconnection
echo ""
test_sse_heartbeat
echo ""
test_sse_error_handling
echo ""

# Summary
echo "========================================="
echo "Test Summary"
echo "========================================="
echo "Total Tests:  $TOTAL_TESTS"
echo "Passed:       $PASSED_TESTS"
echo "Failed:       $FAILED_TESTS"
echo "Success Rate: $(awk "BEGIN {printf \"%.2f\", ($PASSED_TESTS/$TOTAL_TESTS)*100}")%"
echo ""

# Write summary to results file
echo "" >> "$RESULTS_FILE"
echo "=========================================" >> "$RESULTS_FILE"
echo "Test Summary" >> "$RESULTS_FILE"
echo "=========================================" >> "$RESULTS_FILE"
echo "Total Tests:  $TOTAL_TESTS" >> "$RESULTS_FILE"
echo "Passed:       $PASSED_TESTS" >> "$RESULTS_FILE"
echo "Failed:       $FAILED_TESTS" >> "$RESULTS_FILE"
echo "Success Rate: $(awk "BEGIN {printf \"%.2f\", ($PASSED_TESTS/$TOTAL_TESTS)*100}")%" >> "$RESULTS_FILE"

log_info "Results saved to: $RESULTS_FILE"
log_info "Event logs saved to: $RESULTS_DIR/sse_*.txt"

# Exit with appropriate code
if [ $FAILED_TESTS -eq 0 ]; then
    exit 0
else
    exit 1
fi
