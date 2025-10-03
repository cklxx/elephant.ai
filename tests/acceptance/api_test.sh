#!/bin/bash

# ALEX Backend API Acceptance Tests
# Tests all REST API endpoints for correctness and reliability

set -e

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8080}"
RESULTS_DIR="$(dirname "$0")/results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULTS_FILE="$RESULTS_DIR/api_test_${TIMESTAMP}.txt"

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
echo "ALEX Backend API Acceptance Tests" > "$RESULTS_FILE"
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

# Test: Health Check
test_health_check() {
    log_test "Health Check Endpoint"

    response=$(curl -s -w "\n%{http_code}" "$BASE_URL/health")
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    log_response "Response Code: $http_code"
    log_response "Response Body: $body"

    if [ "$http_code" = "200" ]; then
        if echo "$body" | grep -q '"status":"ok"'; then
            log_pass "Health check returned 200 OK with correct status"
            return 0
        else
            log_fail "Health check returned 200 but incorrect body: $body"
            return 1
        fi
    else
        log_fail "Health check failed with status code: $http_code"
        return 1
    fi
}

# Test: Create Task
test_create_task() {
    log_test "Create Task - Basic"

    payload='{"task":"List files in current directory"}'
    response=$(curl -s -w "\n%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL/api/tasks")

    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    log_response "Request: $payload"
    log_response "Response Code: $http_code"
    log_response "Response Body: $body"

    if [ "$http_code" = "201" ]; then
        task_id=$(echo "$body" | grep -o '"task_id":"[^"]*"' | cut -d'"' -f4)
        session_id=$(echo "$body" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)
        status=$(echo "$body" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)

        if [ -n "$task_id" ] && [ -n "$session_id" ] && [ -n "$status" ]; then
            log_pass "Task created successfully: task_id=$task_id, session_id=$session_id, status=$status"
            echo "$task_id|$session_id" > "$RESULTS_DIR/last_task.txt"
            return 0
        else
            log_fail "Task created but response missing required fields"
            return 1
        fi
    else
        log_fail "Task creation failed with status code: $http_code"
        return 1
    fi
}

