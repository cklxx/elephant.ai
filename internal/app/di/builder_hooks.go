package di

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	appcontext "alex/internal/app/agent/context"
	agentcoordinator "alex/internal/app/agent/coordinator"
	"alex/internal/app/agent/hooks"
	"alex/internal/app/agent/preparation"
	ctxmgr "alex/internal/app/context"
	"alex/internal/app/subscription"
	toolregistry "alex/internal/app/toolregistry"
	agent "alex/internal/domain/agent/ports/agent"
	portsllm "alex/internal/domain/agent/ports/llm"
	"alex/internal/infra/adapters"
	codinginfra "alex/internal/infra/coding"
	"alex/internal/infra/external"
	"alex/internal/infra/external/teamrun"
	"alex/internal/infra/llm"
	"alex/internal/infra/memory"
	"alex/internal/infra/process"
	toolspolicy "alex/internal/infra/tools"
	okrtools "alex/internal/infra/tools/builtin/okr"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/parser"
)

func (b *containerBuilder) buildOKRGoalStore() *okrtools.GoalStore {
	if !b.config.Proactive.Enabled || !b.config.Proactive.OKR.Enabled {
		return nil
	}
	okrCfg := okrtools.DefaultOKRConfig()
	if goalsRoot := b.config.Proactive.OKR.GoalsRoot; goalsRoot != "" {
		okrCfg.GoalsRoot = resolveStorageDir(goalsRoot, okrCfg.GoalsRoot)
	}
	return okrtools.NewGoalStore(okrCfg)
}

func (b *containerBuilder) buildHookRegistry(memoryEngine memory.Engine, llmFactory portsllm.LLMClientFactory, okrStore *okrtools.GoalStore) *hooks.Registry {
	registry := hooks.NewRegistry(b.logger)
	if !b.config.Proactive.Enabled {
		b.logger.Info("Non-memory proactive hooks disabled by config")
	}

	// Register OKR context hook (pre-task OKR injection)
	if okrStore != nil {
		okrHook := hooks.NewOKRContextHook(okrStore, b.logger, hooks.OKRContextConfig{
			Enabled:    b.config.Proactive.OKR.Enabled,
			AutoInject: b.config.Proactive.OKR.AutoInject,
		})
		registry.Register(okrHook)
	}

	if b.config.Proactive.Enabled && b.config.Proactive.Memory.Enabled && memoryEngine != nil && llmFactory != nil {
		memHook := hooks.NewMemoryCaptureHook(memoryEngine, llmFactory, b.logger, hooks.MemoryCaptureConfig{
			Enabled: b.config.Proactive.Memory.Enabled,
			Profile: b.resolveSubscriptionOrDefaultProfile(),
		})
		registry.Register(memHook)
	}

	b.logger.Info("Hook registry built with %d hooks", registry.HookCount())
	return registry
}

