package performance

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TestScenario defines a specific performance test scenario
type TestScenario struct {
	Name               string             `json:"name"`
	Description        string             `json:"description"`
	Duration           time.Duration      `json:"duration"`
	Concurrency        int                `json:"concurrency"`
	Operations         []OperationType    `json:"operations"`
	ValidationCriteria ValidationCriteria `json:"validation_criteria"`
	Setup              func() error       `json:"-"`
	Teardown           func() error       `json:"-"`
	Enabled            bool               `json:"enabled"`
}

// OperationType defines different types of operations to test
type OperationType string

const (
	MCPConnectionOp        OperationType = "mcp_connection"
	MCPToolCallOp          OperationType = "mcp_tool_call"
	ContextCompressionOp   OperationType = "context_compression"
	ContextRetrievalOp     OperationType = "context_retrieval"
	MemoryAllocationOp     OperationType = "memory_allocation"
	ConcurrentProcessingOp OperationType = "concurrent_processing"
	NetworkIOOp            OperationType = "network_io"
	FileIOOp               OperationType = "file_io"
)

// ValidationCriteria defines pass/fail criteria for test scenarios
type ValidationCriteria struct {
	MaxResponseTime    time.Duration `json:"max_response_time"`
	MaxMemoryUsage     int64         `json:"max_memory_usage"`
	MinThroughput      float64       `json:"min_throughput"`
	MaxErrorRate       float64       `json:"max_error_rate"`
	MaxGCPause         time.Duration `json:"max_gc_pause"`
	MinCacheHitRate    float64       `json:"min_cache_hit_rate"`
	MaxMemoryLeakScore float64       `json:"max_memory_leak_score"`

	// Context-specific criteria
	MaxContextCompressionTime time.Duration `json:"max_context_compression_time"`
	MaxContextRetrievalTime   time.Duration `json:"max_context_retrieval_time"`

	// MCP-specific criteria
	MaxMCPConnectionTime   time.Duration `json:"max_mcp_connection_time"`
	MaxMCPToolCallLatency  time.Duration `json:"max_mcp_tool_call_latency"`
	MaxMCPProtocolOverhead time.Duration `json:"max_mcp_protocol_overhead"`
}

// ScenarioResult represents the outcome of a test scenario execution
type ScenarioResult struct {
	Scenario         TestScenario       `json:"scenario"`
	StartTime        time.Time          `json:"start_time"`
	EndTime          time.Time          `json:"end_time"`
	Duration         time.Duration      `json:"duration"`
	Passed           bool               `json:"passed"`
	Failures         []string           `json:"failures"`
	Metrics          PerformanceMetrics `json:"metrics"`
	OperationResults []OperationResult  `json:"operation_results"`
}

// OperationResult tracks results for individual operations within a scenario
type OperationResult struct {
	Operation    OperationType `json:"operation"`
	Count        int           `json:"count"`
	TotalTime    time.Duration `json:"total_time"`
	AverageTime  time.Duration `json:"average_time"`
	MinTime      time.Duration `json:"min_time"`
	MaxTime      time.Duration `json:"max_time"`
	SuccessCount int           `json:"success_count"`
	ErrorCount   int           `json:"error_count"`
	ErrorRate    float64       `json:"error_rate"`
}

// ScenarioRunner executes performance test scenarios
type ScenarioRunner struct {
	scenarios    []TestScenario
	config       *VerificationConfig
	framework    *VerificationFramework
	results      []ScenarioResult
	resultsMutex sync.RWMutex
}

// NewScenarioRunner creates a new scenario runner
func NewScenarioRunner(config *VerificationConfig, framework *VerificationFramework) *ScenarioRunner {
	return &ScenarioRunner{
		scenarios: GetStandardScenarios(),
		config:    config,
		framework: framework,
		results:   make([]ScenarioResult, 0),
	}
}

