package workflow

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// WorkflowPhase represents the aggregate state for a workflow.
type WorkflowPhase string

const (
	// PhasePending indicates no nodes have started.
	PhasePending WorkflowPhase = "pending"
	// PhaseRunning indicates at least one node is executing and none have failed.
	PhaseRunning WorkflowPhase = "running"
	// PhaseSucceeded indicates all nodes completed successfully.
	PhaseSucceeded WorkflowPhase = "succeeded"
	// PhaseFailed indicates at least one node failed.
	PhaseFailed WorkflowPhase = "failed"
)

// Workflow tracks the lifecycle of nodes within a workflow instance.
type Workflow struct {
	mu        sync.RWMutex
	id        string
	nodes     map[string]*Node
	order     []string
	logger    *slog.Logger
	listeners []Listener
}

// WorkflowSnapshot captures a consistent view of the workflow for debugging or reporting.
type WorkflowSnapshot struct {
	ID          string           `json:"id"`
	Phase       WorkflowPhase    `json:"phase"`
	Order       []string         `json:"order"`
	Nodes       []NodeSnapshot   `json:"nodes"`
	StartedAt   time.Time        `json:"started_at,omitempty"`
	CompletedAt time.Time        `json:"completed_at,omitempty"`
	Duration    time.Duration    `json:"duration"`
	Summary     map[string]int64 `json:"summary"`
}

// EventType enumerates workflow lifecycle signals emitted to listeners.
type EventType string

const (
	// EventNodeAdded indicates a node was registered with the workflow.
	EventNodeAdded EventType = "node_added"
	// EventNodeStarted indicates a node entered the running state.
	EventNodeStarted EventType = "node_started"
	// EventNodeSucceeded indicates a node finished successfully.
	EventNodeSucceeded EventType = "node_succeeded"
	// EventNodeFailed indicates a node finished with an error.
	EventNodeFailed EventType = "node_failed"
	// EventWorkflowUpdated emits the full snapshot after any transition.
	EventWorkflowUpdated EventType = "workflow_updated"
)

// Event represents a workflow lifecycle notification.
type Event struct {
	Type      EventType         `json:"type"`
	Timestamp time.Time         `json:"timestamp"`
	Workflow  string            `json:"workflow"`
	Phase     WorkflowPhase     `json:"phase,omitempty"`
	Node      *NodeSnapshot     `json:"node,omitempty"`
	Snapshot  *WorkflowSnapshot `json:"snapshot,omitempty"`
}

// Listener receives workflow lifecycle events.
type Listener interface {
	OnWorkflowEvent(Event)
}

// New creates an empty workflow with the provided identifier.
func New(id string, logger *slog.Logger) *Workflow {
	return &Workflow{
		id:     id,
		nodes:  make(map[string]*Node),
		logger: logger,
	}
}

// AddListener attaches a workflow listener for lifecycle events.
func (w *Workflow) AddListener(listener Listener) {
	if listener == nil {
		return
	}
	w.mu.Lock()
	w.listeners = append(w.listeners, listener)
	w.mu.Unlock()
}

// AddNode registers a node with the workflow, enforcing unique identifiers.
func (w *Workflow) AddNode(node *Node) error {
	w.mu.Lock()
	if node == nil {
		w.mu.Unlock()
		return fmt.Errorf("nil node")
	}
	if node.ID() == "" {
		w.mu.Unlock()
		return fmt.Errorf("node id is required")
	}

	if _, exists := w.nodes[node.ID()]; exists {
		w.mu.Unlock()
		return fmt.Errorf("node id %q already exists", node.ID())
	}
	w.nodes[node.ID()] = node
	w.order = append(w.order, node.ID())
	w.mu.Unlock()

	snapshot := node.Snapshot()
	workflowSnapshot := w.Snapshot()
	w.emit(Event{
		Type:      EventNodeAdded,
		Workflow:  w.id,
		Phase:     workflowSnapshot.Phase,
		Node:      &snapshot,
		Snapshot:  &workflowSnapshot,
		Timestamp: time.Now(),
	})
	return nil
}

// Node returns a registered node by id.
func (w *Workflow) Node(id string) (*Node, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	node, ok := w.nodes[id]
	return node, ok
}

// StartNode transitions the given node into the running state and emits lifecycle events.
func (w *Workflow) StartNode(id string) (NodeSnapshot, WorkflowSnapshot, error) {
	return w.transitionNode(id, func(node *Node) (NodeSnapshot, error) {
		return node.Start()
	}, EventNodeStarted)
}

// CompleteNodeSuccess transitions the given node into the succeeded state and emits lifecycle events.
func (w *Workflow) CompleteNodeSuccess(id string, output any) (NodeSnapshot, WorkflowSnapshot, error) {
	return w.transitionNode(id, func(node *Node) (NodeSnapshot, error) {
		return node.CompleteSuccess(output)
	}, EventNodeSucceeded)
}

