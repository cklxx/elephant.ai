package agent_eval

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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
	RunID                  string                            `json:"run_id"`
	GeneratedAt            time.Time                         `json:"generated_at"`
	StartedAt              time.Time                         `json:"started_at"`
	CompletedAt            time.Time                         `json:"completed_at"`
	TotalDurationMs        int64                             `json:"total_duration_ms"`
	ThroughputCasesPerSec  float64                           `json:"throughput_cases_per_sec"`
	SuitePath              string                            `json:"suite_path"`
	SuiteName              string                            `json:"suite_name"`
	TotalCollections       int                               `json:"total_collections"`
	PassedCollections      int                               `json:"passed_collections"`
	CollectionPassRatio    string                            `json:"collection_pass_ratio"`
	AverageOverallScore    float64                           `json:"average_overall_score"`
	AveragePassAt1Rate     float64                           `json:"average_pass_at_1_rate"`
	AveragePassAt5Rate     float64                           `json:"average_pass_at_5_rate"`
	AverageTop1HitRate     float64                           `json:"average_top1_hit_rate"`
	AverageTopKHitRate     float64                           `json:"average_topk_hit_rate"`
	TotalCases             int                               `json:"total_cases"`
	ApplicableCases        int                               `json:"applicable_cases"`
	NotApplicableCases     int                               `json:"not_applicable_cases"`
	PassedCases            int                               `json:"passed_cases"`
	FailedCases            int                               `json:"failed_cases"`
	PassAt1Cases           int                               `json:"pass_at_1_cases"`
	PassAt5Cases           int                               `json:"pass_at_5_cases"`
	CasePassRatio          string                            `json:"case_pass_ratio"`
	DeliverableCaseRatio   string                            `json:"deliverable_case_ratio"`
	DeliverableGoodRatio   string                            `json:"deliverable_good_ratio"`
	DeliverableBadRatio    string                            `json:"deliverable_bad_ratio"`
	AvailabilityErrors     int                               `json:"availability_errors"`
	DeliverableCases       int                               `json:"deliverable_cases"`
	DeliverableGoodCases   int                               `json:"deliverable_good_cases"`
	DeliverableBadCases    int                               `json:"deliverable_bad_cases"`
	CaseLatencyP50Ms       float64                           `json:"case_latency_p50_ms"`
	CaseLatencyP95Ms       float64                           `json:"case_latency_p95_ms"`
	CaseLatencyP99Ms       float64                           `json:"case_latency_p99_ms"`
	CollectionLatencyP50Ms float64                           `json:"collection_latency_p50_ms"`
	CollectionLatencyP95Ms float64                           `json:"collection_latency_p95_ms"`
	CollectionLatencyP99Ms float64                           `json:"collection_latency_p99_ms"`
	CollectionResults      []FoundationSuiteCollectionResult `json:"collection_results"`
	Recommendations        []string                          `json:"recommendations"`
	ReportArtifacts        []EvaluationArtifact              `json:"report_artifacts,omitempty"`
	FailedCollectionRuns   int                               `json:"failed_collection_runs,omitempty"`
}

