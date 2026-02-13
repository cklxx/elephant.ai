//go:build integration

package coding

import (
	"context"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	externalinfra "alex/internal/infra/external"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
)

func TestManagedExternalExecutor_Integration_BothAgents(t *testing.T) {
	repoRoot := findIntegrationRepoRoot(t)
	chdirRepoRootForIntegration(t, repoRoot)

	cfg := runtimeconfig.DefaultExternalAgentsConfig()
	cfg.Codex.Enabled = true
	cfg.Codex.Binary = "codex"
	cfg.Codex.Timeout = 120 * time.Second
	cfg.Codex.ApprovalPolicy = "never"
	cfg.Codex.Sandbox = "danger-full-access"

	cfg.ClaudeCode.Enabled = true
	cfg.ClaudeCode.Binary = "claude"
	cfg.ClaudeCode.DefaultMode = "autonomous"
	cfg.ClaudeCode.Timeout = 120 * time.Second
	cfg.ClaudeCode.MaxTurns = 4

	registry := externalinfra.NewRegistry(cfg, logging.OrNop(nil))
	managed := NewManagedExternalExecutor(registry, logging.OrNop(nil))

	tests := []struct {
		name       string
		agentType  string
		marker     string
		prereqFunc func(*testing.T)
	}{
		{
			name:      "codex",
			agentType: "codex",
			marker:    "MANAGED_CODEX_E2E_OK",
			prereqFunc: func(t *testing.T) {
				if _, err := osexec.LookPath("codex"); err != nil {
					t.Skip("codex binary not found in PATH")
				}
			},
		},
		{
			name:      "claude_code",
			agentType: "claude_code",
			marker:    "MANAGED_CLAUDE_E2E_OK",
			prereqFunc: func(t *testing.T) {
				if _, err := osexec.LookPath("claude"); err != nil {
					t.Skip("claude binary not found in PATH")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.prereqFunc != nil {
				tt.prereqFunc(t)
			}

			req := agent.ExternalAgentRequest{
				TaskID:        "managed-integration-" + tt.name,
				AgentType:     tt.agentType,
				Prompt:        "Reply exactly with " + tt.marker,
				WorkingDir:    repoRoot,
				ExecutionMode: "execute",
				AutonomyLevel: "full",
				Config: map[string]string{
					"task_kind":          "coding",
					"verify":             "false",
					"retry_max_attempts": "1",
				},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			res, err := managed.Execute(ctx, req)
			if err != nil {
				t.Fatalf("execute failed: %v", err)
			}
			if res == nil {
				t.Fatal("expected result")
			}
			if !strings.Contains(res.Answer, tt.marker) {
				t.Fatalf("expected answer containing %q, got: %q", tt.marker, res.Answer)
			}
			t.Logf("answer=%q tokens=%d cost=%v", res.Answer, res.TokensUsed, res.Metadata["cost_usd"])
		})
	}
}

func findIntegrationRepoRoot(t *testing.T) string {
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

func chdirRepoRootForIntegration(t *testing.T, repoRoot string) {
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
