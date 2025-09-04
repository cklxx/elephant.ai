package performance

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// IntegrationStrategy defines how the performance verification framework 
// integrates with the existing Alex codebase
type IntegrationStrategy struct {
	config         *IntegrationConfig
	framework      *VerificationFramework
	monitor        *PerformanceMonitor
	abTestManager  *ABTestManager
	scenarioRunner *ScenarioRunner
	
	// Integration points
	configPath     string
	resultsPath    string
	baselinePath   string
}

// IntegrationConfig defines configuration for framework integration
type IntegrationConfig struct {
	// Paths
	ConfigDir         string `json:"config_dir"`
	ResultsDir        string `json:"results_dir"`
	BaselineFile      string `json:"baseline_file"`
	LogFile           string `json:"log_file"`
	
	// Integration settings
	AutoStart         bool  `json:"auto_start"`
	RunOnBuild        bool  `json:"run_on_build"`
	RunOnTest         bool  `json:"run_on_test"`
	CIPipelineEnabled bool  `json:"ci_pipeline_enabled"`
	
	// Verification settings
	VerificationConfig `json:"verification"`
	MonitoringConfig   `json:"monitoring"`
}

// NewIntegrationStrategy creates a new integration strategy
func NewIntegrationStrategy(configPath string) (*IntegrationStrategy, error) {
	config, err := LoadIntegrationConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load integration config: %v", err)
	}
	
	// Create verification framework
	framework := NewVerificationFramework(&config.VerificationConfig)
	
	// Create monitoring system
	monitor := NewPerformanceMonitor(&config.MonitoringConfig, framework)
	
	// Create scenario runner
	scenarioRunner := NewScenarioRunner(&config.VerificationConfig, framework)
	
	return &IntegrationStrategy{
		config:         config,
		framework:      framework,
		monitor:        monitor,
		abTestManager:  NewABTestManager(),
		scenarioRunner: scenarioRunner,
		configPath:     configPath,
		resultsPath:    filepath.Join(config.ResultsDir, "results.json"),
		baselinePath:   config.BaselineFile,
	}, nil
}

// LoadIntegrationConfig loads integration configuration from file
func LoadIntegrationConfig(configPath string) (*IntegrationConfig, error) {
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default configuration
		config := GetDefaultIntegrationConfig()
		if err := SaveIntegrationConfig(configPath, config); err != nil {
			return nil, fmt.Errorf("failed to create default config: %v", err)
		}
		return config, nil
	}
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}
	
	var config IntegrationConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}
	
	return &config, nil
}

// SaveIntegrationConfig saves integration configuration to file
func SaveIntegrationConfig(configPath string, config *IntegrationConfig) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}
	
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}
	
	return os.WriteFile(configPath, data, 0644)
}

// GetDefaultIntegrationConfig returns default integration configuration
func GetDefaultIntegrationConfig() *IntegrationConfig {
	return &IntegrationConfig{
		ConfigDir:         "./performance",
		ResultsDir:        "./performance/results",
		BaselineFile:      "./performance/baseline.json",
		LogFile:           "./performance/performance.log",
		AutoStart:         false,
		RunOnBuild:        true,
		RunOnTest:         true,
		CIPipelineEnabled: true,
		
		VerificationConfig: VerificationConfig{
			BaselineFile:        "./performance/baseline.json",
			BenchmarkDuration:   30 * time.Second,
			WarmupDuration:     5 * time.Second,
			MaxConcurrency:     10,
			FeatureFlags:       make(map[string]bool),
			ABTestingEnabled:   false,
			TrafficSplit:       0.5,
			RegressionThreshold: 0.05,
			AlertingEnabled:    true,
			RollbackEnabled:    false,
			MaxResponseTime:    500 * time.Millisecond,
			MaxMemoryUsage:     100 * 1024 * 1024, // 100MB
			MinThroughput:      50.0,
			MaxErrorRate:       0.01,
		},
		
		MonitoringConfig: MonitoringConfig{
			CollectionInterval:         30 * time.Second,
			AnalysisInterval:          5 * time.Minute,
			AlertCooldown:             10 * time.Minute,
			ResponseTimeThreshold:     200 * time.Millisecond,
			MemoryUsageThreshold:      150 * 1024 * 1024, // 150MB
			ThroughputThreshold:       25.0,
			ErrorRateThreshold:        0.02,
			RegressionWindow:          1 * time.Hour,
			RegressionThreshold:       0.05,
			MinDataPointsForRegression: 10,
			AutoRollbackEnabled:       false,
			RollbackThreshold:         0.10,
			RollbackConfirmationTime:  5 * time.Minute,
			DashboardEnabled:          true,
			ReportingEnabled:          true,
			HistoryRetentionDays:      30,
		},
	}
}

