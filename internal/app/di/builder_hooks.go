package di

import (
	"alex/internal/shared/utils"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	appcontext "alex/internal/app/agent/context"
	agentcoordinator "alex/internal/app/agent/coordinator"
	"alex/internal/app/agent/hooks"
	kernelagent "alex/internal/app/agent/kernel"
	"alex/internal/app/agent/llmclient"
	"alex/internal/app/agent/preparation"
	ctxmgr "alex/internal/app/context"
	"alex/internal/app/subscription"
	toolregistry "alex/internal/app/toolregistry"
	agent "alex/internal/domain/agent/ports/agent"
	portsllm "alex/internal/domain/agent/ports/llm"
	codinginfra "alex/internal/infra/coding"
	"alex/internal/infra/external"
	"alex/internal/infra/external/teamrun"
	"alex/internal/infra/process"
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
	kernelID := strings.TrimSpace(kernelagent.DefaultRuntimeSettings().KernelID)
	if kernelID == "" {
		kernelID = kernelagent.DefaultKernelID
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
			ctxmgr.WithJournalWriter(b.buildJournalWriter()),
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
		agentcoordinator.WithKernelAlignmentContextProvider(kernelContextProvider),
		agentcoordinator.WithCheckpointStore(parent.CheckpointStore),
		agentcoordinator.WithCredentialRefresher(credentialRefresher),
		agentcoordinator.WithToolSLACollector(toolSLACollector),
		agentcoordinator.WithTeamDefinitions(convertTeamConfigs(b.config.ExternalAgents.Teams)),
		agentcoordinator.WithTeamRunRecorder(teamRunRecorder),
	)

	// Inherit runtime config resolver from parent coordinator so that
	// alternate coordinators (e.g. Lark) pick up runtime overrides
	// (provider switches, credential re-resolution).
	if parent.AgentCoordinator != nil {
		if resolver := parent.AgentCoordinator.GetRuntimeConfigResolver(); resolver != nil {
			coordinator.SetRuntimeConfigResolver(resolver)
		}
	}

	// Register orchestration tools (run_tasks, reply_agent).
	toolRegistry.RegisterOrchestration()

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

func stringOr(value, fallback string) string {
	if v := strings.TrimSpace(value); v != "" {
		return v
	}
	return fallback
}

func intOr(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

// buildKernelEngine creates the kernel agent loop engine from code-owned defaults.
func (b *containerBuilder) buildKernelEngine(coordinator *agentcoordinator.AgentCoordinator, llmFactory portsllm.LLMClientFactory) (*kernelagent.Engine, error) {
	settings := kernelagent.DefaultRuntimeSettings()
	kernelID := stringOr(settings.KernelID, kernelagent.DefaultKernelID)
	schedule := stringOr(settings.Schedule, kernelagent.DefaultKernelSchedule)
	timeoutSeconds := intOr(settings.TimeoutSeconds, kernelagent.DefaultKernelTimeoutSeconds)
	leaseSeconds := intOr(settings.LeaseSeconds, kernelagent.DefaultKernelLeaseSeconds)
	maxConcurrent := intOr(settings.MaxConcurrent, kernelagent.DefaultKernelMaxConcurrent)
	maxCycleHistory := intOr(settings.MaxCycleHistory, kernelagent.DefaultKernelMaxCycleHistory)
	seedState := settings.SeedState
	if utils.IsBlank(seedState) {
		seedState = kernelagent.DefaultSeedStateContent
	}
	channel := stringOr(settings.Channel, kernelagent.DefaultKernelChannel)
	userID := stringOr(settings.UserID, kernelagent.DefaultKernelUserID)
	chatID := strings.TrimSpace(settings.ChatID)
	agents := kernelagent.CloneAgentConfigs(settings.Agents)

	// Validate cron schedule at build time (fail fast).
	if err := kernelagent.ValidateSchedule(schedule); err != nil {
		return nil, fmt.Errorf("kernel schedule: %w", err)
	}

	leaseDuration := time.Duration(leaseSeconds) * time.Second
	kernelStoreDir := resolveStorageDir("", "~/.alex/kernel")
	kernelStore := kernelinfra.NewFileStore(kernelStoreDir, leaseDuration)
	if err := kernelStore.EnsureSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("kernel dispatch schema: %w", err)
	}

	stateRoot := resolveStorageDir("", kernelagent.DefaultStateRootDir)
	stateDir := filepath.Join(stateRoot, kernelID)

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

	seededAt := time.Now()
	initDoc := kernelagent.RenderInitMarkdown(kernelagent.InitDocSnapshot{
		GeneratedAt:      seededAt,
		KernelID:         kernelID,
		Schedule:         schedule,
		StateDir:         stateRoot,
		StatePath:        stateFile.Path(),
		InitPath:         stateFile.InitPath(),
		SystemPromptPath: stateFile.SystemPromptPath(),
		TimeoutSeconds:   timeoutSeconds,
		LeaseSeconds:     leaseSeconds,
		MaxConcurrent:    maxConcurrent,
		Channel:          channel,
		UserID:           userID,
		ChatID:           chatID,
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
	renderedSystemPrompt := kernelagent.RenderSystemPromptMarkdown(systemPrompt, seededAt)
	if err := stateFile.WriteSystemPrompt(renderedSystemPrompt); err != nil {
		if kernelagent.IsSandboxPathRestriction(err) {
			fallbackPath, fallbackErr := kernelagent.AppendKernelStateFallback("SYSTEM_PROMPT.md fallback", renderedSystemPrompt)
			if fallbackErr != nil {
				b.logger.Warn("Kernel system prompt doc write blocked by sandbox restrictions (fallback write to %s failed: %v)", fallbackPath, fallbackErr)
			} else {
				b.logger.Warn("Kernel system prompt doc write blocked by sandbox restrictions; fallback written to %s", fallbackPath)
			}
		} else {
			b.logger.Warn("Kernel system prompt doc write failed: %v", err)
		}
	}

	// Build planner: HybridPlanner (LLM + static fallback) when llm_planner enabled.
	staticPlanner := kernelagent.NewStaticPlanner(kernelID, agents)
	var planner kernelagent.Planner = staticPlanner

	plannerSettings := settings.Planner
	if plannerSettings.Enabled && llmFactory != nil {
		plannerProfile := b.resolveSubscriptionOrDefaultProfile()

		plannerTimeout := time.Duration(plannerSettings.TimeoutSeconds) * time.Second
		if plannerTimeout <= 0 {
			plannerTimeout = 30 * time.Second
		}
		maxDispatches := plannerSettings.MaxDispatches
		if maxDispatches <= 0 {
			maxDispatches = 5
		}
		maxTeamsPerCycle := plannerSettings.MaxTeamsPerCycle
		if maxTeamsPerCycle <= 0 {
			maxTeamsPerCycle = kernelagent.DefaultKernelMaxTeamsPerCycle
		}
		teamTimeoutSeconds := plannerSettings.TeamTimeoutSeconds
		if teamTimeoutSeconds <= 0 {
			teamTimeoutSeconds = kernelagent.DefaultKernelTeamTimeoutSeconds
		}
		allowedTeamTemplates := collectTeamTemplateNames(b.config.ExternalAgents.Teams)
		goalFilePath := plannerSettings.GoalFile
		if goalFilePath == "" {
			goalFilePath = filepath.Join(stateDir, "GOAL.md")
		}

		llmPlanner := kernelagent.NewLLMPlanner(
			kernelID,
			llmFactory,
			kernelagent.LLMPlannerConfig{
				Profile:              plannerProfile,
				Refresher:            llmclient.CredentialRefresher(buildCredentialRefresher()),
				MaxDispatches:        maxDispatches,
				GoalFilePath:         goalFilePath,
				Timeout:              plannerTimeout,
				TeamDispatchEnabled:  plannerSettings.TeamDispatchEnabled,
				MaxTeamsPerCycle:     maxTeamsPerCycle,
				TeamTimeoutSeconds:   teamTimeoutSeconds,
				AllowedTeamTemplates: allowedTeamTemplates,
			},
			agents,
			logging.NewKernelLogger("LLMPlanner"),
		)
		planner = kernelagent.NewHybridPlanner(staticPlanner, llmPlanner, logging.NewKernelLogger("HybridPlanner"))
		b.logger.Info("Kernel LLM planner enabled (provider=%s model=%s goal=%s)", plannerProfile.Provider, plannerProfile.Model, goalFilePath)
		logging.NewKernelLogger("HybridPlanner").Info("HybridPlanner created (provider=%s model=%s baseURL=%s goal=%s maxDispatches=%d maxTeams=%d timeout=%s)",
			plannerProfile.Provider, plannerProfile.Model, plannerProfile.BaseURL, goalFilePath, maxDispatches, maxTeamsPerCycle, plannerTimeout)
	}

	timeout := time.Duration(timeoutSeconds) * time.Second
	executor := kernelagent.NewCoordinatorExecutor(coordinator, timeout)
	executor.SetSelectionResolver(b.buildKernelSelectionResolver())

	engine := kernelagent.NewEngine(
		kernelagent.KernelConfig{
			KernelID:        kernelID,
			Schedule:        schedule,
			SeedState:       seedState,
			MaxConcurrent:   maxConcurrent,
			MaxCycleHistory: maxCycleHistory,
			Channel:         channel,
			ChatID:          chatID,
			UserID:          userID,
		},
		stateFile, kernelStore, planner, executor, logging.NewKernelLogger("KernelEngine"),
	)
	engine.SetSystemPromptProvider(func() string { return coordinator.GetSystemPrompt() })

	b.logger.Info("Kernel engine built (kernel_id=%s, schedule=%s, agents=%d)", kernelID, schedule, len(agents))
	return engine, nil
}

func collectTeamTemplateNames(teams []runtimeconfig.TeamConfig) []string {
	if len(teams) == 0 {
		return nil
	}
	out := make([]string, 0, len(teams))
	for _, team := range teams {
		name := strings.TrimSpace(team.Name)
		if name == "" {
			continue
		}
		out = append(out, name)
	}
	return out
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
