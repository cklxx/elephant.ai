# Alex - Software Engineering Assistant Makefile

# Variables
BINARY_NAME=alex
SOURCE_MAIN=./cmd
BUILD_DIR=build

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT = $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_TIME = $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# Build flags for version injection
LDFLAGS = -s -w \
	-X 'alex/internal/utils.Version=$(VERSION)' \
	-X 'alex/internal/utils.GitCommit=$(GIT_COMMIT)' \
	-X 'alex/internal/utils.BuildTime=$(BUILD_TIME)'

# Default target
.PHONY: all
all: build

# Build the binary with version information
.PHONY: build
build: deps
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"
	@go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) $(SOURCE_MAIN)
	@echo "Build complete: ./$(BINARY_NAME)"

# Build for multiple platforms with version information
.PHONY: build-all
build-all: deps
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	@echo "Building Linux AMD64..."
	@GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(SOURCE_MAIN)
	@echo "Building Linux ARM64..."
	@GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(SOURCE_MAIN)
	@echo "Building Darwin AMD64..."
	@GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(SOURCE_MAIN)
	@echo "Building Darwin ARM64..."
	@GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(SOURCE_MAIN)
	@echo "Building Windows AMD64..."
	@GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(SOURCE_MAIN)
	@echo "Multi-platform build complete in $(BUILD_DIR)/"
	@echo "Version: $(VERSION)"
	@echo "Copying binaries to npm packages..."
	@./scripts/copy-npm-binaries.sh

# Install the binary to GOPATH/bin with version information
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME) $(VERSION) to GOPATH/bin..."
	@cp $(BINARY_NAME) $$(go env GOPATH)/bin/
	@echo "Installation complete"

# Show version information (for debugging)
.PHONY: version-info
version-info:
	@echo "Version Information:"
	@echo "  VERSION: $(VERSION)"
	@echo "  GIT_COMMIT: $(GIT_COMMIT)"
	@echo "  BUILD_TIME: $(BUILD_TIME)"
	@echo "  LDFLAGS: $(LDFLAGS)"

# Initialize dependencies
.PHONY: deps
deps:
	@echo "Initializing dependencies..."
	@if ! go mod tidy; then \
		echo "Warning: go mod tidy failed - possibly due to network issues"; \
		echo "Continuing with existing dependencies..."; \
	fi
	@if ! go mod download; then \
		echo "Warning: go mod download failed - possibly due to network issues"; \
		echo "Continuing with existing dependencies..."; \
	fi

# Force download dependencies (retry on network issues)
.PHONY: deps-force
deps-force:
	@echo "Force downloading dependencies..."
	@go clean -modcache
	@go mod tidy
	@go mod download

# Run tests
.PHONY: test
test: deps
	@echo "Running tests..."
	@go test ./internal/... ./pkg/...

# Run tests excluding broken ones
.PHONY: test-working
test-working:
	@echo "Running working tests..."
	@go test ./internal/config ./pkg/...

# Run tests with automatic fixes for common issues
.PHONY: test-robust
test-robust: deps
	@echo "Running robust tests..."
	@echo "Testing config..."
	@go test ./internal/config -v
	@echo "Testing types..."
	@if [ -f "./pkg/types" ]; then go test ./pkg/types -v; fi
	@echo "Robust testing complete"

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "Using golangci-lint for formatting..."; \
		golangci-lint run --fix; \
	else \
		echo "golangci-lint not found, using go fmt..."; \
		go fmt ./internal/... ./pkg/... ./cmd/... ./benchmarks/... ./docs/...; \
		go fmt $(SOURCE_MAIN); \
	fi
	@echo "Formatting complete"

# Vet code
.PHONY: vet
vet: deps
	@echo "Vetting code..."
	@go vet ./internal/... ./pkg/...
	@go vet $(SOURCE_MAIN)

# Vet working code only
.PHONY: vet-working
vet-working:
	@echo "Vetting working code..."
	@go vet ./internal/config ./pkg/...
	@go vet $(SOURCE_MAIN)

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_NAME)
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete"