// InitializeFramework sets up the performance verification framework
func (is *IntegrationStrategy) InitializeFramework() error {
	// Create necessary directories
	if err := os.MkdirAll(is.config.ConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}
	if err := os.MkdirAll(is.config.ResultsDir, 0755); err != nil {
		return fmt.Errorf("failed to create results directory: %v", err)
	}
	
	// Initialize framework components
	if err := is.framework.LoadBaseline(); err != nil {
		log.Printf("Warning: failed to load baseline, will create new one: %v", err)
	}
	
	// Add monitoring alert handlers
	is.monitor.AddAlertHandler(&LogAlertHandler{LogFile: is.config.LogFile})
	if is.config.CIPipelineEnabled {
		is.monitor.AddAlertHandler(&CIAlertHandler{})
	}
	
	// Start monitoring if auto-start is enabled
	if is.config.AutoStart {
		if err := is.monitor.Start(); err != nil {
			return fmt.Errorf("failed to start monitoring: %v", err)
		}
	}
	
	log.Printf("Performance verification framework initialized at %s", is.config.ConfigDir)
	return nil
}

// RunBenchmarkSuite executes the full benchmark suite
func (is *IntegrationStrategy) RunBenchmarkSuite() (*BenchmarkSuiteResult, error) {
	log.Println("Starting performance benchmark suite...")
	
	suite := NewBenchmarkSuite(&is.config.VerificationConfig)
	benchmarkResults := suite.RunFullSuite()
	
	scenarioResults, err := is.scenarioRunner.RunAllScenarios()
	if err != nil {
		return nil, fmt.Errorf("scenario execution failed: %v", err)
	}
	
	result := &BenchmarkSuiteResult{
		BenchmarkResults: benchmarkResults,
		ScenarioResults:  scenarioResults,
		Timestamp:        time.Now(),
		Passed:           true,
		Summary:          is.generateSummary(benchmarkResults, scenarioResults),
	}
	
	// Check if any scenarios failed
	for _, scenario := range scenarioResults {
		if !scenario.Passed {
			result.Passed = false
			break
		}
	}
	
	// Save results
	if err := is.saveResults(result); err != nil {
		log.Printf("Warning: failed to save results: %v", err)
	}
	
	log.Printf("Benchmark suite completed. Passed: %t", result.Passed)
	return result, nil
}

// BenchmarkSuiteResult combines all benchmark and scenario results
type BenchmarkSuiteResult struct {
	BenchmarkResults []BenchmarkResult `json:"benchmark_results"`
	ScenarioResults  []ScenarioResult  `json:"scenario_results"`
	Timestamp        time.Time         `json:"timestamp"`
	Passed           bool              `json:"passed"`
	Summary          ResultSummary     `json:"summary"`
}

// ResultSummary provides high-level summary of results
type ResultSummary struct {
	TotalBenchmarks      int     `json:"total_benchmarks"`
	TotalScenarios       int     `json:"total_scenarios"`
	PassedScenarios      int     `json:"passed_scenarios"`
	FailedScenarios      int     `json:"failed_scenarios"`
	AverageResponseTime  float64 `json:"average_response_time_ms"`
	AverageMemoryUsage   int64   `json:"average_memory_usage_bytes"`
	AverageThroughput    float64 `json:"average_throughput_ops"`
	OverallErrorRate     float64 `json:"overall_error_rate"`
}

