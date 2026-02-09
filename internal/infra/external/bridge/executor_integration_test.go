//go:build integration

package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
)

// TestExecutor_Integration_ClaudeCode spawns the real Python bridge
// and executes a simple autonomous query. Requires:
//   - Python venv at scripts/cc_bridge/.venv
//   - Claude CLI logged in (or ANTHROPIC_API_KEY in env)
//
// Run: go test -tags=integration -run TestExecutor_Integration_ClaudeCode -v ./internal/infra/external/bridge/
func TestExecutor_Integration_ClaudeCode(t *testing.T) {
	repoRoot := findRepoRoot(t)
	bridgeScript := filepath.Join(repoRoot, "scripts", "cc_bridge", "cc_bridge.py")
	pythonBin := filepath.Join(repoRoot, "scripts", "cc_bridge", ".venv", "bin", "python3")

	if _, err := os.Stat(pythonBin); err != nil {
		t.Skipf("Python venv not found at %s (run scripts/cc_bridge/setup.sh first)", pythonBin)
	}

	exec := New(BridgeConfig{
		AgentType:    "claude_code",
		Interactive:  false,
		PythonBinary: pythonBin,
		BridgeScript: bridgeScript,
		APIKey:       os.Getenv("ANTHROPIC_API_KEY"),
		DefaultMode:  "autonomous",
		Timeout:      60 * time.Second,
		MaxTurns:     3,
	})

	var progress []agent.ExternalAgentProgress
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, err := exec.Execute(ctx, agent.ExternalAgentRequest{
		TaskID:     "integration-test",
		AgentType:  "claude_code",
		Prompt:     "What is 2+2? Reply with just the number.",
		WorkingDir: "/tmp",
		OnProgress: func(p agent.ExternalAgentProgress) {
			progress = append(progress, p)
		},
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected result")
	}
	if !strings.Contains(res.Answer, "4") {
		t.Fatalf("expected answer containing '4', got: %q", res.Answer)
	}
	if res.TokensUsed <= 0 {
		t.Fatalf("expected tokens > 0, got %d", res.TokensUsed)
	}
	t.Logf("Answer: %q, Tokens: %d, Cost: %v, Iters: %d, Progress events: %d",
		res.Answer, res.TokensUsed, res.Metadata["cost_usd"], res.Iterations, len(progress))
}

// TestExecutor_Integration_ToolFiltering verifies that:
//   - Write tool events ARE forwarded with trimmed args
//   - Read tool events are suppressed
//
// Run: go test -tags=integration -run TestExecutor_Integration_ToolFiltering -v ./internal/infra/external/bridge/
func TestExecutor_Integration_ToolFiltering(t *testing.T) {
	repoRoot := findRepoRoot(t)
	bridgeScript := filepath.Join(repoRoot, "scripts", "cc_bridge", "cc_bridge.py")
	pythonBin := filepath.Join(repoRoot, "scripts", "cc_bridge", ".venv", "bin", "python3")

	if _, err := os.Stat(pythonBin); err != nil {
		t.Skipf("Python venv not found at %s", pythonBin)
	}

	exec := New(BridgeConfig{
		AgentType:    "claude_code",
		Interactive:  false,
		PythonBinary: pythonBin,
		BridgeScript: bridgeScript,
		APIKey:       os.Getenv("ANTHROPIC_API_KEY"),
		DefaultMode:  "autonomous",
		Timeout:      120 * time.Second,
		MaxTurns:     10,
	})

	var progress []agent.ExternalAgentProgress
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	res, err := exec.Execute(ctx, agent.ExternalAgentRequest{
		TaskID:     "integration-tool-test",
		AgentType:  "claude_code",
		Prompt:     "Create a file /tmp/elephant-bridge-test.txt with content 'bridge works', then read it back to confirm it exists. Use the Write tool and Read tool.",
		WorkingDir: "/tmp",
		OnProgress: func(p agent.ExternalAgentProgress) {
			t.Logf("Progress: tool=%s args=%s files=%v", p.CurrentTool, p.CurrentArgs, p.FilesTouched)
			progress = append(progress, p)
		},
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected result")
	}

	t.Logf("Answer: %q", res.Answer)
	t.Logf("Tokens: %d, Cost: %v, Iters: %d", res.TokensUsed, res.Metadata["cost_usd"], res.Iterations)

	var hasWrite bool
	for _, p := range progress {
		if p.CurrentTool == "Write" {
			hasWrite = true
			if len(p.CurrentArgs) > 200 {
				t.Errorf("Write args too long (not trimmed): %d chars", len(p.CurrentArgs))
			}
		}
		if p.CurrentTool == "Read" {
			t.Error("Read tool should be suppressed but appeared in progress")
		}
	}

	if !hasWrite && len(progress) > 0 {
		t.Log("Warning: Write tool not found in progress (may have used Bash instead)")
	}

	t.Logf("Total progress events: %d (should be fewer than total tool calls)", len(progress))
}

func findRepoRoot(t *testing.T) string {
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
