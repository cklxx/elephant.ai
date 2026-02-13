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

	preparation "alex/internal/app/agent/preparation"
	"alex/internal/app/toolregistry"
	ports "alex/internal/domain/agent/ports"
	"alex/internal/infra/memory"
	"alex/internal/domain/agent/presets"

	"gopkg.in/yaml.v3"
)

const (
	defaultFoundationCasesPath = "evaluation/agent_eval/datasets/foundation_eval_cases.yaml"
)

// FoundationEvaluationOptions controls offline baseline evaluation.
type FoundationEvaluationOptions struct {
	OutputDir    string
	Mode         string
	Preset       string
	Toolset      string
	CasesPath    string
	TopK         int
	ReportFormat string
}

// DefaultFoundationEvaluationOptions returns stable defaults for offline eval.
func DefaultFoundationEvaluationOptions() *FoundationEvaluationOptions {
	return &FoundationEvaluationOptions{
		OutputDir:    "./evaluation_results/foundation",
		Mode:         "web",
		Preset:       string(presets.ToolPresetFull),
		Toolset:      string(toolregistry.ToolsetDefault),
		CasesPath:    defaultFoundationCasesPath,
		TopK:         3,
		ReportFormat: "markdown",
	}
}

// FoundationEvaluationResult is the full output of offline baseline scoring.
type FoundationEvaluationResult struct {
	RunID           string                    `json:"run_id"`
	GeneratedAt     time.Time                 `json:"generated_at"`
	Mode            string                    `json:"mode"`
	Preset          string                    `json:"preset"`
	Toolset         string                    `json:"toolset"`
	CasesPath       string                    `json:"cases_path"`
	TopK            int                       `json:"top_k"`
	Prompt          FoundationPromptSummary   `json:"prompt"`
	Tools           FoundationToolSummary     `json:"tools"`
	Implicit        FoundationImplicitSummary `json:"implicit"`
	OverallScore    float64                   `json:"overall_score"`
	Recommendations []string                  `json:"recommendations"`
	ReportArtifacts []EvaluationArtifact      `json:"report_artifacts,omitempty"`
}

// FoundationPromptSummary holds prompt-quality scoring.
type FoundationPromptSummary struct {
	TotalPrompts int                     `json:"total_prompts"`
	AverageScore float64                 `json:"average_score"`
	StrongCount  int                     `json:"strong_count"`
	WeakCount    int                     `json:"weak_count"`
	Scores       []FoundationPromptScore `json:"scores"`
}

// FoundationPromptScore is per-prompt scorecard.
type FoundationPromptScore struct {
	Name      string   `json:"name"`
	Score     float64  `json:"score"`
	WordCount int      `json:"word_count"`
	Strengths []string `json:"strengths,omitempty"`
	Gaps      []string `json:"gaps,omitempty"`
}

// FoundationToolSummary holds tool usability/discoverability scoring.
type FoundationToolSummary struct {
	TotalTools             int                   `json:"total_tools"`
	AverageUsability       float64               `json:"average_usability"`
	AverageDiscoverability float64               `json:"average_discoverability"`
	PassRate               float64               `json:"pass_rate"`
	CriticalIssues         int                   `json:"critical_issues"`
	IssueBreakdown         map[string]int        `json:"issue_breakdown,omitempty"`
	Scores                 []FoundationToolScore `json:"scores"`
}

// FoundationToolScore is per-tool scorecard.
type FoundationToolScore struct {
	Name                 string   `json:"name"`
	Category             string   `json:"category,omitempty"`
	SafetyLevel          int      `json:"safety_level"`
	UsabilityScore       float64  `json:"usability_score"`
	DiscoverabilityScore float64  `json:"discoverability_score"`
	Issues               []string `json:"issues,omitempty"`
}

// FoundationImplicitSummary contains scenario-based implicit tool readiness.
type FoundationImplicitSummary struct {
	TotalCases               int                    `json:"total_cases"`
	ApplicableCases          int                    `json:"applicable_cases"`
	NotApplicableCases       int                    `json:"not_applicable_cases"`
	PassedCases              int                    `json:"passed_cases"`
	FailedCases              int                    `json:"failed_cases"`
	PassAt1Cases             int                    `json:"pass_at_1_cases"`
	PassAt5Cases             int                    `json:"pass_at_5_cases"`
	PassAt1Rate              float64                `json:"pass_at_1_rate"`
	PassAt5Rate              float64                `json:"pass_at_5_rate"`
	Top1HitRate              float64                `json:"top1_hit_rate"`
	TopKHitRate              float64                `json:"topk_hit_rate"`
	MRR                      float64                `json:"mrr"`
	TotalEvaluationLatencyMs int64                  `json:"total_evaluation_latency_ms"`
	AverageCaseLatencyMs     float64                `json:"average_case_latency_ms"`
	CaseLatencyP50Ms         float64                `json:"case_latency_p50_ms"`
	CaseLatencyP95Ms         float64                `json:"case_latency_p95_ms"`
	CaseLatencyP99Ms         float64                `json:"case_latency_p99_ms"`
	ThroughputCasesPerSec    float64                `json:"throughput_cases_per_sec"`
	CaseResults              []FoundationCaseResult `json:"case_results"`
}

// FoundationCaseResult captures one implicit-intent scenario result.
type FoundationCaseResult struct {
	ID               string                         `json:"id"`
	Category         string                         `json:"category"`
	Intent           string                         `json:"intent"`
	ExpectedTools    []string                       `json:"expected_tools"`
	Deliverable      *FoundationDeliverableContract `json:"deliverable,omitempty"`
	DeliverableCheck *FoundationDeliverableCheck    `json:"deliverable_check,omitempty"`
	TopMatches       []FoundationToolMatch          `json:"top_matches"`
	HitRank          int                            `json:"hit_rank"`
	Passed           bool                           `json:"passed"`
	NotApplicable    bool                           `json:"not_applicable,omitempty"`
	FailureType      string                         `json:"failure_type,omitempty"`
	Reason           string                         `json:"reason"`
	RoutingLatencyMs float64                        `json:"routing_latency_ms"`
}

// FoundationToolMatch is a ranked tool candidate for one scenario.
type FoundationToolMatch struct {
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}

// FoundationCaseSet is the YAML schema for implicit intent scenarios.
type FoundationCaseSet struct {
	Version     string               `yaml:"version"`
	Name        string               `yaml:"name"`
	Description string               `yaml:"description,omitempty"`
	Scenarios   []FoundationScenario `yaml:"scenarios"`
}

// FoundationScenario is one intent + expected tool mapping.
type FoundationScenario struct {
	ID            string                         `yaml:"id"`
	Category      string                         `yaml:"category"`
	Intent        string                         `yaml:"intent"`
	ExpectedTools []string                       `yaml:"expected_tools"`
	Deliverable   *FoundationDeliverableContract `yaml:"deliverable,omitempty"`
}

// FoundationDeliverableContract describes expected file/artifact deliverables for one scenario.
type FoundationDeliverableContract struct {
	OutputDescription  string   `yaml:"output_description,omitempty" json:"output_description,omitempty"`
	ArtifactRequired   bool     `yaml:"artifact_required,omitempty" json:"artifact_required,omitempty"`
	AttachmentRequired bool     `yaml:"attachment_required,omitempty" json:"attachment_required,omitempty"`
	ManifestRequired   bool     `yaml:"manifest_required,omitempty" json:"manifest_required,omitempty"`
	RequiredEvidence   []string `yaml:"required_evidence,omitempty" json:"required_evidence,omitempty"`
	RequiredFileTypes  []string `yaml:"required_file_types,omitempty" json:"required_file_types,omitempty"`
}

// FoundationDeliverableCheck records contract readiness checks from ranked tools.
type FoundationDeliverableCheck struct {
	Applicable         bool     `json:"applicable"`
	SignalCount        int      `json:"signal_count"`
	MatchedSignals     int      `json:"matched_signals"`
	ContractCoverage   float64  `json:"contract_coverage"`
	Status             string   `json:"status"`
	MatchedSignalNames []string `json:"matched_signal_names,omitempty"`
	MissingSignalNames []string `json:"missing_signal_names,omitempty"`
	Reason             string   `json:"reason"`
}

type foundationToolProfile struct {
	Definition   ports.ToolDefinition
	Metadata     ports.ToolMetadata
	TokenWeights map[string]float64
}

// RunFoundationEvaluation executes the offline baseline evaluation without any LLM call.
func RunFoundationEvaluation(ctx context.Context, options *FoundationEvaluationOptions) (*FoundationEvaluationResult, error) {
	if options == nil {
		options = DefaultFoundationEvaluationOptions()
	}
	opts := *options
	if strings.TrimSpace(opts.OutputDir) == "" {
		opts.OutputDir = DefaultFoundationEvaluationOptions().OutputDir
	}
	if strings.TrimSpace(opts.Mode) == "" {
		opts.Mode = DefaultFoundationEvaluationOptions().Mode
	}
	if strings.TrimSpace(opts.Preset) == "" {
		opts.Preset = DefaultFoundationEvaluationOptions().Preset
	}
	if strings.TrimSpace(opts.Toolset) == "" {
		opts.Toolset = DefaultFoundationEvaluationOptions().Toolset
	}
	if strings.TrimSpace(opts.CasesPath) == "" {
		opts.CasesPath = defaultFoundationCasesPath
	}
	if opts.TopK <= 0 {
		opts.TopK = DefaultFoundationEvaluationOptions().TopK
	}
	if strings.TrimSpace(opts.ReportFormat) == "" {
		opts.ReportFormat = DefaultFoundationEvaluationOptions().ReportFormat
	}

	mode := normalizeFoundationMode(opts.Mode)
	if mode != presets.ToolModeCLI && mode != presets.ToolModeWeb {
		return nil, fmt.Errorf("unsupported mode: %s", opts.Mode)
	}

	caseSet, err := LoadFoundationCaseSet(opts.CasesPath)
	if err != nil {
		return nil, err
	}

	promptSummary := evaluatePrompts(mode)

	toolProfiles, err := collectToolProfiles(ctx, mode, opts.Preset, opts.Toolset)
	if err != nil {
		return nil, err
	}
	toolSummary := evaluateTools(toolProfiles)

	implicitSummary := evaluateImplicitCases(caseSet.Scenarios, toolProfiles, opts.TopK)

	overall := clamp01(
		0.25*(promptSummary.AverageScore/100.0)+
			0.30*(toolSummary.AverageUsability/100.0)+
			0.20*(toolSummary.AverageDiscoverability/100.0)+
			0.25*implicitSummary.PassAt5Rate,
	) * 100

	result := &FoundationEvaluationResult{
		RunID:           fmt.Sprintf("foundation-%s", time.Now().UTC().Format("20060102-150405")),
		GeneratedAt:     time.Now().UTC(),
		Mode:            string(mode),
		Preset:          opts.Preset,
		Toolset:         string(toolregistry.NormalizeToolset(opts.Toolset)),
		CasesPath:       opts.CasesPath,
		TopK:            opts.TopK,
		Prompt:          promptSummary,
		Tools:           toolSummary,
		Implicit:        implicitSummary,
		OverallScore:    round1(overall),
		Recommendations: buildFoundationRecommendations(promptSummary, toolSummary, implicitSummary),
	}

	artifacts, err := writeFoundationArtifacts(result, opts.OutputDir, opts.ReportFormat)
	if err != nil {
		return nil, err
	}
	result.ReportArtifacts = artifacts

	return result, nil
}

// LoadFoundationCaseSet loads and validates the scenario YAML.
func LoadFoundationCaseSet(path string) (*FoundationCaseSet, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("foundation case set path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read foundation case set: %w", err)
	}
	var set FoundationCaseSet
	if err := yaml.Unmarshal(data, &set); err != nil {
		return nil, fmt.Errorf("decode foundation case set: %w", err)
	}
	if strings.TrimSpace(set.Name) == "" {
		return nil, fmt.Errorf("foundation case set name is required")
	}
	if strings.TrimSpace(set.Version) == "" {
		return nil, fmt.Errorf("foundation case set version is required")
	}
	if len(set.Scenarios) == 0 {
		return nil, fmt.Errorf("foundation case set must contain scenarios")
	}
	seen := make(map[string]struct{}, len(set.Scenarios))
	for idx, scenario := range set.Scenarios {
		id := strings.TrimSpace(scenario.ID)
		if id == "" {
			return nil, fmt.Errorf("scenario[%d] id is required", idx)
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("duplicate scenario id: %s", id)
		}
		seen[id] = struct{}{}
		if strings.TrimSpace(scenario.Intent) == "" {
			return nil, fmt.Errorf("scenario %s intent is required", id)
		}
		if len(scenario.ExpectedTools) == 0 {
			return nil, fmt.Errorf("scenario %s expected_tools is required", id)
		}
		if scenario.Deliverable != nil {
			contract := scenario.Deliverable
			contract.OutputDescription = strings.TrimSpace(contract.OutputDescription)
			contract.RequiredEvidence = uniqueNonEmptyStrings(contract.RequiredEvidence)
			contract.RequiredFileTypes = uniqueNonEmptyStrings(contract.RequiredFileTypes)
			if contract.OutputDescription == "" &&
				!contract.ArtifactRequired &&
				!contract.AttachmentRequired &&
				!contract.ManifestRequired &&
				len(contract.RequiredEvidence) == 0 &&
				len(contract.RequiredFileTypes) == 0 {
				return nil, fmt.Errorf("scenario %s deliverable must define at least one requirement", id)
			}
		}
	}
	return &set, nil
}

