package preparation

import (
	"context"
	"fmt"
	"strings"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/agent/llmclient"
	domain "alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	llm "alex/internal/domain/agent/ports/llm"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/domain/agent/presets"
	runtimeconfig "alex/internal/shared/config"
	utils "alex/internal/shared/utils"
	"alex/internal/shared/utils/clilatency"
	id "alex/internal/shared/utils/id"
)

// prepareContext holds intermediate state accumulated across Prepare phases.
type prepareContext struct {
	ctx       context.Context
	task      string
	sessionID string
	ids       id.IDs

	session        *storage.Session
	sessionHistory []domain.Message
	rawHistory     []domain.Message

	inheritedState       *agent.TaskState
	preloadedAttachments map[string]ports.Attachment
	preloadedImportant   map[string]ports.ImportantNote
	inheritedIterations  map[string]int
	userAttachments      []ports.Attachment

	toolMode    presets.ToolMode
	toolPreset  string
	personaKey  string

	selectionPinned  bool
	effectiveProfile runtimeconfig.LLMProfile
	taskAnalysis     *agent.TaskAnalysis

	initialCognitive     *agent.CognitiveExtension
	window               agent.ContextWindow
	contextWasCompressed bool

	llmClient       llm.LLMClient
	streamingClient llm.StreamingLLMClient
}

// Prepare builds the execution environment for a task.
func (s *ExecutionPreparationService) Prepare(ctx context.Context, task string, sessionID string) (*agent.ExecutionEnvironment, error) {
	s.logger.Info("PrepareExecution called: task='%s'", task)

	pc := &prepareContext{ctx: ctx, task: task, sessionID: sessionID}
	pc.ids = id.IDsFromContext(ctx)

	if err := s.loadSessionAndHistory(pc); err != nil {
		return nil, err
	}

	s.collectPreloadedState(pc)
	s.resolvePresetsAndAnalysis(pc)
	s.resolveLLMProfile(pc)

	if err := s.initParallelDeps(pc); err != nil {
		return nil, err
	}

	state := s.assembleTaskState(pc)
	services := s.assembleServices(pc)

	s.logger.Info("Execution environment prepared successfully")

	return &agent.ExecutionEnvironment{
		State:        state,
		Services:     services,
		Session:      pc.session,
		SystemPrompt: state.SystemPrompt,
		TaskAnalysis: pc.taskAnalysis,
	}, nil
}

