package di

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	appconfig "alex/internal/app/agent/config"
	appcontext "alex/internal/app/agent/context"
	agentcoordinator "alex/internal/app/agent/coordinator"
	"alex/internal/app/agent/hooks"
	kernelagent "alex/internal/app/agent/kernel"
	"alex/internal/app/agent/preparation"
	ctxmgr "alex/internal/app/context"
	"alex/internal/app/subscription"
	toolregistry "alex/internal/app/toolregistry"
	agent "alex/internal/domain/agent/ports/agent"
	portsllm "alex/internal/domain/agent/ports/llm"
	codinginfra "alex/internal/infra/coding"
	"alex/internal/infra/external"
	kernelinfra "alex/internal/infra/kernel"
	"alex/internal/infra/llm"
	"alex/internal/infra/memory"
	toolspolicy "alex/internal/infra/tools"
	okrtools "alex/internal/infra/tools/builtin/okr"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
	"alex/internal/shared/markdown"
	"alex/internal/shared/parser"
)

func (b *containerBuilder) buildHookRegistry(memoryEngine memory.Engine, llmFactory portsllm.LLMClientFactory) *hooks.Registry {
	registry := hooks.NewRegistry(b.logger)
	if !b.config.Proactive.Enabled {
		b.logger.Info("Non-memory proactive hooks disabled by config")
	}

	// Register OKR context hook (pre-task OKR injection)
	if b.config.Proactive.Enabled && b.config.Proactive.OKR.Enabled {
		okrCfg := okrtools.DefaultOKRConfig()
		if goalsRoot := b.config.Proactive.OKR.GoalsRoot; goalsRoot != "" {
			okrCfg.GoalsRoot = resolveStorageDir(goalsRoot, okrCfg.GoalsRoot)
		}
		okrStore := okrtools.NewGoalStore(okrCfg)
		okrHook := hooks.NewOKRContextHook(okrStore, b.logger, hooks.OKRContextConfig{
			Enabled:    b.config.Proactive.OKR.Enabled,
			AutoInject: b.config.Proactive.OKR.AutoInject,
		})
		registry.Register(okrHook)
	}

	if b.config.Proactive.Enabled && b.config.Proactive.Memory.Enabled && memoryEngine != nil && llmFactory != nil {
		memHook := hooks.NewMemoryCaptureHook(memoryEngine, llmFactory, b.logger, hooks.MemoryCaptureConfig{
			Enabled:       b.config.Proactive.Memory.Enabled,
			Provider:      b.config.LLMProvider,
			Model:         b.config.LLMModel,
			SmallProvider: b.config.LLMSmallProvider,
			SmallModel:    b.config.LLMSmallModel,
			APIKey:        b.config.APIKey,
			BaseURL:       b.config.BaseURL,
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
		ArkAPIKey:     b.config.ArkAPIKey,
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

func (b *containerBuilder) buildOKRContextProvider() preparation.OKRContextProvider {
	if !b.config.Proactive.Enabled || !b.config.Proactive.OKR.Enabled {
		return nil
	}
	okrCfg := okrtools.DefaultOKRConfig()
	if goalsRoot := b.config.Proactive.OKR.GoalsRoot; goalsRoot != "" {
		okrCfg.GoalsRoot = resolveStorageDir(goalsRoot, okrCfg.GoalsRoot)
	}
	store := okrtools.NewGoalStore(okrCfg)
	b.logger.Info("OKR context provider enabled (goals_root=%s)", okrCfg.GoalsRoot)
	return preparation.NewOKRContextProvider(store)
}

func (b *containerBuilder) buildKernelAlignmentContextProvider() preparation.KernelAlignmentContextProvider {
	if !b.config.Proactive.Enabled || !b.config.Proactive.Kernel.Enabled {
		return nil
	}
	kernelID := strings.TrimSpace(b.config.Proactive.Kernel.KernelID)
	if kernelID == "" {
		kernelID = "default"
	}
	provider := preparation.NewKernelAlignmentContextProvider(preparation.KernelAlignmentContextConfig{
		KernelID: kernelID,
	})
	b.logger.Info("Kernel alignment context provider enabled (kernel_id=%s)", kernelID)
	return provider
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

	hookRegistry := b.buildHookRegistry(parent.MemoryEngine, parent.llmFactory)
	okrContextProvider := b.buildOKRContextProvider()
	kernelContextProvider := b.buildKernelAlignmentContextProvider()
	credentialRefresher := buildCredentialRefresher()

	detectedCLIs := codinginfra.DetectLocalCLIs()
	b.applyDetectedExternalAgents(detectedCLIs, false)

	var externalExecutor agent.ExternalAgentExecutor
	externalRegistry := external.NewRegistry(b.config.ExternalAgents, b.logger)
	if len(externalRegistry.SupportedTypes()) > 0 {
		externalExecutor = codinginfra.NewManagedExternalExecutor(externalRegistry, b.logger)
	}

	coordinator := agentcoordinator.NewAgentCoordinator(
		parent.llmFactory,
		toolRegistry,
		parent.SessionStore,
		ctxmgr.NewManager(
			ctxmgr.WithStateStore(parent.StateStore),
			ctxmgr.WithJournalWriter(b.buildJournalWriter()),
			ctxmgr.WithMemoryEngine(parent.MemoryEngine),
			ctxmgr.WithMemoryGate(memoryGateFunc(b.config.Proactive.Memory.Enabled)),
		),
		parent.HistoryManager,
		parser.New(),
		parent.CostTracker,
		appconfig.Config{
			LLMProvider:      b.config.LLMProvider,
			LLMModel:         b.config.LLMModel,
			LLMSmallProvider: b.config.LLMSmallProvider,
			LLMSmallModel:    b.config.LLMSmallModel,
			LLMVisionModel:   b.config.LLMVisionModel,
			APIKey:           b.config.APIKey,
			BaseURL:          b.config.BaseURL,
			LLMProfile: runtimeconfig.LLMProfile{
				Provider: b.config.LLMProvider,
				Model:    b.config.LLMModel,
				APIKey:   b.config.APIKey,
				BaseURL:  b.config.BaseURL,
			},
			MaxTokens:           b.config.MaxTokens,
			MaxIterations:       b.config.MaxIterations,
			ToolMaxConcurrent:   b.config.ToolMaxConcurrent,
			Temperature:         b.config.Temperature,
			TemperatureProvided: b.config.TemperatureProvided,
			TopP:                b.config.TopP,
			StopSequences:       append([]string(nil), b.config.StopSequences...),
			AgentPreset:         b.config.AgentPreset,
			ToolPreset:          b.config.ToolPreset,
			ToolMode:            b.config.ToolMode,
			EnvironmentSummary:  b.config.EnvironmentSummary,
			SessionStaleAfter:   b.config.SessionStaleAfter,
			Proactive:           b.config.Proactive,
			ToolPolicy:          b.config.ToolPolicy,
		},
		agentcoordinator.WithHookRegistry(hookRegistry),
		agentcoordinator.WithExternalExecutor(externalExecutor),
		agentcoordinator.WithOKRContextProvider(okrContextProvider),
		agentcoordinator.WithKernelAlignmentContextProvider(kernelContextProvider),
		agentcoordinator.WithCheckpointStore(parent.CheckpointStore),
		agentcoordinator.WithCredentialRefresher(credentialRefresher),
		agentcoordinator.WithToolSLACollector(toolSLACollector),
	)

	// Register subagent tool after coordinator is created.
	toolRegistry.RegisterSubAgent(coordinator)

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

// buildKernelEngine creates the kernel agent loop engine from config.
func (b *containerBuilder) buildKernelEngine(coordinator *agentcoordinator.AgentCoordinator) (*kernelagent.Engine, error) {
	cfg := b.config.Proactive.Kernel

	// Validate cron schedule at build time (fail fast).
	if err := kernelagent.ValidateSchedule(cfg.Schedule); err != nil {
		return nil, fmt.Errorf("kernel schedule: %w", err)
	}

	leaseDuration := time.Duration(cfg.LeaseSeconds) * time.Second
	kernelStoreDir := resolveStorageDir("", "~/.alex/kernel")
	kernelStore := kernelinfra.NewFileStore(kernelStoreDir, leaseDuration)
	if err := kernelStore.EnsureSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("kernel dispatch schema: %w", err)
	}

	stateRoot := resolveStorageDir("", kernelagent.DefaultStateRootDir)
	stateDir := filepath.Join(stateRoot, cfg.KernelID)
	seedState := kernelagent.DefaultSeedStateContent

	versionedStore := markdown.NewVersionedStore(markdown.StoreConfig{
		Dir:        stateDir,
		AutoCommit: true,
		Logger:     logging.NewKernelLogger("KernelVersionedStore"),
	})
	var stateFile *kernelagent.StateFile
	if err := versionedStore.Init(context.Background()); err != nil {
		b.logger.Warn("Kernel versioned store init failed: %v (falling back to unversioned)", err)
		stateFile = kernelagent.NewStateFile(stateDir)
	} else {
		stateFile = kernelagent.NewVersionedStateFile(stateDir, versionedStore)
	}

	agents := make([]kernelagent.AgentConfig, 0, len(cfg.Agents))
	for _, a := range cfg.Agents {
		agents = append(agents, kernelagent.AgentConfig{
			AgentID:  a.AgentID,
			Prompt:   a.Prompt,
			Priority: a.Priority,
			Enabled:  a.Enabled,
			Metadata: a.Metadata,
		})
	}

	seededAt := time.Now()
	initDoc := kernelagent.RenderInitMarkdown(kernelagent.InitDocSnapshot{
		GeneratedAt:      seededAt,
		KernelID:         cfg.KernelID,
		Schedule:         cfg.Schedule,
		StateDir:         stateRoot,
		StatePath:        stateFile.Path(),
		InitPath:         stateFile.InitPath(),
		SystemPromptPath: stateFile.SystemPromptPath(),
		TimeoutSeconds:   cfg.TimeoutSeconds,
		LeaseSeconds:     cfg.LeaseSeconds,
		MaxConcurrent:    cfg.MaxConcurrent,
		Channel:          cfg.Channel,
		UserID:           cfg.UserID,
		ChatID:           cfg.ChatID,
		SeedState:        seedState,
		Agents:           agents,
	})
	// INIT.md is a bootstrap snapshot; keep it immutable after first creation.
	if err := stateFile.SeedInit(initDoc); err != nil {
		b.logger.Warn("Kernel init doc seed failed: %v", err)
	}

	systemPrompt := strings.TrimSpace(coordinator.GetSystemPrompt())
	if systemPrompt == "" {
		systemPrompt = preparation.DefaultSystemPrompt
	}
	if err := stateFile.WriteSystemPrompt(kernelagent.RenderSystemPromptMarkdown(systemPrompt, seededAt)); err != nil {
		b.logger.Warn("Kernel system prompt doc write failed: %v", err)
	}

	planner := kernelagent.NewStaticPlanner(cfg.KernelID, agents)
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	executor := kernelagent.NewCoordinatorExecutor(coordinator, timeout)
	executor.SetSelectionResolver(b.buildKernelSelectionResolver())

	engine := kernelagent.NewEngine(
		kernelagent.KernelConfig{
			KernelID:        cfg.KernelID,
			Schedule:        cfg.Schedule,
			SeedState:       seedState,
			MaxConcurrent:   cfg.MaxConcurrent,
			MaxCycleHistory: cfg.MaxCycleHistory,
			Channel:         cfg.Channel,
			ChatID:          cfg.ChatID,
			UserID:          cfg.UserID,
		},
		stateFile, kernelStore, planner, executor, logging.NewKernelLogger("KernelEngine"),
	)
	engine.SetSystemPromptProvider(func() string { return coordinator.GetSystemPrompt() })

	b.logger.Info("Kernel engine built (kernel_id=%s, schedule=%s, agents=%d)", cfg.KernelID, cfg.Schedule, len(agents))
	return engine, nil
}

func (b *containerBuilder) buildKernelSelectionResolver() kernelagent.SelectionResolver {
	storePath := subscription.ResolveSelectionStorePath(runtimeconfig.DefaultEnvLookup, nil)
	store := subscription.NewSelectionStore(storePath)
	resolver := subscription.NewSelectionResolver(func() runtimeconfig.CLICredentials {
		return runtimeconfig.LoadCLICredentials()
	})

	return func(ctx context.Context, channel, chatID, userID string) (subscription.ResolvedSelection, bool) {
		channel = strings.ToLower(strings.TrimSpace(channel))
		chatID = strings.TrimSpace(chatID)
		userID = strings.TrimSpace(userID)
		if channel == "" {
			return subscription.ResolvedSelection{}, false
		}

		scopes := make([]subscription.SelectionScope, 0, 3)
		if chatID != "" {
			scopes = append(scopes, subscription.SelectionScope{Channel: channel, ChatID: chatID})
			if userID != "" {
				scopes = append(scopes, subscription.SelectionScope{Channel: channel, ChatID: chatID, UserID: userID})
			}
		}
		scopes = append(scopes, subscription.SelectionScope{Channel: channel})

		selection, _, ok, err := store.GetWithFallback(ctx, scopes...)
		if err != nil {
			b.logger.Warn("Kernel LLM selection load failed: %v", err)
			return subscription.ResolvedSelection{}, false
		}
		if !ok {
			return subscription.ResolvedSelection{}, false
		}

		resolved, ok := resolver.Resolve(selection)
		if !ok {
			b.logger.Warn("Kernel LLM selection resolve failed: provider=%q model=%q", selection.Provider, selection.Model)
			return subscription.ResolvedSelection{}, false
		}
		return resolved, true
	}
}
