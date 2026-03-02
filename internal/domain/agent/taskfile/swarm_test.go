package taskfile

import (
	"context"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
)

// orderTrackingDispatcher records dispatch order and supports Collect blocking.
type orderTrackingDispatcher struct {
	mu         sync.Mutex
	dispatched []string
	results    map[string]agent.BackgroundTaskResult
}

func newOrderTracker() *orderTrackingDispatcher {
	return &orderTrackingDispatcher{
		results: make(map[string]agent.BackgroundTaskResult),
	}
}

func (d *orderTrackingDispatcher) Dispatch(_ context.Context, req agent.BackgroundDispatchRequest) error {
	d.mu.Lock()
	d.dispatched = append(d.dispatched, req.TaskID)
	d.results[req.TaskID] = agent.BackgroundTaskResult{
		ID:     req.TaskID,
		Status: agent.BackgroundTaskStatusCompleted,
		Answer: "done",
	}
	d.mu.Unlock()
	return nil
}

func (d *orderTrackingDispatcher) Status(ids []string) []agent.BackgroundTaskSummary {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []agent.BackgroundTaskSummary
	for _, id := range ids {
		out = append(out, agent.BackgroundTaskSummary{
			ID:     id,
			Status: agent.BackgroundTaskStatusCompleted,
		})
	}
	return out
}

func (d *orderTrackingDispatcher) Collect(ids []string, _ bool, _ time.Duration) []agent.BackgroundTaskResult {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []agent.BackgroundTaskResult
	for _, id := range ids {
		if r, ok := d.results[id]; ok {
			out = append(out, r)
		}
	}
	return out
}

func (d *orderTrackingDispatcher) dispatchedIDs() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	cp := make([]string, len(d.dispatched))
	copy(cp, d.dispatched)
	return cp
}

func TestSwarmScheduler_StageOrdering(t *testing.T) {
	// Three layers: [a,b] → [c,d] → [e]
	tf := &TaskFile{
		Version: "1",
		PlanID:  "swarm-stage-test",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "do A"},
			{ID: "b", Prompt: "do B"},
			{ID: "c", Prompt: "do C", DependsOn: []string{"a"}},
			{ID: "d", Prompt: "do D", DependsOn: []string{"b"}},
			{ID: "e", Prompt: "do E", DependsOn: []string{"c", "d"}},
		},
	}

	tracker := newOrderTracker()
	sched := NewSwarmScheduler(tracker, DefaultSwarmConfig())
	statusPath := filepath.Join(t.TempDir(), "swarm.status.yaml")

	result, err := sched.ExecuteSwarm(context.Background(), tf, "cause-1", statusPath)
	if err != nil {
		t.Fatalf("ExecuteSwarm: %v", err)
	}

	if len(result.TaskIDs) != 5 {
		t.Fatalf("expected 5 task IDs, got %d", len(result.TaskIDs))
	}

	// Verify stage ordering: a,b dispatched before c,d, which are before e.
	dispatched := tracker.dispatchedIDs()
	indexOf := make(map[string]int, len(dispatched))
	for i, id := range dispatched {
		indexOf[id] = i
	}

	// Layer 0 tasks (a,b) must appear before layer 1 tasks (c,d)
	for _, l0 := range []string{"a", "b"} {
		for _, l1 := range []string{"c", "d"} {
			if indexOf[l0] >= indexOf[l1] {
				t.Errorf("%s (layer 0) should be dispatched before %s (layer 1)", l0, l1)
			}
		}
	}

	// Layer 1 tasks (c,d) must appear before layer 2 task (e)
	for _, l1 := range []string{"c", "d"} {
		if indexOf[l1] >= indexOf["e"] {
			t.Errorf("%s (layer 1) should be dispatched before e (layer 2)", l1)
		}
	}
}

func TestSwarmScheduler_FlatDAGAllParallel(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		PlanID:  "swarm-flat-test",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "A"},
			{ID: "b", Prompt: "B"},
			{ID: "c", Prompt: "C"},
			{ID: "d", Prompt: "D"},
			{ID: "e", Prompt: "E"},
		},
	}

	tracker := newOrderTracker()
	sched := NewSwarmScheduler(tracker, DefaultSwarmConfig())
	statusPath := filepath.Join(t.TempDir(), "flat.status.yaml")

	result, err := sched.ExecuteSwarm(context.Background(), tf, "cause-1", statusPath)
	if err != nil {
		t.Fatalf("ExecuteSwarm: %v", err)
	}
	if len(result.TaskIDs) != 5 {
		t.Fatalf("expected 5 task IDs, got %d", len(result.TaskIDs))
	}

	// All tasks should be in a single layer, so all dispatched.
	dispatched := tracker.dispatchedIDs()
	sort.Strings(dispatched)
	expected := []string{"a", "b", "c", "d", "e"}
	for i, id := range dispatched {
		if id != expected[i] {
			t.Errorf("dispatched[%d] = %s, want %s", i, id, expected[i])
		}
	}
}

func TestSwarmScheduler_AdaptiveConcurrency_ScaleUp(t *testing.T) {
	cfg := SwarmConfig{
		InitialConcurrency: 2,
		MaxConcurrency:     10,
		ScaleUpThreshold:   0.9,
		ScaleDownThreshold: 0.7,
		ScaleStep:          3,
		StageTimeout:       5 * time.Second,
	}

	tracker := newOrderTracker()
	sched := NewSwarmScheduler(tracker, cfg)

	if sched.current != 2 {
		t.Fatalf("initial concurrency should be 2, got %d", sched.current)
	}

	// Simulate a successful stage: 100% success rate → scale up
	results := []agent.BackgroundTaskResult{
		{ID: "a", Status: agent.BackgroundTaskStatusCompleted},
		{ID: "b", Status: agent.BackgroundTaskStatusCompleted},
	}
	sched.adjustConcurrency(results)

	if sched.current != 5 {
		t.Errorf("after scale-up, concurrency should be 5, got %d", sched.current)
	}
}

