package tviewui

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"alex/cmd/alex/ui/eventhub"
	"alex/cmd/alex/ui/state"
	"alex/internal/agent/ports"
	"alex/internal/agent/types"
	"alex/internal/mcp"

	"github.com/stretchr/testify/require"
)

func TestFormatMessage_UserAssistantSystem(t *testing.T) {
	ts := time.Now()
	cases := []struct {
		name     string
		message  state.ChatMessage
		expected string
	}{
		{
			name: "user",
			message: state.ChatMessage{
				Role:      state.RoleUser,
				Content:   "Hello",
				CreatedAt: ts,
			},
			expected: "[yellow::b]You[-]\nHello",
		},
		{
			name: "assistant",
			message: state.ChatMessage{
				Role:      state.RoleAssistant,
				AgentID:   "planner",
				Content:   "Hi there",
				CreatedAt: ts,
			},
			expected: "[cyan::b]Alex[-] [gray](planner)[-]\nHi there",
		},
		{
			name: "system",
			message: state.ChatMessage{
				Role:      state.RoleSystem,
				Content:   "Notice",
				CreatedAt: ts,
			},
			expected: "[red::b]System[-]\nNotice",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatMessage(tc.message)
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestRenderTranscript(t *testing.T) {
	now := time.Now()
	transcript := renderTranscript([]state.ChatMessage{
		{
			Role:      state.RoleUser,
			Content:   "First",
			CreatedAt: now,
		},
		{
			Role:      state.RoleAssistant,
			AgentID:   "core",
			Content:   "Second",
			CreatedAt: now,
		},
	})

	expected := "[yellow::b]You[-]\nFirst\n\n[cyan::b]Alex[-] [gray](core)[-]\nSecond"
	if transcript != expected {
		t.Fatalf("unexpected transcript: %q", transcript)
	}
}

func TestNewChatUIFollowDefaults(t *testing.T) {
	coord := newStubCoordinator(nil)

	cases := []struct {
		name             string
		followTranscript *bool
		followStream     *bool
		wantTranscript   bool
		wantStream       bool
	}{
		{
			name:           "defaults",
			wantTranscript: true,
			wantStream:     true,
		},
		{
			name:             "overrides",
			followTranscript: boolPtr(false),
			followStream:     boolPtr(false),
			wantTranscript:   false,
			wantStream:       false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ui, err := NewChatUI(Config{
				Coordinator:      coord,
				FollowTranscript: tc.followTranscript,
				FollowStream:     tc.followStream,
			})
			require.NoError(t, err)
			require.NotNil(t, ui)
			require.Equal(t, tc.wantTranscript, ui.followTranscript.Load())
			require.Equal(t, tc.wantStream, ui.followStream.Load())
			ui.shutdown()
		})
	}
}

func TestChatUIApplySessionSnapshotRestoresFollowDefaults(t *testing.T) {
	coord := newStubCoordinator(nil)
	ui, err := NewChatUI(Config{
		Coordinator:      coord,
		FollowTranscript: boolPtr(false),
		FollowStream:     boolPtr(false),
	})
	require.NoError(t, err)
	defer ui.shutdown()

	ui.followTranscript.Store(true)
	ui.followStream.Store(true)

	ui.applySessionSnapshot(nil)

	require.False(t, ui.followTranscript.Load())
	require.False(t, ui.followStream.Load())
}

func TestRenderStatus(t *testing.T) {
	baseSnapshot := state.Snapshot{}
	tests := []struct {
		name     string
		pending  int
		snapshot state.Snapshot
		session  string
		search   string
		verbose  bool
		expected string
	}{
		{
			name:     "ready",
			snapshot: baseSnapshot,
			expected: "[green::b]Ready[-]  •  [white]Session: (new)[-]  •  [cyan]Tools: 0 active[-]  •  [magenta]MCP: none[-]  •  [blue]Subagents: 0 running[-]  •  [gray]Verbose: off[-]  •  [gray]? Help  Tab focus  End follow  / search  n/N next[-]",
		},
		{
			name:    "one task",
			pending: 1,
			snapshot: state.Snapshot{
				ToolRuns:     []*state.ToolRun{{Status: state.ToolStatusRunning}},
				MCPServers:   []*state.MCPServer{{Status: state.MCPStatusReady}},
				SubagentRuns: []*state.SubagentTask{{Status: state.SubtaskStatusRunning}},
			},
			session:  "session-123",
			verbose:  true,
			expected: "[yellow::b]Running task…[-]  •  [white]Session: session-123[-]  •  [cyan]Tools: 1 active[-]  •  [magenta]MCP: 1/1 ready[-]  •  [blue]Subagents: 1 running[-]  •  [gray]Verbose: on[-]  •  [gray]? Help  Tab focus  End follow  / search  n/N next[-]",
		},
		{
			name:    "with search",
			pending: 2,
			search:  "Search \"foo\": 2/5",
			snapshot: state.Snapshot{
				ToolRuns: []*state.ToolRun{{Status: state.ToolStatusRunning}, {Status: state.ToolStatusRunning}},
			},
			expected: "[yellow::b]Running 2 tasks…[-]  •  [white]Session: (new)[-]  •  [cyan]Tools: 2 active[-]  •  [magenta]MCP: none[-]  •  [blue]Subagents: 0 running[-]  •  [white]Search \"foo\": 2/5[-]  •  [gray]Verbose: off[-]  •  [gray]? Help  Tab focus  End follow  / search  n/N next[-]",
		},
		{
			name: "with tokens",
			snapshot: state.Snapshot{
				Metrics: state.Metrics{
					TotalTokens:   180,
					TokensByAgent: map[string]int{"core": 150, "subagent": 30},
				},
			},
			expected: "[green::b]Ready[-]  •  [white]Session: (new)[-]  •  [cyan]Tools: 0 active[-]  •  [magenta]MCP: none[-]  •  [blue]Subagents: 0 running[-]  •  [white]Tokens: 180 (core=150, subagent=30)[-]  •  [gray]Verbose: off[-]  •  [gray]? Help  Tab focus  End follow  / search  n/N next[-]",
		},
		{
			name: "with cost",
			snapshot: state.Snapshot{
				Metrics: state.Metrics{
					TotalCost:   0.4321,
					CostByModel: map[string]float64{"gpt-4": 0.3, "gpt-4o": 0.1321},
				},
			},
			expected: "[green::b]Ready[-]  •  [white]Session: (new)[-]  •  [cyan]Tools: 0 active[-]  •  [magenta]MCP: none[-]  •  [blue]Subagents: 0 running[-]  •  [white]Cost: $0.4321 (gpt-4=$0.3000, gpt-4o=$0.1321)[-]  •  [gray]Verbose: off[-]  •  [gray]? Help  Tab focus  End follow  / search  n/N next[-]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := renderStatus(tc.pending, tc.snapshot, tc.session, tc.search, tc.verbose)
			if got != tc.expected {
				t.Fatalf("expected %q got %q", tc.expected, got)
			}
		})
	}
}

func TestRenderLiveStream(t *testing.T) {
	stream := renderLiveStream([]*state.ToolRun{{
		ToolName:  "search",
		AgentID:   "core",
		Status:    state.ToolStatusRunning,
		Stream:    []string{"partial", " result"},
		UpdatedAt: time.Now(),
	}})

	if !strings.Contains(stream, "search") {
		t.Fatalf("expected stream to include tool name: %q", stream)
	}
	if !strings.Contains(stream, "partial result") {
		t.Fatalf("expected stream to include content: %q", stream)
	}
}

func TestRenderCostSummary(t *testing.T) {
	if renderCostSummary(state.Metrics{}) != "" {
		t.Fatalf("expected empty summary when no cost present")
	}

	metrics := state.Metrics{TotalCost: 0.12345}
	if got := renderCostSummary(metrics); got != "[white]Cost: $0.1235[-]" {
		t.Fatalf("unexpected rounding: %q", got)
	}

	metrics = state.Metrics{
		TotalCost:   0.4321,
		CostByModel: map[string]float64{"gpt-4": 0.3, "gpt-4o": 0.1321},
	}

	expected := "[white]Cost: $0.4321 (gpt-4=$0.3000, gpt-4o=$0.1321)[-]"
	if got := renderCostSummary(metrics); got != expected {
		t.Fatalf("expected %q got %q", expected, got)
	}
}

func TestFormatCostSummary(t *testing.T) {
	start := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(5 * time.Minute)
	summary := &ports.CostSummary{
		TotalCost:    0.4321,
		RequestCount: 3,
		InputTokens:  1200,
		OutputTokens: 345,
		TotalTokens:  1545,
		ByModel:      map[string]float64{"gpt-4": 0.3, "gpt-4o": 0.1321},
		ByProvider:   map[string]float64{"openai": 0.4321},
		StartTime:    start,
		EndTime:      end,
	}

	expected := strings.Join([]string{
		"Cost for session session-123:",
		" • Total: $0.4321",
		" • Requests: 3",
		" • Tokens: input=1200 output=345 total=1545",
		" • Models: gpt-4=$0.3000, gpt-4o=$0.1321",
		" • Providers: openai=$0.4321",
		" • Window: 2024-06-01T12:00:00Z → 2024-06-01T12:05:00Z",
	}, "\n")

	if got := formatCostSummary("session-123", summary); got != expected {
		t.Fatalf("expected formatted summary to match.\nexpected:\n%s\n\nactual:\n%s", expected, got)
	}
}

func TestCostCommandScenarios(t *testing.T) {
	t.Run("unavailable tracker", func(t *testing.T) {
		ui := &ChatUI{store: state.NewStore(), hub: eventhub.NewHub()}
		ui.searchSummary.Store("")
		ui.costCommand(nil)

		snapshot := ui.store.Snapshot()
		require.NotEmpty(t, snapshot.Messages)
		require.Equal(t, "Cost tracker unavailable.", snapshot.Messages[len(snapshot.Messages)-1].Content)
	})

	t.Run("no active session", func(t *testing.T) {
		tracker := &stubCostTracker{}
		ui := &ChatUI{store: state.NewStore(), hub: eventhub.NewHub(), costTracker: tracker}
		ui.searchSummary.Store("")
		ui.costCommand(nil)

		snapshot := ui.store.Snapshot()
		require.NotEmpty(t, snapshot.Messages)
		require.Contains(t, snapshot.Messages[len(snapshot.Messages)-1].Content, "No active session")
		require.Zero(t, tracker.callCount())
	})

	t.Run("error fetching summary", func(t *testing.T) {
		tracker := &stubCostTracker{err: errors.New("boom")}
		ui := &ChatUI{store: state.NewStore(), hub: eventhub.NewHub(), costTracker: tracker, ctx: context.Background()}
		ui.searchSummary.Store("")
		ui.sessionID = "session-err"

		ui.costCommand(nil)

		snapshot := ui.store.Snapshot()
		require.NotEmpty(t, snapshot.Messages)
		require.Contains(t, snapshot.Messages[len(snapshot.Messages)-1].Content, "Failed to load cost")
		require.Equal(t, "session-err", tracker.lastSession())
	})

	t.Run("no data yet", func(t *testing.T) {
		tracker := &stubCostTracker{}
		ui := &ChatUI{store: state.NewStore(), hub: eventhub.NewHub(), costTracker: tracker, ctx: context.Background()}
		ui.searchSummary.Store("")
		ui.sessionID = "session-empty"

		ui.costCommand(nil)

		snapshot := ui.store.Snapshot()
		require.NotEmpty(t, snapshot.Messages)
		require.Contains(t, snapshot.Messages[len(snapshot.Messages)-1].Content, "No cost data recorded")
		require.Equal(t, "session-empty", tracker.lastSession())
	})

	t.Run("override session argument", func(t *testing.T) {
		tracker := &stubCostTracker{}
		tracker.summary = &ports.CostSummary{TotalCost: 0.25, RequestCount: 1}
		ui := &ChatUI{store: state.NewStore(), hub: eventhub.NewHub(), costTracker: tracker, ctx: context.Background()}
		ui.searchSummary.Store("")
		ui.sessionID = "session-default"

		ui.costCommand([]string{"explicit-session"})

		snapshot := ui.store.Snapshot()
		require.NotEmpty(t, snapshot.Messages)
		require.Contains(t, snapshot.Messages[len(snapshot.Messages)-1].Content, "explicit-session")
		require.Equal(t, "explicit-session", tracker.lastSession())
	})

	t.Run("successful summary", func(t *testing.T) {
		tracker := &stubCostTracker{}
		tracker.summary = &ports.CostSummary{
			TotalCost:    0.4321,
			RequestCount: 2,
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		}
		ui := &ChatUI{store: state.NewStore(), hub: eventhub.NewHub(), costTracker: tracker, ctx: context.Background()}
		ui.searchSummary.Store("")
		ui.sessionID = "session-xyz"

		ui.costCommand(nil)

		snapshot := ui.store.Snapshot()
		require.NotEmpty(t, snapshot.Messages)
		message := snapshot.Messages[len(snapshot.Messages)-1].Content
		require.Contains(t, message, "Cost for session session-xyz")
		require.Contains(t, message, "$0.4321")
		require.Equal(t, 1, tracker.callCount())
	})
}

func TestChatUICostRefresh(t *testing.T) {
	tracker := &stubCostTracker{summary: &ports.CostSummary{
		TotalCost: 0.42,
		ByModel:   map[string]float64{"gpt-4": 0.42},
		EndTime:   time.Now(),
	}}

	ui := &ChatUI{
		store:       state.NewStore(),
		hub:         eventhub.NewHub(),
		costTracker: tracker,
		ctx:         context.Background(),
	}
	ui.searchSummary.Store("")
	ui.sessionID = "session-xyz"

	ui.queueCostRefresh()

	require.Eventually(t, func() bool {
		snapshot := ui.store.Snapshot()
		return math.Abs(snapshot.Metrics.TotalCost-0.42) < 1e-6
	}, time.Second, 10*time.Millisecond)

	require.InDelta(t, 0.42, ui.store.Snapshot().Metrics.TotalCost, 1e-6)
	require.Eventually(t, func() bool {
		return tracker.callCount() >= 1 && !ui.costRefreshPending.Load()
	}, time.Second, 10*time.Millisecond)

	ui.queueCostRefresh()
	require.Eventually(t, func() bool {
		return tracker.callCount() >= 2
	}, time.Second, 10*time.Millisecond)
}

func TestRenderToolRuns(t *testing.T) {
	text := renderToolRuns([]*state.ToolRun{{
		ToolName:  "search",
		AgentID:   "core",
		Status:    state.ToolStatusCompleted,
		Result:    "done",
		Duration:  time.Second,
		UpdatedAt: time.Now(),
	}})

	if !strings.Contains(text, "search") || !strings.Contains(text, "done") {
		t.Fatalf("unexpected tool run rendering: %q", text)
	}
}

func TestRenderSubagents(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(3 * time.Second)
	text := renderSubagents([]*state.SubagentTask{{
		Index:          1,
		Preview:        "Plan things",
		Status:         state.SubtaskStatusCompleted,
		CurrentTool:    "",
		ToolsCompleted: 3,
		Total:          3,
		TokensUsed:     42,
		AgentLevel:     "planner",
		StartedAt:      &start,
		CompletedAt:    &end,
		Duration:       end.Sub(start),
	}})

	if !strings.Contains(text, "Plan things") {
		t.Fatalf("expected preview in output: %q", text)
	}
	if !strings.Contains(text, "3/3 tools") {
		t.Fatalf("expected tool counts in output: %q", text)
	}
	if !strings.Contains(text, "(planner)") {
		t.Fatalf("expected agent level in output: %q", text)
	}
	if !strings.Contains(text, "3.0s") {
		t.Fatalf("expected duration in output: %q", text)
	}
	if !strings.Contains(text, "finished 12:00:03") {
		t.Fatalf("expected completion timestamp in output: %q", text)
	}
}

func TestRenderMCPServers(t *testing.T) {
	now := time.Now()
	text := renderMCPServers([]*state.MCPServer{{
		Name:      "search",
		Status:    state.MCPStatusReady,
		StartedAt: &now,
	}}, "")

	if !strings.Contains(text, "search") {
		t.Fatalf("unexpected MCP rendering: %q", text)
	}
}

func TestRenderMCPServersSpinner(t *testing.T) {
	text := renderMCPServers([]*state.MCPServer{{
		Name:   "startup",
		Status: state.MCPStatusStarting,
	}}, "⠋")

	if !strings.Contains(text, "⠋") {
		t.Fatalf("expected spinner glyph in MCP output: %q", text)
	}
}

func TestFindMatchingLines(t *testing.T) {
	lines := findMatchingLines("First\nsecond\nThird", "Sec")
	if len(lines) != 1 || lines[0] != 1 {
		t.Fatalf("expected match on second line, got %v", lines)
	}

	if matches := findMatchingLines("one\ntwo", ""); matches != nil {
		t.Fatalf("expected nil for empty query, got %v", matches)
	}
}

func TestFormatMCPServerList(t *testing.T) {
	now := time.Date(2024, 12, 25, 15, 4, 5, 0, time.UTC)
	list := formatMCPServerList([]*mcp.ServerInstance{
		{
			Name:         "alpha",
			Status:       mcp.StatusRunning,
			StartedAt:    now,
			RestartCount: 2,
		},
		{
			Name:      "beta",
			Status:    mcp.StatusError,
			LastError: errors.New("boom"),
		},
	})

	if !strings.Contains(list, "alpha") || !strings.Contains(list, "beta") {
		t.Fatalf("expected both servers in output: %q", list)
	}
	if !strings.Contains(list, "restarts: 2") {
		t.Fatalf("expected restart count in output: %q", list)
	}
	if !strings.Contains(strings.ToLower(list), "last error") {
		t.Fatalf("expected error details in output: %q", list)
	}
}

func TestMCPCommandListAndRestart(t *testing.T) {
	registry := &stubMCPRegistry{
		servers: []*mcp.ServerInstance{{
			Name:      "gamma",
			Status:    mcp.StatusRunning,
			StartedAt: time.Now(),
		}},
	}

	ui := &ChatUI{
		store:    state.NewStore(),
		hub:      eventhub.NewHub(),
		registry: registry,
		mcpKnown: make(map[string]state.MCPStatus),
	}
	ui.searchSummary.Store("")

	ui.mcpCommand(nil)
	snapshot := ui.store.Snapshot()
	if len(snapshot.Messages) == 0 || !strings.Contains(snapshot.Messages[len(snapshot.Messages)-1].Content, "gamma") {
		t.Fatalf("expected MCP list message in snapshot: %#v", snapshot.Messages)
	}

	ui.mcpCommand([]string{"restart", "gamma"})
	if len(registry.restarted) != 1 || registry.restarted[0] != "gamma" {
		t.Fatalf("expected registry restart call, got %v", registry.restarted)
	}

	snapshot = ui.store.Snapshot()
	if len(snapshot.Messages) == 0 || !strings.Contains(snapshot.Messages[len(snapshot.Messages)-1].Content, "Restarted MCP server gamma") {
		t.Fatalf("expected restart confirmation message, got %#v", snapshot.Messages)
	}
}

func TestExportTranscriptCommandWritesFile(t *testing.T) {
	dir := t.TempDir()

	ui := &ChatUI{
		store: state.NewStore(),
		hub:   eventhub.NewHub(),
	}
	ui.searchSummary.Store("")
	ui.sessionID = "session-abc"

	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	ui.store.Apply(state.MessageAppend{Message: state.ChatMessage{
		Role:      state.RoleUser,
		AgentID:   "user",
		Content:   "Export me",
		CreatedAt: now,
	}})

	target := filepath.Join(dir, "transcript.md")
	ui.exportTranscriptCommand(target)

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("expected transcript file to be written: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "Export me") {
		t.Fatalf("expected export to include message content, got %q", text)
	}
	if !strings.Contains(text, "session-abc") {
		t.Fatalf("expected session identifier in export, got %q", text)
	}

	snapshot := ui.store.Snapshot()
	if len(snapshot.Messages) == 0 || !strings.Contains(snapshot.Messages[len(snapshot.Messages)-1].Content, "Exported transcript to") {
		t.Fatalf("expected confirmation message, got %#v", snapshot.Messages)
	}
}

func TestExportTranscriptCommandDefaultPath(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to read working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	ui := &ChatUI{
		store: state.NewStore(),
		hub:   eventhub.NewHub(),
	}
	ui.searchSummary.Store("")
	ui.sessionID = "session:foo/bar"
	ui.store.Apply(state.MessageAppend{Message: state.ChatMessage{
		Role:    state.RoleAssistant,
		AgentID: "core",
		Content: "Ready",
	}})

	ui.exportTranscriptCommand("")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read export directory: %v", err)
	}

	var match string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "session_foo_bar-transcript-") && strings.HasSuffix(name, ".md") {
			match = filepath.Join(dir, name)
			break
		}
	}

	if match == "" {
		t.Fatalf("expected sanitized transcript file, entries: %v", entries)
	}

	data, err := os.ReadFile(match)
	if err != nil {
		t.Fatalf("failed to read exported file: %v", err)
	}
	if !strings.Contains(string(data), "Ready") {
		t.Fatalf("expected transcript contents in export, got %q", string(data))
	}
}

