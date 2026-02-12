package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/subscription"
	"alex/internal/delivery/server/app"
	serverPorts "alex/internal/delivery/server/ports"
	"alex/internal/domain/agent"
	core "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/infra/analytics/journal"
	"alex/internal/infra/memory"
	"alex/internal/infra/session/filestore"
	sessionstate "alex/internal/infra/session/state_store"
	runtimeconfig "alex/internal/shared/config"
)

type failingAgentCoordinator struct {
	err error
}

func (f *failingAgentCoordinator) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	return nil, f.err
}

func (f *failingAgentCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	return nil, errors.New("not implemented")
}

func (f *failingAgentCoordinator) GetConfig() agent.AgentConfig {
	return agent.AgentConfig{}
}

func (f *failingAgentCoordinator) PreviewContextWindow(ctx context.Context, sessionID string) (agent.ContextWindowPreview, error) {
	return agent.ContextWindowPreview{}, f.err
}

type stubAgentCoordinator struct{}

func (stubAgentCoordinator) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	if id == "" {
		id = "stub-session"
	}
	return &storage.Session{
		ID:        id,
		Messages:  []core.Message{},
		Metadata:  map[string]string{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (stubAgentCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	return &agent.TaskResult{SessionID: sessionID}, nil
}

func (stubAgentCoordinator) GetConfig() agent.AgentConfig {
	return agent.AgentConfig{}
}

func (stubAgentCoordinator) PreviewContextWindow(ctx context.Context, sessionID string) (agent.ContextWindowPreview, error) {
	return agent.ContextWindowPreview{
		Window: agent.ContextWindow{
			SessionID: sessionID,
		},
		ToolMode: "cli",
	}, nil
}

type storeBackedAgentCoordinator struct {
	store storage.SessionStore
}

func (c storeBackedAgentCoordinator) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	if id == "" {
		return c.store.Create(ctx)
	}
	return c.store.Get(ctx, id)
}

func (storeBackedAgentCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	return &agent.TaskResult{SessionID: sessionID}, nil
}

func (storeBackedAgentCoordinator) GetConfig() agent.AgentConfig {
	return agent.AgentConfig{}
}

func (storeBackedAgentCoordinator) PreviewContextWindow(ctx context.Context, sessionID string) (agent.ContextWindowPreview, error) {
	return agent.ContextWindowPreview{
		Window: agent.ContextWindow{
			SessionID: sessionID,
		},
		TokenLimit: 128000,
		ToolMode:   "cli",
	}, nil
}

type previewAgentCoordinator struct{}

func (previewAgentCoordinator) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	if id == "" {
		id = "preview-session"
	}
	now := time.Now()
	return &storage.Session{
		ID:        id,
		Messages:  []core.Message{},
		Metadata:  map[string]string{},
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (previewAgentCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	return &agent.TaskResult{SessionID: sessionID}, nil
}

func (previewAgentCoordinator) GetConfig() agent.AgentConfig {
	return agent.AgentConfig{MaxTokens: 2048, AgentPreset: "dev-debug", ToolPreset: "full"}
}

func (previewAgentCoordinator) PreviewContextWindow(ctx context.Context, sessionID string) (agent.ContextWindowPreview, error) {
	attachments := map[string]core.Attachment{
		"report.md": {
			Name:      "report.md",
			MediaType: "text/markdown",
			Data:      "YmFzZTY0LWRhdGE=",
		},
	}
	messages := []core.Message{
		{Role: "system", Content: "System seed", Attachments: attachments},
		{Role: "user", Content: "Ping", Attachments: attachments},
	}

	return agent.ContextWindowPreview{
		Window: agent.ContextWindow{
			SessionID:    sessionID,
			Messages:     messages,
			SystemPrompt: "base prompt",
			Static: agent.StaticContext{
				Persona: agent.PersonaProfile{ID: "debugger"},
				Tools:   []string{"code_read"},
			},
		},
		TokenEstimate: 321,
		TokenLimit:    2048,
		PersonaKey:    "dev-debug",
		ToolMode:      "cli",
		ToolPreset:    "full",
	}, nil
}

type selectionAwareCoordinator struct {
	selection subscription.ResolvedSelection
	got       chan struct{}
}

func (c *selectionAwareCoordinator) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	if id == "" {
		id = "stub-session"
	}
	return &storage.Session{ID: id, Metadata: map[string]string{}}, nil
}

func (c *selectionAwareCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	if sel, ok := appcontext.GetLLMSelection(ctx); ok {
		c.selection = sel
	}
	close(c.got)
	return &agent.TaskResult{SessionID: sessionID}, nil
}

func (c *selectionAwareCoordinator) GetConfig() agent.AgentConfig {
	return agent.AgentConfig{}
}

func (c *selectionAwareCoordinator) PreviewContextWindow(ctx context.Context, sessionID string) (agent.ContextWindowPreview, error) {
	return agent.ContextWindowPreview{}, nil
}

// buildTestServices creates the 3 services from the given executor and stores.
func buildTestServices(
	exec app.AgentExecutor,
	broadcaster *app.EventBroadcaster,
	sessionStore storage.SessionStore,
	taskStore serverPorts.TaskStore,
	stateStore sessionstate.Store,
	snapshotOpts ...app.SnapshotServiceOption,
) (*app.TaskExecutionService, *app.SessionService, *app.SnapshotService) {
	var taskOpts []app.TaskExecutionServiceOption
	var sessOpts []app.SessionServiceOption
	if stateStore != nil {
		taskOpts = append(taskOpts, app.WithTaskStateStore(stateStore))
		sessOpts = append(sessOpts, app.WithSessionStateStore(stateStore))
		snapshotOpts = append([]app.SnapshotServiceOption{app.WithSnapshotStateStore(stateStore)}, snapshotOpts...)
	}
	tasks := app.NewTaskExecutionService(exec, broadcaster, taskStore, taskOpts...)
	sessions := app.NewSessionService(exec, sessionStore, broadcaster, sessOpts...)
	snapshots := app.NewSnapshotService(exec, broadcaster, snapshotOpts...)
	return tasks, sessions, snapshots
}

func TestHandleCreateTaskReturnsJSONErrorOnSessionDecodeFailure(t *testing.T) {
	rootErr := errors.New("json: cannot unmarshal object into Go struct field ToolResult.messages.tool_results.error of type error")
	tasks, sessions, snapshots := buildTestServices(&failingAgentCoordinator{err: rootErr}, app.NewEventBroadcaster(), nil, nil, nil)
	handler := NewAPIHandler(tasks, sessions, snapshots, app.NewHealthChecker(), false)

	reqBody := bytes.NewBufferString(`{"task":"demo"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/tasks", reqBody)
	rr := httptest.NewRecorder()

	handler.HandleCreateTask(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("expected JSON content type, got %s", contentType)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if resp["error"] != "Failed to create task" {
		t.Fatalf("expected error message 'Failed to create task', got %s", resp["error"])
	}

	expectedDetails := "failed to get/create session: " + rootErr.Error()
	if resp["details"] != expectedDetails {
		t.Fatalf("expected details %q, got %q", expectedDetails, resp["details"])
	}
}

func TestHandleCreateTaskReturnsNotFoundOnMissingSession(t *testing.T) {
	tasks, sessions, snapshots := buildTestServices(&failingAgentCoordinator{err: storage.ErrSessionNotFound}, app.NewEventBroadcaster(), nil, nil, nil)
	handler := NewAPIHandler(tasks, sessions, snapshots, app.NewHealthChecker(), false)

	reqBody := bytes.NewBufferString(`{"task":"demo","session_id":"missing-session"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/tasks", reqBody)
	rr := httptest.NewRecorder()

	handler.HandleCreateTask(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Session not found") {
		t.Fatalf("expected response to mention session not found, got %s", rr.Body.String())
	}
}

func TestHandleCreateTaskHonorsBodyLimit(t *testing.T) {
	tasks, sessions, snapshots := buildTestServices(&stubAgentCoordinator{}, app.NewEventBroadcaster(), nil, nil, nil)
	handler := NewAPIHandler(tasks, sessions, snapshots, app.NewHealthChecker(), false, WithMaxCreateTaskBodySize(64))

	oversizedPayload := `{"task":"` + strings.Repeat("a", 80) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(oversizedPayload))
	rr := httptest.NewRecorder()

	handler.HandleCreateTask(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rr.Code)
	}
}

func TestSnapshotHandlers(t *testing.T) {
	sessionStore := filestore.New(t.TempDir())
	stateStore := sessionstate.NewInMemoryStore()
	broadcaster := app.NewEventBroadcaster()
	taskStore := app.NewInMemoryTaskStore()
	reader := &staticJournalReader{entries: []journal.TurnJournalEntry{{SessionID: "sess-1", TurnID: 1, Summary: "rehydrate"}}}
	tasks, sessions, snapshots := buildTestServices(
		&stubAgentCoordinator{},
		broadcaster,
		sessionStore,
		taskStore,
		stateStore,
		app.WithSnapshotJournalReader(reader),
	)
	handler := NewAPIHandler(tasks, sessions, snapshots, app.NewHealthChecker(), false)

	snapshot := sessionstate.Snapshot{
		SessionID:  "sess-1",
		TurnID:     1,
		LLMTurnSeq: 1,
		CreatedAt:  time.Now().UTC(),
		Summary:    "observed",
		Messages:   []core.Message{{Role: "system", Content: "hello"}},
	}
	if err := stateStore.SaveSnapshot(context.Background(), snapshot); err != nil {
		t.Fatalf("failed to seed snapshot: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/sess-1/snapshots", nil)
	req.SetPathValue("session_id", "sess-1")
	resp := httptest.NewRecorder()
	handler.HandleListSnapshots(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.Code)
	}
	var list SessionSnapshotsResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].TurnID != 1 {
		t.Fatalf("unexpected snapshot list payload: %+v", list)
	}

	turnReq := httptest.NewRequest(http.MethodGet, "/api/sessions/sess-1/turns/1", nil)
	turnReq.SetPathValue("session_id", "sess-1")
	turnReq.SetPathValue("turn_id", "1")
	turnResp := httptest.NewRecorder()
	handler.HandleGetTurnSnapshot(turnResp, turnReq)
	if turnResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for turn snapshot, got %d", turnResp.Code)
	}
	var turn TurnSnapshotResponse
	if err := json.Unmarshal(turnResp.Body.Bytes(), &turn); err != nil {
		t.Fatalf("failed to decode turn response: %v", err)
	}
	if turn.SessionID != "sess-1" || turn.TurnID != 1 {
		t.Fatalf("unexpected turn payload: %+v", turn)
	}

	replayReq := httptest.NewRequest(http.MethodPost, "/api/sessions/sess-1/replay", nil)
	replayReq.SetPathValue("session_id", "sess-1")
	replayResp := httptest.NewRecorder()
	handler.HandleReplaySession(replayResp, replayReq)
	if replayResp.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for replay, got %d", replayResp.Code)
	}
}

