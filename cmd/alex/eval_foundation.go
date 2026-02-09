package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"strings"

	agent_eval "alex/evaluation/agent_eval"
)

func (c *CLI) runFoundationEvaluation(args []string) error {
	fs := flag.NewFlagSet("eval foundation", flag.ContinueOnError)
	var flagBuf bytes.Buffer
	fs.SetOutput(&flagBuf)

	outputDir := fs.String("output", "./evaluation_results/foundation", "Directory to write foundation evaluation outputs")
	mode := fs.String("mode", "web", "Tool mode: web|cli")
	preset := fs.String("preset", "full", "Tool preset (mode-aware, e.g. full/read-only/safe/sandbox/architect/lark-local)")
	toolset := fs.String("toolset", "default", "Toolset to register: default|lark-local")
	casesPath := fs.String("cases", "evaluation/agent_eval/datasets/foundation_eval_cases.yaml", "Path to foundation implicit-intent scenario set (YAML)")
	topK := fs.Int("top-k", 3, "Top-K cutoff for implicit discoverability pass/fail")
	reportFormat := fs.String("format", "markdown", "Report format: markdown|json")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(flagBuf.String()))
	}

	options := agent_eval.DefaultFoundationEvaluationOptions()
	options.OutputDir = *outputDir
	options.Mode = *mode
	options.Preset = *preset
	options.Toolset = *toolset
	options.CasesPath = *casesPath
	options.TopK = *topK
	options.ReportFormat = *reportFormat

	result, err := agent_eval.RunFoundationEvaluation(cliBaseContext(), options)
	if err != nil {
		return err
	}

	log.Printf(
		"Foundation evaluation summary: overall %.1f, prompt %.1f, usability %.1f, discoverability %.1f, pass@1 %d/%d (%.1f%%), pass@5 %d/%d (%.1f%%), top-%d legacy %d/%d (%.1f%%)",
		result.OverallScore,
		result.Prompt.AverageScore,
		result.Tools.AverageUsability,
		result.Tools.AverageDiscoverability,
		result.Implicit.PassAt1Cases,
		result.Implicit.ApplicableCases,
		result.Implicit.PassAt1Rate*100,
		result.Implicit.PassAt5Cases,
		result.Implicit.ApplicableCases,
		result.Implicit.PassAt5Rate*100,
		result.TopK,
		result.Implicit.PassedCases,
		result.Implicit.TotalCases,
		result.Implicit.TopKHitRate*100,
	)
	for _, artifact := range result.ReportArtifacts {
		log.Printf("Foundation artifact: %s (%s) -> %s", artifact.Name, artifact.Format, artifact.Path)
	}

	return nil
}
