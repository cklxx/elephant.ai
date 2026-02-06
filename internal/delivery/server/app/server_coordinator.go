package app

import (
	"context"
	"time"

	agentcoordinator "alex/internal/agent/app/coordinator"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	storage "alex/internal/agent/ports/storage"
	"alex/internal/analytics"
	"alex/internal/analytics/journal"
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

// ServerCoordinator coordinates task execution for the server.
// It acts as a thin facade over TaskExecutionService, SessionService,
// and SnapshotService, keeping the public API unchanged for HTTP handlers.
type ServerCoordinator struct {
	Tasks     *TaskExecutionService
	Sessions  *SessionService
	Snapshots *SnapshotService

	// cancelFuncs is exposed for test access (TestNoCancelFunctionLeak).
	cancelFuncs map[string]context.CancelCauseFunc
	cancelMu    interface{ RLock(); RUnlock() }
}

// ContextSnapshotRecord captures a snapshot of the messages sent to the LLM.
type ContextSnapshotRecord struct {
	SessionID   string
	RunID       string
	ParentRunID string
	RequestID   string
	Iteration   int
	Timestamp   time.Time
	Messages    []ports.Message
	Excluded    []ports.Message
}

// NewServerCoordinator creates a new server coordinator.
func NewServerCoordinator(
	agentCoordinator AgentExecutor,
	broadcaster *EventBroadcaster,
	sessionStore storage.SessionStore,
	taskStore serverPorts.TaskStore,
	stateStore sessionstate.Store,
	opts ...ServerCoordinatorOption,
) *ServerCoordinator {
	// Gather options into a config struct before distributing to services.
	cfg := coordinatorConfig{
		analytics: analytics.NewNoopClient(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	tasks := NewTaskExecutionService(
		agentCoordinator,
		broadcaster,
		taskStore,
		WithTaskAnalytics(cfg.analytics),
		WithTaskObservability(cfg.obs),
		WithTaskProgressTracker(cfg.progressTracker),
		WithTaskStateStore(stateStore),
	)

	sessions := NewSessionService(
		agentCoordinator,
		sessionStore,
		broadcaster,
		WithSessionStateStore(stateStore),
		WithSessionHistoryStore(cfg.historyStore),
	)

	snapshots := NewSnapshotService(
		agentCoordinator,
		broadcaster,
		WithSnapshotStateStore(stateStore),
		WithSnapshotJournalReader(cfg.journalReader),
	)

	return &ServerCoordinator{
		Tasks:       tasks,
		Sessions:    sessions,
		Snapshots:   snapshots,
		cancelFuncs: tasks.cancelFuncs,
		cancelMu:    &tasks.cancelMu,
	}
}

// coordinatorConfig collects option values before distributing to services.
type coordinatorConfig struct {
	analytics       analytics.Client
	journalReader   journal.Reader
	obs             *observability.Observability
	historyStore    sessionstate.Store
	progressTracker *TaskProgressTracker
}

// ServerCoordinatorOption configures optional behavior for the server coordinator.
type ServerCoordinatorOption func(*coordinatorConfig)

// WithAnalyticsClient attaches an analytics client to the coordinator.
func WithAnalyticsClient(client analytics.Client) ServerCoordinatorOption {
	return func(cfg *coordinatorConfig) {
		if client == nil {
			cfg.analytics = analytics.NewNoopClient()
			return
		}
		cfg.analytics = client
	}
}

// WithJournalReader wires a journal reader used for replay operations.
func WithJournalReader(reader journal.Reader) ServerCoordinatorOption {
	return func(cfg *coordinatorConfig) {
		cfg.journalReader = reader
	}
}

// WithHistoryStore wires the turn history store for cleanup operations.
func WithHistoryStore(store sessionstate.Store) ServerCoordinatorOption {
	return func(cfg *coordinatorConfig) {
		cfg.historyStore = store
	}
}

// WithObservability wires the observability provider into the coordinator.
func WithObservability(obs *observability.Observability) ServerCoordinatorOption {
	return func(cfg *coordinatorConfig) {
		cfg.obs = obs
	}
}

// WithProgressTracker wires a task progress tracker into the coordinator.
func WithProgressTracker(tracker *TaskProgressTracker) ServerCoordinatorOption {
	return func(cfg *coordinatorConfig) {
		cfg.progressTracker = tracker
	}
}

// ── Pass-through delegates (preserve existing public API) ──

// ExecuteTaskAsync delegates to TaskExecutionService.
func (s *ServerCoordinator) ExecuteTaskAsync(ctx context.Context, task string, sessionID string, agentPreset string, toolPreset string) (*serverPorts.Task, error) {
	return s.Tasks.ExecuteTaskAsync(ctx, task, sessionID, agentPreset, toolPreset)
}

// GetTask delegates to TaskExecutionService.
func (s *ServerCoordinator) GetTask(ctx context.Context, taskID string) (*serverPorts.Task, error) {
	return s.Tasks.GetTask(ctx, taskID)
}

// ListTasks delegates to TaskExecutionService.
func (s *ServerCoordinator) ListTasks(ctx context.Context, limit int, offset int) ([]*serverPorts.Task, int, error) {
	return s.Tasks.ListTasks(ctx, limit, offset)
}

// ListSessionTasks delegates to TaskExecutionService.
func (s *ServerCoordinator) ListSessionTasks(ctx context.Context, sessionID string) ([]*serverPorts.Task, error) {
	return s.Tasks.ListSessionTasks(ctx, sessionID)
}

// CancelTask delegates to TaskExecutionService.
func (s *ServerCoordinator) CancelTask(ctx context.Context, taskID string) error {
	return s.Tasks.CancelTask(ctx, taskID)
}

// GetSession delegates to SessionService.
func (s *ServerCoordinator) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	return s.Sessions.GetSession(ctx, id)
}

// UpdateSessionPersona delegates to SessionService.
func (s *ServerCoordinator) UpdateSessionPersona(ctx context.Context, id string, persona *ports.UserPersonaProfile) (*storage.Session, error) {
	return s.Sessions.UpdateSessionPersona(ctx, id, persona)
}

// ListSessions delegates to SessionService.
func (s *ServerCoordinator) ListSessions(ctx context.Context, limit int, offset int) ([]string, error) {
	return s.Sessions.ListSessions(ctx, limit, offset)
}

// CreateSession delegates to SessionService.
func (s *ServerCoordinator) CreateSession(ctx context.Context) (*storage.Session, error) {
	return s.Sessions.CreateSession(ctx)
}

// DeleteSession delegates to SessionService.
func (s *ServerCoordinator) DeleteSession(ctx context.Context, id string) error {
	return s.Sessions.DeleteSession(ctx, id)
}

// ForkSession delegates to SessionService.
func (s *ServerCoordinator) ForkSession(ctx context.Context, sessionID string) (*storage.Session, error) {
	return s.Sessions.ForkSession(ctx, sessionID)
}

// EnsureSessionShareToken delegates to SessionService.
func (s *ServerCoordinator) EnsureSessionShareToken(ctx context.Context, sessionID string, reset bool) (string, error) {
	return s.Sessions.EnsureSessionShareToken(ctx, sessionID, reset)
}

// ValidateShareToken delegates to SessionService.
func (s *ServerCoordinator) ValidateShareToken(ctx context.Context, sessionID string, token string) (*storage.Session, error) {
	return s.Sessions.ValidateShareToken(ctx, sessionID, token)
}

// ListSnapshots delegates to SnapshotService.
func (s *ServerCoordinator) ListSnapshots(ctx context.Context, sessionID string, cursor string, limit int) ([]sessionstate.SnapshotMetadata, string, error) {
	return s.Snapshots.ListSnapshots(ctx, sessionID, cursor, limit)
}

// GetSnapshot delegates to SnapshotService.
func (s *ServerCoordinator) GetSnapshot(ctx context.Context, sessionID string, turnID int) (sessionstate.Snapshot, error) {
	return s.Snapshots.GetSnapshot(ctx, sessionID, turnID)
}

// ReplaySession delegates to SnapshotService.
func (s *ServerCoordinator) ReplaySession(ctx context.Context, sessionID string) error {
	return s.Snapshots.ReplaySession(ctx, sessionID)
}

// PreviewContextWindow delegates to SnapshotService.
func (s *ServerCoordinator) PreviewContextWindow(ctx context.Context, sessionID string) (agent.ContextWindowPreview, error) {
	return s.Snapshots.PreviewContextWindow(ctx, sessionID)
}

// GetContextSnapshots delegates to SnapshotService.
func (s *ServerCoordinator) GetContextSnapshots(sessionID string) []ContextSnapshotRecord {
	return s.Snapshots.GetContextSnapshots(sessionID)
}
