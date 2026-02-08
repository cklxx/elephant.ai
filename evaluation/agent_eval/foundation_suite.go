package agent_eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

const defaultFoundationSuitePath = "evaluation/agent_eval/datasets/foundation_eval_suite.yaml"

// FoundationSuiteOptions controls suite-level execution for offline foundation eval.
type FoundationSuiteOptions struct {
	OutputDir    string
	SuitePath    string
	ReportFormat string
}

// DefaultFoundationSuiteOptions returns default options for suite evaluation.
func DefaultFoundationSuiteOptions() *FoundationSuiteOptions {
	return &FoundationSuiteOptions{
		OutputDir:    "./evaluation_results/foundation-suite",
		SuitePath:    defaultFoundationSuitePath,
		ReportFormat: "markdown",
	}
}

// FoundationSuiteSet is the YAML schema for grouped foundation collections.
type FoundationSuiteSet struct {
	Version     string                      `yaml:"version"`
	Name        string                      `yaml:"name"`
	Description string                      `yaml:"description,omitempty"`
	Collections []FoundationSuiteCollection `yaml:"collections"`
}

// FoundationSuiteCollection defines one collection run in a suite.
type FoundationSuiteCollection struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Dimension   string `yaml:"dimension,omitempty"`
	Description string `yaml:"description,omitempty"`
	CasesPath   string `yaml:"cases_path"`
	Mode        string `yaml:"mode,omitempty"`
	Preset      string `yaml:"preset,omitempty"`
	Toolset     string `yaml:"toolset,omitempty"`
	TopK        int    `yaml:"top_k,omitempty"`
}

// FoundationSuiteResult is the aggregate output for a suite run.
type FoundationSuiteResult struct {
	RunID                string                            `json:"run_id"`
	GeneratedAt          time.Time                         `json:"generated_at"`
	SuitePath            string                            `json:"suite_path"`
	SuiteName            string                            `json:"suite_name"`
	TotalCollections     int                               `json:"total_collections"`
	PassedCollections    int                               `json:"passed_collections"`
	AverageOverallScore  float64                           `json:"average_overall_score"`
	AverageTop1HitRate   float64                           `json:"average_top1_hit_rate"`
	AverageTopKHitRate   float64                           `json:"average_topk_hit_rate"`
	TotalCases           int                               `json:"total_cases"`
	PassedCases          int                               `json:"passed_cases"`
	FailedCases          int                               `json:"failed_cases"`
	AvailabilityErrors   int                               `json:"availability_errors"`
	CollectionResults    []FoundationSuiteCollectionResult `json:"collection_results"`
	Recommendations      []string                          `json:"recommendations"`
	ReportArtifacts      []EvaluationArtifact              `json:"report_artifacts,omitempty"`
	FailedCollectionRuns int                               `json:"failed_collection_runs,omitempty"`
}

// FoundationSuiteCollectionResult captures one collection execution and summary.
type FoundationSuiteCollectionResult struct {
	ID                   string                      `json:"id"`
	Name                 string                      `json:"name"`
	Dimension            string                      `json:"dimension,omitempty"`
	CasesPath            string                      `json:"cases_path"`
	Mode                 string                      `json:"mode"`
	Preset               string                      `json:"preset"`
	Toolset              string                      `json:"toolset"`
	TopK                 int                         `json:"top_k"`
	OverallScore         float64                     `json:"overall_score"`
	Top1HitRate          float64                     `json:"top1_hit_rate"`
	TopKHitRate          float64                     `json:"topk_hit_rate"`
	TotalCases           int                         `json:"total_cases"`
	PassedCases          int                         `json:"passed_cases"`
	FailedCases          int                         `json:"failed_cases"`
	AvailabilityErrors   int                         `json:"availability_errors"`
	FailureTypeBreakdown map[string]int              `json:"failure_type_breakdown,omitempty"`
	Summary              *FoundationEvaluationResult `json:"summary,omitempty"`
}