func normalizeFoundationMode(mode string) presets.ToolMode {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case string(presets.ToolModeWeb):
		return presets.ToolModeWeb
	case string(presets.ToolModeCLI):
		return presets.ToolModeCLI
	default:
		return presets.ToolMode(mode)
	}
}

func evaluatePrompts(mode presets.ToolMode) FoundationPromptSummary {
	profiles := collectPromptProfiles(mode)
	scores := make([]FoundationPromptScore, 0, len(profiles))
	var total float64
	strong := 0
	weak := 0
	for _, profile := range profiles {
		score := scorePrompt(profile.name, profile.text)
		scores = append(scores, score)
		total += score.Score
		if score.Score >= 80 {
			strong++
		}
		if score.Score < 70 {
			weak++
		}
	}
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].Score == scores[j].Score {
			return scores[i].Name < scores[j].Name
		}
		return scores[i].Score > scores[j].Score
	})
	avg := 0.0
	if len(scores) > 0 {
		avg = total / float64(len(scores))
	}
	return FoundationPromptSummary{
		TotalPrompts: len(scores),
		AverageScore: round1(avg),
		StrongCount:  strong,
		WeakCount:    weak,
		Scores:       scores,
	}
}

type promptProfile struct {
	name string
	text string
}

func collectPromptProfiles(mode presets.ToolMode) []promptProfile {
	profiles := make([]promptProfile, 0, 10)
	profiles = append(profiles, promptProfile{
		name: "runtime/default-fallback",
		text: composePromptForMode(preparation.DefaultSystemPrompt, mode),
	})

	presetOrder := []presets.AgentPreset{
		presets.PresetDefault,
		presets.PresetCodeExpert,
		presets.PresetResearcher,
		presets.PresetDevOps,
		presets.PresetSecurityAnalyst,
		presets.PresetDesigner,
		presets.PresetArchitect,
	}
	for _, preset := range presetOrder {
		cfg, err := presets.GetPromptConfig(preset)
		if err != nil || cfg == nil {
			continue
		}
		profiles = append(profiles, promptProfile{
			name: fmt.Sprintf("preset/%s", string(preset)),
			text: composePromptForMode(cfg.SystemPrompt, mode),
		})
	}

	return profiles
}

func composePromptForMode(base string, mode presets.ToolMode) string {
	base = strings.TrimSpace(base)
	if mode == presets.ToolModeWeb {
		return strings.TrimSpace(base + `

## Artifacts & Attachments
- When producing long-form deliverables (reports, articles, specs), write them to a Markdown artifact via artifacts_write.
- Provide a short summary in the final answer and point the user to the generated file instead of pasting the full content.
- Keep attachment placeholders out of the main body; list them at the end of the final answer.
- If you want clients to render an attachment card, reference the file with a placeholder like [report.md].`)
	}
	return strings.TrimSpace(base + `

## File Outputs
- When producing long-form deliverables (reports, articles, specs), write them to a Markdown file via write_file.
- Provide a short summary in the final answer and point the user to the generated file path instead of pasting the full content.`)
}

func scorePrompt(name, text string) FoundationPromptScore {
	lower := strings.ToLower(text)
	words := tokenize(text)
	wordCount := len(words)
	strengths := make([]string, 0, 8)
	gaps := make([]string, 0, 8)
	score := 0.0

	type signal struct {
		label    string
		weight   float64
		keywords []string
	}
	signals := []signal{
		{label: "角色定义清晰", weight: 18, keywords: []string{"you are", "identity", "role", "persona"}},
		{label: "工具使用指导", weight: 18, keywords: []string{"tool", "tools", "use ", "invoke"}},
		{label: "执行流程明确", weight: 16, keywords: []string{"approach", "workflow", "checklist", "step", "plan"}},
		{label: "安全约束存在", weight: 16, keywords: []string{"do not", "must not", "never", "avoid", "requires"}},
		{label: "输出契约明确", weight: 16, keywords: []string{"final answer", "output", "report", "summary"}},
		{label: "隐式工具发现提示", weight: 16, keywords: []string{"appropriate tools", "use tools efficiently", "choose", "select", "tool"}},
	}
	for _, sig := range signals {
		if containsAnySubstring(lower, sig.keywords...) {
			score += sig.weight
			strengths = append(strengths, sig.label)
		} else {
			gaps = append(gaps, "缺少"+sig.label)
		}
	}

	switch {
	case wordCount >= 160 && wordCount <= 1400:
		score += 8
		strengths = append(strengths, "长度处于可用区间")
	case wordCount < 80:
		gaps = append(gaps, "提示词过短，信息密度不足")
	case wordCount > 2200:
		gaps = append(gaps, "提示词过长，可能稀释关键指令")
		score -= 6
	default:
		score += 4
	}

	score = math.Max(0, math.Min(100, score))

	return FoundationPromptScore{
		Name:      name,
		Score:     round1(score),
		WordCount: wordCount,
		Strengths: strengths,
		Gaps:      gaps,
	}
}

func collectToolProfiles(ctx context.Context, mode presets.ToolMode, presetName, toolsetName string) ([]foundationToolProfile, error) {
	memRoot, err := os.MkdirTemp("", "foundation-eval-memory-*")
	if err != nil {
		return nil, fmt.Errorf("create temp memory root: %w", err)
	}
	defer os.RemoveAll(memRoot)

	engine := memory.NewMarkdownEngine(memRoot)
	if err := engine.EnsureSchema(ctx); err != nil {
		return nil, fmt.Errorf("initialize memory schema: %w", err)
	}

	registry, err := toolregistry.NewRegistry(toolregistry.Config{
		MemoryEngine: engine,
		Toolset:      toolregistry.NormalizeToolset(toolsetName),
	})
	if err != nil {
		return nil, fmt.Errorf("build tool registry: %w", err)
	}
	defer registry.Close()

	preset := presets.ToolPreset(strings.TrimSpace(presetName))
	filtered, err := presets.NewFilteredToolRegistry(registry, mode, preset)
	if err != nil {
		return nil, fmt.Errorf("filter tool registry (mode=%s preset=%s): %w", mode, presetName, err)
	}

	defs := filtered.List()
	profiles := make([]foundationToolProfile, 0, len(defs))
	for _, def := range defs {
		exec, err := filtered.Get(def.Name)
		if err != nil {
			continue
		}
		meta := exec.Metadata()
		tokenWeights := make(map[string]float64, 32)
		addTokenWeights(tokenWeights, tokenize(def.Name), 3.0)
		addTokenWeights(tokenWeights, tokenize(def.Description), 2.0)
		addTokenWeights(tokenWeights, tokenize(meta.Category), 1.5)
		for _, tag := range meta.Tags {
			addTokenWeights(tokenWeights, tokenize(tag), 1.5)
		}
		for propName, prop := range def.Parameters.Properties {
			addTokenWeights(tokenWeights, tokenize(propName), 1.2)
			addTokenWeights(tokenWeights, tokenize(prop.Description), 0.9)
			if prop.Items != nil {
				addTokenWeights(tokenWeights, tokenize(prop.Items.Description), 0.6)
			}
		}
		profiles = append(profiles, foundationToolProfile{
			Definition:   def,
			Metadata:     meta,
			TokenWeights: tokenWeights,
		})
	}

	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Definition.Name < profiles[j].Definition.Name
	})

	return profiles, nil
}

func evaluateTools(profiles []foundationToolProfile) FoundationToolSummary {
	scores := make([]FoundationToolScore, 0, len(profiles))
	issueBreakdown := make(map[string]int)
	var usabilityTotal float64
	var discoverabilityTotal float64
	pass := 0
	critical := 0

	for _, profile := range profiles {
		score := scoreToolProfile(profile)
		scores = append(scores, score)
		usabilityTotal += score.UsabilityScore
		discoverabilityTotal += score.DiscoverabilityScore
		if score.UsabilityScore >= 70 {
			pass++
		}
		if score.UsabilityScore < 50 {
			critical++
		}
		for _, issue := range score.Issues {
			issueBreakdown[issue]++
		}
	}

	sort.Slice(scores, func(i, j int) bool {
		if scores[i].UsabilityScore == scores[j].UsabilityScore {
			return scores[i].Name < scores[j].Name
		}
		return scores[i].UsabilityScore < scores[j].UsabilityScore
	})

	total := len(scores)
	if total == 0 {
		return FoundationToolSummary{}
	}

	return FoundationToolSummary{
		TotalTools:             total,
		AverageUsability:       round1(usabilityTotal / float64(total)),
		AverageDiscoverability: round1(discoverabilityTotal / float64(total)),
		PassRate:               round1(float64(pass) / float64(total) * 100),
		CriticalIssues:         critical,
		IssueBreakdown:         issueBreakdown,
		Scores:                 scores,
	}
}

func scoreToolProfile(profile foundationToolProfile) FoundationToolScore {
	def := profile.Definition
	meta := profile.Metadata
	issues := make([]string, 0, 8)

	usability := 0.0
	if def.Parameters.Type == "object" {
		usability += 15
	} else {
		issues = append(issues, "schema_not_object")
	}

	requiredValid := 0
	if len(def.Parameters.Required) == 0 {
		usability += 20
	} else {
		for _, req := range def.Parameters.Required {
			if _, ok := def.Parameters.Properties[req]; ok {
				requiredValid++
			}
		}
		coverage := float64(requiredValid) / float64(len(def.Parameters.Required))
		usability += 20 * coverage
		if requiredValid < len(def.Parameters.Required) {
			issues = append(issues, "required_property_missing")
		}
	}

	propCount := len(def.Parameters.Properties)
	if propCount == 0 {
		usability += 20
		usability += 20
		usability += 10
	} else {
		typed := 0
		described := 0
		arrays := 0
		arraysWithItems := 0
		for _, prop := range def.Parameters.Properties {
			if strings.TrimSpace(prop.Type) != "" {
				typed++
			}
			if len(strings.Fields(strings.TrimSpace(prop.Description))) >= 3 {
				described++
			}
			if prop.Type == "array" {
				arrays++
				if prop.Items != nil {
					arraysWithItems++
				}
			}
		}
		usability += 20 * float64(typed) / float64(propCount)
		usability += 20 * float64(described) / float64(propCount)
		if arrays == 0 {
			usability += 10
		} else {
			usability += 10 * float64(arraysWithItems) / float64(arrays)
			if arraysWithItems < arrays {
				issues = append(issues, "array_items_missing")
			}
		}
		if typed < propCount {
			issues = append(issues, "property_type_missing")
		}
		if described < propCount {
			issues = append(issues, "property_description_thin")
		}
	}

	descWords := len(strings.Fields(strings.TrimSpace(def.Description)))
	if descWords >= 5 {
		usability += 10
	} else {
		issues = append(issues, "tool_description_thin")
	}

	if strings.TrimSpace(meta.Category) != "" {
		usability += 3
	} else {
		issues = append(issues, "metadata_category_missing")
	}

	level := meta.EffectiveSafetyLevel()
	if level >= ports.SafetyLevelReadOnly && level <= ports.SafetyLevelIrreversible {
		usability += 2
	} else {
		issues = append(issues, "safety_level_missing")
	}

	discoverability := 0.0
	nameParts := strings.Split(def.Name, "_")
	if len(nameParts) >= 2 {
		discoverability += 20
	} else {
		issues = append(issues, "name_not_action_object")
	}
	if containsVerb(def.Name) || containsVerb(def.Description) {
		discoverability += 20
	} else {
		issues = append(issues, "verb_missing")
	}
	if descWords >= 6 && descWords <= 45 {
		discoverability += 25
	} else if descWords >= 4 {
		discoverability += 15
	} else {
		issues = append(issues, "description_not_informative")
	}
	if strings.TrimSpace(meta.Category) != "" {
		discoverability += 7
	}
	if len(meta.Tags) >= 2 {
		discoverability += 8
	} else if len(meta.Tags) == 1 {
		discoverability += 5
	} else {
		issues = append(issues, "metadata_tags_missing")
	}
	if tokenRichness(profile.TokenWeights) >= 12 {
		discoverability += 20
	} else {
		discoverability += 8 + float64(tokenRichness(profile.TokenWeights))
		issues = append(issues, "semantic_tokens_sparse")
	}

	if len(issues) == 0 {
		issues = nil
	}

	return FoundationToolScore{
		Name:                 def.Name,
		Category:             meta.Category,
		SafetyLevel:          level,
		UsabilityScore:       round1(math.Min(100, usability)),
		DiscoverabilityScore: round1(math.Min(100, discoverability)),
		Issues:               uniqueNonEmptyStrings(issues),
	}
}

