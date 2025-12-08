package workflow

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// NodeStatus represents the lifecycle stage for a workflow node.
type NodeStatus string

const (
	// NodeStatusPending indicates the node has been registered but not started.
	NodeStatusPending NodeStatus = "pending"
	// NodeStatusRunning indicates the node is currently executing.
	NodeStatusRunning NodeStatus = "running"
	// NodeStatusSucceeded indicates the node finished successfully.
	NodeStatusSucceeded NodeStatus = "succeeded"
	// NodeStatusFailed indicates the node finished with an error.
	NodeStatusFailed NodeStatus = "failed"
)

// Node holds execution metadata for a unit of work inside a workflow.
type Node struct {
	mu          sync.RWMutex
	id          string
	input       any
	output      any
	err         error
	status      NodeStatus
	startedAt   time.Time
	completedAt time.Time
	logger      *slog.Logger
}

// NodeSnapshot captures a consistent view of a node for debugging or observability.
type NodeSnapshot struct {
	ID          string        `json:"id"`
	Status      NodeStatus    `json:"status"`
	Input       any           `json:"input,omitempty"`
	Output      any           `json:"output,omitempty"`
	Error       string        `json:"error,omitempty"`
	StartedAt   time.Time     `json:"started_at,omitempty"`
	CompletedAt time.Time     `json:"completed_at,omitempty"`
	Duration    time.Duration `json:"duration"`
}

// NewNode creates a new workflow node with the provided identifier and input payload.
func NewNode(id string, input any, logger *slog.Logger) *Node {
	return &Node{
		id:     id,
		input:  input,
		status: NodeStatusPending,
		logger: logger,
	}
}

// ID returns the node identifier.
func (n *Node) ID() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.id
}

// Status returns the current node status.
func (n *Node) Status() NodeStatus {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.status
}

// Start marks the node as running and records the start time.
func (n *Node) Start() (NodeSnapshot, error) {
	return n.transition(NodeStatusRunning, nil, nil)
}

// CompleteSuccess marks the node as succeeded and stores the output payload.
func (n *Node) CompleteSuccess(output any) (NodeSnapshot, error) {
	return n.transition(NodeStatusSucceeded, output, nil)
}

// CompleteFailure marks the node as failed and attaches the given error.
func (n *Node) CompleteFailure(err error) (NodeSnapshot, error) {
	if err == nil {
		err = fmt.Errorf("unknown node failure")
	}
	return n.transition(NodeStatusFailed, nil, err)
}

// Snapshot returns an immutable snapshot of the node for logging or inspection.
func (n *Node) Snapshot() NodeSnapshot {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.snapshotLocked()
}

func (n *Node) transition(target NodeStatus, output any, err error) (NodeSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	now := time.Now()
	if !n.canTransition(target) {
		return NodeSnapshot{}, fmt.Errorf("cannot transition node %q from %s to %s", n.id, n.status, target)
	}

	if target == NodeStatusRunning {
		n.startedAt = now
		n.completedAt = time.Time{}
		n.output = nil
		n.err = nil
	}

	if target == NodeStatusSucceeded || target == NodeStatusFailed {
		if n.startedAt.IsZero() {
			n.startedAt = now
		}
		n.completedAt = now
		n.output = output
		n.err = err
	}

	n.status = target
	snapshot := n.snapshotLocked()

	if n.logger != nil {
		n.logger.Info("node transition", slog.String("node", n.id), slog.String("status", string(target)), slog.String("error", snapshot.Error))
	}

	return snapshot, nil
}

func (n *Node) snapshotLocked() NodeSnapshot {
	snapshot := NodeSnapshot{
		ID:          n.id,
		Status:      n.status,
		Input:       n.input,
		Output:      n.output,
		StartedAt:   n.startedAt,
		CompletedAt: n.completedAt,
	}
	if n.err != nil {
		snapshot.Error = n.err.Error()
	}
	if !snapshot.StartedAt.IsZero() {
		end := snapshot.CompletedAt
		if end.IsZero() {
			end = time.Now()
		}
		snapshot.Duration = end.Sub(snapshot.StartedAt)
	}
	return snapshot
}

func (n *Node) canTransition(target NodeStatus) bool {
	switch n.status {
	case NodeStatusPending:
		return target == NodeStatusRunning
	case NodeStatusRunning:
		return target == NodeStatusSucceeded || target == NodeStatusFailed
	default:
		return false
	}
}
