package preparation

import (
	"context"
	"fmt"
	"strings"
	"time"

	appconfig "alex/internal/app/agent/config"
	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/agent/cost"
	sessiontitle "alex/internal/app/agent/sessiontitle"
	"alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	llm "alex/internal/domain/agent/ports/llm"
	storage "alex/internal/domain/agent/ports/storage"
	tools "alex/internal/domain/agent/ports/tools"
	toolspolicy "alex/internal/infra/tools"
	"alex/internal/shared/agent/presets"
	"alex/internal/shared/async"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/utils/clilatency"
	id "alex/internal/shared/utils/id"
)

const (
	historyComposeSnippetLimit = 600
	historySummaryMaxTokens    = 320
	historySummaryLLMTimeout   = 4 * time.Second
	historySummaryIntent       = "user_history_summary"
	DefaultSystemPrompt        = "You are eli, a helpful AI coding assistant. Use plan() to set a visible goal header (optional). Use clarify(needs_user_input=true) only when requirements are missing/contradictory or a user answer is required; do not use clarify for explicit operational asks. For explicit approval/consent/manual gates (login, 2FA, CAPTCHA, external confirmation), call request_user with clear steps and wait. Prefer browser_info for read-only browser state, browser_dom for selector-based browser work, and browser_action only for coordinate-based actions or when DOM actions fail. Use web_search when no URL is fixed and source discovery is needed; use web_fetch after a URL is chosen. Use lark_chat_history for prior thread context, lark_send_message for text-only updates, and lark_upload_file only when an actual file must be delivered. Use artifacts_list for inventory and artifacts_write for creating/updating durable outputs. Use write_attachment only to materialize an existing attachment into a downloadable file path. Only capture screenshots when explicit visual proof is needed; screenshots return a vision summary for guidance. Avoid emojis in responses unless the user explicitly requests them."
)

// CredentialRefresher resolves fresh API credentials for a given LLM provider.
// Returns the api key, base URL, and whether resolution succeeded.
// Used by long-running servers (e.g. Lark) to re-resolve CLI credentials
// that may have been refreshed since startup.
type CredentialRefresher func(provider string) (apiKey, baseURL string, ok bool)

// ExecutionPreparationDeps enumerates the dependencies required by the preparation service.
type ExecutionPreparationDeps struct {
	LLMFactory          llm.LLMClientFactory
	ToolRegistry        tools.ToolRegistry
	SessionStore        storage.SessionStore
	ContextMgr          agent.ContextManager
	HistoryMgr          storage.HistoryManager
	Parser              tools.FunctionCallParser
	Config              appconfig.Config
	Logger              agent.Logger
	Clock               agent.Clock
	CostDecorator       *cost.CostTrackingDecorator
	PresetResolver      *PresetResolver // Optional: if nil, one will be created
	EventEmitter        agent.EventListener
	CostTracker         storage.CostTracker
	OKRContextProvider  OKRContextProvider  // Optional: provides OKR context for system prompt
	CredentialRefresher CredentialRefresher // Optional: re-resolves CLI credentials at task time
}