func TestHandleCreateSession(t *testing.T) {
	sessionStore := filestore.New(t.TempDir())
	stateStore := sessionstate.NewInMemoryStore()
	broadcaster := app.NewEventBroadcaster()
	taskStore := app.NewInMemoryTaskStore()
	tasks, sessions, snapshots := buildTestServices(
		storeBackedAgentCoordinator{store: sessionStore},
		broadcaster,
		sessionStore,
		taskStore,
		stateStore,
	)
	handler := NewAPIHandler(tasks, sessions, snapshots, app.NewHealthChecker(), false)

	req := httptest.NewRequest(http.MethodPost, "/api/sessions", nil)
	resp := httptest.NewRecorder()
	handler.HandleCreateSession(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d", resp.Code)
	}

	var payload struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.SessionID == "" {
		t.Fatalf("expected session_id to be set")
	}
	if _, err := sessionStore.Get(context.Background(), payload.SessionID); err != nil {
		t.Fatalf("expected created session to be retrievable: %v", err)
	}
}

func TestHandleListSessionsIncludesTaskSummaryFields(t *testing.T) {
	sessionStore := filestore.New(t.TempDir())
	stateStore := sessionstate.NewInMemoryStore()
	broadcaster := app.NewEventBroadcaster()
	taskStore := app.NewInMemoryTaskStore()
	defer taskStore.Close()
	tasks, sessions, snapshots := buildTestServices(
		storeBackedAgentCoordinator{store: sessionStore},
		broadcaster,
		sessionStore,
		taskStore,
		stateStore,
	)
	handler := NewAPIHandler(tasks, sessions, snapshots, app.NewHealthChecker(), false)

	ctx := context.Background()
	sessionA, err := sessionStore.Create(ctx)
	if err != nil {
		t.Fatalf("create session A: %v", err)
	}
	sessionA.Metadata["title"] = "Session A"
	if err := sessionStore.Save(ctx, sessionA); err != nil {
		t.Fatalf("save session A: %v", err)
	}

	sessionB, err := sessionStore.Create(ctx)
	if err != nil {
		t.Fatalf("create session B: %v", err)
	}
	sessionB.Metadata["title"] = "Session B"
	if err := sessionStore.Save(ctx, sessionB); err != nil {
		t.Fatalf("save session B: %v", err)
	}

	sessionC, err := sessionStore.Create(ctx)
	if err != nil {
		t.Fatalf("create session C: %v", err)
	}
	sessionC.Metadata["title"] = "Session C"
	if err := sessionStore.Save(ctx, sessionC); err != nil {
		t.Fatalf("save session C: %v", err)
	}

	if _, err := taskStore.Create(ctx, sessionA.ID, "session A old", "", ""); err != nil {
		t.Fatalf("create task A old: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if _, err := taskStore.Create(ctx, sessionA.ID, "session A latest", "", ""); err != nil {
		t.Fatalf("create task A latest: %v", err)
	}
	if _, err := taskStore.Create(ctx, sessionB.ID, "session B only", "", ""); err != nil {
		t.Fatalf("create task B only: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sessions?limit=10&offset=0", nil)
	resp := httptest.NewRecorder()
	handler.HandleListSessions(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.Code)
	}

	var payload SessionListResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	byID := make(map[string]SessionResponse, len(payload.Sessions))
	for _, sess := range payload.Sessions {
		byID[sess.ID] = sess
	}

	summaryA, ok := byID[sessionA.ID]
	if !ok {
		t.Fatalf("session A missing from response")
	}
	if summaryA.TaskCount != 2 {
		t.Fatalf("expected session A task_count=2, got %d", summaryA.TaskCount)
	}
	if summaryA.LastTask != "session A latest" {
		t.Fatalf("expected session A last_task 'session A latest', got %q", summaryA.LastTask)
	}

	summaryB, ok := byID[sessionB.ID]
	if !ok {
		t.Fatalf("session B missing from response")
	}
	if summaryB.TaskCount != 1 {
		t.Fatalf("expected session B task_count=1, got %d", summaryB.TaskCount)
	}
	if summaryB.LastTask != "session B only" {
		t.Fatalf("expected session B last_task 'session B only', got %q", summaryB.LastTask)
	}

	summaryC, ok := byID[sessionC.ID]
	if !ok {
		t.Fatalf("session C missing from response")
	}
	if summaryC.TaskCount != 0 {
		t.Fatalf("expected session C task_count=0, got %d", summaryC.TaskCount)
	}
	if summaryC.LastTask != "" {
		t.Fatalf("expected session C last_task empty, got %q", summaryC.LastTask)
	}
}

type staticJournalReader struct {
	entries []journal.TurnJournalEntry
}

func (r *staticJournalReader) Stream(_ context.Context, sessionID string, fn func(journal.TurnJournalEntry) error) error {
	for _, entry := range r.entries {
		e := entry
		if e.SessionID == "" {
			e.SessionID = sessionID
		}
		if err := fn(e); err != nil {
			return err
		}
	}
	return nil
}

func TestHandleGetContextSnapshotsSanitizesDuplicateAttachments(t *testing.T) {
	broadcaster := app.NewEventBroadcaster()
	tasks, sessions, snapshots := buildTestServices(
		&stubAgentCoordinator{},
		broadcaster,
		filestore.New(t.TempDir()),
		app.NewInMemoryTaskStore(),
		sessionstate.NewInMemoryStore(),
	)
	handler := NewAPIHandler(tasks, sessions, snapshots, app.NewHealthChecker(), true)

	attachments := map[string]core.Attachment{
		"preview.png": {
			Name:      "preview.png",
			MediaType: "image/png",
			Data:      "iVBORw0KGgo=",
			URI:       "https://cdn.example/preview.png",
		},
		"notes.txt": {
			Name:      "notes.txt",
			MediaType: "text/plain",
			Data:      "hello",
		},
	}

	message := core.Message{
		Role:        "assistant",
		Content:     "see [preview.png]",
		Attachments: attachments,
	}

	broadcaster.OnEvent(domain.NewDiagnosticContextSnapshotEvent(
		agent.LevelCore,
		"sess-ctx",
		"task-1",
		"",
		1,
		1,
		"req-1",
		[]core.Message{message},
		nil,
		time.Now(),
	))

	broadcaster.OnEvent(domain.NewDiagnosticContextSnapshotEvent(
		agent.LevelCore,
		"sess-ctx",
		"task-1",
		"",
		2,
		2,
		"req-2",
		[]core.Message{message},
		nil,
		time.Now().Add(time.Second),
	))

	req := httptest.NewRequest(http.MethodGet, "/api/internal/sessions/sess-ctx/context", nil)
	req.SetPathValue("session_id", "sess-ctx")
	resp := httptest.NewRecorder()
	handler.HandleGetContextSnapshots(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var body ContextSnapshotResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(body.Snapshots) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(body.Snapshots))
	}

	first := body.Snapshots[0]
	if len(first.Messages) != 1 {
		t.Fatalf("expected 1 message in first snapshot, got %d", len(first.Messages))
	}
	firstAttachments := first.Messages[0].Attachments
	if len(firstAttachments) != 2 {
		t.Fatalf("expected 2 attachments in first snapshot, got %d", len(firstAttachments))
	}
	if firstAttachments["preview.png"].Data != "iVBORw0KGgo=" {
		t.Fatalf("expected image data to remain, got %q", firstAttachments["preview.png"].Data)
	}
	if firstAttachments["notes.txt"].Data != "hello" {
		t.Fatalf("expected text attachment data to remain, got %q", firstAttachments["notes.txt"].Data)
	}

	second := body.Snapshots[1]
	if len(second.Messages) != 1 {
		t.Fatalf("expected 1 message in second snapshot, got %d", len(second.Messages))
	}
	if second.Messages[0].Attachments != nil {
		t.Fatalf("expected duplicate attachments to be omitted, got %v", second.Messages[0].Attachments)
	}
}