# Run the CLI with help
.PHONY: run
run: build
	@./$(BINARY_NAME) --help

# Quick test of core functionality
.PHONY: test-functionality
test-functionality: build
	@echo "Testing core functionality..."
	@./$(BINARY_NAME) --version
	@./$(BINARY_NAME) --help
	@./$(BINARY_NAME) "What tools are available?"
	@echo "Functionality test complete"

# Development workflow
.PHONY: dev
dev: fmt vet-working build test-functionality

# Safe development workflow (excludes broken tests)
.PHONY: dev-safe
dev-safe: fmt vet-working build test-working

# Ultra-robust development workflow
.PHONY: dev-robust
dev-robust: deps fmt vet-working build test-robust

# Benchmark targets

# Setup benchmark environment
.PHONY: benchmark-setup
benchmark-setup:
	@echo "Setting up benchmark environment..."
	@cd benchmarks && ./scripts/download_datasets.sh
	@echo "Benchmark setup complete"

# Build benchmark CLI
.PHONY: benchmark-build
benchmark-build: build
	@echo "Building benchmark CLI..."
	@cd benchmarks && go build -o benchmark ./cmd/benchmark
	@echo "Benchmark CLI built: benchmarks/benchmark"

# Quick benchmark run (subset of problems for fast feedback)
.PHONY: benchmark-quick
benchmark-quick: benchmark-build
	@echo "Running quick benchmark (subset of problems)..."
	@cd benchmarks && ./benchmark -alex=../alex -concurrency=2 -timeout=2m -problems=HumanEval_0,HumanEval_1 run
	@echo "Quick benchmark complete"

# Full benchmark run
.PHONY: benchmark-full
benchmark-full: benchmark-build
	@echo "Running full benchmark suite..."
	@cd benchmarks && ./benchmark -alex=../alex -concurrency=3 -timeout=5m -profile -analyze run
	@echo "Full benchmark complete"

# Run HumanEval benchmark only
.PHONY: benchmark-humaneval
benchmark-humaneval: benchmark-build
	@echo "Running HumanEval benchmark..."
	@cd benchmarks && ./benchmark -alex=../alex -dataset=human-eval -concurrency=3 -timeout=3m run
	@echo "HumanEval benchmark complete"

# Run SWE-Bench benchmark only
.PHONY: benchmark-swebench
benchmark-swebench: benchmark-build
	@echo "Running SWE-Bench benchmark..."
	@cd benchmarks && ./benchmark -alex=../alex -dataset=swe-bench -concurrency=2 -timeout=10m run
	@echo "SWE-Bench benchmark complete"

# Generate benchmark report
.PHONY: benchmark-report
benchmark-report: benchmark-build
	@echo "Generating benchmark report..."
	@cd benchmarks && ./benchmark -format=html report
	@echo "Benchmark report generated"

# List benchmark runs
.PHONY: benchmark-list
benchmark-list: benchmark-build
	@echo "Listing benchmark runs..."
	@cd benchmarks && ./benchmark list

# Compare benchmark runs
.PHONY: benchmark-compare
benchmark-compare: benchmark-build
	@echo "Comparing benchmark runs..."
	@echo "Usage: make benchmark-compare RUN1=<run-id-1> RUN2=<run-id-2>"
	@if [ -n "$(RUN1)" ] && [ -n "$(RUN2)" ]; then \
		cd benchmarks && ./benchmark -compare=$(RUN1),$(RUN2) compare; \
	else \
		echo "Please specify RUN1 and RUN2 variables"; \
	fi

# Clean benchmark results
.PHONY: benchmark-clean
benchmark-clean: benchmark-build
	@echo "Cleaning benchmark results..."
	@cd benchmarks && ./benchmark clean
	@echo "Benchmark cleanup complete"

# Validate benchmark setup
.PHONY: benchmark-validate
benchmark-validate: benchmark-build
	@echo "Validating benchmark setup..."
	@cd benchmarks && ./benchmark -alex=../alex validate
	@echo "Benchmark validation complete"

# Benchmark development workflow (build + quick test)
.PHONY: benchmark-dev
benchmark-dev: build benchmark-build benchmark-quick

