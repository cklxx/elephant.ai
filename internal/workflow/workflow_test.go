package workflow

import (
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNodeLifecycle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(testWriter{t: t}, &slog.HandlerOptions{Level: slog.LevelDebug}))
	node := NewNode("tts", map[string]string{"alias": "intro"}, logger)

	snapshot := node.Snapshot()
	require.Equal(t, NodeStatusPending, snapshot.Status)
	require.Zero(t, snapshot.StartedAt)

	running, err := node.Start()
	require.NoError(t, err)
	require.Equal(t, NodeStatusRunning, running.Status)
	require.NotZero(t, running.StartedAt)
	require.Zero(t, running.CompletedAt)

	_, err = node.Start()
	require.Error(t, err)

	succeeded, err := node.CompleteSuccess("tts/output.mp3")
	require.NoError(t, err)
	require.Equal(t, NodeStatusSucceeded, succeeded.Status)
	require.Equal(t, "tts/output.mp3", succeeded.Output)
	require.NotZero(t, succeeded.CompletedAt)
	require.Greater(t, succeeded.Duration, time.Duration(0))

	_, err = node.CompleteFailure(errors.New("should fail"))
	require.Error(t, err)
}

func TestWorkflowSnapshot(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(testWriter{t: t}, &slog.HandlerOptions{Level: slog.LevelDebug}))

	wf := New("media-pipeline", logger)
	tts := NewNode("tts", "request-1", logger)
	video := NewNode("video", "clip-1", logger)
	staging := NewNode("staging", "job-1", logger)

	require.NoError(t, wf.AddNode(tts))
	require.NoError(t, wf.AddNode(video))
	require.NoError(t, wf.AddNode(staging))

	snapshot := wf.Snapshot()
	require.Equal(t, PhasePending, snapshot.Phase)
	require.Len(t, snapshot.Nodes, 3)
	require.Equal(t, int64(3), snapshot.Summary[string(NodeStatusPending)])

	_, err := tts.Start()
	require.NoError(t, err)
	snapshot = wf.Snapshot()
	require.Equal(t, PhaseRunning, snapshot.Phase)
	require.False(t, snapshot.StartedAt.IsZero())

	_, err = tts.CompleteSuccess("tts/output.mp3")
	require.NoError(t, err)
	_, err = video.Start()
	require.NoError(t, err)

	snapshot = wf.Snapshot()
	require.Equal(t, PhaseRunning, snapshot.Phase)
	require.Equal(t, int64(1), snapshot.Summary[string(NodeStatusSucceeded)])
	require.Equal(t, int64(1), snapshot.Summary[string(NodeStatusRunning)])
	require.Equal(t, int64(1), snapshot.Summary[string(NodeStatusPending)])

	_, err = video.CompleteFailure(errors.New("encode failed"))
	require.NoError(t, err)

	snapshot = wf.Snapshot()
	require.Equal(t, PhaseFailed, snapshot.Phase)
	require.Equal(t, int64(1), snapshot.Summary[string(NodeStatusSucceeded)])
	require.Equal(t, int64(1), snapshot.Summary[string(NodeStatusFailed)])
	require.False(t, snapshot.CompletedAt.IsZero())
	require.Greater(t, snapshot.Duration, time.Duration(0))
}

func TestWorkflowSnapshotPreservesOrder(t *testing.T) {
	wf := New("media-pipeline", nil)

	require.NoError(t, wf.AddNode(NewNode("upload", nil, nil)))
	require.NoError(t, wf.AddNode(NewNode("analyze", nil, nil)))
	require.NoError(t, wf.AddNode(NewNode("summarize", nil, nil)))

	snapshot := wf.Snapshot()
	require.Equal(t, []string{"upload", "analyze", "summarize"}, snapshot.Order)
	require.Equal(t, []NodeSnapshot{
		wf.nodes["upload"].Snapshot(),
		wf.nodes["analyze"].Snapshot(),
		wf.nodes["summarize"].Snapshot(),
	}, snapshot.Nodes)
}

func TestWorkflowEmitsEvents(t *testing.T) {
	wf := New("media-pipeline", nil)
	listener := &capturingListener{}
	wf.AddListener(listener)

	node := NewNode("prepare", map[string]string{"stage": "prepare"}, nil)
	require.NoError(t, wf.AddNode(node))

	_, _, err := wf.StartNode("prepare")
	require.NoError(t, err)

	_, _, err = wf.CompleteNodeSuccess("prepare", "ok")
	require.NoError(t, err)

	events := listener.Events()
	require.Len(t, events, 5)

	require.Equal(t, EventNodeAdded, events[0].Type)
	require.Equal(t, PhasePending, events[0].Phase)
	require.NotNil(t, events[0].Snapshot)
	require.Equal(t, []string{"prepare"}, events[0].Snapshot.Order)
	require.Equal(t, EventNodeStarted, events[1].Type)
	require.Equal(t, PhaseRunning, events[1].Phase)
	require.Equal(t, EventWorkflowUpdated, events[2].Type)
	require.Equal(t, PhaseRunning, events[2].Phase)
	require.Equal(t, EventNodeSucceeded, events[3].Type)
	require.Equal(t, PhaseSucceeded, events[3].Phase)
	require.Equal(t, EventWorkflowUpdated, events[4].Type)
	require.Equal(t, PhaseSucceeded, events[4].Phase)

	require.Equal(t, "media-pipeline", events[0].Workflow)
	require.NotNil(t, events[1].Node)
	require.Equal(t, NodeStatusRunning, events[1].Node.Status)
	require.NotNil(t, events[3].Snapshot)
	require.Equal(t, PhaseSucceeded, events[3].Snapshot.Phase)
}

func TestEvaluatePhaseAggregatesTimestamps(t *testing.T) {
	start := time.Unix(100, 0)
	nodes := []NodeSnapshot{
		{ID: "first", Status: NodeStatusSucceeded, StartedAt: start, CompletedAt: start.Add(2 * time.Second)},
		{ID: "second", Status: NodeStatusFailed, StartedAt: start.Add(3 * time.Second), CompletedAt: start.Add(5 * time.Second)},
		{ID: "pending", Status: NodeStatusPending},
	}

	phase, startedAt, completedAt := evaluatePhase(nodes)
	require.Equal(t, PhaseFailed, phase)
	require.Equal(t, start, startedAt)
	require.Equal(t, start.Add(5*time.Second), completedAt)
}

func TestEvaluatePhaseTreatsProgressAsRunning(t *testing.T) {
	start := time.Unix(200, 0)
	nodes := []NodeSnapshot{
		{ID: "completed", Status: NodeStatusSucceeded, StartedAt: start, CompletedAt: start.Add(time.Second)},
		{ID: "pending", Status: NodeStatusPending},
	}

	phase, startedAt, completedAt := evaluatePhase(nodes)
	require.Equal(t, PhaseRunning, phase)
	require.Equal(t, start, startedAt)
	require.True(t, completedAt.IsZero())
}

type capturingListener struct {
	mu     sync.Mutex
	events []Event
}

func (l *capturingListener) OnWorkflowEvent(evt Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, evt)
}

func (l *capturingListener) Events() []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	copied := make([]Event, len(l.events))
	copy(copied, l.events)
	return copied
}

type testWriter struct {
	t *testing.T
}

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(string(p))
	return len(p), nil
}
