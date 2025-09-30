package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"alex/evaluation/swe_bench"

	"github.com/spf13/cobra"
)

// newBatchCommand creates the run-batch subcommand
func newBatchCommand() *cobra.Command {
	var (
		// Configuration file
		configFile string

		// Model configuration
		modelName        string
		modelTemperature float64
		modelMaxTokens   int

		// Agent configuration
		maxTurns  int
		costLimit float64
		timeout   int

		// Dataset configuration
		datasetType   string
		datasetSubset string
		datasetSplit  string
		datasetFile   string
		instanceLimit int
		instanceSlice string
		instanceIDs   string
		shuffle       bool

		// Execution configuration
		numWorkers    int
		outputPath    string
		resumeFrom    string
		enableLogging bool
		failFast      bool
		maxRetries    int

		// Output options
		quiet    bool
		verbose  bool
		progress bool
	)

	cmd := &cobra.Command{
		Use:   "run-batch",
		Short: "Run SWE-bench batch processing",
		Long: `Run batch processing on SWE-bench dataset with configurable parameters.

Examples:
  # Run with default configuration
  alex run-batch

  # Run with specific model and workers
  alex run-batch --model gpt-4o --workers 5

  # Run with custom configuration file
  alex run-batch --config batch_config.yaml

  # Run on specific dataset subset
  alex run-batch --dataset.type swe_bench --dataset.subset lite --dataset.split dev

  # Resume from previous run
  alex run-batch --resume ./batch_results

  # Run with custom output path
  alex run-batch --output ./my_results`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Create configuration manager
			configManager := swe_bench.NewConfigManager()

			// Load base configuration
			var config *swe_bench.BatchConfig
			var err error

			if configFile != "" {
				config, err = configManager.LoadConfig(configFile)
				if err != nil {
					return fmt.Errorf("failed to load config file: %w", err)
				}
			} else {
				config = swe_bench.DefaultBatchConfig()
			}

			// Apply command line overrides
			applyOverrides(config, modelName, modelTemperature, modelMaxTokens, maxTurns, costLimit, timeout,
				datasetType, datasetSubset, datasetSplit, datasetFile, instanceLimit, instanceSlice, instanceIDs, shuffle,
				numWorkers, outputPath, resumeFrom, enableLogging, failFast, maxRetries)

			// Validate configuration
			if err := configManager.ValidateConfig(config); err != nil {
				return fmt.Errorf("invalid configuration: %w", err)
			}

			// Setup output
			if !quiet {
				fmt.Printf("SWE-bench Batch Processing\n")
				fmt.Printf("==========================\n\n")
				fmt.Printf("Configuration:\n")
				fmt.Printf("  Model: %s\n", config.Agent.Model.Name)
				fmt.Printf("  Dataset: %s (%s, %s)\n", config.Instances.Type, config.Instances.Subset, config.Instances.Split)
				fmt.Printf("  Workers: %d\n", config.NumWorkers)
				fmt.Printf("  Output: %s\n", config.OutputPath)
				if config.ResumeFrom != "" {
					fmt.Printf("  Resume from: %s\n", config.ResumeFrom)
				}
				fmt.Printf("\n")
			}

			// Create output directory
			if err := os.MkdirAll(config.OutputPath, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}

			// Save configuration to output directory
			configPath := filepath.Join(config.OutputPath, "config.yaml")
			if err := configManager.SaveConfig(config, configPath); err != nil {
				log.Printf("Warning: Failed to save config: %v", err)
			}

			// Check if resuming from previous run
			if config.ResumeFrom != "" {
				return runResumeCommand(ctx, config, quiet, progress)
			}

			// Run batch processing
			return runNewBatch(ctx, config, quiet, progress)
		},
	}

	// Add flags
	cmd.Flags().StringVar(&configFile, "config", "", "Configuration file path (YAML)")

	// Model configuration
	cmd.Flags().StringVar(&modelName, "model", "", "Model name (e.g., 'gpt-4o', 'deepseek/deepseek-chat-v3-0324:free')")
	cmd.Flags().Float64Var(&modelTemperature, "temperature", 0, "Model temperature (0.0-2.0)")
	cmd.Flags().IntVar(&modelMaxTokens, "max-tokens", 0, "Maximum tokens per request")

	// Agent configuration
	cmd.Flags().IntVar(&maxTurns, "max-turns", 0, "Maximum turns per instance")
	cmd.Flags().Float64Var(&costLimit, "cost-limit", 0, "Cost limit per instance")
	cmd.Flags().IntVar(&timeout, "timeout", 0, "Timeout per instance (seconds)")

	// Dataset configuration
	cmd.Flags().StringVar(&datasetType, "dataset.type", "", "Dataset type (swe_bench, file, huggingface)")
	cmd.Flags().StringVar(&datasetSubset, "dataset.subset", "", "Dataset subset (lite, full, verified)")
	cmd.Flags().StringVar(&datasetSplit, "dataset.split", "", "Dataset split (dev, test, train)")
	cmd.Flags().StringVar(&datasetFile, "dataset.file", "", "Dataset file path (for file type)")
	cmd.Flags().IntVar(&instanceLimit, "instance-limit", 0, "Limit number of instances")
	cmd.Flags().StringVar(&instanceSlice, "instance-slice", "", "Instance slice (format: 'start,end')")
	cmd.Flags().StringVar(&instanceIDs, "instance-ids", "", "Specific instance IDs (comma-separated)")
	cmd.Flags().BoolVar(&shuffle, "shuffle", false, "Shuffle instances")

	// Execution configuration
	cmd.Flags().IntVar(&numWorkers, "workers", 0, "Number of worker processes")
	cmd.Flags().StringVar(&outputPath, "output", "", "Output directory path")
	cmd.Flags().StringVar(&resumeFrom, "resume", "", "Resume from previous results directory")
	cmd.Flags().BoolVar(&enableLogging, "logging", false, "Enable detailed logging")
	cmd.Flags().BoolVar(&failFast, "fail-fast", false, "Stop on first failure")
	cmd.Flags().IntVar(&maxRetries, "max-retries", 0, "Maximum retries per instance")

	// Output options
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Quiet output (errors only)")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Verbose output")
	cmd.Flags().BoolVar(&progress, "progress", true, "Show progress updates")

	return cmd
}

