package app

import (
	"context"
	"sync"
	"time"

	agentcoordinator "alex/internal/agent/app/coordinator"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	storage "alex/internal/agent/ports/storage"
	"alex/internal/analytics"
	"alex/internal/analytics/journal"
	"alex/internal/logging"
	"alex/internal/observability"
	serverPorts "alex/internal/server/ports"
	sessionstate "alex/internal/session/state_store"
)

// AgentExecutor defines the interface for agent task execution
// This allows for easier testing and mocking
type AgentExecutor interface {
	GetSession(ctx context.Context, id string) (*storage.Session, error)
	ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error)
	GetConfig() agent.AgentConfig
	PreviewContextWindow(ctx context.Context, sessionID string) (agent.ContextWindowPreview, error)
}

// Ensure AgentCoordinator implements AgentExecutor
var _ AgentExecutor = (*agentcoordinator.AgentCoordinator)(nil)

// ServerCoordinator coordinates task execution for the server
// It wraps AgentExecutor and integrates with EventBroadcaster
type ServerCoordinator struct {
	agentCoordinator AgentExecutor
	broadcaster      *EventBroadcaster
	sessionStore     storage.SessionStore
	stateStore       sessionstate.Store
	historyStore     sessionstate.Store
	taskStore        serverPorts.TaskStore
	logger           logging.Logger
	analytics        analytics.Client
	journalReader    journal.Reader
	obs              *observability.Observability

	// Cancel function map for task cancellation support
	cancelFuncs map[string]context.CancelCauseFunc
	cancelMu    sync.RWMutex
}

// ContextSnapshotRecord captures a snapshot of the messages sent to the LLM.
type ContextSnapshotRecord struct {
	SessionID    string
	TaskID       string
	ParentTaskID string
	RequestID    string
	Iteration    int
	Timestamp    time.Time
	Messages     []ports.Message
	Excluded     []ports.Message
}

// NewServerCoordinator creates a new server coordinator
func NewServerCoordinator(
	agentCoordinator AgentExecutor,
	broadcaster *EventBroadcaster,
	sessionStore storage.SessionStore,
	taskStore serverPorts.TaskStore,
	stateStore sessionstate.Store,
	opts ...ServerCoordinatorOption,
) *ServerCoordinator {
	coordinator := &ServerCoordinator{
		agentCoordinator: agentCoordinator,
		broadcaster:      broadcaster,
		sessionStore:     sessionStore,
		stateStore:       stateStore,
		taskStore:        taskStore,
		logger:           logging.NewComponentLogger("ServerCoordinator"),
		analytics:        analytics.NewNoopClient(),
		cancelFuncs:      make(map[string]context.CancelCauseFunc),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(coordinator)
		}
	}

	return coordinator
}

// ServerCoordinatorOption configures optional behavior for the server coordinator.
type ServerCoordinatorOption func(*ServerCoordinator)

// WithAnalyticsClient attaches an analytics client to the coordinator.
func WithAnalyticsClient(client analytics.Client) ServerCoordinatorOption {
	return func(coordinator *ServerCoordinator) {
		if client == nil {
			coordinator.analytics = analytics.NewNoopClient()
			return
		}
		coordinator.analytics = client
	}
}

// WithJournalReader wires a journal reader used for replay operations.
func WithJournalReader(reader journal.Reader) ServerCoordinatorOption {
	return func(coordinator *ServerCoordinator) {
		coordinator.journalReader = reader
	}
}

// WithHistoryStore wires the turn history store for cleanup operations.
func WithHistoryStore(store sessionstate.Store) ServerCoordinatorOption {
	return func(coordinator *ServerCoordinator) {
		coordinator.historyStore = store
	}
}

// WithObservability wires the observability provider into the coordinator.
func WithObservability(obs *observability.Observability) ServerCoordinatorOption {
	return func(coordinator *ServerCoordinator) {
		coordinator.obs = obs
	}
}
