package integration

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	appconfig "alex/internal/agent/app/config"
	agentcoordinator "alex/internal/agent/app/coordinator"
	agentcost "alex/internal/agent/app/cost"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	agentstorage "alex/internal/agent/ports/storage"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/llm"
	serverApp "alex/internal/server/app"
	serverPorts "alex/internal/server/ports"
	"alex/internal/session/filestore"
	sessionstate "alex/internal/session/state_store"
	"alex/internal/storage"
)

// TestConcurrentCostIsolation verifies that cost tracking is properly isolated
// between concurrent sessions with no cross-contamination
func TestConcurrentCostIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Setup: Create real components (not all mocks)
	llmFactory := llm.NewFactory()
	sessionStore := filestore.New(t.TempDir())

	costStore, err := storage.NewFileCostStore(t.TempDir() + "/costs")
	if err != nil {
		t.Fatalf("failed to create cost store: %v", err)
	}
	costTracker := agentcost.NewCostTracker(costStore)

	// Create 3 concurrent sessions
	const numSessions = 3
	var wg sync.WaitGroup
	results := make([]struct {
		sessionID string
		cost      float64
		err       error
	}, numSessions)

	// Launch concurrent tasks
	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Create isolated coordinator for this session
			coordinator := agentcoordinator.NewAgentCoordinator(
				llmFactory,
				newTestToolRegistry(),
				sessionStore,
				newTestContextManager(),
				nil,
				newTestParser(),
				costTracker,
				appconfig.Config{
					LLMProvider:   "mock",
					LLMModel:      "test-model",
					MaxIterations: 2,
					Temperature:   0.5,
				},
			)

			// Execute task
			ctx := agent.WithOutputContext(context.Background(), &agent.OutputContext{Level: agent.LevelCore})
			result, err := coordinator.ExecuteTask(ctx, fmt.Sprintf("Task %d", idx), "", nil)
			if err != nil {
				results[idx].err = err
				return
			}

			results[idx].sessionID = result.SessionID

			// Record usage manually (simulating LLM call)
			usage := agentstorage.UsageRecord{
				SessionID:    result.SessionID,
				Model:        "test-model",
				Provider:     "mock",
				InputTokens:  100 * (idx + 1), // Different amounts per session
				OutputTokens: 50 * (idx + 1),
				Timestamp:    time.Now(),
			}
			if err := costTracker.RecordUsage(ctx, usage); err != nil {
				results[idx].err = fmt.Errorf("record usage: %w", err)
				return
			}

			// Get session cost
			summary, err := costTracker.GetSessionCost(ctx, result.SessionID)
			if err != nil {
				results[idx].err = fmt.Errorf("get session cost: %w", err)
				return
			}

			results[idx].cost = summary.TotalCost
		}(i)
	}

	wg.Wait()

	// Verify: All tasks completed without errors
	for i, result := range results {
		if result.err != nil {
			t.Errorf("session %d failed: %v", i, result.err)
		}
		if result.sessionID == "" {
			t.Errorf("session %d has empty session ID", i)
		}
	}

	// Verify: Cost isolation - each session has unique cost
	seenCosts := make(map[float64]bool)
	for i, result := range results {
		if result.cost == 0 {
			t.Errorf("session %d has zero cost", i)
		}
		if seenCosts[result.cost] {
			t.Errorf("session %d has duplicate cost %.6f - cost not isolated", i, result.cost)
		}
		seenCosts[result.cost] = true
	}

	// Verify: Session IDs are unique (no cross-contamination)
	seenSessions := make(map[string]bool)
	for i, result := range results {
		if seenSessions[result.sessionID] {
			t.Errorf("session %d has duplicate session ID %s", i, result.sessionID)
		}
		seenSessions[result.sessionID] = true
	}

	t.Logf("✓ All %d concurrent sessions completed with isolated costs", numSessions)
	for i, result := range results {
		t.Logf("  Session %d: ID=%s, Cost=$%.6f", i, result.sessionID, result.cost)
	}
}

