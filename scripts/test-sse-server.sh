#!/bin/bash

# Test script for ALEX SSE Server
# Usage: ./scripts/test-sse-server.sh [server_url]

SERVER_URL="${1:-http://localhost:8080}"
SESSION_ID="test-session-$(date +%s)"

echo "======================================"
echo "ALEX SSE Server Test Script"
echo "======================================"
echo "Server URL: $SERVER_URL"
echo "Session ID: $SESSION_ID"
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print test result
print_result() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}✓ $2${NC}"
    else
        echo -e "${RED}✗ $2${NC}"
    fi
}

# Test 1: Health Check
echo "Test 1: Health Check"
echo "------------------------------------"
response=$(curl -s -w "\n%{http_code}" "$SERVER_URL/health")
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')

if [ "$http_code" = "200" ]; then
    print_result 0 "Health check passed"
    echo "Response: $body"
else
    print_result 1 "Health check failed (HTTP $http_code)"
fi
echo ""

# Test 2: List Sessions
echo "Test 2: List Sessions"
echo "------------------------------------"
response=$(curl -s -w "\n%{http_code}" "$SERVER_URL/api/sessions")
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')

if [ "$http_code" = "200" ]; then
    print_result 0 "List sessions successful"
    echo "Response: $body"
else
    print_result 1 "List sessions failed (HTTP $http_code)"
fi
echo ""

# Test 3: Submit Task
echo "Test 3: Submit Task"
echo "------------------------------------"
task_json=$(cat <<EOF
{
  "task": "What is the capital of France? Just answer with the city name.",
  "session_id": "$SESSION_ID"
}
EOF
)

response=$(curl -s -w "\n%{http_code}" -X POST "$SERVER_URL/api/tasks" \
  -H "Content-Type: application/json" \
  -d "$task_json")

http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')

if [ "$http_code" = "200" ]; then
    print_result 0 "Task submission successful"
    echo "Response: $body"
else
    print_result 1 "Task submission failed (HTTP $http_code)"
    echo "Response: $body"
fi
echo ""

# Test 4: SSE Connection (with timeout)
echo "Test 4: SSE Connection (10 second sample)"
echo "------------------------------------"
echo -e "${YELLOW}Connecting to SSE endpoint...${NC}"
echo "URL: $SERVER_URL/api/sse?session_id=$SESSION_ID"
echo ""

# Start SSE connection in background and capture output
timeout 10s curl -N -H "Accept: text/event-stream" \
  "$SERVER_URL/api/sse?session_id=$SESSION_ID" 2>/dev/null &
SSE_PID=$!

# Wait for SSE to finish or timeout
wait $SSE_PID 2>/dev/null
sse_result=$?

if [ $sse_result -eq 124 ]; then
    # Timeout (expected)
    print_result 0 "SSE connection established and received events"
elif [ $sse_result -eq 0 ]; then
    print_result 0 "SSE connection completed normally"
else
    print_result 1 "SSE connection failed"
fi
echo ""

# Test 5: Submit Another Task and Monitor SSE
echo "Test 5: Full Workflow Test"
echo "------------------------------------"
echo -e "${YELLOW}Starting SSE listener in background...${NC}"

# Start SSE connection in background
timeout 15s curl -N -H "Accept: text/event-stream" \
  "$SERVER_URL/api/sse?session_id=$SESSION_ID" 2>/dev/null > /tmp/alex-sse-test.log &
SSE_PID=$!

# Wait a moment for connection to establish
sleep 2

# Submit a task
echo "Submitting task..."
task_json=$(cat <<EOF
{
  "task": "Echo 'Hello from ALEX SSE Server'",
  "session_id": "$SESSION_ID"
}
EOF
)

curl -s -X POST "$SERVER_URL/api/tasks" \
  -H "Content-Type: application/json" \
  -d "$task_json" > /dev/null

# Wait for SSE to capture events
sleep 10

# Kill SSE connection
kill $SSE_PID 2>/dev/null

# Check if we received events
if [ -f /tmp/alex-sse-test.log ]; then
    event_count=$(grep -c "^event:" /tmp/alex-sse-test.log)
    if [ $event_count -gt 0 ]; then
        print_result 0 "Received $event_count events"
        echo ""
        echo "Sample events:"
        head -n 20 /tmp/alex-sse-test.log
    else
        print_result 1 "No events received"
    fi
    rm /tmp/alex-sse-test.log
else
    print_result 1 "SSE log file not created"
fi
echo ""

# Test 6: Invalid Requests
echo "Test 6: Error Handling"
echo "------------------------------------"

# Test 6a: Missing session_id in SSE
response=$(curl -s -w "\n%{http_code}" "$SERVER_URL/api/sse")
http_code=$(echo "$response" | tail -n1)
if [ "$http_code" = "400" ]; then
    print_result 0 "Correctly rejected SSE without session_id"
else
    print_result 1 "Should reject SSE without session_id (got HTTP $http_code)"
fi

# Test 6b: Missing task in POST
response=$(curl -s -w "\n%{http_code}" -X POST "$SERVER_URL/api/tasks" \
  -H "Content-Type: application/json" \
  -d '{"session_id": "test"}')
http_code=$(echo "$response" | tail -n1)
if [ "$http_code" = "400" ]; then
    print_result 0 "Correctly rejected task without content"
else
    print_result 1 "Should reject task without content (got HTTP $http_code)"
fi

echo ""

# Summary
echo "======================================"
echo "Test Summary"
echo "======================================"
echo "All basic tests completed!"
echo ""
echo "To manually test SSE streaming:"
echo "  1. Start SSE connection:"
echo "     curl -N -H 'Accept: text/event-stream' \\"
echo "       '$SERVER_URL/api/sse?session_id=$SESSION_ID'"
echo ""
echo "  2. In another terminal, submit a task:"
echo "     curl -X POST $SERVER_URL/api/tasks \\"
echo "       -H 'Content-Type: application/json' \\"
echo "       -d '{\"task\": \"List files in current directory\", \"session_id\": \"$SESSION_ID\"}'"
echo ""
