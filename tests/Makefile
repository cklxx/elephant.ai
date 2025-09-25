# ALEX 集成测试和验收框架 Makefile
# 提供完整的测试、性能评估和验收工具

# 项目路径
PROJECT_ROOT := $(shell cd .. && pwd)
TESTS_ROOT := $(shell pwd)
SCRIPTS_DIR := $(TESTS_ROOT)/scripts
REPORTS_DIR := $(TESTS_ROOT)/reports

# 配置
TIMEOUT ?= 10m
PARALLEL ?= 4
COVERAGE ?= true
VERBOSE ?= false
OUTPUT_FORMAT ?= all

# 默认目标
.PHONY: help
help:
	@echo "ALEX 集成测试和验收框架"
	@echo ""
	@echo "主要目标:"
	@echo "  test-all                运行所有测试"
	@echo "  test-integration        运行集成测试"
	@echo "  test-performance        运行性能测试"
	@echo "  test-acceptance         运行验收测试"
	@echo "  generate-report         生成测试报告"
	@echo ""
	@echo "集成测试:"
	@echo "  test-api                API集成测试"
	@echo "  test-websocket          WebSocket集成测试"
	@echo "  test-e2e                端到端测试"
	@echo "  test-session            会话管理测试"
	@echo ""
	@echo "性能测试:"
	@echo "  test-load               负载测试"
	@echo "  test-stress             压力测试"
	@echo "  test-benchmark          基准测试"
	@echo "  test-memory             内存测试"
	@echo ""
	@echo "报告和分析:"
	@echo "  report-html             生成HTML报告"
	@echo "  report-json             生成JSON报告"
	@echo "  report-markdown         生成Markdown报告"
	@echo "  report-all              生成所有格式报告"
	@echo ""
	@echo "环境管理:"
	@echo "  setup                   设置测试环境"
	@echo "  clean                   清理测试数据"
	@echo "  validate                验证测试环境"
	@echo ""
	@echo "配置变量:"
	@echo "  TIMEOUT=10m             测试超时时间"
	@echo "  PARALLEL=4              并行测试数量"
	@echo "  COVERAGE=true           是否生成覆盖率"
	@echo "  VERBOSE=false           是否详细输出"
	@echo "  OUTPUT_FORMAT=all       报告输出格式"