func evaluateImplicitCases(scenarios []FoundationScenario, profiles []foundationToolProfile, topK int) FoundationImplicitSummary {
	evalStart := time.Now()
	results := make([]FoundationCaseResult, 0, len(scenarios))
	latencies := make([]float64, 0, len(scenarios))
	profilesByName := make(map[string]foundationToolProfile, len(profiles))
	for _, profile := range profiles {
		profilesByName[profile.Definition.Name] = profile
	}

	passAt1Hits := 0
	passAt5Hits := 0
	topKHits := 0
	notApplicableCases := 0
	mrr := 0.0

	for _, scenario := range scenarios {
		caseStart := time.Now()
		intentTokens := tokenize(scenario.Intent)
		ranked := rankToolsForIntent(intentTokens, profiles)

		expectedAvailable := make([]string, 0, len(scenario.ExpectedTools))
		expectedMissing := make([]string, 0, len(scenario.ExpectedTools))
		for _, expected := range scenario.ExpectedTools {
			if _, ok := profilesByName[expected]; ok {
				expectedAvailable = append(expectedAvailable, expected)
			} else {
				expectedMissing = append(expectedMissing, expected)
			}
		}

		hitRank := 0
		hitScore := 0.0
		expectedSet := make(map[string]struct{}, len(expectedAvailable))
		for _, expected := range expectedAvailable {
			expectedSet[expected] = struct{}{}
		}
		for idx, match := range ranked {
			if _, ok := expectedSet[match.Name]; !ok {
				continue
			}
			if match.Score <= 0 {
				continue
			}
			hitRank = idx + 1
			hitScore = match.Score
			break
		}

		failureType := ""
		reason := ""
		notApplicable := false
		if len(expectedAvailable) == 0 {
			failureType = "availability_error"
			notApplicable = true
			reason = "expected tools unavailable in active mode/preset/toolset: " + strings.Join(expectedMissing, ", ")
		} else if hitRank == 0 {
			failureType = "no_overlap"
			reason = "expected tool has no lexical overlap with intent terms"
		} else if hitRank > topK {
			failureType = "rank_below_top_k"
			reason = fmt.Sprintf("expected tool ranked #%d (score=%.2f), outside Top-%d", hitRank, hitScore, topK)
		} else {
			matchedTerms := make([]string, 0, 6)
			for _, expected := range expectedAvailable {
				profile, ok := profilesByName[expected]
				if !ok {
					continue
				}
				for _, token := range intentTokens {
					norm := normalizeToken(token)
					if norm == "" {
						continue
					}
					if _, exists := profile.TokenWeights[norm]; exists {
						matchedTerms = append(matchedTerms, norm)
					}
				}
			}
			reason = "matched via terms: " + strings.Join(uniqueNonEmptyStrings(matchedTerms), ", ")
			if len(expectedMissing) > 0 {
				reason += fmt.Sprintf(" | additionally unavailable: %s", strings.Join(expectedMissing, ", "))
			}
		}
		topMatches := topToolMatches(ranked, topK)
		passed := !notApplicable && hitRank > 0 && hitRank <= topK
		deliverableCheck := evaluateDeliverableContract(scenario.Deliverable, topMatches, topK, passed)
		if notApplicable {
			notApplicableCases++
		} else {
			if hitRank == 1 {
				passAt1Hits++
			}
			if hitRank > 0 && hitRank <= 5 {
				passAt5Hits++
			}
			if passed {
				topKHits++
			}
			if hitRank > 0 {
				mrr += 1.0 / float64(hitRank)
			}
		}

		results = append(results, FoundationCaseResult{
			ID:               scenario.ID,
			Category:         scenario.Category,
			Intent:           scenario.Intent,
			ExpectedTools:    append([]string(nil), scenario.ExpectedTools...),
			Deliverable:      scenario.Deliverable,
			DeliverableCheck: deliverableCheck,
			TopMatches:       topMatches,
			HitRank:          hitRank,
			Passed:           passed,
			NotApplicable:    notApplicable,
			FailureType:      failureType,
			Reason:           strings.TrimSpace(reason),
			RoutingLatencyMs: round3(float64(time.Since(caseStart).Microseconds()) / 1000.0),
		})
		latencies = append(latencies, float64(time.Since(caseStart).Microseconds())/1000.0)
	}

	if len(results) > 1 {
		sort.Slice(results, func(i, j int) bool {
			if results[i].Passed != results[j].Passed {
				return !results[i].Passed && results[j].Passed
			}
			if results[i].HitRank == results[j].HitRank {
				return results[i].ID < results[j].ID
			}
			if results[i].HitRank == 0 {
				return true
			}
			if results[j].HitRank == 0 {
				return false
			}
			return results[i].HitRank > results[j].HitRank
		})
	}

	total := len(results)
	if total == 0 {
		return FoundationImplicitSummary{}
	}
	applicable := total - notApplicableCases
	failed := applicable - topKHits
	denominator := float64(maxInt(applicable, 1))
	totalEvalMs := float64(time.Since(evalStart).Microseconds()) / 1000.0
	avgLatency := 0.0
	for _, latency := range latencies {
		avgLatency += latency
	}
	if len(latencies) > 0 {
		avgLatency /= float64(len(latencies))
	}

	return FoundationImplicitSummary{
		TotalCases:               total,
		ApplicableCases:          applicable,
		NotApplicableCases:       notApplicableCases,
		PassedCases:              topKHits,
		FailedCases:              failed,
		PassAt1Cases:             passAt1Hits,
		PassAt5Cases:             passAt5Hits,
		PassAt1Rate:              round3(float64(passAt1Hits) / denominator),
		PassAt5Rate:              round3(float64(passAt5Hits) / denominator),
		Top1HitRate:              round3(float64(passAt1Hits) / denominator),
		TopKHitRate:              round3(float64(topKHits) / denominator),
		MRR:                      round3(mrr / denominator),
		TotalEvaluationLatencyMs: int64(math.Round(totalEvalMs)),
		AverageCaseLatencyMs:     round3(avgLatency),
		CaseLatencyP50Ms:         round3(percentileFloat(latencies, 50)),
		CaseLatencyP95Ms:         round3(percentileFloat(latencies, 95)),
		CaseLatencyP99Ms:         round3(percentileFloat(latencies, 99)),
		ThroughputCasesPerSec:    round3(float64(total) / math.Max(totalEvalMs/1000.0, 1e-9)),
		CaseResults:              results,
	}
}

func evaluateDeliverableContract(contract *FoundationDeliverableContract, topMatches []FoundationToolMatch, topK int, casePassed bool) *FoundationDeliverableCheck {
	if contract == nil {
		return nil
	}
	requiredSignals := collectDeliverableSignals(contract)
	if len(requiredSignals) == 0 {
		return &FoundationDeliverableCheck{
			Applicable:       false,
			SignalCount:      0,
			MatchedSignals:   0,
			ContractCoverage: 1,
			Status:           "n/a",
			Reason:           "deliverable contract has no tool-level signal requirement",
		}
	}

	selected := topMatches
	if topK > 0 && len(selected) > topK {
		selected = selected[:topK]
	}
	candidateTools := make(map[string]struct{}, len(selected))
	for _, match := range selected {
		if strings.TrimSpace(match.Name) == "" {
			continue
		}
		candidateTools[match.Name] = struct{}{}
	}

	matchedSignals := make([]string, 0, len(requiredSignals))
	missingSignals := make([]string, 0, len(requiredSignals))
	for signal, toolNames := range requiredSignals {
		matched := false
		for _, toolName := range toolNames {
			if _, ok := candidateTools[toolName]; ok {
				matched = true
				break
			}
		}
		if matched {
			matchedSignals = append(matchedSignals, signal)
		} else {
			missingSignals = append(missingSignals, signal)
		}
	}
	sort.Strings(matchedSignals)
	sort.Strings(missingSignals)

	coverage := float64(len(matchedSignals)) / float64(len(requiredSignals))
	status := "bad"
	reason := "deliverable signals not sufficiently covered by top tool matches"
	if coverage >= 0.80 && casePassed {
		status = "good"
		reason = "deliverable signals covered by top tool matches and routing passed"
	}

	return &FoundationDeliverableCheck{
		Applicable:         true,
		SignalCount:        len(requiredSignals),
		MatchedSignals:     len(matchedSignals),
		ContractCoverage:   round3(coverage),
		Status:             status,
		MatchedSignalNames: matchedSignals,
		MissingSignalNames: missingSignals,
		Reason:             reason,
	}
}

func collectDeliverableSignals(contract *FoundationDeliverableContract) map[string][]string {
	signals := make(map[string][]string, 12)

	if contract.ArtifactRequired {
		signals["artifact_output"] = []string{"artifacts_write", "write_file"}
	}
	if contract.AttachmentRequired {
		signals["attachment_delivery"] = []string{"write_attachment", "lark_upload_file"}
	}
	if contract.ManifestRequired {
		signals["artifact_traceability"] = []string{"artifact_manifest", "artifacts_list"}
	}

	for _, evidence := range contract.RequiredEvidence {
		switch normalizeToken(evidence) {
		case "diff", "patch":
			signals["evidence_diff"] = []string{"search_file", "read_file", "replace_in_file"}
		case "test", "tests", "report":
			signals["evidence_test_report"] = []string{"shell_exec", "execute_code", "artifacts_write"}
		case "screenshot", "visual":
			signals["evidence_visual"] = []string{"browser_screenshot"}
		case "diagram":
			signals["evidence_diagram"] = []string{"diagram_render"}
		case "slides", "pptx":
			signals["evidence_slides"] = []string{"pptx_from_images"}
		case "log", "logs":
			signals["evidence_logs"] = []string{"grep", "ripgrep", "search_file"}
		}
	}

	for _, kind := range contract.RequiredFileTypes {
		switch normalizeToken(kind) {
		case "md", "markdown":
			signals["filetype_markdown"] = []string{"artifacts_write", "write_file"}
		case "json":
			signals["filetype_json"] = []string{"a2ui_emit", "artifacts_write", "write_file"}
		case "html":
			signals["filetype_html"] = []string{"html_edit", "artifacts_write", "write_file"}
		case "png", "jpg", "jpeg":
			signals["filetype_image"] = []string{"browser_screenshot", "diagram_render"}
		case "pptx":
			signals["filetype_pptx"] = []string{"pptx_from_images"}
		}
	}

	return signals
}

func rankToolsForIntent(intentTokens []string, profiles []foundationToolProfile) []FoundationToolMatch {
	tokenSet := make(map[string]struct{}, len(intentTokens))
	for _, token := range intentTokens {
		norm := normalizeToken(token)
		if norm == "" {
			continue
		}
		tokenSet[norm] = struct{}{}
	}

	ranked := make([]FoundationToolMatch, 0, len(profiles))
	for _, profile := range profiles {
		score := 0.0
		for token := range tokenSet {
			score += profile.TokenWeights[token]
		}
		score += heuristicIntentBoost(profile.Definition.Name, tokenSet)
		ranked = append(ranked, FoundationToolMatch{Name: profile.Definition.Name, Score: round2(score)})
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Score == ranked[j].Score {
			return ranked[i].Name < ranked[j].Name
		}
		return ranked[i].Score > ranked[j].Score
	})
	return ranked
}

