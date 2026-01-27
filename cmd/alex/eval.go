package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	agent_eval "alex/evaluation/agent_eval"
)

// handleEval runs the lightweight agent evaluation locally.
func (c *CLI) handleEval(args []string) error {
	fs := flag.NewFlagSet("eval", flag.ContinueOnError)
	var flagBuf bytes.Buffer
	fs.SetOutput(&flagBuf)

	if len(args) > 0 {
		switch args[0] {
		case "agents":
			return c.listAgentProfiles(args[1:])
		case "history":
			return c.listAgentHistory(args[1:])
		case "list":
			return c.listEvaluations(args[1:])
		case "show":
			return c.showEvaluation(args[1:])
		case "delete":
			return c.deleteEvaluation(args[1:])
		}
	}

	defaultDataset := filepath.Join("evaluation", "swe_bench", "real_instances.json")
	datasetPath := fs.String("dataset", defaultDataset, "Path to the SWE-Bench dataset file")
	outputDir := fs.String("output", "./evaluation_results", "Directory to write evaluation outputs")
	instanceLimit := fs.Int("limit", 3, "Number of instances to evaluate")
	workers := fs.Int("workers", 2, "Maximum concurrent workers")
	timeout := fs.Duration("timeout", 300*time.Second, "Per-task timeout")
	reportFormat := fs.String("format", "markdown", "Report format (markdown)")
	disableMetrics := fs.Bool("no-metrics", false, "Disable metrics collection")
	verbose := fs.Bool("v", false, "Enable verbose logging")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(flagBuf.String()))
	}

	cliManager, err := agent_eval.NewCLIManager(*outputDir)
	if err != nil {
		return fmt.Errorf("failed to initialize evaluation manager: %w", err)
	}

	options := agent_eval.DefaultEvaluationOptions()
	options.DatasetPath = *datasetPath
	options.OutputDir = *outputDir
	options.InstanceLimit = *instanceLimit
	options.MaxWorkers = *workers
	options.TimeoutPerTask = *timeout
	options.ReportFormat = *reportFormat
	options.EnableMetrics = !*disableMetrics
	options.Verbose = *verbose

	// Use background context for now; consider cancellation if needed later.
	ctx := cliBaseContext()
	job, err := cliManager.RunEvaluation(ctx, options)
	if err != nil {
		return err
	}

	if job.Results != nil && job.Results.Analysis != nil {
		summary := job.Results.Analysis.Summary
		log.Printf("Evaluation summary: overall %.1f%%, performance grade %s, risk %s", summary.OverallScore*100, summary.PerformanceGrade, summary.RiskLevel)
	}

	return nil
}

func (c *CLI) listAgentProfiles(args []string) error {
	fs := flag.NewFlagSet("eval agents", flag.ContinueOnError)
	var flagBuf bytes.Buffer
	fs.SetOutput(&flagBuf)

	outputDir := fs.String("output", "./evaluation_results", "Directory containing evaluation artifacts")
	limit := fs.Int("limit", 20, "Maximum number of agents to display (0 for all)")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(flagBuf.String()))
	}

	manager := agent_eval.NewEvaluationManager(&agent_eval.EvaluationConfig{OutputDir: *outputDir})
	profiles, err := manager.ListAgentProfiles()
	if err != nil {
		return fmt.Errorf("list agent profiles: %w", err)
	}

	if len(profiles) == 0 {
		fmt.Println("No agent profiles found")
		return nil
	}

	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].UpdatedAt.After(profiles[j].UpdatedAt)
	})
	if *limit > 0 && len(profiles) > *limit {
		profiles = profiles[:*limit]
	}

	fmt.Println("Agent profiles:")
	for _, profile := range profiles {
		fmt.Printf("- %s | success: %.1f%% | quality: %.1f | avg exec: %s | evaluations: %d\n",
			profile.AgentID,
			profile.AvgSuccessRate*100,
			profile.AvgQualityScore,
			profile.AvgExecTime,
			profile.EvaluationCount,
		)
	}

	return nil
}