// LoadFoundationSuiteSet loads and validates a suite YAML.
func LoadFoundationSuiteSet(path string) (*FoundationSuiteSet, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("foundation suite path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read foundation suite: %w", err)
	}

	var set FoundationSuiteSet
	if err := yaml.Unmarshal(data, &set); err != nil {
		return nil, fmt.Errorf("decode foundation suite: %w", err)
	}
	if strings.TrimSpace(set.Version) == "" {
		return nil, fmt.Errorf("foundation suite version is required")
	}
	if strings.TrimSpace(set.Name) == "" {
		return nil, fmt.Errorf("foundation suite name is required")
	}
	if len(set.Collections) == 0 {
		return nil, fmt.Errorf("foundation suite must contain collections")
	}

	seenIDs := make(map[string]struct{}, len(set.Collections))
	defaultEvalOpts := DefaultFoundationEvaluationOptions()
	for idx := range set.Collections {
		collection := &set.Collections[idx]
		collection.ID = strings.TrimSpace(collection.ID)
		collection.Name = strings.TrimSpace(collection.Name)
		collection.CasesPath = strings.TrimSpace(collection.CasesPath)
		collection.Mode = strings.TrimSpace(collection.Mode)
		collection.Preset = strings.TrimSpace(collection.Preset)
		collection.Toolset = strings.TrimSpace(collection.Toolset)
		collection.Dimension = strings.TrimSpace(collection.Dimension)
		if collection.ID == "" {
			return nil, fmt.Errorf("collection[%d] id is required", idx)
		}
		if _, exists := seenIDs[collection.ID]; exists {
			return nil, fmt.Errorf("duplicate collection id: %s", collection.ID)
		}
		seenIDs[collection.ID] = struct{}{}
		if collection.Name == "" {
			return nil, fmt.Errorf("collection %s name is required", collection.ID)
		}
		if collection.CasesPath == "" {
			return nil, fmt.Errorf("collection %s cases_path is required", collection.ID)
		}

		if collection.Mode == "" {
			collection.Mode = defaultEvalOpts.Mode
		}
		if collection.Preset == "" {
			collection.Preset = defaultEvalOpts.Preset
		}
		if collection.Toolset == "" {
			collection.Toolset = defaultEvalOpts.Toolset
		}
		if collection.TopK <= 0 {
			collection.TopK = defaultEvalOpts.TopK
		}

		if _, err := LoadFoundationCaseSet(collection.CasesPath); err != nil {
			return nil, fmt.Errorf("collection %s invalid cases set: %w", collection.ID, err)
		}
	}

	return &set, nil
}

// RunFoundationEvaluationSuite executes a full suite and writes aggregate artifacts.
func RunFoundationEvaluationSuite(ctx context.Context, options *FoundationSuiteOptions) (*FoundationSuiteResult, error) {
	if options == nil {
		options = DefaultFoundationSuiteOptions()
	}
	opts := *options
	if strings.TrimSpace(opts.OutputDir) == "" {
		opts.OutputDir = DefaultFoundationSuiteOptions().OutputDir
	}
	if strings.TrimSpace(opts.SuitePath) == "" {
		opts.SuitePath = defaultFoundationSuitePath
	}
	if strings.TrimSpace(opts.ReportFormat) == "" {
		opts.ReportFormat = DefaultFoundationSuiteOptions().ReportFormat
	}

	suiteSet, err := LoadFoundationSuiteSet(opts.SuitePath)
	if err != nil {
		return nil, err
	}

	cleanedOutputDir, err := sanitizeOutputPath(defaultOutputBaseDir, opts.OutputDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cleanedOutputDir, 0755); err != nil {
		return nil, fmt.Errorf("create foundation suite output dir: %w", err)
	}

	result := &FoundationSuiteResult{
		RunID:             fmt.Sprintf("foundation-suite-%s", time.Now().UTC().Format("20060102-150405")),
		GeneratedAt:       time.Now().UTC(),
		SuitePath:         opts.SuitePath,
		SuiteName:         suiteSet.Name,
		TotalCollections:  len(suiteSet.Collections),
		CollectionResults: make([]FoundationSuiteCollectionResult, 0, len(suiteSet.Collections)),
	}

	for index, collection := range suiteSet.Collections {
		collectionDir := filepath.Join(cleanedOutputDir, fmt.Sprintf("%02d-%s", index+1, sanitizeCollectionID(collection.ID)))
		evalResult, runErr := RunFoundationEvaluation(ctx, &FoundationEvaluationOptions{
			OutputDir:    collectionDir,
			Mode:         collection.Mode,
			Preset:       collection.Preset,
			Toolset:      collection.Toolset,
			CasesPath:    collection.CasesPath,
			TopK:         collection.TopK,
			ReportFormat: opts.ReportFormat,
		})
		if runErr != nil {
			return nil, fmt.Errorf("run foundation collection %s: %w", collection.ID, runErr)
		}

		failureBreakdown := map[string]int{}
		availabilityErrors := 0
		for _, caseResult := range evalResult.Implicit.CaseResults {
			if strings.TrimSpace(caseResult.FailureType) == "" {
				continue
			}
			failureBreakdown[caseResult.FailureType]++
			if caseResult.FailureType == "availability_error" {
				availabilityErrors++
			}
		}
		if len(failureBreakdown) == 0 {
			failureBreakdown = nil
		}

		collectionResult := FoundationSuiteCollectionResult{
			ID:                   collection.ID,
			Name:                 collection.Name,
			Dimension:            collection.Dimension,
			CasesPath:            collection.CasesPath,
			Mode:                 evalResult.Mode,
			Preset:               evalResult.Preset,
			Toolset:              evalResult.Toolset,
			TopK:                 evalResult.TopK,
			OverallScore:         evalResult.OverallScore,
			Top1HitRate:          evalResult.Implicit.Top1HitRate,
			TopKHitRate:          evalResult.Implicit.TopKHitRate,
			TotalCases:           evalResult.Implicit.TotalCases,
			PassedCases:          evalResult.Implicit.PassedCases,
			FailedCases:          evalResult.Implicit.FailedCases,
			AvailabilityErrors:   availabilityErrors,
			FailureTypeBreakdown: failureBreakdown,
			Summary:              evalResult,
		}
		result.CollectionResults = append(result.CollectionResults, collectionResult)

		result.AverageOverallScore += evalResult.OverallScore
		result.AverageTop1HitRate += evalResult.Implicit.Top1HitRate
		result.AverageTopKHitRate += evalResult.Implicit.TopKHitRate
		result.TotalCases += evalResult.Implicit.TotalCases
		result.PassedCases += evalResult.Implicit.PassedCases
		result.FailedCases += evalResult.Implicit.FailedCases
		result.AvailabilityErrors += availabilityErrors
		if evalResult.Implicit.FailedCases == 0 {
			result.PassedCollections++
		}
	}

	if result.TotalCollections > 0 {
		total := float64(result.TotalCollections)
		result.AverageOverallScore = round1(result.AverageOverallScore / total)
		result.AverageTop1HitRate = round3(result.AverageTop1HitRate / total)
		result.AverageTopKHitRate = round3(result.AverageTopKHitRate / total)
	}

	result.Recommendations = buildFoundationSuiteRecommendations(result)

	artifacts, err := writeFoundationSuiteArtifacts(result, cleanedOutputDir, opts.ReportFormat)
	if err != nil {
		return nil, err
	}
	result.ReportArtifacts = artifacts

	return result, nil
}

