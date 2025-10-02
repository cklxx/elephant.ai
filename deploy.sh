#!/bin/bash

###############################################################################
# ALEX SSE 服务一键部署脚本
#
# 支持的部署模式:
#   1. local   - 本地开发模式 (Go + Next.js 本地运行)
#   2. docker  - Docker Compose 生产模式
#   3. dev     - Docker Compose 开发模式 (热重载)
#   4. k8s     - Kubernetes 集群部署
#
# 使用方法:
#   ./deploy.sh <mode> [options]
#
# 示例:
#   ./deploy.sh local          # 本地运行
#   ./deploy.sh docker         # Docker Compose 生产部署
#   ./deploy.sh dev            # Docker Compose 开发模式
#   ./deploy.sh k8s            # Kubernetes 部署
#   ./deploy.sh test           # 运行所有测试
#   ./deploy.sh down           # 停止所有服务
###############################################################################

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_NAME="alex-sse"
DEFAULT_API_KEY=${OPENAI_API_KEY:-""}

# Banner
print_banner() {
    echo -e "${PURPLE}"
    cat << "EOF"
    ___    __    _______  __  __       _______ _______  ______
   /   |  / /   / ____/ |/ / / /      / ____/ / ____/ / ____/
  / /| | / /   / __/  |   / / /      / /   / / __/   / __/
 / ___ |/ /___/ /___ /   | / /___   / /___/ / /___  / /___
/_/  |_/_____/_____//_/|_|/_____/   \____/_/_____/ /_____/

    AI Programming Agent - SSE Service Deployment
EOF
    echo -e "${NC}"
}

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${CYAN}[STEP]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    local mode=$1

    log_step "Checking prerequisites for mode: $mode"

    case $mode in
        local)
            command -v go >/dev/null 2>&1 || { log_error "Go is not installed. Please install Go 1.23+"; exit 1; }
            command -v node >/dev/null 2>&1 || { log_error "Node.js is not installed. Please install Node.js 20+"; exit 1; }
            command -v npm >/dev/null 2>&1 || { log_error "npm is not installed. Please install npm"; exit 1; }
            log_success "Go $(go version | awk '{print $3}')"
            log_success "Node $(node --version)"
            log_success "npm $(npm --version)"
            ;;
        docker|dev)
            command -v docker >/dev/null 2>&1 || { log_error "Docker is not installed. Please install Docker"; exit 1; }
            command -v docker-compose >/dev/null 2>&1 || { log_error "Docker Compose is not installed. Please install Docker Compose"; exit 1; }
            log_success "Docker $(docker --version | awk '{print $3}' | tr -d ',')"
            log_success "Docker Compose $(docker-compose --version | awk '{print $3}' | tr -d ',')"
            ;;
        k8s)
            command -v kubectl >/dev/null 2>&1 || { log_error "kubectl is not installed. Please install kubectl"; exit 1; }
            log_success "kubectl $(kubectl version --client --short 2>/dev/null | awk '{print $3}')"
            ;;
    esac
}

# Setup environment
setup_environment() {
    log_step "Setting up environment variables"

    if [ ! -f ".env" ]; then
        log_warning ".env file not found, creating from template"
        cat > .env << EOF
# ALEX SSE Service Configuration
OPENAI_API_KEY=${DEFAULT_API_KEY}
OPENAI_BASE_URL=https://api.openai.com/v1
ALEX_MODEL=gpt-4
ALEX_VERBOSE=false
REDIS_URL=
EOF
        log_info "Created .env file. Please edit it with your API key if not set."
    fi

    # Check if API key is set
    source .env
    if [ -z "$OPENAI_API_KEY" ] || [ "$OPENAI_API_KEY" == "" ]; then
        log_warning "OPENAI_API_KEY is not set in .env"
        read -p "Enter your OpenAI API key (or press Enter to skip): " api_key
        if [ -n "$api_key" ]; then
            sed -i.bak "s|OPENAI_API_KEY=.*|OPENAI_API_KEY=$api_key|" .env
            log_success "API key updated in .env"
        fi
    else
        log_success "API key configured: ${OPENAI_API_KEY:0:10}..."
    fi

    # Setup web environment
    if [ ! -f "web/.env.local" ]; then
        log_info "Creating web/.env.local"
        echo "NEXT_PUBLIC_API_URL=http://localhost:8080" > web/.env.local
        log_success "Created web/.env.local"
    fi
}

