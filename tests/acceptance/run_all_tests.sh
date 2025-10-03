#!/bin/bash

# ALEX Acceptance Test Suite Runner
# Executes all acceptance tests and generates comprehensive report

set -e

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8080}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
SUMMARY_FILE="$RESULTS_DIR/test_summary_${TIMESTAMP}.txt"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Test suite status
SUITE_RESULTS=()

# Create results directory
mkdir -p "$RESULTS_DIR"

# Banner
echo -e "${BOLD}${CYAN}"
echo "========================================"
echo "  ALEX Acceptance Test Suite Runner"
echo "========================================"
echo -e "${NC}"
echo ""
echo "Base URL: $BASE_URL"
echo "Results Directory: $RESULTS_DIR"
echo "Timestamp: $(date)"
echo ""

# Initialize summary file
cat > "$SUMMARY_FILE" << EOF
ALEX Acceptance Test Suite - Comprehensive Report
==================================================
Timestamp: $(date)
Base URL: $BASE_URL
Test Run ID: $TIMESTAMP

EOF

# Function to check server health
check_server() {
    echo -e "${YELLOW}[CHECK]${NC} Verifying server is running..."

    if curl -s --max-time 5 "$BASE_URL/health" > /dev/null 2>&1; then
        echo -e "${GREEN}[OK]${NC} Server is healthy and responsive"
        echo ""
        return 0
    else
        echo -e "${RED}[ERROR]${NC} Server is not responding at $BASE_URL"
        echo ""
        echo "Please ensure the ALEX server is running:"
        echo "  ./alex-server"
        echo ""
        echo "Or set BASE_URL environment variable:"
        echo "  export BASE_URL=http://your-server:port"
        echo ""
        exit 1
    fi
}

# Function to run a test suite
run_suite() {
    local suite_name=$1
    local suite_script=$2

    echo -e "${BOLD}${BLUE}================================================${NC}"
    echo -e "${BOLD}${BLUE}Running: $suite_name${NC}"
    echo -e "${BOLD}${BLUE}================================================${NC}"
    echo ""

    local start_time=$(date +%s)

    if bash "$suite_script"; then
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))

        echo ""
        echo -e "${GREEN}${BOLD}✓ $suite_name PASSED${NC} (${duration}s)"
        echo ""

        SUITE_RESULTS+=("PASS|$suite_name|${duration}s")

        echo "$suite_name: PASSED (${duration}s)" >> "$SUMMARY_FILE"
    else
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))

        echo ""
        echo -e "${RED}${BOLD}✗ $suite_name FAILED${NC} (${duration}s)"
        echo ""

        SUITE_RESULTS+=("FAIL|$suite_name|${duration}s")

        echo "$suite_name: FAILED (${duration}s)" >> "$SUMMARY_FILE"
    fi
}

# Check server health before running tests
check_server

# Run all test suites
echo -e "${CYAN}Starting acceptance test execution...${NC}"
echo ""

run_suite "Backend API Tests" "$SCRIPT_DIR/api_test.sh"
run_suite "SSE Streaming Tests" "$SCRIPT_DIR/sse_test.sh"
run_suite "Integration & E2E Tests" "$SCRIPT_DIR/integration_test.sh"

# Generate summary
echo ""
echo -e "${BOLD}${CYAN}========================================"
echo "  Test Execution Summary"
echo "========================================${NC}"
echo ""

total_suites=${#SUITE_RESULTS[@]}
passed_suites=0
failed_suites=0

echo "" >> "$SUMMARY_FILE"
echo "========================================" >> "$SUMMARY_FILE"
echo "Suite Summary" >> "$SUMMARY_FILE"
echo "========================================" >> "$SUMMARY_FILE"
echo "" >> "$SUMMARY_FILE"

for result in "${SUITE_RESULTS[@]}"; do
    status=$(echo "$result" | cut -d'|' -f1)
    name=$(echo "$result" | cut -d'|' -f2)
    duration=$(echo "$result" | cut -d'|' -f3)

    if [ "$status" = "PASS" ]; then
        echo -e "${GREEN}✓${NC} $name - ${GREEN}PASSED${NC} ($duration)"
        passed_suites=$((passed_suites + 1))
    else
        echo -e "${RED}✗${NC} $name - ${RED}FAILED${NC} ($duration)"
        failed_suites=$((failed_suites + 1))
    fi
done

echo ""
echo -e "${BOLD}Total Suites:${NC} $total_suites"
echo -e "${BOLD}Passed:${NC}      ${GREEN}$passed_suites${NC}"
echo -e "${BOLD}Failed:${NC}      ${RED}$failed_suites${NC}"

success_rate=$(awk "BEGIN {printf \"%.2f\", ($passed_suites/$total_suites)*100}")
echo -e "${BOLD}Success Rate:${NC} $success_rate%"
echo ""

# Write final summary
echo "" >> "$SUMMARY_FILE"
echo "Total Suites: $total_suites" >> "$SUMMARY_FILE"
echo "Passed: $passed_suites" >> "$SUMMARY_FILE"
echo "Failed: $failed_suites" >> "$SUMMARY_FILE"
echo "Success Rate: $success_rate%" >> "$SUMMARY_FILE"
echo "" >> "$SUMMARY_FILE"

# List all result files
echo "========================================" >> "$SUMMARY_FILE"
echo "Detailed Results" >> "$SUMMARY_FILE"
echo "========================================" >> "$SUMMARY_FILE"
echo "" >> "$SUMMARY_FILE"
echo "Individual test results are available in:" >> "$SUMMARY_FILE"
ls -1 "$RESULTS_DIR"/*.txt | while read -r file; do
    echo "  - $(basename "$file")" >> "$SUMMARY_FILE"
done

# Display final message
echo -e "${CYAN}========================================"
echo "  Results"
echo "========================================${NC}"
echo ""
echo "Summary saved to:"
echo "  $SUMMARY_FILE"
echo ""
echo "Individual test results:"
ls -1 "$RESULTS_DIR"/*.txt | while read -r file; do
    echo "  - $(basename "$file")"
done
echo ""

# Generate acceptance report
if [ $failed_suites -eq 0 ]; then
    echo -e "${GREEN}${BOLD}════════════════════════════════════════${NC}"
    echo -e "${GREEN}${BOLD}  ✓ ALL ACCEPTANCE TESTS PASSED  ✓${NC}"
    echo -e "${GREEN}${BOLD}════════════════════════════════════════${NC}"
    echo ""
    echo -e "${GREEN}The ALEX backend server has passed all acceptance tests.${NC}"
    echo -e "${GREEN}The system is ready for production deployment.${NC}"
    echo ""
    exit 0
else
    echo -e "${RED}${BOLD}════════════════════════════════════════${NC}"
    echo -e "${RED}${BOLD}  ✗ SOME ACCEPTANCE TESTS FAILED  ✗${NC}"
    echo -e "${RED}${BOLD}════════════════════════════════════════${NC}"
    echo ""
    echo -e "${RED}$failed_suites test suite(s) failed.${NC}"
    echo -e "${YELLOW}Please review the detailed results and fix the issues.${NC}"
    echo ""
    exit 1
fi
