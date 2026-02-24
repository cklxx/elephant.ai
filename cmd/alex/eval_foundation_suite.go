package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"strings"

	agent_eval "alex/evaluation/agent_eval"
)

func (c *CLI) runFoundationSuiteEvaluation(args []string) error {
	fs := flag.NewFlagSet("eval foundation-suite", flag.ContinueOnError)
	var flagBuf bytes.Buffer
	fs.SetOutput(&flagBuf)

	outputDir := fs.String("output", "./evaluation_results/foundation-suite", "Directory to write foundation suite outputs")
	suitePath := fs.String("suite", "evaluation/agent_eval/datasets/foundation_eval_suite.yaml", "Path to foundation suite set (YAML)")
	reportFormat := fs.String("format", "markdown", "Report format: markdown|json")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(flagBuf.String()))
	}

	result, err := agent_eval.RunFoundationEvaluationSuite(cliBaseContext(), &agent_eval.FoundationSuiteOptions{
		OutputDir:    *outputDir,
		SuitePath:    *suitePath,
		ReportFormat: *reportFormat,
	})
	if err != nil {
		return err
	}

	log.Printf(
		"Foundation suite summary: collections %s passed, cases %s passed, pass@1 %d/%d (avg %.1f%%), pass@5 %d/%d (avg %.1f%%), avg overall %.1f, failed cases %d, availability errors %d, duration %dms, throughput %.2f cases/s, case p95/p99 %.3f/%.3f ms",
		result.CollectionPassRatio,
		result.CasePassRatio,
		result.PassAt1Cases,
		result.ApplicableCases,
		result.AveragePassAt1Rate*100,
		result.PassAt5Cases,
		result.ApplicableCases,
		result.AveragePassAt5Rate*100,
		result.AverageOverallScore,
		result.FailedCases,
		result.AvailabilityErrors,
		result.TotalDurationMs,
		result.ThroughputCasesPerSec,
		result.CaseLatencyP95Ms,
		result.CaseLatencyP99Ms,
	)
	for _, artifact := range result.ReportArtifacts {
		log.Printf("Foundation suite artifact: %s (%s) -> %s", artifact.Name, artifact.Format, artifact.Path)
	}

	return nil
}
