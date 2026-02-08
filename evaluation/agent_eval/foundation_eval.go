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
	"alex/internal/shared/agent/presets"

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
	TotalCases  int                    `json:"total_cases"`
	PassedCases int                    `json:"passed_cases"`
	FailedCases int                    `json:"failed_cases"`
	Top1HitRate float64                `json:"top1_hit_rate"`
	TopKHitRate float64                `json:"topk_hit_rate"`
	MRR         float64                `json:"mrr"`
	CaseResults []FoundationCaseResult `json:"case_results"`
}

// FoundationCaseResult captures one implicit-intent scenario result.
type FoundationCaseResult struct {
	ID            string                `json:"id"`
	Category      string                `json:"category"`
	Intent        string                `json:"intent"`
	ExpectedTools []string              `json:"expected_tools"`
	TopMatches    []FoundationToolMatch `json:"top_matches"`
	HitRank       int                   `json:"hit_rank"`
	Passed        bool                  `json:"passed"`
	FailureType   string                `json:"failure_type,omitempty"`
	Reason        string                `json:"reason"`
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
	ID            string   `yaml:"id"`
	Category      string   `yaml:"category"`
	Intent        string   `yaml:"intent"`
	ExpectedTools []string `yaml:"expected_tools"`
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
			0.25*implicitSummary.TopKHitRate,
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
- When producing long-form deliverables (reports, articles, specs), write them to a Markdown file via file_write.
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
	results := make([]FoundationCaseResult, 0, len(scenarios))
	profilesByName := make(map[string]foundationToolProfile, len(profiles))
	for _, profile := range profiles {
		profilesByName[profile.Definition.Name] = profile
	}

	top1Hits := 0
	topKHits := 0
	mrr := 0.0

	for _, scenario := range scenarios {
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

		topMatches := topToolMatches(ranked, topK)
		passed := hitRank > 0 && hitRank <= topK
		if hitRank == 1 {
			top1Hits++
		}
		if passed {
			topKHits++
		}
		if hitRank > 0 {
			mrr += 1.0 / float64(hitRank)
		}

		failureType := ""
		reason := ""
		if len(expectedAvailable) == 0 {
			failureType = "availability_error"
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

		results = append(results, FoundationCaseResult{
			ID:            scenario.ID,
			Category:      scenario.Category,
			Intent:        scenario.Intent,
			ExpectedTools: append([]string(nil), scenario.ExpectedTools...),
			TopMatches:    topMatches,
			HitRank:       hitRank,
			Passed:        passed,
			FailureType:   failureType,
			Reason:        strings.TrimSpace(reason),
		})
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
	failed := total - topKHits

	return FoundationImplicitSummary{
		TotalCases:  total,
		PassedCases: topKHits,
		FailedCases: failed,
		Top1HitRate: round3(float64(top1Hits) / float64(total)),
		TopKHitRate: round3(float64(topKHits) / float64(total)),
		MRR:         round3(mrr / float64(total)),
		CaseResults: results,
	}
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
	case "read_file":
		if countMatches("read", "open", "inspect", "view") >= 1 &&
			countMatches("source", "workspace", "file", "content", "line") >= 1 {
			boost += 18
		}
	case "clarify":
		if has("ambiguity", "clarify", "blocking", "requirement", "missing", "unclear", "constraint") {
			boost += 14
		}
	case "web_search":
		if countMatches("search", "lookup", "find", "query", "compare") >= 1 &&
			countMatches("web", "internet", "doc", "reference", "official", "site", "url") >= 1 {
			boost += 14
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
	case "browser_screenshot":
		if has("capture", "screenshot", "proof", "visual", "evidence", "page") {
			boost += 14
		}
	case "write_file":
		if countMatches("write", "create", "new", "save") >= 1 &&
			countMatches("file", "markdown", "note", "report", "content") >= 1 {
			boost += 18
		}
	case "list_dir":
		if countMatches("list", "show", "enumerate", "browse", "tree") >= 1 &&
			countMatches("directory", "folder", "workspace", "path") >= 1 {
			boost += 18
		}
	case "search_file":
		if countMatches("search", "find", "locate", "occurrence", "symbol", "token", "regex", "pattern") >= 1 &&
			countMatches("file", "project", "repo", "source", "code", "across") >= 1 {
			boost += 18
		}
	case "replace_in_file":
		if has("replace", "deprecated", "endpoint", "api", "path", "file", "update") {
			boost += 16
		}
	case "artifact_manifest":
		if countMatches("manifest", "metadata", "generated", "describe", "artifact") >= 2 {
			boost += 22
		}
	case "artifacts_write":
		if countMatches("artifact", "report", "persist", "save", "write", "reference", "final", "output") >= 2 {
			boost += 20
		}
	case "artifacts_list":
		if countMatches("list", "enumerate", "index", "show", "generated", "artifact") >= 2 {
			boost += 10
		}
	case "memory_search":
		if countMatches("memory", "prior", "history", "decision", "note", "context", "summary", "recall") >= 2 {
			boost += 20
		}
	case "list_timers":
		if countMatches("timer", "timers", "reminder", "reminders", "remaining", "active", "schedule") >= 2 {
			boost += 20
		}
	case "a2ui_emit":
		if has("payload", "renderer", "render", "ui", "protocol", "structured") {
			boost += 12
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
	if toolName == "file_edit" {
		if has("list", "directory", "folder", "workspace", "metadata", "state", "session", "url", "tab") {
			boost -= 8
		}
		if has("search", "find", "locate", "occurrence", "symbol", "token", "regex", "pattern") && !has("replace", "edit", "modify", "update", "create") {
			boost -= 8
		}
		if has("artifact", "memory", "timer", "reminder") && !has("replace", "edit", "modify", "update", "create") {
			boost -= 6
		}
	}
	if hasAll("task", "delegate") && toolName == "acp_executor" {
		boost += 8
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
	if implicit.TopKHitRate < 0.85 {
		recs = append(recs, "Implicit tool use: add intent aliases/synonyms to tool descriptions for weakly matched categories.")
	}
	if implicit.Top1HitRate < 0.7 {
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
		recs = append(recs, "Baseline quality is stable; next step is raising Top-1 precision on ambiguous intents.")
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
	"querying":      "query",
	"queries":       "query",
	"docs":          "doc",
	"documentation": "doc",
	"repository":    "repo",
	"codebase":      "repo",
	"repos":         "repo",
	"files":         "file",
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
	"calendar":      "event",
	"meetings":      "event",
	"uploading":     "upload",
	"uploaded":      "upload",
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
	"blocking":      "clarify",
	"block":         "clarify",
	"missing":       "clarify",
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
	"timers":        "timer",
	"jobs":          "job",
	"reminder":      "timer",
	"reminders":     "timer",
	"markdown":      "report",
	"reporting":     "report",
	"reports":       "report",
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
