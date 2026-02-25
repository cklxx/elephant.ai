#!/bin/bash

# ALEX 集成测试运行脚本
# 支持多种测试模式和配置选项

set -e

# 脚本目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TESTS_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# 默认配置
DEFAULT_TIMEOUT="10m"
DEFAULT_PARALLEL=4
DEFAULT_VERBOSE=false
DEFAULT_COVERAGE=false
DEFAULT_OUTPUT_DIR="$TESTS_ROOT/reports"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_header() {
    echo -e "${PURPLE}================================${NC}"
    echo -e "${PURPLE} $1 ${NC}"
    echo -e "${PURPLE}================================${NC}"
}

# 显示帮助信息
show_help() {
    cat << EOF
ALEX 集成测试运行脚本

用法: $0 [OPTIONS] [TEST_SUITE]

测试套件:
  all              运行所有集成测试 (默认)
  api              API集成测试
  websocket        WebSocket集成测试
  e2e              端到端测试
  session          会话管理测试
  performance      性能测试
  stress           压力测试
  load             负载测试

选项:
  -t, --timeout DURATION     测试超时时间 (默认: $DEFAULT_TIMEOUT)
  -p, --parallel NUM          并行测试数量 (默认: $DEFAULT_PARALLEL)
  -v, --verbose              详细输出
  -c, --coverage             生成覆盖率报告
  -o, --output DIR           输出目录 (默认: $DEFAULT_OUTPUT_DIR)
  -f, --filter PATTERN       测试过滤模式
  -r, --retry NUM            失败重试次数 (默认: 0)
  --no-cleanup               不清理测试数据
  --debug                    调试模式
  --ci                       CI模式 (优化输出格式)
  -h, --help                 显示帮助信息

环境变量:
  ALEX_TEST_TIMEOUT          测试超时时间
  ALEX_TEST_PARALLEL         并行数量
  ALEX_TEST_VERBOSE          详细输出 (true/false)
  ALEX_TEST_COVERAGE         覆盖率 (true/false)
  ALEX_TEST_OUTPUT_DIR       输出目录
  ALEX_TEST_DEBUG            调试模式 (true/false)
  ALEX_TEST_NO_CLEANUP       不清理 (true/false)

示例:
  $0                         # 运行所有集成测试
  $0 api                     # 运行API测试
  $0 -v -c performance       # 详细输出和覆盖率的性能测试
  $0 --filter "TestBasic*"   # 运行匹配模式的测试
  $0 --timeout 30m stress    # 30分钟超时的压力测试
  $0 --ci all                # CI模式运行所有测试

EOF
}

# 解析命令行参数
parse_args() {
    TIMEOUT="${ALEX_TEST_TIMEOUT:-$DEFAULT_TIMEOUT}"
    PARALLEL="${ALEX_TEST_PARALLEL:-$DEFAULT_PARALLEL}"
    VERBOSE="${ALEX_TEST_VERBOSE:-$DEFAULT_VERBOSE}"
    COVERAGE="${ALEX_TEST_COVERAGE:-$DEFAULT_COVERAGE}"
    OUTPUT_DIR="${ALEX_TEST_OUTPUT_DIR:-$DEFAULT_OUTPUT_DIR}"
    DEBUG="${ALEX_TEST_DEBUG:-false}"
    NO_CLEANUP="${ALEX_TEST_NO_CLEANUP:-false}"
    CI_MODE=false
    FILTER=""
    RETRY=0
    TEST_SUITE="all"

    while [[ $# -gt 0 ]]; do
        case $1 in
            -t|--timeout)
                TIMEOUT="$2"
                shift 2
                ;;
            -p|--parallel)
                PARALLEL="$2"
                shift 2
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -c|--coverage)
                COVERAGE=true
                shift
                ;;
            -o|--output)
                OUTPUT_DIR="$2"
                shift 2
                ;;
            -f|--filter)
                FILTER="$2"
                shift 2
                ;;
            -r|--retry)
                RETRY="$2"
                shift 2
                ;;
            --no-cleanup)
                NO_CLEANUP=true
                shift
                ;;
            --debug)
                DEBUG=true
                shift
                ;;
            --ci)
                CI_MODE=true
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            all|api|websocket|e2e|session|performance|stress|load)
                TEST_SUITE="$1"
                shift
                ;;
            *)
                log_error "未知参数: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# 验证环境
validate_environment() {
    log_info "验证测试环境..."

    # 检查Go版本
    if ! command -v go &> /dev/null; then
        log_error "Go未安装或不在PATH中"
        exit 1
    fi

    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    log_info "Go版本: $GO_VERSION"

    # 检查项目根目录
    if [ ! -f "$PROJECT_ROOT/go.mod" ]; then
        log_error "找不到go.mod文件，请确保在正确的项目目录中运行"
        exit 1
    fi

    # 检查Alex二进制文件
    if [ ! -f "$PROJECT_ROOT/alex" ]; then
        log_warning "Alex二进制文件不存在，尝试构建..."
        cd "$PROJECT_ROOT"
        make build
        if [ ! -f "$PROJECT_ROOT/alex" ]; then
            log_error "构建Alex失败"
            exit 1
        fi
    fi

    log_success "环境验证通过"
}

