.PHONY: help build test clean dev fmt vet demo

GO ?= scripts/go-with-toolchain.sh

help: ## Show this help
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

build: ## Build alex binary
	@echo "Building alex..."
	@$(GO) build -o alex ./cmd/alex/
	@echo "✓ Build complete: ./alex"

test: ## Run all tests
	@echo "Running tests..."
	@$(GO) test ./... -v

test-domain: ## Run domain layer tests (fast, mocked)
	@$(GO) test ./internal/agent/domain/ -v

test-app: ## Run application layer tests
	@$(GO) test ./internal/agent/app/ -v

clean: ## Clean build artifacts
	@rm -f alex
	@echo "✓ Cleaned"

fmt: ## Format and lint Go code with golangci-lint
	@./scripts/run-golangci-lint.sh run --fix ./...
	@echo "✓ Formatted and linted"

vet: ## Run go vet
	@$(GO) vet ./cmd/... ./internal/...
	@echo "✓ Vet passed"

dev: fmt vet build ## Format, vet, build (development workflow)
	@echo "✓ Development build complete"

demo: build ## Run parallel execution demo
	@./alex --demo-parallel

run: build ## Run alex with arguments (usage: make run ARGS="your task")
	@./alex $(ARGS)

install: build ## Install alex to $GOPATH/bin
	@cp alex $(GOPATH)/bin/alex
	@echo "✓ Installed to $(GOPATH)/bin/alex"

# Architecture validation
check-deps: ## Check that domain has zero infrastructure deps
	@echo "Checking domain layer dependencies..."
	@$(GO) list -f '{{.Imports}}' ./internal/agent/domain/ | grep -v ports | grep -E 'alex/internal/(llm|tools|session|context|messaging|parser)' && echo "❌ Domain layer has infrastructure dependencies!" || echo "✓ Domain layer is clean (only depends on ports)"

# Performance
bench: ## Run benchmarks
	@$(GO) test ./... -bench=. -benchmem

# Documentation
docs: ## Show documentation locations
	@echo "Documentation available in:"
	@echo "  - README.md"
	@echo "  - docs/README.md"
	@echo "  - docs/AGENT.md"
	@echo "  - docs/reference/ALEX.md"
	@echo "  - docs/operations/DEPLOYMENT.md"

# NPM Publishing
npm-copy-binaries: ## Copy built binaries to npm packages
	@echo "Copying binaries to npm packages..."
	@./scripts/copy-npm-binaries.sh

npm-publish: ## Publish to npm (requires binaries in build/)
	@echo "Publishing to npm..."
	@./scripts/publish-npm.sh

npm-test-install: ## Test npm package installation locally
	@echo "Testing local npm installation..."
	@cd npm/alex-code && npm pack
	@echo "Package created. Test with: npm install -g npm/alex-code/alex-code-*.tgz"

# Multi-platform builds
build-all: ## Build binaries for all platforms
	@echo "Building binaries for all platforms..."
	@mkdir -p build
	@echo "Building Linux AMD64..."
	@GOOS=linux GOARCH=amd64 $(GO) build -ldflags="-w -s" -o build/alex-linux-amd64 ./cmd/alex
	@echo "Building Linux ARM64..."
	@GOOS=linux GOARCH=arm64 $(GO) build -ldflags="-w -s" -o build/alex-linux-arm64 ./cmd/alex
	@echo "Building macOS AMD64..."
	@GOOS=darwin GOARCH=amd64 $(GO) build -ldflags="-w -s" -o build/alex-darwin-amd64 ./cmd/alex
	@echo "Building macOS ARM64..."
	@GOOS=darwin GOARCH=arm64 $(GO) build -ldflags="-w -s" -o build/alex-darwin-arm64 ./cmd/alex
	@echo "Building Windows AMD64..."
	@GOOS=windows GOARCH=amd64 $(GO) build -ldflags="-w -s" -o build/alex-windows-amd64.exe ./cmd/alex
	@echo "✓ All builds complete"
	@ls -lh build/

release-npm: build-all npm-copy-binaries ## Build binaries and publish to npm
	@echo "Publishing to npm..."
	@./scripts/publish-npm.sh

# SSE Server targets
server-build: ## Build alex-server binary
	@echo "Building alex-server..."
	@$(GO) build -o alex-server ./cmd/alex-server/
	@echo "✓ Server build complete: ./alex-server"

server-run: server-build ## Run alex-server
	@echo "Starting alex-server on port 8080..."
	@./alex-server

local-llm: ## Run local FunctionGemma inference server (OpenAI-compatible)
	@./scripts/run-local-llm.sh

server-test: ## Run server tests
	@echo "Running server tests..."
	@$(GO) test ./internal/server/... -v

server-test-integration: server-build ## Run integration tests with test script
	@echo "Running SSE server integration tests..."
	@./scripts/test-sse-server.sh

# ========================================
# Deployment Targets
# ========================================

.PHONY: deploy-local deploy-docker deploy-dev deploy-test deploy-status deploy-down

deploy-local: ## Deploy locally (Go + Next.js)
	@./deploy.sh local

deploy-docker: ## Deploy with Docker Compose (production)
	@./deploy.sh docker

deploy-dev: ## Deploy with Docker Compose (development)
	@./deploy.sh dev

deploy-test: ## Run all deployment tests
	@./deploy.sh test

deploy-status: ## Show deployment status
	@./deploy.sh status

deploy-down: ## Stop all deployments
	@./deploy.sh down

# Quick deploy (default: local)
deploy: deploy-local ## Quick deploy (local mode)
