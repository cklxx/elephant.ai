package di

import (
	"fmt"
	"path/filepath"
	"strings"

	appconfig "alex/internal/app/agent/config"
	agentcoordinator "alex/internal/app/agent/coordinator"
	ctxmgr "alex/internal/app/context"
	"alex/internal/app/lifecycle"
	agent "alex/internal/domain/agent/ports/agent"
	agentstorage "alex/internal/domain/agent/ports/storage"
	react "alex/internal/domain/agent/react"
	codinginfra "alex/internal/infra/coding"
	"alex/internal/infra/external"
	"alex/internal/infra/mcp"
	sessionstate "alex/internal/infra/session/state_store"
	toolspolicy "alex/internal/infra/tools"
	"alex/internal/domain/agent/presets"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
	"alex/internal/shared/parser"
)

type containerBuilder struct {
	config     Config
	logger     logging.Logger
	sessionDir string
	costDir    string
}

type sessionResources struct {
	sessionStore agentstorage.SessionStore
	stateStore   sessionstate.Store
	historyStore sessionstate.Store
}

// BuildContainer builds the dependency injection container with the given configuration.
// Heavy initialization (MCP) is deferred until Start() is called.
func BuildContainer(config Config) (*Container, error) {
	builder := newContainerBuilder(config)
	return builder.Build()
}

func newContainerBuilder(config Config) *containerBuilder {
	logger := logging.NewComponentLogger("DI")
	sessionDir := resolveStorageDir(config.SessionDir, "~/.alex/sessions")
	costDir := resolveStorageDir(config.CostDir, "~/.alex/costs")
	if strings.TrimSpace(config.ToolMode) == "" {
		config.ToolMode = string(presets.ToolModeCLI)
	}
	config.SessionDir = sessionDir
	config.CostDir = costDir

	return &containerBuilder{
		config:     config,
		logger:     logger,
		sessionDir: sessionDir,
		costDir:    costDir,
	}
}

