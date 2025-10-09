package eventhub

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"alex/cmd/alex/ui/state"
	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/tools/builtin"
)

func TestHubPublishesToolRunUpdates(t *testing.T) {
	hub := NewHub()
	ch := hub.Subscribe(4)
	defer hub.Unsubscribe(ch)

	ts := time.Now()
	base := domain.NewTaskAnalysisEvent(ports.LevelCore, "sess", "", "", ts).BaseEvent

	hub.PublishAgentEvent(&domain.ToolCallStartEvent{
		BaseEvent: base,
		CallID:    "call-1",
		ToolName:  "search",
		Arguments: map[string]interface{}{"query": "golang"},
	})

	update := <-ch
	delta, ok := update.(state.ToolRunDelta)
	require.True(t, ok)
	require.Equal(t, "call-1", delta.CallID)
	require.NotNil(t, delta.Status)
	require.Equal(t, state.ToolStatusRunning, *delta.Status)
	require.NotNil(t, delta.StartedAt)
	require.True(t, delta.StartedAt.Equal(ts))

	duration := 2 * time.Second
	result := "done"
	hub.PublishAgentEvent(&domain.ToolCallCompleteEvent{
		BaseEvent: base,
		CallID:    "call-1",
		ToolName:  "search",
		Result:    result,
		Duration:  duration,
	})

	update = <-ch
	delta, ok = update.(state.ToolRunDelta)
	require.True(t, ok)
	require.NotNil(t, delta.Status)
	require.Equal(t, state.ToolStatusCompleted, *delta.Status)
	require.NotNil(t, delta.CompletedAt)
	require.True(t, delta.CompletedAt.Equal(ts))
	require.NotNil(t, delta.Duration)
	require.Equal(t, duration, *delta.Duration)
	require.NotNil(t, delta.Result)
	require.Equal(t, result, *delta.Result)

	messageUpdate := <-ch
	message, ok := messageUpdate.(state.MessageAppend)
	require.True(t, ok)
	require.Equal(t, state.RoleTool, message.Message.Role)
	require.Equal(t, result, message.Message.Content)
}

func TestHubPublishesSubtaskUpdatesAlongsideToolRuns(t *testing.T) {
	hub := NewHub()
	ch := hub.Subscribe(8)
	defer hub.Unsubscribe(ch)

	ts := time.Now()
	base := domain.NewTaskAnalysisEvent(ports.LevelSubagent, "sess", "", "", ts).BaseEvent
	inner := &domain.ToolCallStartEvent{
		BaseEvent: base,
		CallID:    "call-42",
		ToolName:  "think",
	}
	subtask := &builtin.SubtaskEvent{
		OriginalEvent:  inner,
		SubtaskIndex:   1,
		TotalSubtasks:  3,
		SubtaskPreview: "plan next steps",
	}

	hub.PublishAgentEvent(subtask)

	first := <-ch
	subDelta, ok := first.(state.SubtaskDelta)
	require.True(t, ok)
	require.Equal(t, 1, subDelta.Index)
	require.NotNil(t, subDelta.Status)
	require.Equal(t, state.SubtaskStatusRunning, *subDelta.Status)
	require.NotNil(t, subDelta.CurrentTool)
	require.Equal(t, "think", *subDelta.CurrentTool)
	require.Equal(t, string(ports.LevelSubagent), subDelta.AgentLevel)
	require.NotNil(t, subDelta.StartedAt)
	require.True(t, subDelta.StartedAt.Equal(ts))

	second := <-ch
	toolDelta, ok := second.(state.ToolRunDelta)
	require.True(t, ok)
	require.Equal(t, "call-42", toolDelta.CallID)

	complete := &domain.TaskCompleteEvent{
		BaseEvent:       base,
		FinalAnswer:     "done",
		TotalTokens:     21,
		TotalIterations: 1,
	}
	hub.PublishAgentEvent(&builtin.SubtaskEvent{
		OriginalEvent:  complete,
		SubtaskIndex:   1,
		TotalSubtasks:  3,
		SubtaskPreview: "plan next steps",
	})

	third := <-ch
	doneDelta, ok := third.(state.SubtaskDelta)
	require.True(t, ok)
	require.NotNil(t, doneDelta.Status)
	require.Equal(t, state.SubtaskStatusCompleted, *doneDelta.Status)
	require.NotNil(t, doneDelta.CompletedAt)
	require.True(t, doneDelta.CompletedAt.Equal(ts))
	require.NotNil(t, doneDelta.TokensUsed)
	require.Equal(t, 21, *doneDelta.TokensUsed)

	fourth := <-ch
	msg, ok := fourth.(state.MessageAppend)
	require.True(t, ok)
	require.Equal(t, state.RoleAssistant, msg.Message.Role)
	require.Equal(t, "done", msg.Message.Content)

	fifth := <-ch
	metrics, ok := fifth.(state.MetricsDelta)
	require.True(t, ok)
	require.Equal(t, string(ports.LevelSubagent), metrics.AgentLevel)
	require.Equal(t, 21, metrics.Tokens)
}

func TestHubPublishesMetricsForTaskComplete(t *testing.T) {
	hub := NewHub()
	ch := hub.Subscribe(4)
	defer hub.Unsubscribe(ch)

	ts := time.Now()
	event := &domain.TaskCompleteEvent{
		BaseEvent:       domain.NewTaskAnalysisEvent(ports.LevelCore, "sess", "", "", ts).BaseEvent,
		FinalAnswer:     "answer",
		TotalTokens:     64,
		TotalIterations: 1,
	}

	hub.PublishAgentEvent(event)

	first := <-ch
	msg, ok := first.(state.MessageAppend)
	require.True(t, ok)
	require.Equal(t, "answer", msg.Message.Content)

	second := <-ch
	metrics, ok := second.(state.MetricsDelta)
	require.True(t, ok)
	require.Equal(t, string(ports.LevelCore), metrics.AgentLevel)
	require.Equal(t, 64, metrics.Tokens)
}

func TestHubBroadcastsMCPDeltas(t *testing.T) {
	hub := NewHub()
	ch := hub.Subscribe(1)
	defer hub.Unsubscribe(ch)

	status := state.MCPStatusReady
	now := time.Now()
	hub.PublishMCPDelta(state.MCPServerDelta{Name: "search", Status: &status, Timestamp: now})

	update := <-ch
	delta, ok := update.(state.MCPServerDelta)
	require.True(t, ok)
	require.Equal(t, "search", delta.Name)
	require.NotNil(t, delta.Status)
	require.Equal(t, state.MCPStatusReady, *delta.Status)
	require.True(t, delta.Timestamp.Equal(now))
}

func TestHubCloseStopsDelivery(t *testing.T) {
	hub := NewHub()
	ch := hub.Subscribe(1)

	hub.Close()

	// Channel should be closed after closing hub.
	_, ok := <-ch
	require.False(t, ok)
}