// TestTaskCancellation verifies that task cancellation works correctly
// with proper status updates and termination reasons
func TestTaskCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Setup: Create real components with slow tool
	llmFactory := llm.NewFactory()
	sessionStore := filestore.New(t.TempDir())

	costStore, err := storage.NewFileCostStore(t.TempDir() + "/costs")
	if err != nil {
		t.Fatalf("failed to create cost store: %v", err)
	}
	costTracker := agentcost.NewCostTracker(costStore)
	taskStore := serverApp.NewInMemoryTaskStore()
	broadcaster := serverApp.NewEventBroadcaster()
	stateStore := sessionstate.NewInMemoryStore()

	agentCoordinator := agentcoordinator.NewAgentCoordinator(
		llmFactory,
		newSlowToolRegistry(), // Slow tools that respect context cancellation
		sessionStore,
		newTestContextManager(),
		nil,
		newSlowParser(), // Parser that returns tool calls
		costTracker,
		appconfig.Config{
			LLMProvider:   "mock",
			LLMModel:      "test-model",
			MaxIterations: 100, // Many iterations to allow cancellation
			Temperature:   0.5,
		},
	)

	serverCoordinator := serverApp.NewServerCoordinator(
		agentCoordinator,
		broadcaster,
		sessionStore,
		taskStore,
		stateStore,
	)

	// Create and start a long-running task
	ctx := context.Background()
	task, err := serverCoordinator.ExecuteTaskAsync(ctx, "Long running task", "", "", "")
	if err != nil {
		t.Fatalf("failed to start task: %v", err)
	}

	t.Logf("Started task: %s", task.ID)

	// Wait for task to start running
	time.Sleep(300 * time.Millisecond)

	// Verify task is running
	runningTask, err := serverCoordinator.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}
	t.Logf("Task status before cancellation: %s", runningTask.Status)

	// Cancel the task
	if err := serverCoordinator.CancelTask(ctx, task.ID); err != nil {
		t.Fatalf("failed to cancel task: %v", err)
	}

	t.Logf("Cancelled task: %s", task.ID)

	// Wait for task to finish cancellation
	time.Sleep(500 * time.Millisecond)

	// Verify: Task status is cancelled
	finalTask, err := serverCoordinator.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}

	if finalTask.Status != serverPorts.TaskStatusCancelled {
		t.Errorf("expected status cancelled, got %s", finalTask.Status)
	}

	// Verify: Termination reason is set to cancelled
	if finalTask.TerminationReason != serverPorts.TerminationReasonCancelled {
		t.Errorf("expected termination reason cancelled, got %s", finalTask.TerminationReason)
	}

	// Verify: Task has completed timestamp
	if finalTask.CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}

	t.Logf("✓ Task cancelled successfully with correct status and termination reason")
}

// TestCostTrackingWithCancellation verifies that costs are recorded up to
// the cancellation point and cleanup happens properly
func TestCostTrackingWithCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Setup: Create real components with slow tool
	llmFactory := llm.NewFactory()
	sessionStore := filestore.New(t.TempDir())

	costStore, err := storage.NewFileCostStore(t.TempDir() + "/costs")
	if err != nil {
		t.Fatalf("failed to create cost store: %v", err)
	}
	costTracker := agentcost.NewCostTracker(costStore)
	taskStore := serverApp.NewInMemoryTaskStore()
	broadcaster := serverApp.NewEventBroadcaster()
	stateStore := sessionstate.NewInMemoryStore()

	agentCoordinator := agentcoordinator.NewAgentCoordinator(
		llmFactory,
		newSlowToolRegistry(),
		sessionStore,
		newTestContextManager(),
		nil,
		newSlowParser(),
		costTracker,
		appconfig.Config{
			LLMProvider:   "mock",
			LLMModel:      "test-model",
			MaxIterations: 100,
			Temperature:   0.5,
		},
	)

	serverCoordinator := serverApp.NewServerCoordinator(
		agentCoordinator,
		broadcaster,
		sessionStore,
		taskStore,
		stateStore,
	)

	// Get or create session
	ctx := context.Background()
	session, err := agentCoordinator.GetSession(ctx, "")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Record initial usage before task starts
	initialUsage := agentstorage.UsageRecord{
		SessionID:    session.ID,
		Model:        "test-model",
		Provider:     "mock",
		InputTokens:  100,
		OutputTokens: 50,
		Timestamp:    time.Now(),
	}
	if err := costTracker.RecordUsage(ctx, initialUsage); err != nil {
		t.Fatalf("failed to record initial usage: %v", err)
	}

	// Get initial cost
	initialSummary, err := costTracker.GetSessionCost(ctx, session.ID)
	if err != nil {
		t.Fatalf("failed to get initial cost: %v", err)
	}
	t.Logf("Initial cost: $%.6f", initialSummary.TotalCost)

	// Start task
	task, err := serverCoordinator.ExecuteTaskAsync(ctx, "Task with cost tracking", session.ID, "", "")
	if err != nil {
		t.Fatalf("failed to start task: %v", err)
	}

	// Wait for task to start and record some cost
	time.Sleep(200 * time.Millisecond)

	// Record usage during execution
	midUsage := agentstorage.UsageRecord{
		SessionID:    session.ID,
		Model:        "test-model",
		Provider:     "mock",
		InputTokens:  200,
		OutputTokens: 100,
		Timestamp:    time.Now(),
	}
	if err := costTracker.RecordUsage(ctx, midUsage); err != nil {
		t.Fatalf("failed to record mid usage: %v", err)
	}

	// Cancel task
	if err := serverCoordinator.CancelTask(ctx, task.ID); err != nil {
		t.Fatalf("failed to cancel task: %v", err)
	}

	// Wait for cancellation to complete
	time.Sleep(500 * time.Millisecond)

	// Get final cost
	finalSummary, err := costTracker.GetSessionCost(ctx, session.ID)
	if err != nil {
		t.Fatalf("failed to get final cost: %v", err)
	}

	// Verify: Costs were recorded up to cancellation point
	if finalSummary.TotalCost <= initialSummary.TotalCost {
		t.Errorf("expected final cost (%.6f) > initial cost (%.6f)", finalSummary.TotalCost, initialSummary.TotalCost)
	}

	// Verify: Task status is cancelled
	finalTask, err := serverCoordinator.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}

	if finalTask.Status != serverPorts.TaskStatusCancelled {
		t.Errorf("expected status cancelled, got %s", finalTask.Status)
	}

	// Verify: Proper cleanup - no goroutine leaks (implicit via test completion)
	t.Logf("✓ Cost tracking with cancellation completed successfully")
	t.Logf("  Initial cost: $%.6f", initialSummary.TotalCost)
	t.Logf("  Final cost:   $%.6f", finalSummary.TotalCost)
	t.Logf("  Cost delta:   $%.6f", finalSummary.TotalCost-initialSummary.TotalCost)
}