# Deploy local mode
deploy_local() {
    log_step "Deploying in LOCAL mode"

    # Build backend
    log_info "Building backend server..."
    make server-build
    log_success "Backend built successfully"

    # Install frontend dependencies
    log_info "Installing frontend dependencies..."
    cd web
    npm install --silent
    log_success "Frontend dependencies installed"
    cd ..

    # Start backend in background
    log_info "Starting backend server on :8080..."
    export OPENAI_API_KEY=$(grep OPENAI_API_KEY .env | cut -d '=' -f2-)
    ./alex-server > logs/server.log 2>&1 &
    SERVER_PID=$!
    echo $SERVER_PID > .server.pid
    log_success "Backend started (PID: $SERVER_PID)"

    # Wait for server to be ready
    log_info "Waiting for server to be ready..."
    for i in {1..30}; do
        if curl -s http://localhost:8080/health > /dev/null 2>&1; then
            log_success "Server is ready!"
            break
        fi
        if [ $i -eq 30 ]; then
            log_error "Server failed to start. Check logs/server.log"
            kill $SERVER_PID 2>/dev/null || true
            exit 1
        fi
        sleep 1
    done

    # Start frontend
    log_info "Starting frontend on :3000..."
    cd web
    npm run dev > ../logs/web.log 2>&1 &
    WEB_PID=$!
    echo $WEB_PID > ../.web.pid
    log_success "Frontend started (PID: $WEB_PID)"
    cd ..

    # Print access info
    echo ""
    log_success "================================"
    log_success " ALEX SSE Service is running!"
    log_success "================================"
    echo -e "${GREEN}Web UI:${NC}    http://localhost:3000"
    echo -e "${GREEN}API:${NC}       http://localhost:8080"
    echo -e "${GREEN}Health:${NC}    http://localhost:8080/health"
    echo -e "${GREEN}SSE:${NC}       http://localhost:8080/api/sse?session_id=xxx"
    echo ""
    echo -e "${YELLOW}Logs:${NC}"
    echo -e "  Server: ${CYAN}tail -f logs/server.log${NC}"
    echo -e "  Web:    ${CYAN}tail -f logs/web.log${NC}"
    echo ""
    echo -e "${YELLOW}Stop:${NC}   ${CYAN}./deploy.sh down${NC}"
    echo ""
}

# Deploy Docker mode
deploy_docker() {
    local mode=$1
    log_step "Deploying in DOCKER mode ($mode)"

    # Determine compose file
    if [ "$mode" == "dev" ]; then
        COMPOSE_FILE="docker-compose.dev.yml"
    else
        COMPOSE_FILE="docker-compose.yml"
    fi

    # Build images
    log_info "Building Docker images..."
    docker-compose -f $COMPOSE_FILE build
    log_success "Images built successfully"

    # Start services
    log_info "Starting services..."
    docker-compose -f $COMPOSE_FILE up -d
    log_success "Services started"

    # Wait for services
    log_info "Waiting for services to be ready..."
    sleep 5

    # Check health
    for i in {1..30}; do
        if curl -s http://localhost:8080/health > /dev/null 2>&1; then
            log_success "Server is healthy!"
            break
        fi
        if [ $i -eq 30 ]; then
            log_error "Server health check failed"
            docker-compose -f $COMPOSE_FILE logs alex-server
            exit 1
        fi
        sleep 1
    done

    # Print access info
    echo ""
    log_success "================================"
    log_success " Docker Deployment Complete!"
    log_success "================================"
    echo -e "${GREEN}Web UI:${NC}    http://localhost:3000"
    echo -e "${GREEN}API:${NC}       http://localhost:8080"
    echo -e "${GREEN}Nginx:${NC}     http://localhost (if enabled)"
    echo ""
    echo -e "${YELLOW}Commands:${NC}"
    echo -e "  Status:    ${CYAN}docker-compose -f $COMPOSE_FILE ps${NC}"
    echo -e "  Logs:      ${CYAN}docker-compose -f $COMPOSE_FILE logs -f${NC}"
    echo -e "  Stop:      ${CYAN}./deploy.sh down${NC}"
    echo ""
}