// loadSessionAndHistory loads the session and its history, producing rawHistory clone.
func (s *ExecutionPreparationService) loadSessionAndHistory(pc *prepareContext) error {
	sessionLoadStarted := time.Now()
	session, err := s.loadSession(pc.ctx, pc.sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	latencySessionID := pc.sessionID
	if session != nil && session.ID != "" {
		latencySessionID = session.ID
	}
	clilatency.PrintfWithContext(pc.ctx,
		"[latency] session_load_ms=%.2f session=%s\n",
		float64(time.Since(sessionLoadStarted))/float64(time.Millisecond),
		latencySessionID,
	)

	historyLoadStarted := time.Now()
	sessionHistory := s.loadSessionHistory(pc.ctx, session)
	clilatency.PrintfWithContext(pc.ctx,
		"[latency] session_history_ms=%.2f messages=%d\n",
		float64(time.Since(historyLoadStarted))/float64(time.Millisecond),
		len(sessionHistory),
	)

	pc.rawHistory = agent.CloneMessages(sessionHistory)
	if session != nil {
		session.Messages = sessionHistory
	}

	pc.session = session
	pc.sessionHistory = sessionHistory
	return nil
}

// collectPreloadedState gathers inherited state, attachments, and important notes.
func (s *ExecutionPreparationService) collectPreloadedState(pc *prepareContext) {
	if appcontext.IsSubagentContext(pc.ctx) {
		pc.inheritedState = agent.GetTaskStateSnapshot(pc.ctx)
	}

	pc.preloadedAttachments = collectSessionAttachments(pc.session)
	pc.preloadedImportant = collectSessionImportant(pc.session)

	inheritedAttachments, inheritedIterations := appcontext.GetInheritedAttachments(pc.ctx)
	pc.inheritedIterations = inheritedIterations

	if len(inheritedAttachments) > 0 {
		if pc.preloadedAttachments == nil {
			pc.preloadedAttachments = make(map[string]ports.Attachment)
		}
		for key, att := range inheritedAttachments {
			name := strings.TrimSpace(key)
			if name == "" {
				name = strings.TrimSpace(att.Name)
			}
			if name == "" {
				continue
			}
			if _, exists := pc.preloadedAttachments[name]; exists {
				continue
			}
			if att.Name == "" {
				att.Name = name
			}
			pc.preloadedAttachments[name] = att
		}
	}
	if pc.inheritedState != nil && len(pc.inheritedState.Attachments) > 0 {
		if pc.preloadedAttachments == nil {
			pc.preloadedAttachments = make(map[string]ports.Attachment)
		}
		mergeAttachmentMaps(pc.preloadedAttachments, pc.inheritedState.Attachments)
	}
	if pc.inheritedState != nil && len(pc.inheritedState.Important) > 0 {
		mergeImportantNotes(pc.preloadedImportant, pc.inheritedState.Important)
	}

	pc.userAttachments = appcontext.GetUserAttachments(pc.ctx)
}

// resolvePresetsAndAnalysis resolves tool/agent presets, performs task analysis,
// and emits the pre-analysis emoji event if applicable.
func (s *ExecutionPreparationService) resolvePresetsAndAnalysis(pc *prepareContext) {
	pc.toolMode = presets.NormalizeToolMode(s.config.ToolMode)
	pc.toolPreset = presets.DefaultToolPresetForMode(pc.toolMode, s.config.ToolPreset)
	if resolved, source := s.presetResolver.resolveToolPreset(pc.ctx, pc.toolMode, pc.toolPreset); resolved != "" {
		pc.toolPreset = strings.TrimSpace(resolved)
		s.logger.Info("Using tool preset %s (source=%s)", resolved, source)
	}
	pc.toolPreset = presets.DefaultToolPresetForMode(pc.toolMode, pc.toolPreset)

	pc.personaKey = s.config.AgentPreset
	if preset, source := s.presetResolver.resolveAgentPreset(pc.ctx, s.config.AgentPreset); preset != "" {
		pc.personaKey = preset
		s.logger.Info("Using persona preset %s (source=%s)", preset, source)
	}

	selection, hasSelection := appcontext.GetLLMSelection(pc.ctx)
	pc.selectionPinned = hasSelection &&
		selection.Pinned &&
		utils.HasContent(selection.Provider) &&
		utils.HasContent(selection.Model)

	if pc.selectionPinned {
		if analysis, ok := quickTriageTask(pc.task); ok {
			pc.taskAnalysis = analysis
		}
	} else {
		if analysis, ok := quickTriageTask(pc.task); ok {
			pc.taskAnalysis = analysis
		} else {
			s.preAnalyzeTaskAsync(pc.ctx, pc.session, pc.task)
		}
	}

	if pc.taskAnalysis != nil && pc.taskAnalysis.ReactEmoji != "" {
		s.eventEmitter.OnEvent(domain.NewPreAnalysisEmojiEvent(
			agent.LevelCore,
			pc.session.ID,
			pc.ids.RunID,
			pc.ids.ParentRunID,
			pc.taskAnalysis.ReactEmoji,
			s.clock.Now(),
		))
	}
	if pc.session != nil && pc.taskAnalysis != nil {
		if pc.session.Metadata == nil {
			pc.session.Metadata = make(map[string]string)
		}
		if utils.IsBlank(pc.session.Metadata["title"]) {
			if title := utils.NormalizeSessionTitle(pc.taskAnalysis.ActionName); title != "" {
				pc.session.Metadata["title"] = title
			}
		}
	}
}

// resolveLLMProfile determines the effective LLM profile, applying pinned
// selection or vision overrides as needed.
func (s *ExecutionPreparationService) resolveLLMProfile(pc *prepareContext) {
	selection, _ := appcontext.GetLLMSelection(pc.ctx)

	pc.effectiveProfile = s.config.DefaultLLMProfile()
	if pc.selectionPinned {
		pc.effectiveProfile.Provider = selection.Provider
		pc.effectiveProfile.Model = selection.Model
		pc.effectiveProfile.APIKey = selection.APIKey
		pc.effectiveProfile.BaseURL = selection.BaseURL
		pc.effectiveProfile.Headers = cloneHeaders(selection.Headers)
	}
	if !pc.selectionPinned && taskNeedsVision(pc.task, pc.preloadedAttachments, pc.userAttachments) {
		if visionProfile, ok := s.config.VisionLLMProfile(); ok {
			pc.effectiveProfile.Provider = visionProfile.Provider
			pc.effectiveProfile.Model = visionProfile.Model
		}
	}
}

// initParallelDeps concurrently builds the context window and initialises the
// LLM client, returning the first error (if any).
func (s *ExecutionPreparationService) initParallelDeps(pc *prepareContext) error {
	prepareCtx, cancelPrepare := context.WithCancel(pc.ctx)
	defer cancelPrepare()
	prepareErrs := make(chan error, 2)

	go s.buildContextWindow(prepareCtx, pc, prepareErrs)
	go s.initLLMClient(prepareCtx, pc, prepareErrs)

	var prepareErr error
	for i := 0; i < 2; i++ {
		if err := <-prepareErrs; err != nil && prepareErr == nil {
			prepareErr = err
			cancelPrepare()
		}
	}
	return prepareErr
}

// buildContextWindow runs in a goroutine to build the context window.
func (s *ExecutionPreparationService) buildContextWindow(prepareCtx context.Context, pc *prepareContext, errs chan<- error) {
	if s.contextMgr == nil {
		errs <- nil
		return
	}
	if skip, reason := shouldSkipContextWindow(pc.task, pc.session); skip {
		clilatency.PrintfWithContext(pc.ctx, "[latency] context_window=skipped reason=%s\n", reason)
		pc.window.Messages = pc.session.Messages
		pc.window.SystemPrompt = DefaultSystemPrompt
		errs <- nil
		return
	}

	originalCount := len(pc.session.Messages)
	windowStarted := time.Now()
	unattended := appcontext.IsUnattendedContext(prepareCtx)
	var okrContext string
	if s.okrContextProvider != nil {
		okrContext = s.okrContextProvider()
	}

	channel := appcontext.ChannelFromContext(prepareCtx)
	channelHint := ""
	if s.channelHints != nil {
		channelHint = s.channelHints[channel]
	}

	window, err := s.contextMgr.BuildWindow(prepareCtx, pc.session, agent.ContextWindowConfig{
		TokenLimit:         s.config.MaxTokens,
		PersonaKey:         pc.personaKey,
		ToolMode:           string(pc.toolMode),
		ToolPreset:         pc.toolPreset,
		EnvironmentSummary: s.config.EnvironmentSummary,
		TaskInput:          pc.task,
		PromptMode:         s.config.Proactive.Prompt.Mode,
		PromptTimezone:     s.config.Proactive.Prompt.Timezone,
		BootstrapFiles:     append([]string(nil), s.config.Proactive.Prompt.BootstrapFiles...),
		BootstrapMaxChars:  s.config.Proactive.Prompt.BootstrapMaxChars,
		ReplyTagsEnabled:   s.config.Proactive.Prompt.ReplyTagsEnabled,
		Skills:             buildSkillsConfig(s.config.Proactive.Skills),
		OKRContext:         okrContext,
		Unattended:         unattended,
		Channel:            channel,
		ChannelHint:        channelHint,
	})
	if err != nil {
		errs <- fmt.Errorf("build context window: %w", err)
		return
	}
	clilatency.PrintfWithContext(pc.ctx,
		"[latency] context_window_ms=%.2f original=%d final=%d\n",
		float64(time.Since(windowStarted))/float64(time.Millisecond),
		originalCount,
		len(window.Messages),
	)
	pc.session.Messages = window.Messages
	pc.initialCognitive = buildCognitiveFromWindow(window)
	if compressedCount := len(window.Messages); compressedCount < originalCount {
		pc.contextWasCompressed = true
		compressionEvent := domain.NewDiagnosticContextCompressionEvent(
			agent.LevelCore,
			pc.session.ID,
			pc.ids.RunID,
			pc.ids.ParentRunID,
			originalCount,
			compressedCount,
			s.clock.Now(),
		)
		s.eventEmitter.OnEvent(compressionEvent)
	}
	pc.window = window
	errs <- nil
}

// initLLMClient runs in a goroutine to create and wrap the LLM client.
func (s *ExecutionPreparationService) initLLMClient(prepareCtx context.Context, pc *prepareContext, errs chan<- error) {
	s.logger.Debug("Getting isolated LLM client: provider=%s, model=%s", pc.effectiveProfile.Provider, pc.effectiveProfile.Model)
	llmInitStarted := time.Now()
	apiKeySource := "profile"
	if pc.selectionPinned {
		apiKeySource = "pinned_selection"
	} else if s.credentialRefresher != nil {
		apiKeySource = "credential_refresher"
	}
	refreshCreds := !pc.selectionPinned || (pc.selectionPinned && utils.IsBlank(pc.effectiveProfile.APIKey))
	s.logger.Debug("LLM config resolved: provider=%s model=%s pinned=%t api_key_source=%s",
		pc.effectiveProfile.Provider, pc.effectiveProfile.Model, pc.selectionPinned, apiKeySource)

	client, _, err := llmclient.GetIsolatedClientFromProfile(
		s.llmFactory,
		pc.effectiveProfile,
		llmclient.CredentialRefresher(s.credentialRefresher),
		refreshCreds,
	)
	clilatency.PrintfWithContext(pc.ctx,
		"[latency] llm_client_init_ms=%.2f provider=%s model=%s\n",
		float64(time.Since(llmInitStarted))/float64(time.Millisecond),
		strings.TrimSpace(pc.effectiveProfile.Provider),
		strings.TrimSpace(pc.effectiveProfile.Model),
	)
	if err != nil {
		errs <- fmt.Errorf("failed to get LLM client: %w", err)
		return
	}

	client = s.wrapPinnedRateLimitFallback(
		prepareCtx,
		pc.selectionPinned,
		pc.task,
		pc.preloadedAttachments,
		pc.userAttachments,
		pc.effectiveProfile,
		client,
	)
	s.logger.Debug("Isolated LLM client obtained successfully")

	client = s.costDecorator.Wrap(prepareCtx, pc.session.ID, client)
	streaming, ok := llm.EnsureStreamingClient(client).(llm.StreamingLLMClient)
	if !ok {
		errs <- fmt.Errorf("failed to wrap LLM client with streaming support")
		return
	}
	pc.llmClient = client
	pc.streamingClient = streaming
	errs <- nil
}

// assembleTaskState builds the TaskState from all resolved data.
func (s *ExecutionPreparationService) assembleTaskState(pc *prepareContext) *domain.TaskState {
	if pc.contextWasCompressed && len(pc.preloadedImportant) > 0 {
		if msg := buildImportantNotesMessage(pc.preloadedImportant); msg != nil {
			pc.window.Messages = append(pc.window.Messages, *msg)
			pc.session.Messages = append(pc.session.Messages, *msg)
		}
	}

	systemPrompt := s.buildSystemPrompt(pc)

	history := s.recallUserHistory(pc.ctx, pc.llmClient, pc.task, pc.rawHistory)
	stateMessages := append([]domain.Message(nil), pc.session.Messages...)
	if history != nil && len(history.messages) > 0 {
		stateMessages = history.messages
	}

	state := &domain.TaskState{
		SystemPrompt:         systemPrompt,
		Messages:             stateMessages,
		SessionID:            pc.session.ID,
		RunID:                pc.ids.RunID,
		ParentRunID:          pc.ids.ParentRunID,
		Attachments:          pc.preloadedAttachments,
		AttachmentIterations: make(map[string]int),
		Important:            pc.preloadedImportant,
		Plans:                nil,
		Cognitive:            pc.initialCognitive,
		PlanReviewEnabled:    appcontext.PlanReviewEnabled(pc.ctx),
	}
	for key := range pc.preloadedAttachments {
		if key == "" {
			continue
		}
		state.AttachmentIterations[key] = 0
	}
	for key, iter := range pc.inheritedIterations {
		name := strings.TrimSpace(key)
		if name == "" {
			continue
		}
		state.AttachmentIterations[name] = iter
	}

	if pc.inheritedState != nil {
		s.applyInheritedStateSnapshot(state, pc.inheritedState)
	}

	if len(pc.userAttachments) > 0 {
		if state.Attachments == nil {
			state.Attachments = make(map[string]ports.Attachment)
		}
		pending := make(map[string]ports.Attachment)
		for _, att := range pc.userAttachments {
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

	return state
}

// assembleServices builds the domain.Services struct for execution.
func (s *ExecutionPreparationService) assembleServices(pc *prepareContext) domain.Services {
	toolRegistry := s.selectToolRegistry(pc.ctx, pc.toolMode, pc.toolPreset)
	return domain.Services{
		LLM:          pc.streamingClient,
		ToolExecutor: toolRegistry,
		ToolLimiter:  NewToolExecutionLimiter(s.config.ToolMaxConcurrent),
		Parser:       s.parser,
		Context:      s.contextMgr,
	}
}