func heuristicIntentBoost(toolName string, tokenSet map[string]struct{}) float64 {
	has := func(tokens ...string) bool {
		for _, token := range tokens {
			if _, ok := tokenSet[token]; ok {
				return true
			}
		}
		return false
	}
	hasAll := func(tokens ...string) bool {
		for _, token := range tokens {
			if _, ok := tokenSet[token]; !ok {
				return false
			}
		}
		return true
	}
	countMatches := func(tokens ...string) int {
		count := 0
		for _, token := range tokens {
			if _, ok := tokenSet[token]; ok {
				count++
			}
		}
		return count
	}

	boost := 0.0
	switch toolName {
	case "plan":
		if has("phase", "milestone", "checkpoint", "risk", "roadmap", "timeline", "migration") {
			boost += 14
		}
		if countMatches("minimal", "smallest", "viable", "weekly", "review", "checkpoint", "rollback") >= 2 {
			boost += 16
		}
		if countMatches("plan", "steps", "fix", "before", "mutation", "change", "apply", "execution") >= 3 {
			boost += 18
		}
		if countMatches("phased", "checklist", "rollback", "checkpoint", "reproduce", "patch", "verify", "gates") >= 3 {
			boost += 20
		}
		if countMatches("before", "task", "updates", "rollout", "phased", "milestones", "risk", "checkpoints") >= 4 {
			boost += 18
		}
		if countMatches("release", "checklist", "milestones", "rollback", "checkpoint", "checkpoints") >= 3 {
			boost += 26
		}
	case "read_file":
		if countMatches("read", "open", "inspect", "view") >= 1 &&
			countMatches("source", "workspace", "file", "content", "line") >= 1 {
			boost += 18
		}
		if countMatches("read", "local", "workspace", "notes", "file", "before") >= 3 {
			boost += 18
		}
		if countMatches("failing", "failure", "function", "context", "logic", "contract", "neighboring") >= 3 {
			boost += 20
		}
		if countMatches("logic", "window", "suspected", "behavior", "before", "patch", "regression") >= 3 {
			boost += 24
		}
	case "clarify":
		if has("ambiguity", "clarify", "blocking", "requirement", "missing", "unclear", "constraint", "conflict") {
			boost += 14
		}
		if countMatches("you", "decide", "anything", "work", "delegate", "default", "low", "reversible", "status", "message", "thread", "again") >= 6 &&
			countMatches("approval", "consent", "confirm", "manual", "external", "irreversible", "critical") == 0 {
			boost -= 24
		}
	case "web_search":
		if countMatches("search", "lookup", "find", "query", "compare") >= 1 &&
			countMatches("web", "internet", "doc", "reference", "official", "site", "url") >= 1 {
			boost += 14
		}
		if countMatches("authoritative", "canonical", "trusted", "reference", "primary", "discover", "shortlist") >= 2 {
			boost += 20
		}
		if has("no", "fixed", "web") || hasAll("source", "not", "selected") {
			boost += 12
		}
	case "web_fetch":
		if countMatches("fetch", "read", "extract", "open", "pull") >= 1 &&
			countMatches("url", "exact", "single", "provided", "fixed", "page", "content") >= 2 {
			boost += 18
		}
		if countMatches("single", "exact", "provided", "fixed", "url", "source", "no", "search") >= 4 {
			boost += 18
		}
		if countMatches("only", "source", "url", "avoid", "broad", "search", "discovery", "already", "chosen") >= 4 {
			boost += 20
		}
		if countMatches("approved", "canonical", "single", "exact", "url", "ingest", "without", "discovery") >= 4 {
			boost += 26
		}
		if countMatches("single", "approved", "link", "ingest", "only", "page") >= 3 {
			boost += 20
		}
	case "browser_dom":
		if has("selector", "dom", "form", "field", "fill", "submit") {
			boost += 14
		}
	case "browser_action":
		if has("coordinate", "canvas", "pixel", "drag", "position") {
			boost += 12
		}
	case "browser_info":
		if countMatches("browser", "tab", "state", "session", "metadata", "url", "current", "title", "viewport", "info", "status", "web") >= 2 {
			boost += 18
		}
		if countMatches("read", "inspect", "state", "status", "metadata", "without", "interaction") >= 3 {
			boost += 14
		}
	case "browser_screenshot":
		if has("capture", "screenshot", "proof", "visual", "evidence", "page") {
			boost += 14
		}
		if countMatches("single", "exact", "approved", "canonical", "url", "ingest", "without", "discovery") >= 4 &&
			!has("visual", "proof", "screenshot", "ui", "capture") {
			boost -= 36
		}
	case "write_file":
		if countMatches("write", "create", "new", "save") >= 1 &&
			countMatches("file", "markdown", "note", "report", "content") >= 1 {
			boost += 18
		}
		if countMatches("ledger", "durable", "audit", "artifact", "progress", "proof") >= 3 {
			boost -= 12
		}
		if countMatches("artifacts_write", "artifact", "artifacts", "report", "downstream", "reusable", "deliverable") >= 2 &&
			!has("workspace", "local", "markdown", "create", "write", "file") {
			boost -= 22
		}
		if countMatches("identify", "locate", "candidate", "pattern", "inside", "files") >= 3 &&
			!has("write", "create", "save", "draft", "new") {
			boost -= 18
		}
		if countMatches("local", "new", "file", "materialization", "materialize", "workspace", "markdown") >= 3 {
			boost += 20
		}
	case "list_dir":
		if countMatches("list", "show", "enumerate", "browse", "tree") >= 1 &&
			countMatches("directory", "folder", "workspace", "path") >= 1 {
			boost += 18
		}
		if countMatches("inventory", "candidate", "path", "directory", "nested", "root", "roots") >= 3 {
			boost += 14
		}
	case "search_file":
		if countMatches("search", "find", "locate", "occurrence", "symbol", "token", "regex", "pattern") >= 1 &&
			countMatches("file", "project", "repo", "source", "code", "across") >= 1 {
			boost += 18
		}
		if countMatches("semantic", "content", "inside", "files", "instead", "path", "names") >= 3 {
			boost += 16
		}
		if countMatches("multihop", "reference", "references", "chain", "authoritative", "statement", "resolve", "across", "files") >= 3 {
			boost += 18
		}
		if has("regex", "needle", "sweep", "fast", "quickly") && !has("content", "snippet", "lines", "inside", "within") {
			// Prefer ripgrep for fast regex repository sweeps over semantic content search.
			boost -= 8
		}
	case "replace_in_file":
		if has("replace", "deprecated", "endpoint", "api", "path", "file", "update") {
			boost += 16
		}
	case "write_attachment":
		if countMatches("attach", "download", "artifact", "generated", "deliver", "share", "export", "write", "file", "summary", "handoff") >= 2 {
			boost += 30
		}
		if hasAll("write", "attach") {
			boost += 8
		}
	case "find":
		if countMatches("find", "locate", "lookup", "name", "filename", "directory", "path") >= 2 {
			boost += 20
		}
		if countMatches("name", "path", "directory", "filename") >= 2 &&
			!has("content", "line", "lines", "snippet", "inside", "within") {
			boost += 8
		}
		if countMatches("nested", "path", "root", "tree", "directory", "name") >= 3 {
			boost += 8
		}
		if countMatches("monorepo", "scope", "candidate", "folder", "filename", "before", "open") >= 3 &&
			!has("content", "snippet", "inside", "within") {
			boost += 12
		}
		if countMatches("directory", "name", "constraint", "constraints", "before", "open") >= 3 &&
			!has("content", "snippet", "inside", "within") {
			boost += 14
		}
		if countMatches("path", "pattern", "before", "content", "inspection", "reduce", "large", "tree") >= 3 {
			boost += 22
		}
		if countMatches("entrypoint", "entrypoints", "module", "layer", "package", "path", "folder", "directory") >= 3 {
			boost += 14
		}
	case "grep":
		if countMatches("grep", "log", "error", "line", "pattern", "match") >= 2 {
			boost += 18
		}
		if countMatches("simple", "grep", "local", "log", "logs", "502", "http") >= 3 {
			boost += 18
		}
	case "lark_calendar_query":
		if countMatches("calendar", "event", "query", "upcoming", "schedule", "check") >= 2 {
			boost += 16
		}
		if countMatches("compute", "calculate", "deterministic", "numeric", "consistency", "snippet", "slices", "fragments") >= 3 &&
			!has("calendar", "event", "meeting", "schedule") {
			boost -= 26
		}
	case "lark_calendar_create":
		if countMatches("calendar", "event", "block", "deadline", "focus", "recovery", "work") >= 2 {
			boost += 18
		}
		if countMatches("create", "calendar", "events", "meeting", "meetings", "kickoff", "review") >= 3 {
			boost += 22
		}
		if countMatches("reserve", "execution", "window", "calendar", "block", "create", "creating") >= 3 {
			boost += 24
		}
	case "lark_calendar_delete":
		if countMatches("calendar", "event", "delete", "remove", "cancel", "stale", "obsolete", "cleanup") >= 2 {
			boost += 20
		}
	case "lark_chat_history":
		if countMatches("chat", "thread", "conversation", "history", "context", "recent", "before") >= 2 {
			boost += 24
		}
		if countMatches("reconstruct", "chronology", "prior", "thread", "turns", "before", "replying", "answer") >= 3 {
			boost += 18
		}
		if countMatches("prior", "chat", "context", "thread", "history", "before", "replying", "no", "file", "transfer") >= 5 {
			boost += 24
		}
	case "okr_write":
		if countMatches("okr", "objective", "key", "result", "progress", "update", "write", "status") >= 2 {
			boost += 16
		}
	case "okr_read":
		if countMatches("okr", "objective", "status", "read", "current", "before", "baseline") >= 2 {
			boost += 18
		}
		if countMatches("workspace", "local", "repo", "repository", "path", "file", "notes") >= 2 &&
			!has("okr", "objective", "key", "result", "goal") {
			boost -= 26
		}
		if countMatches("local", "workspace", "notes", "file", "read", "before") >= 3 &&
			!has("okr", "objective", "key", "result", "goal", "kr") {
			boost -= 34
		}
		if countMatches("code", "source", "function", "failing", "logic", "contract", "module", "repository", "repo") >= 3 &&
			!has("okr", "objective", "key", "result", "goal", "kr") {
			boost -= 28
		}
	case "artifact_manifest":
		if countMatches("manifest", "metadata", "generated", "describe", "artifact") >= 2 {
			boost += 22
		}
	case "artifacts_write":
		if countMatches("artifact", "report", "persist", "save", "write", "reference", "final", "output") >= 2 {
			boost += 20
		}
		if countMatches("concise", "chat", "durable", "reusable", "downstream", "audit", "package", "full") >= 2 {
			boost += 18
		}
		if countMatches("progress", "progres", "momentum", "completed", "proof", "evidence", "summary") >= 2 {
			boost += 14
		}
		if countMatches("progress", "progres", "momentum", "completed", "artifact", "durable", "proof") >= 3 &&
			!has("browser", "dom", "click", "drag", "canvas", "selector", "ui", "page") {
			boost += 16
		}
		if has("artifact", "concise") &&
			countMatches("progress", "progres", "momentum", "completed", "action", "actions", "proof", "evidence") >= 3 {
			boost += 28
		}
		if countMatches("multi", "round", "ledger", "durable", "progress", "artifact", "record") >= 3 {
			boost += 22
		}
		if countMatches("list", "inventory", "enumerate", "existing", "current", "artifacts", "artifact") >= 3 &&
			!has("write", "create", "save", "new", "persist") {
			boost -= 26
		}
		if countMatches("diagram", "architecture", "visual", "brief", "render") >= 3 &&
			!has("artifact", "report", "persist", "write", "deliverable") {
			boost -= 20
		}
	case "diagram_render":
		if countMatches("diagram", "architecture", "visual", "brief", "service", "relationship") >= 3 {
			boost += 24
		}
	case "artifacts_list":
		if countMatches("list", "enumerate", "index", "show", "generated", "artifact") >= 2 {
			boost += 10
		}
		if countMatches("enumerate", "outputs", "produced", "run", "choose", "files", "release", "reviewer") >= 3 {
			boost += 22
		}
		if countMatches("list", "inventory", "enumerate", "existing", "current", "artifacts", "artifact") >= 3 {
			boost += 16
		}
		if countMatches("before", "release", "share", "latest", "valid", "existing", "generated", "outputs", "artifacts") >= 4 {
			boost += 24
		}
		if countMatches("before", "share", "sharing", "execution", "output", "outputs", "list", "existing", "latest", "valid", "artifacts") >= 5 {
			boost += 26
		}
		if countMatches("surface", "existing", "outputs", "produced", "before", "release") >= 3 {
			boost += 24
		}
	case "artifacts_delete":
		if countMatches("delete", "remove", "prune", "cleanup", "stale", "obsolete", "artifact", "artifacts", "legacy") >= 2 {
			boost += 24
		}
		if countMatches("stale", "failed", "run", "bundles", "polluted", "evidence", "cleanup") >= 3 {
			boost += 20
		}
	case "memory_search":
		if countMatches("memory", "prior", "history", "decision", "note", "context", "summary", "recall") >= 2 {
			boost += 20
		}
		if hasAll("before", "offset") && has("known", "exact", "line", "lines") {
			boost += 20
		}
		if countMatches("preference", "habit", "style", "tone", "persona", "format", "choice") >= 2 {
			boost += 16
		}
		if has("recall", "recover", "retrieve") &&
			countMatches("preference", "preferred", "habit", "style", "tone", "persona", "format", "choice", "interaction", "interactions", "behavior", "pattern") >= 1 {
			boost += 14
		}
		if countMatches("motivation", "successful", "pattern", "previous", "cadence", "nudge") >= 2 {
			boost += 12
		}
		if countMatches("previous", "prior", "successful", "pattern", "decision", "policy", "history") >= 3 {
			boost += 10
		}
		if countMatches("communication", "tone", "style", "voice", "habit", "preference", "persona", "soul") >= 3 {
			boost += 20
		}
		if countMatches("memory", "preference", "retrieval", "retrieve", "habit", "persona", "policy", "history") >= 3 {
			boost += 18
		}
		if countMatches("memory", "historical", "incident", "signature", "regression", "guardrail", "before", "patch") >= 3 {
			boost += 22
		}
		if countMatches("sparse", "hidden", "long", "tail", "fact", "facts", "corpus", "notes", "retrieve") >= 3 {
			boost += 24
		}
		if countMatches("historical", "remediation", "playbook", "worked", "similar", "incident", "incidents") >= 3 {
			boost += 24
		}
		if has("memory_get") && has("selected", "exact", "open", "detail", "detailed", "guidance", "context") {
			boost -= 16
		}
	case "memory_get":
		if countMatches("open", "exact", "line", "lines", "offset", "fragment", "citation", "verbatim", "proof", "evidence", "selected", "note") >= 2 {
			boost += 24
		}
		if has("memory_get") {
			boost += 18
		}
		if countMatches("selected", "memory", "note", "open", "detail", "detailed", "guidance", "context", "root", "cause") >= 3 {
			boost += 20
		}
	case "request_user":
		if countMatches("human", "manual", "approval", "approve", "confirm", "consent", "operator", "go-signal", "gate", "out-of-band", "acknowledgement", "wait") >= 2 {
			boost += 26
		}
		if hasAll("before", "continue") && has("manual", "approval", "confirm", "human") {
			boost += 10
		}
		if countMatches("sensitive", "personal", "private", "consent", "confirmation") >= 2 {
			boost += 12
		}
		if countMatches("external", "outreach", "third", "party", "before", "approval", "consent") >= 2 {
			boost += 14
		}
		if countMatches("secret", "token", "user", "provided", "before", "execution", "request") >= 3 {
			boost += 18
		}
		if countMatches("critical", "irreversible", "human", "go", "ahead", "before", "step") >= 3 {
			boost += 28
		}
		if countMatches("freeze", "wait", "greenlight", "silence", "no", "continue") >= 3 {
			boost += 34
		}
		if countMatches("user", "requir", "explicit", "consent", "before", "outreach") >= 4 {
			boost += 34
		}
		if countMatches("you", "decide", "anything", "work", "delegate", "default", "low", "reversible", "status", "message", "thread", "again") >= 6 &&
			countMatches("approval", "consent", "confirm", "manual", "external", "irreversible", "critical") == 0 {
			boost -= 28
		}
		if countMatches("view", "check", "list", "inspect", "status", "repo", "branch", "directory", "structure", "workspace", "read", "only") >= 4 &&
			countMatches("approval", "consent", "manual", "external", "irreversible", "critical", "captcha", "2fa", "login", "publish", "go-signal", "greenlight") == 0 {
			boost -= 32
		}
	case "cancel_timer":
		if countMatches("cancel", "remove", "delete", "drop", "prune", "obsolete", "stale", "duplicate", "timer", "reminder") >= 2 {
			boost += 22
		}
		if countMatches("withdraw", "stale", "nudge", "queue", "queued", "reminder") >= 3 {
			boost += 22
		}
	case "set_timer":
		if countMatches("set", "new", "create", "schedule", "later", "after", "timer", "reminder") >= 2 {
			boost += 12
		}
		if countMatches("arm", "short", "nudge", "next", "focus", "window") >= 3 {
			boost += 20
		}
	case "ripgrep":
		if countMatches("regex", "pattern", "needle", "sweep", "scan", "repo", "repository", "across", "fast", "quick", "hotspot") >= 2 {
			boost += 26
		}
	case "execute_code":
		if countMatches("script", "snippet", "compute", "calculate", "deterministic", "metric", "validate", "score") >= 2 {
			boost += 16
		}
		if countMatches("consistency", "numeric", "fragments", "slices", "deterministic", "check") >= 3 {
			boost += 16
		}
		if countMatches("shell", "command", "cli", "terminal", "process") >= 2 {
			boost -= 14
		}
	case "shell_exec":
		if countMatches("shell", "command", "cli", "terminal", "process", "port", "inspect", "check", "grep", "log") >= 2 {
			boost += 18
		}
		if has("shell_exec") {
			boost += 20
		}
		if countMatches("reproduce", "failure", "failing", "test", "command", "before", "fix") >= 3 {
			boost += 22
		}
		if countMatches("view", "check", "list", "inspect", "repo", "branch", "status", "directory", "structure", "workspace", "read", "only") >= 4 &&
			countMatches("approval", "consent", "manual", "external", "irreversible", "critical", "captcha", "2fa", "login", "publish", "go-signal", "greenlight") == 0 {
			boost += 18
		}
	case "scheduler_list_jobs":
		if countMatches("job", "jobs", "list", "inventory", "registered", "cadence", "schedule", "show") >= 2 {
			boost += 18
		}
		if countMatches("audit", "inspect", "current", "existing", "recurring", "automation", "automations", "next", "fire", "time", "times", "before", "change", "policy", "cadence") >= 3 {
			boost += 18
		}
		if countMatches("freeze", "frozen", "mutation", "resume", "state", "scheduled", "before", "write", "writes") >= 3 {
			boost += 22
		}
		if countMatches("show", "registered", "recurrence", "recurrences", "recurring", "before", "mutation") >= 3 {
			boost += 24
		}
		if countMatches("inspect", "current", "schedule", "state", "frozen", "mutation", "first") >= 3 {
			boost += 28
		}
		if countMatches("reveal", "currently", "registered", "recurring", "automations") >= 3 {
			boost += 30
		}
	case "scheduler_create_job":
		if countMatches("recurring", "weekday", "daily", "nightly", "followup", "accountability", "checkin", "scheduler", "job") >= 2 {
			boost += 20
		}
		if countMatches("schedule", "automatic", "followup", "reply", "status", "when", "no") >= 3 {
			boost += 14
		}
		if countMatches("register", "new", "cadence", "stable", "identifier", "recurring", "job") >= 3 {
			boost += 20
		}
		if countMatches("spin", "fresh", "recurring", "line", "stable", "handle") >= 3 {
			boost += 22
		}
	case "scheduler_delete_job":
		if countMatches("obsolete", "stale", "scheduler", "job", "delete", "remove", "checkin") >= 2 {
			boost += 14
		}
		if countMatches("legacy", "deprecation", "deprecated", "retired", "obsolete", "recurring", "cadence", "checkin", "checkins", "automation", "automations", "remove", "delete") >= 3 {
			boost += 16
		}
		if countMatches("violates", "policy", "remove", "removed", "not", "recreated", "recreate", "cadence", "recurring") >= 4 {
			boost += 24
		}
		if countMatches("violate", "policy", "remove", "not", "recreate", "cadence", "recurring") >= 4 {
			boost += 24
		}
		if countMatches("old", "retired", "recurring", "cadence", "violate", "policy", "removed") >= 3 {
			boost += 24
		}
		if countMatches("sunset", "retire", "standing", "recurring", "cadence", "circulation") >= 3 {
			boost += 24
		}
		if countMatches("legacy", "recurring", "automation", "violates", "policy", "retired", "remove", "obsolete", "schedule") >= 3 {
			boost += 28
		}
	case "list_timers":
		if countMatches("timer", "timers", "reminder", "reminders", "remaining", "active", "schedule") >= 2 {
			boost += 20
		}
		if countMatches("queued", "queue", "later", "nudge", "today", "active") >= 3 {
			boost += 20
		}
	case "lark_upload_file":
		if countMatches("upload", "file", "lark", "thread", "chat", "conversation") >= 2 {
			boost += 24
		}
	case "channel":
		if countMatches("send", "message", "status", "thread", "chat", "lark") >= 2 {
			boost += 14
		}
		if countMatches("user", "requir", "explicit", "consent", "before", "outreach", "approval", "external") >= 4 &&
			countMatches("send", "message", "status", "thread", "chat", "lark") < 2 {
			boost -= 36
		}
		if countMatches("you", "decide", "anything", "work", "delegate", "default", "low", "reversible", "status", "message", "thread", "again") >= 6 &&
			countMatches("approval", "consent", "confirm", "manual", "external", "irreversible", "critical") == 0 {
			boost += 28
		}
	case "lark_send_message":
		if countMatches("send", "message", "update", "status", "lark", "thread", "chat") >= 2 {
			boost += 14
		}
		if countMatches("status", "announce", "broadcast", "notify", "share") >= 1 {
			boost += 8
		}
		if countMatches("checkin", "encourage", "nudge", "progress", "reminder", "followup") >= 2 {
			boost += 10
		}
		if has("without", "no", "not") && countMatches("upload", "attach", "file") >= 1 {
			boost += 14
		}
		if countMatches("checkpoint", "status", "message", "short", "brief", "thread", "chat") >= 3 &&
			!has("edit", "replace", "patch", "file", "upload", "attach") {
			boost += 14
		}
		if countMatches("thread", "status", "ping", "brief", "short", "no", "file", "transfer") >= 4 {
			boost += 18
		}
	case "a2ui_emit":
		if has("payload", "renderer", "render", "ui", "protocol", "structured") {
			boost += 12
		}
	}

	// Exact tool-name mentions in intent should bias ranking for that tool.
	if has(toolName) {
		switch toolName {
		case "plan", "clarify", "find", "search_file", "read_file", "write_file":
			boost += 6
		default:
			boost += 24
		}
	}

	if strings.HasSuffix(toolName, "_list") && has("list", "show", "enumerate", "all") {
		boost += 4
	}
	if strings.HasSuffix(toolName, "_create") && has("create", "new") {
		boost += 3
	}
	if strings.HasSuffix(toolName, "_update") && has("update", "modify", "change") {
		boost += 3
	}
	if strings.HasSuffix(toolName, "_delete") && has("delete", "remove") {
		boost += 3
	}
	if strings.HasSuffix(toolName, "_query") && has("query", "search", "lookup") {
		boost += 3
	}
	if strings.HasPrefix(toolName, "web_") && has("web", "url", "page") {
		boost += 2
	}
	if strings.HasPrefix(toolName, "lark_") && has("lark") {
		boost += 2
	}
	if toolName == "lark_calendar_create" || toolName == "lark_calendar_update" || toolName == "lark_calendar_delete" {
		if has("query", "check", "upcoming", "search", "list") &&
			!has("create", "new", "update", "modify", "change", "delete", "remove") {
			boost -= 6
		}
	}
	if toolName == "replace_in_file" {
		if has("okr", "objective", "key", "result", "progress") && !has("replace", "patch", "endpoint", "string") {
			boost -= 6
		}
	}
	if toolName == "lark_task_manage" {
		if has("artifact", "artifacts", "manifest", "cleanup", "delete", "remove", "stale") &&
			!has("task", "assign", "owner", "due", "todo") {
			boost -= 12
		}
	}
	if toolName == "clarify" {
		if countMatches("manual", "approval", "approve", "confirm", "consent", "operator", "gate", "user") >= 2 {
			boost -= 24
		}
		if countMatches("delegate", "executor", "parallel", "subagent", "handoff", "heavy") >= 2 {
			boost -= 12
		}
		if countMatches("memory", "habit", "preference", "style", "persona", "soul") >= 2 {
			boost -= 18
		}
		if countMatches("create", "event", "calendar", "schedule", "timer", "artifact", "attach", "downloadable") >= 2 &&
			!has("unclear", "ambiguity", "clarify", "conflict") {
			boost -= 14
		}
		if has("artifact", "report", "attachment", "downloadable") &&
			!has("unclear", "ambiguity", "clarify", "conflict") {
			boost -= 10
		}
		if countMatches("request", "user", "approval", "confirm", "before", "proceed", "execution", "secret", "token") >= 4 {
			boost -= 24
		}
		if countMatches("replace", "patch", "rewrite", "update", "shift", "run", "show", "apply", "exact", "existing", "inplace", "event") >= 3 &&
			!has("unclear", "ambiguity", "clarify", "conflict", "missing", "question") {
			boost -= 30
		}
		if countMatches("tracked", "task", "item", "operationalize", "commitment", "calendar", "block", "reserve", "window") >= 3 &&
			!has("unclear", "ambiguity", "clarify", "conflict", "missing", "question") {
			boost -= 30
		}
		if countMatches("critical", "irreversible", "human", "go", "ahead", "approval", "before") >= 3 &&
			!has("unclear", "ambiguity", "clarify", "conflict", "missing", "question") {
			boost -= 28
		}
	}
	if toolName == "lark_calendar_delete" || toolName == "lark_calendar_create" || toolName == "lark_calendar_update" {
		if has("artifact", "artifacts", "manifest", "cleanup", "stale", "obsolete", "legacy") &&
			!has("calendar", "event", "meeting", "schedule") {
			boost -= 10
		}
	}
	if toolName == "file_edit" {
		if has("list", "directory", "folder", "workspace", "metadata", "state", "session", "url", "tab") {
			boost -= 10
		}
		if has("search", "find", "locate", "occurrence", "symbol", "token", "regex", "pattern") && !has("replace", "edit", "modify", "update", "create") {
			boost -= 12
		}
		if has("artifact", "memory", "timer", "reminder") && !has("replace", "edit", "modify", "update", "create") {
			boost -= 10
		}
		if has("attach", "download", "generated", "deliver", "export") && !has("replace", "edit", "modify", "update", "create") {
			boost -= 10
		}
	}
	if toolName == "read_file" {
		if has("inventory", "candidate", "path", "directory", "nested", "root", "roots") && !has("content", "line", "lines", "snippet", "open", "read") {
			boost -= 12
		}
	}
	if toolName == "write_attachment" {
		if countMatches("lark", "thread", "chat", "conversation", "upload") >= 2 {
			boost -= 18
		}
		if has("concise", "chat", "durable", "reusable", "downstream", "audit") &&
			!has("download", "thread", "upload", "attach", "handoff", "receiver") {
			boost -= 16
		}
		if countMatches("artifact", "persist", "store", "report", "bundle", "manifest") >= 2 &&
			!has("attach", "upload", "download", "thread", "chat") {
			boost -= 20
		}
		if countMatches("write", "file", "save", "create") >= 2 &&
			!has("attach", "upload", "download", "thread", "chat") {
			boost -= 12
		}
	}
	if toolName == "lark_upload_file" {
		if countMatches("history", "context", "recent", "before", "conversation", "thread", "chat") >= 3 &&
			!has("upload", "attach", "file", "share", "send") {
			boost -= 26
		}
		if countMatches("send", "message", "status", "announce", "notify") >= 2 &&
			!has("upload", "attach", "file") {
			boost -= 20
		}
		if countMatches("checkin", "encourage", "nudge", "progress", "reminder", "followup") >= 2 &&
			!has("upload", "attach", "file", "download") {
			boost -= 16
		}
		if has("without", "no", "not") && countMatches("upload", "attach", "file") >= 1 {
			boost -= 30
		}
	}
	if toolName == "set_timer" {
		if countMatches("cancel", "remove", "delete", "drop", "prune", "obsolete", "stale", "duplicate", "timer", "reminder") >= 2 {
			boost -= 18
		}
		if countMatches("scheduler", "job", "cron", "cadence", "recurring", "daily", "weekly") >= 2 {
			boost -= 14
		}
		if countMatches("audit", "inspect", "before", "policy", "automation", "automations", "next", "fire", "time", "times") >= 3 &&
			!has("timer", "reminder", "minutes", "hours") {
			boost -= 18
		}
		if has("conflict", "interrupt", "interruption", "boundary") && has("timer", "reminder") {
			boost -= 12
		}
	}
	if toolName == "cancel_timer" {
		if countMatches("list", "active", "remaining", "show", "enumerate", "status") >= 2 &&
			!has("cancel", "remove", "delete") {
			boost -= 16
		}
		if countMatches("scheduler", "job", "recurring", "cadence", "checkin") >= 2 {
			boost -= 16
		}
	}
	if toolName == "search_file" {
		if countMatches("memory", "history", "prior", "note", "notes", "recall", "habit", "preference") >= 2 {
			boost -= 18
		}
		if countMatches("official", "rfc", "spec", "web", "source", "url", "reference") >= 2 {
			boost -= 12
		}
		if countMatches("regex", "needle", "sweep", "repo", "fast", "quick") >= 2 {
			boost -= 10
		}
		if countMatches("motivation", "pattern", "previous", "successful", "memory", "recall") >= 2 {
			boost -= 12
		}
		if countMatches("decision", "policy", "preference", "history", "prior", "memory") >= 3 {
			boost -= 12
		}
		if countMatches("filename", "path", "folder", "directory", "nested", "candidate", "before", "open") >= 3 &&
			!has("content", "snippet", "inside", "within", "line", "lines") {
			boost -= 16
		}
		if countMatches("persona", "soul", "interaction", "tone", "style", "habit", "preference", "memory") >= 3 &&
			!has("file", "repo", "repository", "source", "code", "content", "search") {
			boost -= 18
		}
		if countMatches("memory", "historical", "incident", "signature", "regression", "guardrail", "before", "patch") >= 3 &&
			!has("file", "repo", "repository", "source", "code", "content", "search") {
			boost -= 24
		}
		if countMatches("entrypoint", "entrypoints", "layer", "module", "package", "path", "folder", "directory") >= 3 &&
			!has("inside", "content", "snippet", "line", "lines", "semantic") {
			boost -= 12
		}
	}
	if toolName == "memory_get" {
		if hasAll("before", "offset") && has("known", "exact", "line", "lines") {
			boost -= 18
		}
	}
	if toolName == "browser_screenshot" {
		if countMatches("authoritative", "canonical", "reference", "rfc", "official", "primary") >= 2 &&
			!has("visual", "screenshot", "proof", "ui", "page") {
			boost -= 16
		}
		if countMatches("url", "link", "ticket", "approved", "exact", "single", "source", "ingest") >= 3 &&
			!has("visual", "screenshot", "proof", "ui", "page") {
			boost -= 18
		}
		if countMatches("extract", "text", "content", "from", "url", "single", "exact", "source") >= 4 &&
			!has("visual", "proof", "screenshot", "ui") {
			boost -= 24
		}
		if countMatches("single", "approved", "link", "ingest", "only", "page", "source") >= 3 &&
			!has("visual", "proof", "screenshot", "ui", "capture") {
			boost -= 30
		}
	}
	if toolName == "web_fetch" {
		if countMatches("authoritative", "canonical", "reference", "discover", "shortlist") >= 2 &&
			!has("fixed", "provided", "single", "exact", "url", "source") {
			boost -= 12
		}
		if countMatches("fixed", "provided", "single", "exact", "url", "approved", "ticket", "source", "direct", "ingest") >= 3 {
			boost += 16
		}
		if countMatches("extract", "text", "content", "from", "url", "single", "exact", "source") >= 4 {
			boost += 16
		}
	}
	if toolName == "web_search" {
		if countMatches("exact", "single", "provided", "fixed", "one", "specific") >= 2 &&
			has("url", "page") {
			boost -= 10
		}
		if countMatches("approved", "ticket", "single", "exact", "url", "direct", "ingest") >= 3 {
			boost -= 12
		}
		if countMatches("no", "search", "single", "exact", "url", "source") >= 4 {
			boost -= 22
		}
		if countMatches("only", "source", "url", "avoid", "broad", "search", "discovery", "already", "chosen") >= 4 {
			boost -= 24
		}
	}
	if toolName == "replace_in_file" {
		if countMatches("grep", "log", "logs", "filter", "pattern", "match", "line") >= 2 &&
			!has("replace", "patch", "edit", "modify", "update") {
			boost -= 14
		}
		if countMatches("enumerate", "outputs", "produced", "run", "choose", "files", "release", "reviewer") >= 3 &&
			!has("replace", "patch", "edit", "modify", "update") {
			boost -= 18
		}
		if countMatches("send", "message", "status", "checkpoint", "thread", "chat", "no", "file") >= 4 &&
			!has("replace", "patch", "edit", "modify", "update") {
			boost -= 22
		}
		if countMatches("thread", "status", "ping", "brief", "short", "no", "file", "transfer", "checkpoint") >= 4 &&
			!has("replace", "patch", "edit", "modify", "update") {
			boost -= 30
		}
		if countMatches("list", "directory", "directories", "tree", "workspace", "paths", "inventory") >= 3 &&
			!has("replace", "patch", "edit", "modify", "update") {
			boost -= 22
		}
		if countMatches("list", "directories", "files", "recursively", "before", "choosing", "target", "file") >= 4 &&
			!has("replace", "patch", "edit", "modify", "update") {
			boost -= 32
		}
	}
	if toolName == "browser_action" {
		if countMatches("state", "status", "metadata", "url", "tab", "session", "current", "info") >= 3 &&
			!has("click", "drag", "coordinate", "canvas", "pixel", "tap") {
			boost -= 18
		}
		if countMatches("read", "inspect", "state", "status", "metadata", "without", "interaction") >= 3 &&
			!has("click", "drag", "coordinate", "canvas", "pixel", "tap", "type", "typing", "press") {
			boost -= 28
		}
		if countMatches("artifact", "report", "progress", "progres", "proof", "deliverable", "summary") >= 2 &&
			!has("click", "drag", "coordinate", "canvas", "pixel", "tap", "browser", "dom", "page", "ui") {
			boost -= 16
		}
		if countMatches("momentum", "completed", "progress", "progres", "artifact", "durable", "action", "actions") >= 3 &&
			!has("click", "drag", "coordinate", "canvas", "pixel", "tap", "browser", "dom", "page", "ui") {
			boost -= 30
		}
		if countMatches("memory", "prior", "history", "habit", "persona", "sparse", "fact", "corpus", "note", "notes") >= 3 &&
			!has("click", "drag", "coordinate", "canvas", "pixel", "tap", "browser", "dom", "page", "ui") {
			boost -= 24
		}
	}
	if toolName == "browser_dom" {
		if countMatches("canvas", "coordinate", "pixel", "drag", "offset") >= 2 {
			boost -= 12
		}
	}
	if toolName == "artifacts_delete" {
		if countMatches("scheduler", "job", "cron", "cadence", "run") >= 2 &&
			!has("artifact", "artifacts", "manifest", "bundle") {
			boost -= 14
		}
		if countMatches("legacy", "recurring", "automation", "automations", "job", "jobs", "cadence", "deprecation", "retired") >= 3 &&
			!has("artifact", "artifacts", "manifest", "bundle", "output", "report") {
			boost -= 20
		}
	}
	if toolName == "write_file" {
		if countMatches("enumerate", "list", "inspect", "audit", "show", "state", "current", "existing") >= 3 &&
			!has("write", "create", "new", "save", "draft", "record") {
			boost -= 20
		}
		if countMatches("scheduler", "recurring", "cadence", "jobs", "automation", "automations") >= 2 &&
			!has("write", "create", "new", "save", "draft", "record", "runbook") {
			boost -= 24
		}
		if countMatches("inspect", "current", "schedule", "state", "before", "change", "mutation", "frozen") >= 3 &&
			!has("write", "create", "new", "save", "draft", "record", "runbook") {
			boost -= 26
		}
	}
	if toolName == "search_file" {
		if countMatches("directory", "name", "constraint", "constraints", "before", "open") >= 3 &&
			!has("content", "snippet", "inside", "within", "line", "lines") {
			boost -= 14
		}
		if countMatches("entrypoint", "entrypoints", "layer", "module", "package", "path", "folder", "directory") >= 3 &&
			!has("inside", "content", "snippet", "line", "lines", "semantic") {
			boost -= 24
		}
		if countMatches("path", "topology", "directory", "folder", "narrow", "first", "before", "reading") >= 3 &&
			!has("content", "inside", "snippet", "semantic", "line", "lines") {
			boost -= 16
		}
	}
	if toolName == "scheduler_create_job" {
		if countMatches("violates", "policy", "remove", "removed", "not", "recreated", "recreate", "cadence", "recurring") >= 4 {
			boost -= 24
		}
		if countMatches("violat", "policy", "remove", "not", "recreat", "cadence", "recurring") >= 4 {
			boost -= 24
		}
	}
	if toolName == "lark_calendar_query" || toolName == "lark_calendar_update" || toolName == "lark_calendar_create" {
		if countMatches("scheduler", "recurring", "automation", "automations", "job", "jobs", "cadence") >= 2 &&
			!has("calendar", "event", "meeting") {
			boost -= 20
		}
		if countMatches("violat", "policy", "remove", "not", "recreat", "cadence", "recurring") >= 4 &&
			!has("calendar", "event", "meeting") {
			boost -= 24
		}
		if countMatches("legacy", "recurring", "automation", "retire", "retired", "obsolete", "schedule", "remove") >= 3 &&
			!has("calendar", "event", "meeting") {
			boost -= 28
		}
	}
	if toolName == "video_generate" {
		if countMatches("scheduler", "recurring", "automation", "automations", "job", "jobs", "cadence", "state", "inspect", "audit") >= 3 {
			boost -= 32
		}
		if countMatches("enumerate", "outputs", "produced", "run", "choose", "files", "release", "reviewer") >= 3 {
			boost -= 26
		}
	}
	if toolName == "lark_task_manage" {
		if countMatches("plan", "roadmap", "phase", "milestone", "strategy") >= 2 &&
			!has("task", "owner", "due", "todo", "assign") {
			boost -= 18
		}
		if countMatches("consent", "approval", "confirm", "external", "outreach", "sensitive") >= 2 &&
			!has("task", "owner", "due", "todo", "assign", "batch", "update") {
			boost -= 16
		}
		if countMatches("plan", "steps", "before", "mutation", "change", "apply", "fix", "execution") >= 3 &&
			!has("task", "owner", "due", "todo", "assign", "batch", "update") {
			boost -= 16
		}
		if countMatches("phased", "checklist", "rollback", "checkpoint", "reproduce", "patch", "verify", "gates") >= 3 &&
			!has("task", "owner", "due", "todo", "assign", "batch", "update") {
			boost -= 20
		}
		if countMatches("before", "task", "updates", "rollout", "phased", "milestones", "risk", "checkpoints") >= 4 &&
			!has("task", "owner", "due", "todo", "assign", "batch", "update", "manage", "ticket") {
			boost -= 22
		}
		if countMatches("release", "checklist", "milestones", "rollback", "checkpoint", "checkpoints") >= 3 &&
			!has("task", "owner", "due", "todo", "assign", "batch", "update", "manage", "ticket") {
			boost -= 30
		}
		if countMatches("operationalize", "tracked", "task", "item", "commitment", "work", "ticket") >= 3 {
			boost += 22
		}
	}
	if toolName == "lark_send_message" {
		if countMatches("file", "report", "package", "upload", "in", "thread") >= 3 &&
			has("without", "plain", "status") {
			boost -= 18
		}
		if countMatches("text", "status", "message", "checkpoint", "without", "file", "upload", "attachment") >= 4 {
			boost += 20
		}
		if countMatches("no", "file", "no", "upload", "status", "thread", "message") >= 4 {
			boost += 24
		}
		if countMatches("brief", "textual", "checkpoint", "status", "forbid", "forbids", "without", "upload", "file") >= 4 {
			boost += 34
		}
		if countMatches("avoid", "file", "transfer", "compact", "progress", "update", "thread") >= 4 {
			boost += 26
		}
		if countMatches("prior", "chat", "context", "thread", "history", "before", "replying") >= 3 &&
			!has("send", "message", "update", "status", "notify", "broadcast") {
			boost -= 30
		}
	}
	if toolName == "lark_upload_file" {
		if countMatches("review", "cannot", "proceed", "without", "generated", "report", "file", "thread", "deliver", "package") >= 4 {
			boost += 20
		}
		if countMatches("artifact", "artifacts", "report", "reusable", "downstream", "full", "deep", "dive") >= 3 &&
			!has("upload", "attach", "thread", "chat", "lark", "conversation") {
			boost -= 26
		}
		if countMatches("no", "file", "no", "upload", "status", "thread", "message") >= 4 {
			boost -= 38
		}
		if countMatches("brief", "textual", "checkpoint", "status", "forbid", "forbids", "without", "upload", "file") >= 4 {
			boost -= 56
		}
		if countMatches("avoid", "file", "transfer", "compact", "progress", "update", "thread") >= 4 {
			boost -= 30
		}
	}
	if toolName == "artifacts_write" {
		if countMatches("artifacts_write", "artifact", "artifacts", "reusable", "downstream", "full", "report", "deep", "dive") >= 3 {
			boost += 20
		}
		if countMatches("list", "enumerate", "inventory", "existing", "latest", "valid", "release", "share", "outputs", "artifacts") >= 4 &&
			!has("write", "create", "save", "new", "persist") {
			boost -= 32
		}
	}
	if toolName == "clarify" {
		if countMatches("memory_get", "selected", "memory", "note", "open", "detail", "detailed", "guidance", "context") >= 3 &&
			!has("unclear", "ambiguity", "clarify", "conflict", "missing") {
			boost -= 24
		}
		if countMatches("memory_search", "memory_get", "before", "action", "retrieve", "recall", "history") >= 3 &&
			!has("unclear", "ambiguity", "clarify", "conflict", "missing") {
			boost -= 18
		}
	}
	if toolName == "memory_search" {
		if has("clarify") && has("conflict", "unclear", "ambiguity", "latest", "preference") {
			boost -= 18
		}
	}
	if toolName == "search_file" {
		if countMatches("find", "read_file", "ripgrep", "replace_in_file", "memory_search", "memory_get", "list_dir", "a2ui_emit", "artifacts_write") >= 2 &&
			!has("content", "inside", "semantic", "snippet", "line", "lines") {
			boost -= 16
		}
	}
	if toolName == "find" {
		if has("search_file") {
			boost -= 14
		}
		if hasAll("find", "read_file") {
			boost += 14
		}
		if countMatches("path", "topology", "directory", "folder", "narrow", "first", "before", "reading", "open") >= 3 {
			boost += 20
		}
	}
	if toolName == "read_file" {
		if hasAll("find", "read_file") {
			boost += 14
		}
		if has("find") && has("ordered", "events", "sequential") {
			boost += 10
		}
		if countMatches("path", "topology", "directory", "folder", "narrow", "first", "before", "reading", "open") >= 3 {
			boost -= 12
		}
	}
	if toolName == "replace_in_file" {
		if countMatches("new", "file", "not", "place", "inplace", "materialize", "generated") >= 4 {
			boost -= 30
		}
		if has("write_file") && !has("replace_in_file", "patch", "replace") {
			boost -= 16
		}
		if countMatches("multihop", "reference", "references", "chain", "authoritative", "statement", "resolve", "across", "files") >= 3 &&
			!has("replace", "patch", "edit", "modify", "update", "fix") {
			boost -= 28
		}
	}
	if toolName == "write_file" {
		if has("write_file") {
			boost += 16
		}
		if countMatches("new", "file", "not", "place", "inplace", "materialize", "generated") >= 4 {
			boost += 18
		}
	}
	if toolName == "write_attachment" {
		if countMatches("a2ui_emit", "artifacts_write") >= 2 && !has("downloadable", "handoff", "receiver", "attachment") {
			boost -= 18
		}
		if countMatches("replace_in_file", "artifacts_write") >= 2 && !has("downloadable", "handoff", "receiver", "attachment") {
			boost -= 16
		}
	}
	if toolName == "lark_calendar_query" {
		if countMatches("create", "new", "add", "schedule", "meeting", "meetings", "events") >= 3 {
			boost -= 20
		}
	}
	if toolName == "lark_task_manage" {
		if has("request_user") || countMatches("approval", "confirm", "consent", "manual", "before") >= 3 {
			boost -= 18
		}
	}
	if toolName == "request_user" {
		if has("request_user") {
			boost += 24
		}
	}
	if toolName == "shell_exec" {
		if has("grep") && !has("script", "compute", "calculate", "python", "snippet") {
			boost += 12
		}
		if countMatches("run", "shell", "verification", "check", "checks", "after", "code", "change", "changes") >= 4 {
			boost += 18
		}
	}
	if toolName == "execute_code" {
		if has("grep") && !has("python", "script", "snippet", "compute", "calculate") {
			boost -= 14
		}
		if countMatches("run", "shell", "verification", "check", "checks", "after", "code", "change", "changes") >= 4 {
			boost -= 18
		}
		if countMatches("reproduce", "failure", "failing", "test", "command", "before", "fix") >= 3 {
			boost -= 18
		}
	}
	if toolName == "plan" {
		if countMatches("thread", "checkpoint", "message", "status", "textual", "short", "in", "stage", "reviewer") >= 3 &&
			!has("milestone", "roadmap", "rollback", "phase", "phased", "strategy", "timeline") {
			boost -= 24
		}
		if countMatches("calendar", "event", "meeting", "update", "shift") >= 3 &&
			!has("milestone", "roadmap", "rollback", "phase", "phased", "strategy", "timeline") {
			boost -= 28
		}
		if countMatches("multistep", "planning", "rollback", "checkpoints", "before", "execution") >= 3 &&
			!has("ui", "browser", "click", "drag", "canvas") {
			boost += 14
		}
		if countMatches("legacy", "recurring", "automation", "retired", "policy", "remove", "obsolete", "schedule") >= 3 &&
			!has("milestone", "roadmap", "rollback", "phase", "phased", "strategy", "timeline") {
			boost -= 30
		}
	}
	if toolName == "browser_action" {
		if countMatches("multistep", "planning", "rollback", "checkpoints", "before", "execution") >= 3 &&
			!has("ui", "browser", "click", "drag", "canvas", "coordinate", "pixel") {
			boost -= 24
		}
	}
	if toolName == "lark_calendar_update" {
		if countMatches("update", "existing", "calendar", "event", "meeting", "shift", "minutes", "day", "timeline") >= 3 {
			boost += 20
		}
		if countMatches("register", "cadence", "identifier", "scheduler", "job", "recurring") >= 3 &&
			!has("calendar", "event", "meeting") {
			boost -= 20
		}
	}
	if toolName == "scheduler_list_jobs" {
		if countMatches("show", "current", "currently", "registered", "jobs", "execution", "cadence", "list") >= 3 {
			boost += 18
		}
	}
	if toolName == "scheduler_create_job" {
		if countMatches("show", "current", "currently", "registered", "jobs", "execution", "cadence", "list") >= 3 &&
			!has("create", "new", "add") {
			boost -= 22
		}
	}
	if toolName == "request_user" {
		if countMatches("manual", "user", "confirmation", "before", "continuing", "continue", "cutover", "production") >= 4 {
			boost += 18
		}
	}
	if toolName == "write_attachment" {
		if countMatches("downloadable", "handoff", "receiver", "immediate", "deliver") >= 2 {
			boost += 28
		}
		if has("not") && countMatches("background", "storage", "persist", "persistence", "only") >= 2 {
			boost += 18
		}
		if countMatches("workspace", "local", "markdown", "file", "not", "attachment") >= 4 {
			boost -= 44
		}
		if has("not", "no", "without") && has("attachment", "attach") {
			boost -= 40
		}
	}
	if toolName == "write_file" {
		if countMatches("workspace", "local", "markdown", "file", "not", "attachment") >= 4 {
			boost += 20
		}
	}
	if toolName == "scheduler_create_job" {
		if countMatches("remove", "delete", "obsolete", "legacy", "retired", "deprecation", "old") >= 2 {
			boost -= 18
		}
		if countMatches("audit", "inspect", "existing", "current", "before", "change", "policy", "cadence") >= 3 &&
			!has("create", "new", "add") {
			boost -= 14
		}
	}
	if toolName == "scheduler_list_jobs" {
		if countMatches("remove", "delete", "obsolete", "legacy", "retired", "deprecation") >= 2 &&
			!has("list", "show", "inspect", "audit", "current", "existing", "before") {
			boost -= 10
		}
	}
	if toolName == "cancel_timer" {
		if countMatches("calendar", "event", "meeting") >= 2 &&
			!has("timer", "reminder", "alarm") {
			boost -= 20
		}
	}
	if toolName == "request_user" {
		if countMatches("requires", "require", "user", "approval", "confirm", "before", "publish", "continue") >= 4 {
			boost += 16
		}
	}
	if toolName == "clarify" {
		if countMatches("requires", "require", "user", "approval", "confirm", "before", "publish", "continue") >= 4 {
			boost -= 20
		}
	}
	if hasAll("task", "delegate") && toolName == "acp_executor" {
		boost += 8
	}
	if toolName == "acp_executor" {
		if countMatches("delegate", "executor", "subagent", "parallel", "heavy", "long", "deep") >= 2 {
			boost += 16
		}
	}
	return boost
}

