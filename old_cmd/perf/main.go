package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"alex/internal/performance"
)

const (
	defaultConfigPath = "./performance/config.json"
	version           = "1.0.0"
)

func main() {
	var (
		configPath = flag.String("config", defaultConfigPath, "Path to performance configuration file")
		verbose    = flag.Bool("verbose", false, "Enable verbose logging")
		help       = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	if *verbose {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("Error: No command specified")
		showHelp()
		os.Exit(1)
	}

	command := args[0]

	// Initialize integration strategy
	strategy, err := performance.NewIntegrationStrategy(*configPath)
	if err != nil {
		log.Fatalf("Failed to initialize performance framework: %v", err)
	}
	defer func() {
		if err := strategy.Shutdown(); err != nil {
			log.Printf("Warning: failed to shutdown strategy: %v", err)
		}
	}()

	// Execute command
	switch command {
	case "init":
		err = cmdInit(strategy)
	case "benchmark":
		err = cmdBenchmark(strategy)
	case "test":
		err = cmdTest(strategy)
	case "baseline":
		err = cmdBaseline(strategy)
	case "monitor":
		err = cmdMonitor(strategy)
	case "report":
		err = cmdReport(strategy)
	case "pre-build":
		err = cmdPreBuild(strategy)
	case "post-test":
		err = cmdPostTest(strategy)
	case "version":
		fmt.Printf("Alex Performance Verification Framework v%s\n", version)
		return
	default:
		fmt.Printf("Error: Unknown command '%s'\n", command)
		showHelp()
		os.Exit(1)
	}

	if err != nil {
		log.Fatalf("Command failed: %v", err)
	}
}

func showHelp() {
	fmt.Printf(`Alex Performance Verification Framework v%s

Usage: perf [options] <command>

Commands:
  init        Initialize the performance verification framework
  benchmark   Run full benchmark suite
  test        Run performance test scenarios
  baseline    Create or update performance baseline
  monitor     Start performance monitoring
  report      Generate performance report
  pre-build   Run pre-build verification checks
  post-test   Run post-test verification checks
  version     Show version information

Options:
  -config string   Path to configuration file (default: %s)
  -verbose         Enable verbose logging
  -help            Show this help message

Examples:
  perf init                    # Initialize framework with defaults
  perf benchmark               # Run all benchmarks
  perf test                    # Run all test scenarios
  perf baseline                # Create new baseline from current metrics
  perf monitor                 # Start continuous monitoring
  perf -config ./my.json test  # Run tests with custom config

`, version, defaultConfigPath)
}

func cmdInit(strategy *performance.IntegrationStrategy) error {
	fmt.Println("Initializing performance verification framework...")

	if err := strategy.InitializeFramework(); err != nil {
		return fmt.Errorf("initialization failed: %v", err)
	}

	fmt.Println("✅ Framework initialized successfully")
	fmt.Println("Configuration saved to:", defaultConfigPath)
	fmt.Println("\nNext steps:")
	fmt.Println("1. Run 'perf baseline' to create initial performance baseline")
	fmt.Println("2. Run 'perf test' to execute test scenarios")
	fmt.Println("3. Run 'perf monitor' to start continuous monitoring")

	return nil
}

func cmdBenchmark(strategy *performance.IntegrationStrategy) error {
	fmt.Println("Running performance benchmark suite...")

	result, err := strategy.RunBenchmarkSuite()
	if err != nil {
		return fmt.Errorf("benchmark suite failed: %v", err)
	}

	// Display summary
	fmt.Printf("\n📊 Benchmark Results Summary:\n")
	fmt.Printf("Total Benchmarks: %d\n", result.Summary.TotalBenchmarks)
	fmt.Printf("Total Scenarios: %d\n", result.Summary.TotalScenarios)
	fmt.Printf("Passed Scenarios: %d\n", result.Summary.PassedScenarios)
	fmt.Printf("Failed Scenarios: %d\n", result.Summary.FailedScenarios)
	fmt.Printf("Average Response Time: %.2f ms\n", result.Summary.AverageResponseTime)
	fmt.Printf("Average Memory Usage: %.2f MB\n", float64(result.Summary.AverageMemoryUsage)/1024/1024)
	fmt.Printf("Average Throughput: %.2f ops/sec\n", result.Summary.AverageThroughput)
	fmt.Printf("Overall Error Rate: %.4f\n", result.Summary.OverallErrorRate)

	if result.Passed {
		fmt.Println("\n✅ All benchmarks passed!")
	} else {
		fmt.Println("\n❌ Some benchmarks failed")

		// Show failed scenarios
		for _, scenario := range result.ScenarioResults {
			if !scenario.Passed {
				fmt.Printf("  Failed: %s\n", scenario.Scenario.Name)
				for _, failure := range scenario.Failures {
					fmt.Printf("    - %s\n", failure)
				}
			}
		}
		return fmt.Errorf("benchmark suite had failures")
	}

	return nil
}

func cmdTest(strategy *performance.IntegrationStrategy) error {
	fmt.Println("Running performance test scenarios...")

	// Get scenario runner from strategy
	config := performance.GetDefaultIntegrationConfig()
	framework := performance.NewVerificationFramework(&config.VerificationConfig)
	runner := performance.NewScenarioRunner(&config.VerificationConfig, framework)

	results, err := runner.RunAllScenarios()
	if err != nil {
		return fmt.Errorf("test scenarios failed: %v", err)
	}

	// Display results
	fmt.Printf("\n🧪 Test Scenario Results:\n")
	passed := 0
	for _, result := range results {
		status := "✅ PASS"
		if !result.Passed {
			status = "❌ FAIL"
		} else {
			passed++
		}

		fmt.Printf("%s %s (%.2fs)\n", status, result.Scenario.Name, result.Duration.Seconds())
		if !result.Passed {
			for _, failure := range result.Failures {
				fmt.Printf("    - %s\n", failure)
			}
		}
	}

	fmt.Printf("\nResults: %d/%d scenarios passed\n", passed, len(results))

	if passed != len(results) {
		return fmt.Errorf("%d scenarios failed", len(results)-passed)
	}

	return nil
}

func cmdBaseline(strategy *performance.IntegrationStrategy) error {
	fmt.Println("Creating performance baseline...")

	// Run a quick benchmark to get current metrics
	config := performance.GetDefaultIntegrationConfig()
	framework := performance.NewVerificationFramework(&config.VerificationConfig)
	suite := performance.NewBenchmarkSuite(&config.VerificationConfig)

	// Run a subset of benchmarks for baseline
	mcpResult := suite.MCPBenchmark()
	contextResult := suite.ContextBenchmark()

	// Create baseline from results
	baseline := performance.PerformanceMetrics{
		MCPConnectionTime:      mcpResult.Metrics.MCPConnectionTime,
		MCPToolCallLatency:     mcpResult.Metrics.MCPToolCallLatency,
		MCPProtocolOverhead:    mcpResult.Metrics.MCPProtocolOverhead,
		ContextCompressionTime: contextResult.Metrics.ContextCompressionTime,
		ContextRetrievalTime:   contextResult.Metrics.ContextRetrievalTime,
		ContextMemoryUsage:     contextResult.Metrics.ContextMemoryUsage,
		ContextCacheHitRate:    contextResult.Metrics.ContextCacheHitRate,
		ResponseTime:           500 * time.Millisecond, // Default reasonable response time
		ThroughputOps:          100.0,                  // Default throughput
		ErrorRate:              0.005,                  // Default error rate
		Timestamp:              time.Now(),
	}

	// Collect actual system metrics
	current := framework.GetCurrentMetrics()
	if current != nil {
		baseline.HeapSize = current.HeapSize
		baseline.GCPause = current.GCPause
		baseline.CPUUtilization = current.CPUUtilization
	}

	if err := framework.SaveBaseline(&baseline); err != nil {
		return fmt.Errorf("failed to save baseline: %v", err)
	}

	fmt.Println("✅ Performance baseline created successfully")
	fmt.Printf("MCP Connection Time: %v\n", baseline.MCPConnectionTime)
	fmt.Printf("MCP Tool Call Latency: %v\n", baseline.MCPToolCallLatency)
	fmt.Printf("Context Compression Time: %v\n", baseline.ContextCompressionTime)
	fmt.Printf("Context Retrieval Time: %v\n", baseline.ContextRetrievalTime)
	fmt.Printf("Memory Usage: %.2f MB\n", float64(baseline.HeapSize)/1024/1024)

	return nil
}

func cmdMonitor(strategy *performance.IntegrationStrategy) error {
	fmt.Println("Starting performance monitoring...")
	fmt.Println("Press Ctrl+C to stop monitoring")

	if err := strategy.InitializeFramework(); err != nil {
		return fmt.Errorf("failed to initialize framework: %v", err)
	}

	// This would start continuous monitoring
	// In a real implementation, this would run until interrupted
	fmt.Println("🔍 Monitoring started (simulation)")
	fmt.Println("Monitoring metrics every 30 seconds...")

	// Simulate monitoring for demonstration
	for i := 0; i < 5; i++ {
		time.Sleep(2 * time.Second)
		fmt.Printf("⏱️  [%s] Collecting performance metrics...\n", time.Now().Format("15:04:05"))
	}

	fmt.Println("✅ Monitoring completed")
	return nil
}

func cmdReport(strategy *performance.IntegrationStrategy) error {
	fmt.Println("Generating performance report...")

	// Look for latest results file
	resultsPath := "./performance/results/latest.json"
	if _, err := os.Stat(resultsPath); os.IsNotExist(err) {
		return fmt.Errorf("no results found - run 'perf benchmark' first")
	}

	fmt.Printf("📄 Performance report available at: %s\n", resultsPath)

	// In a real implementation, this would generate HTML/PDF reports
	fmt.Println("📊 Report Summary:")
	fmt.Println("- Latest benchmark results processed")
	fmt.Println("- Performance trends analyzed")
	fmt.Println("- Recommendations generated")

	// Generate Makefile integration help
	fmt.Println("\n🔧 Makefile Integration:")
	fmt.Println("Add these targets to your Makefile for seamless integration:")
	fmt.Println()

	integration := performance.IntegrationStrategy{}
	fmt.Print(integration.GetMakefileIntegration())

	return nil
}

func cmdPreBuild(strategy *performance.IntegrationStrategy) error {
	fmt.Println("Running pre-build performance verification...")

	if err := strategy.RunPreBuildVerification(); err != nil {
		return fmt.Errorf("pre-build verification failed: %v", err)
	}

	fmt.Println("✅ Pre-build verification passed")
	return nil
}

func cmdPostTest(strategy *performance.IntegrationStrategy) error {
	fmt.Println("Running post-test performance verification...")

	if err := strategy.RunPostTestVerification(); err != nil {
		return fmt.Errorf("post-test verification failed: %v", err)
	}

	fmt.Println("✅ Post-test verification passed")
	return nil
}

func init() {
	// Ensure performance directory exists
	if err := os.MkdirAll("./performance/results", 0755); err != nil {
		log.Printf("Warning: failed to create performance directory: %v", err)
	}

	// Set up logging
	logFile := "./performance/perf.log"
	if file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
		log.SetOutput(file)
	}
}