// GetStandardScenarios returns the standard set of performance test scenarios
func GetStandardScenarios() []TestScenario {
	return []TestScenario{
		{
			Name:        "MCP Basic Operations",
			Description: "Test basic MCP connection and tool call performance",
			Duration:    30 * time.Second,
			Concurrency: 5,
			Operations:  []OperationType{MCPConnectionOp, MCPToolCallOp},
			ValidationCriteria: ValidationCriteria{
				MaxResponseTime:        100 * time.Millisecond,
				MaxMemoryUsage:         50 * 1024 * 1024, // 50MB
				MinThroughput:          100.0,
				MaxErrorRate:           0.01,
				MaxMCPConnectionTime:   50 * time.Millisecond,
				MaxMCPToolCallLatency:  30 * time.Millisecond,
				MaxMCPProtocolOverhead: 10 * time.Millisecond,
			},
			Enabled: true,
		},
		{
			Name:        "Context Operations Stress Test",
			Description: "Test context compression and retrieval under load",
			Duration:    60 * time.Second,
			Concurrency: 10,
			Operations:  []OperationType{ContextCompressionOp, ContextRetrievalOp},
			ValidationCriteria: ValidationCriteria{
				MaxResponseTime:           200 * time.Millisecond,
				MaxMemoryUsage:            100 * 1024 * 1024, // 100MB
				MinThroughput:             50.0,
				MaxErrorRate:              0.02,
				MaxContextCompressionTime: 150 * time.Millisecond,
				MaxContextRetrievalTime:   50 * time.Millisecond,
				MinCacheHitRate:           0.80,
			},
			Enabled: true,
		},
		{
			Name:        "Memory Leak Detection",
			Description: "Test for memory leaks during sustained operations",
			Duration:    120 * time.Second,
			Concurrency: 3,
			Operations:  []OperationType{MemoryAllocationOp, ContextCompressionOp, MCPToolCallOp},
			ValidationCriteria: ValidationCriteria{
				MaxResponseTime:    500 * time.Millisecond,
				MaxMemoryUsage:     200 * 1024 * 1024, // 200MB
				MaxMemoryLeakScore: 0.10,              // 10% memory growth is acceptable
				MaxGCPause:         50 * time.Millisecond,
				MaxErrorRate:       0.005,
			},
			Enabled: true,
		},
		{
			Name:        "High Concurrency Load Test",
			Description: "Test system behavior under high concurrent load",
			Duration:    45 * time.Second,
			Concurrency: 25,
			Operations:  []OperationType{ConcurrentProcessingOp, MCPToolCallOp, ContextRetrievalOp},
			ValidationCriteria: ValidationCriteria{
				MaxResponseTime: 300 * time.Millisecond,
				MaxMemoryUsage:  150 * 1024 * 1024, // 150MB
				MinThroughput:   150.0,
				MaxErrorRate:    0.03,
			},
			Enabled: true,
		},
		{
			Name:        "IO Performance Test",
			Description: "Test file and network I/O performance",
			Duration:    30 * time.Second,
			Concurrency: 8,
			Operations:  []OperationType{FileIOOp, NetworkIOOp},
			ValidationCriteria: ValidationCriteria{
				MaxResponseTime: 250 * time.Millisecond,
				MaxMemoryUsage:  80 * 1024 * 1024, // 80MB
				MinThroughput:   75.0,
				MaxErrorRate:    0.02,
			},
			Enabled: true,
		},
		{
			Name:        "Edge Case Scenario",
			Description: "Test handling of edge cases and error conditions",
			Duration:    20 * time.Second,
			Concurrency: 5,
			Operations:  []OperationType{MCPConnectionOp, ContextCompressionOp, NetworkIOOp},
			ValidationCriteria: ValidationCriteria{
				MaxResponseTime: 1000 * time.Millisecond, // More lenient for edge cases
				MaxMemoryUsage:  100 * 1024 * 1024,
				MinThroughput:   20.0, // Lower throughput expected
				MaxErrorRate:    0.10, // Higher error rate acceptable for edge cases
			},
			Setup: func() error {
				// Setup edge case conditions (network instability, resource constraints, etc.)
				return nil
			},
			Teardown: func() error {
				// Clean up edge case conditions
				return nil
			},
			Enabled: true,
		},
	}
}

// AddScenario adds a custom test scenario
func (sr *ScenarioRunner) AddScenario(scenario TestScenario) {
	sr.scenarios = append(sr.scenarios, scenario)
}

// RunScenario executes a single test scenario
func (sr *ScenarioRunner) RunScenario(scenario TestScenario) (*ScenarioResult, error) {
	if !scenario.Enabled {
		return nil, fmt.Errorf("scenario %s is disabled", scenario.Name)
	}

	result := &ScenarioResult{
		Scenario:         scenario,
		StartTime:        time.Now(),
		Passed:           true,
		Failures:         make([]string, 0),
		OperationResults: make([]OperationResult, 0),
	}

	// Run setup if provided
	if scenario.Setup != nil {
		if err := scenario.Setup(); err != nil {
			return nil, fmt.Errorf("scenario setup failed: %v", err)
		}
	}

	// Run teardown at the end
	defer func() {
		if scenario.Teardown != nil {
			if err := scenario.Teardown(); err != nil {
				fmt.Printf("Warning: scenario teardown failed: %v\n", err)
			}
		}
	}()

	// Execute the scenario
	ctx, cancel := context.WithTimeout(context.Background(), scenario.Duration)
	defer cancel()

	var wg sync.WaitGroup
	operationStats := make(map[OperationType]*OperationStats)
	statsMutex := sync.RWMutex{}

	// Initialize operation stats
	for _, op := range scenario.Operations {
		operationStats[op] = &OperationStats{
			Times:     make([]time.Duration, 0),
			Successes: 0,
			Errors:    0,
		}
	}

	// Start concurrent workers
	for i := 0; i < scenario.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sr.runWorker(ctx, scenario.Operations, operationStats, &statsMutex)
		}()
	}

	wg.Wait()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Collect final metrics
	result.Metrics = sr.framework.collectMetrics()

	// Process operation results
	for op, stats := range operationStats {
		opResult := sr.calculateOperationResult(op, stats)
		result.OperationResults = append(result.OperationResults, opResult)
	}

	// Validate results
	result.Passed, result.Failures = sr.validateScenarioResult(scenario.ValidationCriteria, result.Metrics, result.OperationResults)

	// Store result
	sr.resultsMutex.Lock()
	sr.results = append(sr.results, *result)
	sr.resultsMutex.Unlock()

	return result, nil
}