func (b *containerBuilder) buildToolRegistry(_ *llm.Factory, memoryEngine memory.Engine, slaCollector *toolspolicy.SLACollector) (*toolregistry.Registry, error) {
	toolRegistry, err := toolregistry.NewRegistry(toolregistry.Config{
		Profile:       b.config.Profile,
		TavilyAPIKey:  b.config.TavilyAPIKey,
		MemoryEngine:  memoryEngine,
		HTTPLimits:    b.config.HTTPLimits,
		ToolPolicy:    toolspolicy.NewToolPolicy(b.config.ToolPolicy),
		SLACollector:  slaCollector,
		Toolset:       b.config.Toolset,
		BrowserConfig: b.config.BrowserConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create tool registry: %w", err)
	}
	return toolRegistry, nil
}

func (b *containerBuilder) buildOKRContextProvider(store *okrtools.GoalStore) preparation.OKRContextProvider {
	if store == nil {
		return nil
	}
	b.logger.Info("OKR context provider enabled")
	return preparation.NewOKRContextProvider(store)
}

// buildAlternateFrom creates an AlternateCoordinator that shares the parent
// container's heavy resources (LLM Factory, Session Store, Memory Engine,
// Cost Tracker, Context Manager, History Manager, Parser) but owns its own
// ToolRegistry and AgentCoordinator configured with the builder's config
// (which may differ in ToolMode, Toolset, BrowserConfig).
func (b *containerBuilder) buildAlternateFrom(parent *Container) (*AlternateCoordinator, error) {
	b.logger.Debug("Building alternate coordinator (tool_mode=%s, toolset=%s)", b.config.ToolMode, b.config.Toolset)

	toolSLACollector, err := toolspolicy.NewSLACollector(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create alternate SLA collector: %w", err)
	}

	toolRegistry, err := b.buildToolRegistry(parent.llmFactory, parent.MemoryEngine, toolSLACollector)
	if err != nil {
		return nil, fmt.Errorf("failed to create alternate tool registry: %w", err)
	}

	okrStore := b.buildOKRGoalStore()
	hookRegistry := b.buildHookRegistry(parent.MemoryEngine, parent.llmFactory, okrStore)
	okrContextProvider := b.buildOKRContextProvider(okrStore)
	teamRunRecorder, err := teamrun.NewFileRecorder(filepath.Join(b.sessionDir, "_team_runs"), b.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize team run recorder: %w", err)
	}
	credentialRefresher := buildCredentialRefresher()

	detectedCLIs := codinginfra.DetectLocalCLIs()
	b.applyDetectedExternalAgents(detectedCLIs, false)

	var externalExecutor agent.ExternalAgentExecutor
	externalRegistry := external.NewRegistry(b.config.ExternalAgents, process.NewController(), b.logger)
	if len(externalRegistry.SupportedTypes()) > 0 {
		externalExecutor = codinginfra.NewManagedExternalExecutor(externalRegistry, b.logger)
	}

	coordinator := agentcoordinator.NewAgentCoordinator(
		parent.llmFactory,
		toolRegistry,
		parent.SessionStore,
		ctxmgr.NewManager(
			ctxmgr.WithStateStore(parent.StateStore),
			ctxmgr.WithMemoryEngine(parent.MemoryEngine),
			ctxmgr.WithMemoryGate(memoryGateFunc(b.config.Proactive.Memory.Enabled)),
		),
		parent.HistoryManager,
		parser.New(),
		parent.CostTracker,
		b.buildAgentAppConfig(),
		agentcoordinator.WithHookRegistry(hookRegistry),
		agentcoordinator.WithExternalExecutor(externalExecutor),
		agentcoordinator.WithOKRContextProvider(okrContextProvider),
		agentcoordinator.WithCheckpointStore(parent.CheckpointStore),
		agentcoordinator.WithCredentialRefresher(credentialRefresher),
		agentcoordinator.WithToolSLACollector(toolSLACollector),
		agentcoordinator.WithTeamDefinitions(convertTeamConfigs(b.config.ExternalAgents.Teams)),
		agentcoordinator.WithTeamRunRecorder(teamRunRecorder),
		agentcoordinator.WithAtomicWriter(adapters.NewOSAtomicWriter()),
	)

	// Inherit runtime config resolver from parent coordinator so that
	// alternate coordinators (e.g. Lark) pick up runtime overrides
	// (provider switches, credential re-resolution).
	if parent.AgentCoordinator != nil {
		if resolver := parent.AgentCoordinator.GetRuntimeConfigResolver(); resolver != nil {
			coordinator.SetRuntimeConfigResolver(resolver)
		}
	}

	b.logger.Info("Alternate coordinator built (tool_mode=%s, toolset=%s)", b.config.ToolMode, b.config.Toolset)

	return &AlternateCoordinator{
		AgentCoordinator: coordinator,
		toolRegistry:     toolRegistry,
	}, nil
}

func memoryGateFunc(enabled bool) func(context.Context) bool {
	return func(ctx context.Context) bool {
		if !enabled {
			return false
		}
		policy := appcontext.ResolveMemoryPolicy(ctx)
		return policy.Enabled
	}
}

// resolveSubscriptionOrDefaultProfile checks the channel-level subscription store
// first (so that /model use overrides apply to ALL LLM paths), then falls back to
// the config file's default profile.
func (b *containerBuilder) resolveSubscriptionOrDefaultProfile() runtimeconfig.LLMProfile {
	storePath := subscription.ResolveSelectionStorePath(runtimeconfig.DefaultEnvLookup, nil)
	store := subscription.NewSelectionStore(storePath)
	resolver := subscription.NewSelectionResolver(func() runtimeconfig.CLICredentials {
		return runtimeconfig.LoadCLICredentials()
	})
	channelScope := subscription.SelectionScope{Channel: "lark"}
	if sel, _, ok, _ := store.GetWithFallback(context.Background(), channelScope); ok {
		if resolved, ok := resolver.Resolve(sel); ok {
			b.logger.Debug("Using subscription profile for auxiliary LLM: provider=%s model=%s", resolved.Provider, resolved.Model)
			return runtimeconfig.LLMProfile{
				Provider: resolved.Provider,
				Model:    resolved.Model,
				APIKey:   resolved.APIKey,
				BaseURL:  resolved.BaseURL,
				Headers:  resolved.Headers,
			}
		}
	}
	return b.resolveDefaultLLMProfile()
}

// resolveDefaultLLMProfile returns the shared runtime LLM profile.
func (b *containerBuilder) resolveDefaultLLMProfile() runtimeconfig.LLMProfile {
	return runtimeconfig.LLMProfile{
		Provider: strings.TrimSpace(b.config.LLMProvider),
		Model:    strings.TrimSpace(b.config.LLMModel),
		APIKey:   strings.TrimSpace(b.config.APIKey),
		BaseURL:  strings.TrimSpace(b.config.BaseURL),
	}
}
