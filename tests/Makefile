# ALEX Integration Tests Makefile
# Provides test runner targets for the integration test framework.

# Project paths
PROJECT_ROOT := $(shell cd .. && pwd)
TESTS_ROOT := $(shell pwd)
SCRIPTS_DIR := $(TESTS_ROOT)/scripts
REPORTS_DIR := $(TESTS_ROOT)/reports

# Configuration
TIMEOUT ?= 10m
PARALLEL ?= 4
COVERAGE ?= true
VERBOSE ?= false

.PHONY: help
help:
	@echo "ALEX Integration Tests"
	@echo ""
	@echo "Targets:"
	@echo "  test-integration        Run integration tests"
	@echo "  test-api                API integration tests"
	@echo "  test-websocket          WebSocket integration tests"
	@echo "  test-e2e                End-to-end tests"
	@echo "  test-session            Session management tests"
	@echo "  test-quick              Quick smoke subset"
	@echo ""
	@echo "Environment:"
	@echo "  setup                   Prepare test environment"
	@echo "  clean                   Remove test artifacts"
	@echo "  validate                Verify environment is ready"
	@echo ""
	@echo "Configuration:"
	@echo "  TIMEOUT=10m             Test timeout"
	@echo "  PARALLEL=4              Parallel test count"
	@echo "  COVERAGE=true           Generate coverage"
	@echo "  VERBOSE=false           Verbose output"

# Environment
.PHONY: setup
setup:
	@echo "Setting up test environment..."
	@mkdir -p $(REPORTS_DIR)
	@chmod +x $(SCRIPTS_DIR)/*.sh
	@echo "Done"

.PHONY: validate
validate:
	@echo "Validating test environment..."
	@$(SCRIPTS_DIR)/run-integration.sh --help > /dev/null
	@echo "Environment OK"

.PHONY: clean
clean:
	@echo "Cleaning test artifacts..."
	@rm -rf $(REPORTS_DIR)/*
	@rm -rf /tmp/alex_*test*
	@echo "Done"

# Integration tests
.PHONY: test-integration
test-integration: setup
	@echo "Running integration tests..."
	@$(SCRIPTS_DIR)/run-integration.sh \
		--timeout $(TIMEOUT) \
		--parallel $(PARALLEL) \
		$(if $(filter true,$(COVERAGE)),--coverage) \
		$(if $(filter true,$(VERBOSE)),-v) \
		all

.PHONY: test-api
test-api: setup
	@$(SCRIPTS_DIR)/run-integration.sh \
		--timeout $(TIMEOUT) \
		$(if $(filter true,$(COVERAGE)),--coverage) \
		$(if $(filter true,$(VERBOSE)),-v) \
		api

.PHONY: test-websocket
test-websocket: setup
	@$(SCRIPTS_DIR)/run-integration.sh \
		--timeout $(TIMEOUT) \
		$(if $(filter true,$(COVERAGE)),--coverage) \
		$(if $(filter true,$(VERBOSE)),-v) \
		websocket

.PHONY: test-e2e
test-e2e: setup
	@$(SCRIPTS_DIR)/run-integration.sh \
		--timeout 20m \
		$(if $(filter true,$(COVERAGE)),--coverage) \
		$(if $(filter true,$(VERBOSE)),-v) \
		e2e

.PHONY: test-session
test-session: setup
	@$(SCRIPTS_DIR)/run-integration.sh \
		--timeout $(TIMEOUT) \
		$(if $(filter true,$(COVERAGE)),--coverage) \
		$(if $(filter true,$(VERBOSE)),-v) \
		session

.PHONY: test-quick
test-quick: setup
	@$(SCRIPTS_DIR)/run-integration.sh \
		--timeout 5m \
		--parallel $(PARALLEL) \
		--filter "TestBasic*" \
		api

# CI
.PHONY: ci-test
ci-test: setup
	@$(SCRIPTS_DIR)/run-integration.sh \
		--ci \
		--timeout 15m \
		--parallel $(PARALLEL) \
		--coverage \
		all

# Info
.PHONY: debug-env
debug-env:
	@echo "PROJECT_ROOT: $(PROJECT_ROOT)"
	@echo "TESTS_ROOT:   $(TESTS_ROOT)"
	@echo "SCRIPTS_DIR:  $(SCRIPTS_DIR)"
	@echo "REPORTS_DIR:  $(REPORTS_DIR)"
