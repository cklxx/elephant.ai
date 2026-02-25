package coordinator

import (
	"context"
	"fmt"

	"alex/internal/app/agent/llmclient"
	agent "alex/internal/domain/agent/ports/agent"
	llm "alex/internal/domain/agent/ports/llm"
	storage "alex/internal/domain/agent/ports/storage"
	tools "alex/internal/domain/agent/ports/tools"
	materialports "alex/internal/domain/materials/ports"
	"alex/internal/domain/agent/ports"
	"alex/internal/infra/tools/builtin/shared"
)

// CancelBackgroundTask cancels an individual background task by searching all sessions.
func (c *AgentCoordinator) CancelBackgroundTask(ctx context.Context, taskID string) error {
	if c.bgRegistry == nil {
		return fmt.Errorf("background task registry not available")
	}
	return c.bgRegistry.CancelTask(ctx, taskID)
}

// GetCostTracker returns the cost tracker instance
func (c *AgentCoordinator) GetCostTracker() storage.CostTracker {
	return c.costTracker
}

// GetToolRegistry returns the tool registry instance
func (c *AgentCoordinator) GetToolRegistry() tools.ToolRegistry {
	return c.toolRegistry
}

// GetToolRegistryWithoutSubagent returns a filtered registry that excludes subagent
// This is used by subagent tool to prevent nested subagent calls
func (c *AgentCoordinator) GetToolRegistryWithoutSubagent() tools.ToolRegistry {
	// Check if the registry implements WithoutSubagent method
	type registryWithFilter interface {
		WithoutSubagent() tools.ToolRegistry
	}

	if filtered, ok := c.toolRegistry.(registryWithFilter); ok {
		return filtered.WithoutSubagent()
	}

	// Fallback: return original registry if filtering not supported
	return c.toolRegistry
}

// GetLLMClient returns an LLM client
func (c *AgentCoordinator) GetLLMClient() (llm.LLMClient, error) {
	cfg := c.effectiveConfig(context.Background())
	profile := cfg.DefaultLLMProfile()
	client, _, err := llmclient.GetClientFromProfile(
		c.llmFactory,
		profile,
		llmclient.CredentialRefresher(c.credentialRefresher),
		true,
	)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// GetParser returns the function call parser
func (c *AgentCoordinator) GetParser() agent.FunctionCallParser {
	return c.parser
}

// GetContextManager returns the context manager
func (c *AgentCoordinator) GetContextManager() agent.ContextManager {
	return c.contextMgr
}

// SetEnvironmentSummary updates the environment context appended to system prompts.
func (c *AgentCoordinator) SetEnvironmentSummary(summary string) {
	c.config.EnvironmentSummary = summary
	if c.prepService != nil {
		c.prepService.SetEnvironmentSummary(summary)
	}
}

// SetAttachmentMigrator wires an attachment migrator for boundary externalization.
// Agent state keeps inline payloads; CDN rewriting happens at HTTP/SSE boundaries.
func (c *AgentCoordinator) SetAttachmentMigrator(migrator materialports.Migrator) {
	c.attachmentMigrator = migrator
}

// SetAttachmentPersister wires an eager persister that writes inline attachment
// payloads to durable storage during the ReAct loop, replacing base64 Data with
// stable URIs.  This offloads attachment content from memory and reduces event
// serialization size.
func (c *AgentCoordinator) SetAttachmentPersister(p ports.AttachmentPersister) {
	c.attachmentPersister = p
}

// SetTimerManager wires the agent timer manager so tools can create/list/cancel
// timers during execution.
func (c *AgentCoordinator) SetTimerManager(mgr shared.TimerManagerService) {
	c.timerManager = mgr
}

// SetScheduler wires the scheduler runtime service so scheduler tools can
// operate against the active scheduler instance during execution.
func (c *AgentCoordinator) SetScheduler(service any) {
	c.schedulerService = service
}

// Close releases coordinator-owned resources such as background persisters.
func (c *AgentCoordinator) Close() error {
	if c == nil || c.attachmentPersister == nil {
		return nil
	}
	if closer, ok := c.attachmentPersister.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}
