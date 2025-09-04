#!/bin/bash

# Alex Performance Verification Framework - Automation Script
# This script provides automation for the performance verification framework

set -e

# Configuration
ALEX_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PERF_DIR="$ALEX_ROOT/performance"
RESULTS_DIR="$PERF_DIR/results"
CONFIG_FILE="$PERF_DIR/config.json"
LOG_FILE="$PERF_DIR/verification.log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

# Error handling
error() {
    echo -e "${RED}Error: $1${NC}" >&2
    log "ERROR: $1"
    exit 1
}

# Success message
success() {
    echo -e "${GREEN}✅ $1${NC}"
    log "SUCCESS: $1"
}

# Warning message
warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
    log "WARNING: $1"
}

# Info message
info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
    log "INFO: $1"
}

# Check dependencies
check_dependencies() {
    info "Checking dependencies..."
    
    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        error "Go is not installed or not in PATH"
    fi
    
    # Check if jq is installed (for JSON processing)
    if ! command -v jq &> /dev/null; then
        warning "jq is not installed - some features may not work"
    fi
    
    # Check if Alex binary exists
    if [[ ! -f "$ALEX_ROOT/alex" ]]; then
        info "Alex binary not found, building..."
        cd "$ALEX_ROOT"
        make build || error "Failed to build Alex"
    fi
    
    success "Dependencies checked"
}

# Initialize performance framework
init_framework() {
    info "Initializing performance framework..."
    
    # Create directories
    mkdir -p "$PERF_DIR" "$RESULTS_DIR"
    
    # Initialize framework
    cd "$ALEX_ROOT"
    go run ./cmd/perf init -config "$CONFIG_FILE" || error "Failed to initialize framework"
    
    success "Performance framework initialized"
}

# Run baseline creation
create_baseline() {
    info "Creating performance baseline..."
    
    cd "$ALEX_ROOT"
    go run ./cmd/perf baseline -config "$CONFIG_FILE" || error "Failed to create baseline"
    
    success "Performance baseline created"
}

# Run benchmark suite
run_benchmarks() {
    info "Running performance benchmarks..."
    
    cd "$ALEX_ROOT"
    if go run ./cmd/perf benchmark -config "$CONFIG_FILE"; then
        success "Benchmarks completed successfully"
        return 0
    else
        warning "Some benchmarks failed"
        return 1
    fi
}

# Run test scenarios
run_tests() {
    info "Running performance test scenarios..."
    
    cd "$ALEX_ROOT"
    if go run ./cmd/perf test -config "$CONFIG_FILE"; then
        success "All test scenarios passed"
        return 0
    else
        warning "Some test scenarios failed"
        return 1
    fi
}

# Generate report
generate_report() {
    info "Generating performance report..."
    
    cd "$ALEX_ROOT"
    go run ./cmd/perf report -config "$CONFIG_FILE" || error "Failed to generate report"
    
    if [[ -f "$RESULTS_DIR/latest.json" ]]; then
        info "Latest results summary:"
        if command -v jq &> /dev/null; then
            jq '.summary' "$RESULTS_DIR/latest.json" || warning "Failed to parse results"
        else
            warning "Install jq to see detailed results summary"
        fi
    fi
    
    success "Performance report generated"
}

# Start monitoring
start_monitoring() {
    info "Starting performance monitoring..."
    
    cd "$ALEX_ROOT"
    go run ./cmd/perf monitor -config "$CONFIG_FILE" &
    MONITOR_PID=$!
    
    info "Performance monitoring started (PID: $MONITOR_PID)"
    echo "$MONITOR_PID" > "$PERF_DIR/monitor.pid"
    
    success "Monitoring is running in background"
}

# Stop monitoring
stop_monitoring() {
    if [[ -f "$PERF_DIR/monitor.pid" ]]; then
        local pid=$(cat "$PERF_DIR/monitor.pid")
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid"
            rm -f "$PERF_DIR/monitor.pid"
            success "Performance monitoring stopped"
        else
            warning "Monitoring process was not running"
            rm -f "$PERF_DIR/monitor.pid"
        fi
    else
        warning "No monitoring PID file found"
    fi
}

# CI/CD integration
ci_integration() {
    info "Running CI/CD performance verification..."
    
    local failed=0
    
    # Run pre-build checks
    info "Running pre-build verification..."
    cd "$ALEX_ROOT"
    if ! go run ./cmd/perf pre-build -config "$CONFIG_FILE"; then
        warning "Pre-build verification failed"
        ((failed++))
    fi
    
    # Run test scenarios (required for CI)
    if ! run_tests; then
        warning "Test scenarios failed"
        ((failed++))
    fi
    
    # Run post-test checks
    info "Running post-test verification..."
    if ! go run ./cmd/perf post-test -config "$CONFIG_FILE"; then
        warning "Post-test verification failed"
        ((failed++))
    fi
    
    if [[ $failed -eq 0 ]]; then
        success "CI/CD performance verification passed"
        return 0
    else
        error "CI/CD performance verification failed ($failed failures)"
    fi
}

