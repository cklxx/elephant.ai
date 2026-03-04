#!/bin/bash
# Test script to simulate Claude Code events for visualizer

VISUALIZER_URL="${VISUALIZER_URL:-http://localhost:3002/api/visualizer/events}"

echo "🦀 Testing Claude Code Visualizer"
echo "================================="
echo "Sending test events to: $VISUALIZER_URL"
echo ""

# Test 1: Read event
echo "📖 Test 1: Sending Read event..."
echo '{
  "hook_event_name": "tool-use",
  "tool_name": "Read",
  "tool_input": {
    "file_path": "/Users/bytedance/code/elephant.ai/README.md"
  }
}' | ~/.claude/hooks/visualizer-hook.sh
sleep 1

# Test 2: Write event
echo "✍️  Test 2: Sending Write event..."
echo '{
  "hook_event_name": "tool-use",
  "tool_name": "Write",
  "tool_input": {
    "file_path": "/Users/bytedance/code/elephant.ai/internal/domain/agent/react/engine.go"
  }
}' | ~/.claude/hooks/visualizer-hook.sh
sleep 1

# Test 3: Grep event
echo "🔍 Test 3: Sending Grep event..."
echo '{
  "hook_event_name": "tool-use",
  "tool_name": "Grep",
  "tool_input": {
    "path": "/Users/bytedance/code/elephant.ai/web"
  }
}' | ~/.claude/hooks/visualizer-hook.sh
sleep 1

# Test 4: Multiple events in different folders
echo "📁 Test 4: Sending events to multiple folders..."
for folder in "internal/llm" "web/components" "internal/tools" "cmd/alex"; do
  echo '{
    "hook_event_name": "tool-use",
    "tool_name": "Read",
    "tool_input": {
      "file_path": "/Users/bytedance/code/elephant.ai/'$folder'/test.go"
    }
  }' | ~/.claude/hooks/visualizer-hook.sh
  sleep 0.3
done

echo ""
echo "✅ Test events sent!"
echo "Check the visualizer at: http://localhost:3002/visualizer"
echo ""

# Verify events were received
echo "📊 Checking API..."
EVENTS=$(curl -s "http://localhost:3002/api/visualizer/events?limit=20" | jq '.count')
echo "Total events in API: $EVENTS"