func (c *CLI) listAgentHistory(args []string) error {
	fs := flag.NewFlagSet("eval history", flag.ContinueOnError)
	var flagBuf bytes.Buffer
	fs.SetOutput(&flagBuf)

	outputDir := fs.String("output", "./evaluation_results", "Directory containing evaluation artifacts")
	limit := fs.Int("limit", 10, "Maximum number of evaluations to display (0 for all)")
	after := fs.String("after", "", "RFC3339 timestamp to include evaluations on/after this time")
	before := fs.String("before", "", "RFC3339 timestamp to include evaluations on/before this time")
	minScore := fs.Float64("min-score", 0, "Minimum overall score (0-1) to include")
	dataset := fs.String("dataset", "", "Substring match for dataset path")
	datasetType := fs.String("dataset-type", "", "Dataset type to filter evaluations")
	tags := fs.String("tags", "", "Comma-separated agent tags to filter evaluations")

	var agentID string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		agentID = args[0]
		args = args[1:]
	}

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(flagBuf.String()))
	}

	if agentID == "" {
		remaining := fs.Args()
		if len(remaining) > 0 {
			agentID = remaining[0]
		}
	}
	if agentID == "" {
		return fmt.Errorf("usage: alex eval history <agent_id> [--output path] [--limit N] [--after RFC3339] [--before RFC3339] [--min-score 0-1] [--dataset path] [--dataset-type type] [--tags tag1,tag2]")
	}

	manager := agent_eval.NewEvaluationManager(&agent_eval.EvaluationConfig{OutputDir: *outputDir})

	if profile, err := manager.GetAgentProfile(agentID); err == nil && profile != nil {
		fmt.Printf("Agent %s | success: %.1f%% | quality: %.1f | evaluations: %d\n",
			profile.AgentID,
			profile.AvgSuccessRate*100,
			profile.AvgQualityScore,
			profile.EvaluationCount,
		)
	}

	query := agent_eval.EvaluationQuery{AgentID: agentID, Limit: *limit}
	if *after != "" {
		parsed, err := time.Parse(time.RFC3339, *after)
		if err != nil {
			return fmt.Errorf("invalid --after timestamp: %w", err)
		}
		query.After = parsed
	}
	if *before != "" {
		parsed, err := time.Parse(time.RFC3339, *before)
		if err != nil {
			return fmt.Errorf("invalid --before timestamp: %w", err)
		}
		query.Before = parsed
	}
	if *minScore > 0 {
		query.MinScore = *minScore
	}
	if *dataset != "" {
		query.DatasetPath = *dataset
	}
	if *datasetType != "" {
		query.DatasetType = *datasetType
	}
	if *tags != "" {
		query.Tags = splitTags(*tags)
	}

	evaluations, err := manager.QueryEvaluations(query)
	if err != nil {
		return fmt.Errorf("list agent evaluations for %s: %w", agentID, err)
	}

	if len(evaluations) == 0 {
		fmt.Println("No evaluations found for agent", agentID)
		return nil
	}

	sort.Slice(evaluations, func(i, j int) bool {
		return evaluations[i].Timestamp.After(evaluations[j].Timestamp)
	})
	if *limit > 0 && len(evaluations) > *limit {
		evaluations = evaluations[:*limit]
	}

	fmt.Printf("Evaluations for %s:\n", agentID)
	for _, eval := range evaluations {
		overall := 0.0
		grade := ""
		if eval.Analysis != nil {
			overall = eval.Analysis.Summary.OverallScore * 100
			grade = eval.Analysis.Summary.PerformanceGrade
		}
		fmt.Printf("- %s | %s | score: %.1f%% %s | tasks: %d\n",
			eval.JobID,
			eval.Timestamp.Format(time.RFC3339),
			overall,
			grade,
			len(eval.Results),
		)
	}

	return nil
}

func (c *CLI) listEvaluations(args []string) error {
	fs := flag.NewFlagSet("eval list", flag.ContinueOnError)
	var flagBuf bytes.Buffer
	fs.SetOutput(&flagBuf)

	outputDir := fs.String("output", "./evaluation_results", "Directory containing evaluation artifacts")
	limit := fs.Int("limit", 20, "Maximum number of evaluations to display (0 for all)")
	agentID := fs.String("agent", "", "Optional agent id to filter evaluations")
	after := fs.String("after", "", "RFC3339 timestamp to include evaluations on/after this time")
	before := fs.String("before", "", "RFC3339 timestamp to include evaluations on/before this time")
	minScore := fs.Float64("min-score", 0, "Minimum overall score (0-1) to include")
	dataset := fs.String("dataset", "", "Substring match for dataset path")
	datasetType := fs.String("dataset-type", "", "Dataset type to filter evaluations")
	tags := fs.String("tags", "", "Comma-separated agent tags to filter evaluations")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(flagBuf.String()))
	}

	manager := agent_eval.NewEvaluationManager(&agent_eval.EvaluationConfig{OutputDir: *outputDir})

	query := agent_eval.EvaluationQuery{AgentID: *agentID, Limit: *limit}
	if *after != "" {
		parsed, err := time.Parse(time.RFC3339, *after)
		if err != nil {
			return fmt.Errorf("invalid --after timestamp: %w", err)
		}
		query.After = parsed
	}
	if *before != "" {
		parsed, err := time.Parse(time.RFC3339, *before)
		if err != nil {
			return fmt.Errorf("invalid --before timestamp: %w", err)
		}
		query.Before = parsed
	}
	if *minScore > 0 {
		query.MinScore = *minScore
	}
	if *dataset != "" {
		query.DatasetPath = *dataset
	}
	if *datasetType != "" {
		query.DatasetType = *datasetType
	}
	if *tags != "" {
		query.Tags = splitTags(*tags)
	}

	evaluations, err := manager.QueryEvaluations(query)
	if err != nil {
		if *agentID == "" {
			return fmt.Errorf("list evaluations: %w", err)
		}
		return fmt.Errorf("list evaluations for agent %s: %w", *agentID, err)
	}

	if len(evaluations) == 0 {
		if *agentID == "" {
			fmt.Println("No evaluations found")
		} else {
			fmt.Println("No evaluations found for agent", *agentID)
		}
		return nil
	}

	sort.Slice(evaluations, func(i, j int) bool {
		return evaluations[i].Timestamp.After(evaluations[j].Timestamp)
	})
	if *limit > 0 && len(evaluations) > *limit {
		evaluations = evaluations[:*limit]
	}

	fmt.Println("Evaluations:")
	for _, eval := range evaluations {
		overall := 0.0
		grade := ""
		if eval.Analysis != nil {
			overall = eval.Analysis.Summary.OverallScore * 100
			grade = eval.Analysis.Summary.PerformanceGrade
		}

		taskCount := len(eval.Results)
		if eval.Metrics != nil && eval.Metrics.TotalTasks > 0 {
			taskCount = eval.Metrics.TotalTasks
		}

		fmt.Printf("- %s | agent: %s | %s | score: %.1f%% %s | tasks: %d\n",
			eval.JobID,
			eval.AgentID,
			eval.Timestamp.Format(time.RFC3339),
			overall,
			grade,
			taskCount,
		)
	}

	return nil
}

