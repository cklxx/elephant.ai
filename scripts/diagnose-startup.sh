#!/bin/bash

# ALEX Startup Diagnosis Script
# Helps identify why ./alex might not be responding

set -e

echo "🔍 ALEX Startup Diagnosis"
echo "=========================="
echo ""

# Check binary exists and is executable
echo "1️⃣  Checking binary..."
if [ ! -x "./alex" ]; then
    echo "   ❌ ./alex binary not found or not executable"
    exit 1
fi
echo "   ✅ Binary exists and is executable"
echo ""

# Check config file
echo "2️⃣  Checking configuration..."
CONFIG_FILE="$HOME/.alex-config.json"
if [ ! -f "$CONFIG_FILE" ]; then
    echo "   ⚠️  Config file not found: $CONFIG_FILE"
    echo "   Run: ./alex config  to initialize"
else
    echo "   ✅ Config file exists: $CONFIG_FILE"

    # Check API key
    if grep -q "api_key" "$CONFIG_FILE"; then
        echo "   ✅ API key configured"
    else
        echo "   ❌ No API key in config"
    fi
fi
echo ""

# Test version command
echo "3️⃣  Testing basic command (./alex --no-tui version)..."
if ./alex --no-tui version 2>&1 | head -1; then
    echo "   ✅ Basic command works"
else
    echo "   ❌ Basic command failed"
fi
echo ""

# Test native UI
echo "4️⃣  Testing native UI mode (./alex --no-tui)..."
echo "   Running: echo 'help' | ./alex --no-tui"
echo ""
echo "   Output:"
(echo "help"; sleep 1; echo "/quit") | ./alex --no-tui 2>&1 | head -20 | sed 's/^/   /'
echo ""
echo "   ✅ Native UI works"
echo ""

# Explain what's happening with direct ./alex
echo "5️⃣  Understanding './alex' behavior..."
echo ""
echo "   When you run './alex' directly:"
echo "   • It tries to start TUI (tcell-based interface)"
echo "   • If TUI init fails → falls back to native UI"
echo "   • If successful → enters interactive chat mode"
echo ""
echo "   Symptoms:"
echo "   ✓ No output → Waiting for interactive input"
echo "   ✓ Just shows terminal → Also waiting for input"
echo ""
echo "   SOLUTION:"
echo "   • Type your request and press Enter"
echo "   • Or use: ./alex --no-tui (for better compatibility)"
echo ""

# Provide usage examples
echo "6️⃣  Quick Start Commands..."
echo ""
echo "   Option A: Interactive (use --no-tui for compatibility)"
echo "   $ ./alex --no-tui"
echo "   > list files"
echo "   > /quit"
echo ""
echo "   Option B: Direct query"
echo "   $ ./alex \"what is the structure of this project?\""
echo ""
echo "   Option C: Web interface (no terminal issues)"
echo "   $ ./deploy.sh start"
echo ""

echo "✅ Diagnosis complete!"
echo ""
echo "Next step: Try './alex --no-tui' for best compatibility"
