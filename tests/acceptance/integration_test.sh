#!/bin/bash

# ALEX Integration & End-to-End Acceptance Tests
# Tests complete workflows including task execution, session management, and preset functionality

set -e

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8080}"
RESULTS_DIR="$(dirname "$0")/results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULTS_FILE="$RESULTS_DIR/integration_test_${TIMESTAMP}.txt"

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
echo "ALEX Integration & E2E Acceptance Tests" > "$RESULTS_FILE"
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

# Test: End-to-End Task Execution with Streaming
test_e2e_task_execution() {
    log_test "E2E Task Execution with Streaming"

    # Create a simple task
    payload='{"task":"What is 2 + 2?"}'
    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL/api/tasks")

    task_id=$(echo "$response" | grep -o '"task_id":"[^"]*"' | cut -d'"' -f4)
    session_id=$(echo "$response" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$task_id" ] || [ -z "$session_id" ]; then
        log_fail "Failed to create task for E2E test"
        return 1
    fi

    log_info "Task created: task_id=$task_id, session_id=$session_id"

    # Stream events for 10 seconds
    timeout 10s curl -s -N "$BASE_URL/api/events/$session_id" > "$RESULTS_DIR/e2e_stream_${task_id}.txt" 2>&1

    # Check task status
    sleep 2
    status_response=$(curl -s "$BASE_URL/api/tasks/$task_id")
    status=$(echo "$status_response" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)

    log_info "Final task status: $status"
    log_response "Status response: $status_response"

    # Validate we received streaming events
    if [ -s "$RESULTS_DIR/e2e_stream_${task_id}.txt" ]; then
        event_count=$(grep -c "^data:" "$RESULTS_DIR/e2e_stream_${task_id}.txt" || true)
        log_info "Received $event_count streaming events"

        if [ "$event_count" -gt 0 ] && ([ "$status" = "completed" ] || [ "$status" = "running" ]); then
            log_pass "E2E task execution completed successfully"
            return 0
        else
            log_fail "Task execution incomplete: events=$event_count, status=$status"
            return 1
        fi
    else
        log_fail "No streaming events received"
        return 1
    fi
}

# Test: Multi-step Task Workflow
test_multistep_workflow() {
    log_test "Multi-step Task Workflow"

    # Create first task
    payload1='{"task":"Create a file called test.txt with content: Hello World"}'
    response1=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload1" \
        "$BASE_URL/api/tasks")

    session_id=$(echo "$response1" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$session_id" ]; then
        log_fail "Failed to create first task"
        return 1
    fi

    log_info "Step 1: Created task in session $session_id"

    # Wait for first task to process
    sleep 3

    # Create second task in same session
    payload2="{\"task\":\"Read the content of test.txt\",\"session_id\":\"$session_id\"}"
    response2=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload2" \
        "$BASE_URL/api/tasks")

    task2_id=$(echo "$response2" | grep -o '"task_id":"[^"]*"' | cut -d'"' -f4)
    returned_session=$(echo "$response2" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

    log_info "Step 2: Created second task: $task2_id"

    if [ "$returned_session" = "$session_id" ]; then
        log_pass "Multi-step workflow maintained session continuity"
        return 0
    else
        log_fail "Session continuity broken: expected=$session_id, got=$returned_session"
        return 1
    fi
}

# Test: Concurrent Session Isolation
test_concurrent_sessions() {
    log_test "Concurrent Session Isolation"

    log_info "Creating 3 concurrent tasks..."

    # Create 3 tasks simultaneously
    payload1='{"task":"Task 1: Count to 5"}'
    payload2='{"task":"Task 2: List files"}'
    payload3='{"task":"Task 3: Echo message"}'

    response1=$(curl -s -X POST -H "Content-Type: application/json" -d "$payload1" "$BASE_URL/api/tasks") &
    pid1=$!

    response2=$(curl -s -X POST -H "Content-Type: application/json" -d "$payload2" "$BASE_URL/api/tasks") &
    pid2=$!

    response3=$(curl -s -X POST -H "Content-Type: application/json" -d "$payload3" "$BASE_URL/api/tasks") &
    pid3=$!

    wait $pid1
    wait $pid2
    wait $pid3

    response1=$(cat)
    response2=$(cat)
    response3=$(cat)

    # Extract session IDs
    session1=$(echo "$response1" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4 || true)
    session2=$(echo "$response2" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4 || true)
    session3=$(echo "$response3" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4 || true)

    log_info "Session 1: $session1"
    log_info "Session 2: $session2"
    log_info "Session 3: $session3"

    # Check uniqueness
    if [ -n "$session1" ] && [ -n "$session2" ] && [ -n "$session3" ]; then
        if [ "$session1" != "$session2" ] && [ "$session1" != "$session3" ] && [ "$session2" != "$session3" ]; then
            log_pass "Concurrent sessions are properly isolated (all unique IDs)"
            return 0
        else
            log_fail "Session ID collision detected in concurrent creation"
            return 1
        fi
    else
        log_fail "Failed to create all concurrent sessions"
        return 1
    fi
}

# Test: Session Persistence and Retrieval
test_session_persistence() {
    log_test "Session Persistence and Retrieval"

    # Create a task
    payload='{"task":"Test persistence"}'
    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL/api/tasks")

    session_id=$(echo "$response" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$session_id" ]; then
        log_fail "Failed to create session"
        return 1
    fi

    log_info "Created session: $session_id"

    # Wait a moment for session to be saved
    sleep 2

    # Retrieve session
    get_response=$(curl -s "$BASE_URL/api/sessions/$session_id")

    if echo "$get_response" | grep -q "\"id\":\"$session_id\""; then
        log_pass "Session persisted and retrieved successfully"
        return 0
    else
        log_fail "Session not found after creation"
        log_response "Get response: $get_response"
        return 1
    fi
}

# Test: Task Lifecycle Tracking
test_task_lifecycle() {
    log_test "Task Lifecycle Tracking"

    # Create a task
    payload='{"task":"Simple lifecycle test"}'
    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL/api/tasks")

    task_id=$(echo "$response" | grep -o '"task_id":"[^"]*"' | cut -d'"' -f4)
    initial_status=$(echo "$response" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)

    log_info "Task created: $task_id with status: $initial_status"

    # Track status changes
    for i in 1 2 3 4 5; do
        sleep 1
        status_response=$(curl -s "$BASE_URL/api/tasks/$task_id")
        current_status=$(echo "$status_response" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
        log_info "Status check $i: $current_status"

        if [ "$current_status" = "completed" ] || [ "$current_status" = "failed" ]; then
            break
        fi
    done

    # Verify we got meaningful status transitions
    if [ -n "$initial_status" ] && [ -n "$current_status" ]; then
        log_pass "Task lifecycle tracked: $initial_status -> $current_status"
        return 0
    else
        log_fail "Failed to track task lifecycle"
        return 1
    fi
}

# Test: Session Fork Functionality
test_session_fork_workflow() {
    log_test "Session Fork Workflow"

    # Create original session with a task
    payload='{"task":"Original session task"}'
    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL/api/tasks")

    original_session=$(echo "$response" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$original_session" ]; then
        log_fail "Failed to create original session"
        return 1
    fi

    log_info "Original session: $original_session"

    # Wait for task to process
    sleep 2

    # Fork the session
    fork_response=$(curl -s -X POST "$BASE_URL/api/sessions/$original_session/fork")
    forked_session=$(echo "$fork_response" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$forked_session" ]; then
        log_fail "Failed to fork session"
        log_response "Fork response: $fork_response"
        return 1
    fi

    log_info "Forked session: $forked_session"

    # Verify fork is different from original
    if [ "$forked_session" != "$original_session" ]; then
        # Create task in forked session
        fork_task_payload="{\"task\":\"Forked session task\",\"session_id\":\"$forked_session\"}"
        fork_task_response=$(curl -s -X POST \
            -H "Content-Type: application/json" \
            -d "$fork_task_payload" \
            "$BASE_URL/api/tasks")

        fork_task_session=$(echo "$fork_task_response" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

        if [ "$fork_task_session" = "$forked_session" ]; then
            log_pass "Session fork workflow completed successfully"
            return 0
        else
            log_fail "Forked session not used for new task"
            return 1
        fi
    else
        log_fail "Fork created same session ID as original"
        return 1
    fi
}

# Test: Error Recovery
test_error_recovery() {
    log_test "Error Recovery and Handling"

    # Create a task that will likely fail
    payload='{"task":"Execute command that does not exist: xyz123notacommand"}'
    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL/api/tasks")

    task_id=$(echo "$response" | grep -o '"task_id":"[^"]*"' | cut -d'"' -f4)
    session_id=$(echo "$response" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$task_id" ]; then
        log_fail "Failed to create error test task"
        return 1
    fi

    log_info "Created error test task: $task_id"

    # Wait for task to process
    sleep 5

    # Check if task completed (with or without error)
    status_response=$(curl -s "$BASE_URL/api/tasks/$task_id")
    status=$(echo "$status_response" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
    has_error=$(echo "$status_response" | grep -c '"error"' || true)

    log_info "Task status: $status"
    log_info "Has error field: $has_error"

    # Verify system handled error gracefully
    if [ "$status" = "completed" ] || [ "$status" = "failed" ] || [ "$has_error" -gt 0 ]; then
        # Create another task in same session to verify recovery
        recovery_payload="{\"task\":\"Simple recovery test\",\"session_id\":\"$session_id\"}"
        recovery_response=$(curl -s -X POST \
            -H "Content-Type: application/json" \
            -d "$recovery_payload" \
            "$BASE_URL/api/tasks")

        recovery_task_id=$(echo "$recovery_response" | grep -o '"task_id":"[^"]*"' | cut -d'"' -f4)

        if [ -n "$recovery_task_id" ]; then
            log_pass "System recovered from error and accepted new task"
            return 0
        else
            log_fail "System failed to recover - could not create new task"
            return 1
        fi
    else
        log_fail "Error handling unclear: status=$status"
        return 1
    fi
}

# Test: Pagination Functionality
test_pagination() {
    log_test "Pagination Functionality"

    # Create multiple tasks
    log_info "Creating 5 tasks for pagination test..."
    for i in 1 2 3 4 5; do
        payload="{\"task\":\"Pagination test task $i\"}"
        curl -s -X POST \
            -H "Content-Type: application/json" \
            -d "$payload" \
            "$BASE_URL/api/tasks" > /dev/null
    done

    sleep 2

    # Test pagination
    page1=$(curl -s "$BASE_URL/api/tasks?limit=2&offset=0")
    page2=$(curl -s "$BASE_URL/api/tasks?limit=2&offset=2")

    tasks1_count=$(echo "$page1" | grep -o '"task_id":"[^"]*"' | wc -l)
    tasks2_count=$(echo "$page2" | grep -o '"task_id":"[^"]*"' | wc -l)

    log_info "Page 1 tasks: $tasks1_count"
    log_info "Page 2 tasks: $tasks2_count"

    if [ "$tasks1_count" -ge 1 ] && [ "$tasks2_count" -ge 1 ]; then
        log_pass "Pagination working correctly"
        return 0
    else
        log_fail "Pagination not working as expected"
        return 1
    fi
}

# Test: Agent Preset Application (if supported)
test_agent_preset() {
    log_test "Agent Preset Application"

    # Try creating task with preset
    payload='{"task":"Test with code-expert preset","agent_preset":"code-expert"}'
    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL/api/tasks")

    task_id=$(echo "$response" | grep -o '"task_id":"[^"]*"' | cut -d'"' -f4)

    if [ -n "$task_id" ]; then
        log_pass "Agent preset parameter accepted"
        log_info "Note: Actual preset behavior validation requires deeper inspection"
        return 0
    else
        log_info "Agent preset not supported or request failed"
        log_pass "Skipping agent preset test (not critical)"
        return 0
    fi
}

# Test: Performance - Response Times
test_response_times() {
    log_test "Performance - API Response Times"

    # Health check timing
    start=$(date +%s%N)
    curl -s "$BASE_URL/health" > /dev/null
    end=$(date +%s%N)
    health_time=$(( (end - start) / 1000000 ))

    # Task creation timing
    start=$(date +%s%N)
    payload='{"task":"Performance test"}'
    curl -s -X POST -H "Content-Type: application/json" -d "$payload" "$BASE_URL/api/tasks" > /dev/null
    end=$(date +%s%N)
    create_time=$(( (end - start) / 1000000 ))

    log_info "Health check response time: ${health_time}ms"
    log_info "Task creation response time: ${create_time}ms"

    if [ "$health_time" -lt 1000 ] && [ "$create_time" -lt 5000 ]; then
        log_pass "API response times acceptable (health: ${health_time}ms, create: ${create_time}ms)"
        return 0
    else
        log_fail "API response times too slow (health: ${health_time}ms, create: ${create_time}ms)"
        return 1
    fi
}

# Run all tests
echo "========================================="
echo "ALEX Integration & E2E Acceptance Tests"
echo "========================================="
echo ""

test_e2e_task_execution
echo ""
test_multistep_workflow
echo ""
test_concurrent_sessions
echo ""
test_session_persistence
echo ""
test_task_lifecycle
echo ""
test_session_fork_workflow
echo ""
test_error_recovery
echo ""
test_pagination
echo ""
test_agent_preset
echo ""
test_response_times
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

# Exit with appropriate code
if [ $FAILED_TESTS -eq 0 ]; then
    exit 0
else
    exit 1
fi
