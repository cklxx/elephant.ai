package swe_bench

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/workflow"
)

func setTestConfigPath(t *testing.T, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", path)
}

func TestNewAlexAgentUsesRuntimeConfigOverrides(t *testing.T) {
	setTestConfigPath(t, `
runtime:
  api_key: "test-key"
  llm_provider: "mock"
  llm_model: "mock-eval-model"
  temperature: 0.5
  max_tokens: 777
`)

	cfg := DefaultBatchConfig()
	cfg.Agent.Model.Name = ""
	cfg.Agent.Model.Temperature = 0
	cfg.Agent.Model.MaxTokens = 0
	cfg.Agent.MaxTurns = 0

	agent, err := NewAlexAgent(cfg)
	if err != nil {
		t.Fatalf("NewAlexAgent returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := agent.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	})

	resolved := agent.coordinator.GetConfig()
	if resolved.LLMProvider != "mock" {
		t.Fatalf("expected provider mock, got %s", resolved.LLMProvider)
	}
	if resolved.LLMModel != "mock-eval-model" {
		t.Fatalf("expected model mock-eval-model, got %s", resolved.LLMModel)
	}
	if diff := math.Abs(resolved.Temperature - 0.5); diff > 1e-9 {
		t.Fatalf("expected temperature 0.5, got %f", resolved.Temperature)
	}
	if resolved.MaxTokens != 777 {
		t.Fatalf("expected max tokens 777, got %d", resolved.MaxTokens)
	}
	if resolved.MaxIterations != 10 {
		t.Fatalf("expected default max iterations 10, got %d", resolved.MaxIterations)
	}
	if agent.runtimeConfig.SessionDir != "~/.alex-sessions-swebench" {
		t.Fatalf("expected sessions dir override, got %s", agent.runtimeConfig.SessionDir)
	}
	if agent.runtimeConfig.CostDir != "~/.alex-costs-swebench" {
		t.Fatalf("expected cost dir override, got %s", agent.runtimeConfig.CostDir)
	}
	if cfg.Agent.Model.Name != resolved.LLMModel {
		t.Fatalf("batch config model not updated, want %s got %s", resolved.LLMModel, cfg.Agent.Model.Name)
	}
	if diff := math.Abs(cfg.Agent.Model.Temperature - 0.5); diff > 1e-9 {
		t.Fatalf("batch config temperature mismatch: %f", cfg.Agent.Model.Temperature)
	}
	if cfg.Agent.Model.MaxTokens != resolved.MaxTokens {
		t.Fatalf("batch config max tokens mismatch: %d vs %d", cfg.Agent.Model.MaxTokens, resolved.MaxTokens)
	}
	if cfg.Agent.MaxTurns != resolved.MaxIterations {
		t.Fatalf("batch config max turns mismatch: %d vs %d", cfg.Agent.MaxTurns, resolved.MaxIterations)
	}
}

func TestNewAlexAgentAdjustsBaseURLForOpenAIModels(t *testing.T) {
	setTestConfigPath(t, `
runtime:
  api_key: "test-key"
  llm_provider: "openai"
  llm_model: "gpt-4.1-mini"
`)

	cfg := DefaultBatchConfig()
	cfg.Agent.Model.Name = ""
	cfg.Agent.MaxTurns = 5

	agent, err := NewAlexAgent(cfg)
	if err != nil {
		t.Fatalf("NewAlexAgent returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := agent.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	})

	if agent.runtimeConfig.BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("expected OpenAI base URL, got %s", agent.runtimeConfig.BaseURL)
	}

	resolved := agent.coordinator.GetConfig()
	if resolved.MaxIterations != 5 {
		t.Fatalf("expected max iterations from batch config, got %d", resolved.MaxIterations)
	}
}

func TestResolveTaskResultStatus(t *testing.T) {
	tests := []struct {
		name       string
		result     *agent.TaskResult
		wantStatus ResultStatus
		wantType   string
	}{
		{
			name: "successful workflow remains completed",
			result: &agent.TaskResult{
				StopReason: "final_answer",
				Workflow: &workflow.WorkflowSnapshot{
					Phase: workflow.PhaseSucceeded,
				},
			},
			wantStatus: StatusCompleted,
			wantType:   "",
		},
		{
			name: "max iterations stop reason becomes failed",
			result: &agent.TaskResult{
				StopReason: "max_iterations",
				Workflow: &workflow.WorkflowSnapshot{
					Phase: workflow.PhaseSucceeded,
				},
			},
			wantStatus: StatusFailed,
			wantType:   "max_iterations_error",
		},
		{
			name: "failed workflow becomes failed",
			result: &agent.TaskResult{
				StopReason: "final_answer",
				Workflow: &workflow.WorkflowSnapshot{
					Phase: workflow.PhaseFailed,
				},
			},
			wantStatus: StatusFailed,
			wantType:   "workflow_failed",
		},
		{
			name: "execute node max iterations becomes failed",
			result: &agent.TaskResult{
				StopReason: "final_answer",
				Workflow: &workflow.WorkflowSnapshot{
					Phase: workflow.PhaseSucceeded,
					Nodes: []workflow.NodeSnapshot{
						{
							ID:     "execute",
							Status: workflow.NodeStatusSucceeded,
							Output: map[string]any{"stop": "max_iterations"},
						},
					},
				},
			},
			wantStatus: StatusFailed,
			wantType:   "max_iterations_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotErr, gotType := resolveTaskResultStatus(tt.result)
			if gotStatus != tt.wantStatus {
				t.Fatalf("status mismatch: got=%s want=%s", gotStatus, tt.wantStatus)
			}
			if gotType != tt.wantType {
				t.Fatalf("error type mismatch: got=%s want=%s", gotType, tt.wantType)
			}
			if tt.wantStatus == StatusFailed && strings.TrimSpace(gotErr) == "" {
				t.Fatalf("expected non-empty error for failed status")
			}
		})
	}
}

func TestBuildTaskPromptIncludesWorkspaceConventions(t *testing.T) {
	agent := &AlexAgent{}
	prompt := agent.buildTaskPrompt(Instance{
		ID:               "django__django-11885",
		RepoURL:          "django/django",
		BaseCommit:       "04ac9b45a34440fa447feb6ae934687aacbfc5f4",
		ProblemStatement: "example",
	})

	if !strings.Contains(prompt, "## Workspace Conventions:") {
		t.Fatalf("expected workspace conventions section in prompt")
	}
	if !strings.Contains(prompt, "~/code/django") {
		t.Fatalf("expected default home workspace hint in prompt")
	}
	if !strings.Contains(prompt, "/tmp/repos/django") {
		t.Fatalf("expected deterministic tmp workspace hint in prompt")
	}
}