func topToolMatches(matches []FoundationToolMatch, topK int) []FoundationToolMatch {
	if topK <= 0 || len(matches) == 0 {
		return nil
	}
	if topK > len(matches) {
		topK = len(matches)
	}
	return append([]FoundationToolMatch(nil), matches[:topK]...)
}

func buildFoundationRecommendations(prompt FoundationPromptSummary, tools FoundationToolSummary, implicit FoundationImplicitSummary) []string {
	recs := make([]string, 0, 9)
	if prompt.AverageScore < 80 {
		recs = append(recs, "Prompt: add explicit 'tool selection strategy' language for implicit tasks (e.g. map user intent to tool category before acting).")
	}
	if tools.AverageUsability < 85 {
		recs = append(recs, "Tool schema: enrich thin parameter descriptions and ensure required keys map to real properties.")
	}
	if tools.AverageDiscoverability < 80 {
		recs = append(recs, "Tool discoverability: improve descriptions with action verbs + domain nouns (who/what/when output) to reduce ambiguity.")
	}
	if implicit.PassAt5Rate < 0.85 {
		recs = append(recs, "Implicit tool use: add intent aliases/synonyms to tool descriptions for weakly matched categories.")
	}
	if implicit.PassAt1Rate < 0.7 {
		recs = append(recs, "Ranking precision: clarify overlapping tools by emphasizing differentiators (selector vs coordinate, search vs fetch, etc.).")
	}
	if availabilityErrors := countFailureType(implicit.CaseResults, "availability_error"); availabilityErrors > 0 {
		recs = append(recs, fmt.Sprintf("Tool availability: %d cases still reference unavailable tools; complete registration/visibility parity before rerunning foundation eval.", availabilityErrors))
	}

	commonIssues := topIssues(tools.IssueBreakdown, 3)
	for _, issue := range commonIssues {
		switch issue {
		case "property_description_thin":
			recs = append(recs, "Top issue: parameter descriptions are thin; add concise constraints/examples for each key parameter.")
		case "metadata_tags_missing":
			recs = append(recs, "Top issue: missing tags; populate tags to improve routeability and discoverability scoring.")
		case "semantic_tokens_sparse":
			recs = append(recs, "Top issue: sparse semantic vocabulary; expand descriptions with domain-specific terms users naturally use.")
		}
	}

	if len(recs) == 0 {
		recs = append(recs, "Baseline quality is stable; next step is raising pass@1 precision on ambiguous intents.")
	}
	return uniqueNonEmptyStrings(recs)
}