func (b *containerBuilder) Build() (*Container, error) {
	b.logger.Debug("Building container with session_dir=%s, cost_dir=%s", b.sessionDir, b.costDir)
	detectedCLIs := codinginfra.DetectLocalCLIs()
	b.applyDetectedExternalAgents(detectedCLIs, true)
	b.logLocalCodingCLIDetection(detectedCLIs)

	llmFactory := b.buildLLMFactory()
	resources, err := b.buildSessionResources()
	if err != nil {
		return nil, err
	}

	memoryEngine, err := b.buildMemoryEngine()
	if err != nil {
		return nil, err
	}
	journalWriter := b.buildJournalWriter()
	contextOptions := []ctxmgr.Option{
		ctxmgr.WithStateStore(resources.stateStore),
		ctxmgr.WithJournalWriter(journalWriter),
	}
	contextOptions = append(contextOptions, ctxmgr.WithMemoryEngine(memoryEngine))
	contextOptions = append(contextOptions, ctxmgr.WithMemoryGate(memoryGateFunc(b.config.Proactive.Memory.Enabled)))
	contextMgr := ctxmgr.NewManager(contextOptions...)
	historyMgr := ctxmgr.NewHistoryManager(resources.historyStore, b.logger, agent.SystemClock{})
	parserImpl := parser.New()
	llmFactory.EnableToolCallParsing(parserImpl)

	costTracker, err := b.buildCostTracker()
	if err != nil {
		return nil, err
	}

	toolSLACollector, err := toolspolicy.NewSLACollector(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create SLA collector: %w", err)
	}

	toolRegistry, err := b.buildToolRegistry(llmFactory, memoryEngine, toolSLACollector)
	if err != nil {
		return nil, err
	}

	var externalExecutor agent.ExternalAgentExecutor
	externalRegistry := external.NewRegistry(b.config.ExternalAgents, b.logger)
	if len(externalRegistry.SupportedTypes()) > 0 {
		externalExecutor = codinginfra.NewManagedExternalExecutor(externalRegistry, b.logger)
	}

	mcpRegistry := mcp.NewRegistry(
		mcp.WithPlaywrightBrowser(mcp.PlaywrightBrowserConfig{
			Connector:   b.config.BrowserConfig.Connector,
			CDPURL:      b.config.BrowserConfig.CDPURL,
			ChromePath:  b.config.BrowserConfig.ChromePath,
			Headless:    b.config.BrowserConfig.Headless,
			UserDataDir: b.config.BrowserConfig.UserDataDir,
			Timeout:     b.config.BrowserConfig.Timeout,
			BridgeToken: b.config.BrowserConfig.BridgeToken,
		}),
	)
	tracker := newMCPInitializationTracker()

	hookRegistry := b.buildHookRegistry(memoryEngine, llmFactory)
	okrContextProvider := b.buildOKRContextProvider()
	kernelContextProvider := b.buildKernelAlignmentContextProvider()
	checkpointStore := react.NewFileCheckpointStore(filepath.Join(b.sessionDir, "checkpoints"))
	credentialRefresher := buildCredentialRefresher()

	coordinator := agentcoordinator.NewAgentCoordinator(
		llmFactory,
		toolRegistry,
		resources.sessionStore,
		contextMgr,
		historyMgr,
		parserImpl,
		costTracker,
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
		agentcoordinator.WithCheckpointStore(checkpointStore),
		agentcoordinator.WithCredentialRefresher(credentialRefresher),
		agentcoordinator.WithToolSLACollector(toolSLACollector),
		agentcoordinator.WithTeamDefinitions(convertTeamConfigs(b.config.ExternalAgents.Teams)),
	)

	// Register subagent tool after coordinator is created.
	toolRegistry.RegisterSubAgent(coordinator)

	b.logger.Info("Container built successfully (heavy initialization deferred to Start())")

	container := &Container{
		AgentCoordinator: coordinator,
		SessionStore:     resources.sessionStore,
		StateStore:       resources.stateStore,
		HistoryStore:     resources.historyStore,
		HistoryManager:   historyMgr,
		CostTracker:      costTracker,
		MemoryEngine:     memoryEngine,
		CheckpointStore:  checkpointStore,
		MCPRegistry:      mcpRegistry,
		mcpInitTracker:   tracker,
		config:           b.config,
		toolRegistry:     toolRegistry,
		llmFactory:       llmFactory,
		mcpStarted:       false,
	}
	if drainable, ok := memoryEngine.(lifecycle.Drainable); ok {
		container.Drainables = append(container.Drainables, drainable)
	}

	// Build kernel engine if enabled.
	if b.config.Proactive.Kernel.Enabled {
		kernelEngine, err := b.buildKernelEngine(coordinator, llmFactory)
		if err != nil {
			b.logger.Warn("Kernel engine init failed: %v (kernel disabled)", err)
		} else {
			container.KernelEngine = kernelEngine
		}
	}

	return container, nil
}

func (b *containerBuilder) logLocalCodingCLIDetection(detected []codinginfra.LocalCLIDetection) {
	if len(detected) == 0 {
		b.logger.Info("Coding CLI auto-detect: none found (checked: codex, claude, kimi)")
		return
	}
	for _, item := range detected {
		if !item.AdapterSupport {
			b.logger.Info(
				"Coding CLI auto-detect: found %s (%s) at %s [adapter=unsupported]",
				item.ID,
				item.Binary,
				item.Path,
			)
			continue
		}
		enabled := b.isExternalAgentEnabled(item.AgentType)
		b.logger.Info(
			"Coding CLI auto-detect: found %s (%s) at %s [agent_type=%s enabled=%t]",
			item.ID,
			item.Binary,
			item.Path,
			item.AgentType,
			enabled,
		)
	}
}