func applyOverrides(config *swe_bench.BatchConfig, modelName string, modelTemperature float64, modelMaxTokens int,
	maxTurns int, costLimit float64, timeout int, datasetType, datasetSubset, datasetSplit, datasetFile string,
	instanceLimit int, instanceSlice, instanceIDs string, shuffle bool, numWorkers int, outputPath, resumeFrom string,
	enableLogging, failFast bool, maxRetries int) {

	// Model configuration
	if modelName != "" {
		config.Agent.Model.Name = modelName
	}
	if modelTemperature != 0 {
		config.Agent.Model.Temperature = modelTemperature
	}
	if modelMaxTokens != 0 {
		config.Agent.Model.MaxTokens = modelMaxTokens
	}

	// Agent configuration
	if maxTurns != 0 {
		config.Agent.MaxTurns = maxTurns
	}
	if costLimit != 0 {
		config.Agent.CostLimit = costLimit
	}
	if timeout != 0 {
		config.Agent.Timeout = timeout
	}

	// Dataset configuration
	if datasetType != "" {
		config.Instances.Type = datasetType
	}
	if datasetSubset != "" {
		config.Instances.Subset = datasetSubset
	}
	if datasetSplit != "" {
		config.Instances.Split = datasetSplit
	}
	if datasetFile != "" {
		config.Instances.FilePath = datasetFile
	}
	if instanceLimit != 0 {
		config.Instances.InstanceLimit = instanceLimit
	}
	if instanceSlice != "" {
		if slice, err := parseInstanceSlice(instanceSlice); err == nil {
			config.Instances.InstanceSlice = slice
		}
	}
	if instanceIDs != "" {
		config.Instances.InstanceIDs = strings.Split(instanceIDs, ",")
	}
	if shuffle {
		config.Instances.Shuffle = shuffle
	}

	// Execution configuration
	if numWorkers != 0 {
		config.NumWorkers = numWorkers
	}
	if outputPath != "" {
		config.OutputPath = outputPath
	}
	if resumeFrom != "" {
		config.ResumeFrom = resumeFrom
	}
	if enableLogging {
		config.EnableLogging = enableLogging
	}
	if failFast {
		config.FailFast = failFast
	}
	if maxRetries != 0 {
		config.MaxRetries = maxRetries
	}
}