func countFailureType(results []FoundationCaseResult, failureType string) int {
	if failureType == "" {
		return 0
	}
	count := 0
	for _, result := range results {
		if result.FailureType == failureType {
			count++
		}
	}
	return count
}

func topIssues(freq map[string]int, limit int) []string {
	if len(freq) == 0 || limit <= 0 {
		return nil
	}
	type kv struct {
		k string
		v int
	}
	pairs := make([]kv, 0, len(freq))
	for k, v := range freq {
		pairs = append(pairs, kv{k: k, v: v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].v == pairs[j].v {
			return pairs[i].k < pairs[j].k
		}
		return pairs[i].v > pairs[j].v
	})
	if len(pairs) > limit {
		pairs = pairs[:limit]
	}
	res := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		res = append(res, pair.k)
	}
	return res
}

func containsAnySubstring(value string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(value, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

var verbTokens = map[string]struct{}{
	"read": {}, "write": {}, "edit": {}, "update": {}, "delete": {}, "remove": {},
	"list": {}, "find": {}, "search": {}, "fetch": {}, "query": {}, "create": {},
	"render": {}, "send": {}, "upload": {}, "execute": {}, "plan": {}, "clarify": {},
	"cancel": {}, "manage": {}, "set": {}, "play": {},
}

func containsVerb(value string) bool {
	for _, token := range tokenize(value) {
		if _, ok := verbTokens[normalizeToken(token)]; ok {
			return true
		}
	}
	return false
}

func tokenRichness(weights map[string]float64) int {
	count := 0
	for token, weight := range weights {
		if token == "" || weight <= 0 {
			continue
		}
		count++
	}
	return count
}

func addTokenWeights(dst map[string]float64, tokens []string, weight float64) {
	for _, token := range tokens {
		norm := normalizeToken(token)
		if norm == "" {
			continue
		}
		dst[norm] += weight
	}
}

var stopwords = map[string]struct{}{
	"the": {}, "a": {}, "an": {}, "and": {}, "or": {}, "to": {}, "of": {}, "for": {}, "in": {},
	"on": {}, "with": {}, "from": {}, "by": {}, "is": {}, "are": {}, "this": {}, "that": {},
	"it": {}, "as": {}, "be": {}, "at": {}, "into": {}, "under": {}, "all": {}, "current": {},
	"need": {}, "needs": {}, "your": {}, "their": {}, "our": {}, "can": {}, "should": {},
}

var tokenAliases = map[string]string{
	"locate":        "search",
	"lookup":        "search",
	"scan":          "search",
	"check":         "query",
	"inspect":       "query",
	"inspection":    "query",
	"view":          "query",
	"upcoming":      "query",
	"querying":      "query",
	"queries":       "query",
	"docs":          "doc",
	"documentation": "doc",
	"repository":    "repo",
	"codebase":      "repo",
	"repos":         "repo",
	"files":         "file",
	"filename":      "name",
	"filenames":     "name",
	"names":         "name",
	"folders":       "directory",
	"folder":        "directory",
	"dirs":          "directory",
	"url":           "web",
	"website":       "web",
	"webpage":       "web",
	"internet":      "web",
	"note":          "write",
	"notes":         "write",
	"symbol":        "search",
	"symbols":       "search",
	"occurrence":    "search",
	"occurrences":   "search",
	"workspace":     "directory",
	"project":       "repo",
	"projects":      "repo",
	"events":        "event",
	"logs":          "log",
	"calendar":      "event",
	"meetings":      "event",
	"uploading":     "upload",
	"uploaded":      "upload",
	"generate":      "artifact",
	"generated":     "artifact",
	"attachments":   "attach",
	"attachment":    "attach",
	"downloadable":  "attach",
	"creating":      "create",
	"created":       "create",
	"updating":      "update",
	"updated":       "update",
	"deleting":      "delete",
	"deleted":       "delete",
	"deprecated":    "replace",
	"endpoint":      "replace",
	"rendering":     "render",
	"executing":     "execute",
	"execution":     "execute",
	"planning":      "plan",
	"clarification": "clarify",
	"ambiguous":     "clarify",
	"ambiguity":     "clarify",
	"conflict":      "clarify",
	"blocking":      "clarify",
	"block":         "clarify",
	"missing":       "clarify",
	"interruptions": "interrupt",
	"phase":         "plan",
	"phased":        "plan",
	"milestone":     "plan",
	"checkpoint":    "plan",
	"risk":          "plan",
	"selector":      "dom",
	"selectors":     "dom",
	"submit":        "dom",
	"form":          "dom",
	"payload":       "a2ui",
	"renderer":      "a2ui",
	"protocol":      "a2ui",
	"structured":    "a2ui",
	"messages":      "message",
	"tasks":         "task",
	"objective":     "okr",
	"objectives":    "okr",
	"kr":            "result",
	"krs":           "result",
	"timers":        "timer",
	"jobs":          "job",
	"reminder":      "timer",
	"reminders":     "timer",
	"checkin":       "job",
	"follow-up":     "followup",
	"followup":      "job",
	"markdown":      "report",
	"reporting":     "report",
	"reports":       "report",
	"authoritative": "official",
	"primary":       "official",
	"canonical":     "official",
	"trusted":       "official",
	"references":    "reference",
	"discover":      "search",
	"shortlist":     "search",
	"greenlight":    "approval",
	"freeze":        "wait",
	"frozen":        "wait",
	"silence":       "wait",
	"reconstruct":   "history",
	"recurrence":    "recurring",
	"recurrences":   "recurring",
	"queued":        "queue",
	"nudge":         "reminder",
	"withdraw":      "cancel",
	"sunset":        "retire",
	"standing":      "recurring",
	"playbook":      "pattern",
	"lineage":       "manifest",
	"ingest":        "fetch",
	"sandbox":       "shell",
	"sandboxed":     "shell",
	"terminal":      "shell",
	"bash":          "shell",
	"fixed":         "exact",
	"provided":      "exact",
	"pinned":        "exact",
	"inventory":     "list",
	"sensitive":     "consent",
	"private":       "consent",
	"personal":      "consent",
	"nested":        "directory",
	"roots":         "directory",
	"candidate":     "directory",
	"offsets":       "offset",
	"known":         "exact",
	"reusable":      "artifact",
	"durable":       "artifact",
	"downstream":    "artifact",
}

func tokenize(value string) []string {
	value = strings.ToLower(value)
	tokens := make([]string, 0, 24)
	var sb strings.Builder
	flush := func() {
		if sb.Len() == 0 {
			return
		}
		token := sb.String()
		sb.Reset()
		if token == "" {
			return
		}
		tokens = append(tokens, token)
	}

	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			sb.WriteRune(r)
			continue
		}
		flush()
	}
	flush()

	result := make([]string, 0, len(tokens)*2)
	for _, token := range tokens {
		if strings.Contains(token, "_") {
			parts := strings.Split(token, "_")
			for _, part := range parts {
				if strings.TrimSpace(part) != "" {
					result = append(result, part)
				}
			}
		}
		result = append(result, token)
	}

	return result
}