// FoundationSuiteCollectionResult captures one collection execution and summary.
type FoundationSuiteCollectionResult struct {
	ID                    string                      `json:"id"`
	Name                  string                      `json:"name"`
	Dimension             string                      `json:"dimension,omitempty"`
	CasesPath             string                      `json:"cases_path"`
	Mode                  string                      `json:"mode"`
	Preset                string                      `json:"preset"`
	Toolset               string                      `json:"toolset"`
	TopK                  int                         `json:"top_k"`
	OverallScore          float64                     `json:"overall_score"`
	PassAt1Cases          int                         `json:"pass_at_1_cases"`
	PassAt5Cases          int                         `json:"pass_at_5_cases"`
	PassAt1Rate           float64                     `json:"pass_at_1_rate"`
	PassAt5Rate           float64                     `json:"pass_at_5_rate"`
	Top1HitRate           float64                     `json:"top1_hit_rate"`
	TopKHitRate           float64                     `json:"topk_hit_rate"`
	TotalCases            int                         `json:"total_cases"`
	ApplicableCases       int                         `json:"applicable_cases"`
	NotApplicableCases    int                         `json:"not_applicable_cases"`
	PassedCases           int                         `json:"passed_cases"`
	FailedCases           int                         `json:"failed_cases"`
	CasePassRatio         string                      `json:"case_pass_ratio"`
	DeliverableCaseRatio  string                      `json:"deliverable_case_ratio"`
	DeliverableGoodRatio  string                      `json:"deliverable_good_ratio"`
	DeliverableBadRatio   string                      `json:"deliverable_bad_ratio"`
	AvailabilityErrors    int                         `json:"availability_errors"`
	DeliverableCases      int                         `json:"deliverable_cases"`
	DeliverableGoodCases  int                         `json:"deliverable_good_cases"`
	DeliverableBadCases   int                         `json:"deliverable_bad_cases"`
	CollectionDurationMs  int64                       `json:"collection_duration_ms"`
	ThroughputCasesPerSec float64                     `json:"throughput_cases_per_sec"`
	CaseLatencyP50Ms      float64                     `json:"case_latency_p50_ms"`
	CaseLatencyP95Ms      float64                     `json:"case_latency_p95_ms"`
	CaseLatencyP99Ms      float64                     `json:"case_latency_p99_ms"`
	FailureTypeBreakdown  map[string]int              `json:"failure_type_breakdown,omitempty"`
	Summary               *FoundationEvaluationResult `json:"summary,omitempty"`
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
	startedAt := time.Now().UTC()
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
		StartedAt:         startedAt,
		SuitePath:         opts.SuitePath,
		SuiteName:         suiteSet.Name,
		TotalCollections:  len(suiteSet.Collections),
		CollectionResults: make([]FoundationSuiteCollectionResult, 0, len(suiteSet.Collections)),
	}
	collectionLatencies := make([]float64, 0, len(suiteSet.Collections))
	caseLatencies := make([]float64, 0, 512)

	for index, collection := range suiteSet.Collections {
		collectionStart := time.Now()
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
		deliverableCases := 0
		deliverableGoodCases := 0
		deliverableBadCases := 0
		for _, caseResult := range evalResult.Implicit.CaseResults {
			if strings.TrimSpace(caseResult.FailureType) == "" {
				if caseResult.DeliverableCheck != nil && caseResult.DeliverableCheck.Applicable {
					deliverableCases++
					if strings.EqualFold(caseResult.DeliverableCheck.Status, "good") {
						deliverableGoodCases++
					} else if strings.EqualFold(caseResult.DeliverableCheck.Status, "bad") {
						deliverableBadCases++
					}
				}
				continue
			}
			failureBreakdown[caseResult.FailureType]++
			if caseResult.FailureType == "availability_error" {
				availabilityErrors++
			}
			if caseResult.DeliverableCheck != nil && caseResult.DeliverableCheck.Applicable {
				deliverableCases++
				if strings.EqualFold(caseResult.DeliverableCheck.Status, "good") {
					deliverableGoodCases++
				} else if strings.EqualFold(caseResult.DeliverableCheck.Status, "bad") {
					deliverableBadCases++
				}
			}
		}
		if len(failureBreakdown) == 0 {
			failureBreakdown = nil
		}

		collectionDurationMs := float64(time.Since(collectionStart).Microseconds()) / 1000.0
		collectionLatencies = append(collectionLatencies, collectionDurationMs)
		collectionResult := FoundationSuiteCollectionResult{
			ID:                    collection.ID,
			Name:                  collection.Name,
			Dimension:             collection.Dimension,
			CasesPath:             collection.CasesPath,
			Mode:                  evalResult.Mode,
			Preset:                evalResult.Preset,
			Toolset:               evalResult.Toolset,
			TopK:                  evalResult.TopK,
			OverallScore:          evalResult.OverallScore,
			PassAt1Cases:          evalResult.Implicit.PassAt1Cases,
			PassAt5Cases:          evalResult.Implicit.PassAt5Cases,
			PassAt1Rate:           evalResult.Implicit.PassAt1Rate,
			PassAt5Rate:           evalResult.Implicit.PassAt5Rate,
			Top1HitRate:           evalResult.Implicit.Top1HitRate,
			TopKHitRate:           evalResult.Implicit.TopKHitRate,
			TotalCases:            evalResult.Implicit.TotalCases,
			ApplicableCases:       evalResult.Implicit.ApplicableCases,
			NotApplicableCases:    evalResult.Implicit.NotApplicableCases,
			PassedCases:           evalResult.Implicit.PassedCases,
			FailedCases:           evalResult.Implicit.FailedCases,
			CasePassRatio:         fmt.Sprintf("%d/%d", evalResult.Implicit.PassedCases, evalResult.Implicit.ApplicableCases),
			DeliverableCaseRatio:  fmt.Sprintf("%d/%d", deliverableCases, evalResult.Implicit.TotalCases),
			DeliverableGoodRatio:  fmt.Sprintf("%d/%d", deliverableGoodCases, deliverableCases),
			DeliverableBadRatio:   fmt.Sprintf("%d/%d", deliverableBadCases, deliverableCases),
			AvailabilityErrors:    availabilityErrors,
			DeliverableCases:      deliverableCases,
			DeliverableGoodCases:  deliverableGoodCases,
			DeliverableBadCases:   deliverableBadCases,
			CollectionDurationMs:  int64(math.Round(collectionDurationMs)),
			ThroughputCasesPerSec: round3(float64(evalResult.Implicit.TotalCases) / math.Max(collectionDurationMs/1000.0, 1e-9)),
			CaseLatencyP50Ms:      evalResult.Implicit.CaseLatencyP50Ms,
			CaseLatencyP95Ms:      evalResult.Implicit.CaseLatencyP95Ms,
			CaseLatencyP99Ms:      evalResult.Implicit.CaseLatencyP99Ms,
			FailureTypeBreakdown:  failureBreakdown,
			Summary:               evalResult,
		}
		result.CollectionResults = append(result.CollectionResults, collectionResult)
		for _, caseResult := range evalResult.Implicit.CaseResults {
			caseLatencies = append(caseLatencies, caseResult.RoutingLatencyMs)
		}

		result.AverageOverallScore += evalResult.OverallScore
		result.AveragePassAt1Rate += evalResult.Implicit.PassAt1Rate
		result.AveragePassAt5Rate += evalResult.Implicit.PassAt5Rate
		result.AverageTop1HitRate += evalResult.Implicit.Top1HitRate
		result.AverageTopKHitRate += evalResult.Implicit.TopKHitRate
		result.TotalCases += evalResult.Implicit.TotalCases
		result.ApplicableCases += evalResult.Implicit.ApplicableCases
		result.NotApplicableCases += evalResult.Implicit.NotApplicableCases
		result.PassedCases += evalResult.Implicit.PassedCases
		result.FailedCases += evalResult.Implicit.FailedCases
		result.PassAt1Cases += evalResult.Implicit.PassAt1Cases
		result.PassAt5Cases += evalResult.Implicit.PassAt5Cases
		result.AvailabilityErrors += availabilityErrors
		result.DeliverableCases += deliverableCases
		result.DeliverableGoodCases += deliverableGoodCases
		result.DeliverableBadCases += deliverableBadCases
		if evalResult.Implicit.FailedCases == 0 {
			result.PassedCollections++
		}
	}

	if result.TotalCollections > 0 {
		total := float64(result.TotalCollections)
		result.AverageOverallScore = round1(result.AverageOverallScore / total)
		result.AveragePassAt1Rate = round3(result.AveragePassAt1Rate / total)
		result.AveragePassAt5Rate = round3(result.AveragePassAt5Rate / total)
		result.AverageTop1HitRate = round3(result.AverageTop1HitRate / total)
		result.AverageTopKHitRate = round3(result.AverageTopKHitRate / total)
	}
	result.CollectionPassRatio = fmt.Sprintf("%d/%d", result.PassedCollections, result.TotalCollections)
	result.CasePassRatio = fmt.Sprintf("%d/%d", result.PassedCases, result.ApplicableCases)
	result.DeliverableCaseRatio = fmt.Sprintf("%d/%d", result.DeliverableCases, result.TotalCases)
	result.DeliverableGoodRatio = fmt.Sprintf("%d/%d", result.DeliverableGoodCases, result.DeliverableCases)
	result.DeliverableBadRatio = fmt.Sprintf("%d/%d", result.DeliverableBadCases, result.DeliverableCases)
	result.CaseLatencyP50Ms = round3(percentileFloat(caseLatencies, 50))
	result.CaseLatencyP95Ms = round3(percentileFloat(caseLatencies, 95))
	result.CaseLatencyP99Ms = round3(percentileFloat(caseLatencies, 99))
	result.CollectionLatencyP50Ms = round3(percentileFloat(collectionLatencies, 50))
	result.CollectionLatencyP95Ms = round3(percentileFloat(collectionLatencies, 95))
	result.CollectionLatencyP99Ms = round3(percentileFloat(collectionLatencies, 99))
	result.CompletedAt = time.Now().UTC()
	result.TotalDurationMs = int64(math.Round(float64(result.CompletedAt.Sub(result.StartedAt).Microseconds()) / 1000.0))
	result.ThroughputCasesPerSec = round3(float64(result.TotalCases) / math.Max(float64(result.TotalDurationMs)/1000.0, 1e-9))

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
		recs = append(recs, fmt.Sprintf("Implicit routing still has %d failed cases; prioritize collections with lowest pass@5 first.", result.FailedCases))
	}
	if result.AvailabilityErrors > 0 {
		recs = append(recs, fmt.Sprintf("Availability has %d failures across suite; close registration/preset parity gaps before semantic tuning.", result.AvailabilityErrors))
	}
	if result.AveragePassAt1Rate < 0.7 {
		recs = append(recs, "pass@1 precision is below 70%; tighten overlapping tool semantics and add disambiguation terms.")
	}
	if result.AveragePassAt5Rate < 0.9 {
		recs = append(recs, "pass@5 is below 90%; add intent aliases and targeted boosts for weak tool categories.")
	}
	if len(recs) == 0 {
		recs = append(recs, "Suite baseline is stable; next target is raising pass@1 precision for long, multi-constraint intents.")
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
	b.WriteString(fmt.Sprintf("- Started At (UTC): `%s`\n", result.StartedAt.Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("- Completed At (UTC): `%s`\n", result.CompletedAt.Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("- Suite: `%s`\n", result.SuiteName))
	b.WriteString(fmt.Sprintf("- Suite Path: `%s`\n\n", result.SuitePath))

	b.WriteString("## Aggregate Summary\n\n")
	b.WriteString("| Metric | Value |\n")
	b.WriteString("|---|---:|\n")
	b.WriteString(fmt.Sprintf("| Collections | %d |\n", result.TotalCollections))
	b.WriteString(fmt.Sprintf("| Collections Passed (0 failed cases) | %s |\n", result.CollectionPassRatio))
	b.WriteString(fmt.Sprintf("| Average Overall Score | %.1f |\n", result.AverageOverallScore))
	b.WriteString(fmt.Sprintf("| Average pass@1 | %.1f%% |\n", result.AveragePassAt1Rate*100))
	b.WriteString(fmt.Sprintf("| Average pass@5 | %.1f%% |\n", result.AveragePassAt5Rate*100))
	b.WriteString(fmt.Sprintf("| Average Top-1 Hit Rate (legacy) | %.1f%% |\n", result.AverageTop1HitRate*100))
	b.WriteString(fmt.Sprintf("| Average Top-K Hit Rate (legacy) | %.1f%% |\n", result.AverageTopKHitRate*100))
	b.WriteString(fmt.Sprintf("| Total Cases | %d |\n", result.TotalCases))
	b.WriteString(fmt.Sprintf("| Applicable Cases | %d |\n", result.ApplicableCases))
	b.WriteString(fmt.Sprintf("| N/A Cases | %d |\n", result.NotApplicableCases))
	b.WriteString(fmt.Sprintf("| Passed Cases | %s |\n", result.CasePassRatio))
	b.WriteString(fmt.Sprintf("| pass@1 Cases | %d/%d |\n", result.PassAt1Cases, result.ApplicableCases))
	b.WriteString(fmt.Sprintf("| pass@5 Cases | %d/%d |\n", result.PassAt5Cases, result.ApplicableCases))
	b.WriteString(fmt.Sprintf("| Deliverable Cases | %s |\n", result.DeliverableCaseRatio))
	b.WriteString(fmt.Sprintf("| Deliverable Good | %s |\n", result.DeliverableGoodRatio))
	b.WriteString(fmt.Sprintf("| Deliverable Bad | %s |\n", result.DeliverableBadRatio))
	b.WriteString(fmt.Sprintf("| Failed Cases | %d |\n", result.FailedCases))
	b.WriteString(fmt.Sprintf("| Availability Errors | %d |\n\n", result.AvailabilityErrors))
	b.WriteString(fmt.Sprintf("| Total Duration (ms) | %d |\n", result.TotalDurationMs))
	b.WriteString(fmt.Sprintf("| Throughput (cases/s) | %.2f |\n", result.ThroughputCasesPerSec))
	b.WriteString(fmt.Sprintf("| Case Latency p50/p95/p99 (ms) | %.3f / %.3f / %.3f |\n", result.CaseLatencyP50Ms, result.CaseLatencyP95Ms, result.CaseLatencyP99Ms))
	b.WriteString(fmt.Sprintf("| Collection Latency p50/p95/p99 (ms) | %.3f / %.3f / %.3f |\n\n", result.CollectionLatencyP50Ms, result.CollectionLatencyP95Ms, result.CollectionLatencyP99Ms))

	if len(result.CollectionResults) > 0 {
		rows := append([]FoundationSuiteCollectionResult(nil), result.CollectionResults...)
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].PassAt5Rate == rows[j].PassAt5Rate {
				return rows[i].ID < rows[j].ID
			}
			return rows[i].PassAt5Rate < rows[j].PassAt5Rate
		})
		b.WriteString("## Collection Breakdown\n\n")
		b.WriteString("| Collection | Dimension | Mode/Preset/Toolset | Top-K | Cases (pass/applicable) | Deliverable (x/x) | Deliverable Good | Deliverable Bad | N/A | pass@1 (x/x) | pass@5 (x/x) | Failed | Availability | Duration(ms) | Cases/s | Case p95(ms) |\n")
		b.WriteString("|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n")
		for _, row := range rows {
			dimension := row.Dimension
			if strings.TrimSpace(dimension) == "" {
				dimension = "-"
			}
				b.WriteString(fmt.Sprintf(
					"| `%s` | `%s` | `%s / %s / %s` | %d | %s | %s | %s | %s | %d | %d/%d (%.1f%%) | %d/%d (%.1f%%) | %d | %d | %d | %.2f | %.3f |\n",
				row.ID,
				dimension,
				row.Mode,
				row.Preset,
				row.Toolset,
				row.TopK,
				row.CasePassRatio,
				row.DeliverableCaseRatio,
				row.DeliverableGoodRatio,
				row.DeliverableBadRatio,
				row.NotApplicableCases,
				row.PassAt1Cases,
				row.ApplicableCases,
				row.PassAt1Rate*100,
				row.PassAt5Cases,
				row.ApplicableCases,
				row.PassAt5Rate*100,
				row.FailedCases,
				row.AvailabilityErrors,
				row.CollectionDurationMs,
				row.ThroughputCasesPerSec,
				row.CaseLatencyP95Ms,
			))
		}
		b.WriteString("\n")

		b.WriteString(buildDeliverableSamplingSection(rows))

		b.WriteString("## Top1 Failure Leaderboard\n\n")
		b.WriteString("| Collection | Case | Expected | Rank | Top1 Match | Reason |\n")
		b.WriteString("|---|---|---|---:|---|---|\n")
		type top1Miss struct {
			CollectionID string
			CaseID       string
			Expected     []string
			HitRank      int
			Top1         string
			Reason       string
		}
		misses := make([]top1Miss, 0, 128)
		for _, row := range rows {
			if row.Summary == nil {
				continue
			}
			for _, caseResult := range row.Summary.Implicit.CaseResults {
				if caseResult.HitRank <= 1 {
					continue
				}
				top1 := "-"
				if len(caseResult.TopMatches) > 0 {
					top1 = fmt.Sprintf("%s(%.2f)", caseResult.TopMatches[0].Name, caseResult.TopMatches[0].Score)
				}
				misses = append(misses, top1Miss{
					CollectionID: row.ID,
					CaseID:       caseResult.ID,
					Expected:     caseResult.ExpectedTools,
					HitRank:      caseResult.HitRank,
					Top1:         top1,
					Reason:       caseResult.Reason,
				})
			}
		}
		sort.Slice(misses, func(i, j int) bool {
			if misses[i].HitRank == misses[j].HitRank {
				if misses[i].CollectionID == misses[j].CollectionID {
					return misses[i].CaseID < misses[j].CaseID
				}
				return misses[i].CollectionID < misses[j].CollectionID
			}
			return misses[i].HitRank > misses[j].HitRank
		})
		limit := 30
		if len(misses) < limit {
			limit = len(misses)
		}
		if limit == 0 {
			b.WriteString("| - | - | - | - | - | no top1 failures |\n")
		} else {
			for i := 0; i < limit; i++ {
				m := misses[i]
				b.WriteString(fmt.Sprintf(
					"| `%s` | `%s` | `%s` | %d | %s | %s |\n",
					m.CollectionID,
					m.CaseID,
					strings.Join(m.Expected, ", "),
					m.HitRank,
					escapeTable(m.Top1),
					escapeTable(m.Reason),
				))
			}
		}
		b.WriteString("\n")

		b.WriteString(buildTop1ConflictClusterSection(rows))

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

func buildDeliverableSamplingSection(rows []FoundationSuiteCollectionResult) string {
	type sample struct {
		CollectionID string
		CaseID       string
		Expected     []string
		TopMatches   []FoundationToolMatch
		Coverage     float64
		Reason       string
		SignalCount  int
		Matched      int
	}

	var b strings.Builder
	deliverableTotal := 0
	goodSamples := make([]sample, 0, 64)
	badSamples := make([]sample, 0, 64)
	for _, row := range rows {
		if row.Summary == nil {
			continue
		}
		for _, caseResult := range row.Summary.Implicit.CaseResults {
			if caseResult.DeliverableCheck == nil || !caseResult.DeliverableCheck.Applicable {
				continue
			}
			deliverableTotal++
			item := sample{
				CollectionID: row.ID,
				CaseID:       caseResult.ID,
				Expected:     caseResult.ExpectedTools,
				TopMatches:   caseResult.TopMatches,
				Coverage:     caseResult.DeliverableCheck.ContractCoverage,
				Reason:       caseResult.DeliverableCheck.Reason,
				SignalCount:  caseResult.DeliverableCheck.SignalCount,
				Matched:      caseResult.DeliverableCheck.MatchedSignals,
			}
			if strings.EqualFold(caseResult.DeliverableCheck.Status, "good") {
				goodSamples = append(goodSamples, item)
			} else if strings.EqualFold(caseResult.DeliverableCheck.Status, "bad") {
				badSamples = append(badSamples, item)
			}
		}
	}

	b.WriteString("## Deliverable Sampling Check\n\n")
	b.WriteString(fmt.Sprintf("- Deliverable cases: `%d/%d`\n", deliverableTotal, totalCaseCountFromRows(rows)))
	b.WriteString(fmt.Sprintf("- Good checks: `%d/%d`\n", len(goodSamples), deliverableTotal))
	b.WriteString(fmt.Sprintf("- Bad checks: `%d/%d`\n\n", len(badSamples), deliverableTotal))

	sort.Slice(goodSamples, func(i, j int) bool {
		if goodSamples[i].Coverage == goodSamples[j].Coverage {
			if goodSamples[i].SignalCount == goodSamples[j].SignalCount {
				if goodSamples[i].CollectionID == goodSamples[j].CollectionID {
					return goodSamples[i].CaseID < goodSamples[j].CaseID
				}
				return goodSamples[i].CollectionID < goodSamples[j].CollectionID
			}
			return goodSamples[i].SignalCount > goodSamples[j].SignalCount
		}
		return goodSamples[i].Coverage > goodSamples[j].Coverage
	})
	sort.Slice(badSamples, func(i, j int) bool {
		if badSamples[i].Coverage == badSamples[j].Coverage {
			if badSamples[i].SignalCount == badSamples[j].SignalCount {
				if badSamples[i].CollectionID == badSamples[j].CollectionID {
					return badSamples[i].CaseID < badSamples[j].CaseID
				}
				return badSamples[i].CollectionID < badSamples[j].CollectionID
			}
			return badSamples[i].SignalCount > badSamples[j].SignalCount
		}
		return badSamples[i].Coverage < badSamples[j].Coverage
	})

	b.WriteString("### Good Case Samples\n\n")
	b.WriteString("| Collection | Case | Expected | Top Matches | Contract (matched/required) | Coverage | Why Good |\n")
	b.WriteString("|---|---|---|---|---:|---:|---|\n")
	goodLimit := 12
	if len(goodSamples) < goodLimit {
		goodLimit = len(goodSamples)
	}
	if goodLimit == 0 {
		b.WriteString("| - | - | - | - | - | - | no good deliverable sample |\n\n")
	} else {
		for i := 0; i < goodLimit; i++ {
			row := goodSamples[i]
			b.WriteString(fmt.Sprintf(
				"| `%s` | `%s` | `%s` | %s | %d/%d | %.1f%% | %s |\n",
				row.CollectionID,
				row.CaseID,
				strings.Join(row.Expected, ", "),
				escapeTable(formatTopMatches(row.TopMatches)),
				row.Matched,
				row.SignalCount,
				row.Coverage*100,
				escapeTable(row.Reason),
			))
		}
		b.WriteString("\n")
	}

	b.WriteString("### Bad Case Samples\n\n")
	b.WriteString("| Collection | Case | Expected | Top Matches | Contract (matched/required) | Coverage | Why Bad |\n")
	b.WriteString("|---|---|---|---|---:|---:|---|\n")
	badLimit := 12
	if len(badSamples) < badLimit {
		badLimit = len(badSamples)
	}
	if badLimit == 0 {
		b.WriteString("| - | - | - | - | - | - | no bad deliverable sample |\n\n")
	} else {
		for i := 0; i < badLimit; i++ {
			row := badSamples[i]
			b.WriteString(fmt.Sprintf(
				"| `%s` | `%s` | `%s` | %s | %d/%d | %.1f%% | %s |\n",
				row.CollectionID,
				row.CaseID,
				strings.Join(row.Expected, ", "),
				escapeTable(formatTopMatches(row.TopMatches)),
				row.Matched,
				row.SignalCount,
				row.Coverage*100,
				escapeTable(row.Reason),
			))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func totalCaseCountFromRows(rows []FoundationSuiteCollectionResult) int {
	total := 0
	for _, row := range rows {
		total += row.TotalCases
	}
	return total
}

type top1ConflictCluster struct {
	Expected   string
	Top1       string
	Count      int
	SampleCase string
	Collections map[string]struct{}
}

func buildTop1ConflictClusterSection(rows []FoundationSuiteCollectionResult) string {
	var b strings.Builder
	clusters := make(map[string]*top1ConflictCluster, 64)
	totalMisses := 0
	for _, row := range rows {
		if row.Summary == nil {
			continue
		}
		for _, caseResult := range row.Summary.Implicit.CaseResults {
			if caseResult.NotApplicable || caseResult.HitRank <= 1 {
				continue
			}
			if len(caseResult.ExpectedTools) == 0 {
				continue
			}
			top1 := "-"
			if len(caseResult.TopMatches) > 0 {
				top1 = caseResult.TopMatches[0].Name
			}
			expected := strings.Join(caseResult.ExpectedTools, "+")
			key := expected + " => " + top1
			cluster, ok := clusters[key]
			if !ok {
				cluster = &top1ConflictCluster{
					Expected:    expected,
					Top1:        top1,
					SampleCase:  caseResult.ID,
					Collections: map[string]struct{}{row.ID: {}},
				}
				clusters[key] = cluster
			}
			cluster.Count++
			cluster.Collections[row.ID] = struct{}{}
			totalMisses++
		}
	}

	b.WriteString("## Top1 Conflict Clusters (Systematic)\n\n")
	if totalMisses == 0 {
		b.WriteString("- Top1 misses: `0/0`\n")
		b.WriteString("- No conflict clusters detected.\n\n")
		return b.String()
	}

	list := make([]top1ConflictCluster, 0, len(clusters))
	for _, cluster := range clusters {
		list = append(list, *cluster)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Count == list[j].Count {
			if list[i].Expected == list[j].Expected {
				return list[i].Top1 < list[j].Top1
			}
			return list[i].Expected < list[j].Expected
		}
		return list[i].Count > list[j].Count
	})

	b.WriteString(fmt.Sprintf("- Top1 misses: `%d/%d`\n\n", totalMisses, totalCaseCountFromRows(rows)))
	b.WriteString("| Conflict Pair (expected => top1) | Misses (x/x) | Miss Share | Collections | Sample Case |\n")
	b.WriteString("|---|---:|---:|---:|---|\n")
	limit := 20
	if len(list) < limit {
		limit = len(list)
	}
	for i := 0; i < limit; i++ {
		cluster := list[i]
		b.WriteString(fmt.Sprintf(
			"| `%s => %s` | %d/%d | %.1f%% | %d | `%s` |\n",
			cluster.Expected,
			cluster.Top1,
			cluster.Count,
			totalMisses,
			(float64(cluster.Count)*100.0)/float64(totalMisses),
			len(cluster.Collections),
			cluster.SampleCase,
		))
	}
	b.WriteString("\n")

	b.WriteString("### Cluster-Oriented Optimization Actions\n\n")
	b.WriteString("| Cluster Family | Action |\n")
	b.WriteString("|---|---|\n")
	b.WriteString("| `memory_search => search_file/clarify` | Raise memory-intent boosts and add stronger penalties for file-search/clarify under memory/habit/persona tokens. |\n")
	b.WriteString("| `request_user => clarify` | Force approval-gate preference to `request_user` when consent/confirmation is explicit. |\n")
	b.WriteString("| `lark_* => lark_upload_file` | Gate upload routing on explicit upload/attachment/file signals; route context/status intents to history/message tools. |\n")
	b.WriteString("| `write_file|artifacts_write => write_attachment` | Penalize attachment path when deliverable is durable artifact/file storage without upload requirement. |\n")
	b.WriteString("| `ripgrep => search_file` | Prioritize regex/fast-scan repository intents for `ripgrep`; dampen generic `search_file` in that context. |\n")
	b.WriteString("| `shell_exec => execute_code` | Distinguish command/CLI/process checks (`shell_exec`) from deterministic computation snippets (`execute_code`). |\n\n")

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