// OperationStats tracks statistics for operation execution
type OperationStats struct {
	Times     []time.Duration
	Successes int
	Errors    int
	mutex     sync.Mutex
}

// runWorker executes operations in a worker goroutine
func (sr *ScenarioRunner) runWorker(ctx context.Context, operations []OperationType,
	operationStats map[OperationType]*OperationStats, statsMutex *sync.RWMutex) {

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Execute each operation type
			for _, op := range operations {
				start := time.Now()
				success := sr.executeOperation(op)
				duration := time.Since(start)

				statsMutex.RLock()
				stats := operationStats[op]
				statsMutex.RUnlock()

				stats.mutex.Lock()
				stats.Times = append(stats.Times, duration)
				if success {
					stats.Successes++
				} else {
					stats.Errors++
				}
				stats.mutex.Unlock()
			}
		}
	}
}

// executeOperation executes a specific operation type
func (sr *ScenarioRunner) executeOperation(op OperationType) bool {
	switch op {
	case MCPConnectionOp:
		return sr.executeMCPConnection()
	case MCPToolCallOp:
		return sr.executeMCPToolCall()
	case ContextCompressionOp:
		return sr.executeContextCompression()
	case ContextRetrievalOp:
		return sr.executeContextRetrieval()
	case MemoryAllocationOp:
		return sr.executeMemoryAllocation()
	case ConcurrentProcessingOp:
		return sr.executeConcurrentProcessing()
	case NetworkIOOp:
		return sr.executeNetworkIO()
	case FileIOOp:
		return sr.executeFileIO()
	default:
		return false
	}
}

// calculateOperationResult computes metrics for an operation type
func (sr *ScenarioRunner) calculateOperationResult(op OperationType, stats *OperationStats) OperationResult {
	stats.mutex.Lock()
	defer stats.mutex.Unlock()

	if len(stats.Times) == 0 {
		return OperationResult{
			Operation: op,
			Count:     0,
		}
	}

	// Calculate timing statistics
	totalTime := time.Duration(0)
	minTime := stats.Times[0]
	maxTime := stats.Times[0]

	for _, t := range stats.Times {
		totalTime += t
		if t < minTime {
			minTime = t
		}
		if t > maxTime {
			maxTime = t
		}
	}

	count := len(stats.Times)
	avgTime := totalTime / time.Duration(count)
	errorRate := float64(stats.Errors) / float64(stats.Successes+stats.Errors)

	return OperationResult{
		Operation:    op,
		Count:        count,
		TotalTime:    totalTime,
		AverageTime:  avgTime,
		MinTime:      minTime,
		MaxTime:      maxTime,
		SuccessCount: stats.Successes,
		ErrorCount:   stats.Errors,
		ErrorRate:    errorRate,
	}
}

