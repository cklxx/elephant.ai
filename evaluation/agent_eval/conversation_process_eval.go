package agent_eval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	ports "alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"

	"gopkg.in/yaml.v3"
)

const defaultConversationCasesPath = "evaluation/agent_eval/datasets/conversation_process_routing.yaml"

// ConversationScenario represents a single routing test case.
type ConversationScenario struct {
	ID            string   `yaml:"id"`
	Category      string   `yaml:"category"`
	Intent        string   `yaml:"intent"`
	ExpectedTools []string `yaml:"expected_tools"`
	ExpectedMode  string   `yaml:"expected_mode,omitempty"`  // expected mode arg for respond tool (direct/think/delegate)
	WorkerStatus  string   `yaml:"worker_status,omitempty"`  // custom worker status; default "all idle"
}

// ConversationDataset holds the full scenario set.
type ConversationDataset struct {
	Version     string                 `yaml:"version"`
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Scenarios   []ConversationScenario `yaml:"scenarios"`
}

// ConversationEvalResult is the result of a single scenario evaluation.
type ConversationEvalResult struct {
	ScenarioID    string
	Category      string
	Intent        string
	ExpectedTools []string
	ActualTools   []string
	ExpectedMode  string
	ActualMode    string
	Reply         string
	Pass          bool
	Latency       time.Duration
	Error         string
}

// ConversationEvalSummary is the aggregate result.
type ConversationEvalSummary struct {
	Total      int
	Passed     int
	Failed     int
	Errors     int
	ByCategory map[string]*CategoryResult
	Results    []ConversationEvalResult
}

// CategoryResult tracks per-category stats.
type CategoryResult struct {
	Total  int
	Passed int
}

// PassRate returns the overall pass rate.
func (s *ConversationEvalSummary) PassRate() float64 {
	if s.Total == 0 {
		return 0
	}
	return float64(s.Passed) / float64(s.Total)
}

// LoadConversationDataset loads scenarios from YAML. Resolves relative paths
// from the repo root (walks up from cwd looking for go.mod).
func LoadConversationDataset(path string) (*ConversationDataset, error) {
	if path == "" {
		path = defaultConversationCasesPath
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read dataset: %w", err)
	}
	if os.IsNotExist(err) {
		// Try resolving from repo root.
		root := findRepoRoot()
		if root != "" {
			data, err = os.ReadFile(root + "/" + path)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("read dataset: %w", err)
	}
	var ds ConversationDataset
	if err := yaml.Unmarshal(data, &ds); err != nil {
		return nil, fmt.Errorf("parse dataset: %w", err)
	}
	return &ds, nil
}

// RunConversationEval runs the conversation process routing evaluation against
// a real LLM client. systemPrompt and tools should match the production config.
func RunConversationEval(
	ctx context.Context,
	client portsllm.LLMClient,
	systemPrompt string,
	tools []ports.ToolDefinition,
	scenarios []ConversationScenario,
) *ConversationEvalSummary {
	summary := &ConversationEvalSummary{
		ByCategory: make(map[string]*CategoryResult),
	}

	for _, sc := range scenarios {
		result := evalOneScenario(ctx, client, systemPrompt, tools, sc)
		summary.Total++
		if result.Error != "" {
			summary.Errors++
		} else if result.Pass {
			summary.Passed++
		} else {
			summary.Failed++
		}

		cat := summary.ByCategory[sc.Category]
		if cat == nil {
			cat = &CategoryResult{}
			summary.ByCategory[sc.Category] = cat
		}
		cat.Total++
		if result.Pass {
			cat.Passed++
		}

		summary.Results = append(summary.Results, result)
	}

	return summary
}

func evalOneScenario(
	ctx context.Context,
	client portsllm.LLMClient,
	systemPrompt string,
	tools []ports.ToolDefinition,
	sc ConversationScenario,
) ConversationEvalResult {
	result := ConversationEvalResult{
		ScenarioID:    sc.ID,
		Category:      sc.Category,
		Intent:        sc.Intent,
		ExpectedTools: sc.ExpectedTools,
	}

	workerStatus := sc.WorkerStatus
	if workerStatus == "" {
		workerStatus = "all idle"
	}
	userMsg := fmt.Sprintf("Worker status: %s\n\nUser message: %s", workerStatus, sc.Intent)

	start := time.Now()
	resp, err := client.Complete(ctx, ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMsg},
		},
		Tools:       tools,
		Temperature: 0.0, // deterministic for eval
		MaxTokens:   512,
	})
	result.Latency = time.Since(start)

	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Reply = resp.Content
	result.ExpectedMode = sc.ExpectedMode
	for _, tc := range resp.ToolCalls {
		result.ActualTools = append(result.ActualTools, tc.Name)
		// Extract mode from respond tool calls.
		if tc.Name == "respond" {
			if mode, ok := tc.Arguments["mode"].(string); ok {
				result.ActualMode = mode
			}
		}
	}

	result.Pass = toolSetsMatch(sc.ExpectedTools, result.ActualTools)
	// If expected_mode is set, also validate mode selection.
	// Missing ActualMode when ExpectedMode is set counts as a failure.
	if result.Pass && sc.ExpectedMode != "" {
		result.Pass = result.ActualMode == sc.ExpectedMode
	}
	return result
}