# Deploy Kubernetes
deploy_k8s() {
    log_step "Deploying to Kubernetes"

    # Check cluster connection
    log_info "Checking cluster connection..."
    kubectl cluster-info > /dev/null 2>&1 || { log_error "Cannot connect to Kubernetes cluster"; exit 1; }
    log_success "Connected to cluster"

    # Update secret
    log_info "Updating secrets..."
    source .env
    kubectl create secret generic alex-secrets \
        --from-literal=OPENAI_API_KEY=$OPENAI_API_KEY \
        -n alex-system \
        --dry-run=client -o yaml | kubectl apply -f -
    log_success "Secrets updated"

    # Apply manifests
    log_info "Applying Kubernetes manifests..."
    kubectl apply -f k8s/deployment.yaml
    log_success "Manifests applied"

    # Wait for deployment
    log_info "Waiting for deployment to be ready..."
    kubectl wait --for=condition=available --timeout=300s \
        deployment/alex-server -n alex-system
    kubectl wait --for=condition=available --timeout=300s \
        deployment/alex-web -n alex-system
    log_success "Deployments are ready"

    # Get service info
    echo ""
    log_success "================================"
    log_success " Kubernetes Deployment Complete!"
    log_success "================================"
    echo ""
    log_info "Pods:"
    kubectl get pods -n alex-system
    echo ""
    log_info "Services:"
    kubectl get svc -n alex-system
    echo ""
    log_info "Ingress:"
    kubectl get ingress -n alex-system
    echo ""
    echo -e "${YELLOW}Port Forward (for testing):${NC}"
    echo -e "  ${CYAN}kubectl port-forward svc/alex-web-service 3000:3000 -n alex-system${NC}"
    echo -e "  ${CYAN}kubectl port-forward svc/alex-server-service 8080:8080 -n alex-system${NC}"
    echo ""
}

# Run tests
run_tests() {
    log_step "Running all tests"

    # Backend unit tests
    log_info "Running backend unit tests..."
    make server-test
    log_success "Backend tests passed"

    # Build backend
    log_info "Building backend for integration tests..."
    make server-build
    log_success "Backend built"

    # Start server for integration tests
    log_info "Starting server for integration tests..."
    export OPENAI_API_KEY=$(grep OPENAI_API_KEY .env | cut -d '=' -f2-)
    ./alex-server > logs/test-server.log 2>&1 &
    TEST_SERVER_PID=$!

    # Wait for server
    sleep 3

    # Run integration tests
    log_info "Running integration tests..."
    if ./scripts/integration-test.sh http://localhost:8080; then
        log_success "Integration tests passed"
    else
        log_error "Integration tests failed"
        kill $TEST_SERVER_PID 2>/dev/null || true
        exit 1
    fi

    # Cleanup
    kill $TEST_SERVER_PID 2>/dev/null || true

    echo ""
    log_success "================================"
    log_success " All Tests Passed!"
    log_success "================================"
    echo ""
}