func TestExportTranscriptCommandNoMessages(t *testing.T) {
	dir := t.TempDir()
	ui := &ChatUI{
		store: state.NewStore(),
		hub:   eventhub.NewHub(),
	}
	ui.searchSummary.Store("")

	ui.exportTranscriptCommand(filepath.Join(dir, "ignored.md"))

	snapshot := ui.store.Snapshot()
	if len(snapshot.Messages) == 0 {
		t.Fatalf("expected notice message when nothing to export")
	}
	if got := snapshot.Messages[len(snapshot.Messages)-1].Content; !strings.Contains(got, "No messages to export yet") {
		t.Fatalf("expected no-messages warning, got %q", got)
	}
}

func TestVerboseCommandTogglesState(t *testing.T) {
	ui := &ChatUI{
		store:     state.NewStore(),
		hub:       eventhub.NewHub(),
		outputCtx: &types.OutputContext{},
	}
	ui.searchSummary.Store("")

	ui.verboseCommand([]string{"on"})
	if !ui.verbose {
		t.Fatalf("expected verbose mode to be enabled")
	}
	if ui.outputCtx == nil || !ui.outputCtx.Verbose {
		t.Fatalf("expected output context verbose flag to be true")
	}
	snapshot := ui.store.Snapshot()
	if got := snapshot.Messages[len(snapshot.Messages)-1].Content; !strings.Contains(got, "Verbose mode enabled") {
		t.Fatalf("expected confirmation message, got %q", got)
	}

	ui.verboseCommand([]string{"toggle"})
	if ui.verbose {
		t.Fatalf("expected verbose mode to be disabled after toggle")
	}
	if ui.outputCtx == nil || ui.outputCtx.Verbose {
		t.Fatalf("expected output context verbose flag to be false")
	}
	snapshot = ui.store.Snapshot()
	if got := snapshot.Messages[len(snapshot.Messages)-1].Content; !strings.Contains(got, "Verbose mode disabled") {
		t.Fatalf("expected disabled message, got %q", got)
	}

	ui.verboseCommand(nil)
	snapshot = ui.store.Snapshot()
	if got := snapshot.Messages[len(snapshot.Messages)-1].Content; !strings.Contains(got, "Verbose mode is disabled") {
		t.Fatalf("expected status message without args, got %q", got)
	}
}