# Full performance suite
full_suite() {
    info "Running full performance verification suite..."
    
    local start_time=$(date +%s)
    local failed=0
    
    # Initialize if needed
    if [[ ! -f "$CONFIG_FILE" ]]; then
        init_framework
    fi
    
    # Create baseline if needed
    if [[ ! -f "$PERF_DIR/baseline.json" ]]; then
        create_baseline
    fi
    
    # Run benchmarks
    if ! run_benchmarks; then
        ((failed++))
    fi
    
    # Run test scenarios
    if ! run_tests; then
        ((failed++))
    fi
    
    # Generate report
    generate_report
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    info "Performance suite completed in ${duration}s"
    
    if [[ $failed -eq 0 ]]; then
        success "Full performance suite passed"
        return 0
    else
        error "Performance suite had $failed failures"
    fi
}

# Cleanup function
cleanup() {
    info "Cleaning up performance data..."
    
    # Stop monitoring if running
    stop_monitoring
    
    # Clean old results (keep last 10)
    if [[ -d "$RESULTS_DIR" ]]; then
        find "$RESULTS_DIR" -name "results_*.json" -type f | sort -r | tail -n +11 | xargs rm -f
        success "Cleaned old results"
    fi
    
    # Clean old logs (keep last 7 days)
    find "$PERF_DIR" -name "*.log" -mtime +7 -delete 2>/dev/null || true
    
    success "Cleanup completed"
}

# Help function
show_help() {
    cat << EOF
Alex Performance Verification Framework - Automation Script

Usage: $0 <command> [options]

Commands:
    init            Initialize performance framework
    baseline        Create performance baseline
    benchmark       Run benchmark suite
    test            Run test scenarios
    monitor         Start performance monitoring
    stop-monitor    Stop performance monitoring
    report          Generate performance report
    ci              Run CI/CD integration checks
    full            Run full performance suite
    cleanup         Clean up old performance data
    help            Show this help message

Options:
    --verbose       Enable verbose output
    --config FILE   Use custom configuration file
    --no-color      Disable colored output

Examples:
    $0 init                    # Initialize framework
    $0 full                    # Run complete performance suite
    $0 ci                      # Run CI/CD checks
    $0 benchmark               # Run only benchmarks
    $0 test                    # Run only test scenarios
    $0 monitor                 # Start monitoring
    $0 cleanup                 # Clean old data

Environment Variables:
    ALEX_PERF_CONFIG           Path to configuration file
    ALEX_PERF_VERBOSE          Enable verbose mode (1/true)
    ALEX_PERF_NO_COLOR         Disable colors (1/true)

EOF
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --verbose)
                set -x
                shift
                ;;
            --config)
                CONFIG_FILE="$2"
                shift 2
                ;;
            --no-color)
                RED=''
                GREEN=''
                YELLOW=''
                BLUE=''
                NC=''
                shift
                ;;
            --help)
                show_help
                exit 0
                ;;
            *)
                break
                ;;
        esac
    done
}

# Main function
main() {
    # Parse arguments first
    parse_args "$@"
    
    # Set up environment
    export ALEX_ROOT
    export PERF_DIR
    
    # Check environment variables
    if [[ -n "$ALEX_PERF_CONFIG" ]]; then
        CONFIG_FILE="$ALEX_PERF_CONFIG"
    fi
    
    if [[ -n "$ALEX_PERF_VERBOSE" && "$ALEX_PERF_VERBOSE" != "0" ]]; then
        set -x
    fi
    
    if [[ -n "$ALEX_PERF_NO_COLOR" && "$ALEX_PERF_NO_COLOR" != "0" ]]; then
        RED=''
        GREEN=''
        YELLOW=''
        BLUE=''
        NC=''
    fi
    
    # Ensure log directory exists
    mkdir -p "$(dirname "$LOG_FILE")"
    
    # Check dependencies
    check_dependencies
    
    # Execute command
    local command="${1:-help}"
    case "$command" in
        init)
            init_framework
            ;;
        baseline)
            create_baseline
            ;;
        benchmark)
            run_benchmarks
            ;;
        test)
            run_tests
            ;;
        monitor)
            start_monitoring
            ;;
        stop-monitor)
            stop_monitoring
            ;;
        report)
            generate_report
            ;;
        ci)
            ci_integration
            ;;
        full)
            full_suite
            ;;
        cleanup)
            cleanup
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            error "Unknown command: $command. Use '$0 help' for usage."
            ;;
    esac
}

# Execute main function with all arguments
main "$@"