# 准备测试环境
prepare_test_environment() {
    log_info "准备测试环境..."

    # 创建输出目录
    mkdir -p "$OUTPUT_DIR"
    mkdir -p "$OUTPUT_DIR/coverage"
    mkdir -p "$OUTPUT_DIR/logs"
    mkdir -p "$OUTPUT_DIR/reports"

    # 清理之前的测试数据（如果不是no-cleanup模式）
    if [ "$NO_CLEANUP" != "true" ]; then
        log_info "清理之前的测试数据..."
        rm -rf /tmp/alex_*test*
        rm -rf "$OUTPUT_DIR"/*.log
        rm -rf "$OUTPUT_DIR"/*.json
    fi

    # 设置环境变量
    export ALEX_TEST_MODE=true
    export ALEX_TEST_OUTPUT_DIR="$OUTPUT_DIR"
    export ALEX_DEBUG="$DEBUG"

    log_success "测试环境准备完成"
}

# 构建Go测试参数
build_go_test_args() {
    local test_package="$1"
    local args=()

    # 基础参数
    args+=("-timeout" "$TIMEOUT")

    if [ "$VERBOSE" = "true" ]; then
        args+=("-v")
    fi

    if [ "$COVERAGE" = "true" ]; then
        local coverage_file="$OUTPUT_DIR/coverage/$(basename "$test_package").coverage"
        args+=("-coverprofile=$coverage_file")
        args+=("-covermode=atomic")
    fi

    # 并行设置
    if [ "$PARALLEL" -gt 1 ]; then
        args+=("-parallel" "$PARALLEL")
    fi

    # 过滤器
    if [ -n "$FILTER" ]; then
        args+=("-run" "$FILTER")
    fi

    # 输出格式
    if [ "$CI_MODE" = "true" ]; then
        args+=("-json")
    fi

    echo "${args[@]}"
}

# 运行单个测试套件
run_test_suite() {
    local suite_name="$1"
    local test_package="$2"
    local description="$3"

    log_header "运行 $description"

    local start_time=$(date +%s)
    local log_file="$OUTPUT_DIR/logs/${suite_name}.log"
    local json_file="$OUTPUT_DIR/reports/${suite_name}.json"

    # 构建测试命令
    local test_args=($(build_go_test_args "$test_package"))
    local cmd="go test ${test_args[@]} $test_package"

    log_info "执行命令: $cmd"
    log_info "日志文件: $log_file"

    # 执行测试
    local exit_code=0
    if [ "$CI_MODE" = "true" ]; then
        # CI模式：JSON输出
        eval "$cmd" > "$json_file" 2>&1 || exit_code=$?
        # 同时输出到日志文件（人类可读格式）
        go test -v "${test_args[@]}" "$test_package" > "$log_file" 2>&1 || true
    else
        # 正常模式：标准输出
        eval "$cmd" 2>&1 | tee "$log_file" || exit_code=$?
    fi

    local end_time=$(date +%s)
    local duration=$((end_time - start_time))

    # 分析结果
    local total_tests=0
    local passed_tests=0
    local failed_tests=0
    local skipped_tests=0

    if [ -f "$log_file" ]; then
        # 从日志文件中提取测试统计
        total_tests=$(grep -c "^=== RUN" "$log_file" 2>/dev/null || echo 0)
        passed_tests=$(grep -c "^--- PASS:" "$log_file" 2>/dev/null || echo 0)
        failed_tests=$(grep -c "^--- FAIL:" "$log_file" 2>/dev/null || echo 0)
        skipped_tests=$(grep -c "^--- SKIP:" "$log_file" 2>/dev/null || echo 0)
    fi

    # 输出结果摘要
    echo ""
    log_info "测试套件: $description"
    log_info "执行时间: ${duration}秒"
    log_info "总测试数: $total_tests"
    log_info "通过: $passed_tests"
    log_info "失败: $failed_tests"
    log_info "跳过: $skipped_tests"

    if [ $exit_code -eq 0 ]; then
        log_success "$description - 测试通过"
    else
        log_error "$description - 测试失败 (退出码: $exit_code)"

        # 重试机制
        if [ "$RETRY" -gt 0 ]; then
            log_warning "重试 $description (剩余重试次数: $RETRY)"
            RETRY=$((RETRY - 1))
            run_test_suite "$suite_name" "$test_package" "$description"
            return $?
        fi
    fi

    return $exit_code
}

# 运行API集成测试
run_api_tests() {
    run_test_suite "api" "$TESTS_ROOT/integration/api" "API集成测试"
}

# 运行WebSocket集成测试
run_websocket_tests() {
    run_test_suite "websocket" "$TESTS_ROOT/integration/websocket" "WebSocket集成测试"
}

# 运行端到端测试
run_e2e_tests() {
    run_test_suite "e2e" "$TESTS_ROOT/integration/e2e" "端到端测试"
}

# 运行会话测试
run_session_tests() {
    run_test_suite "session" "$TESTS_ROOT/integration/session" "会话管理测试"
}

# 运行性能测试
run_performance_tests() {
    # 性能测试需要更长的超时时间
    local original_timeout="$TIMEOUT"
    if [ "$TIMEOUT" = "$DEFAULT_TIMEOUT" ]; then
        TIMEOUT="30m"
    fi

    run_test_suite "performance" "$TESTS_ROOT/performance/benchmark" "性能基准测试"

    TIMEOUT="$original_timeout"
}

# 运行压力测试
run_stress_tests() {
    # 压力测试需要更长的超时时间
    local original_timeout="$TIMEOUT"
    if [ "$TIMEOUT" = "$DEFAULT_TIMEOUT" ]; then
        TIMEOUT="45m"
    fi

    run_test_suite "stress" "$TESTS_ROOT/performance/stress" "压力测试"

    TIMEOUT="$original_timeout"
}

# 运行负载测试
run_load_tests() {
    # 负载测试需要更长的超时时间
    local original_timeout="$TIMEOUT"
    if [ "$TIMEOUT" = "$DEFAULT_TIMEOUT" ]; then
        TIMEOUT="20m"
    fi

    run_test_suite "load" "$TESTS_ROOT/performance/load" "负载测试"

    TIMEOUT="$original_timeout"
}

# 生成覆盖率报告
generate_coverage_report() {
    if [ "$COVERAGE" != "true" ]; then
        return 0
    fi

    log_header "生成覆盖率报告"

    local coverage_files=()
    for file in "$OUTPUT_DIR"/coverage/*.coverage; do
        if [ -f "$file" ]; then
            coverage_files+=("$file")
        fi
    done

    if [ ${#coverage_files[@]} -eq 0 ]; then
        log_warning "没有找到覆盖率文件"
        return 0
    fi

    # 合并覆盖率文件
    local merged_coverage="$OUTPUT_DIR/coverage/merged.coverage"
    log_info "合并覆盖率文件..."

    # 使用gocovmerge合并（如果可用），否则使用简单合并
    if command -v gocovmerge &> /dev/null; then
        gocovmerge "${coverage_files[@]}" > "$merged_coverage"
    else
        # 简单合并（可能有重复）
        cat "${coverage_files[@]}" > "$merged_coverage"
    fi

    # 生成HTML报告
    local html_report="$OUTPUT_DIR/coverage/coverage.html"
    log_info "生成HTML覆盖率报告: $html_report"
    go tool cover -html="$merged_coverage" -o "$html_report"

    # 生成文本摘要
    local coverage_summary="$OUTPUT_DIR/coverage/summary.txt"
    log_info "生成覆盖率摘要: $coverage_summary"
    go tool cover -func="$merged_coverage" > "$coverage_summary"

    # 显示覆盖率摘要
    local total_coverage=$(tail -1 "$coverage_summary" | awk '{print $3}')
    log_info "总覆盖率: $total_coverage"

    log_success "覆盖率报告生成完成"
}

# 生成测试报告
generate_test_report() {
    log_header "生成测试报告"

    local report_file="$OUTPUT_DIR/test_report.html"
    local summary_file="$OUTPUT_DIR/test_summary.json"

    # 创建JSON摘要
    cat > "$summary_file" << EOF
{
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "environment": {
    "go_version": "$(go version | awk '{print $3}')",
    "os": "$(uname -s)",
    "arch": "$(uname -m)"
  },
  "configuration": {
    "timeout": "$TIMEOUT",
    "parallel": $PARALLEL,
    "verbose": $VERBOSE,
    "coverage": $COVERAGE,
    "test_suite": "$TEST_SUITE",
    "filter": "$FILTER"
  },
  "results": {
    "suites": []
  }
}
EOF

    # 分析每个测试套件的结果
    for log_file in "$OUTPUT_DIR"/logs/*.log; do
        if [ -f "$log_file" ]; then
            local suite_name=$(basename "$log_file" .log)
            local total=$(grep -c "^=== RUN" "$log_file" 2>/dev/null || echo 0)
            local passed=$(grep -c "^--- PASS:" "$log_file" 2>/dev/null || echo 0)
            local failed=$(grep -c "^--- FAIL:" "$log_file" 2>/dev/null || echo 0)
            local skipped=$(grep -c "^--- SKIP:" "$log_file" 2>/dev/null || echo 0)

            # 更新JSON摘要（简化实现）
            log_info "分析 $suite_name: 总计 $total, 通过 $passed, 失败 $failed, 跳过 $skipped"
        fi
    done

    # 创建简单的HTML报告
    cat > "$report_file" << EOF
<!DOCTYPE html>
<html>
<head>
    <title>ALEX 集成测试报告</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .header { background: #f8f9fa; padding: 20px; border-radius: 5px; }
        .suite { margin: 20px 0; padding: 15px; border: 1px solid #ddd; border-radius: 5px; }
        .pass { color: green; }
        .fail { color: red; }
        .skip { color: orange; }
        .timestamp { color: #666; font-size: 0.9em; }
    </style>
</head>
<body>
    <div class="header">
        <h1>ALEX 集成测试报告</h1>
        <p class="timestamp">生成时间: $(date)</p>
        <p>测试套件: $TEST_SUITE</p>
        <p>配置: 超时 $TIMEOUT, 并行 $PARALLEL</p>
    </div>

    <h2>测试结果</h2>
EOF

    # 添加每个测试套件的详细结果
    for log_file in "$OUTPUT_DIR"/logs/*.log; do
        if [ -f "$log_file" ]; then
            local suite_name=$(basename "$log_file" .log)
            local total=$(grep -c "^=== RUN" "$log_file" 2>/dev/null || echo 0)
            local passed=$(grep -c "^--- PASS:" "$log_file" 2>/dev/null || echo 0)
            local failed=$(grep -c "^--- FAIL:" "$log_file" 2>/dev/null || echo 0)

            cat >> "$report_file" << EOF
    <div class="suite">
        <h3>$suite_name</h3>
        <p>总计: $total | <span class="pass">通过: $passed</span> | <span class="fail">失败: $failed</span></p>
    </div>
EOF
        fi
    done

    cat >> "$report_file" << EOF
</body>
</html>
EOF

    log_info "HTML报告: $report_file"
    log_info "JSON摘要: $summary_file"
    log_success "测试报告生成完成"
}

# 清理测试数据
cleanup_test_data() {
    if [ "$NO_CLEANUP" = "true" ]; then
        log_info "跳过清理 (--no-cleanup)"
        return 0
    fi

    log_info "清理测试数据..."

    # 清理临时文件
    rm -rf /tmp/alex_*test*

    # 清理进程
    pkill -f "alex.*test" 2>/dev/null || true

    log_success "清理完成"
}

# 主函数
main() {
    # 解析参数
    parse_args "$@"

    # 显示配置
    log_header "ALEX 集成测试"
    log_info "测试套件: $TEST_SUITE"
    log_info "超时时间: $TIMEOUT"
    log_info "并行数量: $PARALLEL"
    log_info "详细输出: $VERBOSE"
    log_info "覆盖率: $COVERAGE"
    log_info "输出目录: $OUTPUT_DIR"
    if [ -n "$FILTER" ]; then
        log_info "过滤器: $FILTER"
    fi

    # 验证环境
    validate_environment

    # 准备测试环境
    prepare_test_environment

    # 记录开始时间
    local start_time=$(date +%s)
    local overall_exit_code=0

    # 根据测试套件运行相应测试
    case "$TEST_SUITE" in
        "all")
            run_api_tests || overall_exit_code=1
            run_websocket_tests || overall_exit_code=1
            run_session_tests || overall_exit_code=1
            run_e2e_tests || overall_exit_code=1
            ;;
        "api")
            run_api_tests || overall_exit_code=1
            ;;
        "websocket")
            run_websocket_tests || overall_exit_code=1
            ;;
        "e2e")
            run_e2e_tests || overall_exit_code=1
            ;;
        "session")
            run_session_tests || overall_exit_code=1
            ;;
        "performance")
            run_performance_tests || overall_exit_code=1
            ;;
        "stress")
            run_stress_tests || overall_exit_code=1
            ;;
        "load")
            run_load_tests || overall_exit_code=1
            ;;
        *)
            log_error "未知的测试套件: $TEST_SUITE"
            exit 1
            ;;
    esac

    # 生成报告
    generate_coverage_report
    generate_test_report

    # 计算总时间
    local end_time=$(date +%s)
    local total_duration=$((end_time - start_time))

    # 清理
    cleanup_test_data

    # 最终结果
    log_header "测试完成"
    log_info "总耗时: ${total_duration}秒"
    log_info "结果文件: $OUTPUT_DIR"

    if [ $overall_exit_code -eq 0 ]; then
        log_success "所有测试通过！"
    else
        log_error "部分测试失败"
    fi

    exit $overall_exit_code
}

# 执行主函数
main "$@"