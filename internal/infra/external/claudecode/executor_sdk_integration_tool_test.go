//go:build integration

package claudecode

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
)

// TestSDKBridgeExecutor_Integration_ToolFiltering verifies that:
//   - Write tool events ARE forwarded with trimmed args
//   - Read tool events are suppressed
//
// Run: go test -tags=integration -run TestSDKBridgeExecutor_Integration_Tool -v ./internal/infra/external/claudecode/
func TestSDKBridgeExecutor_Integration_ToolFiltering(t *testing.T) {
	repoRoot := findRepoRoot(t)
	bridgeScript := filepath.Join(repoRoot, "scripts", "cc_bridge", "cc_bridge.py")
	pythonBin := filepath.Join(repoRoot, "scripts", "cc_bridge", ".venv", "bin", "python3")

	if _, err := os.Stat(pythonBin); err != nil {
		t.Skipf("Python venv not found at %s", pythonBin)
	}

	exec := NewSDKBridge(SDKBridgeConfig{
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
		TaskID:    "integration-tool-test",
		AgentType: "claude_code",
		Prompt:    "Create a file /tmp/elephant-bridge-test.txt with content 'bridge works', then read it back to confirm it exists. Use the Write tool and Read tool.",
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

	// Check that Write was forwarded.
	var hasWrite bool
	for _, p := range progress {
		if p.CurrentTool == "Write" {
			hasWrite = true
			// Verify args are trimmed (only file_path, no content).
			if len(p.CurrentArgs) > 200 {
				t.Errorf("Write args too long (not trimmed): %d chars", len(p.CurrentArgs))
			}
		}
		// Read should NOT appear.
		if p.CurrentTool == "Read" {
			t.Error("Read tool should be suppressed but appeared in progress")
		}
	}

	if !hasWrite && len(progress) > 0 {
		t.Log("Warning: Write tool not found in progress (may have used Bash instead)")
	}

	t.Logf("Total progress events: %d (should be fewer than total tool calls)", len(progress))
}