// toolSetsMatch checks if expected and actual tool sets match.
// Empty expected means "reply only" (no tool calls).
func toolSetsMatch(expected, actual []string) bool {
	if len(expected) == 0 {
		return len(actual) == 0
	}
	// Check that every expected tool appears in actual.
	actualSet := make(map[string]bool, len(actual))
	for _, t := range actual {
		actualSet[t] = true
	}
	for _, t := range expected {
		if !actualSet[t] {
			return false
		}
	}
	return true
}

// FormatConversationEvalReport formats the eval summary as Markdown.
func FormatConversationEvalReport(summary *ConversationEvalSummary) string {
	var sb strings.Builder
	sb.WriteString("# Conversation Process Routing Eval\n\n")
	sb.WriteString(fmt.Sprintf("**Total**: %d | **Passed**: %d | **Failed**: %d | **Errors**: %d | **Pass Rate**: %.1f%%\n\n",
		summary.Total, summary.Passed, summary.Failed, summary.Errors, summary.PassRate()*100))

	sb.WriteString("## By Category\n\n")
	sb.WriteString("| Category | Total | Passed | Rate |\n")
	sb.WriteString("|----------|-------|--------|------|\n")
	for cat, cr := range summary.ByCategory {
		rate := float64(0)
		if cr.Total > 0 {
			rate = float64(cr.Passed) / float64(cr.Total) * 100
		}
		sb.WriteString(fmt.Sprintf("| %s | %d | %d | %.0f%% |\n", cat, cr.Total, cr.Passed, rate))
	}

	// Show failures
	var failures []ConversationEvalResult
	for _, r := range summary.Results {
		if !r.Pass {
			failures = append(failures, r)
		}
	}
	if len(failures) > 0 {
		sb.WriteString("\n## Failures\n\n")
		for _, f := range failures {
			modeInfo := ""
		if f.ExpectedMode != "" || f.ActualMode != "" {
			modeInfo = fmt.Sprintf(" expected_mode=%s actual_mode=%s", f.ExpectedMode, f.ActualMode)
		}
		sb.WriteString(fmt.Sprintf("- **%s** (%s): intent=%q expected=%v actual=%v%s reply=%q\n",
				f.ScenarioID, f.Category, f.Intent, f.ExpectedTools, f.ActualTools, modeInfo, f.Reply))
			if f.Error != "" {
				sb.WriteString(fmt.Sprintf("  error: %s\n", f.Error))
			}
		}
	}

	return sb.String()
}

// findRepoRoot walks up from cwd looking for go.mod.
func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