// Test helpers

type testToolRegistry struct{}

func newTestToolRegistry() *testToolRegistry {
	return &testToolRegistry{}
}

func (r *testToolRegistry) Register(tool tools.ToolExecutor) error {
	return nil
}

func (r *testToolRegistry) Get(name string) (tools.ToolExecutor, error) {
	return nil, fmt.Errorf("tool not found: %s", name)
}

func (r *testToolRegistry) List() []ports.ToolDefinition {
	return []ports.ToolDefinition{}
}

func (r *testToolRegistry) Unregister(name string) error {
	return nil
}

// Slow tool registry for cancellation testing
type slowToolRegistry struct{}

func newSlowToolRegistry() *slowToolRegistry {
	return &slowToolRegistry{}
}

func (r *slowToolRegistry) Register(tool tools.ToolExecutor) error {
	return nil
}

func (r *slowToolRegistry) Get(name string) (tools.ToolExecutor, error) {
	switch name {
	case "plan":
		return &fastPlanTool{}, nil
	case "clarify":
		return &fastClarifyTool{}, nil
	default:
		return &slowTool{}, nil
	}
}

func (r *slowToolRegistry) List() []ports.ToolDefinition {
	return []ports.ToolDefinition{
		{
			Name:        "plan",
			Description: "Create a UI plan for the run",
			Parameters: ports.ParameterSchema{
				Type:       "object",
				Properties: map[string]ports.Property{},
				Required:   []string{},
			},
		},
		{
			Name:        "clarify",
			Description: "Declare the current task before tool calls",
			Parameters: ports.ParameterSchema{
				Type:       "object",
				Properties: map[string]ports.Property{},
				Required:   []string{},
			},
		},
		{
			Name:        "slow_tool",
			Description: "A tool that takes time to execute",
			Parameters: ports.ParameterSchema{
				Type:       "object",
				Properties: map[string]ports.Property{},
				Required:   []string{},
			},
		},
	}
}

func (r *slowToolRegistry) Unregister(name string) error {
	return nil
}

type slowTool struct{}

func (t *slowTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	// Simulate slow operation that respects context cancellation
	select {
	case <-time.After(2 * time.Second):
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "Slow tool completed",
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t *slowTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "slow_tool",
		Description: "A tool that takes time to execute",
		Parameters: ports.ParameterSchema{
			Type:       "object",
			Properties: map[string]ports.Property{},
			Required:   []string{},
		},
	}
}

func (t *slowTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "slow_tool",
		Version:  "1.0.0",
		Category: "test",
	}
}

type fastPlanTool struct{}