func TestHandleGetContextWindowPreviewReturnsWindow(t *testing.T) {
	tasks, sessions, snapshots := buildTestServices(&previewAgentCoordinator{}, app.NewEventBroadcaster(), nil, nil, nil)
	handler := NewAPIHandler(tasks, sessions, snapshots, app.NewHealthChecker(), false, WithDevMode(true))

	req := httptest.NewRequest(http.MethodGet, "/api/dev/sessions/dev-ctx/context-window", nil)
	req.SetPathValue("session_id", "dev-ctx")
	resp := httptest.NewRecorder()

	handler.HandleGetContextWindowPreview(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var body ContextWindowPreviewResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body.SessionID != "dev-ctx" {
		t.Fatalf("expected session id dev-ctx, got %s", body.SessionID)
	}
	if body.TokenEstimate != 321 {
		t.Fatalf("expected token estimate 321, got %d", body.TokenEstimate)
	}
	if body.TokenLimit != 2048 {
		t.Fatalf("expected token limit 2048, got %d", body.TokenLimit)
	}
	if body.PersonaKey != "dev-debug" {
		t.Fatalf("expected persona key dev-debug, got %s", body.PersonaKey)
	}
	if body.ToolPreset != "full" {
		t.Fatalf("expected tool preset full, got %s", body.ToolPreset)
	}

	if len(body.Window.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(body.Window.Messages))
	}
	if len(body.Window.Messages[0].Attachments) != 1 {
		t.Fatalf("expected first message to retain attachment, got %v", body.Window.Messages[0].Attachments)
	}
	if body.Window.Messages[1].Attachments != nil {
		t.Fatalf("expected duplicate attachments to be skipped in subsequent messages, got %v", body.Window.Messages[1].Attachments)
	}
	if body.Window.SystemPrompt != "base prompt" {
		t.Fatalf("unexpected system prompt: %s", body.Window.SystemPrompt)
	}
}

