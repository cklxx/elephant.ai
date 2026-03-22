package agent_eval

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	ports "alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
)

// TestConversationProcessRouting_Offline validates dataset structure
// without requiring an LLM. Always runs.
func TestConversationProcessRouting_Offline(t *testing.T) {
	ds, err := LoadConversationDataset("")
	if err != nil {
		t.Fatalf("load dataset: %v", err)
	}
	if len(ds.Scenarios) == 0 {
		t.Fatal("dataset has no scenarios")
	}

	ids := make(map[string]bool)
	validCategories := map[string]bool{
		"reply_only":      true,
		"dispatch":        true,
		"dispatch_inject": true,
		"stop":            true,
	}

	for _, sc := range ds.Scenarios {
		if sc.ID == "" {
			t.Error("scenario with empty ID")
		}
		if ids[sc.ID] {
			t.Errorf("duplicate scenario ID: %s", sc.ID)
		}
		ids[sc.ID] = true

		if !validCategories[sc.Category] {
			t.Errorf("scenario %s: invalid category %q", sc.ID, sc.Category)
		}
		if sc.Intent == "" {
			t.Errorf("scenario %s: empty intent", sc.ID)
		}

		// Validate expected_tools consistency with category.
		switch sc.Category {
		case "reply_only":
			if len(sc.ExpectedTools) != 0 {
				t.Errorf("scenario %s: reply_only should have empty expected_tools, got %v", sc.ID, sc.ExpectedTools)
			}
		case "dispatch", "dispatch_inject":
			if len(sc.ExpectedTools) != 1 || sc.ExpectedTools[0] != "dispatch_worker" {
				t.Errorf("scenario %s: dispatch should expect [dispatch_worker], got %v", sc.ID, sc.ExpectedTools)
			}
		case "stop":
			if len(sc.ExpectedTools) != 1 || sc.ExpectedTools[0] != "stop_worker" {
				t.Errorf("scenario %s: stop should expect [stop_worker], got %v", sc.ID, sc.ExpectedTools)
			}
		}
	}

	t.Logf("Validated %d scenarios across %d categories", len(ds.Scenarios), len(validCategories))
}

// TestConversationProcessRouting_LLM runs the full routing eval against a real
// LLM. Skipped unless CONVERSATION_EVAL_LLM=1 is set (openbench sets this).
func TestConversationProcessRouting_LLM(t *testing.T) {
	if os.Getenv("CONVERSATION_EVAL_LLM") != "1" {
		t.Skip("set CONVERSATION_EVAL_LLM=1 to run LLM routing eval")
	}

	ds, err := LoadConversationDataset("")
	if err != nil {
		t.Fatalf("load dataset: %v", err)
	}

	client, systemPrompt, tools := buildConversationEvalFixture(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	summary := RunConversationEval(ctx, client, systemPrompt, tools, ds.Scenarios)
	report := FormatConversationEvalReport(summary)
	t.Log(report)

	// Write report to file.
	outDir := os.Getenv("EVAL_OUTPUT_DIR")
	if outDir == "" {
		outDir = ".openmax/bench"
	}
	os.MkdirAll(outDir, 0o755)
	ts := time.Now().Format("20060102-150405")
	outFile := fmt.Sprintf("%s/conversation-routing-%s.md", outDir, ts)
	os.WriteFile(outFile, []byte(report), 0o644)
	t.Logf("Report written to %s", outFile)

	// Fail if pass rate is below threshold.
	threshold := 0.80
	if summary.PassRate() < threshold {
		t.Errorf("pass rate %.1f%% below threshold %.0f%%", summary.PassRate()*100, threshold*100)
	}
}

// buildConversationEvalFixture creates a minimal LLM client and prompt setup
// matching production conversation process configuration.
func buildConversationEvalFixture(t *testing.T) (portsllm.LLMClient, string, []ports.ToolDefinition) {
	t.Helper()

	// Use the production system prompt (without memory context).
	systemPrompt := buildEvalConversationSystemPrompt()

	tools := []ports.ToolDefinition{
		{
			Name:        "dispatch_worker",
			Description: "When the user wants something done (coding, research, analysis, writing) → launch a background agent. Also use when injecting follow-up context into a running task.",
			Parameters: ports.ParameterSchema{
				Type: "object",
				Properties: map[string]ports.Property{
					"task": {
						Type:        "string",
						Description: "Task description preserving the user's original intent.",
					},
					"ack": {
						Type:        "string",
						Description: "One-sentence reply shown to the user describing what you're about to do. Be specific about the action, not generic.",
					},
				},
				Required: []string{"task", "ack"},
			},
		},
		{
			Name:        "stop_worker",
			Description: "When the user wants to cancel or abort a running task → stop the specified worker.",
			Parameters: ports.ParameterSchema{
				Type: "object",
				Properties: map[string]ports.Property{
					"task_id": {
						Type:        "string",
						Description: "Task ID to stop. Empty string stops all running tasks.",
					},
					"ack": {
						Type:        "string",
						Description: "One-sentence reply shown to the user describing the action.",
					},
				},
				Required: []string{"ack"},
			},
		},
	}

	// Try to get a real LLM client from environment.
	client := newConversationEvalLLMClient(t)
	return client, systemPrompt, tools
}

// buildEvalConversationSystemPrompt builds the system prompt matching the
// production conversation_process.go output (without memory/narration).
func buildEvalConversationSystemPrompt() string {
	var sections []string

	replyRules := `## Reply rules (HARD CONSTRAINTS)
- 中文: ≤12字, 禁句号, 省略主语/我, 口语化
- English: ≤15 words, lowercase, fragments, no period
- NEVER use 其实/然后/的话/非常/请/您/好的/可以的
- One short sentence only. No explanations.`

	base := `You are an IM chatbot. Reply ultra-short or use tools.

## Decision flow (check in order)
1. Stop/cancel intent → stop_worker
2. Action request (coding, research, analysis, writing, anything that takes time) → dispatch_worker
3. Follow-up to a running task → dispatch_worker (inject)
4. Everything else (greetings, chitchat, factual Q&A) → reply directly

Every tool call MUST include an "ack" parameter — the reply shown to the user.
When a skill matches, include its name in the dispatch task.
Cross-task: include "#N" in task description to reference task N's result.

` + replyRules + `

## Safety
- Never fabricate info or status.
- Never include secrets.`

	sections = append(sections, strings.TrimSpace(base))
	sections = append(sections, fmt.Sprintf("Current date: %s (Asia/Shanghai)", time.Now().Format("2006-01-02")))

	return strings.Join(sections, "\n\n")
}
