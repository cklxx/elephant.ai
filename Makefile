.PHONY: help build test clean dev fmt vet demo

help: ## Show this help
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

build: ## Build alex binary
	@echo "Building alex..."
	@go build -o alex ./cmd/alex/
	@echo "✓ Build complete: ./alex"

test: ## Run all tests
	@echo "Running tests..."
	@go test ./... -v

test-domain: ## Run domain layer tests (fast, mocked)
	@go test ./internal/agent/domain/ -v

test-app: ## Run application layer tests
	@go test ./internal/agent/app/ -v

clean: ## Clean build artifacts
	@rm -f alex
	@echo "✓ Cleaned"

fmt: ## Format Go code
	@go fmt ./...
	@echo "✓ Formatted"

vet: ## Run go vet
	@go vet ./cmd/... ./internal/...
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
	@go list -f '{{.Imports}}' ./internal/agent/domain/ | grep -v ports | grep -E 'alex/internal/(llm|tools|session|context|messaging|parser)' && echo "❌ Domain layer has infrastructure dependencies!" || echo "✓ Domain layer is clean (only depends on ports)"

# Performance
bench: ## Run benchmarks
	@go test ./... -bench=. -benchmem

# Documentation
docs: ## Generate documentation
	@echo "Documentation available in:"
	@echo "  - NEW_ARCHITECTURE.md"
	@echo "  - docs/architecture/"
	@echo "  - REFACTORING_SUMMARY.md"

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