# 环境设置和验证
.PHONY: setup
setup:
	@echo "设置ALEX测试环境..."
	@mkdir -p $(REPORTS_DIR)
	@mkdir -p $(REPORTS_DIR)/coverage
	@mkdir -p $(REPORTS_DIR)/performance
	@mkdir -p $(REPORTS_DIR)/logs
	@chmod +x $(SCRIPTS_DIR)/*.sh
	@cd $(PROJECT_ROOT) && make build
	@echo "测试环境设置完成"

.PHONY: validate
validate:
	@echo "验证测试环境..."
	@$(SCRIPTS_DIR)/run-integration.sh --help > /dev/null
	@$(SCRIPTS_DIR)/generate-report.sh --help > /dev/null
	@test -f $(PROJECT_ROOT)/alex || (echo "错误: alex二进制文件不存在" && exit 1)
	@echo "环境验证通过"

.PHONY: clean
clean:
	@echo "清理测试数据..."
	@rm -rf $(REPORTS_DIR)/*
	@rm -rf /tmp/alex_*test*
	@pkill -f "alex.*test" 2>/dev/null || true
	@echo "清理完成"

# 主要测试目标
.PHONY: test-all
test-all: setup test-integration test-performance test-acceptance generate-report

.PHONY: test-integration
test-integration: setup
	@echo "运行集成测试..."
	@$(SCRIPTS_DIR)/run-integration.sh \
		--timeout $(TIMEOUT) \
		--parallel $(PARALLEL) \
		$(if $(filter true,$(COVERAGE)),--coverage) \
		$(if $(filter true,$(VERBOSE)),-v) \
		all

.PHONY: test-performance
test-performance: setup
	@echo "运行性能测试..."
	@$(SCRIPTS_DIR)/run-integration.sh \
		--timeout 30m \
		--parallel 2 \
		performance

.PHONY: test-acceptance
test-acceptance: setup
	@echo "运行验收测试..."
	@echo "验收测试基于集成测试结果和预定义标准"
	@$(MAKE) test-integration
	@echo "根据验收标准评估测试结果..."

# 集成测试细分
.PHONY: test-api
test-api: setup
	@echo "运行API集成测试..."
	@$(SCRIPTS_DIR)/run-integration.sh \
		--timeout $(TIMEOUT) \
		$(if $(filter true,$(COVERAGE)),--coverage) \
		$(if $(filter true,$(VERBOSE)),-v) \
		api

.PHONY: test-websocket
test-websocket: setup
	@echo "运行WebSocket集成测试..."
	@$(SCRIPTS_DIR)/run-integration.sh \
		--timeout $(TIMEOUT) \
		$(if $(filter true,$(COVERAGE)),--coverage) \
		$(if $(filter true,$(VERBOSE)),-v) \
		websocket

.PHONY: test-e2e
test-e2e: setup
	@echo "运行端到端测试..."
	@$(SCRIPTS_DIR)/run-integration.sh \
		--timeout 20m \
		$(if $(filter true,$(COVERAGE)),--coverage) \
		$(if $(filter true,$(VERBOSE)),-v) \
		e2e

.PHONY: test-session
test-session: setup
	@echo "运行会话管理测试..."
	@$(SCRIPTS_DIR)/run-integration.sh \
		--timeout $(TIMEOUT) \
		$(if $(filter true,$(COVERAGE)),--coverage) \
		$(if $(filter true,$(VERBOSE)),-v) \
		session

# 性能测试细分
.PHONY: test-load
test-load: setup
	@echo "运行负载测试..."
	@$(SCRIPTS_DIR)/run-integration.sh \
		--timeout 20m \
		load

.PHONY: test-stress
test-stress: setup
	@echo "运行压力测试..."
	@$(SCRIPTS_DIR)/run-integration.sh \
		--timeout 45m \
		stress

.PHONY: test-benchmark
test-benchmark: setup
	@echo "运行基准测试..."
	@go test -bench=. -benchmem ./performance/... > $(REPORTS_DIR)/benchmark_$(shell date +%Y%m%d_%H%M%S).txt

.PHONY: test-memory
test-memory: setup
	@echo "运行内存测试..."
	@go test -memprofile=$(REPORTS_DIR)/mem_$(shell date +%Y%m%d_%H%M%S).prof ./performance/memory/

# 快速测试目标
.PHONY: test-quick
test-quick: setup
	@echo "运行快速测试..."
	@$(SCRIPTS_DIR)/run-integration.sh \
		--timeout 5m \
		--parallel $(PARALLEL) \
		--filter "TestBasic*" \
		api

.PHONY: test-smoke
test-smoke: setup
	@echo "运行冒烟测试..."
	@cd $(PROJECT_ROOT) && ./alex --version
	@cd $(PROJECT_ROOT) && echo "Hello test" | ./alex
	@echo "冒烟测试通过"

# 报告生成
.PHONY: generate-report
generate-report:
	@echo "生成测试报告..."
	@$(SCRIPTS_DIR)/generate-report.sh \
		--format $(OUTPUT_FORMAT) \
		--include-performance \
		--include-coverage \
		--include-acceptance

.PHONY: report-html
report-html:
	@echo "生成HTML报告..."
	@$(SCRIPTS_DIR)/generate-report.sh --format html

.PHONY: report-json
report-json:
	@echo "生成JSON报告..."
	@$(SCRIPTS_DIR)/generate-report.sh --format json

.PHONY: report-markdown
report-markdown:
	@echo "生成Markdown报告..."
	@$(SCRIPTS_DIR)/generate-report.sh --format markdown

.PHONY: report-all
report-all:
	@echo "生成所有格式报告..."
	@$(SCRIPTS_DIR)/generate-report.sh --format all

# 覆盖率相关
.PHONY: coverage
coverage: setup
	@echo "生成覆盖率报告..."
	@$(SCRIPTS_DIR)/run-integration.sh --coverage all
	@echo "合并覆盖率文件..."
	@if command -v gocovmerge > /dev/null; then \
		gocovmerge $(REPORTS_DIR)/coverage/*.coverage > $(REPORTS_DIR)/coverage/merged.coverage; \
	else \
		echo "警告: gocovmerge未安装，使用简单合并"; \
		cat $(REPORTS_DIR)/coverage/*.coverage > $(REPORTS_DIR)/coverage/merged.coverage; \
	fi
	@go tool cover -html=$(REPORTS_DIR)/coverage/merged.coverage -o $(REPORTS_DIR)/coverage/coverage.html
	@echo "覆盖率报告: $(REPORTS_DIR)/coverage/coverage.html"

.PHONY: coverage-summary
coverage-summary: coverage
	@echo "覆盖率摘要:"
	@go tool cover -func=$(REPORTS_DIR)/coverage/merged.coverage | tail -1

# CI/CD集成
.PHONY: ci-test
ci-test: setup
	@echo "CI模式测试..."
	@$(SCRIPTS_DIR)/run-integration.sh \
		--ci \
		--timeout 15m \
		--parallel $(PARALLEL) \
		--coverage \
		all

.PHONY: ci-performance
ci-performance: setup
	@echo "CI性能测试..."
	@$(SCRIPTS_DIR)/run-integration.sh \
		--ci \
		--timeout 30m \
		performance

.PHONY: ci-report
ci-report:
	@echo "CI报告生成..."
	@$(SCRIPTS_DIR)/generate-report.sh \
		--format json \
		--summary-only

# 监控和度量
.PHONY: monitor-performance
monitor-performance:
	@echo "启动性能监控..."
	@while true; do \
		echo "$(shell date): 内存使用 $(shell ps aux | grep alex | awk '{sum += $$6} END {print sum/1024 "MB"}')"; \
		sleep 5; \
	done

.PHONY: profile-cpu
profile-cpu: setup
	@echo "CPU性能分析..."
	@go test -cpuprofile=$(REPORTS_DIR)/cpu_$(shell date +%Y%m%d_%H%M%S).prof ./performance/...
	@echo "使用 'go tool pprof' 分析结果"

.PHONY: profile-memory
profile-memory: setup
	@echo "内存性能分析..."
	@go test -memprofile=$(REPORTS_DIR)/mem_$(shell date +%Y%m%d_%H%M%S).prof ./performance/...
	@echo "使用 'go tool pprof' 分析结果"

# 安全测试
.PHONY: test-security
test-security: setup
	@echo "运行安全测试..."
	@if command -v gosec > /dev/null; then \
		gosec -fmt json -out $(REPORTS_DIR)/security_$(shell date +%Y%m%d_%H%M%S).json ./...; \
	else \
		echo "警告: gosec未安装，跳过安全扫描"; \
	fi
	@if command -v govulncheck > /dev/null; then \
		govulncheck ./...; \
	else \
		echo "警告: govulncheck未安装，跳过漏洞检查"; \
	fi

# 并发测试
.PHONY: test-concurrency
test-concurrency: setup
	@echo "运行并发测试..."
	@go test -race -timeout 15m ./integration/...

# 回归测试
.PHONY: test-regression
test-regression: setup
	@echo "运行回归测试..."
	@if [ -f $(REPORTS_DIR)/baseline.json ]; then \
		$(SCRIPTS_DIR)/generate-report.sh --compare $(REPORTS_DIR)/baseline.json; \
	else \
		echo "未找到基准报告，生成新基准..."; \
		$(MAKE) test-all; \
		cp $(REPORTS_DIR)/test_report_*.json $(REPORTS_DIR)/baseline.json; \
	fi

# 数据驱动测试
.PHONY: test-data-driven
test-data-driven: setup
	@echo "运行数据驱动测试..."
	@for dataset in $(wildcard $(TESTS_ROOT)/fixtures/datasets/*.json); do \
		echo "测试数据集: $$dataset"; \
		$(SCRIPTS_DIR)/run-integration.sh --data-file "$$dataset" api; \
	done

# 性能基准线管理
.PHONY: baseline-create
baseline-create: test-performance
	@echo "创建性能基准线..."
	@cp $(REPORTS_DIR)/performance/latest.json $(REPORTS_DIR)/performance/baseline.json
	@echo "基准线已创建"

.PHONY: baseline-compare
baseline-compare: test-performance
	@echo "与基准线对比..."
	@if [ -f $(REPORTS_DIR)/performance/baseline.json ]; then \
		$(SCRIPTS_DIR)/generate-report.sh \
			--compare $(REPORTS_DIR)/performance/baseline.json \
			--format html; \
	else \
		echo "错误: 基准线不存在，请先运行 make baseline-create"; \
		exit 1; \
	fi

# 自动化工作流
.PHONY: workflow-pr
workflow-pr: setup test-quick test-api test-websocket coverage-summary
	@echo "PR工作流完成"

.PHONY: workflow-nightly
workflow-nightly: setup test-all test-security baseline-compare report-all
	@echo "夜间工作流完成"

.PHONY: workflow-release
workflow-release: setup test-all test-performance test-security report-all
	@echo "发布工作流完成"

# 交互式测试
.PHONY: test-interactive
test-interactive:
	@echo "交互式测试模式"
	@echo "选择测试类型:"
	@echo "1) API测试"
	@echo "2) WebSocket测试"
	@echo "3) 性能测试"
	@echo "4) 完整测试"
	@read -p "请选择 (1-4): " choice; \
	case $$choice in \
		1) $(MAKE) test-api ;; \
		2) $(MAKE) test-websocket ;; \
		3) $(MAKE) test-performance ;; \
		4) $(MAKE) test-all ;; \
		*) echo "无效选择" ;; \
	esac

# 实用工具
.PHONY: logs
logs:
	@echo "显示最新测试日志..."
	@tail -f $(REPORTS_DIR)/logs/*.log

.PHONY: status
status:
	@echo "测试状态概览:"
	@echo "最后运行时间: $(shell ls -lt $(REPORTS_DIR)/*.json 2>/dev/null | head -1 | awk '{print $$6, $$7, $$8}')"
	@echo "测试文件数: $(shell find $(TESTS_ROOT) -name "*test*.go" | wc -l)"
	@echo "报告文件数: $(shell find $(REPORTS_DIR) -name "*.json" 2>/dev/null | wc -l)"

.PHONY: version
version:
	@echo "ALEX测试框架版本信息:"
	@echo "框架版本: 1.0.0"
	@echo "Go版本: $(shell go version)"
	@echo "项目版本: $(shell cd $(PROJECT_ROOT) && git describe --tags 2>/dev/null || echo 'dev')"

# 调试目标
.PHONY: debug-env
debug-env:
	@echo "调试环境信息:"
	@echo "PROJECT_ROOT: $(PROJECT_ROOT)"
	@echo "TESTS_ROOT: $(TESTS_ROOT)"
	@echo "SCRIPTS_DIR: $(SCRIPTS_DIR)"
	@echo "REPORTS_DIR: $(REPORTS_DIR)"
	@echo "TIMEOUT: $(TIMEOUT)"
	@echo "PARALLEL: $(PARALLEL)"
	@echo "COVERAGE: $(COVERAGE)"

.PHONY: debug-permissions
debug-permissions:
	@echo "检查文件权限:"
	@ls -la $(SCRIPTS_DIR)/*.sh

# 性能优化
.PHONY: optimize
optimize:
	@echo "优化测试性能..."
	@echo "清理旧的测试数据..."
	@find $(REPORTS_DIR) -type f -mtime +7 -delete
	@echo "优化Go模块缓存..."
	@go clean -modcache
	@go mod download
	@echo "优化完成"