// ExecutionPreparationService prepares everything needed before executing a task.
type ExecutionPreparationService struct {
	llmFactory          llm.LLMClientFactory
	toolRegistry        tools.ToolRegistry
	sessionStore        storage.SessionStore
	contextMgr          agent.ContextManager
	historyMgr          storage.HistoryManager
	parser              tools.FunctionCallParser
	config              appconfig.Config
	logger              agent.Logger
	clock               agent.Clock
	costDecorator       *cost.CostTrackingDecorator
	toolPolicy          toolspolicy.ToolPolicy
	presetResolver      *PresetResolver
	eventEmitter        agent.EventListener
	costTracker         storage.CostTracker
	okrContextProvider  OKRContextProvider
	credentialRefresher CredentialRefresher
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

	toolPolicy := toolspolicy.NewToolPolicy(deps.Config.ToolPolicy)

	return &ExecutionPreparationService{
		llmFactory:          deps.LLMFactory,
		toolRegistry:        deps.ToolRegistry,
		sessionStore:        deps.SessionStore,
		contextMgr:          deps.ContextMgr,
		historyMgr:          deps.HistoryMgr,
		parser:              deps.Parser,
		config:              deps.Config,
		logger:              logger,
		clock:               clock,
		costDecorator:       costDecorator,
		toolPolicy:          toolPolicy,
		presetResolver:      presetResolver,
		eventEmitter:        eventEmitter,
		costTracker:         deps.CostTracker,
		okrContextProvider:  deps.OKRContextProvider,
		credentialRefresher: deps.CredentialRefresher,
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
			var okrContext string
			if s.okrContextProvider != nil {
				okrContext = s.okrContextProvider()
			}
			var err error
			window, err = s.contextMgr.BuildWindow(ctx, session, agent.ContextWindowConfig{
				TokenLimit:         s.config.MaxTokens,
				PersonaKey:         personaKey,
				ToolMode:           string(toolMode),
				ToolPreset:         toolPreset,
				EnvironmentSummary: s.config.EnvironmentSummary,
				TaskInput:          task,
				Skills:             buildSkillsConfig(s.config.Proactive.Skills),
				OKRContext:         okrContext,
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
- When producing long-form deliverables (reports, articles, specs), write them to a Markdown file via write_file.
- Provide a short summary in the final answer and point the user to the generated file path instead of pasting the full content.`)
	} else {
		systemPrompt = strings.TrimSpace(systemPrompt + `

## Artifacts & Attachments
- When producing long-form deliverables (reports, articles, specs), write them to a Markdown artifact via artifacts_write.
- Provide a short summary in the final answer and point the user to the generated file instead of pasting the full content.
- Keep attachment placeholders out of the main body; list them at the end of the final answer.
- If you want clients to render an attachment card, reference the file with a placeholder like [report.md].`)
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
		// quickTriageTask is instant (no LLM call); use it for synchronous routing.
		if analysis, preferSmall, ok := quickTriageTask(task); ok {
			taskAnalysis = analysis
			preferSmallModel = preferSmall
		} else {
			// Fire LLM-based pre-analysis asynchronously to avoid blocking the
			// prepare phase with a small-model round-trip (up to 4 s).
			// The title will be persisted in the background when the call completes.
			s.preAnalyzeTaskAsync(ctx, session, task)
		}
	}
	if taskAnalysis != nil && taskAnalysis.ReactEmoji != "" {
		s.eventEmitter.OnEvent(domain.NewWorkflowPreAnalysisEmojiEvent(
			agent.LevelCore,
			session.ID,
			ids.RunID,
			ids.ParentRunID,
			taskAnalysis.ReactEmoji,
			s.clock.Now(),
		))
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
	// Re-resolve CLI credentials at task time for providers that support
	// token refresh (e.g. Codex). This keeps long-running
	// servers (Lark) working even after the initial startup token expires.
	if !selectionPinned && s.credentialRefresher != nil {
		if apiKey, baseURL, ok := s.credentialRefresher(effectiveProvider); ok {
			llmConfig.APIKey = apiKey
			if baseURL != "" {
				llmConfig.BaseURL = baseURL
			}
		}
	}
	if selectionPinned {
		llmConfig.APIKey = selection.APIKey
		llmConfig.BaseURL = selection.BaseURL
		if len(selection.Headers) > 0 {
			llmConfig.Headers = selection.Headers
		}
		// When the pinned selection resolved with an empty API key (e.g.
		// expired CLI token), try the credential refresher for the selected
		// provider so that long-running Lark servers can recover automatically.
		if llmConfig.APIKey == "" && s.credentialRefresher != nil {
			if apiKey, baseURL, ok := s.credentialRefresher(effectiveProvider); ok {
				llmConfig.APIKey = apiKey
				if baseURL != "" {
					llmConfig.BaseURL = baseURL
				}
			}
		}
	}
	apiKeySource := "config"
	if selectionPinned {
		apiKeySource = "pinned_selection"
	} else if s.credentialRefresher != nil {
		apiKeySource = "credential_refresher"
	}
	s.logger.Debug("LLM config resolved: provider=%s model=%s pinned=%t api_key_source=%s key_prefix=%s",
		effectiveProvider, effectiveModel, selectionPinned, apiKeySource, safeKeyPrefix(llmConfig.APIKey))
	if mismatch, detail := detectKeyProviderMismatch(effectiveProvider, llmConfig.APIKey); mismatch {
		s.logger.Warn("API key may not match provider %s: %s", effectiveProvider, detail)
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
		PlanReviewEnabled:    appcontext.PlanReviewEnabled(ctx),
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

// preAnalyzeTaskAsync fires preAnalyzeTask in a background goroutine and
// persists the resulting title to the session store when done. This removes
// the small-model round-trip from the critical path of Prepare().
func (s *ExecutionPreparationService) preAnalyzeTaskAsync(ctx context.Context, session *storage.Session, task string) {
	if session == nil || strings.TrimSpace(session.ID) == "" {
		return
	}
	if session.Metadata != nil && strings.TrimSpace(session.Metadata["title"]) != "" {
		return
	}

	sessionID := session.ID
	ids := id.IDsFromContext(ctx)

	// Snapshot a minimal session for the goroutine so it doesn't race with the
	// caller mutating the original session object.
	sessionSnapshot := &storage.Session{
		ID:       sessionID,
		Metadata: map[string]string{},
	}

	bgCtx := context.Background()
	if logID := id.LogIDFromContext(ctx); logID != "" {
		bgCtx = id.WithLogID(bgCtx, logID)
	}

	async.Go(s.logger, "preanalysis-title", func() {
		analysis, _ := s.preAnalyzeTask(bgCtx, sessionSnapshot, task)
		if analysis == nil {
			return
		}
		if analysis.ReactEmoji != "" {
			s.eventEmitter.OnEvent(domain.NewWorkflowPreAnalysisEmojiEvent(
				agent.LevelCore,
				sessionID,
				ids.RunID,
				ids.ParentRunID,
				analysis.ReactEmoji,
				s.clock.Now(),
			))
		}
		title := sessiontitle.NormalizeSessionTitle(analysis.ActionName)
		if title == "" {
			return
		}

		persistCtx, cancel := context.WithTimeout(bgCtx, 2*time.Second)
		defer cancel()

		sess, err := s.sessionStore.Get(persistCtx, sessionID)
		if err != nil {
			s.logger.Warn("Async title: failed to load session: %v", err)
			return
		}
		if sess.Metadata == nil {
			sess.Metadata = make(map[string]string)
		}
		if strings.TrimSpace(sess.Metadata["title"]) != "" {
			return // Title already set by plan tool or elsewhere.
		}
		sess.Metadata["title"] = title
		if err := s.sessionStore.Save(persistCtx, sess); err != nil {
			s.logger.Warn("Async title: failed to persist: %v", err)
		}
	})
}

// safeKeyPrefix returns a short, safe-to-log prefix of an API key.
func safeKeyPrefix(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:8] + "..."
}

// detectKeyProviderMismatch checks for obvious API key / provider mismatches.
// It detects known vendor-specific key prefixes being sent to the wrong provider.
func detectKeyProviderMismatch(provider, apiKey string) (mismatch bool, detail string) {
	if apiKey == "" {
		return false, ""
	}
	lower := strings.ToLower(provider)
	prefix := safeKeyPrefix(apiKey)

	// Known non-OpenAI key prefixes that should never be sent to OpenAI/Codex.
	knownNonOpenAI := []string{"sk-kimi-", "sk-ant-", "sk-deepseek-"}
	if lower == "codex" || lower == "openai-responses" || lower == "openai" {
		for _, bad := range knownNonOpenAI {
			if strings.HasPrefix(apiKey, bad) {
				return true, fmt.Sprintf("key prefix=%s looks like a %s key, not valid for provider %s",
					prefix, strings.TrimSuffix(strings.TrimPrefix(bad, "sk-"), "-"), provider)
			}
		}
	}
	// Anthropic keys start with sk-ant-.
	if lower == "anthropic" && !strings.HasPrefix(apiKey, "sk-ant-") {
		return true, fmt.Sprintf("key prefix=%s expected sk-ant-* for provider %s", prefix, provider)
	}
	return false, ""
}

func buildSkillsConfig(cfg runtimeconfig.SkillsConfig) agent.SkillsConfig {
	return agent.SkillsConfig{
		AutoActivation: agent.SkillAutoActivationConfig{
			Enabled:             cfg.AutoActivation.Enabled,
			MaxActivated:        cfg.AutoActivation.MaxActivated,
			TokenBudget:         cfg.AutoActivation.TokenBudget,
			ConfidenceThreshold: cfg.AutoActivation.ConfidenceThreshold,
			CacheTTLSeconds:     cfg.CacheTTLSeconds,
			FallbackToIndex:     true,
		},
		Feedback: agent.SkillFeedbackConfig{
			Enabled:   cfg.Feedback.Enabled,
			StorePath: cfg.Feedback.StorePath,
		},
	}
}
