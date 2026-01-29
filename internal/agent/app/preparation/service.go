package preparation

import (
	"context"
	"fmt"
	"strings"
	"time"

	appconfig "alex/internal/agent/app/config"
	appcontext "alex/internal/agent/app/context"
	"alex/internal/agent/app/cost"
	sessiontitle "alex/internal/agent/app/sessiontitle"
	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	llm "alex/internal/agent/ports/llm"
	storage "alex/internal/agent/ports/storage"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/agent/presets"
	"alex/internal/utils/clilatency"
	id "alex/internal/utils/id"
)

const (
	historyComposeSnippetLimit = 600
	historySummaryMaxTokens    = 320
	historySummaryLLMTimeout   = 4 * time.Second
	historySummaryIntent       = "user_history_summary"
	DefaultSystemPrompt        = "You are ALEX, a helpful AI coding assistant. Follow Plan → (Clarify if needed) → ReAct → Finalize. Always call plan() before any action tool call. If plan(complexity=\"complex\"), call clarify() before each task's first action tool call. If plan(complexity=\"simple\"), skip clarify unless you need user input. Prefer sandbox_browser_dom for browser work (selectors, click/fill/query); use sandbox_browser only for coordinate-based actions or when DOM actions fail. Only capture screenshots when needed; screenshots return a vision summary for guidance. If you hit login/2FA/CAPTCHA or any auth gate, call request_user with clear steps for the user to log in, then wait before continuing. Avoid emojis in responses unless the user explicitly requests them."
)

// ExecutionPreparationDeps enumerates the dependencies required by the preparation service.
type ExecutionPreparationDeps struct {
	LLMFactory     llm.LLMClientFactory
	ToolRegistry   tools.ToolRegistry
	SessionStore   storage.SessionStore
	ContextMgr     agent.ContextManager
	HistoryMgr     storage.HistoryManager
	Parser         tools.FunctionCallParser
	Config         appconfig.Config
	Logger         agent.Logger
	Clock          agent.Clock
	CostDecorator  *cost.CostTrackingDecorator
	PresetResolver *PresetResolver // Optional: if nil, one will be created
	EventEmitter   agent.EventListener
	CostTracker    storage.CostTracker
}

// ExecutionPreparationService prepares everything needed before executing a task.
type ExecutionPreparationService struct {
	llmFactory     llm.LLMClientFactory
	toolRegistry   tools.ToolRegistry
	sessionStore   storage.SessionStore
	contextMgr     agent.ContextManager
	historyMgr     storage.HistoryManager
	parser         tools.FunctionCallParser
	config         appconfig.Config
	logger         agent.Logger
	clock          agent.Clock
	costDecorator  *cost.CostTrackingDecorator
	presetResolver *PresetResolver
	eventEmitter   agent.EventListener
	costTracker    storage.CostTracker
}

// NewExecutionPreparationService creates a service instance.
func NewExecutionPreparationService(deps ExecutionPreparationDeps) *ExecutionPreparationService {
	logger := deps.Logger
	if logger == nil {
		logger = agent.NoopLogger{}
	}
	clock := deps.Clock
	if clock == nil {
		clock = agent.SystemClock{}
	}

	costDecorator := deps.CostDecorator
	if costDecorator == nil {
		costDecorator = cost.NewCostTrackingDecorator(nil, logger, clock)
	}

	eventEmitter := deps.EventEmitter
	if eventEmitter == nil {
		eventEmitter = agent.NoopEventListener{}
	}

	presetResolver := deps.PresetResolver
	if presetResolver == nil {
		presetResolver = NewPresetResolverWithDeps(PresetResolverDeps{
			Logger:       logger,
			Clock:        clock,
			EventEmitter: eventEmitter,
		})
	}

	return &ExecutionPreparationService{
		llmFactory:     deps.LLMFactory,
		toolRegistry:   deps.ToolRegistry,
		sessionStore:   deps.SessionStore,
		contextMgr:     deps.ContextMgr,
		historyMgr:     deps.HistoryMgr,
		parser:         deps.Parser,
		config:         deps.Config,
		logger:         logger,
		clock:          clock,
		costDecorator:  costDecorator,
		presetResolver: presetResolver,
		eventEmitter:   eventEmitter,
		costTracker:    deps.CostTracker,
	}
}

