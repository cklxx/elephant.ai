package http

import (
	runtimeconfig "alex/internal/config"
	"alex/internal/logging"
	"alex/internal/memory"
	"alex/internal/observability"
	"alex/internal/sandbox"
	"alex/internal/server/app"
	"alex/internal/subscription"
)

const (
	defaultMaxCreateTaskBodySize int64 = 20 << 20 // 20 MiB
)

// APIHandler handles REST API endpoints
type APIHandler struct {
	coordinator           *app.ServerCoordinator
	healthChecker         *app.HealthCheckerImpl
	logger                logging.Logger
	internalMode          bool
	devMode               bool
	obs                   *observability.Observability
	evaluationSvc         *app.EvaluationService
	attachmentStore       *AttachmentStore
	sandboxClient         *sandbox.Client
	maxCreateTaskBodySize int64
	selectionResolver     *subscription.SelectionResolver
	memoryEngine          memory.Engine
}

// APIHandlerOption configures API handler behavior.
type APIHandlerOption func(*APIHandler)

// WithAPIObservability wires observability components into the handler.
func WithAPIObservability(obs *observability.Observability) APIHandlerOption {
	return func(handler *APIHandler) {
		handler.obs = obs
	}
}

// WithEvaluationService wires evaluation service for web-triggered runs.
func WithEvaluationService(service *app.EvaluationService) APIHandlerOption {
	return func(handler *APIHandler) {
		handler.evaluationSvc = service
	}
}

// WithAttachmentStore wires an attachment store used to persist client-provided payloads
// and expose them as URL-backed attachments.
func WithAttachmentStore(store *AttachmentStore) APIHandlerOption {
	return func(handler *APIHandler) {
		handler.attachmentStore = store
	}
}

// WithSandboxClient wires a sandbox client for sandbox-related endpoints.
func WithSandboxClient(client *sandbox.Client) APIHandlerOption {
	return func(handler *APIHandler) {
		handler.sandboxClient = client
	}
}

// WithSelectionResolver wires a subscription selection resolver for per-request overrides.
func WithSelectionResolver(resolver *subscription.SelectionResolver) APIHandlerOption {
	return func(handler *APIHandler) {
		handler.selectionResolver = resolver
	}
}

// WithMemoryEngine wires a memory engine for dev memory snapshots.
func WithMemoryEngine(engine memory.Engine) APIHandlerOption {
	return func(handler *APIHandler) {
		handler.memoryEngine = engine
	}
}

// WithMaxCreateTaskBodySize overrides the maximum accepted body size for CreateTask requests.
func WithMaxCreateTaskBodySize(limit int64) APIHandlerOption {
	return func(handler *APIHandler) {
		if limit > 0 {
			handler.maxCreateTaskBodySize = limit
		}
	}
}

// WithDevMode enables development-only endpoints.
func WithDevMode(enabled bool) APIHandlerOption {
	return func(handler *APIHandler) {
		handler.devMode = enabled
	}
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(coordinator *app.ServerCoordinator, healthChecker *app.HealthCheckerImpl, internalMode bool, opts ...APIHandlerOption) *APIHandler {
	handler := &APIHandler{
		coordinator:           coordinator,
		healthChecker:         healthChecker,
		logger:                logging.NewComponentLogger("APIHandler"),
		internalMode:          internalMode,
		maxCreateTaskBodySize: defaultMaxCreateTaskBodySize,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(handler)
	}
	if handler.maxCreateTaskBodySize <= 0 {
		handler.maxCreateTaskBodySize = defaultMaxCreateTaskBodySize
	}
	if handler.selectionResolver == nil {
		handler.selectionResolver = subscription.NewSelectionResolver(func() runtimeconfig.CLICredentials {
			return runtimeconfig.LoadCLICredentials()
		})
	}
	return handler
}
