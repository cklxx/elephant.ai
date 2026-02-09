package agent_eval

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	ports "alex/internal/domain/agent/ports"
)

func TestLoadFoundationCaseSetValid(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "cases.yaml")
	content := `
version: "1"
name: "foundation"
scenarios:
  - id: "case-a"
    category: "browser"
    intent: "Find a selector and submit the form."
    expected_tools: ["browser_dom"]
    deliverable:
      output_description: "Capture proof screenshot and summary artifact"
      artifact_required: true
      required_evidence: ["screenshot"]
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write cases: %v", err)
	}

	set, err := LoadFoundationCaseSet(path)
	if err != nil {
		t.Fatalf("LoadFoundationCaseSet error: %v", err)
	}
	if set.Name != "foundation" {
		t.Fatalf("unexpected set name: %s", set.Name)
	}
	if len(set.Scenarios) != 1 {
		t.Fatalf("unexpected scenario count: %d", len(set.Scenarios))
	}
	if set.Scenarios[0].Deliverable == nil || !set.Scenarios[0].Deliverable.ArtifactRequired {
		t.Fatalf("expected deliverable contract parsed, got %+v", set.Scenarios[0].Deliverable)
	}
}

func TestLoadFoundationCaseSetRejectsDuplicateIDs(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "cases.yaml")
	content := `
version: "1"
name: "dup"
scenarios:
  - id: "same"
    category: "a"
    intent: "x"
    expected_tools: ["plan"]
  - id: "same"
    category: "b"
    intent: "y"
    expected_tools: ["clarify"]
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write cases: %v", err)
	}

	if _, err := LoadFoundationCaseSet(path); err == nil {
		t.Fatalf("expected duplicate id error")
	}
}

