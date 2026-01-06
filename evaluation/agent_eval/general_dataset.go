package agent_eval

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"alex/evaluation/swe_bench"
)

//go:embed datasets/general_agent_eval.json
var embeddedGeneralDataset []byte

type GeneralAgentTask struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Goal           string     `json:"goal"`
	Context        string     `json:"context"`
	Input          string     `json:"input"`
	Constraints    []string   `json:"constraints"`
	ExpectedOutput string     `json:"expected_output"`
	Skills         []string   `json:"skills"`
	Difficulty     Difficulty `json:"difficulty"`
	Domain         string     `json:"domain"`
	Surface        string     `json:"surface"`
}

func loadGeneralAgentDataset(path string, limit int) ([]swe_bench.Instance, error) {
	var tasks []GeneralAgentTask

	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		if err := decodeGeneralDataset(bytes.NewReader(embeddedGeneralDataset), &tasks); err != nil {
			return nil, fmt.Errorf("decode embedded general agent dataset: %w", err)
		}
	} else {
		// Ensure the path is a single file name without directory traversal.
		if strings.Contains(cleanPath, "/") || strings.Contains(cleanPath, "\\") || strings.Contains(cleanPath, "..") {
			return nil, fmt.Errorf("invalid general agent dataset path")
		}

		file, err := os.Open(cleanPath)
		if err != nil {
			return nil, fmt.Errorf("open general agent dataset: %w", err)
		}
		defer file.Close()

		if err := decodeGeneralDataset(file, &tasks); err != nil {
			return nil, err
		}
	}

	if limit > 0 && limit < len(tasks) {
		tasks = tasks[:limit]
	}

	instances := make([]swe_bench.Instance, 0, len(tasks))
	for _, task := range tasks {
		if strings.TrimSpace(task.Surface) == "" {
			task.Surface = "web"
		}
		instances = append(instances, task.toInstance())
	}

	return instances, nil
}

func (t GeneralAgentTask) toInstance() swe_bench.Instance {
	var problem strings.Builder

	problem.WriteString("## Goal\n")
	problem.WriteString(t.Goal)
	problem.WriteString("\n\n## Context\n")
	problem.WriteString(t.Context)
	problem.WriteString("\n\n## Input\n")
	problem.WriteString(t.Input)
	problem.WriteString("\n\n## Expected Output\n")
	problem.WriteString(t.ExpectedOutput)

	if len(t.Constraints) > 0 {
		problem.WriteString("\n\n## Constraints\n")
		for _, constraint := range t.Constraints {
			problem.WriteString("- ")
			problem.WriteString(constraint)
			problem.WriteString("\n")
		}
	}

	hints := formatConstraints(t.Constraints)

	return swe_bench.Instance{
		ID:               t.ID,
		RepoURL:          "general-agent-benchmark",
		BaseCommit:       "v1",
		ProblemStatement: problem.String(),
		Hints:            hints,
		Metadata: map[string]any{
			"title":          t.Title,
			"domain":         t.Domain,
			"skills":         t.Skills,
			"difficulty":     t.Difficulty,
			"expectedOutput": t.ExpectedOutput,
			"surface":        t.Surface,
		},
	}
}

func formatConstraints(constraints []string) string {
	if len(constraints) == 0 {
		return ""
	}

	lines := make([]string, 0, len(constraints))
	for _, constraint := range constraints {
		lines = append(lines, "- "+constraint)
	}

	return strings.Join(lines, "\n")
}

func decodeGeneralDataset(reader io.Reader, tasks *[]GeneralAgentTask) error {
	if err := json.NewDecoder(reader).Decode(tasks); err != nil {
		return fmt.Errorf("decode general agent dataset: %w", err)
	}
	return nil
}