func TestHandleGetContextWindowPreviewDisabledOutsideDev(t *testing.T) {
	tasks, sessions, snapshots := buildTestServices(&previewAgentCoordinator{}, app.NewEventBroadcaster(), nil, nil, nil)
	handler := NewAPIHandler(tasks, sessions, snapshots, app.NewHealthChecker(), false)

	req := httptest.NewRequest(http.MethodGet, "/api/dev/sessions/dev-ctx/context-window", nil)
	req.SetPathValue("session_id", "dev-ctx")
	resp := httptest.NewRecorder()

	handler.HandleGetContextWindowPreview(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when dev mode disabled, got %d", resp.Code)
	}
}

func TestHandleGetMemorySnapshot(t *testing.T) {
	ctx := context.Background()
	memoryRoot := t.TempDir()
	engine := memory.NewMarkdownEngine(memoryRoot)
	if err := engine.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	userID := "user-1"
	userRoot := memory.ResolveUserRoot(memoryRoot, userID)
	if err := os.MkdirAll(userRoot, 0o755); err != nil {
		t.Fatalf("mkdir user root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userRoot, "MEMORY.md"), []byte("long-term note"), 0o644); err != nil {
		t.Fatalf("write long term: %v", err)
	}
	_, err := engine.AppendDaily(ctx, userID, memory.DailyEntry{
		Title:     "Note",
		Content:   "daily note",
		CreatedAt: time.Date(2026, 2, 3, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("append daily: %v", err)
	}

	sessionStore := filestore.New(t.TempDir())
	session := &storage.Session{
		ID:        "sess-1",
		Messages:  []core.Message{},
		Metadata:  map[string]string{"user_id": userID},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := sessionStore.Save(ctx, session); err != nil {
		t.Fatalf("save session: %v", err)
	}

	tasks, sessions, snapshots := buildTestServices(
		storeBackedAgentCoordinator{store: sessionStore},
		app.NewEventBroadcaster(),
		sessionStore,
		app.NewInMemoryTaskStore(),
		nil,
	)
	handler := NewAPIHandler(tasks, sessions, snapshots, app.NewHealthChecker(), false, WithDevMode(true), WithMemoryEngine(engine))

	req := httptest.NewRequest(http.MethodGet, "/api/dev/memory?session_id=sess-1", nil)
	resp := httptest.NewRecorder()

	handler.HandleGetMemorySnapshot(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var body MemorySnapshot
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.UserID != userID {
		t.Fatalf("expected user_id %q, got %q", userID, body.UserID)
	}
	if body.LongTerm != "long-term note" {
		t.Fatalf("unexpected long-term content: %q", body.LongTerm)
	}
	if len(body.Daily) != 1 {
		t.Fatalf("expected 1 daily entry, got %d", len(body.Daily))
	}
	if body.Daily[0].Date != "2026-02-03" {
		t.Fatalf("unexpected daily date: %q", body.Daily[0].Date)
	}
	if !strings.Contains(body.Daily[0].Content, "daily note") {
		t.Fatalf("unexpected daily content: %q", body.Daily[0].Content)
	}
}

func TestHandleWebVitalsAcceptsPayload(t *testing.T) {
	handler := NewAPIHandler(nil, nil, nil, app.NewHealthChecker(), false)
	req := httptest.NewRequest(http.MethodPost, "/api/metrics/web-vitals", strings.NewReader(`{"name":"CLS","value":0.1,"page":"/sessions/123"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	handler.HandleWebVitals(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.Code)
	}
}

func TestHandleWebVitalsRejectsBadMethod(t *testing.T) {
	router := NewRouter(
		RouterDeps{
			Broadcaster:   app.NewEventBroadcaster(),
			HealthChecker: app.NewHealthChecker(),
		},
		RouterConfig{Environment: "test"},
	)
	req := httptest.NewRequest(http.MethodGet, "/api/metrics/web-vitals", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.Code)
	}
}

func TestHandleCreateTaskInjectsSelection(t *testing.T) {
	coord := &selectionAwareCoordinator{got: make(chan struct{})}
	tasks, sessions, snapshots := buildTestServices(coord, app.NewEventBroadcaster(), nil, app.NewInMemoryTaskStore(), nil)
	handler := NewAPIHandler(tasks, sessions, snapshots, app.NewHealthChecker(), false, WithSelectionResolver(subscription.NewSelectionResolver(func() runtimeconfig.CLICredentials {
		return runtimeconfig.CLICredentials{
			Codex: runtimeconfig.CLICredential{
				Provider: "codex",
				APIKey:   "tok",
				BaseURL:  "https://chatgpt.com/backend-api/codex",
				Source:   runtimeconfig.SourceCodexCLI,
			},
		}
	})))

	body := `{"task":"hi","llm_selection":{"mode":"cli","provider":"codex","model":"gpt-5.2-codex","source":"codex_cli"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(body))
	rr := httptest.NewRecorder()

	handler.HandleCreateTask(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	select {
	case <-coord.got:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for selection injection")
	}

	if coord.selection.Provider != "codex" || coord.selection.Model != "gpt-5.2-codex" {
		t.Fatalf("selection not injected: %#v", coord.selection)
	}
}

func TestHandleListActiveTasks(t *testing.T) {
	taskStore := app.NewInMemoryTaskStore()
	tasks, sessions, snapshots := buildTestServices(
		&stubAgentCoordinator{},
		app.NewEventBroadcaster(),
		nil,
		taskStore,
		nil,
	)
	handler := NewAPIHandler(tasks, sessions, snapshots, app.NewHealthChecker(), false)

	ctx := context.Background()

	t1, err := taskStore.Create(ctx, "sess-1", "running task", "", "")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	_ = taskStore.SetStatus(ctx, t1.ID, serverPorts.TaskStatusRunning)

	t2, err := taskStore.Create(ctx, "sess-1", "pending task", "", "")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	t3, err := taskStore.Create(ctx, "sess-2", "completed task", "", "")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	_ = taskStore.SetStatus(ctx, t3.ID, serverPorts.TaskStatusRunning)
	_ = taskStore.SetResult(ctx, t3.ID, &agent.TaskResult{SessionID: "sess-2"})

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/active", nil)
	rr := httptest.NewRecorder()
	handler.HandleListActiveTasks(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Tasks []TaskStatusResponse `json:"tasks"`
		Total int                  `json:"total"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Total != 2 {
		t.Fatalf("expected 2 active tasks, got %d", resp.Total)
	}
	activeIDs := map[string]bool{}
	for _, task := range resp.Tasks {
		activeIDs[task.RunID] = true
	}
	if !activeIDs[t1.ID] || !activeIDs[t2.ID] {
		t.Fatalf("expected active task IDs %s and %s, got %v", t1.ID, t2.ID, activeIDs)
	}
	if activeIDs[t3.ID] {
		t.Fatal("completed task should not appear in active list")
	}
}

func TestHandleGetTaskStats(t *testing.T) {
	taskStore := app.NewInMemoryTaskStore()
	tasks, sessions, snapshots := buildTestServices(
		&stubAgentCoordinator{},
		app.NewEventBroadcaster(),
		nil,
		taskStore,
		nil,
	)
	handler := NewAPIHandler(tasks, sessions, snapshots, app.NewHealthChecker(), false)

	ctx := context.Background()

	t1, _ := taskStore.Create(ctx, "sess-1", "running", "", "")
	_ = taskStore.SetStatus(ctx, t1.ID, serverPorts.TaskStatusRunning)

	_, _ = taskStore.Create(ctx, "sess-1", "pending", "", "")

	t3, _ := taskStore.Create(ctx, "sess-2", "completed", "", "")
	_ = taskStore.SetStatus(ctx, t3.ID, serverPorts.TaskStatusRunning)
	_ = taskStore.SetResult(ctx, t3.ID, &agent.TaskResult{SessionID: "sess-2", Iterations: 5})

	t4, _ := taskStore.Create(ctx, "sess-2", "failed", "", "")
	_ = taskStore.SetError(ctx, t4.ID, errors.New("boom"))

	t5, _ := taskStore.Create(ctx, "sess-3", "cancelled", "", "")
	_ = taskStore.SetStatus(ctx, t5.ID, serverPorts.TaskStatusCancelled)

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/stats", nil)
	rr := httptest.NewRecorder()
	handler.HandleGetTaskStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var stats app.TaskStats
	if err := json.Unmarshal(rr.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if stats.TotalCount != 5 {
		t.Fatalf("expected 5 total, got %d", stats.TotalCount)
	}
	if stats.RunningCount != 1 {
		t.Fatalf("expected 1 running, got %d", stats.RunningCount)
	}
	if stats.PendingCount != 1 {
		t.Fatalf("expected 1 pending, got %d", stats.PendingCount)
	}
	if stats.CompletedCount != 1 {
		t.Fatalf("expected 1 completed, got %d", stats.CompletedCount)
	}
	if stats.FailedCount != 1 {
		t.Fatalf("expected 1 failed, got %d", stats.FailedCount)
	}
	if stats.CancelledCount != 1 {
		t.Fatalf("expected 1 cancelled, got %d", stats.CancelledCount)
	}
	if stats.ActiveCount != 2 {
		t.Fatalf("expected 2 active, got %d", stats.ActiveCount)
	}
}

func (r *staticJournalReader) ReadAll(_ context.Context, sessionID string) ([]journal.TurnJournalEntry, error) {
	entries := make([]journal.TurnJournalEntry, len(r.entries))
	copy(entries, r.entries)
	for i := range entries {
		if entries[i].SessionID == "" {
			entries[i].SessionID = sessionID
		}
	}
	return entries, nil
}
