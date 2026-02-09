package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	appconfig "alex/internal/app/agent/config"
	agentcoordinator "alex/internal/app/agent/coordinator"
	"alex/internal/app/toolregistry"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	llm "alex/internal/domain/agent/ports/llm"
	"alex/internal/domain/agent/ports/mocks"
	agentstorage "alex/internal/domain/agent/ports/storage"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/memory"
	"alex/internal/infra/tools/builtin/shared"
	"alex/internal/shared/config"
	"alex/internal/shared/utils/id"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

type staticLarkOAuth struct {
	token string
}

func (s *staticLarkOAuth) UserAccessToken(_ context.Context, _ string) (string, error) {
	return s.token, nil
}

func (s *staticLarkOAuth) StartURL() string { return "" }

type calendarRequestRecorder struct {
	mu      sync.Mutex
	called  bool
	method  string
	path    string
	auth    string
	summary string
	start   string
	end     string
	err     error
}

func (r *calendarRequestRecorder) record(req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.called = true
	r.method = req.Method
	r.path = req.URL.Path
	r.auth = req.Header.Get("Authorization")

	var payload struct {
		Summary   string `json:"summary"`
		StartTime struct {
			Timestamp string `json:"timestamp"`
		} `json:"start_time"`
		EndTime struct {
			Timestamp string `json:"timestamp"`
		} `json:"end_time"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		r.err = err
		return
	}
	r.summary = payload.Summary
	r.start = payload.StartTime.Timestamp
	r.end = payload.EndTime.Timestamp
}

type recordingApprover struct {
	mu       sync.Mutex
	requests []*tools.ApprovalRequest
}

func (a *recordingApprover) RequestApproval(_ context.Context, req *tools.ApprovalRequest) (*tools.ApprovalResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.requests = append(a.requests, req)
	return &tools.ApprovalResponse{Approved: true, Action: "approve"}, nil
}

func (a *recordingApprover) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.requests)
}

func (a *recordingApprover) last() *tools.ApprovalRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.requests) == 0 {
		return nil
	}
	return a.requests[len(a.requests)-1]
}

type injectingCoordinator struct {
	inner      *agentcoordinator.AgentCoordinator
	larkClient *lark.Client
	oauth      shared.LarkOAuthService
	approver   tools.Approver

	mu         sync.Mutex
	lastResult *agent.TaskResult
	lastErr    error
}

func (c *injectingCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	ctx = shared.WithLarkClient(ctx, c.larkClient)
	if c.oauth != nil {
		ctx = shared.WithLarkOAuth(ctx, c.oauth)
	}
	ctx = shared.WithApprover(ctx, c.approver)
	ctx = agent.WithOutputContext(ctx, &agent.OutputContext{Level: agent.LevelCore})
	result, err := c.inner.ExecuteTask(ctx, task, sessionID, listener)
	c.mu.Lock()
	c.lastResult = result
	c.lastErr = err
	c.mu.Unlock()
	return result, err
}

func (c *injectingCoordinator) snapshot() (*agent.TaskResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastResult, c.lastErr
}

type stubLLMFactory struct {
	client llm.LLMClient
}

func (f stubLLMFactory) GetClient(_, _ string, _ llm.LLMConfig) (llm.LLMClient, error) {
	if f.client == nil {
		return nil, fmt.Errorf("no llm client configured")
	}
	return f.client, nil
}

func (f stubLLMFactory) GetIsolatedClient(provider, model string, cfg llm.LLMConfig) (llm.LLMClient, error) {
	return f.GetClient(provider, model, cfg)
}

func (f stubLLMFactory) DisableRetry() {}

type testContextManager struct{}

func (m testContextManager) EstimateTokens(messages []ports.Message) int {
	return len(messages) * 10
}

func (m testContextManager) Compress(messages []ports.Message, _ int) ([]ports.Message, error) {
	return messages, nil
}

func (m testContextManager) AutoCompact(messages []ports.Message, _ int) ([]ports.Message, bool) {
	return messages, false
}

func (m testContextManager) ShouldCompress(_ []ports.Message, _ int) bool { return false }

func (m testContextManager) Preload(context.Context) error { return nil }

func (m testContextManager) BuildWindow(_ context.Context, session *agentstorage.Session, _ agent.ContextWindowConfig) (agent.ContextWindow, error) {
	if session == nil {
		return agent.ContextWindow{}, fmt.Errorf("session required")
	}
	return agent.ContextWindow{SessionID: session.ID, Messages: session.Messages}, nil
}

func (m testContextManager) RecordTurn(context.Context, agent.ContextTurnRecord) error { return nil }

type recordingNotifier struct {
	mu       sync.Mutex
	messages []larkMessage
}

func (n *recordingNotifier) SendLark(_ context.Context, chatID string, content string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.messages = append(n.messages, larkMessage{ChatID: chatID, Content: content})
	return nil
}

func (n *recordingNotifier) SendMoltbook(_ context.Context, _ string) error { return nil }

func (n *recordingNotifier) lastMessage() (larkMessage, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) == 0 {
		return larkMessage{}, false
	}
	return n.messages[len(n.messages)-1], true
}

type inMemorySessionStore struct {
	mu       sync.Mutex
	sessions map[string]*agentstorage.Session
}

func newInMemorySessionStore() *inMemorySessionStore {
	return &inMemorySessionStore{sessions: make(map[string]*agentstorage.Session)}
}

func (s *inMemorySessionStore) Create(_ context.Context) (*agentstorage.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sessionID := id.NewSessionID()
	session := &agentstorage.Session{ID: sessionID, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	s.sessions[sessionID] = session
	return session, nil
}

func (s *inMemorySessionStore) Get(_ context.Context, sessionID string) (*agentstorage.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session, ok := s.sessions[sessionID]; ok {
		return session, nil
	}
	session := &agentstorage.Session{ID: sessionID, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	s.sessions[sessionID] = session
	return session, nil
}

func (s *inMemorySessionStore) Save(_ context.Context, session *agentstorage.Session) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	session.UpdatedAt = time.Now()
	s.sessions[session.ID] = session
	return nil
}

func (s *inMemorySessionStore) List(_ context.Context, limit int, offset int) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}
	if offset >= len(ids) {
		return []string{}, nil
	}
	ids = ids[offset:]
	if limit > 0 && len(ids) > limit {
		ids = ids[:limit]
	}
	return ids, nil
}

func (s *inMemorySessionStore) Delete(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	return nil
}

func TestSchedulerCalendarFlowE2E(t *testing.T) {
	recorder := &calendarRequestRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/calendar/v4/calendars"):
			resp := map[string]any{
				"code": 0,
				"msg":  "ok",
				"data": map[string]any{
					"calendar_list": []map[string]any{
						{
							"calendar_id": "cal-primary",
							"type":        "primary",
							"role":        "owner",
						},
					},
					"has_more": false,
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/calendar/v4/calendars/") && strings.HasSuffix(r.URL.Path, "/events"):
			recorder.record(r)
			if recorder.err != nil {
				http.Error(w, recorder.err.Error(), http.StatusBadRequest)
				return
			}
			resp := map[string]any{
				"code": 0,
				"msg":  "ok",
				"data": map[string]any{
					"event": map[string]any{
						"event_id": "evt_123",
						"summary":  recorder.summary,
						"start_time": map[string]any{
							"timestamp": recorder.start,
						},
						"end_time": map[string]any{
							"timestamp": recorder.end,
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	larkClient := lark.NewClient("app_id", "app_secret", lark.WithOpenBaseUrl(server.URL), lark.WithHttpClient(server.Client()))

	callCount := 0
	llmClient := &mocks.MockLLMClient{CompleteFunc: func(_ context.Context, _ ports.CompletionRequest) (*ports.CompletionResponse, error) {
		callCount++
		switch callCount {
		case 1:
			return &ports.CompletionResponse{
				Content: "Creating calendar event",
				ToolCalls: []ports.ToolCall{{
					ID:   "call-1",
					Name: "lark_calendar_create",
					Arguments: map[string]any{
						"summary":    "Team sync",
						"start_time": "1700000000",
						"end_time":   "1700003600",
					},
				}},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{TotalTokens: 10},
			}, nil
		default:
			return &ports.CompletionResponse{
				Content:    "Created calendar event evt_123.",
				StopReason: "stop",
				Usage:      ports.TokenUsage{TotalTokens: 5},
			}, nil
		}
	}}

	registry, err := toolregistry.NewRegistry(toolregistry.Config{
		MemoryEngine: memory.NewMarkdownEngine(t.TempDir()),
		HTTPLimits:   config.DefaultHTTPLimitsConfig(),
	})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	sessionStore := newInMemorySessionStore()
	coord := agentcoordinator.NewAgentCoordinator(
		stubLLMFactory{client: llmClient},
		registry,
		sessionStore,
		testContextManager{},
		nil,
		&mocks.MockParser{},
		nil,
		appconfig.Config{
			LLMProvider:   "mock",
			LLMModel:      "test-model",
			MaxIterations: 3,
			Temperature:   0.2,
		},
	)

	approver := &recordingApprover{}
	injecting := &injectingCoordinator{
		inner:      coord,
		larkClient: larkClient,
		oauth:      &staticLarkOAuth{token: "user-token"},
		approver:   approver,
	}

	notifier := &recordingNotifier{}
	sched := New(Config{Enabled: true}, injecting, notifier, nil)

	trigger := Trigger{
		Name:     "calendar-flow",
		Schedule: "* * * * *",
		Task:     "Create a calendar event for the next team sync.",
		Channel:  "lark",
		UserID:   "ou_123",
		ChatID:   "oc_chat",
	}

	if err := sched.executeTrigger(trigger); err != nil {
		t.Fatalf("execute trigger: %v", err)
	}

	result, execErr := injecting.snapshot()
	if execErr != nil {
		t.Fatalf("execute trigger: %v", execErr)
	}
	if result == nil {
		t.Fatal("expected task result")
	}

	if approver.count() != 1 {
		t.Fatalf("expected 1 approval request, got %d", approver.count())
	}
	lastReq := approver.last()
	if lastReq == nil || lastReq.Operation != "lark_calendar_create" || lastReq.ToolName != "lark_calendar_create" {
		t.Fatalf("unexpected approval request: %+v", lastReq)
	}

	if !recorder.called {
		t.Fatal("expected calendar create request to be sent")
	}
	if recorder.err != nil {
		t.Fatalf("calendar request decode failed: %v", recorder.err)
	}
	if recorder.method != http.MethodPost {
		t.Fatalf("expected POST request, got %s", recorder.method)
	}
	if recorder.path != "/open-apis/calendar/v4/calendars/cal-primary/events" {
		t.Fatalf("unexpected request path: %s", recorder.path)
	}
	if recorder.auth != "Bearer user-token" {
		t.Fatalf("unexpected auth header: %s", recorder.auth)
	}
	if recorder.summary != "Team sync" {
		t.Fatalf("unexpected summary: %s", recorder.summary)
	}
	if recorder.start != "1700000000" || recorder.end != "1700003600" {
		t.Fatalf("unexpected timestamps: start=%s end=%s", recorder.start, recorder.end)
	}

	var toolResult *ports.ToolResult
	for _, msg := range result.Messages {
		for _, tr := range msg.ToolResults {
			if tr.CallID == "call-1" {
				toolResult = &tr
				break
			}
		}
	}
	if toolResult == nil {
		t.Fatal("expected tool result for call-1")
	}
	if eventID, ok := toolResult.Metadata["event_id"].(string); !ok || eventID != "evt_123" {
		t.Fatalf("expected event_id metadata, got %v", toolResult.Metadata["event_id"])
	}
	if calendarID, ok := toolResult.Metadata["calendar_id"].(string); !ok || calendarID != "cal-primary" {
		t.Fatalf("expected calendar_id metadata, got %v", toolResult.Metadata["calendar_id"])
	}

	lastMsg, ok := notifier.lastMessage()
	if !ok {
		t.Fatal("expected scheduler notifier message")
	}
	if lastMsg.ChatID != "oc_chat" {
		t.Fatalf("unexpected notifier chat_id: %s", lastMsg.ChatID)
	}
	if lastMsg.Content != result.Answer {
		t.Fatalf("unexpected notifier content: %s", lastMsg.Content)
	}
}
