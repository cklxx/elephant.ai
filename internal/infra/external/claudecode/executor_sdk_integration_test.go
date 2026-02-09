//go:build integration

package claudecode

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
)

// TestSDKBridgeExecutor_Integration_RealBridge spawns the real Python bridge
// and executes a simple autonomous query. Requires:
//   - Python venv at scripts/cc_bridge/.venv
//   - Claude CLI logged in (or ANTHROPIC_API_KEY in env)
//
// Run: go test -tags=integration -run TestSDKBridgeExecutor_Integration -v ./internal/infra/external/claudecode/
func TestSDKBridgeExecutor_Integration_RealBridge(t *testing.T) {
	// Resolve paths relative to repo root.
	repoRoot := findRepoRoot(t)
	bridgeScript := filepath.Join(repoRoot, "scripts", "cc_bridge", "cc_bridge.py")
	pythonBin := filepath.Join(repoRoot, "scripts", "cc_bridge", ".venv", "bin", "python3")

	if _, err := os.Stat(pythonBin); err != nil {
		t.Skipf("Python venv not found at %s (run scripts/cc_bridge/setup.sh first)", pythonBin)
	}

	exec := NewSDKBridge(SDKBridgeConfig{
		PythonBinary: pythonBin,
		BridgeScript: bridgeScript,
		APIKey:       os.Getenv("ANTHROPIC_API_KEY"), // optional if CLI is logged in
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