// RunPreBuildVerification runs verification before build
func (is *IntegrationStrategy) RunPreBuildVerification() error {
	if !is.config.RunOnBuild {
		return nil
	}
	
	log.Println("Running pre-build performance verification...")
	
	// Run a subset of critical scenarios
	criticalScenarios := []string{"MCP Basic Operations", "Memory Leak Detection"}
	
	for _, scenarioName := range criticalScenarios {
		for _, scenario := range is.scenarioRunner.scenarios {
			if scenario.Name == scenarioName && scenario.Enabled {
				result, err := is.scenarioRunner.RunScenario(scenario)
				if err != nil {
					return fmt.Errorf("pre-build verification failed for %s: %v", scenarioName, err)
				}
				if !result.Passed {
					return fmt.Errorf("pre-build verification failed for %s: %v", scenarioName, result.Failures)
				}
			}
		}
	}
	
	log.Println("Pre-build verification completed successfully")
	return nil
}

// RunPostTestVerification runs verification after tests
func (is *IntegrationStrategy) RunPostTestVerification() error {
	if !is.config.RunOnTest {
		return nil
	}
	
	log.Println("Running post-test performance verification...")
	
	// Collect current metrics and compare with baseline
	current := is.framework.collectMetrics()
	comparison := is.framework.CompareWithBaseline()
	
	if comparison != nil {
		// Check for significant regressions
		if comparison.ResponseTimeDiff > is.config.VerificationConfig.RegressionThreshold {
			return fmt.Errorf("post-test verification failed: response time regression of %.2f%%",
				comparison.ResponseTimeDiff*100)
		}
		if comparison.MemoryUsageDiff > is.config.VerificationConfig.RegressionThreshold {
			return fmt.Errorf("post-test verification failed: memory usage regression of %.2f%%",
				comparison.MemoryUsageDiff*100)
		}
	}
	
	// Validate against criteria
	validation := is.framework.ValidateMetrics(current)
	if !validation.Passed {
		return fmt.Errorf("post-test verification failed: %v", validation.Failures)
	}
	
	log.Println("Post-test verification completed successfully")
	return nil
}

// generateSummary creates a summary of all results
func (is *IntegrationStrategy) generateSummary(benchmarks []BenchmarkResult, scenarios []ScenarioResult) ResultSummary {
	summary := ResultSummary{
		TotalBenchmarks: len(benchmarks),
		TotalScenarios:  len(scenarios),
	}
	
	// Count passed/failed scenarios
	for _, scenario := range scenarios {
		if scenario.Passed {
			summary.PassedScenarios++
		} else {
			summary.FailedScenarios++
		}
	}
	
	// Calculate averages from scenarios
	if len(scenarios) > 0 {
		var totalResponseTime float64
		var totalMemoryUsage int64
		var totalThroughput float64
		var totalErrorRate float64
		
		for _, scenario := range scenarios {
			totalResponseTime += float64(scenario.Metrics.ResponseTime.Nanoseconds()) / 1e6 // Convert to ms
			totalMemoryUsage += scenario.Metrics.HeapSize
			totalThroughput += scenario.Metrics.ThroughputOps
			totalErrorRate += scenario.Metrics.ErrorRate
		}
		
		count := float64(len(scenarios))
		summary.AverageResponseTime = totalResponseTime / count
		summary.AverageMemoryUsage = totalMemoryUsage / int64(count)
		summary.AverageThroughput = totalThroughput / count
		summary.OverallErrorRate = totalErrorRate / count
	}
	
	return summary
}

// saveResults saves benchmark results to file
func (is *IntegrationStrategy) saveResults(result *BenchmarkSuiteResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %v", err)
	}
	
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("results_%s.json", timestamp)
	path := filepath.Join(is.config.ResultsDir, filename)
	
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write results file: %v", err)
	}
	
	// Also update the latest results file
	latestPath := filepath.Join(is.config.ResultsDir, "latest.json")
	return os.WriteFile(latestPath, data, 0644)
}

