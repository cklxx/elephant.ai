//go:build integration

package http

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/app/di"
	"alex/internal/delivery/server/app"
	"alex/internal/infra/attachments"
)

// testServer wraps the common DI + router + httptest setup for Suite 2 tests.
type testServer struct {
	Server      *httptest.Server
	Router      http.Handler
	Broadcaster *app.EventBroadcaster
	TaskStore   *app.InMemoryTaskStore
	TasksSvc    *app.TaskExecutionService
}

func newTestServer(t *testing.T, leaderToken string) *testServer {
	t.Helper()

	config := di.Config{
		LLMProvider: "mock",
		LLMModel:    "test",
	}

	container, err := di.BuildContainer(config)
	if err != nil {
		t.Fatalf("BuildContainer failed: %v", err)
	}
	t.Cleanup(func() { _ = container.Shutdown() })

	if err := container.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	broadcaster := app.NewEventBroadcaster()
	taskStore := app.NewInMemoryTaskStore()
	t.Cleanup(taskStore.Close)

	tasksSvc := app.NewTaskExecutionService(
		container.AgentCoordinator,
		broadcaster,
		taskStore,
	)

	sessionsSvc := app.NewSessionService(
		container.AgentCoordinator,
		container.SessionStore,
		broadcaster,
	)
	snapshotsSvc := app.NewSnapshotService(
		container.AgentCoordinator,
		broadcaster,
		app.WithSnapshotStateStore(container.StateStore),
	)

	healthChecker := app.NewHealthChecker()
	healthChecker.RegisterProbe(app.NewLLMFactoryProbe(container))

	router := NewRouter(
		RouterDeps{
			Tasks:         tasksSvc,
			Sessions:      sessionsSvc,
			Snapshots:     snapshotsSvc,
			Broadcaster:   broadcaster,
			HealthChecker: healthChecker,
			AttachmentCfg: attachments.StoreConfig{Dir: t.TempDir()},
		},
		RouterConfig{
			Environment:    "development",
			LeaderAPIToken: leaderToken,
		},
	)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	return &testServer{
		Server:      srv,
		Router:      router,
		Broadcaster: broadcaster,
		TaskStore:   taskStore,
		TasksSvc:    tasksSvc,
	}
}

// sseEvent represents a parsed SSE event from the stream.
type sseEvent struct {
	EventType string
	Data      map[string]interface{}
}

// collectSSEEvents connects to an SSE endpoint and collects events until the
// context is cancelled or the terminal condition is met. Returns collected events.
func collectSSEEvents(ctx context.Context, url string) ([]sseEvent, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SSE returned status %d", resp.StatusCode)
	}

	var events []sseEvent
	scanner := bufio.NewScanner(resp.Body)
	var eventType, dataLine string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			dataLine = strings.TrimPrefix(line, "data: ")
		} else if line == "" && dataLine != "" {
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(dataLine), &data); err == nil {
				events = append(events, sseEvent{EventType: eventType, Data: data})
			}
			eventType = ""
			dataLine = ""
		} else if strings.HasPrefix(line, ": ") {
			// heartbeat comment, ignore
			continue
		}
	}

	return events, nil
}

// createTask sends POST /api/tasks and returns the parsed response.
func createTask(t *testing.T, baseURL, taskText string) CreateTaskResponse {
	t.Helper()
	body, _ := json.Marshal(CreateTaskRequest{Task: taskText})
	resp, err := http.Post(baseURL+"/api/tasks", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/tasks failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errBody bytes.Buffer
		errBody.ReadFrom(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, errBody.String())
	}

	var result CreateTaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return result
}