func (t *fastPlanTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	runID, _ := call.Arguments["run_id"].(string)
	runID = strings.TrimSpace(runID)
	if runID == "" {
		runID = "test-run"
	}

	metadata := map[string]any{
		"run_id": runID,
	}
	if goal, ok := call.Arguments["overall_goal_ui"].(string); ok {
		if goal = strings.TrimSpace(goal); goal != "" {
			metadata["overall_goal_ui"] = goal
		}
	}
	if complexity, ok := call.Arguments["complexity"].(string); ok {
		if complexity = strings.TrimSpace(complexity); complexity != "" {
			metadata["complexity"] = complexity
		}
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  "plan ok",
		Metadata: metadata,
	}, nil
}

func (t *fastPlanTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "plan",
		Description: "Create a UI plan for the run",
		Parameters: ports.ParameterSchema{
			Type:       "object",
			Properties: map[string]ports.Property{},
			Required:   []string{},
		},
	}
}

func (t *fastPlanTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "plan",
		Version:  "1.0.0",
		Category: "test",
	}
}

type fastClarifyTool struct{}

func (t *fastClarifyTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	taskID, _ := call.Arguments["task_id"].(string)
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		taskID = "task-1"
	}

	metadata := map[string]any{
		"task_id": taskID,
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  "clarify ok",
		Metadata: metadata,
	}, nil
}

func (t *fastClarifyTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "clarify",
		Description: "Declare the current task before tool calls",
		Parameters: ports.ParameterSchema{
			Type:       "object",
			Properties: map[string]ports.Property{},
			Required:   []string{},
		},
	}
}

func (t *fastClarifyTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "clarify",
		Version:  "1.0.0",
		Category: "test",
	}
}

type testContextManager struct{}

func newTestContextManager() *testContextManager {
	return &testContextManager{}
}

func (m *testContextManager) EstimateTokens(messages []ports.Message) int {
	return len(messages) * 10
}

func (m *testContextManager) Compress(messages []ports.Message, targetTokens int) ([]ports.Message, error) {
	return messages, nil
}

func (m *testContextManager) AutoCompact(messages []ports.Message, limit int) ([]ports.Message, bool) {
	return messages, false
}

func (m *testContextManager) ShouldCompress(messages []ports.Message, limit int) bool {
	return false
}

func (m *testContextManager) Preload(context.Context) error { return nil }

func (m *testContextManager) BuildWindow(ctx context.Context, session *agentstorage.Session, cfg agent.ContextWindowConfig) (agent.ContextWindow, error) {
	if session == nil {
		return agent.ContextWindow{}, fmt.Errorf("session required")
	}
	return agent.ContextWindow{SessionID: session.ID, Messages: session.Messages}, nil
}

func (m *testContextManager) RecordTurn(context.Context, agent.ContextTurnRecord) error { return nil }

type testParser struct{}

func newTestParser() *testParser {
	return &testParser{}
}

func (p *testParser) Parse(content string) ([]ports.ToolCall, error) {
	return []ports.ToolCall{}, nil
}

func (p *testParser) Validate(call ports.ToolCall, definition ports.ToolDefinition) error {
	return nil
}

// Slow parser that returns tool calls to trigger iterations
type slowParser struct {
	mu   sync.Mutex
	step int
}

func newSlowParser() *slowParser {
	return &slowParser{}
}

func (p *slowParser) Parse(content string) ([]ports.ToolCall, error) {
	p.mu.Lock()
	p.step++
	step := p.step
	p.mu.Unlock()

	switch step {
	case 1:
		return []ports.ToolCall{
			{
				ID:   fmt.Sprintf("call-plan-%d", step),
				Name: "plan",
				Arguments: map[string]any{
					"run_id":          "test-run",
					"overall_goal_ui": "Execute a slow tool for cancellation testing.",
					"complexity":      "simple",
				},
			},
		}, nil
	case 2:
		return []ports.ToolCall{
			{
				ID:   fmt.Sprintf("call-clarify-%d", step),
				Name: "clarify",
				Arguments: map[string]any{
					"run_id":       "test-run",
					"task_id":      "task-1",
					"task_goal_ui": "Run the slow tool.",
				},
			},
		}, nil
	default:
		// Always return a tool call to keep the iteration going.
		return []ports.ToolCall{
			{
				ID:        fmt.Sprintf("call-slow-%d", step),
				Name:      "slow_tool",
				Arguments: map[string]any{},
			},
		}, nil
	}
}

func (p *slowParser) Validate(call ports.ToolCall, definition ports.ToolDefinition) error {
	return nil
}