// GetMakefileIntegration returns Makefile targets for integration
func (is *IntegrationStrategy) GetMakefileIntegration() string {
	return `
# Performance Verification Framework Integration

# Performance verification targets
.PHONY: perf-init
perf-init:
	@echo "Initializing performance verification framework..."
	@go run -ldflags "$(LDFLAGS)" ./cmd/perf init
	@echo "Performance framework initialized"

.PHONY: perf-benchmark
perf-benchmark: build
	@echo "Running performance benchmarks..."
	@go run -ldflags "$(LDFLAGS)" ./cmd/perf benchmark
	@echo "Benchmarks completed"

.PHONY: perf-test
perf-test: build
	@echo "Running performance test scenarios..."
	@go run -ldflags "$(LDFLAGS)" ./cmd/perf test
	@echo "Performance tests completed"

.PHONY: perf-baseline
perf-baseline: build
	@echo "Creating new performance baseline..."
	@go run -ldflags "$(LDFLAGS)" ./cmd/perf baseline
	@echo "Baseline created"

.PHONY: perf-monitor
perf-monitor: build
	@echo "Starting performance monitoring..."
	@go run -ldflags "$(LDFLAGS)" ./cmd/perf monitor
	@echo "Performance monitoring started"

.PHONY: perf-report
perf-report:
	@echo "Generating performance report..."
	@go run -ldflags "$(LDFLAGS)" ./cmd/perf report
	@echo "Performance report generated"

# Integration with existing targets
dev: fmt vet-working build perf-test test-functionality
build: perf-init deps
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"
	@go run -ldflags "$(LDFLAGS)" ./cmd/perf pre-build
	@go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) $(SOURCE_MAIN)
	@echo "Build complete: ./$(BINARY_NAME)"

test: deps perf-test
	@echo "Running tests..."
	@go test ./internal/... ./pkg/...
	@go run -ldflags "$(LDFLAGS)" ./cmd/perf post-test

# CI/CD Integration
.PHONY: ci-perf
ci-perf: perf-benchmark perf-report
	@if [ -f "./performance/results/latest.json" ]; then \
		echo "Performance results available"; \
		cat ./performance/results/latest.json | jq '.summary'; \
	fi
`
}

// LogAlertHandler handles alerts by writing to log file
type LogAlertHandler struct {
	LogFile string
}

func (lah *LogAlertHandler) HandleAlert(alert *Alert) error {
	levelNames := []string{"INFO", "WARNING", "ERROR", "CRITICAL"}
	levelName := "UNKNOWN"
	if int(alert.Level) < len(levelNames) {
		levelName = levelNames[alert.Level]
	}
	
	logEntry := fmt.Sprintf("[%s] %s - %s: %s\n", 
		alert.Timestamp.Format(time.RFC3339),
		levelName,
		alert.Title, 
		alert.Description)
	
	file, err := os.OpenFile(lah.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Warning: failed to close log file: %v", err)
		}
	}()
	
	_, err = file.WriteString(logEntry)
	return err
}

// CIAlertHandler handles alerts in CI/CD environment
type CIAlertHandler struct{}

func (ciah *CIAlertHandler) HandleAlert(alert *Alert) error {
	// In CI/CD, output alerts in a format that CI systems can parse
	if alert.Level == AlertCritical || alert.Level == AlertError {
		fmt.Printf("::error::%s - %s\n", alert.Title, alert.Description)
	} else {
		fmt.Printf("::warning::%s - %s\n", alert.Title, alert.Description)
	}
	return nil
}

// Shutdown gracefully shuts down the integration framework
func (is *IntegrationStrategy) Shutdown() error {
	log.Println("Shutting down performance verification framework...")
	
	is.monitor.Stop()
	is.framework.StopMonitoring()
	
	// Save final baseline if needed
	current := is.framework.GetCurrentMetrics()
	if current != nil {
		if err := is.framework.SaveBaseline(current); err != nil {
			log.Printf("Warning: failed to save final baseline: %v", err)
		}
	}
	
	log.Println("Performance framework shutdown complete")
	return nil
}