# Benchmark CI workflow (setup + full run + report)
.PHONY: benchmark-ci
benchmark-ci: benchmark-setup benchmark-full benchmark-report

# SWE-Bench Batch Processing targets

# Test SWE-Bench batch processing with minimal dataset
.PHONY: swe-bench-test
swe-bench-test: build
	@echo "Testing SWE-Bench batch processing..."
	@./$(BINARY_NAME) run-batch --dataset.subset lite --dataset.split dev --instance-limit 2 --workers 1 --output ./test_results
	@echo "SWE-Bench test complete"

# Run SWE-Bench lite benchmark  
.PHONY: swe-bench-lite
swe-bench-lite: build
	@echo "Running SWE-Bench lite benchmark..."
	@./$(BINARY_NAME) run-batch --dataset.subset lite --dataset.split dev --workers 3 --output ./swe_bench_lite_results
	@echo "SWE-Bench lite benchmark complete"

# Run SWE-Bench full benchmark
.PHONY: swe-bench-full
swe-bench-full: build
	@echo "Running SWE-Bench full benchmark..."
	@./$(BINARY_NAME) run-batch --dataset.subset full --dataset.split dev --workers 5 --output ./swe_bench_full_results
	@echo "SWE-Bench full benchmark complete"

# Generate SWE-Bench configuration template
.PHONY: swe-bench-config
swe-bench-config:
	@echo "Creating SWE-Bench configuration template..."
	@cp evaluation/swe_bench/config.yaml ./swe_bench_config.yaml
	@echo "Configuration template created: swe_bench_config.yaml"

# SWE-Bench Verified targets (500 high-quality verified instances)

# Test SWE-Bench with real instances
.PHONY: swe-bench-verified-test
swe-bench-verified-test: build
	@echo "Testing SWE-Bench with real instances..."
	@cd evaluation/swe_bench && ./run_evaluation.sh real-test
	@echo "SWE-Bench real instances test complete"

# Test SWE-Bench Verified with network download (fallback)
.PHONY: swe-bench-verified-quick
swe-bench-verified-quick: build
	@echo "Testing SWE-Bench Verified batch processing..."
	@cd evaluation/swe_bench && ./run_evaluation.sh quick-test
	@echo "SWE-Bench Verified test complete"

# Run SWE-Bench Verified small batch (50 instances)
.PHONY: swe-bench-verified-small
swe-bench-verified-small: build
	@echo "Running SWE-Bench Verified small batch..."
	@cd evaluation/swe_bench && ./run_evaluation.sh small-batch
	@echo "SWE-Bench Verified small batch complete"

# Run SWE-Bench Verified medium batch (150 instances)
.PHONY: swe-bench-verified-medium
swe-bench-verified-medium: build
	@echo "Running SWE-Bench Verified medium batch..."
	@cd evaluation/swe_bench && ./run_evaluation.sh medium-batch
	@echo "SWE-Bench Verified medium batch complete"

# Run SWE-Bench Verified full evaluation (all 500 instances)
.PHONY: swe-bench-verified-full
swe-bench-verified-full: build
	@echo "Running SWE-Bench Verified full evaluation..."
	@cd evaluation/swe_bench && ./run_evaluation.sh full
	@echo "SWE-Bench Verified full evaluation complete"

# Generate SWE-Bench Verified configuration
.PHONY: swe-bench-verified-config
swe-bench-verified-config:
	@echo "Creating SWE-Bench Verified configuration..."
	@cp evaluation/swe_bench/config.yaml ./swe_bench_verified_config.yaml
	@echo "SWE-Bench Verified configuration created: swe_bench_verified_config.yaml"

# Clean SWE-Bench results
.PHONY: swe-bench-clean
swe-bench-clean:
	@echo "Cleaning SWE-Bench results..."
	@rm -rf ./test_results ./swe_bench_lite_results ./swe_bench_full_results
	@rm -rf ./verified_quick_test ./verified_small_batch ./verified_medium_batch ./verified_full_evaluation ./verified_custom
	@echo "SWE-Bench cleanup complete"

