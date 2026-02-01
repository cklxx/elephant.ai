package larktesting

import (
	"context"
	"fmt"
	"sync"
	"time"

	ports "alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	storage "alex/internal/agent/ports/storage"
	"alex/internal/channels"
	larkgw "alex/internal/channels/lark"
	"alex/internal/logging"
)

// TurnResult captures the outcome of a single scenario turn.
type TurnResult struct {
	TurnIndex int
	Duration  time.Duration
	Calls     []larkgw.MessengerCall
	Task      string
	Called    bool
	Errors    []string
}

// ScenarioResult captures the outcome of an entire scenario.
type ScenarioResult struct {
	Name     string
	Passed   bool
	Turns    []TurnResult
	Duration time.Duration
}

// Runner executes scenarios against a Lark Gateway with a RecordingMessenger.
type Runner struct {
	logger logging.Logger
}

// NewRunner creates a scenario runner.
func NewRunner(logger logging.Logger) *Runner {
	return &Runner{logger: logging.OrNop(logger)}
}

// RunAll loads all scenarios from a directory and returns a full TestReport.
func (r *Runner) RunAll(ctx context.Context, scenarioDir string) (*TestReport, error) {
	scenarios, err := LoadScenariosFromDir(scenarioDir)
	if err != nil {
		return nil, err
	}

	var results []*ScenarioResult
	for _, s := range scenarios {
		results = append(results, r.Run(ctx, s))
	}

	return BuildReport(results), nil
}

// Run executes a single scenario and returns the result. A single Gateway is
// created for the entire scenario so that dedup cache and session slots persist
// across turns. The mock executor cycles through per-turn responses.
func (r *Runner) Run(ctx context.Context, scenario *Scenario) *ScenarioResult {
	start := time.Now()
	result := &ScenarioResult{Name: scenario.Name}

	rec := larkgw.NewRecordingMessenger()
	executor := newMultiMockExecutor(scenario.Turns)

	cfg := larkgw.Config{
		BaseConfig: channels.BaseConfig{
			SessionPrefix: scenario.Setup.Config.SessionPrefix,
			AllowDirect:   scenario.Setup.Config.AllowDirect,
			AllowGroups:   scenario.Setup.Config.AllowGroups,
			MemoryEnabled: scenario.Setup.Config.MemoryEnabled,
		},
		AppID:             "test_scenario",
		AppSecret:         "secret_scenario",
		ShowToolProgress:  scenario.Setup.Config.ShowToolProgress,
		ReactEmoji:        scenario.Setup.Config.ReactEmoji,
		PlanReviewEnabled: scenario.Setup.Config.PlanReviewEnabled,
		AutoChatContext:   scenario.Setup.Config.AutoChatContext,
	}
	if cfg.SessionPrefix == "" {
		cfg.SessionPrefix = "test-lark"
	}

	gw, err := larkgw.NewGateway(cfg, executor, r.logger)
	if err != nil {
		result.Turns = append(result.Turns, TurnResult{
			Errors: []string{fmt.Sprintf("failed to create gateway: %v", err)},
		})
		return result
	}
	gw.SetMessenger(rec)

	if scenario.Setup.Config.PlanReviewEnabled {
		gw.SetPlanReviewStore(&stubPlanReviewStore{})
	}

	for i, turn := range scenario.Turns {
		// Advance the executor to this turn's mock response.
		executor.setTurn(i)

		if turn.DelayMS > 0 {
			time.Sleep(time.Duration(turn.DelayMS) * time.Millisecond)
		}

		rec.Reset()
		executor.resetCapture()
		turnStart := time.Now()

		chatType := turn.ChatType
		if chatType == "" {
			chatType = "p2p"
		}

		err := gw.InjectMessage(ctx, turn.ChatID, chatType, turn.SenderID, turn.MessageID, turn.Content)

		// Wait briefly for async reactions (goroutines).
		time.Sleep(50 * time.Millisecond)

		turnDuration := time.Since(turnStart)

		tr := TurnResult{
			TurnIndex: i,
			Duration:  turnDuration,
			Calls:     rec.Calls(),
			Task:      executor.getCapturedTask(),
			Called:    executor.wasCalled(),
		}

		if err != nil {
			tr.Errors = append(tr.Errors, fmt.Sprintf("InjectMessage error: %v", err))
		}

		assertionErrors := evaluateAssertions(turn.Assertions, tr)
		tr.Errors = append(tr.Errors, assertionErrors...)

		result.Turns = append(result.Turns, tr)
	}

	result.Duration = time.Since(start)
	result.Passed = true
	for _, tr := range result.Turns {
		if len(tr.Errors) > 0 {
			result.Passed = false
			break
		}
	}

	return result
}

// multiMockExecutor implements AgentExecutor with per-turn mock responses.
type multiMockExecutor struct {
	mu           sync.Mutex
	turns        []Turn
	currentTurn  int
	capturedTask string
	called       bool
}

func newMultiMockExecutor(turns []Turn) *multiMockExecutor {
	return &multiMockExecutor{turns: turns}
}

func (m *multiMockExecutor) setTurn(i int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentTurn = i
}

func (m *multiMockExecutor) resetCapture() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.capturedTask = ""
	m.called = false
}

func (m *multiMockExecutor) getCapturedTask() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.capturedTask
}

func (m *multiMockExecutor) wasCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.called
}

func (m *multiMockExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	if sessionID == "" {
		sessionID = "test-session"
	}
	return &storage.Session{ID: sessionID, Metadata: map[string]string{}}, nil
}

func (m *multiMockExecutor) ExecuteTask(_ context.Context, task string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.called = true
	m.capturedTask = task

	var mockResp *MockResponse
	if m.currentTurn < len(m.turns) {
		mockResp = m.turns[m.currentTurn].MockResponse
	}

	if mockResp == nil {
		return &agent.TaskResult{Answer: "(no mock response configured)"}, nil
	}

	if mockResp.Error != "" {
		return nil, fmt.Errorf("%s", mockResp.Error)
	}

	result := &agent.TaskResult{
		Answer:     mockResp.Answer,
		StopReason: mockResp.StopReason,
	}

	for _, sys := range mockResp.SystemMessages {
		result.Messages = append(result.Messages, ports.Message{
			Role:    "system",
			Content: sys,
		})
	}

	return result, nil
}

func (m *multiMockExecutor) ResetSession(_ context.Context, _ string) error {
	return nil
}

// stubPlanReviewStore is a minimal in-memory plan review store.
type stubPlanReviewStore struct {
	mu      sync.Mutex
	pending map[string]larkgw.PlanReviewPending
	saved   []larkgw.PlanReviewPending
}

func (s *stubPlanReviewStore) EnsureSchema(_ context.Context) error { return nil }

func (s *stubPlanReviewStore) SavePending(_ context.Context, p larkgw.PlanReviewPending) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saved = append(s.saved, p)
	if s.pending == nil {
		s.pending = make(map[string]larkgw.PlanReviewPending)
	}
	s.pending[p.UserID+":"+p.ChatID] = p
	return nil
}

func (s *stubPlanReviewStore) GetPending(_ context.Context, userID, chatID string) (larkgw.PlanReviewPending, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == nil {
		return larkgw.PlanReviewPending{}, false, nil
	}
	p, ok := s.pending[userID+":"+chatID]
	return p, ok, nil
}

func (s *stubPlanReviewStore) ClearPending(_ context.Context, userID, chatID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending != nil {
		delete(s.pending, userID+":"+chatID)
	}
	return nil
}
