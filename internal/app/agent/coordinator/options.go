package coordinator

import (
	"alex/internal/app/agent/preparation"
	corehook "alex/internal/core/hook"
	agent "alex/internal/domain/agent/ports/agent"
	react "alex/internal/domain/agent/react"
	toolspolicy "alex/internal/infra/tools"
)

// CoordinatorOption configures optional dependencies for the agent coordinator.
type CoordinatorOption func(*AgentCoordinator)

// WithLogger overrides the default coordinator logger.
func WithLogger(logger agent.Logger) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if logger != nil {
			c.logger = logger
		}
	}
}

// WithHookRuntime sets the hook runtime for pre/post-task processing.
func WithHookRuntime(rt *corehook.HookRuntime) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if rt != nil {
			c.hookRuntime = rt
		}
	}
}

// WithCheckpointStore provides a checkpoint store for ReAct recovery.
func WithCheckpointStore(store react.CheckpointStore) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if store != nil {
			c.checkpointStore = store
		}
	}
}

// WithOKRContextProvider provides the OKR context provider for system prompt injection.
func WithOKRContextProvider(provider preparation.OKRContextProvider) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if provider != nil {
			c.okrContextProvider = provider
		}
	}
}

// WithCredentialRefresher provides a function that re-resolves CLI credentials
// at task execution time. This keeps long-running servers (e.g. Lark) working
// when startup tokens expire and need OAuth refresh.
func WithCredentialRefresher(fn preparation.CredentialRefresher) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if fn != nil {
			c.credentialRefresher = fn
		}
	}
}

// WithToolSLACollector provides the runtime tool SLA collector used for
// translator payload enrichment.
func WithToolSLACollector(collector *toolspolicy.SLACollector) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if collector != nil {
			c.toolSLACollector = collector
		}
	}
}

// WithChannelHints provides the channel-name to formatting-hint mapping.
// The preparation service uses this to resolve a pre-rendered hint for the
// active delivery channel, removing hardcoded channel checks from prompt
// composition.
func WithChannelHints(hints map[string]string) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if hints != nil {
			c.channelHints = hints
		}
	}
}

// WithAtomicWriter provides the atomic file writer used for context compaction artifacts.
func WithAtomicWriter(writer agent.AtomicFileWriter) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if writer != nil {
			c.atomicWriter = writer
		}
	}
}

// WithTurnRecorder provides a tape-based audit trail for per-message recording.
func WithTurnRecorder(recorder agent.TurnRecorder) CoordinatorOption {
	return func(c *AgentCoordinator) {
		if recorder != nil {
			c.turnRecorder = recorder
		}
	}
}
