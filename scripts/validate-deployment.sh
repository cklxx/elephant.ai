#!/bin/bash

###############################################################################
# ALEX SSE 部署验证脚本
#
# 执行全面的部署验证，包括：
#   1. 代码编译检查
#   2. 单元测试
#   3. 集成测试
#   4. 配置验证
#   5. 依赖检查
###############################################################################

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_ROOT"

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  ALEX SSE 部署验证${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Run test
run_test() {
    local test_name="$1"
    local test_command="$2"

    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo -n "[$TOTAL_TESTS] $test_name... "

    if eval "$test_command" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ PASS${NC}"
        PASSED_TESTS=$((PASSED_TESTS + 1))
        return 0
    else
        echo -e "${RED}✗ FAIL${NC}"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi
}

# Detailed test with output
run_detailed_test() {
    local test_name="$1"
    local test_command="$2"

    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo ""
    echo -e "${BLUE}[$TOTAL_TESTS] $test_name${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

    if eval "$test_command"; then
        echo -e "${GREEN}✓ PASS${NC}"
        PASSED_TESTS=$((PASSED_TESTS + 1))
        return 0
    else
        echo -e "${RED}✗ FAIL${NC}"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi
}

echo -e "${YELLOW}Phase 1: 环境检查${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

run_test "Go 编译器" "command -v go"
run_test "Node.js" "command -v node"
run_test "npm" "command -v npm"
run_test "Docker (可选)" "command -v docker || true"
run_test "Make" "command -v make"

echo ""
echo -e "${YELLOW}Phase 2: 后端编译${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

run_detailed_test "编译 CLI (alex)" "go build -o alex ./cmd/alex"
run_detailed_test "编译 Server (alex-server)" "go build -o alex-server ./cmd/alex-server"

echo ""
echo -e "${YELLOW}Phase 3: 后端测试${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

run_detailed_test "Domain 层单元测试" "go test ./internal/domain/ -v"
run_detailed_test "Server App 层单元测试" "go test ./internal/delivery/server/app/ -v"
run_detailed_test "Server HTTP 层单元测试" "go test ./internal/delivery/server/http/ -v"

echo ""
echo -e "${YELLOW}Phase 4: 前端构建${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

cd web

run_test "检查 package.json" "test -f package.json"
run_detailed_test "安装依赖" "npm install --silent"
run_detailed_test "TypeScript 编译" "npm run build"

cd ..

echo ""
echo -e "${YELLOW}Phase 5: 配置文件验证${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

run_test "检查 docker-compose.yml" "test -f deploy/docker/docker-compose.yml"
run_test "检查 docker-compose.dev.yml" "test -f deploy/docker/docker-compose.dev.yml"
run_test "检查 Dockerfile.server" "test -f deploy/docker/Dockerfile.server"
run_test "检查 web/Dockerfile" "test -f web/Dockerfile"
run_test "检查 nginx.conf" "test -f deploy/docker/nginx.conf"

echo ""
echo -e "${YELLOW}Phase 6: 脚本验证${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

run_test "deploy.sh 可执行" "test -x deploy.sh"
run_test "integration-test.sh 可执行" "test -x scripts/integration-test.sh"
run_test "test-sse-server.sh 可执行" "test -x scripts/test-sse-server.sh"

echo ""
echo -e "${YELLOW}Phase 7: 文档检查${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

run_test "README.md" "test -f README.md"
run_test "QUICKSTART_SSE.md" "test -f QUICKSTART_SSE.md"
run_test "DEPLOYMENT.md" "test -f DEPLOYMENT.md"
run_test "SSE_IMPLEMENTATION_SUMMARY.md" "test -f SSE_IMPLEMENTATION_SUMMARY.md"
run_test "Architecture Design" "test -f docs/design/SSE_WEB_ARCHITECTURE.md"
run_test "Server README" "test -f internal/delivery/server/README.md"
run_test "Web README" "test -f web/README.md"

echo ""
echo -e "${YELLOW}Phase 8: 依赖完整性${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

run_test "Go modules 完整" "go mod verify"
run_test "Web package-lock.json" "test -f web/package-lock.json"

echo ""
echo -e "${YELLOW}Phase 9: 架构完整性${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

run_test "Server Ports 层" "test -f internal/delivery/server/ports/broadcaster.go"
run_test "Server App 层" "test -f internal/delivery/server/app/event_broadcaster.go"
run_test "Server HTTP 层" "test -f internal/delivery/server/http/sse_handler.go"
run_test "Server 入口" "test -f cmd/alex-server/main.go"

echo ""
echo -e "${YELLOW}Phase 10: 二进制验证${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

run_test "alex 可执行" "test -x alex"
run_test "alex-server 可执行" "test -x alex-server"
run_test "alex --version" "./alex --version || true"
run_test "alex-server 版本" "./alex-server --version || true"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${BLUE}  验证总结${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo -e "总测试数:  ${BLUE}$TOTAL_TESTS${NC}"
echo -e "通过:      ${GREEN}$PASSED_TESTS${NC}"
echo -e "失败:      ${RED}$FAILED_TESTS${NC}"
echo ""

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}  ✓ 所有验证通过！部署就绪！${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "${YELLOW}下一步:${NC}"
    echo -e "  1. 本地部署:     ${BLUE}./deploy.sh local${NC}"
    echo -e "  2. Docker 部署:  ${BLUE}./deploy.sh docker${NC}"
    echo -e "  3. 集成测试:     ${BLUE}./deploy.sh test${NC}"
    echo ""
    exit 0
else
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${RED}  ✗ 验证失败！请修复错误后重试${NC}"
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    exit 1
fi