func (c *CLI) showEvaluation(args []string) error {
	fs := flag.NewFlagSet("eval show", flag.ContinueOnError)
	var flagBuf bytes.Buffer
	fs.SetOutput(&flagBuf)

	jobID := fs.String("job", "", "Evaluation job id to inspect")
	outputDir := fs.String("output", "./evaluation_results", "Directory containing evaluation artifacts")
	maxTasks := fs.Int("tasks", 10, "Maximum number of task results to display (0 for all)")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(flagBuf.String()))
	}
	if *jobID == "" {
		return fmt.Errorf("job id is required")
	}

	manager := agent_eval.NewEvaluationManager(&agent_eval.EvaluationConfig{OutputDir: *outputDir})
	results, err := manager.GetJobResults(*jobID)
	if err != nil {
		return fmt.Errorf("load evaluation %s: %w", *jobID, err)
	}

	fmt.Printf("Evaluation %s (agent: %s)\n", results.JobID, results.AgentID)
	fmt.Printf("Timestamp: %s\n", results.Timestamp.Format(time.RFC3339))

	if results.Analysis != nil {
		summary := results.Analysis.Summary
		fmt.Printf("Overall: %.1f%% | Grade: %s | Risk: %s\n", summary.OverallScore*100, summary.PerformanceGrade, summary.RiskLevel)
	}

	if results.Metrics != nil {
		fmt.Printf("Tasks: %d | Success rate: %.1f%% | Avg exec: %s | Avg cost: %.2f\n",
			results.Metrics.TotalTasks,
			results.Metrics.Performance.SuccessRate*100,
			results.Metrics.Performance.AvgExecutionTime,
			results.Metrics.Resources.AvgCostPerTask,
		)
	}

	if len(results.AutoScores) > 0 {
		fmt.Println("Task scores:")
		limit := len(results.AutoScores)
		if *maxTasks > 0 && limit > *maxTasks {
			limit = *maxTasks
		}
		for i := 0; i < limit; i++ {
			score := results.AutoScores[i]
			var status string
			for _, res := range results.Results {
				if res.TaskID == score.TaskID {
					status = string(res.Status)
					break
				}
			}
			fmt.Printf("- %s | instance: %s | score: %.1f (%s) | status: %s | reason: %s\n",
				score.TaskID,
				score.InstanceID,
				score.Score,
				score.Grade,
				status,
				score.Reason,
			)
		}
		if *maxTasks > 0 && len(results.AutoScores) > *maxTasks {
			fmt.Printf("... %d more tasks omitted\n", len(results.AutoScores)-*maxTasks)
		}
	}

	return nil
}

func (c *CLI) deleteEvaluation(args []string) error {
	fs := flag.NewFlagSet("eval delete", flag.ContinueOnError)
	var flagBuf bytes.Buffer
	fs.SetOutput(&flagBuf)

	jobID := fs.String("job", "", "Evaluation job id to delete")
	outputDir := fs.String("output", "./evaluation_results", "Directory containing evaluation artifacts")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(flagBuf.String()))
	}
	if *jobID == "" {
		return fmt.Errorf("job id is required")
	}

	manager := agent_eval.NewEvaluationManager(&agent_eval.EvaluationConfig{OutputDir: *outputDir})
	if err := manager.DeleteEvaluation(*jobID); err != nil {
		return fmt.Errorf("delete evaluation %s: %w", *jobID, err)
	}

	fmt.Printf("Deleted evaluation %s from %s\n", *jobID, *outputDir)
	return nil
}

func splitTags(csv string) []string {
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		tags = append(tags, trimmed)
	}
	return tags
}
