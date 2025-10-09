package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStoreApplyMessageAppend(t *testing.T) {
	store := NewStore()
	now := time.Now()

	store.Apply(MessageAppend{Message: ChatMessage{Content: "   "}})
	store.Apply(MessageAppend{Message: ChatMessage{
		Role:      RoleSystem,
		AgentID:   "core",
		Content:   "analysis ready",
		CreatedAt: now,
	}})

	snapshot := store.Snapshot()
	require.Len(t, snapshot.Messages, 1)
	require.Equal(t, "analysis ready", snapshot.Messages[0].Content)
	require.Equal(t, RoleSystem, snapshot.Messages[0].Role)
}

func TestStoreApplyToolRunLifecycle(t *testing.T) {
	store := NewStore()
	start := time.Now()
	running := ToolStatusRunning

	store.Apply(ToolRunDelta{
		CallID:    "call-1",
		ToolName:  "search",
		AgentID:   "core",
		StartedAt: &start,
		Status:    &running,
		Timestamp: start,
	})

	snapshot := store.Snapshot()
	require.Len(t, snapshot.ToolRuns, 1)
	run := snapshot.ToolRuns[0]
	require.Equal(t, "call-1", run.ID)
	require.Equal(t, ToolStatusRunning, run.Status)
	require.NotNil(t, run.StartedAt)
	require.Nil(t, run.CompletedAt)

	duration := 2 * time.Second
	completeAt := start.Add(duration)
	completed := ToolStatusCompleted
	result := "done"

	store.Apply(ToolRunDelta{
		CallID:      "call-1",
		CompletedAt: &completeAt,
		Duration:    &duration,
		Status:      &completed,
		Result:      &result,
		Timestamp:   completeAt,
	})

	snapshot = store.Snapshot()
	require.Len(t, snapshot.ToolRuns, 1)
	run = snapshot.ToolRuns[0]
	require.Equal(t, ToolStatusCompleted, run.Status)
	require.NotNil(t, run.CompletedAt)
	require.Equal(t, duration, run.Duration)
	require.Equal(t, "done", run.Result)
}

func TestStoreApplySubtaskDelta(t *testing.T) {
	store := NewStore()
	now := time.Now()
	running := SubtaskStatusRunning
	currentTool := "search"
	agentLevel := "subagent"

	store.Apply(SubtaskDelta{
		Index:       0,
		Total:       2,
		Preview:     "collect facts",
		Status:      &running,
		CurrentTool: &currentTool,
		AgentLevel:  agentLevel,
		Timestamp:   now,
	})

	next := now.Add(time.Second)
	cleared := ""
	store.Apply(SubtaskDelta{
		Index:               0,
		CurrentTool:         &cleared,
		ToolsCompletedDelta: 1,
		Timestamp:           next,
	})

	tokens := 128
	completed := SubtaskStatusCompleted
	finished := next.Add(2 * time.Second)
	store.Apply(SubtaskDelta{
		Index:       0,
		Status:      &completed,
		TokensUsed:  &tokens,
		CurrentTool: &cleared,
		CompletedAt: &finished,
		Timestamp:   finished,
	})

	snapshot := store.Snapshot()
	require.Len(t, snapshot.SubagentRuns, 1)
	task := snapshot.SubagentRuns[0]
	require.Equal(t, SubtaskStatusCompleted, task.Status)
	require.Equal(t, 1, task.ToolsCompleted)
	require.Equal(t, tokens, task.TokensUsed)
	require.Equal(t, agentLevel, task.AgentLevel)
	require.NotNil(t, task.StartedAt)
	require.WithinDuration(t, now, *task.StartedAt, time.Millisecond)
	require.NotNil(t, task.CompletedAt)
	require.WithinDuration(t, finished, *task.CompletedAt, time.Millisecond)
	require.Equal(t, finished.Sub(*task.StartedAt), task.Duration)
}

