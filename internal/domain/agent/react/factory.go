package react

import (
	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
)

// NewReactEngine creates a new ReAct engine with injected infrastructure dependencies.
func NewReactEngine(cfg ReactEngineConfig) *ReactEngine {
	logger := cfg.Logger
	if logger == nil {
		logger = agent.NoopLogger{}
	}

	clock := cfg.Clock
	if clock == nil {
		clock = agent.SystemClock{}
	}
	idGenerator := cfg.IDGenerator
	if idGenerator == nil {
		idGenerator = defaultIDGenerator{}
	}
	idContextReader := cfg.IDContextReader
	if idContextReader == nil {
		idContextReader = defaultIDContextReader{}
	}
	latencyReporter := cfg.LatencyReporter
	if latencyReporter == nil {
		latencyReporter = defaultLatencyReporter{}
	}
	jsonCodec := cfg.JSONCodec
	if jsonCodec == nil {
		jsonCodec = defaultJSONCodec{}
	}
	goRunner := cfg.GoRunner
	if goRunner == nil {
		goRunner = defaultGoRunner{}
	}
	workingDirResolver := cfg.WorkingDirResolver
	if workingDirResolver == nil {
		workingDirResolver = defaultWorkingDirResolver{}
	}
	workspaceMgrFactory := cfg.WorkspaceMgrFactory
	if workspaceMgrFactory == nil {
		workspaceMgrFactory = defaultWorkspaceManagerFactory{}
	}

	stopReasons := cfg.StopReasons
	if len(stopReasons) == 0 {
		stopReasons = []string{"final_answer", "done", "complete"}
	}

	maxIterations := cfg.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 1
	}

	completion := buildCompletionDefaults(cfg.CompletionDefaults)

	finalReview := cfg.FinalAnswerReview
	if finalReview.MaxExtraIterations <= 0 {
		finalReview.MaxExtraIterations = 1
	}
	domain.SetEventIDGenerator(idGenerator)

	return &ReactEngine{
		maxIterations:       maxIterations,
		stopReasons:         stopReasons,
		logger:              logger,
		clock:               clock,
		idGenerator:         idGenerator,
		idContextReader:     idContextReader,
		latencyReporter:     latencyReporter,
		jsonCodec:           jsonCodec,
		goRunner:            goRunner,
		workingDirResolver:  workingDirResolver,
		workspaceMgrFactory: workspaceMgrFactory,
		eventListener:       cfg.EventListener,
		completion:          completion,
		finalAnswerReview:   finalReview,
		attachmentMigrator:  cfg.AttachmentMigrator,
		attachmentPersister: cfg.AttachmentPersister,
		checkpointStore:     cfg.CheckpointStore,
		workflow:            cfg.Workflow,
		iterationHook:       cfg.IterationHook,
		backgroundExecutor:  cfg.BackgroundExecutor,
		backgroundManager:   cfg.BackgroundManager,
		externalExecutor:    cfg.ExternalExecutor,
	}
}

func buildCompletionDefaults(cfg CompletionDefaults) completionConfig {
	temperature := 0.7
	if cfg.Temperature != nil {
		temperature = *cfg.Temperature
	}

	maxTokens := 12000
	if cfg.MaxTokens != nil && *cfg.MaxTokens > 0 {
		maxTokens = *cfg.MaxTokens
	}

	topP := 1.0
	if cfg.TopP != nil {
		topP = *cfg.TopP
	}

	stopSequences := make([]string, len(cfg.StopSequences))
	copy(stopSequences, cfg.StopSequences)

	return completionConfig{
		temperature:   temperature,
		maxTokens:     maxTokens,
		topP:          topP,
		stopSequences: stopSequences,
	}
}