func TestLoadFoundationCaseSetRejectsEmptyDeliverableContract(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "cases.yaml")
	content := `
version: "1"
name: "invalid-deliverable"
scenarios:
  - id: "case-a"
    category: "artifact"
    intent: "Deliver a report."
    expected_tools: ["artifacts_write"]
    deliverable: {}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write cases: %v", err)
	}

	if _, err := LoadFoundationCaseSet(path); err == nil {
		t.Fatalf("expected empty deliverable contract error")
	}
}

func TestScoreToolProfileDetectsThinDefinition(t *testing.T) {
	t.Parallel()
	score := scoreToolProfile(foundationToolProfile{
		Definition: ports.ToolDefinition{
			Name:        "foo",
			Description: "run",
			Parameters: ports.ParameterSchema{
				Type: "object",
				Properties: map[string]ports.Property{
					"input": {Type: "", Description: "x"},
				},
				Required: []string{"input", "missing"},
			},
		},
		Metadata: ports.ToolMetadata{
			Name:        "foo",
			Category:    "",
			Tags:        nil,
			Dangerous:   false,
			SafetyLevel: 0,
		},
		TokenWeights: map[string]float64{"foo": 1},
	})

	if score.UsabilityScore >= 80 {
		t.Fatalf("expected low usability score, got %.1f", score.UsabilityScore)
	}
	if !slices.Contains(score.Issues, "required_property_missing") {
		t.Fatalf("expected required_property_missing issue, got %v", score.Issues)
	}
	if !slices.Contains(score.Issues, "property_type_missing") {
		t.Fatalf("expected property_type_missing issue, got %v", score.Issues)
	}
}

func TestEvaluateImplicitCasesComputesTopK(t *testing.T) {
	t.Parallel()
	scenarios := []FoundationScenario{
		{
			ID:            "pass-case",
			Category:      "web",
			Intent:        "Download and inspect full page content from a URL",
			ExpectedTools: []string{"web_fetch"},
		},
		{
			ID:            "fail-case",
			Category:      "visual",
			Intent:        "Render architecture as a picture",
			ExpectedTools: []string{"web_search"},
		},
	}
	profiles := []foundationToolProfile{
		{
			Definition: ports.ToolDefinition{Name: "web_fetch"},
			TokenWeights: map[string]float64{
				"download": 3, "content": 3, "web": 2, "url": 2, "fetch": 4,
			},
		},
		{
			Definition: ports.ToolDefinition{Name: "web_search"},
			TokenWeights: map[string]float64{
				"search": 4, "web": 2, "query": 3,
			},
		},
	}

	summary := evaluateImplicitCases(scenarios, profiles, 2)
	if summary.TotalCases != 2 {
		t.Fatalf("unexpected total cases: %d", summary.TotalCases)
	}
	if summary.PassedCases != 1 {
		t.Fatalf("expected 1 passed case, got %d", summary.PassedCases)
	}
	if summary.FailedCases != 1 {
		t.Fatalf("expected 1 failed case, got %d", summary.FailedCases)
	}
	if summary.PassAt5Rate <= 0 {
		t.Fatalf("expected non-zero pass@5 rate, got pass@1=%.3f pass@5=%.3f", summary.PassAt1Rate, summary.PassAt5Rate)
	}
}

func TestEvaluateImplicitCasesAvailabilityMarkedNA(t *testing.T) {
	t.Parallel()

	scenarios := []FoundationScenario{
		{
			ID:            "na-case",
			Category:      "availability",
			Intent:        "Need a tool that is unavailable in this runtime.",
			ExpectedTools: []string{"nonexistent_tool"},
		},
	}
	profiles := []foundationToolProfile{
		{
			Definition:   ports.ToolDefinition{Name: "plan"},
			TokenWeights: map[string]float64{"plan": 2},
		},
	}

	summary := evaluateImplicitCases(scenarios, profiles, 3)
	if summary.TotalCases != 1 {
		t.Fatalf("expected total cases 1, got %d", summary.TotalCases)
	}
	if summary.ApplicableCases != 0 {
		t.Fatalf("expected applicable cases 0, got %d", summary.ApplicableCases)
	}
	if summary.NotApplicableCases != 1 {
		t.Fatalf("expected N/A cases 1, got %d", summary.NotApplicableCases)
	}
	if summary.FailedCases != 0 {
		t.Fatalf("expected failed cases 0 for N/A scenario, got %d", summary.FailedCases)
	}
	if len(summary.CaseResults) != 1 || !summary.CaseResults[0].NotApplicable {
		t.Fatalf("expected case marked NotApplicable, got %+v", summary.CaseResults)
	}
}

func TestEvaluateImplicitCasesComputesDeliverableChecks(t *testing.T) {
	t.Parallel()

	scenarios := []FoundationScenario{
		{
			ID:            "deliverable-case",
			Category:      "artifact",
			Intent:        "Persist output artifact, verify manifest, and attach to user message.",
			ExpectedTools: []string{"artifacts_write"},
			Deliverable: &FoundationDeliverableContract{
				OutputDescription:  "artifact + manifest + attachment",
				ArtifactRequired:   true,
				AttachmentRequired: true,
				ManifestRequired:   true,
			},
		},
	}
	profiles := []foundationToolProfile{
		{
			Definition: ports.ToolDefinition{Name: "artifacts_write"},
			TokenWeights: map[string]float64{
				"artifact": 6, "persist": 5, "output": 4,
			},
		},
		{
			Definition: ports.ToolDefinition{Name: "artifact_manifest"},
			TokenWeights: map[string]float64{
				"manifest": 6, "verify": 4, "artifact": 4,
			},
		},
		{
			Definition: ports.ToolDefinition{Name: "write_attachment"},
			TokenWeights: map[string]float64{
				"attach": 6, "message": 4, "user": 3,
			},
		},
	}

	summary := evaluateImplicitCases(scenarios, profiles, 3)
	if len(summary.CaseResults) != 1 {
		t.Fatalf("expected single case result, got %d", len(summary.CaseResults))
	}
	check := summary.CaseResults[0].DeliverableCheck
	if check == nil || !check.Applicable {
		t.Fatalf("expected deliverable check, got %+v", check)
	}
	if check.SignalCount == 0 || check.MatchedSignals == 0 {
		t.Fatalf("expected non-empty signal matching, got %+v", check)
	}
	if check.Status != "good" {
		t.Fatalf("expected good deliverable check, got %+v", check)
	}
}

func TestRankToolsForIntentCriticalFoundationCases(t *testing.T) {
	t.Parallel()

	makeProfile := func(name string, tokens map[string]float64) foundationToolProfile {
		return foundationToolProfile{
			Definition:   ports.ToolDefinition{Name: name},
			TokenWeights: tokens,
		}
	}

	cases := []struct {
		name     string
		intent   string
		expected string
		profiles []foundationToolProfile
	}{
		{
			name:     "search-file-cross-project",
			intent:   "Locate all occurrences of a specific symbol across project files.",
			expected: "search_file",
			profiles: []foundationToolProfile{
				makeProfile("file_edit", map[string]float64{"file": 7, "edit": 6, "replace": 6, "create": 5, "content": 5, "search": 5}),
				makeProfile("web_search", map[string]float64{"search": 7, "web": 6, "query": 5}),
				makeProfile("replace_in_file", map[string]float64{"replace": 8, "file": 7, "update": 5, "search": 3}),
				makeProfile("search_file", map[string]float64{"search": 6, "regex": 6, "pattern": 5, "symbol": 5, "token": 4, "file": 5}),
			},
		},
		{
			name:     "browser-session-inspection",
			intent:   "Check browser tab state, current URL, and session metadata.",
			expected: "browser_info",
			profiles: []foundationToolProfile{
				makeProfile("web_fetch", map[string]float64{"web": 8, "fetch": 7, "url": 6}),
				makeProfile("artifact_manifest", map[string]float64{"artifact": 8, "manifest": 7, "metadata": 6}),
				makeProfile("browser_action", map[string]float64{"browser": 7, "action": 6, "click": 5}),
				makeProfile("browser_info", map[string]float64{"browser": 7, "metadata": 7, "info": 6, "status": 6, "url": 5}),
			},
		},
		{
			name:     "create-markdown-note",
			intent:   "Create a new markdown note file with the provided content.",
			expected: "write_file",
			profiles: []foundationToolProfile{
				makeProfile("file_edit", map[string]float64{"file": 8, "edit": 7, "create": 7, "new": 5, "content": 6}),
				makeProfile("replace_in_file", map[string]float64{"replace": 8, "file": 7, "update": 6, "content": 5}),
				makeProfile("read_file", map[string]float64{"read": 8, "file": 6, "content": 5}),
				makeProfile("write_file", map[string]float64{"write": 8, "file": 7, "content": 6, "create": 5, "report": 4}),
			},
		},
		{
			name:     "list-workspace-directory",
			intent:   "Show files and folders under a target directory in the workspace.",
			expected: "list_dir",
			profiles: []foundationToolProfile{
				makeProfile("file_edit", map[string]float64{"file": 8, "edit": 7, "replace": 6, "directory": 5, "workspace": 4}),
				makeProfile("replace_in_file", map[string]float64{"replace": 8, "file": 7, "path": 6, "directory": 5}),
				makeProfile("read_file", map[string]float64{"read": 8, "file": 7, "path": 6}),
				makeProfile("list_dir", map[string]float64{"list": 8, "directory": 7, "folder": 6, "workspace": 6, "file": 6, "browse": 4}),
			},
		},
		{
			name:     "write-attachment-downloadable",
			intent:   "Write a generated file as downloadable attachment for the user.",
			expected: "write_attachment",
			profiles: []foundationToolProfile{
				makeProfile("file_edit", map[string]float64{"file": 8, "edit": 7, "write": 7, "create": 6, "content": 6}),
				makeProfile("write_file", map[string]float64{"write": 8, "file": 7, "content": 6, "create": 5}),
				makeProfile("replace_in_file", map[string]float64{"replace": 8, "file": 7, "update": 6}),
				makeProfile("write_attachment", map[string]float64{"write": 6, "attach": 7, "download": 6, "artifact": 6, "generated": 4}),
			},
		},
		{
			name:     "find-files-by-name",
			intent:   "Find files whose names contain migration in the repository.",
			expected: "find",
			profiles: []foundationToolProfile{
				makeProfile("file_edit", map[string]float64{"file": 8, "edit": 7, "replace": 6, "create": 5, "find": 3}),
				makeProfile("search_file", map[string]float64{"search": 8, "file": 7, "pattern": 6, "find": 4}),
				makeProfile("replace_in_file", map[string]float64{"replace": 8, "file": 7, "path": 6}),
				makeProfile("find", map[string]float64{"find": 7, "name": 6, "directory": 5, "path": 5, "file": 4}),
			},
		},
		{
			name:     "grep-error-log-lines",
			intent:   "Search error.log for HTTP 500 lines with a simple grep pattern.",
			expected: "grep",
			profiles: []foundationToolProfile{
				makeProfile("search_file", map[string]float64{"search": 8, "file": 7, "pattern": 6, "log": 3}),
				makeProfile("ripgrep", map[string]float64{"search": 7, "regex": 7, "pattern": 6, "file": 5}),
				makeProfile("web_search", map[string]float64{"search": 7, "web": 6, "query": 5}),
				makeProfile("grep", map[string]float64{"grep": 7, "log": 7, "error": 6, "line": 6, "pattern": 5}),
			},
		},
		{
			name:     "lark-calendar-query",
			intent:   "Proactively check upcoming calendar events before scheduling work.",
			expected: "lark_calendar_query",
			profiles: []foundationToolProfile{
				makeProfile("lark_calendar_update", map[string]float64{"calendar": 7, "event": 6, "update": 6, "change": 5}),
				makeProfile("lark_calendar_delete", map[string]float64{"calendar": 7, "event": 6, "delete": 6, "remove": 5}),
				makeProfile("lark_calendar_create", map[string]float64{"calendar": 7, "event": 6, "create": 6, "new": 5}),
				makeProfile("lark_calendar_query", map[string]float64{"calendar": 7, "event": 6, "query": 6, "check": 5, "upcoming": 5}),
			},
		},
		{
			name:     "artifacts-delete-stale",
			intent:   "Delete obsolete artifacts created by earlier failed runs.",
			expected: "artifacts_delete",
			profiles: []foundationToolProfile{
				makeProfile("lark_task_manage", map[string]float64{"task": 7, "manage": 6, "delete": 4, "remove": 4}),
				makeProfile("lark_calendar_delete", map[string]float64{"calendar": 7, "event": 6, "delete": 6, "remove": 5}),
				makeProfile("artifacts_list", map[string]float64{"artifact": 7, "list": 7, "generated": 5}),
				makeProfile("artifacts_delete", map[string]float64{"artifact": 7, "delete": 7, "remove": 6, "stale": 5, "obsolete": 5, "legacy": 4}),
			},
		},
		{
			name:     "okr-write-progress",
			intent:   "Update OKR objective progress after completing milestone work.",
			expected: "okr_write",
			profiles: []foundationToolProfile{
				makeProfile("replace_in_file", map[string]float64{"replace": 8, "update": 7, "file": 6, "path": 5}),
				makeProfile("todo_update", map[string]float64{"update": 7, "todo": 7, "task": 6}),
				makeProfile("lark_task_manage", map[string]float64{"task": 7, "manage": 6, "update": 5}),
				makeProfile("okr_write", map[string]float64{"okr": 7, "write": 6, "objective": 6, "progress": 6, "update": 5}),
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ranked := rankToolsForIntent(tokenize(tc.intent), tc.profiles)
			rank := 0
			for idx, match := range ranked {
				if match.Name == tc.expected {
					rank = idx + 1
					break
				}
			}
			if rank == 0 || rank > 3 {
				limit := 3
				if len(ranked) < limit {
					limit = len(ranked)
				}
				top := make([]string, 0, limit)
				for i := 0; i < limit; i++ {
					top = append(top, ranked[i].Name)
				}
				t.Fatalf("expected %s in top-3, got rank=%d (top=%v)", tc.expected, rank, top)
			}
		})
	}
}

func TestEvaluateImplicitCasesMarksAvailabilityAsNotApplicable(t *testing.T) {
	t.Parallel()
	scenarios := []FoundationScenario{
		{
			ID:            "availability-fail",
			Category:      "artifact",
			Intent:        "List generated artifacts for this run.",
			ExpectedTools: []string{"artifacts_list"},
		},
	}
	profiles := []foundationToolProfile{
		{
			Definition: ports.ToolDefinition{Name: "artifact_manifest"},
			TokenWeights: map[string]float64{
				"artifact": 4, "manifest": 5,
			},
		},
	}

	summary := evaluateImplicitCases(scenarios, profiles, 3)
	if summary.TotalCases != 1 || summary.NotApplicableCases != 1 || summary.FailedCases != 0 {
		t.Fatalf("expected one N/A case and zero failed cases, got %+v", summary)
	}
	result := summary.CaseResults[0]
	if result.FailureType != "availability_error" {
		t.Fatalf("expected availability_error, got %q", result.FailureType)
	}
	if !result.NotApplicable {
		t.Fatalf("expected NotApplicable result when expected tool is unavailable")
	}
}

func TestHeuristicIntentBoostTargetsRecentFailurePatterns(t *testing.T) {
	t.Parallel()

	makeSet := func(tokens ...string) map[string]struct{} {
		out := make(map[string]struct{}, len(tokens))
		for _, token := range tokens {
			out[token] = struct{}{}
		}
		return out
	}

	if boost := heuristicIntentBoost("request_user", makeSet("manual", "approval", "continue")); boost <= 0 {
		t.Fatalf("expected request_user boost > 0, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("memory_get", makeSet("open", "exact", "fragment", "offset", "citation")); boost <= 0 {
		t.Fatalf("expected memory_get boost > 0, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("memory_search", makeSet("preference", "habit", "format", "choice")); boost <= 0 {
		t.Fatalf("expected memory_search boost > 0, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("memory_search", makeSet("recover", "interaction", "habit")); boost <= 0 {
		t.Fatalf("expected memory_search recovery boost > 0, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("memory_search", makeSet("before", "offset", "exact")); boost <= 0 {
		t.Fatalf("expected memory_search before-offset boost > 0, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("ripgrep", makeSet("regex", "needle", "sweep", "repo", "fast")); boost <= 0 {
		t.Fatalf("expected ripgrep boost > 0, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("web_search", makeSet("official", "reference", "discover", "canonical")); boost <= 0 {
		t.Fatalf("expected web_search authority boost > 0, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("lark_upload_file", makeSet("upload", "file", "lark", "thread")); boost <= 0 {
		t.Fatalf("expected lark_upload_file thread upload boost > 0, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("cancel_timer", makeSet("cancel", "stale", "duplicate", "reminder")); boost <= 0 {
		t.Fatalf("expected cancel_timer cleanup boost > 0, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("artifacts_write", makeSet("concise", "chat", "durable", "artifact", "report")); boost <= 0 {
		t.Fatalf("expected artifacts_write delivery boost > 0, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("clarify", makeSet("manual", "approval", "user", "confirm")); boost >= 10 {
		t.Fatalf("expected clarify to be penalized under manual-approval gate intents, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("write_attachment", makeSet("concise", "chat", "durable", "artifact", "report")); boost >= 20 {
		t.Fatalf("expected write_attachment to be penalized for durable artifact intent, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("write_attachment", makeSet("upload", "file", "lark", "thread")); boost >= 20 {
		t.Fatalf("expected write_attachment to be penalized for lark upload intents, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("search_file", makeSet("memory", "prior", "note", "recall")); boost >= 0 {
		t.Fatalf("expected search_file penalty under memory retrieval intent, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("lark_upload_file", makeSet("chat", "history", "context", "recent", "before")); boost >= 0 {
		t.Fatalf("expected lark_upload_file penalty for context-only intents, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("lark_chat_history", makeSet("chat", "history", "context", "recent", "before")); boost <= 0 {
		t.Fatalf("expected lark_chat_history boost for context intents, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("acp_executor", makeSet("delegate", "heavy", "parallel", "executor")); boost <= 0 {
		t.Fatalf("expected acp_executor delegation boost > 0, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("clarify", makeSet("delegate", "executor", "parallel", "heavy")); boost >= 0 {
		t.Fatalf("expected clarify penalty for delegation intents, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("search_file", makeSet("official", "rfc", "web", "source")); boost >= 0 {
		t.Fatalf("expected search_file penalty for official web research intents, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("shell_exec", makeSet("shell", "command", "port", "process", "check")); boost <= 0 {
		t.Fatalf("expected shell_exec command/port boost > 0, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("set_timer", makeSet("scheduler", "job", "daily", "cadence")); boost >= 0 {
		t.Fatalf("expected set_timer penalty under scheduler-job intents, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("cancel_timer", makeSet("list", "active", "remaining", "timer")); boost >= 0 {
		t.Fatalf("expected cancel_timer penalty for list-active intents, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("request_user", makeSet("sensitive", "personal", "confirmation")); boost <= 0 {
		t.Fatalf("expected request_user boost for sensitive confirmation intents, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("scheduler_create_job", makeSet("recurring", "weekday", "followup", "checkin", "job")); boost <= 0 {
		t.Fatalf("expected scheduler_create_job boost for recurring check-in intents, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("scheduler_create_job", makeSet("schedule", "automatic", "followup", "status", "reply", "no")); boost <= 0 {
		t.Fatalf("expected scheduler_create_job boost for automatic follow-up intents, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("scheduler_delete_job", makeSet("obsolete", "scheduler", "checkin", "job", "remove")); boost <= 0 {
		t.Fatalf("expected scheduler_delete_job boost for obsolete check-in intents, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("artifacts_write", makeSet("progress", "momentum", "completed", "artifact", "summary")); boost <= 0 {
		t.Fatalf("expected artifacts_write motivation-progress boost > 0, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("memory_search", makeSet("motivation", "previous", "successful", "pattern", "recall")); boost <= 0 {
		t.Fatalf("expected memory_search boost for motivation pattern recall intents, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("search_file", makeSet("motivation", "previous", "successful", "pattern", "memory", "recall")); boost >= 0 {
		t.Fatalf("expected search_file penalty for motivation-memory recall intents, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("clarify", makeSet("create", "calendar", "event", "focus", "recovery", "block")); boost >= 0 {
		t.Fatalf("expected clarify penalty for actionable create-calendar intents, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("lark_calendar_create", makeSet("create", "calendar", "focus", "recovery", "block")); boost <= 0 {
		t.Fatalf("expected lark_calendar_create boost for focus/recovery block intents, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("write_attachment", makeSet("downloadable", "summary", "attachment", "handoff")); boost <= 0 {
		t.Fatalf("expected write_attachment boost for downloadable summary handoff intents, got %.2f", boost)
	}
	if boost := heuristicIntentBoost("set_timer", makeSet("conflict", "interrupt", "reminder", "boundary")); boost >= 0 {
		t.Fatalf("expected set_timer penalty for interruption-boundary conflict intents, got %.2f", boost)
	}
	searchBoost := heuristicIntentBoost("search_file", makeSet("regex", "needle", "sweep", "repo", "fast"))
	rgBoost := heuristicIntentBoost("ripgrep", makeSet("regex", "needle", "sweep", "repo", "fast"))
	if rgBoost <= searchBoost {
		t.Fatalf("expected ripgrep boost (%.2f) to exceed search_file boost (%.2f)", rgBoost, searchBoost)
	}

	searchFirstBoost := heuristicIntentBoost("web_search", makeSet("no", "source", "selected", "official", "reference"))
	fetchBoost := heuristicIntentBoost("web_fetch", makeSet("no", "source", "selected", "official", "reference"))
	if searchFirstBoost <= fetchBoost {
		t.Fatalf("expected web_search boost (%.2f) to exceed web_fetch boost (%.2f) when source is not selected", searchFirstBoost, fetchBoost)
	}

	fetchExactBoost := heuristicIntentBoost("web_fetch", makeSet("fetch", "exact", "single", "provided", "url", "page"))
	searchExactBoost := heuristicIntentBoost("web_search", makeSet("fetch", "exact", "single", "provided", "url", "page"))
	if fetchExactBoost <= searchExactBoost {
		t.Fatalf("expected web_fetch boost (%.2f) to exceed web_search boost (%.2f) for exact provided URL", fetchExactBoost, searchExactBoost)
	}
}

func TestRunFoundationEvaluationEndToEnd(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	casePath := filepath.Join(tmp, "cases.yaml")
	caseYAML := `