# Test: Create Task with Session ID
test_create_task_with_session() {
    log_test "Create Task - With Existing Session"

    # First create a task to get a session
    payload1='{"task":"Echo hello world"}'
    response1=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload1" \
        "$BASE_URL/api/tasks")

    session_id=$(echo "$response1" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$session_id" ]; then
        log_fail "Failed to get session_id from first task"
        return 1
    fi

    log_info "Using session_id: $session_id"

    # Create second task in same session
    payload2="{\"task\":\"List current directory\",\"session_id\":\"$session_id\"}"
    response2=$(curl -s -w "\n%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d "$payload2" \
        "$BASE_URL/api/tasks")

    http_code=$(echo "$response2" | tail -n1)
    body=$(echo "$response2" | sed '$d')

    log_response "Request: $payload2"
    log_response "Response Code: $http_code"
    log_response "Response Body: $body"

    if [ "$http_code" = "201" ]; then
        returned_session=$(echo "$body" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)
        if [ "$returned_session" = "$session_id" ]; then
            log_pass "Task created in existing session successfully"
            return 0
        else
            log_fail "Task created but session_id mismatch: expected=$session_id, got=$returned_session"
            return 1
        fi
    else
        log_fail "Task creation with session failed with status code: $http_code"
        return 1
    fi
}

# Test: Get Task Status
test_get_task_status() {
    log_test "Get Task Status"

    # Read last created task
    if [ ! -f "$RESULTS_DIR/last_task.txt" ]; then
        log_fail "No task ID available from previous tests"
        return 1
    fi

    task_info=$(cat "$RESULTS_DIR/last_task.txt")
    task_id=$(echo "$task_info" | cut -d'|' -f1)

    log_info "Testing with task_id: $task_id"

    response=$(curl -s -w "\n%{http_code}" "$BASE_URL/api/tasks/$task_id")
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    log_response "Response Code: $http_code"
    log_response "Response Body: $body"

    if [ "$http_code" = "200" ]; then
        if echo "$body" | grep -q '"task_id"' && echo "$body" | grep -q '"session_id"' && echo "$body" | grep -q '"status"'; then
            log_pass "Task status retrieved successfully"
            return 0
        else
            log_fail "Task status response missing required fields"
            return 1
        fi
    elif [ "$http_code" = "404" ]; then
        log_fail "Task not found (404) - possible storage issue"
        return 1
    else
        log_fail "Get task status failed with status code: $http_code"
        return 1
    fi
}

# Test: List Tasks
test_list_tasks() {
    log_test "List Tasks - Default Pagination"

    response=$(curl -s -w "\n%{http_code}" "$BASE_URL/api/tasks")
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    log_response "Response Code: $http_code"
    log_response "Response Body: $body"

    if [ "$http_code" = "200" ]; then
        if echo "$body" | grep -q '"tasks"' && echo "$body" | grep -q '"total"'; then
            log_pass "Task list retrieved successfully"
            return 0
        else
            log_fail "Task list response missing required fields"
            return 1
        fi
    else
        log_fail "List tasks failed with status code: $http_code"
        return 1
    fi
}

# Test: List Tasks with Pagination
test_list_tasks_pagination() {
    log_test "List Tasks - With Pagination"

    response=$(curl -s -w "\n%{http_code}" "$BASE_URL/api/tasks?limit=5&offset=0")
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    log_response "Response Code: $http_code"
    log_response "Response Body: $body"

    if [ "$http_code" = "200" ]; then
        limit=$(echo "$body" | grep -o '"limit":[0-9]*' | cut -d':' -f2)
        offset=$(echo "$body" | grep -o '"offset":[0-9]*' | cut -d':' -f2)

        if [ "$limit" = "5" ] && [ "$offset" = "0" ]; then
            log_pass "Task pagination parameters applied correctly"
            return 0
        else
            log_fail "Pagination parameters not applied: limit=$limit, offset=$offset"
            return 1
        fi
    else
        log_fail "List tasks with pagination failed with status code: $http_code"
        return 1
    fi
}

# Test: Cancel Task
test_cancel_task() {
    log_test "Cancel Task"

    # Create a long-running task first
    payload='{"task":"Count from 1 to 1000000"}'
    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL/api/tasks")

    task_id=$(echo "$response" | grep -o '"task_id":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$task_id" ]; then
        log_fail "Failed to create task for cancellation test"
        return 1
    fi

    log_info "Created task for cancellation: $task_id"

    # Wait a moment for task to start
    sleep 1

    # Cancel the task
    response=$(curl -s -w "\n%{http_code}" -X POST \
        "$BASE_URL/api/tasks/$task_id/cancel")

    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    log_response "Response Code: $http_code"
    log_response "Response Body: $body"

    if [ "$http_code" = "200" ]; then
        if echo "$body" | grep -q '"status":"cancelled"'; then
            log_pass "Task cancelled successfully"
            return 0
        else
            log_fail "Task cancel returned 200 but unexpected body"
            return 1
        fi
    else
        log_fail "Cancel task failed with status code: $http_code"
        return 1
    fi
}

# Test: Create Session (implicit via task)
test_session_creation() {
    log_test "Session Creation (Implicit)"

    payload='{"task":"Test session creation"}'
    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL/api/tasks")

    session_id=$(echo "$response" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

    if [ -n "$session_id" ] && [ "$session_id" != "creating..." ]; then
        log_pass "Session created successfully: $session_id"
        echo "$session_id" > "$RESULTS_DIR/test_session.txt"
        return 0
    else
        log_fail "Session creation failed or returned invalid session_id: $session_id"
        return 1
    fi
}

# Test: Get Session
test_get_session() {
    log_test "Get Session Details"

    if [ ! -f "$RESULTS_DIR/test_session.txt" ]; then
        log_fail "No session ID available from previous tests"
        return 1
    fi

    session_id=$(cat "$RESULTS_DIR/test_session.txt")
    log_info "Testing with session_id: $session_id"

    response=$(curl -s -w "\n%{http_code}" "$BASE_URL/api/sessions/$session_id")
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    log_response "Response Code: $http_code"
    log_response "Response Body: $body"

    if [ "$http_code" = "200" ]; then
        if echo "$body" | grep -q '"id"'; then
            log_pass "Session details retrieved successfully"
            return 0
        else
            log_fail "Session response missing required fields"
            return 1
        fi
    elif [ "$http_code" = "404" ]; then
        log_fail "Session not found (404) - possible storage issue"
        return 1
    else
        log_fail "Get session failed with status code: $http_code"
        return 1
    fi
}

# Test: List Sessions
test_list_sessions() {
    log_test "List All Sessions"

    response=$(curl -s -w "\n%{http_code}" "$BASE_URL/api/sessions")
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    log_response "Response Code: $http_code"
    log_response "Response Body: $body"

    if [ "$http_code" = "200" ]; then
        if echo "$body" | grep -q '"sessions"' && echo "$body" | grep -q '"total"'; then
            total=$(echo "$body" | grep -o '"total":[0-9]*' | cut -d':' -f2)
            log_pass "Session list retrieved successfully (total: $total)"
            return 0
        else
            log_fail "Session list response missing required fields"
            return 1
        fi
    else
        log_fail "List sessions failed with status code: $http_code"
        return 1
    fi
}

# Test: Fork Session
test_fork_session() {
    log_test "Fork Session"

    if [ ! -f "$RESULTS_DIR/test_session.txt" ]; then
        log_fail "No session ID available from previous tests"
        return 1
    fi

    session_id=$(cat "$RESULTS_DIR/test_session.txt")
    log_info "Forking session: $session_id"

    response=$(curl -s -w "\n%{http_code}" -X POST \
        "$BASE_URL/api/sessions/$session_id/fork")

    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    log_response "Response Code: $http_code"
    log_response "Response Body: $body"

    if [ "$http_code" = "201" ]; then
        new_session_id=$(echo "$body" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
        if [ -n "$new_session_id" ] && [ "$new_session_id" != "$session_id" ]; then
            log_pass "Session forked successfully: $new_session_id"
            echo "$new_session_id" > "$RESULTS_DIR/forked_session.txt"
            return 0
        else
            log_fail "Session fork returned invalid new session ID"
            return 1
        fi
    else
        log_fail "Fork session failed with status code: $http_code"
        return 1
    fi
}

# Test: Delete Session
test_delete_session() {
    log_test "Delete Session"

    # Create a temporary session to delete
    payload='{"task":"Temporary task for deletion test"}'
    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL/api/tasks")

    session_id=$(echo "$response" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$session_id" ]; then
        log_fail "Failed to create session for deletion test"
        return 1
    fi

    log_info "Deleting session: $session_id"

    response=$(curl -s -w "\n%{http_code}" -X DELETE \
        "$BASE_URL/api/sessions/$session_id")

    http_code=$(echo "$response" | tail -n1)

    log_response "Response Code: $http_code"

    if [ "$http_code" = "204" ]; then
        log_pass "Session deleted successfully"

        # Verify session is gone
        verify_response=$(curl -s -w "\n%{http_code}" "$BASE_URL/api/sessions/$session_id")
        verify_code=$(echo "$verify_response" | tail -n1)

        if [ "$verify_code" = "404" ]; then
            log_pass "Session deletion verified (404 on GET)"
            return 0
        else
            log_fail "Session still exists after deletion (status: $verify_code)"
            return 1
        fi
    else
        log_fail "Delete session failed with status code: $http_code"
        return 1
    fi
}

# Test: Invalid Requests
test_invalid_task_creation() {
    log_test "Invalid Request - Empty Task"

    payload='{"task":""}'
    response=$(curl -s -w "\n%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL/api/tasks")

    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    log_response "Request: $payload"
    log_response "Response Code: $http_code"
    log_response "Response Body: $body"

    if [ "$http_code" = "400" ]; then
        log_pass "Empty task correctly rejected with 400"
        return 0
    else
        log_fail "Empty task should return 400, got: $http_code"
        return 1
    fi
}

test_invalid_task_id() {
    log_test "Invalid Request - Non-existent Task ID"

    response=$(curl -s -w "\n%{http_code}" "$BASE_URL/api/tasks/nonexistent-task-id")
    http_code=$(echo "$response" | tail -n1)

    log_response "Response Code: $http_code"

    if [ "$http_code" = "404" ]; then
        log_pass "Non-existent task correctly returned 404"
        return 0
    else
        log_fail "Non-existent task should return 404, got: $http_code"
        return 1
    fi
}

test_invalid_session_id() {
    log_test "Invalid Request - Non-existent Session ID"

    response=$(curl -s -w "\n%{http_code}" "$BASE_URL/api/sessions/nonexistent-session-id")
    http_code=$(echo "$response" | tail -n1)

    log_response "Response Code: $http_code"

    if [ "$http_code" = "404" ]; then
        log_pass "Non-existent session correctly returned 404"
        return 0
    else
        log_fail "Non-existent session should return 404, got: $http_code"
        return 1
    fi
}

# Run all tests
echo "========================================="
echo "ALEX Backend API Acceptance Tests"
echo "========================================="
echo ""

# Basic endpoint tests
test_health_check
echo ""

# Task management tests
test_create_task
echo ""
test_create_task_with_session
echo ""
test_get_task_status
echo ""
test_list_tasks
echo ""
test_list_tasks_pagination
echo ""
test_cancel_task
echo ""

# Session management tests
test_session_creation
echo ""
test_get_session
echo ""
test_list_sessions
echo ""
test_fork_session
echo ""
test_delete_session
echo ""

# Error handling tests
test_invalid_task_creation
echo ""
test_invalid_task_id
echo ""
test_invalid_session_id
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