func normalizeToken(token string) string {
	token = strings.ToLower(strings.TrimSpace(token))
	token = strings.Trim(token, "_")
	if token == "" {
		return ""
	}
	if _, skip := stopwords[token]; skip {
		return ""
	}
	if alias, ok := tokenAliases[token]; ok {
		token = alias
	}
	if strings.HasSuffix(token, "ing") && len(token) > 5 {
		token = strings.TrimSuffix(token, "ing")
	}
	if strings.HasSuffix(token, "ed") && len(token) > 4 {
		token = strings.TrimSuffix(token, "ed")
	}
	if strings.HasSuffix(token, "s") && len(token) > 4 {
		token = strings.TrimSuffix(token, "s")
	}
	if alias, ok := tokenAliases[token]; ok {
		token = alias
	}
	if _, skip := stopwords[token]; skip {
		return ""
	}
	if len(token) < 2 {
		return ""
	}
	return token
}

func uniqueNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	uniq := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		uniq = append(uniq, value)
	}
	if len(uniq) == 0 {
		return nil
	}
	return uniq
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func percentileFloat(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if percentile <= 0 {
		return values[0]
	}
	if percentile >= 100 {
		return values[len(values)-1]
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	rank := (percentile / 100.0) * float64(len(sorted)-1)
	low := int(math.Floor(rank))
	high := int(math.Ceil(rank))
	if low == high {
		return sorted[low]
	}
	weight := rank - float64(low)
	return sorted[low]*(1-weight) + sorted[high]*weight
}

func writeFoundationArtifacts(result *FoundationEvaluationResult, outputDir, format string) ([]EvaluationArtifact, error) {
	cleanedOutputDir, err := sanitizeOutputPath(defaultOutputBaseDir, outputDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cleanedOutputDir, 0755); err != nil {
		return nil, fmt.Errorf("create foundation output dir: %w", err)
	}

	artifacts := make([]EvaluationArtifact, 0, 2)

	jsonPath := filepath.Join(cleanedOutputDir, fmt.Sprintf("foundation_result_%s.json", result.RunID))
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal foundation result: %w", err)
	}
	if err := os.WriteFile(jsonPath, data, 0644); err != nil {
		return nil, fmt.Errorf("write foundation json: %w", err)
	}
	artifacts = append(artifacts, EvaluationArtifact{
		Type:   "foundation_result",
		Format: "json",
		Name:   filepath.Base(jsonPath),
		Path:   jsonPath,
	})

	if strings.EqualFold(strings.TrimSpace(format), "json") {
		return artifacts, nil
	}

	mdPath := filepath.Join(cleanedOutputDir, fmt.Sprintf("foundation_report_%s.md", result.RunID))
	if err := os.WriteFile(mdPath, []byte(buildFoundationMarkdownReport(result)), 0644); err != nil {
		return nil, fmt.Errorf("write foundation markdown: %w", err)
	}
	artifacts = append(artifacts, EvaluationArtifact{
		Type:   "foundation_report",
		Format: "markdown",
		Name:   filepath.Base(mdPath),
		Path:   mdPath,
	})

	return artifacts, nil
}