# Stop services
stop_services() {
    log_step "Stopping all services"

    # Stop local services
    if [ -f ".server.pid" ]; then
        SERVER_PID=$(cat .server.pid)
        log_info "Stopping backend server (PID: $SERVER_PID)..."
        kill $SERVER_PID 2>/dev/null || true
        rm .server.pid
        log_success "Backend stopped"
    fi

    if [ -f ".web.pid" ]; then
        WEB_PID=$(cat .web.pid)
        log_info "Stopping frontend (PID: $WEB_PID)..."
        kill $WEB_PID 2>/dev/null || true
        rm .web.pid
        log_success "Frontend stopped"
    fi

    # Stop Docker services
    if [ -f "docker-compose.yml" ]; then
        log_info "Stopping Docker Compose services..."
        docker-compose down 2>/dev/null || true
        docker-compose -f docker-compose.dev.yml down 2>/dev/null || true
        log_success "Docker services stopped"
    fi

    log_success "All services stopped"
}

# Show status
show_status() {
    log_step "Service Status"
    echo ""

    # Local services
    if [ -f ".server.pid" ] && kill -0 $(cat .server.pid) 2>/dev/null; then
        echo -e "${GREEN}✓${NC} Backend Server:  Running (PID: $(cat .server.pid))"
        curl -s http://localhost:8080/health | jq '.' 2>/dev/null || echo "  Health check failed"
    else
        echo -e "${RED}✗${NC} Backend Server:  Not running"
    fi

    if [ -f ".web.pid" ] && kill -0 $(cat .web.pid) 2>/dev/null; then
        echo -e "${GREEN}✓${NC} Frontend:        Running (PID: $(cat .web.pid))"
    else
        echo -e "${RED}✗${NC} Frontend:        Not running"
    fi

    echo ""

    # Docker services
    if docker ps --format '{{.Names}}' | grep -q "alex-"; then
        echo -e "${YELLOW}Docker Services:${NC}"
        docker-compose ps 2>/dev/null || docker-compose -f docker-compose.dev.yml ps 2>/dev/null || true
    fi

    echo ""
}

# Show usage
show_usage() {
    cat << EOF
${CYAN}ALEX SSE Service Deployment Tool${NC}

${YELLOW}Usage:${NC}
  ./deploy.sh <command> [options]

${YELLOW}Commands:${NC}
  ${GREEN}local${NC}          Deploy locally (Go + Next.js)
  ${GREEN}docker${NC}         Deploy with Docker Compose (production)
  ${GREEN}dev${NC}            Deploy with Docker Compose (development)
  ${GREEN}k8s${NC}            Deploy to Kubernetes
  ${GREEN}test${NC}           Run all tests
  ${GREEN}status${NC}         Show service status
  ${GREEN}down${NC}           Stop all services
  ${GREEN}help${NC}           Show this help message

${YELLOW}Examples:${NC}
  ./deploy.sh local          # Start locally
  ./deploy.sh docker         # Docker production
  ./deploy.sh test           # Run tests
  ./deploy.sh down           # Stop everything
  ./deploy.sh status         # Check status

${YELLOW}Environment:${NC}
  Configure via .env file (created automatically)

${YELLOW}Documentation:${NC}
  - Quick Start:  QUICKSTART_SSE.md
  - Deployment:   DEPLOYMENT.md
  - Architecture: docs/design/SSE_WEB_ARCHITECTURE.md

EOF
}

# Main function
main() {
    # Create logs directory
    mkdir -p logs

    # Parse command
    COMMAND=${1:-help}

    case $COMMAND in
        local)
            print_banner
            check_prerequisites local
            setup_environment
            deploy_local
            ;;
        docker)
            print_banner
            check_prerequisites docker
            setup_environment
            deploy_docker production
            ;;
        dev)
            print_banner
            check_prerequisites dev
            setup_environment
            deploy_docker dev
            ;;
        k8s)
            print_banner
            check_prerequisites k8s
            setup_environment
            deploy_k8s
            ;;
        test)
            print_banner
            setup_environment
            run_tests
            ;;
        status)
            show_status
            ;;
        down|stop)
            stop_services
            ;;
        help|--help|-h)
            show_usage
            ;;
        *)
            log_error "Unknown command: $COMMAND"
            echo ""
            show_usage
            exit 1
            ;;
    esac
}

# Run main
main "$@"
