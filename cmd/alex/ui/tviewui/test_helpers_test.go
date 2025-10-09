package tviewui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/mcp"
)

type stubMCPRegistry struct {
	servers   []*mcp.ServerInstance
	restarted []string
	err       error
}

func (s *stubMCPRegistry) ListServers() []*mcp.ServerInstance {
	return s.servers
}

func (s *stubMCPRegistry) RestartServer(name string) error {
	s.restarted = append(s.restarted, name)
	return s.err
}

type stubCostTracker struct {
	mu      sync.Mutex
	summary *ports.CostSummary
	err     error
	calls   int
	session string
}

func (s *stubCostTracker) RecordUsage(context.Context, ports.UsageRecord) error { return nil }

func (s *stubCostTracker) GetSessionCost(_ context.Context, sessionID string) (*ports.CostSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.session = sessionID
	return s.summary, s.err
}

func (s *stubCostTracker) GetDailyCost(context.Context, time.Time) (*ports.CostSummary, error) {
	return nil, nil
}

func (s *stubCostTracker) GetMonthlyCost(context.Context, int, int) (*ports.CostSummary, error) {
	return nil, nil
}

func (s *stubCostTracker) GetDateRangeCost(context.Context, time.Time, time.Time) (*ports.CostSummary, error) {
	return nil, nil
}

func (s *stubCostTracker) Export(context.Context, ports.ExportFormat, ports.ExportFilter) ([]byte, error) {
	return nil, nil
}

func (s *stubCostTracker) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *stubCostTracker) lastSession() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.session
}

func boolPtr(value bool) *bool {
	return &value
}

type stubCoordinator struct {
	mu       sync.Mutex
	sessions map[string]*ports.Session
	nextID   int
	cost     ports.CostTracker
	executed []string
}

func newStubCoordinator(cost ports.CostTracker) *stubCoordinator {
	if cost == nil {
		cost = &stubCostTracker{}
	}
	return &stubCoordinator{
		sessions: make(map[string]*ports.Session),
		cost:     cost,
	}
}

func (s *stubCoordinator) ExecuteTask(ctx context.Context, task, sessionID string, listener ports.EventListener) (*ports.TaskResult, error) {
	s.mu.Lock()
	if sessionID == "" {
		sessionID = s.ensureSessionLocked()
	}
	s.executed = append(s.executed, task)
	s.mu.Unlock()

	if listener != nil {
		now := time.Now()
		analysis := domain.NewTaskAnalysisEvent(ports.LevelCore, sessionID, fmt.Sprintf("Plan %s", task), "Draft plan", now)
		listener.OnEvent(analysis)

		baseStart := domain.NewTaskAnalysisEvent(ports.LevelCore, sessionID, "", "", now.Add(10*time.Millisecond)).BaseEvent
		listener.OnEvent(&domain.ToolCallStartEvent{
			BaseEvent: baseStart,
			Iteration: 1,
			CallID:    "tool-1",
			ToolName:  "shell",
			Arguments: map[string]interface{}{"cmd": "echo"},
		})

		baseStream := domain.NewTaskAnalysisEvent(ports.LevelCore, sessionID, "", "", now.Add(20*time.Millisecond)).BaseEvent
		listener.OnEvent(&domain.ToolCallStreamEvent{
			BaseEvent: baseStream,
			CallID:    "tool-1",
			Chunk:     "stream chunk",
		})

		baseComplete := domain.NewTaskAnalysisEvent(ports.LevelCore, sessionID, "", "", now.Add(30*time.Millisecond)).BaseEvent
		listener.OnEvent(&domain.ToolCallCompleteEvent{
			BaseEvent: baseComplete,
			CallID:    "tool-1",
			ToolName:  "shell",
			Result:    "done",
			Duration:  50 * time.Millisecond,
		})

		baseThink := domain.NewTaskAnalysisEvent(ports.LevelCore, sessionID, "", "", now.Add(40*time.Millisecond)).BaseEvent
		listener.OnEvent(&domain.ThinkCompleteEvent{
			BaseEvent: baseThink,
			Iteration: 1,
			Content:   fmt.Sprintf("Answer for %s", task),
		})

		baseDone := domain.NewTaskAnalysisEvent(ports.LevelCore, sessionID, "", "", now.Add(50*time.Millisecond)).BaseEvent
		listener.OnEvent(&domain.TaskCompleteEvent{
			BaseEvent:       baseDone,
			FinalAnswer:     fmt.Sprintf("Final answer for %s", task),
			TotalIterations: 1,
			TotalTokens:     42,
			StopReason:      "complete",
			Duration:        50 * time.Millisecond,
		})
	}

	return &ports.TaskResult{
		Answer:     fmt.Sprintf("Final answer for %s", task),
		Messages:   []ports.Message{{Role: "assistant", Content: fmt.Sprintf("Final answer for %s", task)}},
		Iterations: 1,
		TokensUsed: 42,
		StopReason: "complete",
		SessionID:  sessionID,
	}, nil
}

func (s *stubCoordinator) ListSessions(context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *stubCoordinator) GetSession(ctx context.Context, id string) (*ports.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		return s.createSessionLocked(), nil
	}
	if session, ok := s.sessions[id]; ok {
		return session, nil
	}
	session := &ports.Session{
		ID:        id,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.sessions[id] = session
	return session, nil
}

func (s *stubCoordinator) GetCostTracker() ports.CostTracker {
	return s.cost
}

func (s *stubCoordinator) ensureSessionLocked() string {
	session := s.createSessionLocked()
	return session.ID
}

func (s *stubCoordinator) createSessionLocked() *ports.Session {
	s.nextID++
	id := fmt.Sprintf("session-%03d", s.nextID)
	session := &ports.Session{
		ID:        id,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.sessions[id] = session
	return session
}
