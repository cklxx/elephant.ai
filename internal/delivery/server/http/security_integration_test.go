//go:build integration

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	serverApp "alex/internal/delivery/server/app"
	agentdomain "alex/internal/domain/agent/ports"
	agentports "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/infra/attachments"
	"alex/internal/infra/session/filestore"
	"alex/internal/infra/tools/builtin/aliases"
	"alex/internal/infra/tools/builtin/pathutil"
	"alex/internal/infra/tools/builtin/shared"
	agenterrors "alex/internal/shared/errors"
)

func TestSecurity_PathTraversal(t *testing.T) {
	server := newSecurityIntegrationServer(t, &pathTraversalExecutor{
		securitySessionSource: newSecuritySessionSource(),
		workspaceRoot:         findRepoRoot(t),
		readFile:              aliases.NewReadFile(shared.FileToolConfig{}),
	})
	defer server.Close()

	created := server.createTask(t, map[string]any{
		"task": "attempt file read outside workspace",
	})

	task := server.waitForTerminalTask(t, created.RunID, 5*time.Second)
	if task.Status != "failed" {
		t.Fatalf("expected failed status, got %q", task.Status)
	}
	if !strings.Contains(task.Error, "escapes workspace root") {
		t.Fatalf("expected workspace escape error, got %q", task.Error)
	}
	if strings.Contains(task.Error, "root:") {
		t.Fatalf("unexpected file content leak in error: %q", task.Error)
	}
}

