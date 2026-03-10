package di

import (
	"alex/internal/shared/utils"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	agentcoordinator "alex/internal/app/agent/coordinator"
	ctxmgr "alex/internal/app/context"
	"alex/internal/app/lifecycle"
	"alex/internal/delivery/channels"
	agent "alex/internal/domain/agent/ports/agent"
	agentstorage "alex/internal/domain/agent/ports/storage"
	"alex/internal/domain/agent/presets"
	"alex/internal/infra/adapters"
	checkpointinfra "alex/internal/infra/checkpoint"
	codinginfra "alex/internal/infra/coding"
	"alex/internal/infra/external"
	"alex/internal/infra/external/teamrun"
	"alex/internal/infra/process"
	sessionstate "alex/internal/infra/session/state_store"
	toolspolicy "alex/internal/infra/tools"
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
func BuildContainer(config Config) (*Container, error) {
	builder := newContainerBuilder(config)
	return builder.Build()
}

func newContainerBuilder(config Config) *containerBuilder {
	logger := logging.NewComponentLogger("DI")
	sessionDir := resolveStorageDir(config.SessionDir, "~/.alex/sessions")
	costDir := resolveStorageDir(config.CostDir, "~/.alex/costs")
	if utils.IsBlank(config.ToolMode) {
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

	// Start CLI detection in background — runs concurrently with LLM factory + session init.
	cliCh := make(chan []codinginfra.LocalCLIDetection, 1)
	go func() { cliCh <- codinginfra.DetectLocalCLIs() }()

	llmFactory := b.buildLLMFactory()
	resources, err := b.buildSessionResources()
	if err != nil {
		return nil, err
	}
	taskStore, err := b.buildTaskStore()
	if err != nil {
		return nil, fmt.Errorf("build task store: %w", err)
	}

	// Collect CLI detection results (should be ready by now).
	detectedCLIs := <-cliCh
	b.applyDetectedExternalAgents(detectedCLIs, true)
	b.logLocalCodingCLIDetection(detectedCLIs)

	bgCtx, bgCancel := context.WithCancel(context.Background())
	buildOK := false
	defer func() {
		if !buildOK {
			bgCancel()
		}
	}()
	memoryEngine, err := b.buildMemoryEngine(bgCtx)
	if err != nil {
		return nil, err
	}
	contextOptions := []ctxmgr.Option{
		ctxmgr.WithStateStore(resources.stateStore),
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
	externalRegistry := external.NewRegistry(b.config.ExternalAgents, process.NewController(), b.logger)
	if len(externalRegistry.SupportedTypes()) > 0 {
		externalExecutor = codinginfra.NewManagedExternalExecutor(externalRegistry, b.logger)
	}

	hookRegistry := b.buildHookRegistry(memoryEngine, llmFactory)
	okrContextProvider := b.buildOKRContextProvider()
	checkpointStore := checkpointinfra.NewFileCheckpointStore(filepath.Join(b.sessionDir, "checkpoints"))
	teamRunRecorder, err := teamrun.NewFileRecorder(filepath.Join(b.sessionDir, "_team_runs"), b.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize team run recorder: %w", err)
	}
	credentialRefresher := buildCredentialRefresher()

	coordinator := agentcoordinator.NewAgentCoordinator(
		llmFactory,
		toolRegistry,
		resources.sessionStore,
		contextMgr,
		historyMgr,
		parserImpl,
		costTracker,
		b.buildAgentAppConfig(),
		agentcoordinator.WithHookRegistry(hookRegistry),
		agentcoordinator.WithExternalExecutor(externalExecutor),
		agentcoordinator.WithOKRContextProvider(okrContextProvider),
		agentcoordinator.WithCheckpointStore(checkpointStore),
		agentcoordinator.WithCredentialRefresher(credentialRefresher),
		agentcoordinator.WithToolSLACollector(toolSLACollector),
		agentcoordinator.WithChannelHints(channels.DefaultHints()),
		agentcoordinator.WithTeamDefinitions(convertTeamConfigs(b.config.ExternalAgents.Teams)),
		agentcoordinator.WithTeamRunRecorder(teamRunRecorder),
		agentcoordinator.WithAtomicWriter(adapters.NewOSAtomicWriter()),
	)

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
		TaskStore:        taskStore,
		config:           b.config,
		toolRegistry:     toolRegistry,
		llmFactory:       llmFactory,
		bgCancel:         bgCancel,
	}
	if drainable, ok := memoryEngine.(lifecycle.Drainable); ok {
		container.Drainables = append(container.Drainables, drainable)
	}
	if taskStoreCloser, ok := taskStore.(interface{ Close() }); ok {
		container.Drainables = append(container.Drainables, lifecycle.DrainFunc{
			DrainName: "task-store",
			Fn:        func(context.Context) { taskStoreCloser.Close() },
		})
	}

	buildOK = true
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
		agentType := utils.TrimLower(item.AgentType)
		var enabled *bool
		var binary *string
		switch agentType {
		case "codex":
			enabled = &b.config.ExternalAgents.Codex.Enabled
			binary = &b.config.ExternalAgents.Codex.Binary
		case "claude_code":
			enabled = &b.config.ExternalAgents.ClaudeCode.Enabled
			binary = &b.config.ExternalAgents.ClaudeCode.Binary
		case "kimi":
			enabled = &b.config.ExternalAgents.Kimi.Enabled
			binary = &b.config.ExternalAgents.Kimi.Binary
		default:
			continue
		}
		wasEnabled := *enabled
		if !wasEnabled {
			*enabled = true
		}
		changedBinary := false
		if shouldAdoptDetectedBinary(*binary, item.Binary) {
			changedBinary = *binary != item.Path
			*binary = item.Path
		}
		if log && (!wasEnabled || changedBinary) {
			b.logger.Info("Coding CLI auto-enable: agent_type=%s enabled with binary=%s", agentType, *binary)
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
	currentLower := utils.TrimLower(current)
	detectedLower := utils.TrimLower(detected)
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
	switch utils.TrimLower(agentType) {
	case "codex":
		return b.config.ExternalAgents.Codex.Enabled
	case "claude_code":
		return b.config.ExternalAgents.ClaudeCode.Enabled
	case "kimi":
		return b.config.ExternalAgents.Kimi.Enabled
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
				Name:              r.Name,
				AgentType:         r.AgentType,
				CapabilityProfile: r.CapabilityProfile,
				TargetCLI:         r.TargetCLI,
				PromptTemplate:    r.PromptTemplate,
				ExecutionMode:     r.ExecutionMode,
				AutonomyLevel:     r.AutonomyLevel,
				WorkspaceMode:     r.WorkspaceMode,
				Config:            r.Config,
				InheritContext:    r.InheritContext,
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