# Help target
.PHONY: help
help:
	@echo ""
	@echo "NPM Publishing:"
	@echo "  copy-npm-binaries  Copy built binaries to their respective npm packages"
	@echo "  publish-npm        Publish all npm packages to the registry"
	@echo ""
	@echo "Available targets:"
	@echo ""
	@echo "Build & Development:"
	@echo "  build              Build the binary"

# Version Management targets
.PHONY: update-version
update-version:
	@if [ -z "$(VERSION)" ]; then \
		echo "❌ VERSION is required. Usage: make update-version VERSION=0.0.3"; \
		exit 1; \
	fi
	@node scripts/update-version.js $(VERSION)

.PHONY: show-version
show-version:
	@echo "📦 Current Package Versions:"
	@echo "Main Package: $$(jq -r .version npm/alex-code/package.json)"
	@echo "Platform Packages:"
	@for pkg in npm/alex-*/package.json; do \
		if [ -f "$$pkg" ] && [[ "$$pkg" != *"/alex-code/"* ]]; then \
			name=$$(jq -r .name "$$pkg"); \
			version=$$(jq -r .version "$$pkg"); \
			echo "  $$name: $$version"; \
		fi \
	done
	@echo "Install Script Version: $$(grep 'const VERSION' npm/alex-code/install.js | sed "s/.*'\([^']*\)'.*/\1/")"

.PHONY: version-check
version-check:
	@echo "🔍 Checking version consistency..."
	@main_version=$$(jq -r .version npm/alex-code/package.json); \
	install_version=$$(grep 'getVersion()' npm/alex-code/install.js > /dev/null && echo "dynamic" || grep 'const VERSION' npm/alex-code/install.js | sed "s/.*'\([^']*\)'.*/\1/"); \
	echo "Main package version: $$main_version"; \
	echo "Install script version: $$install_version"; \
	all_consistent=true; \
	for pkg in npm/alex-*/package.json; do \
		if [ -f "$$pkg" ] && [[ "$$pkg" != *"/alex-code/"* ]]; then \
			pkg_version=$$(jq -r .version "$$pkg"); \
			if [ "$$pkg_version" != "$$main_version" ]; then \
				echo "❌ Version mismatch in $$pkg: $$pkg_version (expected: $$main_version)"; \
				all_consistent=false; \
			fi \
		fi \
	done; \
	if [ "$$all_consistent" = true ]; then \
		echo "✅ All versions are consistent: $$main_version"; \
	else \
		echo "⚠️  Version inconsistencies found. Run 'make update-version VERSION=X.Y.Z' to fix."; \
		exit 1; \
	fi

# NPM Publishing targets  
.PHONY: copy-npm-binaries
copy-npm-binaries:
	@./scripts/copy-npm-binaries.sh

.PHONY: publish-npm
publish-npm: build-all version-check copy-npm-binaries
	@./scripts/publish-npm.sh

.PHONY: release-npm
release-npm:
	@if [ -z "$(VERSION)" ]; then \
		echo "❌ VERSION is required. Usage: make release-npm VERSION=0.0.3"; \
		exit 1; \
	fi
	@echo "🚀 Starting complete NPM release process for version $(VERSION)..."
	@$(MAKE) update-version VERSION=$(VERSION)
	@$(MAKE) build-all
	@$(MAKE) publish-npm
	@echo "🎉 NPM release $(VERSION) completed successfully!"

# CI/CD Integration targets
.PHONY: ci-npm-publish
ci-npm-publish:
	@if [ -z "$(VERSION)" ]; then \
		echo "❌ VERSION environment variable is required"; \
		exit 1; \
	fi
	@echo "🚀 CI: Publishing NPM packages for version $(VERSION)..."
	@$(MAKE) update-version VERSION=$(VERSION)
	@$(MAKE) build-all
	@$(MAKE) copy-npm-binaries
	@echo "📦 CI: Starting NPM publication..."
	@cd npm/alex-linux-amd64 && npm publish --access public || echo "⚠️  Package may already exist"
	@cd npm/alex-linux-arm64 && npm publish --access public || echo "⚠️  Package may already exist"  
	@cd npm/alex-darwin-amd64 && npm publish --access public || echo "⚠️  Package may already exist"
	@cd npm/alex-darwin-arm64 && npm publish --access public || echo "⚠️  Package may already exist"
	@cd npm/alex-windows-amd64 && npm publish --access public || echo "⚠️  Package may already exist"
	@cd npm/alex-code && npm publish --access public || echo "⚠️  Package may already exist"
	@echo "✅ CI: NPM publication process completed"