// validateScenarioResult validates scenario results against criteria
func (sr *ScenarioRunner) validateScenarioResult(criteria ValidationCriteria,
	metrics PerformanceMetrics, operationResults []OperationResult) (bool, []string) {

	passed := true
	failures := make([]string, 0)

	// Validate general metrics
	if criteria.MaxResponseTime > 0 && metrics.ResponseTime > criteria.MaxResponseTime {
		passed = false
		failures = append(failures, fmt.Sprintf("Response time %v exceeds limit %v",
			metrics.ResponseTime, criteria.MaxResponseTime))
	}

	if criteria.MaxMemoryUsage > 0 && metrics.HeapSize > criteria.MaxMemoryUsage {
		passed = false
		failures = append(failures, fmt.Sprintf("Memory usage %d exceeds limit %d",
			metrics.HeapSize, criteria.MaxMemoryUsage))
	}

	if criteria.MinThroughput > 0 && metrics.ThroughputOps < criteria.MinThroughput {
		passed = false
		failures = append(failures, fmt.Sprintf("Throughput %.2f below minimum %.2f",
			metrics.ThroughputOps, criteria.MinThroughput))
	}

	if criteria.MaxErrorRate > 0 && metrics.ErrorRate > criteria.MaxErrorRate {
		passed = false
		failures = append(failures, fmt.Sprintf("Error rate %.4f exceeds maximum %.4f",
			metrics.ErrorRate, criteria.MaxErrorRate))
	}

	// Validate operation-specific metrics
	for _, result := range operationResults {
		if result.ErrorRate > criteria.MaxErrorRate {
			passed = false
			failures = append(failures, fmt.Sprintf("Operation %s error rate %.4f exceeds maximum %.4f",
				result.Operation, result.ErrorRate, criteria.MaxErrorRate))
		}

		// Operation-specific validations
		switch result.Operation {
		case MCPConnectionOp:
			if criteria.MaxMCPConnectionTime > 0 && result.AverageTime > criteria.MaxMCPConnectionTime {
				passed = false
				failures = append(failures, fmt.Sprintf("MCP connection time %v exceeds limit %v",
					result.AverageTime, criteria.MaxMCPConnectionTime))
			}
		case MCPToolCallOp:
			if criteria.MaxMCPToolCallLatency > 0 && result.AverageTime > criteria.MaxMCPToolCallLatency {
				passed = false
				failures = append(failures, fmt.Sprintf("MCP tool call latency %v exceeds limit %v",
					result.AverageTime, criteria.MaxMCPToolCallLatency))
			}
		case ContextCompressionOp:
			if criteria.MaxContextCompressionTime > 0 && result.AverageTime > criteria.MaxContextCompressionTime {
				passed = false
				failures = append(failures, fmt.Sprintf("Context compression time %v exceeds limit %v",
					result.AverageTime, criteria.MaxContextCompressionTime))
			}
		case ContextRetrievalOp:
			if criteria.MaxContextRetrievalTime > 0 && result.AverageTime > criteria.MaxContextRetrievalTime {
				passed = false
				failures = append(failures, fmt.Sprintf("Context retrieval time %v exceeds limit %v",
					result.AverageTime, criteria.MaxContextRetrievalTime))
			}
		}
	}

	return passed, failures
}

// RunAllScenarios executes all enabled scenarios
func (sr *ScenarioRunner) RunAllScenarios() ([]ScenarioResult, error) {
	results := make([]ScenarioResult, 0)

	for _, scenario := range sr.scenarios {
		if !scenario.Enabled {
			continue
		}

		fmt.Printf("Running scenario: %s\n", scenario.Name)
		result, err := sr.RunScenario(scenario)
		if err != nil {
			return results, fmt.Errorf("scenario %s failed: %v", scenario.Name, err)
		}

		results = append(results, *result)

		if !result.Passed {
			fmt.Printf("Scenario %s FAILED: %v\n", scenario.Name, result.Failures)
		} else {
			fmt.Printf("Scenario %s PASSED\n", scenario.Name)
		}
	}

	return results, nil
}

// GetResults returns all scenario results
func (sr *ScenarioRunner) GetResults() []ScenarioResult {
	sr.resultsMutex.RLock()
	defer sr.resultsMutex.RUnlock()

	// Return a copy to prevent external modifications
	results := make([]ScenarioResult, len(sr.results))
	copy(results, sr.results)
	return results
}

// Operation execution methods (simplified implementations for demonstration)

func (sr *ScenarioRunner) executeMCPConnection() bool {
	time.Sleep(time.Microsecond * 100) // Simulate MCP connection
	return true
}

func (sr *ScenarioRunner) executeMCPToolCall() bool {
	time.Sleep(time.Microsecond * 50) // Simulate MCP tool call
	return true
}

func (sr *ScenarioRunner) executeContextCompression() bool {
	time.Sleep(time.Microsecond * 200) // Simulate context compression
	return true
}

func (sr *ScenarioRunner) executeContextRetrieval() bool {
	time.Sleep(time.Microsecond * 30) // Simulate context retrieval
	return true
}

func (sr *ScenarioRunner) executeMemoryAllocation() bool {
	// Allocate and release memory
	data := make([]byte, 1024*10) // 10KB
	_ = data
	return true
}

func (sr *ScenarioRunner) executeConcurrentProcessing() bool {
	time.Sleep(time.Microsecond * 75) // Simulate processing
	return true
}

func (sr *ScenarioRunner) executeNetworkIO() bool {
	time.Sleep(time.Microsecond * 150) // Simulate network I/O
	return true
}

func (sr *ScenarioRunner) executeFileIO() bool {
	time.Sleep(time.Microsecond * 100) // Simulate file I/O
	return true
}