// ---------------------------------------------------------------------------
// Test 2.1: Task Creation E2E
// POST /api/tasks with mock LLM → verify 201 + SSE events
// ---------------------------------------------------------------------------
func TestAPI_TaskCreation_E2E(t *testing.T) {
	ts := newTestServer(t, "")

	// Create task
	result := createTask(t, ts.Server.URL, "Say hello")

	if result.RunID == "" {
		t.Fatal("expected non-empty run_id")
	}
	if result.SessionID == "" {
		t.Fatal("expected non-empty session_id")
	}
	if result.Status != "pending" && result.Status != "running" {
		t.Fatalf("expected status pending or running, got %q", result.Status)
	}

	// Subscribe to SSE for the task's session and collect events.
	// The mock LLM responds instantly, so the task should complete quickly.
	sseCtx, sseCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer sseCancel()

	sseURL := fmt.Sprintf("%s/api/tasks/%s/events?session_id=%s", ts.Server.URL, result.RunID, result.SessionID)
	events, err := collectSSEEvents(sseCtx, sseURL)
	if err != nil && sseCtx.Err() == nil {
		t.Fatalf("SSE collection error: %v", err)
	}

	// Verify we got the key lifecycle events.
	var hasConnected, hasInputReceived, hasTerminal bool
	for _, evt := range events {
		switch evt.EventType {
		case "connected":
			hasConnected = true
		case "workflow.input.received":
			hasInputReceived = true
			if task, ok := evt.Data["task"].(string); ok {
				if task != "Say hello" {
					t.Errorf("expected task='Say hello', got %q", task)
				}
			}
		case "workflow.result.final":
			hasTerminal = true
		case "workflow.result.cancelled":
			hasTerminal = true
		}
	}

	if !hasConnected {
		t.Error("missing 'connected' SSE event")
	}
	if !hasInputReceived {
		t.Error("missing 'workflow.input.received' SSE event")
	}
	if !hasTerminal {
		t.Error("missing terminal SSE event (workflow.result.final or workflow.result.cancelled)")
	}

	// Verify run_id consistency across events.
	for _, evt := range events {
		if runID, ok := evt.Data["run_id"].(string); ok && runID != "" {
			if runID != result.RunID {
				t.Errorf("event %s has run_id=%s, expected %s", evt.EventType, runID, result.RunID)
			}
		}
	}

	// Verify task is retrievable via GET.
	resp, err := http.Get(fmt.Sprintf("%s/api/tasks/%s", ts.Server.URL, result.RunID))
	if err != nil {
		t.Fatalf("GET task: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET task status: %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Test 2.2: Task Cancellation
// Start task → POST cancel → verify task_cancelled event or graceful completion
// ---------------------------------------------------------------------------
func TestAPI_TaskCancel(t *testing.T) {
	ts := newTestServer(t, "")

	// Create task (mock LLM responds fast, so task may complete before cancel).
	result := createTask(t, ts.Server.URL, "Long running task")

	// Cancel the task immediately.
	cancelURL := fmt.Sprintf("%s/api/tasks/%s/cancel", ts.Server.URL, result.RunID)
	cancelResp, err := http.Post(cancelURL, "application/json", nil)
	if err != nil {
		t.Fatalf("POST cancel: %v", err)
	}
	defer cancelResp.Body.Close()

	// Cancel returns 200 (pending/running) or 409 (already completed by mock LLM).
	if cancelResp.StatusCode != http.StatusOK && cancelResp.StatusCode != http.StatusConflict {
		var body bytes.Buffer
		body.ReadFrom(cancelResp.Body)
		t.Fatalf("cancel returned %d: %s", cancelResp.StatusCode, body.String())
	}

	if cancelResp.StatusCode == http.StatusOK {
		var cancelBody map[string]interface{}
		if err := json.NewDecoder(cancelResp.Body).Decode(&cancelBody); err != nil {
			t.Fatalf("decode cancel response: %v", err)
		}
		if status, ok := cancelBody["status"].(string); !ok || status != "cancelled" {
			t.Errorf("expected cancel status=cancelled, got %q", cancelBody["status"])
		}
		if taskID, ok := cancelBody["task_id"].(string); !ok || taskID != result.RunID {
			t.Errorf("expected task_id=%s, got %q", result.RunID, cancelBody["task_id"])
		}
	}

	// Wait briefly for async updates to settle.
	time.Sleep(500 * time.Millisecond)

	// Verify task record reflects terminal state.
	getResp, err := http.Get(fmt.Sprintf("%s/api/tasks/%s", ts.Server.URL, result.RunID))
	if err != nil {
		t.Fatalf("GET task: %v", err)
	}
	defer getResp.Body.Close()

	var taskStatus map[string]interface{}
	if err := json.NewDecoder(getResp.Body).Decode(&taskStatus); err != nil {
		t.Fatalf("decode task status: %v", err)
	}

	status, _ := taskStatus["status"].(string)
	// Task may be cancelled, completed, or failed depending on timing.
	switch status {
	case "cancelled", "completed", "failed":
		// All acceptable terminal states.
	default:
		t.Errorf("expected terminal status (cancelled/completed/failed), got %q", status)
	}
}

// ---------------------------------------------------------------------------
// Test 2.3: Concurrent Task Isolation
// 2 tasks in parallel → each gets own events, no cross-talk
// ---------------------------------------------------------------------------
func TestAPI_ConcurrentTasks(t *testing.T) {
	ts := newTestServer(t, "")

	// Create two tasks.
	result1 := createTask(t, ts.Server.URL, "Task Alpha")
	result2 := createTask(t, ts.Server.URL, "Task Beta")

	if result1.SessionID == result2.SessionID && result1.RunID == result2.RunID {
		t.Fatal("expected different run IDs for concurrent tasks")
	}

	// Collect SSE events for both tasks concurrently.
	var wg sync.WaitGroup
	var events1, events2 []sseEvent
	var err1, err2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		sseURL := fmt.Sprintf("%s/api/tasks/%s/events?session_id=%s", ts.Server.URL, result1.RunID, result1.SessionID)
		events1, err1 = collectSSEEvents(ctx, sseURL)
	}()
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		sseURL := fmt.Sprintf("%s/api/tasks/%s/events?session_id=%s", ts.Server.URL, result2.RunID, result2.SessionID)
		events2, err2 = collectSSEEvents(ctx, sseURL)
	}()
	wg.Wait()

	if err1 != nil {
		t.Logf("SSE stream 1 error (may be expected): %v", err1)
	}
	if err2 != nil {
		t.Logf("SSE stream 2 error (may be expected): %v", err2)
	}

	// Verify no cross-talk: events for task1 should not contain task2's run_id.
	for _, evt := range events1 {
		if runID, ok := evt.Data["run_id"].(string); ok && runID != "" {
			if runID == result2.RunID {
				t.Errorf("task1 SSE stream received event for task2 (run_id=%s)", runID)
			}
		}
	}
	for _, evt := range events2 {
		if runID, ok := evt.Data["run_id"].(string); ok && runID != "" {
			if runID == result1.RunID {
				t.Errorf("task2 SSE stream received event for task1 (run_id=%s)", runID)
			}
		}
	}

	// Each stream should have at least a connected event.
	hasConnected := func(events []sseEvent) bool {
		for _, evt := range events {
			if evt.EventType == "connected" {
				return true
			}
		}
		return false
	}
	if !hasConnected(events1) {
		t.Error("task1 SSE stream missing connected event")
	}
	if !hasConnected(events2) {
		t.Error("task2 SSE stream missing connected event")
	}

	// Each stream should have its own terminal event.
	hasTerminal := func(events []sseEvent) bool {
		for _, evt := range events {
			if evt.EventType == "workflow.result.final" || evt.EventType == "workflow.result.cancelled" {
				return true
			}
		}
		return false
	}
	if !hasTerminal(events1) {
		t.Error("task1 SSE stream missing terminal event")
	}
	if !hasTerminal(events2) {
		t.Error("task2 SSE stream missing terminal event")
	}
}

// ---------------------------------------------------------------------------
// Test 2.7: Leader API Authentication
// No token → 401, wrong token → 401, correct token → 200
// ---------------------------------------------------------------------------
func TestAPI_LeaderAuth(t *testing.T) {
	const leaderToken = "test-secret-token-42"
	ts := newTestServer(t, leaderToken)

	endpoint := ts.Server.URL + "/api/leader/openapi.json"

	t.Run("no_token_returns_401", func(t *testing.T) {
		resp, err := http.Get(endpoint)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("wrong_token_returns_401", func(t *testing.T) {
		req, _ := http.NewRequest("GET", endpoint, nil)
		req.Header.Set("Authorization", "Bearer wrong-token")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("correct_token_returns_200", func(t *testing.T) {
		req, _ := http.NewRequest("GET", endpoint, nil)
		req.Header.Set("Authorization", "Bearer "+leaderToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("x_api_key_header_returns_200", func(t *testing.T) {
		req, _ := http.NewRequest("GET", endpoint, nil)
		req.Header.Set("X-API-Key", leaderToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}
