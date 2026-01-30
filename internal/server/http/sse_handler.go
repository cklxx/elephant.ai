package http

import (
	"alex/internal/agent/types"
	"alex/internal/logging"
	"alex/internal/observability"
	"alex/internal/presentation/formatter"
	"alex/internal/server/app"
)

const (
	inlineAttachmentRetentionLimit = 128 * 1024 // Keep small text blobs inline for preview fallbacks.
	sseSentAttachmentCacheSize     = 512
	sseFinalAnswerCacheSize        = 128
)

// sseAllowlist enumerates events that are relevant to the product surface. Any
// envelope not present here will be suppressed to keep the frontend stream
// lean and avoid noisy system-level lifecycle spam.
var sseAllowlist = map[string]bool{
	types.EventNodeStarted:                  true,
	types.EventNodeCompleted:                true,
	types.EventNodeFailed:                   true,
	types.EventNodeOutputDelta:              true,
	types.EventNodeOutputSummary:            true,
	types.EventToolStarted:                  true,
	types.EventToolProgress:                 true,
	types.EventToolCompleted:                true,
	types.EventArtifactManifest:             true,
	types.EventInputReceived:                true,
	types.EventSubflowProgress:              true,
	types.EventSubflowCompleted:             true,
	types.EventResultFinal:                  true,
	types.EventResultCancelled:              true,
	types.EventDiagnosticError:              true,
	types.EventDiagnosticContextCompression: true,
	types.EventDiagnosticToolFiltering:      true,
	types.EventDiagnosticEnvironmentSnapshot: true,
	types.EventExecutorUpdate:               true,
	types.EventExecutorUserMessage:          true,
	types.EventProactiveContextRefresh:      true,
}

var blockedNodeIDs = map[string]bool{
	"react:context": true,
}

var blockedNodePrefixes = []string{
	"react:",
}

// SSEHandler handles Server-Sent Events connections
type SSEHandler struct {
	broadcaster     *app.EventBroadcaster
	logger          logging.Logger
	formatter       *formatter.ToolFormatter
	obs             *observability.Observability
	dataCache       *DataCache
	attachmentStore *AttachmentStore
}

// SSEHandlerOption configures optional instrumentation for the SSE handler.
type SSEHandlerOption func(*SSEHandler)

// WithSSEObservability wires the observability provider into the handler.
func WithSSEObservability(obs *observability.Observability) SSEHandlerOption {
	return func(handler *SSEHandler) {
		handler.obs = obs
	}
}

// WithSSEDataCache wires a data cache used to offload large inline payloads.
func WithSSEDataCache(cache *DataCache) SSEHandlerOption {
	return func(handler *SSEHandler) {
		handler.dataCache = cache
	}
}

// WithSSEAttachmentStore wires a persistent attachment store so inline
// payloads (e.g. HTML artifacts) can be written to static storage.
func WithSSEAttachmentStore(store *AttachmentStore) SSEHandlerOption {
	return func(handler *SSEHandler) {
		handler.attachmentStore = store
	}
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler(broadcaster *app.EventBroadcaster, opts ...SSEHandlerOption) *SSEHandler {
	handler := &SSEHandler{
		broadcaster: broadcaster,
		logger:      logging.NewComponentLogger("SSEHandler"),
		formatter:   formatter.NewToolFormatter(),
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(handler)
	}
	return handler
}