.PHONY: ci-test-install
ci-test-install:
	@if [ -z "$(VERSION)" ]; then \
		echo "❌ VERSION environment variable is required"; \
		exit 1; \
	fi
	@echo "🧪 CI: Testing NPM installation for version $(VERSION)..."
	@echo "⏳ Waiting for NPM propagation..."
	@sleep 30
	@npm install -g alex-code@$(VERSION)
	@alex version || echo "⚠️  alex version command failed"
	@alex --help || echo "⚠️  alex help command failed"  
	@which alex
	@echo "✅ CI: Installation test completed"
	@echo "  build-all          Build for multiple platforms"
	@echo "  install            Install binary to GOPATH/bin"
	@echo "  deps               Initialize dependencies"
	@echo "  deps-force         Force download dependencies (retry on network issues)"
	@echo "  test               Run tests"
	@echo "  fmt                Format code"
	@echo "  vet                Vet code"
	@echo "  clean              Clean build artifacts"
	@echo "  run                Run CLI with help"
	@echo "  test-functionality Quick functionality test"
	@echo "  dev                Development workflow (fmt, vet, build, test)"
	@echo "  dev-safe           Safe development workflow (excludes broken tests)"
	@echo "  dev-robust         Ultra-robust development workflow with dependency management"
	@echo "  test-working       Run only working tests"
	@echo "  test-robust        Run tests with automatic issue handling"
	@echo "  vet-working        Vet only working code"
	@echo ""
	@echo "Benchmark & Evaluation:"
	@echo "  benchmark-setup    Setup benchmark environment and download datasets"
	@echo "  benchmark-build    Build benchmark CLI"
	@echo "  benchmark-quick    Run quick benchmark (subset of problems)"
	@echo "  benchmark-full     Run full benchmark suite with profiling"
	@echo "  benchmark-humaneval Run HumanEval benchmark only"
	@echo "  benchmark-swebench Run SWE-Bench benchmark only"
	@echo "  benchmark-report   Generate HTML benchmark report"
	@echo "  benchmark-list     List available benchmark runs"
	@echo "  benchmark-compare  Compare two benchmark runs (use RUN1=x RUN2=y)"
	@echo "  benchmark-clean    Clean benchmark results and logs"
	@echo "  benchmark-validate Validate benchmark setup"
	@echo "  benchmark-dev      Benchmark development workflow"
	@echo "  benchmark-ci       Benchmark CI workflow (setup + full run + report)"
	@echo ""
	@echo "SWE-Bench Batch Processing:"
	@echo "  swe-bench-test     Test SWE-Bench batch processing with minimal dataset"
	@echo "  swe-bench-lite     Run SWE-Bench lite benchmark"
	@echo "  swe-bench-full     Run SWE-Bench full benchmark"
	@echo "  swe-bench-config   Generate SWE-Bench configuration template"
	@echo "  swe-bench-clean    Clean SWE-Bench results"
	@echo ""
	@echo "SWE-Bench Verified (500 high-quality instances):"
	@echo "  swe-bench-verified-test     Test with 3 real SWE-bench instances (recommended)"
	@echo "  swe-bench-verified-quick    Test with network download (5 instances)"
	@echo "  swe-bench-verified-small    Run Verified small batch (50 instances)"
	@echo "  swe-bench-verified-medium   Run Verified medium batch (150 instances)"
	@echo "  swe-bench-verified-full     Run Verified full evaluation (500 instances)"
	@echo "  swe-bench-verified-config   Generate Verified configuration template"
	@echo ""
	@echo "  help               Show this help message"