func parseInstanceSlice(slice string) ([]int, error) {
	parts := strings.Split(slice, ",")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid slice format, expected 'start,end'")
	}

	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid start value: %w", err)
	}

	end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("invalid end value: %w", err)
	}

	return []int{start, end}, nil
}

func runNewBatch(ctx context.Context, config *swe_bench.BatchConfig, quiet, progress bool) error {
	// Create dataset loader
	dataLoader := swe_bench.NewDatasetLoader()

	// Load instances
	if !quiet {
		fmt.Printf("Loading dataset instances...\n")
	}

	instances, err := dataLoader.LoadInstances(ctx, config.Instances)
	if err != nil {
		return fmt.Errorf("failed to load instances: %w", err)
	}

	if !quiet {
		fmt.Printf("Loaded %d instances\n\n", len(instances))
	}

	// Create batch processor
	processor := swe_bench.NewBatchProcessor(config)

	// Setup progress reporting
	if progress && !quiet {
		setupProgressReporting(processor)
	}

	// Run batch processing
	if !quiet {
		fmt.Printf("Starting batch processing...\n")
	}

	startTime := time.Now()
	result, err := processor.ProcessBatch(ctx, instances, config)
	if err != nil {
		return fmt.Errorf("batch processing failed: %w", err)
	}

	// Print results summary
	if !quiet {
		printResultsSummary(result, time.Since(startTime))
	}

	return nil
}

func runResumeCommand(ctx context.Context, config *swe_bench.BatchConfig, quiet, progress bool) error {
	if !quiet {
		fmt.Printf("Resuming batch processing from: %s\n\n", config.ResumeFrom)
	}

	// Create batch processor
	processor := swe_bench.NewBatchProcessor(config)

	// Setup progress reporting
	if progress && !quiet {
		setupProgressReporting(processor)
	}

	// Resume processing
	startTime := time.Now()
	result, err := processor.Resume(ctx, config.ResumeFrom, config)
	if err != nil {
		return fmt.Errorf("failed to resume batch processing: %w", err)
	}

	// Print results summary
	if !quiet {
		printResultsSummary(result, time.Since(startTime))
	}

	return nil
}

func setupProgressReporting(processor *swe_bench.BatchProcessorImpl) {
	// This would setup real-time progress reporting
	// For now, this is a placeholder
}

func printResultsSummary(result *swe_bench.BatchResult, duration time.Duration) {
	fmt.Printf("\n")
	fmt.Printf("Batch Processing Results\n")
	fmt.Printf("========================\n\n")
	fmt.Printf("Total Tasks:     %d\n", result.TotalTasks)
	fmt.Printf("Completed:       %d\n", result.CompletedTasks)
	fmt.Printf("Failed:          %d\n", result.FailedTasks)
	fmt.Printf("Success Rate:    %.2f%%\n", result.SuccessRate)
	fmt.Printf("Total Duration:  %s\n", duration)
	fmt.Printf("Avg Duration:    %s\n", result.AvgDuration)
	fmt.Printf("Total Tokens:    %d\n", result.TotalTokens)
	fmt.Printf("Total Cost:      $%.4f\n", result.TotalCost)

	if len(result.ErrorSummary) > 0 {
		fmt.Printf("\nError Summary:\n")
		for errorType, count := range result.ErrorSummary {
			fmt.Printf("  %s: %d\n", errorType, count)
		}
	}

	fmt.Printf("\nResults saved to: %s\n", result.Config.OutputPath)
	fmt.Printf("  - batch_results.json (full results)\n")
	fmt.Printf("  - preds.json (SWE-bench format)\n")
	fmt.Printf("  - summary.json (summary)\n")
	fmt.Printf("  - detailed_results.json (detailed)\n")
}