version: "1"
name: "mini"
scenarios:
  - id: "one"
    category: "planning"
    intent: "Break this task into milestones and explicit checkpoints."
    expected_tools: ["plan"]
  - id: "two"
    category: "browser"
    intent: "Find selectors on a webpage and submit a form."
    expected_tools: ["browser_dom"]
`
	if err := os.WriteFile(casePath, []byte(caseYAML), 0644); err != nil {
		t.Fatalf("write case yaml: %v", err)
	}

	result, err := RunFoundationEvaluation(context.Background(), &FoundationEvaluationOptions{
		OutputDir:    filepath.Join(tmp, "out"),
		Mode:         "web",
		Preset:       "full",
		Toolset:      "default",
		CasesPath:    casePath,
		TopK:         3,
		ReportFormat: "json",
	})
	if err != nil {
		t.Fatalf("RunFoundationEvaluation error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected result, got nil")
	}
	if result.Prompt.TotalPrompts == 0 {
		t.Fatalf("expected prompt scores")
	}
	if result.Tools.TotalTools == 0 {
		t.Fatalf("expected tool scores")
	}
	if result.Implicit.TotalCases != 2 {
		t.Fatalf("expected 2 cases, got %d", result.Implicit.TotalCases)
	}
	if len(result.ReportArtifacts) == 0 {
		t.Fatalf("expected report artifacts")
	}
	if _, err := os.Stat(result.ReportArtifacts[0].Path); err != nil {
		t.Fatalf("artifact not written: %v", err)
	}
}