func buildFoundationSuiteRecommendations(result *FoundationSuiteResult) []string {
	recs := make([]string, 0, 8)
	if result.FailedCases > 0 {
		recs = append(recs, fmt.Sprintf("Implicit routing still has %d failed cases; prioritize collections with lowest Top-K hit first.", result.FailedCases))
	}
	if result.AvailabilityErrors > 0 {
		recs = append(recs, fmt.Sprintf("Availability has %d failures across suite; close registration/preset parity gaps before semantic tuning.", result.AvailabilityErrors))
	}
	if result.AverageTop1HitRate < 0.7 {
		recs = append(recs, "Top-1 precision is below 70%; tighten overlapping tool semantics and add disambiguation terms.")
	}
	if result.AverageTopKHitRate < 0.9 {
		recs = append(recs, "Top-K hit rate is below 90%; add intent aliases and targeted boosts for weak tool categories.")
	}
	if len(recs) == 0 {
		recs = append(recs, "Suite baseline is stable; next target is raising Top-1 precision for long, multi-constraint intents.")
	}
	return uniqueNonEmptyStrings(recs)
}

func writeFoundationSuiteArtifacts(result *FoundationSuiteResult, outputDir, format string) ([]EvaluationArtifact, error) {
	artifacts := make([]EvaluationArtifact, 0, 2)

	jsonPath := filepath.Join(outputDir, fmt.Sprintf("foundation_suite_result_%s.json", result.RunID))
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal foundation suite result: %w", err)
	}
	if err := os.WriteFile(jsonPath, data, 0644); err != nil {
		return nil, fmt.Errorf("write foundation suite json: %w", err)
	}
	artifacts = append(artifacts, EvaluationArtifact{
		Type:   "foundation_suite_result",
		Format: "json",
		Name:   filepath.Base(jsonPath),
		Path:   jsonPath,
	})

	if strings.EqualFold(strings.TrimSpace(format), "json") {
		return artifacts, nil
	}

	mdPath := filepath.Join(outputDir, fmt.Sprintf("foundation_suite_report_%s.md", result.RunID))
	if err := os.WriteFile(mdPath, []byte(buildFoundationSuiteMarkdownReport(result)), 0644); err != nil {
		return nil, fmt.Errorf("write foundation suite markdown: %w", err)
	}
	artifacts = append(artifacts, EvaluationArtifact{
		Type:   "foundation_suite_report",
		Format: "markdown",
		Name:   filepath.Base(mdPath),
		Path:   mdPath,
	})

	return artifacts, nil
}

