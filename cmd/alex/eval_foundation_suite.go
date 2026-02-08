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
		"Foundation suite summary: collections %d/%d passed, avg overall %.1f, avg top-1 %.1f%%, avg top-k %.1f%%, failed cases %d, availability errors %d",
		result.PassedCollections,
		result.TotalCollections,
		result.AverageOverallScore,
		result.AverageTop1HitRate*100,
		result.AverageTopKHitRate*100,
		result.FailedCases,
		result.AvailabilityErrors,
	)
	for _, artifact := range result.ReportArtifacts {
		log.Printf("Foundation suite artifact: %s (%s) -> %s", artifact.Name, artifact.Format, artifact.Path)
	}

	return nil
}
