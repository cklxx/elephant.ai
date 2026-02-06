package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	larktesting "alex/internal/delivery/channels/lark/testing"
)

func runLarkCommand(args []string) error {
	if len(args) == 0 {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("usage: alex lark scenario run [--dir path] [--json-out file] [--md-out file]")}
	}

	switch args[0] {
	case "scenario", "scenarios":
		return runLarkScenarioCommand(args[1:])
	default:
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("unknown lark subcommand %q (expected: scenario)", args[0])}
	}
}

func runLarkScenarioCommand(args []string) error {
	if len(args) == 0 {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("usage: alex lark scenario run [--dir path] [--json-out file] [--md-out file]")}
	}
	switch args[0] {
	case "run":
		return runLarkScenarioRun(args[1:])
	default:
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("unknown lark scenario subcommand %q (expected: run)", args[0])}
	}
}

type stringListFlag []string

func (s *stringListFlag) String() string { return strings.Join(*s, ",") }
func (s *stringListFlag) Set(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			*s = append(*s, part)
		}
	}
	return nil
}

func runLarkScenarioRun(args []string) error {
	fs := flag.NewFlagSet("alex lark scenario run", flag.ContinueOnError)
	var flagBuf bytes.Buffer
	fs.SetOutput(&flagBuf)

	defaultDir := filepath.Join("tests", "scenarios", "lark")
	dir := fs.String("dir", defaultDir, "Directory containing Lark scenario .yaml files")
	jsonOut := fs.String("json-out", "", "Write JSON report to this file (optional)")
	mdOut := fs.String("md-out", "", "Write Markdown report to this file (optional)")
	name := fs.String("name", "", "Run only a single scenario by name (optional)")
	failFast := fs.Bool("fail-fast", false, "Stop after the first failing scenario")
	var tags stringListFlag
	fs.Var(&tags, "tag", "Run only scenarios that contain these tag(s). Can be repeated or comma-separated.")

	if err := fs.Parse(args); err != nil {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("%v: %s", err, strings.TrimSpace(flagBuf.String()))}
	}

	scenarios, err := larktesting.LoadScenariosFromDir(*dir)
	if err != nil {
		return &ExitCodeError{Code: 2, Err: err}
	}

	filtered := filterScenarios(scenarios, *name, tags)
	if len(filtered) == 0 {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("no scenarios matched (dir=%s name=%q tags=%v)", *dir, *name, []string(tags))}
	}

	runner := larktesting.NewRunner(nil)
	ctx := context.Background()
	var results []*larktesting.ScenarioResult

	for _, s := range filtered {
		res := runner.Run(ctx, s)
		results = append(results, res)
		if *failFast && !res.Passed {
			break
		}
	}

	report := larktesting.BuildReport(results)

	if *jsonOut != "" {
		if err := writeFile(*jsonOut, mustJSON(report)); err != nil {
			return &ExitCodeError{Code: 2, Err: err}
		}
	}
	if *mdOut != "" {
		if err := writeFile(*mdOut, []byte(report.ToMarkdown())); err != nil {
			return &ExitCodeError{Code: 2, Err: err}
		}
	}

	fmt.Printf("Lark scenarios: total=%d passed=%d failed=%d duration=%s\n",
		report.Summary.Total, report.Summary.Passed, report.Summary.Failed, report.Duration.Round(0).String())
	if *jsonOut != "" {
		fmt.Printf("JSON report: %s\n", *jsonOut)
	}
	if *mdOut != "" {
		fmt.Printf("Markdown report: %s\n", *mdOut)
	}

	if report.Summary.Failed > 0 {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("lark scenarios failed: %d", report.Summary.Failed)}
	}
	return nil
}

func filterScenarios(all []*larktesting.Scenario, name string, tags []string) []*larktesting.Scenario {
	var out []*larktesting.Scenario
	manualRequested := hasTag(tags, "manual")
	for _, s := range all {
		if name != "" && s.Name != name {
			continue
		}
		if hasTag(s.Tags, "manual") && name == "" && !manualRequested {
			continue
		}
		if len(tags) > 0 && !scenarioHasAnyTag(s, tags) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func hasTag(tags []string, needle string) bool {
	for _, tag := range tags {
		if strings.EqualFold(strings.TrimSpace(tag), needle) {
			return true
		}
	}
	return false
}

func scenarioHasAnyTag(s *larktesting.Scenario, tags []string) bool {
	if s == nil {
		return false
	}
	tagSet := make(map[string]bool, len(s.Tags))
	for _, t := range s.Tags {
		tagSet[t] = true
	}
	for _, t := range tags {
		if tagSet[t] {
			return true
		}
	}
	return false
}

func mustJSON(report *larktesting.TestReport) []byte {
	b, err := report.ToJSON()
	if err != nil {
		// Should never happen; report is a simple struct.
		return []byte("{}")
	}
	return b
}

func writeFile(path string, contents []byte) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("empty output path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
