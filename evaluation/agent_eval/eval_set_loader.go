package agent_eval

import (
	"fmt"
	"os"
	"strings"

	"alex/evaluation/swe_bench"

	"gopkg.in/yaml.v3"
)

// EvalSetDefinition is the YAML-backed description for constructing an eval set
// from a source dataset plus filters and composition rules.
type EvalSetDefinition struct {
	Version          string            `yaml:"version"`
	Name             string            `yaml:"name"`
	Description      string            `yaml:"description,omitempty"`
	Mode             string            `yaml:"mode,omitempty"` // baseline | challenge
	Dataset          EvalSetDataset    `yaml:"dataset"`
	Filters          EvalSetFilters    `yaml:"filters,omitempty"`
	CompositionRules []CompositionRule `yaml:"composition_rules,omitempty"`
	RubricPath       string            `yaml:"rubric_path,omitempty"`
}

// EvalSetDataset describes the source dataset to build the eval set from.
type EvalSetDataset struct {
	Type  string `yaml:"type"`
	Path  string `yaml:"path,omitempty"`
	Limit int    `yaml:"limit,omitempty"`
}

// EvalSetFilters limits which tasks enter the eval set.
type EvalSetFilters struct {
	Difficulty []DifficultyTier `yaml:"difficulty,omitempty"`
	Domains    []EvalDomain     `yaml:"domains,omitempty"`
	Tags       []string         `yaml:"tags,omitempty"`
}

// LoadEvalSetDefinition reads and validates an eval set definition from a YAML file.
func LoadEvalSetDefinition(path string) (*EvalSetDefinition, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("eval set path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read eval set definition: %w", err)
	}

	var def EvalSetDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("decode eval set definition: %w", err)
	}

	if strings.TrimSpace(def.Name) == "" {
		return nil, fmt.Errorf("eval set name is required")
	}
	if strings.TrimSpace(def.Version) == "" {
		return nil, fmt.Errorf("eval set version is required")
	}

	return &def, nil
}

// BuildEvalSetFromDefinition constructs an EvalSet and matching instances for
// evaluation runs from a definition file.
func BuildEvalSetFromDefinition(def *EvalSetDefinition) (*EvalSet, []swe_bench.Instance, error) {
	if def == nil {
		return nil, nil, fmt.Errorf("eval set definition is nil")
	}

	datasetType := strings.TrimSpace(def.Dataset.Type)
	if datasetType == "" {
		datasetType = "general_agent"
	}

	switch datasetType {
	case "general_agent":
		tasks, err := loadGeneralAgentTasks(def.Dataset.Path, def.Dataset.Limit)
		if err != nil {
			return nil, nil, err
		}

		filtered := filterGeneralAgentTasks(tasks, def.Filters)
		builder := NewEvalSetBuilder(def.Name, def.Version).WithDescription(def.Description)
		builder.AddTasks(convertGeneralTasks(filtered))
		for _, rule := range def.CompositionRules {
			builder.WithCompositionRule(rule)
		}

		set, err := builder.Build()
		if err != nil {
			return nil, nil, err
		}

		if violations := ValidateComposition(set); len(violations) > 0 {
			return nil, nil, fmt.Errorf("eval set composition violations: %s", strings.Join(violations, "; "))
		}

		instances := make([]swe_bench.Instance, 0, len(filtered))
		for _, task := range filtered {
			instances = append(instances, task.toInstance())
		}

		return set, instances, nil
	default:
		return nil, nil, fmt.Errorf("unsupported eval set dataset type: %s", datasetType)
	}
}

func filterGeneralAgentTasks(tasks []GeneralAgentTask, filters EvalSetFilters) []GeneralAgentTask {
	if len(filters.Difficulty) == 0 && len(filters.Domains) == 0 && len(filters.Tags) == 0 {
		return tasks
	}

	diffSet := make(map[DifficultyTier]struct{}, len(filters.Difficulty))
	for _, d := range filters.Difficulty {
		diffSet[d] = struct{}{}
	}
	domainSet := make(map[EvalDomain]struct{}, len(filters.Domains))
	for _, d := range filters.Domains {
		domainSet[d] = struct{}{}
	}
	tagSet := make(map[string]struct{}, len(filters.Tags))
	for _, tag := range filters.Tags {
		tagSet[tag] = struct{}{}
	}

	filtered := make([]GeneralAgentTask, 0, len(tasks))
	for _, task := range tasks {
		if len(diffSet) > 0 {
			if _, ok := diffSet[DifficultyTier(task.Difficulty)]; !ok {
				continue
			}
		}
		if len(domainSet) > 0 {
			if _, ok := domainSet[EvalDomain(task.Domain)]; !ok {
				continue
			}
		}
		if len(tagSet) > 0 && !hasAnyTag(task.Skills, filters.Tags) {
			continue
		}
		filtered = append(filtered, task)
	}

	return filtered
}

func convertGeneralTasks(tasks []GeneralAgentTask) []EvalTask {
	evalTasks := make([]EvalTask, 0, len(tasks))
	for _, task := range tasks {
		evalTasks = append(evalTasks, generalTaskToEvalTask(task))
	}
	return evalTasks
}

func generalTaskToEvalTask(task GeneralAgentTask) EvalTask {
	pass := make([]string, 0, len(task.Constraints)+1)
	pass = append(pass, task.Constraints...)
	if strings.TrimSpace(task.ExpectedOutput) != "" {
		pass = append(pass, "expected_output: "+strings.TrimSpace(task.ExpectedOutput))
	}

	tags := make([]string, 0, len(task.Skills)+1)
	tags = append(tags, task.Skills...)
	if strings.TrimSpace(task.Surface) != "" {
		tags = append(tags, "surface:"+strings.TrimSpace(task.Surface))
	}

	return EvalTask{
		ID:             task.ID,
		Title:          task.Title,
		Goal:           task.Goal,
		Difficulty:     DifficultyTier(task.Difficulty),
		Domain:         EvalDomain(task.Domain),
		Tags:           tags,
		ExpectedSteps:  len(task.Constraints),
		PassCriteria:   pass,
		Weight:         1.0,
		MaxTokenBudget: 0,
	}
}
