#!/bin/bash
# Test script for Chat TUI formatting

echo "Testing ALEX Chat TUI Formatting"
echo "================================="
echo ""
echo "This script will test various formatting scenarios"
echo ""

# Test 1: Command mode with markdown
echo "Test 1: Command mode with code block"
./alex "show a simple Go hello world function" | head -20

echo ""
echo "Test 2: Command mode with list"
./alex "list 3 programming languages" | head -20

echo ""
echo "Test 3: Command mode with table (if supported)"
./alex "show a 2x2 table" | head -20

echo ""
echo "================================="
echo "For interactive chat TUI, run: ./alex"
echo "Then test manually:"
echo "  1. Type: 'show me python code'"
echo "  2. Type: 'create a markdown table'"
echo "  3. Type: 'explain with bullet points'"
echo "================================="