func TestSwarmScheduler_AdaptiveConcurrency_ScaleDown(t *testing.T) {
	cfg := SwarmConfig{
		InitialConcurrency: 6,
		MaxConcurrency:     10,
		ScaleUpThreshold:   0.9,
		ScaleDownThreshold: 0.7,
		ScaleStep:          2,
		StageTimeout:       5 * time.Second,
	}

	tracker := newOrderTracker()
	sched := NewSwarmScheduler(tracker, cfg)

	// 50% failure rate → scale down
	results := []agent.BackgroundTaskResult{
		{ID: "a", Status: agent.BackgroundTaskStatusCompleted},
		{ID: "b", Status: agent.BackgroundTaskStatusFailed},
		{ID: "c", Status: agent.BackgroundTaskStatusCompleted},
		{ID: "d", Status: agent.BackgroundTaskStatusFailed},
	}
	sched.adjustConcurrency(results)

	if sched.current != 4 {
		t.Errorf("after scale-down, concurrency should be 4, got %d", sched.current)
	}
}

func TestSwarmScheduler_ConcurrencyBounds(t *testing.T) {
	cfg := SwarmConfig{
		InitialConcurrency: 49,
		MaxConcurrency:     50,
		ScaleUpThreshold:   0.9,
		ScaleDownThreshold: 0.7,
		ScaleStep:          5,
		StageTimeout:       5 * time.Second,
	}

	tracker := newOrderTracker()
	sched := NewSwarmScheduler(tracker, cfg)

	// Scale up should cap at max
	results := []agent.BackgroundTaskResult{
		{ID: "a", Status: agent.BackgroundTaskStatusCompleted},
	}
	sched.adjustConcurrency(results)
	if sched.current != 50 {
		t.Errorf("concurrency should cap at 50, got %d", sched.current)
	}

	// Scale down should not go below 1
	sched.current = 2
	failResults := []agent.BackgroundTaskResult{
		{ID: "a", Status: agent.BackgroundTaskStatusFailed},
		{ID: "b", Status: agent.BackgroundTaskStatusFailed},
	}
	sched.adjustConcurrency(failResults)
	if sched.current != 1 {
		t.Errorf("concurrency floor should be 1, got %d", sched.current)
	}
}

func TestSwarmScheduler_ValidationError(t *testing.T) {
	tf := &TaskFile{Version: "1"} // no tasks

	tracker := newOrderTracker()
	sched := NewSwarmScheduler(tracker, DefaultSwarmConfig())

	_, err := sched.ExecuteSwarm(context.Background(), tf, "cause-1", "/tmp/test.status.yaml")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSwarmScheduler_DepsCleared(t *testing.T) {
	// Verify that dispatched requests have DependsOn cleared
	tf := &TaskFile{
		Version: "1",
		PlanID:  "deps-clear-test",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "A"},
			{ID: "b", Prompt: "B", DependsOn: []string{"a"}},
		},
	}

	var mu sync.Mutex
	var dispatched []agent.BackgroundDispatchRequest
	mock := &mockDispatcher{
		collectFn: func(ids []string, wait bool, timeout time.Duration) []agent.BackgroundTaskResult {
			var out []agent.BackgroundTaskResult
			for _, id := range ids {
				out = append(out, agent.BackgroundTaskResult{
					ID:     id,
					Status: agent.BackgroundTaskStatusCompleted,
				})
			}
			return out
		},
	}
	// Override Dispatch to capture requests
	origDispatch := mock.Dispatch
	_ = origDispatch
	captureDispatcher := &capturingDispatcher{
		inner:      mock,
		mu:         &mu,
		dispatched: &dispatched,
	}

	sched := NewSwarmScheduler(captureDispatcher, DefaultSwarmConfig())
	statusPath := filepath.Join(t.TempDir(), "deps.status.yaml")

	_, err := sched.ExecuteSwarm(context.Background(), tf, "cause-1", statusPath)
	if err != nil {
		t.Fatalf("ExecuteSwarm: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, req := range dispatched {
		if len(req.DependsOn) > 0 {
			t.Errorf("task %q should have DependsOn cleared, got %v", req.TaskID, req.DependsOn)
		}
	}
}

// capturingDispatcher wraps a dispatcher to capture dispatched requests.
type capturingDispatcher struct {
	inner      agent.BackgroundTaskDispatcher
	mu         *sync.Mutex
	dispatched *[]agent.BackgroundDispatchRequest
}

func (c *capturingDispatcher) Dispatch(ctx context.Context, req agent.BackgroundDispatchRequest) error {
	c.mu.Lock()
	*c.dispatched = append(*c.dispatched, req)
	c.mu.Unlock()
	return c.inner.Dispatch(ctx, req)
}

func (c *capturingDispatcher) Status(ids []string) []agent.BackgroundTaskSummary {
	return c.inner.Status(ids)
}

func (c *capturingDispatcher) Collect(ids []string, wait bool, timeout time.Duration) []agent.BackgroundTaskResult {
	return c.inner.Collect(ids, wait, timeout)
}
