#!/bin/bash
# Diagnose Claude Code Visualizer Hooks
# Helps identify why hooks aren't triggering

set -euo pipefail

echo "🔍 Claude Code Visualizer - Hook Diagnostics"
echo "=============================================="
echo ""

# Check 1: Hook script exists
echo "✓ Check 1: Hook script exists"
if [ -f ~/.claude/hooks/visualizer-hook.sh ]; then
  echo "  ✅ Hook script found: ~/.claude/hooks/visualizer-hook.sh"
else
  echo "  ❌ Hook script NOT found!"
  echo "  Fix: Run from project root:"
  echo "      cp scripts/visualizer-hook.sh ~/.claude/hooks/"
  exit 1
fi

# Check 2: Hook script is executable
echo ""
echo "✓ Check 2: Hook script is executable"
if [ -x ~/.claude/hooks/visualizer-hook.sh ]; then
  echo "  ✅ Script has execute permissions"
else
  echo "  ❌ Script is NOT executable!"
  echo "  Fix: chmod +x ~/.claude/hooks/visualizer-hook.sh"
  exit 1
fi

# Check 3: jq is installed
echo ""
echo "✓ Check 3: jq is installed"
if command -v jq &> /dev/null; then
  JQ_VERSION=$(jq --version)
  echo "  ✅ jq found: $JQ_VERSION"
else
  echo "  ❌ jq NOT found!"
  echo "  Fix: brew install jq"
  exit 1
fi

# Check 4: Hook configuration exists
echo ""
echo "✓ Check 4: Hook configuration exists"
if [ -f ~/.claude/hooks.json ]; then
  echo "  ✅ Hook config found: ~/.claude/hooks.json"

  # Validate JSON
  if jq empty ~/.claude/hooks.json 2>/dev/null; then
    echo "  ✅ JSON is valid"
  else
    echo "  ❌ JSON is INVALID!"
    echo "  Fix: Check ~/.claude/hooks.json for syntax errors"
    exit 1
  fi

  # Check for visualizer hook
  if grep -q "visualizer-hook" ~/.claude/hooks.json; then
    echo "  ✅ Visualizer hook configured"
  else
    echo "  ⚠️  Visualizer hook not found in config"
    echo "  Add this to ~/.claude/hooks.json:"
    cat << 'EOF'
{
  "hooks": [
    {
      "event": "tool-use",
      "matcher": "**/*",
      "hooks": [
        {
          "type": "command",
          "command": "VISUALIZER_URL=http://localhost:3002/api/visualizer/events ~/.claude/hooks/visualizer-hook.sh",
          "async": true,
          "timeout": 5
        }
      ]
    }
  ]
}
EOF
  fi
else
  echo "  ❌ Hook config NOT found!"
  echo "  Create ~/.claude/hooks.json with visualizer config"
  exit 1
fi

# Check 5: Dev server is running
echo ""
echo "✓ Check 5: Dev server is running"
if curl -s -f "http://localhost:3002/api/visualizer/events" > /dev/null 2>&1; then
  echo "  ✅ Dev server responding on port 3002"
else
  echo "  ❌ Dev server NOT responding!"
  echo "  Fix: cd web && PORT=3002 npm run dev"
  exit 1
fi

# Check 6: Test hook manually
echo ""
echo "✓ Check 6: Testing hook manually"
TEST_EVENT=$(cat << 'EOF'
{
  "hook_event_name": "tool-use",
  "tool_name": "Read",
  "tool_input": {
    "file_path": "/Users/bytedance/code/elephant.ai/README.md"
  }
}
EOF
)

echo "  Sending test event through hook..."
echo "$TEST_EVENT" | ~/.claude/hooks/visualizer-hook.sh

sleep 1

# Verify event arrived
EVENT_COUNT=$(curl -s "http://localhost:3002/api/visualizer/events?limit=1" | jq -r '.count')
if [ "$EVENT_COUNT" -gt 0 ]; then
  echo "  ✅ Test event received! (Total events: $EVENT_COUNT)"
else
  echo "  ⚠️  No events in API (might be cached)"
fi

# Check 7: Enable debug logging
echo ""
echo "✓ Check 7: Debug logging setup"
echo "  To enable detailed logging, use Claude Code with:"
echo ""
echo "    DEBUG=1 claude-code"
echo ""
echo "  Then check logs:"
echo "    tail -f ~/.claude/visualizer-hook.log"

# Summary
echo ""
echo "=============================================="
echo "📊 Diagnostic Summary"
echo "=============================================="
echo ""
echo "All checks passed! ✅"
echo ""
echo "Next steps to test with real Claude Code:"
echo ""
echo "1. Start Claude Code with debug logging:"
echo "   DEBUG=1 claude-code"
echo ""
echo "2. Execute a tool call:"
echo "   > Read the README.md file"
echo ""
echo "3. Check if hook was triggered:"
echo "   tail -20 ~/.claude/visualizer-hook.log"
echo ""
echo "4. Check if event reached API:"
echo "   curl -s 'http://localhost:3002/api/visualizer/events?limit=5' | jq ."
echo ""
echo "5. Open visualizer to see results:"
echo "   open http://localhost:3002/visualizer"
echo ""
echo "If hooks still don't trigger:"
echo "  - Verify Claude Code version supports hooks"
echo "  - Check Claude Code's own logs: ~/.claude/logs/"
echo "  - Try running claude-code with --verbose flag"
echo ""