func TestStoreApplyMCPDelta(t *testing.T) {
	store := NewStore()
	now := time.Now()
	ready := MCPStatusReady
	started := now.Add(-time.Minute)
	errText := ""

	store.Apply(MCPServerDelta{
		Name:      "search",
		Status:    &ready,
		StartedAt: &started,
		LastError: &errText,
		Timestamp: now,
	})

	snapshot := store.Snapshot()
	require.Len(t, snapshot.MCPServers, 1)
	server := snapshot.MCPServers[0]
	require.Equal(t, "search", server.Name)
	require.Equal(t, MCPStatusReady, server.Status)
	require.NotNil(t, server.StartedAt)
	require.Equal(t, now, server.UpdatedAt)
}

func TestStoreApplyMetricsDelta(t *testing.T) {
	store := NewStore()
	now := time.Now()

	store.Apply(MetricsDelta{AgentLevel: "core", Tokens: 120, Timestamp: now})
	store.Apply(MetricsDelta{AgentLevel: "subagent", Tokens: 30})
	store.Apply(MetricsDelta{AgentLevel: "core", Tokens: 150})

	snapshot := store.Snapshot()
	require.Equal(t, 300, snapshot.Metrics.TotalTokens)
	require.Equal(t, now, snapshot.Metrics.UpdatedAt)
	require.Equal(t, 270, snapshot.Metrics.TokensByAgent["core"])
	require.Equal(t, 30, snapshot.Metrics.TokensByAgent["subagent"])
	require.Zero(t, snapshot.Metrics.TotalCost)
	require.Empty(t, snapshot.Metrics.CostByModel)

	later := now.Add(time.Minute)
	store.Apply(MetricsDelta{AgentLevel: "core", Tokens: 0, Timestamp: later})

	snapshot = store.Snapshot()
	require.Equal(t, 300, snapshot.Metrics.TotalTokens)
	require.Equal(t, later, snapshot.Metrics.UpdatedAt)
}

func TestStoreApplyMetricsCostSummary(t *testing.T) {
	store := NewStore()
	now := time.Now()

	store.Apply(MetricsCostSummary{
		TotalCost: 1.25,
		CostByModel: map[string]float64{
			" gpt-4 ": 0.75,
			"gpt-4o":  0.5,
			"zero":    0,
		},
		Timestamp: now,
	})

	snapshot := store.Snapshot()
	require.InDelta(t, 1.25, snapshot.Metrics.TotalCost, 1e-9)
	require.Equal(t, now, snapshot.Metrics.UpdatedAt)
	require.Len(t, snapshot.Metrics.CostByModel, 2)
	require.InDelta(t, 0.75, snapshot.Metrics.CostByModel["gpt-4"], 1e-9)
	require.InDelta(t, 0.5, snapshot.Metrics.CostByModel["gpt-4o"], 1e-9)

	// Applying without explicit total should sum individual entries.
	store.Apply(MetricsCostSummary{
		CostByModel: map[string]float64{"gpt-4": 0.25},
	})

	snapshot = store.Snapshot()
	require.InDelta(t, 0.25, snapshot.Metrics.TotalCost, 1e-9)
	require.Len(t, snapshot.Metrics.CostByModel, 1)
}

func TestStoreReset(t *testing.T) {
	store := NewStore()
	now := time.Now()
	running := ToolStatusRunning

	store.Apply(MessageAppend{Message: ChatMessage{Role: RoleUser, Content: "hello", CreatedAt: now}})
	store.Apply(ToolRunDelta{CallID: "call-1", ToolName: "search", Status: &running})
	store.Apply(SubtaskDelta{Index: 1, Total: 2, Preview: "plan"})
	ready := MCPStatusReady
	store.Apply(MCPServerDelta{Name: "code", Status: &ready})
	store.Apply(MetricsDelta{AgentLevel: "core", Tokens: 42})
	store.Apply(MetricsCostSummary{TotalCost: 0.75, CostByModel: map[string]float64{"gpt-4": 0.75}})

	store.Apply(Reset{})

	snapshot := store.Snapshot()
	require.Empty(t, snapshot.Messages)
	require.Empty(t, snapshot.ToolRuns)
	require.Empty(t, snapshot.SubagentRuns)
	require.Empty(t, snapshot.MCPServers)
	require.Zero(t, snapshot.Metrics.TotalTokens)
	require.Empty(t, snapshot.Metrics.TokensByAgent)
	require.Zero(t, snapshot.Metrics.TotalCost)
	require.Empty(t, snapshot.Metrics.CostByModel)
}
