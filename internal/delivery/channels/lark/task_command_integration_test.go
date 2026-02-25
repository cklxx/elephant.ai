//go:build integration

package lark

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"alex/internal/delivery/channels"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	codinginfra "alex/internal/infra/coding"
	externalinfra "alex/internal/infra/external"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
)

func TestGatewayIntegration_TaskCommandsDispatchRealExternalAgents(t *testing.T) {
	repoRoot := findLarkIntegrationRepoRoot(t)
	chdirRepoRoot(t, repoRoot)

	tests := []struct {
		name      string
		command   string
		marker    string
		setupFunc func(cfg *runtimeconfig.ExternalAgentsConfig)
		prereq    func(*testing.T)
	}{
		{
			name:    "codex",
			command: "/codex Reply exactly with LARK_CODEX_GATEWAY_E2E_OK",
			marker:  "LARK_CODEX_GATEWAY_E2E_OK",
			setupFunc: func(cfg *runtimeconfig.ExternalAgentsConfig) {
				cfg.Codex.Enabled = true
				cfg.Codex.Binary = "codex"
				cfg.Codex.ApprovalPolicy = "never"
				cfg.Codex.Sandbox = "danger-full-access"
				cfg.Codex.Timeout = 120 * time.Second
			},
			prereq: func(t *testing.T) {
				if _, err := osexec.LookPath("codex"); err != nil {
					t.Skip("codex binary not found in PATH")
				}
			},
		},
		{
			name:    "claude_code",
			command: "/cc Reply exactly with LARK_CLAUDE_GATEWAY_E2E_OK",
			marker:  "LARK_CLAUDE_GATEWAY_E2E_OK",
			setupFunc: func(cfg *runtimeconfig.ExternalAgentsConfig) {
				cfg.ClaudeCode.Enabled = true
				cfg.ClaudeCode.Binary = "claude"
				cfg.ClaudeCode.DefaultMode = "autonomous"
				cfg.ClaudeCode.MaxTurns = 4
				cfg.ClaudeCode.Timeout = 120 * time.Second
			},
			prereq: func(t *testing.T) {
				if _, err := osexec.LookPath("claude"); err != nil {
					t.Skip("claude binary not found in PATH")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.prereq != nil {
				tt.prereq(t)
			}

			cfg := runtimeconfig.DefaultExternalAgentsConfig()
			if tt.setupFunc != nil {
				tt.setupFunc(&cfg)
			}

			registry := externalinfra.NewRegistry(cfg, logging.OrNop(nil))
			managed := codinginfra.NewManagedExternalExecutor(registry, logging.OrNop(nil))
			exec := &integrationDispatchExecutor{
				external:   managed,
				workingDir: repoRoot,
			}

			rec := NewRecordingMessenger()
			gw := newTestGatewayWithMessenger(exec, rec, channels.BaseConfig{
				SessionPrefix: "itest-lark",
				AllowDirect:   true,
				ReplyTimeout:  2 * time.Minute,
			})

			msgID := fmt.Sprintf("om_lark_integration_%s_%d", tt.name, time.Now().UnixNano())
			err := gw.InjectMessage(context.Background(), "oc_lark_integration", "p2p", "ou_integration", msgID, tt.command)
			if err != nil {
				t.Fatalf("InjectMessage failed: %v", err)
			}
			gw.WaitForTasks()

			replies := rec.CallsByMethod("ReplyMessage")
			if len(replies) == 0 {
				t.Fatalf("expected reply message, got calls=%#v", rec.Calls())
			}

			replyText := extractTextContent(replies[len(replies)-1].Content, nil)
			if !strings.Contains(replyText, tt.marker) {
				t.Fatalf("expected reply containing %q, got: %q", tt.marker, replyText)
			}
		})
	}
}

type integrationDispatchExecutor struct {
	external   agent.ExternalAgentExecutor
	workingDir string
}

func (e *integrationDispatchExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	if strings.TrimSpace(sessionID) == "" {
		sessionID = fmt.Sprintf("itest-%d", time.Now().UnixNano())
	}
	return &storage.Session{ID: sessionID, Metadata: map[string]string{}}, nil
}

func (e *integrationDispatchExecutor) ExecuteTask(ctx context.Context, task string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	if e.external == nil {
		return nil, fmt.Errorf("external executor is nil")
	}
	agentType, description, err := parseDispatchPromptForIntegration(task)
	if err != nil {
		return nil, err
	}

	req := agent.ExternalAgentRequest{
		TaskID:        fmt.Sprintf("lark-task-%d", time.Now().UnixNano()),
		AgentType:     agentType,
		Prompt:        description,
		WorkingDir:    e.workingDir,
		ExecutionMode: "execute",
		AutonomyLevel: "full",
		Config: map[string]string{
			"task_kind":          "coding",
			"verify":             "false",
			"retry_max_attempts": "1",
		},
	}

	res, err := e.external.Execute(ctx, req)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return &agent.TaskResult{Answer: ""}, nil
	}
	return &agent.TaskResult{
		Answer:     strings.TrimSpace(res.Answer),
		TokensUsed: res.TokensUsed,
	}, nil
}

func parseDispatchPromptForIntegration(prompt string) (agentType, description string, err error) {
	const openTag = "<user_task_description>"
	const closeTag = "</user_task_description>"

	lines := strings.Split(prompt, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- agent_type:") {
			agentType = strings.TrimSpace(strings.TrimPrefix(trimmed, "- agent_type:"))
			break
		}
	}
	if agentType == "" {
		return "", "", fmt.Errorf("dispatch prompt missing agent_type: %q", prompt)
	}

	start := strings.Index(prompt, openTag)
	end := strings.Index(prompt, closeTag)
	if start < 0 || end < 0 || end < start {
		return "", "", fmt.Errorf("dispatch prompt missing user_task_description block")
	}
	description = strings.TrimSpace(prompt[start+len(openTag) : end])
	if description == "" {
		return "", "", fmt.Errorf("dispatch prompt contains empty user_task_description")
	}
	return agentType, description, nil
}

func findLarkIntegrationRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.mod found)")
		}
		dir = parent
	}
}

func chdirRepoRoot(t *testing.T, repoRoot string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir to repo root failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
}