func (b *containerBuilder) applyDetectedExternalAgents(detected []codinginfra.LocalCLIDetection, log bool) {
	for _, item := range detected {
		if !item.AdapterSupport {
			continue
		}
		agentType := strings.ToLower(strings.TrimSpace(item.AgentType))
		switch agentType {
		case "codex":
			wasEnabled := b.config.ExternalAgents.Codex.Enabled
			changedBinary := false
			if !wasEnabled {
				b.config.ExternalAgents.Codex.Enabled = true
			}
			if shouldAdoptDetectedBinary(b.config.ExternalAgents.Codex.Binary, item.Binary) {
				changedBinary = b.config.ExternalAgents.Codex.Binary != item.Path
				b.config.ExternalAgents.Codex.Binary = item.Path
			}
			if log && (!wasEnabled || changedBinary) {
				b.logger.Info(
					"Coding CLI auto-enable: agent_type=%s enabled with binary=%s",
					agentType,
					b.config.ExternalAgents.Codex.Binary,
				)
			}
		case "claude_code":
			wasEnabled := b.config.ExternalAgents.ClaudeCode.Enabled
			changedBinary := false
			if !wasEnabled {
				b.config.ExternalAgents.ClaudeCode.Enabled = true
			}
			if shouldAdoptDetectedBinary(b.config.ExternalAgents.ClaudeCode.Binary, item.Binary) {
				changedBinary = b.config.ExternalAgents.ClaudeCode.Binary != item.Path
				b.config.ExternalAgents.ClaudeCode.Binary = item.Path
			}
			if log && (!wasEnabled || changedBinary) {
				b.logger.Info(
					"Coding CLI auto-enable: agent_type=%s enabled with binary=%s",
					agentType,
					b.config.ExternalAgents.ClaudeCode.Binary,
				)
			}
		}
	}
}

func shouldAdoptDetectedBinary(current, detectedBinary string) bool {
	trimmedCurrent := strings.TrimSpace(current)
	trimmedDetected := strings.TrimSpace(detectedBinary)
	if trimmedDetected == "" {
		return false
	}
	if trimmedCurrent == "" {
		return true
	}
	if strings.EqualFold(trimmedCurrent, trimmedDetected) {
		return true
	}
	if isEquivalentCLIBinary(trimmedCurrent, trimmedDetected) {
		return true
	}
	if strings.EqualFold(filepath.Base(trimmedCurrent), trimmedDetected) {
		return true
	}
	return false
}

func isEquivalentCLIBinary(current, detected string) bool {
	currentLower := strings.ToLower(strings.TrimSpace(current))
	detectedLower := strings.ToLower(strings.TrimSpace(detected))
	if currentLower == detectedLower {
		return true
	}
	switch {
	case (currentLower == "claude" || currentLower == "claude-code") &&
		(detectedLower == "claude" || detectedLower == "claude-code"):
		return true
	default:
		return false
	}
}

func (b *containerBuilder) isExternalAgentEnabled(agentType string) bool {
	switch strings.ToLower(strings.TrimSpace(agentType)) {
	case "codex":
		return b.config.ExternalAgents.Codex.Enabled
	case "claude_code":
		return b.config.ExternalAgents.ClaudeCode.Enabled
	default:
		return false
	}
}

// convertTeamConfigs maps config-layer TeamConfig to domain-layer TeamDefinition.
func convertTeamConfigs(configs []runtimeconfig.TeamConfig) []agent.TeamDefinition {
	if len(configs) == 0 {
		return nil
	}
	teams := make([]agent.TeamDefinition, 0, len(configs))
	for _, cfg := range configs {
		roles := make([]agent.TeamRoleDefinition, 0, len(cfg.Roles))
		for _, r := range cfg.Roles {
			roles = append(roles, agent.TeamRoleDefinition{
				Name:           r.Name,
				AgentType:      r.AgentType,
				PromptTemplate: r.PromptTemplate,
				ExecutionMode:  r.ExecutionMode,
				AutonomyLevel:  r.AutonomyLevel,
				WorkspaceMode:  r.WorkspaceMode,
				Config:         r.Config,
				InheritContext: r.InheritContext,
			})
		}
		stages := make([]agent.TeamStageDefinition, 0, len(cfg.Stages))
		for _, s := range cfg.Stages {
			stages = append(stages, agent.TeamStageDefinition{
				Name:  s.Name,
				Roles: s.Roles,
			})
		}
		teams = append(teams, agent.TeamDefinition{
			Name:        cfg.Name,
			Description: cfg.Description,
			Roles:       roles,
			Stages:      stages,
		})
	}
	return teams
}
