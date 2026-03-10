package app

import (
	"context"
	"strings"
	"sync"
	"time"

	serverPorts "alex/internal/delivery/server/ports"
	"alex/internal/infra/analytics"
	"alex/internal/infra/observability"
	"alex/internal/shared/logging"
)

const (
	defaultTaskLeaseTTL           = 45 * time.Second
	defaultTaskLeaseRenewInterval = 15 * time.Second
	defaultTaskMaxInFlight        = 64
	defaultResumeClaimBatchSize   = 128
)

// BridgeOrphanResumer detects and processes orphaned bridge subprocesses
// left behind after a process restart. Implementations classify each orphan
// (adopt, harvest, retry, or fail) and update the task store accordingly.
type BridgeOrphanResumer interface {
	// ResumeOrphans scans workDir for orphaned bridge outputs and processes them.
	// Returns a summary of actions taken per task.
	ResumeOrphans(ctx context.Context, workDir string) []OrphanResumeResult
}

// OrphanResumeResult captures what happened to a single orphaned bridge.
type OrphanResumeResult struct {
	TaskID string
	Action string
	Error  error
}

// TaskExecutionService handles asynchronous task execution, cancellation,
// and task store queries. Extracted from ServerCoordinator.
type TaskExecutionService struct {
	agentCoordinator AgentExecutor
	broadcaster      *EventBroadcaster
	progressTracker  *TaskProgressTracker
	taskStore        serverPorts.TaskStore
	stateStore       interface {
		Init(ctx context.Context, sessionID string) error
	}
	bridgeResumer BridgeOrphanResumer
	bridgeWorkDir string
	analytics     analytics.Client
	obs           *observability.Observability
	logger        logging.Logger

	cancelFuncs map[string]context.CancelCauseFunc
	cancelMu    sync.RWMutex

	ownerID              string
	leaseTTL             time.Duration
	leaseRenewInterval   time.Duration
	resumeClaimBatchSize int
	admissionSem         chan struct{}
}

// SessionTaskSummary captures task_count/last_task style metadata for a session.
type SessionTaskSummary struct {
	TaskCount int
	LastTask  string
}

// sessionTaskSummaryStore is an optional optimization interface implemented by
// task stores that can summarize multiple sessions in one pass.
type sessionTaskSummaryStore interface {
	SummarizeSessionTasks(ctx context.Context, sessionIDs []string) (map[string]SessionTaskSummary, error)
}

// NewTaskExecutionService creates a new task execution service.
func NewTaskExecutionService(
	agentCoordinator AgentExecutor,
	broadcaster *EventBroadcaster,
	taskStore serverPorts.TaskStore,
	opts ...TaskExecutionServiceOption,
) *TaskExecutionService {
	svc := &TaskExecutionService{
		agentCoordinator:     agentCoordinator,
		broadcaster:          broadcaster,
		taskStore:            taskStore,
		analytics:            analytics.NewNoopClient(),
		logger:               logging.NewComponentLogger("TaskExecutionService"),
		cancelFuncs:          make(map[string]context.CancelCauseFunc),
		ownerID:              defaultTaskOwnerID(),
		leaseTTL:             defaultTaskLeaseTTL,
		leaseRenewInterval:   defaultTaskLeaseRenewInterval,
		resumeClaimBatchSize: defaultResumeClaimBatchSize,
		admissionSem:         make(chan struct{}, defaultTaskMaxInFlight),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

// TaskExecutionServiceOption configures optional behavior.
type TaskExecutionServiceOption func(*TaskExecutionService)

// WithTaskAnalytics attaches an analytics client.
func WithTaskAnalytics(client analytics.Client) TaskExecutionServiceOption {
	return func(svc *TaskExecutionService) {
		if client == nil {
			svc.analytics = analytics.NewNoopClient()
			return
		}
		svc.analytics = client
	}
}

// WithTaskObservability wires observability.
func WithTaskObservability(obs *observability.Observability) TaskExecutionServiceOption {
	return func(svc *TaskExecutionService) {
		svc.obs = obs
	}
}

// WithTaskProgressTracker wires a progress tracker.
func WithTaskProgressTracker(tracker *TaskProgressTracker) TaskExecutionServiceOption {
	return func(svc *TaskExecutionService) {
		svc.progressTracker = tracker
	}
}

// WithTaskStateStore wires a state store for session init.
func WithTaskStateStore(store interface {
	Init(ctx context.Context, sessionID string) error
}) TaskExecutionServiceOption {
	return func(svc *TaskExecutionService) {
		svc.stateStore = store
	}
}

// WithBridgeResumer wires orphan bridge detection and resumption.
// workDir is the base directory where .elephant/bridge/ dirs are created.
func WithBridgeResumer(resumer BridgeOrphanResumer, workDir string) TaskExecutionServiceOption {
	return func(svc *TaskExecutionService) {
		svc.bridgeResumer = resumer
		svc.bridgeWorkDir = workDir
	}
}

// WithTaskOwnerID sets the task-lease owner identifier for this process.
func WithTaskOwnerID(ownerID string) TaskExecutionServiceOption {
	return func(svc *TaskExecutionService) {
		ownerID = strings.TrimSpace(ownerID)
		if ownerID == "" {
			return
		}
		svc.ownerID = ownerID
	}
}

// WithTaskLeaseConfig configures claim lease TTL and renew interval.
func WithTaskLeaseConfig(ttl, renewInterval time.Duration) TaskExecutionServiceOption {
	return func(svc *TaskExecutionService) {
		if ttl > 0 {
			svc.leaseTTL = ttl
		}
		if renewInterval > 0 {
			svc.leaseRenewInterval = renewInterval
		}
	}
}

// WithTaskAdmissionLimit configures global in-flight task admission.
// maxInFlight <= 0 disables the admission limiter.
func WithTaskAdmissionLimit(maxInFlight int) TaskExecutionServiceOption {
	return func(svc *TaskExecutionService) {
		if maxInFlight <= 0 {
			svc.admissionSem = nil
			return
		}
		svc.admissionSem = make(chan struct{}, maxInFlight)
	}
}

// WithResumeClaimBatchSize configures max claimed tasks per resume pass.
func WithResumeClaimBatchSize(batchSize int) TaskExecutionServiceOption {
	return func(svc *TaskExecutionService) {
		if batchSize <= 0 {
			return
		}
		svc.resumeClaimBatchSize = batchSize
	}
}