func buildFoundationSuiteMarkdownReport(result *FoundationSuiteResult) string {
	var b strings.Builder

	b.WriteString("# Foundation Suite Evaluation Report\n\n")
	b.WriteString(fmt.Sprintf("- Run ID: `%s`\n", result.RunID))
	b.WriteString(fmt.Sprintf("- Generated At (UTC): `%s`\n", result.GeneratedAt.Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("- Suite: `%s`\n", result.SuiteName))
	b.WriteString(fmt.Sprintf("- Suite Path: `%s`\n\n", result.SuitePath))

	b.WriteString("## Aggregate Summary\n\n")
	b.WriteString("| Metric | Value |\n")
	b.WriteString("|---|---:|\n")
	b.WriteString(fmt.Sprintf("| Collections | %d |\n", result.TotalCollections))
	b.WriteString(fmt.Sprintf("| Collections Passed (0 failed cases) | %d |\n", result.PassedCollections))
	b.WriteString(fmt.Sprintf("| Average Overall Score | %.1f |\n", result.AverageOverallScore))
	b.WriteString(fmt.Sprintf("| Average Top-1 Hit Rate | %.1f%% |\n", result.AverageTop1HitRate*100))
	b.WriteString(fmt.Sprintf("| Average Top-K Hit Rate | %.1f%% |\n", result.AverageTopKHitRate*100))
	b.WriteString(fmt.Sprintf("| Total Cases | %d |\n", result.TotalCases))
	b.WriteString(fmt.Sprintf("| Passed Cases | %d |\n", result.PassedCases))
	b.WriteString(fmt.Sprintf("| Failed Cases | %d |\n", result.FailedCases))
	b.WriteString(fmt.Sprintf("| Availability Errors | %d |\n\n", result.AvailabilityErrors))

	if len(result.CollectionResults) > 0 {
		rows := append([]FoundationSuiteCollectionResult(nil), result.CollectionResults...)
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].TopKHitRate == rows[j].TopKHitRate {
				return rows[i].ID < rows[j].ID
			}
			return rows[i].TopKHitRate < rows[j].TopKHitRate
		})
		b.WriteString("## Collection Breakdown\n\n")
		b.WriteString("| Collection | Dimension | Mode/Preset/Toolset | Top-K | Cases | Top-1 | Top-K | Failed | Availability |\n")
		b.WriteString("|---|---|---|---:|---:|---:|---:|---:|---:|\n")
		for _, row := range rows {
			dimension := row.Dimension
			if strings.TrimSpace(dimension) == "" {
				dimension = "-"
			}
			b.WriteString(fmt.Sprintf(
				"| `%s` | `%s` | `%s / %s / %s` | %d | %d | %.1f%% | %.1f%% | %d | %d |\n",
				row.ID,
				dimension,
				row.Mode,
				row.Preset,
				row.Toolset,
				row.TopK,
				row.TotalCases,
				row.Top1HitRate*100,
				row.TopKHitRate*100,
				row.FailedCases,
				row.AvailabilityErrors,
			))
		}
		b.WriteString("\n")

		b.WriteString("## Worst Failed Cases by Collection\n\n")
		for _, row := range rows {
			if row.Summary == nil || row.FailedCases == 0 {
				continue
			}
			b.WriteString(fmt.Sprintf("### `%s` (%s)\n\n", row.ID, row.Name))
			b.WriteString("| Case | Failure Type | Expected | Top Matches | Reason |\n")
			b.WriteString("|---|---|---|---|---|\n")
			for _, caseResult := range row.Summary.Implicit.CaseResults {
				if caseResult.Passed {
					continue
				}
				failureType := caseResult.FailureType
				if strings.TrimSpace(failureType) == "" {
					failureType = "ranking"
				}
				b.WriteString(fmt.Sprintf(
					"| `%s` | `%s` | `%s` | %s | %s |\n",
					caseResult.ID,
					failureType,
					strings.Join(caseResult.ExpectedTools, ", "),
					escapeTable(formatTopMatches(caseResult.TopMatches)),
					escapeTable(caseResult.Reason),
				))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("## Recommendations\n\n")
	for _, rec := range result.Recommendations {
		b.WriteString(fmt.Sprintf("- %s\n", rec))
	}
	b.WriteString("\n")

	return b.String()
}

func sanitizeCollectionID(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "collection"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "collection"
	}
	return out
}
