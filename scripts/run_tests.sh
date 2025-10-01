#!/bin/bash

# ALEX Test Suite Runner
# Runs all tests with coverage and generates reports

set -e

echo "🧪 ALEX Test Suite"
echo "=================="
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test results
FAILED_PACKAGES=()
PASSED_PACKAGES=()

# Function to run tests for a package
run_package_tests() {
    local package=$1
    local name=$2

    echo -n "Testing $name... "

    if go test -timeout 30s "$package" > /tmp/test_$name.log 2>&1; then
        echo -e "${GREEN}✓ PASS${NC}"
        PASSED_PACKAGES+=("$name")
        return 0
    else
        echo -e "${RED}✗ FAIL${NC}"
        FAILED_PACKAGES+=("$name")
        echo "  Error log saved to /tmp/test_$name.log"
        return 1
    fi
}

echo "Phase 1: Core Domain Tests"
echo "----------------------------"
run_package_tests "./internal/agent/domain" "domain"
run_package_tests "./internal/agent/app" "app"
run_package_tests "./internal/agent/ports/mocks" "mocks"
echo ""

echo "Phase 2: Infrastructure Tests"
echo "------------------------------"
run_package_tests "./internal/llm" "llm"
run_package_tests "./internal/storage" "storage"
run_package_tests "./internal/context" "context"
run_package_tests "./internal/parser" "parser"
run_package_tests "./internal/messaging" "messaging"
run_package_tests "./internal/session/filestore" "session"
echo ""

echo "Phase 3: Tool Tests"
echo "-------------------"
run_package_tests "./internal/tools/builtin" "tools-builtin"
run_package_tests "./internal/tools" "tools-registry"
echo ""

echo "Phase 4: New Feature Tests"
echo "--------------------------"
run_package_tests "./internal/errors" "errors"
run_package_tests "./internal/diff" "diff"
run_package_tests "./internal/backup" "backup"
run_package_tests "./internal/approval" "approval"
run_package_tests "./internal/rag" "rag"
run_package_tests "./internal/mcp" "mcp"
echo ""

echo "Phase 5: Observability Tests"
echo "-----------------------------"
run_package_tests "./internal/observability" "observability"
echo ""

echo "Phase 6: Integration Tests"
echo "---------------------------"
run_package_tests "./evaluation/swe_bench" "swe-bench" || true
echo ""

echo "================="
echo "📊 Test Summary"
echo "================="
echo ""

TOTAL=$((${#PASSED_PACKAGES[@]} + ${#FAILED_PACKAGES[@]}))
echo "Total packages tested: $TOTAL"
echo -e "${GREEN}Passed: ${#PASSED_PACKAGES[@]}${NC}"
echo -e "${RED}Failed: ${#FAILED_PACKAGES[@]}${NC}"
echo ""

if [ ${#FAILED_PACKAGES[@]} -gt 0 ]; then
    echo -e "${RED}Failed packages:${NC}"
    for pkg in "${FAILED_PACKAGES[@]}"; do
        echo "  - $pkg (see /tmp/test_$pkg.log)"
    done
    echo ""
    exit 1
else
    echo -e "${GREEN}✅ All tests passed!${NC}"
    echo ""

    # Run coverage report
    echo "📈 Generating coverage report..."
    go test -coverprofile=coverage.out ./internal/... > /dev/null 2>&1 || true
    if [ -f coverage.out ]; then
        COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
        echo -e "${GREEN}Total coverage: $COVERAGE${NC}"
        rm coverage.out
    fi

    exit 0
fi