// Prepare builds the execution environment for a task.
func (s *ExecutionPreparationService) Prepare(ctx context.Context, task string, sessionID string) (*agent.ExecutionEnvironment, error) {
	s.logger.Info("PrepareExecution called: task='%s'", task)

	sessionLoadStarted := time.Now()
	session, err := s.loadSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	latencySessionID := sessionID
	if session != nil && session.ID != "" {
		latencySessionID = session.ID
	}
	clilatency.PrintfWithContext(ctx,
		"[latency] session_load_ms=%.2f session=%s\n",
		float64(time.Since(sessionLoadStarted))/float64(time.Millisecond),
		latencySessionID,
	)

	ids := id.IDsFromContext(ctx)

	historyLoadStarted := time.Now()
	sessionHistory := s.loadSessionHistory(ctx, session)
	clilatency.PrintfWithContext(ctx,
		"[latency] session_history_ms=%.2f messages=%d\n",
		float64(time.Since(historyLoadStarted))/float64(time.Millisecond),
		len(sessionHistory),
	)
	rawHistory := agent.CloneMessages(sessionHistory)
	if session != nil {
		session.Messages = sessionHistory
	}

	var inheritedState *agent.TaskState
	if appcontext.IsSubagentContext(ctx) {
		inheritedState = agent.GetTaskStateSnapshot(ctx)
	}

	var (
		initialWorldState    map[string]any
		initialWorldDiff     map[string]any
		window               agent.ContextWindow
		contextWasCompressed bool
	)

	toolMode := presets.ToolMode(strings.TrimSpace(s.config.ToolMode))
	if toolMode == "" {
		toolMode = presets.ToolModeCLI
	}
	toolPreset := strings.TrimSpace(s.config.ToolPreset)
	if s.presetResolver != nil {
		if resolved, source := s.presetResolver.resolveToolPreset(ctx, toolMode, toolPreset); resolved != "" {
			toolPreset = resolved
			s.logger.Info("Using tool preset %s (source=%s)", resolved, source)
		}
	}
	if toolMode == presets.ToolModeCLI && toolPreset == "" {
		toolPreset = string(presets.ToolPresetFull)
	}
	personaKey := s.config.AgentPreset
	if s.presetResolver != nil {
		if preset, source := s.presetResolver.resolveAgentPreset(ctx, s.config.AgentPreset); preset != "" {
			personaKey = preset
			s.logger.Info("Using persona preset %s (source=%s)", preset, source)
		}
	}
	if s.contextMgr != nil {
		if skip, reason := shouldSkipContextWindow(task, session); skip {
			clilatency.PrintfWithContext(ctx, "[latency] context_window=skipped reason=%s\n", reason)
			window.Messages = session.Messages
			window.SystemPrompt = DefaultSystemPrompt
		} else {
			originalCount := len(session.Messages)
			windowStarted := time.Now()
			var err error
			window, err = s.contextMgr.BuildWindow(ctx, session, agent.ContextWindowConfig{
				TokenLimit:         s.config.MaxTokens,
				PersonaKey:         personaKey,
				ToolMode:           string(toolMode),
				ToolPreset:         toolPreset,
				EnvironmentSummary: s.config.EnvironmentSummary,
			})
			if err != nil {
				return nil, fmt.Errorf("build context window: %w", err)
			}
			clilatency.PrintfWithContext(ctx,
				"[latency] context_window_ms=%.2f original=%d final=%d\n",
				float64(time.Since(windowStarted))/float64(time.Millisecond),
				originalCount,
				len(window.Messages),
			)
			session.Messages = window.Messages
			initialWorldState, initialWorldDiff = buildWorldStateFromWindow(window)
			if compressedCount := len(window.Messages); compressedCount < originalCount {
				contextWasCompressed = true
				compressionEvent := domain.NewWorkflowDiagnosticContextCompressionEvent(
					agent.LevelCore,
					session.ID,
					ids.RunID,
					ids.ParentRunID,
					originalCount,
					compressedCount,
					s.clock.Now(),
				)
				s.eventEmitter.OnEvent(compressionEvent)
			}
		}
	}
	systemPrompt := strings.TrimSpace(window.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = DefaultSystemPrompt
	}
	if toolMode == presets.ToolModeCLI {
		systemPrompt = strings.TrimSpace(systemPrompt + `

## File Outputs
- When producing long-form deliverables (reports, articles, specs), write them to a Markdown file via file_write.
- Provide a short summary in the final answer and point the user to the generated file path instead of pasting the full content.`)
	} else {
		systemPrompt = strings.TrimSpace(systemPrompt + `

## Artifacts & Attachments
- When producing long-form deliverables (reports, articles, specs), write them to a Markdown artifact via artifacts_write.
- Provide a short summary in the final answer and point the user to the generated file instead of pasting the full content.
- Keep attachment placeholders out of the main body; list them at the end of the final answer.
- If you want clients to render an attachment card, reference the file with a placeholder like [report.md].`)
	}
	if runID := strings.TrimSpace(ids.RunID); runID != "" {
		systemPrompt = strings.TrimSpace(systemPrompt +
			"\n\n## Runtime Identifiers\n" +
			fmt.Sprintf("- run_id: %s\n", runID) +
			"- Use this exact run_id for plan() (and clarify() if used).")
	}

	preloadedAttachments := collectSessionAttachments(session)
	preloadedImportant := collectSessionImportant(session)
	inheritedAttachments, inheritedIterations := appcontext.GetInheritedAttachments(ctx)
	if len(inheritedAttachments) > 0 {
		if preloadedAttachments == nil {
			preloadedAttachments = make(map[string]ports.Attachment)
		}
		for key, att := range inheritedAttachments {
			name := strings.TrimSpace(key)
			if name == "" {
				name = strings.TrimSpace(att.Name)
			}
			if name == "" {
				continue
			}
			if _, exists := preloadedAttachments[name]; exists {
				continue
			}
			if att.Name == "" {
				att.Name = name
			}
			preloadedAttachments[name] = att
		}
	}
	if inheritedState != nil && len(inheritedState.Attachments) > 0 {
		if preloadedAttachments == nil {
			preloadedAttachments = make(map[string]ports.Attachment)
		}
		mergeAttachmentMaps(preloadedAttachments, inheritedState.Attachments)
	}
	if inheritedState != nil && len(inheritedState.Important) > 0 {
		mergeImportantNotes(preloadedImportant, inheritedState.Important)
	}
	if contextWasCompressed && len(preloadedImportant) > 0 {
		if msg := buildImportantNotesMessage(preloadedImportant); msg != nil {
			window.Messages = append(window.Messages, *msg)
			session.Messages = append(session.Messages, *msg)
		}
	}

	selection, hasSelection := appcontext.GetLLMSelection(ctx)
	selectionPinned := hasSelection &&
		selection.Pinned &&
		strings.TrimSpace(selection.Provider) != "" &&
		strings.TrimSpace(selection.Model) != ""

	var taskAnalysis *agent.TaskAnalysis
	var preferSmallModel bool
	if selectionPinned {
		if analysis, _, ok := quickTriageTask(task); ok {
			taskAnalysis = analysis
		}
	} else {
		taskAnalysis, preferSmallModel = s.preAnalyzeTask(ctx, session, task)
	}
	if session != nil && taskAnalysis != nil {
		if session.Metadata == nil {
			session.Metadata = make(map[string]string)
		}
		if strings.TrimSpace(session.Metadata["title"]) == "" {
			if title := sessiontitle.NormalizeSessionTitle(taskAnalysis.ActionName); title != "" {
				session.Metadata["title"] = title
			}
		}
	}

	effectiveModel := s.config.LLMModel
	effectiveProvider := s.config.LLMProvider
	if selectionPinned {
		effectiveProvider = selection.Provider
		effectiveModel = selection.Model
	} else if preferSmallModel && strings.TrimSpace(s.config.LLMSmallModel) != "" {
		effectiveProvider = strings.TrimSpace(s.config.LLMSmallProvider)
		if effectiveProvider == "" {
			effectiveProvider = s.config.LLMProvider
		}
		effectiveModel = s.config.LLMSmallModel
	}
	if !selectionPinned && taskNeedsVision(task, preloadedAttachments, appcontext.GetUserAttachments(ctx)) {
		if visionModel := strings.TrimSpace(s.config.LLMVisionModel); visionModel != "" {
			effectiveProvider = s.config.LLMProvider
			effectiveModel = visionModel
		}
	}

	s.logger.Debug("Getting isolated LLM client: provider=%s, model=%s", effectiveProvider, effectiveModel)
	// Use GetIsolatedClient to ensure session-level cost tracking isolation
	llmInitStarted := time.Now()
	llmConfig := llm.LLMConfig{
		APIKey:  s.config.APIKey,
		BaseURL: s.config.BaseURL,
	}
	if selectionPinned {
		llmConfig.APIKey = selection.APIKey
		llmConfig.BaseURL = selection.BaseURL
		if len(selection.Headers) > 0 {
			llmConfig.Headers = selection.Headers
		}
	}
	llmClient, err := s.llmFactory.GetIsolatedClient(effectiveProvider, effectiveModel, llmConfig)
	clilatency.PrintfWithContext(ctx,
		"[latency] llm_client_init_ms=%.2f provider=%s model=%s\n",
		float64(time.Since(llmInitStarted))/float64(time.Millisecond),
		strings.TrimSpace(effectiveProvider),
		strings.TrimSpace(effectiveModel),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get LLM client: %w", err)
	}
	s.logger.Debug("Isolated LLM client obtained successfully")

	// Use Wrap instead of Attach to avoid modifying shared client state
	llmClient = s.costDecorator.Wrap(ctx, session.ID, llmClient)

	streamingClient, ok := llm.EnsureStreamingClient(llmClient).(llm.StreamingLLMClient)
	if !ok {
		return nil, fmt.Errorf("failed to wrap LLM client with streaming support")
	}

	history := s.recallUserHistory(ctx, llmClient, task, rawHistory)
	stateMessages := append([]domain.Message(nil), session.Messages...)
	if history != nil && len(history.messages) > 0 {
		stateMessages = history.messages
	}

	state := &domain.TaskState{
		SystemPrompt:         systemPrompt,
		Messages:             stateMessages,
		SessionID:            session.ID,
		RunID:                ids.RunID,
		ParentRunID:          ids.ParentRunID,
		Attachments:          preloadedAttachments,
		AttachmentIterations: make(map[string]int),
		Important:            preloadedImportant,
		Plans:                nil,
		Beliefs:              nil,
		KnowledgeRefs:        nil,
		WorldState:           initialWorldState,
		WorldDiff:            initialWorldDiff,
	}
	for key := range preloadedAttachments {
		if key == "" {
			continue
		}
		state.AttachmentIterations[key] = 0
	}
	if len(inheritedIterations) > 0 {
		for key, iter := range inheritedIterations {
			name := strings.TrimSpace(key)
			if name == "" {
				continue
			}
			state.AttachmentIterations[name] = iter
		}
	}

	if inheritedState != nil {
		s.applyInheritedStateSnapshot(state, inheritedState)
	}

	if userAttachments := appcontext.GetUserAttachments(ctx); len(userAttachments) > 0 {
		if state.Attachments == nil {
			state.Attachments = make(map[string]ports.Attachment)
		}
		pending := make(map[string]ports.Attachment)
		for _, att := range userAttachments {
			name := strings.TrimSpace(att.Name)
			if name == "" {
				continue
			}
			att.Name = name
			if att.Source == "" {
				att.Source = "user_upload"
			}
			state.Attachments[name] = att
			pending[name] = att
		}
		if len(pending) > 0 {
			state.PendingUserAttachments = pending
		}
	}

	toolRegistry := s.selectToolRegistry(ctx, toolMode, toolPreset)
	services := domain.Services{
		LLM:          streamingClient,
		ToolExecutor: toolRegistry,
		ToolLimiter:  NewToolExecutionLimiter(s.config.ToolMaxConcurrent),
		Parser:       s.parser,
		Context:      s.contextMgr,
	}

	s.logger.Info("Execution environment prepared successfully")

	return &agent.ExecutionEnvironment{
		State:        state,
		Services:     services,
		Session:      session,
		SystemPrompt: systemPrompt,
		TaskAnalysis: taskAnalysis,
	}, nil
}
