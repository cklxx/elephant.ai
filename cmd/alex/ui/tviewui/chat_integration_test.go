package tviewui

import (
	"strings"
	"testing"
	"time"

	"alex/cmd/alex/ui/eventhub"
	"alex/cmd/alex/ui/state"

	"github.com/stretchr/testify/require"
)

func TestChatUIHeadlessTaskFlow(t *testing.T) {
	tracker := &stubCostTracker{}
	coordinator := newStubCoordinator(tracker)

	ui, err := NewChatUI(Config{
		Coordinator: coordinator,
		Store:       state.NewStore(),
		Hub:         eventhub.NewHub(),
		Registry:    &stubMCPRegistry{},
		CostTracker: tracker,
	})
	require.NoError(t, err)

	updates := ui.hub.Subscribe(64)
	defer ui.hub.Unsubscribe(updates)

	session, err := coordinator.GetSession(ui.ctx, "")
	require.NoError(t, err)
	require.NotNil(t, session)

	ui.setSessionID(session.ID)

	done := make(chan struct{})
	go func() {
		listener := eventhub.NewListener(ui.hub)
		_, _ = coordinator.ExecuteTask(ui.ctx, "headless regression", session.ID, listener)
		close(done)
	}()

	timeout := time.After(2 * time.Second)
	assistantSeen := false
	toolCompleted := false
	tokensSeen := false

	for !assistantSeen || !toolCompleted || !tokensSeen {
		select {
		case update := <-updates:
			ui.store.Apply(update)
			switch u := update.(type) {
			case state.MessageAppend:
				if strings.Contains(u.Message.Content, "Final answer for headless regression") {
					assistantSeen = true
				}
			case state.ToolRunDelta:
				if u.Status != nil && *u.Status == state.ToolStatusCompleted {
					toolCompleted = true
				}
			case state.MetricsDelta:
				if u.Tokens == 42 {
					tokensSeen = true
				}
			}
		case <-done:
			// continue draining until conditions met or timeout
		case <-timeout:
			t.Fatalf("timed out waiting for updates")
		}
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("coordinator did not finish in time")
	}

	// Drain any remaining updates.
	drainTimer := time.NewTimer(50 * time.Millisecond)
	for {
		select {
		case update := <-updates:
			ui.store.Apply(update)
		case <-drainTimer.C:
			drainTimer.Stop()
			goto drained
		}
	}

drained:
	snapshot := ui.store.Snapshot()
	require.GreaterOrEqual(t, len(snapshot.Messages), 1)
	require.True(t, strings.Contains(snapshot.Messages[len(snapshot.Messages)-1].Content, "Final answer for headless regression"))
	require.NotEmpty(t, snapshot.ToolRuns)
	require.Equal(t, state.ToolStatusCompleted, snapshot.ToolRuns[0].Status)
	require.Equal(t, 42, snapshot.Metrics.TotalTokens)

	status := renderStatus(0, snapshot, session.ID, "", false)
	require.Contains(t, status, "Tokens: 42")
}