func TestSecurity_DataRace(t *testing.T) {
	server := newSecurityIntegrationServer(t, &dataRaceExecutor{
		securitySessionSource: newSecuritySessionSource(),
	})
	defer server.Close()

	const (
		taskCount   = 12
		readerCount = 4
	)

	var (
		readersWG sync.WaitGroup
		tasksWG   sync.WaitGroup
		mu        sync.Mutex
		runIDs    = make([]string, 0, taskCount)
		taskErrs  = make(chan error, taskCount+readerCount)
		doneCh    = make(chan struct{})
	)

	for i := 0; i < readerCount; i++ {
		readersWG.Add(1)
		go func() {
			defer readersWG.Done()
			for {
				select {
				case <-doneCh:
					return
				default:
				}
				server.getJSON(t, "/api/tasks?limit=200", &map[string]any{})
				resp, err := server.client.Get(server.baseURL + "/health")
				if err != nil {
					taskErrs <- fmt.Errorf("health request failed: %w", err)
					return
				}
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					taskErrs <- fmt.Errorf("health status = %d", resp.StatusCode)
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	for i := 0; i < taskCount; i++ {
		tasksWG.Add(1)
		go func(i int) {
			defer tasksWG.Done()
			created := server.createTask(t, map[string]any{
				"task": fmt.Sprintf("race-task-%02d", i),
			})
			mu.Lock()
			runIDs = append(runIDs, created.RunID)
			mu.Unlock()

			task := server.waitForTerminalTask(t, created.RunID, 5*time.Second)
			if task.Status != "completed" {
				taskErrs <- fmt.Errorf("task %s status = %s error=%q", created.RunID, task.Status, task.Error)
			}
		}(i)
	}

	tasksWG.Wait()
	close(doneCh)
	readersWG.Wait()
	close(taskErrs)

	for err := range taskErrs {
		if err != nil {
			t.Fatal(err)
		}
	}

	var listed struct {
		Tasks []securityTaskStatus `json:"tasks"`
		Total int                  `json:"total"`
	}
	server.getJSON(t, "/api/tasks?limit=200", &listed)
	if listed.Total < taskCount {
		t.Fatalf("expected at least %d tasks listed, got %d", taskCount, listed.Total)
	}
	if len(runIDs) != taskCount {
		t.Fatalf("expected %d run ids, got %d", taskCount, len(runIDs))
	}
}

func TestSecurity_LLMAllUnavailable(t *testing.T) {
	const friendly = "All configured LLM providers timed out. Please retry shortly."

	server := newSecurityIntegrationServer(t, &llmAllUnavailableExecutor{
		securitySessionSource: newSecuritySessionSource(),
		err: agenterrors.NewTransientError(
			fmt.Errorf("openai timeout; anthropic timeout; kimi timeout: %w", context.DeadlineExceeded),
			friendly,
		),
	})
	defer server.Close()

	created := server.createTask(t, map[string]any{
		"task": "summarize latest blocker state",
	})

	task := server.waitForTerminalTask(t, created.RunID, 5*time.Second)
	if task.Status != "failed" {
		t.Fatalf("expected failed status, got %q", task.Status)
	}
	if task.Error != friendly {
		t.Fatalf("expected friendly error %q, got %q", friendly, task.Error)
	}
	if strings.Contains(strings.ToLower(task.Error), "panic") {
		t.Fatalf("unexpected panic text in task error: %q", task.Error)
	}

	resp, err := server.client.Get(server.baseURL + "/health")
	if err != nil {
		t.Fatalf("health request after LLM failure: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected health 200 after LLM failure, got %d", resp.StatusCode)
	}
}

type securityIntegrationServer struct {
	baseURL string
	client  *http.Client
	server  *httptest.Server
}

func newSecurityIntegrationServer(t *testing.T, exec serverApp.AgentExecutor) *securityIntegrationServer {
	t.Helper()

	broadcaster := serverApp.NewEventBroadcaster()
	taskStore := serverApp.NewInMemoryTaskStore()
	sessionStore := filestore.New(t.TempDir())

	tasksSvc := serverApp.NewTaskExecutionService(exec, broadcaster, taskStore)
	sessionsSvc := serverApp.NewSessionService(exec, sessionStore, broadcaster)
	snapshotsSvc := serverApp.NewSnapshotService(exec, broadcaster)

	router := NewRouter(
		RouterDeps{
			Tasks:         tasksSvc,
			Sessions:      sessionsSvc,
			Snapshots:     snapshotsSvc,
			Broadcaster:   broadcaster,
			HealthChecker: serverApp.NewHealthChecker(),
			AttachmentCfg: attachments.StoreConfig{Dir: t.TempDir()},
		},
		RouterConfig{Environment: "development"},
	)

	srv := httptest.NewServer(router)

	client := srv.Client()
	client.Timeout = 5 * time.Second

	return &securityIntegrationServer{
		baseURL: srv.URL,
		client:  client,
		server:  srv,
	}
}

func (s *securityIntegrationServer) Close() {
	if s != nil && s.server != nil {
		s.server.Close()
	}
}

func (s *securityIntegrationServer) createTask(t *testing.T, payload map[string]any) CreateTaskResponse {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal create task payload: %v", err)
	}
	resp, err := s.client.Post(s.baseURL+"/api/tasks", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create task request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected create task status 201, got %d", resp.StatusCode)
	}
	var created CreateTaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create task response: %v", err)
	}
	if strings.TrimSpace(created.RunID) == "" {
		t.Fatal("create task returned empty run_id")
	}
	return created
}

func (s *securityIntegrationServer) waitForTerminalTask(t *testing.T, runID string, timeout time.Duration) securityTaskStatus {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var task securityTaskStatus
		s.getJSON(t, "/api/tasks/"+runID, &task)
		switch task.Status {
		case "completed", "failed", "cancelled":
			return task
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for terminal task status: %s", runID)
	return securityTaskStatus{}
}

func (s *securityIntegrationServer) getJSON(t *testing.T, path string, out any) {
	t.Helper()

	resp, err := s.client.Get(s.baseURL + path)
	if err != nil {
		t.Fatalf("get %s failed: %v", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get %s status = %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode %s response: %v", path, err)
	}
}

type securityTaskStatus struct {
	RunID       string  `json:"run_id"`
	SessionID   string  `json:"session_id"`
	Status      string  `json:"status"`
	Error       string  `json:"error"`
	CompletedAt *string `json:"completed_at,omitempty"`
}

type securitySessionSource struct {
	next atomic.Uint64
	mu   sync.Mutex
	byID map[string]*storage.Session
}

func newSecuritySessionSource() *securitySessionSource {
	return &securitySessionSource{byID: make(map[string]*storage.Session)}
}

func (s *securitySessionSource) GetSession(_ context.Context, id string) (*storage.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(id) == "" {
		id = fmt.Sprintf("security-session-%d", s.next.Add(1))
	}
	if existing := s.byID[id]; existing != nil {
		cp := *existing
		return &cp, nil
	}
	now := time.Now()
	session := &storage.Session{
		ID:        id,
		Messages:  []agentdomain.Message{},
		Metadata:  map[string]string{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.byID[id] = session
	cp := *session
	return &cp, nil
}

type pathTraversalExecutor struct {
	*securitySessionSource
	workspaceRoot string
	readFile      interface {
		Execute(context.Context, agentdomain.ToolCall) (*agentdomain.ToolResult, error)
	}
}

func (e *pathTraversalExecutor) ExecuteTask(ctx context.Context, task string, sessionID string, listener agentports.EventListener) (*agentports.TaskResult, error) {
	ctx = pathutil.WithWorkingDir(ctx, e.workspaceRoot)
	result, err := e.readFile.Execute(ctx, agentdomain.ToolCall{
		ID:   "security-path-traversal",
		Name: "read_file",
		Arguments: map[string]any{
			"path": "../../etc/passwd",
		},
	})
	if err != nil {
		return nil, err
	}
	if result == nil || result.Error == nil {
		return nil, fmt.Errorf("expected path traversal to be blocked")
	}
	return nil, result.Error
}

func (e *pathTraversalExecutor) GetConfig() agentports.AgentConfig {
	return agentports.AgentConfig{LLMProvider: "mock", LLMModel: "security-path"}
}

func (e *pathTraversalExecutor) PreviewContextWindow(ctx context.Context, sessionID string) (agentports.ContextWindowPreview, error) {
	return agentports.ContextWindowPreview{}, nil
}

type dataRaceExecutor struct {
	*securitySessionSource
	runs atomic.Uint64
}

func (e *dataRaceExecutor) ExecuteTask(ctx context.Context, task string, sessionID string, listener agentports.EventListener) (*agentports.TaskResult, error) {
	runNo := e.runs.Add(1)
	time.Sleep(time.Duration(10+(runNo%5)*5) * time.Millisecond)
	return &agentports.TaskResult{
		Answer:     "ok: " + task,
		Iterations: 1,
		TokensUsed: int(runNo),
		SessionID:  sessionID,
		RunID:      fmt.Sprintf("race-run-%d", runNo),
		StopReason: "completed",
	}, nil
}

func (e *dataRaceExecutor) GetConfig() agentports.AgentConfig {
	return agentports.AgentConfig{LLMProvider: "mock", LLMModel: "security-race"}
}

func (e *dataRaceExecutor) PreviewContextWindow(ctx context.Context, sessionID string) (agentports.ContextWindowPreview, error) {
	return agentports.ContextWindowPreview{}, nil
}

type llmAllUnavailableExecutor struct {
	*securitySessionSource
	err error
}

func (e *llmAllUnavailableExecutor) ExecuteTask(ctx context.Context, task string, sessionID string, listener agentports.EventListener) (*agentports.TaskResult, error) {
	return nil, e.err
}

func (e *llmAllUnavailableExecutor) GetConfig() agentports.AgentConfig {
	return agentports.AgentConfig{LLMProvider: "mock", LLMModel: "security-llm-fail"}
}

func (e *llmAllUnavailableExecutor) PreviewContextWindow(ctx context.Context, sessionID string) (agentports.ContextWindowPreview, error) {
	return agentports.ContextWindowPreview{}, nil
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}