func TestFollowCommandShowsStatus(t *testing.T) {
	ui := &ChatUI{
		store:                   state.NewStore(),
		hub:                     eventhub.NewHub(),
		defaultFollowTranscript: true,
		defaultFollowStream:     false,
	}
	ui.restoreFollowDefaults()
	ui.searchSummary.Store("")

	ui.followCommand(nil)

	snapshot := ui.store.Snapshot()
	if len(snapshot.Messages) == 0 {
		t.Fatalf("expected status message")
	}
	msg := snapshot.Messages[len(snapshot.Messages)-1].Content
	if !strings.Contains(msg, "transcript=on") || !strings.Contains(msg, "stream=off") {
		t.Fatalf("expected follow state summary, got %q", msg)
	}
}

func TestFollowCommandUpdatesAndPersists(t *testing.T) {
	ui := &ChatUI{
		store:                   state.NewStore(),
		hub:                     eventhub.NewHub(),
		defaultFollowTranscript: true,
		defaultFollowStream:     false,
	}
	ui.restoreFollowDefaults()
	ui.searchSummary.Store("")

	var saved struct {
		transcript bool
		stream     bool
	}
	ui.saveFollowPreferences = func(transcript, stream bool) (string, error) {
		saved.transcript = transcript
		saved.stream = stream
		return "/tmp/config.json", nil
	}

	ui.followCommand([]string{"stream", "on"})

	if !ui.defaultFollowStream || !ui.followStream.Load() {
		t.Fatalf("expected follow stream default to be enabled")
	}
	if saved.transcript != true || saved.stream != true {
		t.Fatalf("expected persisted values true/true, got %v/%v", saved.transcript, saved.stream)
	}

	snapshot := ui.store.Snapshot()
	if len(snapshot.Messages) == 0 {
		t.Fatalf("expected confirmation message")
	}
	msg := snapshot.Messages[len(snapshot.Messages)-1].Content
	if !strings.Contains(msg, "Saved to /tmp/config.json") {
		t.Fatalf("expected saved path in message, got %q", msg)
	}
}

func TestFollowCommandPersistError(t *testing.T) {
	ui := &ChatUI{
		store:                   state.NewStore(),
		hub:                     eventhub.NewHub(),
		defaultFollowTranscript: true,
		defaultFollowStream:     true,
	}
	ui.restoreFollowDefaults()
	ui.searchSummary.Store("")

	ui.saveFollowPreferences = func(transcript, stream bool) (string, error) {
		return "", errors.New("boom")
	}

	ui.followCommand([]string{"transcript", "off"})

	if ui.defaultFollowTranscript {
		t.Fatalf("expected transcript follow default to change despite error")
	}

	snapshot := ui.store.Snapshot()
	if len(snapshot.Messages) == 0 {
		t.Fatalf("expected error message")
	}
	msg := snapshot.Messages[len(snapshot.Messages)-1].Content
	if !strings.Contains(msg, "failed to persist") {
		t.Fatalf("expected persistence error message, got %q", msg)
	}
}
