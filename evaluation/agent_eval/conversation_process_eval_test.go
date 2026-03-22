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
// LLM. Skipped unless CONVERSATION_EVAL_LLM=1 is set.
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

	writeReport(t, "conversation-routing", report)

	threshold := 0.80
	if summary.PassRate() < threshold {
		t.Errorf("pass rate %.1f%% below threshold %.0f%%", summary.PassRate()*100, threshold*100)
	}
}

// TestConversationProcessRouting_AB runs an A/B comparison of prompt variants.
// CONVERSATION_EVAL_LLM=1 required. Runs each variant 1x and reports comparison.
func TestConversationProcessRouting_AB(t *testing.T) {
	if os.Getenv("CONVERSATION_EVAL_LLM") != "1" {
		t.Skip("set CONVERSATION_EVAL_LLM=1 to run A/B prompt eval")
	}

	ds, err := LoadConversationDataset("")
	if err != nil {
		t.Fatalf("load dataset: %v", err)
	}

	client := newConversationEvalLLMClient(t)
	tools := buildEvalTools()

	variants := []promptVariant{
		{
			Name: "current",
			Prompt: buildPromptVariant_Current(),
		},
		{
			Name: "minimal-rules",
			Prompt: buildPromptVariant_MinimalRules(),
		},
		{
			Name: "example-driven",
			Prompt: buildPromptVariant_ExampleDriven(),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var results []variantResult
	for _, v := range variants {
		t.Logf("--- Running variant: %s ---", v.Name)
		summary := RunConversationEval(ctx, client, v.Prompt, tools, ds.Scenarios)
		results = append(results, variantResult{Name: v.Name, Summary: summary})
	}

	report := formatABReport(results)
	t.Log(report)
	writeReport(t, "conversation-routing-ab", report)
}

// ---------------------------------------------------------------------------
// Prompt variants for A/B testing
// ---------------------------------------------------------------------------

type promptVariant struct {
	Name   string
	Prompt string
}

type variantResult struct {
	Name    string
	Summary *ConversationEvalSummary
}

// buildPromptVariant_Current is the current production prompt.
func buildPromptVariant_Current() string {
	return buildEvalConversationSystemPrompt()
}

// buildPromptVariant_MinimalRules uses minimal rules with no reply length
// constraints — tests whether the LLM naturally keeps replies short.
func buildPromptVariant_MinimalRules() string {
	base := `You are an IM chatbot. Reply ultra-short or use tools.

## Decision flow (check in order)
1. Stop/cancel intent (停/取消/别做了/算了/cancel/stop/nevermind) → stop_worker
2. Action request (coding, research, analysis, writing, anything that takes time) → dispatch_worker
3. Follow-up to a running task → dispatch_worker (inject)
4. Everything else (greetings, chitchat, factual Q&A) → reply directly

Every tool call MUST include an "ack" parameter — the reply shown to the user.
Cross-task: include "#N" in task description to reference task N's result.

## Safety
- Never fabricate info or status.
- Never include secrets.`

	return base + "\n\nCurrent date: " + time.Now().Format("2006-01-02") + " (Asia/Shanghai)"
}

// buildPromptVariant_ExampleDriven uses the previous experiment winner (more
// examples with ack format shown). Tests whether showing ack samples helps.
func buildPromptVariant_ExampleDriven() string {
	replyRules := `## Reply rules (HARD CONSTRAINTS)
- 中文: ≤12字, 禁句号, 省略主语/我, 口语化
- English: ≤15 words, lowercase, fragments, no period
- NEVER use 其实/然后/的话/非常/请/您/好的/可以的
- One short sentence only. No explanations.`

	base := `You are an IM chatbot. Reply ultra-short or use tools.

## Decision examples
- "你好" → reply directly: "你好"
- "重构 auth 模块" → dispatch_worker (task="重构 auth 模块", ack="重构 auth 模块中")
- "停" → stop_worker (ack="已停止")
- "取消任务" → stop_worker (ack="已取消")
- "帮我写个技术方案" → dispatch_worker (task="写技术方案", ack="开始写技术方案")
- "谢谢" → reply directly: "不客气"
- "算了不用了" → stop_worker (ack="已停止")
- "用 PostgreSQL 不要 MySQL" → dispatch_worker (inject follow-up)
- "👍" → reply directly: "👍"
- "lint" → dispatch_worker (task="lint", ack="跑 lint 中")

## Decision rules
1. Stop/cancel intent (停/取消/别做了/算了/cancel/stop/nevermind) → stop_worker
2. Action request (anything that takes time) → dispatch_worker
3. Follow-up to running task → dispatch_worker (inject)
4. Everything else → reply directly

Every tool call MUST include an "ack" parameter — the reply shown to the user.
Cross-task: include "#N" in task description to reference task N's result.

` + replyRules + `

## Safety
- Never fabricate info or status.
- Never include secrets.`

	return base + "\n\nCurrent date: " + time.Now().Format("2006-01-02") + " (Asia/Shanghai)"
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func buildConversationEvalFixture(t *testing.T) (portsllm.LLMClient, string, []ports.ToolDefinition) {
	t.Helper()
	client := newConversationEvalLLMClient(t)
	return client, buildEvalConversationSystemPrompt(), buildEvalTools()
}

func buildEvalTools() []ports.ToolDefinition {
	return []ports.ToolDefinition{
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
}

// buildEvalConversationSystemPrompt builds the current production prompt.
func buildEvalConversationSystemPrompt() string {
	replyRules := `## Reply rules (HARD CONSTRAINTS)
- 中文: ≤12字, 禁句号, 省略主语/我, 口语化
- English: ≤15 words, lowercase, fragments, no period
- NEVER use 其实/然后/的话/非常/请/您/好的/可以的
- One short sentence only. No explanations.`

	base := `You are an IM chatbot. Reply ultra-short or use tools.

## Decision examples
- "你好" → reply directly
- "重构 auth 模块" → dispatch_worker
- "帮我看看为什么 CI 挂了" → dispatch_worker
- "停" / "cancel that" / "算了不用了" / "停掉现在的" → stop_worker
- "用 PostgreSQL 不要 MySQL" → dispatch_worker (inject follow-up)
- "lint" → dispatch_worker

## Decision rules
1. Stop/cancel intent (停/取消/别做了/算了/cancel/stop/nevermind) → stop_worker
2. Action request (anything that takes time) → dispatch_worker
3. Follow-up to running task → dispatch_worker (inject)
4. Everything else → reply directly

Every tool call MUST include an "ack" parameter — the reply shown to the user.
Cross-task: include "#N" in task description to reference task N's result.

` + replyRules + `

## Safety
- Never fabricate info or status.
- Never include secrets.`

	return base + "\n\nCurrent date: " + time.Now().Format("2006-01-02") + " (Asia/Shanghai)"
}

func writeReport(t *testing.T, prefix, report string) {
	t.Helper()
	outDir := os.Getenv("EVAL_OUTPUT_DIR")
	if outDir == "" {
		root := findRepoRoot()
		if root != "" {
			outDir = root + "/.openmax/bench"
		} else {
			outDir = ".openmax/bench"
		}
	}
	os.MkdirAll(outDir, 0o755)
	ts := time.Now().Format("20060102-150405")
	outFile := fmt.Sprintf("%s/%s-%s.md", outDir, prefix, ts)
	os.WriteFile(outFile, []byte(report), 0o644)
	t.Logf("Report written to %s", outFile)
}

func formatABReport(results []variantResult) string {
	var sb strings.Builder
	sb.WriteString("# Conversation Process Routing — A/B Comparison\n\n")

	// Summary table
	sb.WriteString("| Variant | Total | Passed | Failed | Errors | Pass Rate |\n")
	sb.WriteString("|---------|-------|--------|--------|--------|----------|\n")
	for _, r := range results {
		s := r.Summary
		sb.WriteString(fmt.Sprintf("| **%s** | %d | %d | %d | %d | **%.1f%%** |\n",
			r.Name, s.Total, s.Passed, s.Failed, s.Errors, s.PassRate()*100))
	}

	// Per-category breakdown
	sb.WriteString("\n## By Category\n\n")
	categories := []string{"reply_only", "dispatch", "dispatch_inject", "stop"}
	sb.WriteString("| Category |")
	for _, r := range results {
		sb.WriteString(fmt.Sprintf(" %s |", r.Name))
	}
	sb.WriteString("\n|----------|")
	for range results {
		sb.WriteString("---------|")
	}
	sb.WriteString("\n")
	for _, cat := range categories {
		sb.WriteString(fmt.Sprintf("| %s |", cat))
		for _, r := range results {
			cr := r.Summary.ByCategory[cat]
			if cr != nil && cr.Total > 0 {
				sb.WriteString(fmt.Sprintf(" %d/%d (%.0f%%) |", cr.Passed, cr.Total, float64(cr.Passed)/float64(cr.Total)*100))
			} else {
				sb.WriteString(" - |")
			}
		}
		sb.WriteString("\n")
	}

	// Per-scenario diff: show scenarios where variants disagree
	sb.WriteString("\n## Disagreements (scenarios where variants differ)\n\n")
	if len(results) >= 2 {
		base := results[0]
		for i := 1; i < len(results); i++ {
			other := results[i]
			sb.WriteString(fmt.Sprintf("\n### %s vs %s\n\n", base.Name, other.Name))
			for j, br := range base.Summary.Results {
				if j >= len(other.Summary.Results) {
					break
				}
				or := other.Summary.Results[j]
				if br.Pass != or.Pass {
					winner := base.Name
					if or.Pass {
						winner = other.Name
					}
					sb.WriteString(fmt.Sprintf("- **%s** (%s): intent=%q → winner=%s\n",
						br.ScenarioID, br.Category, br.Intent, winner))
					sb.WriteString(fmt.Sprintf("  - %s: tools=%v pass=%v\n", base.Name, br.ActualTools, br.Pass))
					sb.WriteString(fmt.Sprintf("  - %s: tools=%v pass=%v\n", other.Name, or.ActualTools, or.Pass))
				}
			}
		}
	}

	return sb.String()
}
