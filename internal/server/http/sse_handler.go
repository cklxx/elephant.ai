package http

import (
	"alex/internal/agent/domain"
	"alex/internal/logging"
	"alex/internal/observability"
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
	"workflow.node.started":                    true,
	"workflow.node.completed":                  true,
	"workflow.node.failed":                     true,
	"workflow.node.output.delta":               true,
	"workflow.node.output.summary":             true,
	"workflow.tool.started":                    true,
	"workflow.tool.progress":                   true,
	"workflow.tool.completed":                  true,
	"workflow.artifact.manifest":               true,
	"workflow.input.received":                  true,
	"workflow.subflow.progress":                true,
	"workflow.subflow.completed":               true,
	"workflow.result.final":                    true,
	"workflow.result.cancelled":                true,
	"workflow.diagnostic.error":                true,
	"workflow.diagnostic.context_compression":  true,
	"workflow.diagnostic.tool_filtering":       true,
	"workflow.diagnostic.environment_snapshot": true,
	"workflow.executor.update":                 true,
	"workflow.executor.user_message":           true,
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
	formatter       *domain.ToolFormatter
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
		formatter:   domain.NewToolFormatter(),
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(handler)
	}
	return handler
}