// CompleteNodeFailure transitions the given node into the failed state and emits lifecycle events.
func (w *Workflow) CompleteNodeFailure(id string, err error) (NodeSnapshot, WorkflowSnapshot, error) {
	return w.transitionNode(id, func(node *Node) (NodeSnapshot, error) {
		return node.CompleteFailure(err)
	}, EventNodeFailed)
}

// Snapshot returns a deterministic snapshot of the workflow and all registered nodes.
func (w *Workflow) Snapshot() WorkflowSnapshot {
	w.mu.RLock()
	orderedIDs := make([]string, 0, len(w.order))
	for _, id := range w.order {
		if _, exists := w.nodes[id]; exists {
			orderedIDs = append(orderedIDs, id)
		}
	}

	snapshots := make([]NodeSnapshot, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		snapshots = append(snapshots, w.nodes[id].Snapshot())
	}
	w.mu.RUnlock()

	phase, startedAt, completedAt := evaluatePhase(snapshots)
	summary := summarize(snapshots)
	duration := time.Duration(0)
	if !startedAt.IsZero() {
		end := completedAt
		if end.IsZero() {
			end = time.Now()
		}
		duration = end.Sub(startedAt)
	}

	snapshot := WorkflowSnapshot{
		ID:          w.id,
		Phase:       phase,
		Order:       orderedIDs,
		Nodes:       snapshots,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		Duration:    duration,
		Summary:     summary,
	}

	if w.logger != nil {
		w.logger.Debug("workflow snapshot", slog.String("workflow", w.id), slog.String("phase", string(phase)), slog.Int("nodes", len(snapshots)))
	}

	return snapshot
}

func (w *Workflow) transitionNode(id string, transition func(*Node) (NodeSnapshot, error), eventType EventType) (NodeSnapshot, WorkflowSnapshot, error) {
	w.mu.RLock()
	node, ok := w.nodes[id]
	w.mu.RUnlock()
	if !ok {
		return NodeSnapshot{}, WorkflowSnapshot{}, fmt.Errorf("node %q not found", id)
	}

	nodeSnapshot, err := transition(node)
	if err != nil {
		return NodeSnapshot{}, WorkflowSnapshot{}, err
	}

	workflowSnapshot := w.Snapshot()

	ts := time.Now()
	w.emit(Event{Type: eventType, Workflow: w.id, Phase: workflowSnapshot.Phase, Node: &nodeSnapshot, Snapshot: &workflowSnapshot, Timestamp: ts})
	w.emit(Event{Type: EventWorkflowUpdated, Workflow: w.id, Phase: workflowSnapshot.Phase, Snapshot: &workflowSnapshot, Timestamp: ts})

	return nodeSnapshot, workflowSnapshot, nil
}

func (w *Workflow) emit(event Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	w.mu.RLock()
	listeners := append([]Listener(nil), w.listeners...)
	w.mu.RUnlock()

	for _, listener := range listeners {
		listener.OnWorkflowEvent(event)
	}
}

func evaluatePhase(nodes []NodeSnapshot) (WorkflowPhase, time.Time, time.Time) {
	if len(nodes) == 0 {
		return PhasePending, time.Time{}, time.Time{}
	}

	var startedAt time.Time
	var completedAt time.Time
	allSucceeded := true
	hasRunning := false
	hasProgress := false

	for _, node := range nodes {
		if node.Status == NodeStatusFailed {
			if node.CompletedAt.After(completedAt) {
				completedAt = node.CompletedAt
			}
			if startedAt.IsZero() || node.StartedAt.Before(startedAt) {
				startedAt = node.StartedAt
			}
			return PhaseFailed, startedAt, completedAt
		}
		if node.Status == NodeStatusRunning {
			hasRunning = true
		}
		if node.Status == NodeStatusSucceeded || node.Status == NodeStatusRunning {
			hasProgress = true
		}
		if node.Status != NodeStatusSucceeded {
			allSucceeded = false
		}

		if !node.StartedAt.IsZero() && (startedAt.IsZero() || node.StartedAt.Before(startedAt)) {
			startedAt = node.StartedAt
		}
		if !node.CompletedAt.IsZero() && node.CompletedAt.After(completedAt) {
			completedAt = node.CompletedAt
		}
	}

	switch {
	case allSucceeded:
		return PhaseSucceeded, startedAt, completedAt
	case hasRunning || hasProgress:
		return PhaseRunning, startedAt, time.Time{}
	default:
		return PhasePending, time.Time{}, time.Time{}
	}
}

func summarize(nodes []NodeSnapshot) map[string]int64 {
	summary := map[string]int64{
		string(NodeStatusPending):   0,
		string(NodeStatusRunning):   0,
		string(NodeStatusSucceeded): 0,
		string(NodeStatusFailed):    0,
	}
	for _, node := range nodes {
		summary[string(node.Status)]++
	}
	return summary
}
