package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/agent/presets"
	"alex/internal/utils/clilatency"
	id "alex/internal/utils/id"
)

const (
	historyComposeSnippetLimit = 600
	historySummaryMaxTokens    = 320
	historySummaryLLMTimeout   = 4 * time.Second
	historySummaryIntent       = "user_history_summary"
	defaultSystemPrompt        = "You are ALEX, a helpful AI coding assistant. Follow Plan → (Clearify if needed) → ReAct → Finalize. Always call plan() before any action tool call. If plan(complexity=\"complex\"), call clearify() before each task's first action tool call. If plan(complexity=\"simple\"), skip clearify unless you need user input. Avoid emojis in responses unless the user explicitly requests them."
)

// ExecutionPreparationDeps enumerates the dependencies required by the preparation service.
type ExecutionPreparationDeps struct {
	LLMFactory     ports.LLMClientFactory
	ToolRegistry   ports.ToolRegistry
	SessionStore   ports.SessionStore
	ContextMgr     ports.ContextManager
	HistoryMgr     ports.HistoryManager
	Parser         ports.FunctionCallParser
	Config         Config
	Logger         ports.Logger
	Clock          ports.Clock
	CostDecorator  *CostTrackingDecorator
	PresetResolver *PresetResolver // Optional: if nil, one will be created
	EventEmitter   ports.EventListener
	CostTracker    ports.CostTracker
}

// ExecutionPreparationService prepares everything needed before executing a task.
type ExecutionPreparationService struct {
	llmFactory     ports.LLMClientFactory
	toolRegistry   ports.ToolRegistry
	sessionStore   ports.SessionStore
	contextMgr     ports.ContextManager
	historyMgr     ports.HistoryManager
	parser         ports.FunctionCallParser
	config         Config
	logger         ports.Logger
	clock          ports.Clock
	costDecorator  *CostTrackingDecorator
	presetResolver *PresetResolver
	eventEmitter   ports.EventListener
	costTracker    ports.CostTracker
}

type historyRecall struct {
	messages []ports.Message
}

// NewExecutionPreparationService creates a service instance.
func NewExecutionPreparationService(deps ExecutionPreparationDeps) *ExecutionPreparationService {
	logger := deps.Logger
	if logger == nil {
		logger = ports.NoopLogger{}
	}
	clock := deps.Clock
	if clock == nil {
		clock = ports.SystemClock{}
	}

	costDecorator := deps.CostDecorator
	if costDecorator == nil {
		costDecorator = NewCostTrackingDecorator(nil, logger, clock)
	}

	eventEmitter := deps.EventEmitter
	if eventEmitter == nil {
		eventEmitter = ports.NoopEventListener{}
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
func (s *ExecutionPreparationService) Prepare(ctx context.Context, task string, sessionID string) (*ports.ExecutionEnvironment, error) {
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
	clilatency.Printf(
		"[latency] session_load_ms=%.2f session=%s\n",
		float64(time.Since(sessionLoadStarted))/float64(time.Millisecond),
		latencySessionID,
	)

	ids := id.IDsFromContext(ctx)
	if session != nil {
		ids.SessionID = session.ID
	}

	historyLoadStarted := time.Now()
	sessionHistory := s.loadSessionHistory(ctx, session)
	clilatency.Printf(
		"[latency] session_history_ms=%.2f messages=%d\n",
		float64(time.Since(historyLoadStarted))/float64(time.Millisecond),
		len(sessionHistory),
	)
	rawHistory := ports.CloneMessages(sessionHistory)
	if session != nil {
		session.Messages = sessionHistory
	}

	var inheritedState *ports.TaskState
	if isSubagentContext(ctx) {
		inheritedState = ports.GetTaskStateSnapshot(ctx)
	}

	var (
		initialWorldState    map[string]any
		initialWorldDiff     map[string]any
		window               ports.ContextWindow
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
	if toolMode == presets.ToolModeWeb {
		toolPreset = ""
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
			clilatency.Printf("[latency] context_window=skipped reason=%s\n", reason)
			window.Messages = session.Messages
			window.SystemPrompt = defaultSystemPrompt
		} else {
			originalCount := len(session.Messages)
			windowStarted := time.Now()
			var err error
			window, err = s.contextMgr.BuildWindow(ctx, session, ports.ContextWindowConfig{
				TokenLimit:         s.config.MaxTokens,
				PersonaKey:         personaKey,
				ToolMode:           string(toolMode),
				ToolPreset:         toolPreset,
				EnvironmentSummary: s.config.EnvironmentSummary,
			})
			if err != nil {
				return nil, fmt.Errorf("build context window: %w", err)
			}
			clilatency.Printf(
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
					ports.LevelCore,
					session.ID,
					ids.TaskID,
					ids.ParentTaskID,
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
		systemPrompt = defaultSystemPrompt
	}
	systemPrompt = strings.TrimSpace(systemPrompt + `

## Artifacts & Attachments
- When producing long-form deliverables (reports, articles, specs), write them to a Markdown artifact via artifacts_write.
- Provide a short summary in the final answer and point the user to the generated file instead of pasting the full content.
- If you want clients to render an attachment card, reference the file with a placeholder like [report.md].`)
	if runID := strings.TrimSpace(ids.TaskID); runID != "" {
		systemPrompt = strings.TrimSpace(systemPrompt +
			"\n\n## Runtime Identifiers\n" +
			fmt.Sprintf("- run_id: %s\n", runID) +
			"- Use this exact run_id for plan() (and clearify() if used).")
	}

	preloadedAttachments := collectSessionAttachments(session)
	preloadedImportant := collectSessionImportant(session)
	inheritedAttachments, inheritedIterations := GetInheritedAttachments(ctx)
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

	taskAnalysis, preferSmallModel := s.preAnalyzeTask(ctx, session, task)
	if session != nil && taskAnalysis != nil {
		if session.Metadata == nil {
			session.Metadata = make(map[string]string)
		}
		if strings.TrimSpace(session.Metadata["title"]) == "" {
			if title := normalizeSessionTitle(taskAnalysis.ActionName); title != "" {
				session.Metadata["title"] = title
			}
		}
	}

	effectiveModel := s.config.LLMModel
	effectiveProvider := s.config.LLMProvider
	if preferSmallModel && strings.TrimSpace(s.config.LLMSmallModel) != "" {
		effectiveProvider = strings.TrimSpace(s.config.LLMSmallProvider)
		if effectiveProvider == "" {
			effectiveProvider = s.config.LLMProvider
		}
		effectiveModel = s.config.LLMSmallModel
	}
	if taskNeedsVision(task, preloadedAttachments, GetUserAttachments(ctx)) {
		if visionModel := strings.TrimSpace(s.config.LLMVisionModel); visionModel != "" {
			effectiveProvider = s.config.LLMProvider
			effectiveModel = visionModel
		}
	}

	s.logger.Debug("Getting isolated LLM client: provider=%s, model=%s", effectiveProvider, effectiveModel)
	// Use GetIsolatedClient to ensure session-level cost tracking isolation
	llmInitStarted := time.Now()
	llmClient, err := s.llmFactory.GetIsolatedClient(effectiveProvider, effectiveModel, ports.LLMConfig{
		APIKey:  s.config.APIKey,
		BaseURL: s.config.BaseURL,
	})
	clilatency.Printf(
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

	streamingClient, ok := ports.EnsureStreamingClient(llmClient).(ports.StreamingLLMClient)
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
		TaskID:               ids.TaskID,
		ParentTaskID:         ids.ParentTaskID,
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

	if userAttachments := GetUserAttachments(ctx); len(userAttachments) > 0 {
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
		Parser:       s.parser,
		Context:      s.contextMgr,
	}

	s.logger.Info("Execution environment prepared successfully")

	return &ports.ExecutionEnvironment{
		State:        state,
		Services:     services,
		Session:      session,
		SystemPrompt: systemPrompt,
		TaskAnalysis: taskAnalysis,
	}, nil
}

func (s *ExecutionPreparationService) loadSessionHistory(ctx context.Context, session *ports.Session) []ports.Message {
	if session == nil {
		return nil
	}
	if s.historyMgr != nil {
		history, err := s.historyMgr.Replay(ctx, session.ID, 0)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("Failed to replay session history (session=%s): %v", session.ID, err)
			}
		} else if len(history) > 0 {
			return ports.CloneMessages(history)
		}
	}
	if len(session.Messages) == 0 {
		return nil
	}
	return ports.CloneMessages(session.Messages)
}

func (s *ExecutionPreparationService) recallUserHistory(ctx context.Context, llm ports.LLMClient, task string, messages []ports.Message) *historyRecall {
	if len(messages) == 0 {
		return nil
	}

	rawMessages := historyMessagesFromSession(messages)
	if len(rawMessages) == 0 {
		return nil
	}

	recall := &historyRecall{}
	if s.shouldSummarizeHistory(rawMessages) {
		summaryMessages := s.composeHistorySummary(ctx, llm, rawMessages)
		if len(summaryMessages) > 0 {
			recall.messages = summaryMessages
			return recall
		}
		s.logger.Warn("History recall summary failed, falling back to raw messages")
	}

	recall.messages = rawMessages
	return recall
}

func (s *ExecutionPreparationService) composeHistorySummary(ctx context.Context, llm ports.LLMClient, messages []ports.Message) []ports.Message {
	if llm == nil || len(messages) == 0 {
		return nil
	}
	prompt := buildHistorySummaryPrompt(messages)
	if prompt == "" {
		return nil
	}
	requestID := id.NewRequestID()
	req := ports.CompletionRequest{
		Messages: []ports.Message{
			{
				Role:    "system",
				Content: "You are a memory specialist who condenses previous assistant conversations into concise, high-signal summaries. Capture the user objectives, assistant actions, and any follow-up commitments in a neutral tone. Limit the response to 2-3 short paragraphs or bullet points.",
			},
			{
				Role:    "user",
				Content: prompt,
				Source:  ports.MessageSourceUserHistory,
			},
		},
		Temperature: 0.1,
		MaxTokens:   historySummaryMaxTokens,
		Metadata: map[string]any{
			"request_id": requestID,
			"intent":     historySummaryIntent,
		},
	}
	summaryCtx, cancel := context.WithTimeout(ctx, historySummaryLLMTimeout)
	defer cancel()
	streaming, ok := ports.EnsureStreamingClient(llm).(ports.StreamingLLMClient)
	if !ok {
		resp, err := llm.Complete(summaryCtx, req)
		if err != nil {
			s.logger.Warn("History summary composition failed (request_id=%s): %v", requestID, err)
			return nil
		}
		if resp == nil || strings.TrimSpace(resp.Content) == "" {
			s.logger.Warn("History summary composition returned empty response (request_id=%s)", requestID)
			return nil
		}
		summary := strings.TrimSpace(resp.Content)
		return []ports.Message{{
			Role:    "system",
			Content: summary,
			Source:  ports.MessageSourceUserHistory,
		}}
	}
	resp, err := streaming.StreamComplete(summaryCtx, req, ports.CompletionStreamCallbacks{
		OnContentDelta: func(ports.ContentDelta) {},
	})
	if err != nil {
		s.logger.Warn("History summary composition failed (request_id=%s): %v", requestID, err)
		return nil
	}
	if resp == nil || strings.TrimSpace(resp.Content) == "" {
		s.logger.Warn("History summary composition returned empty response (request_id=%s)", requestID)
		return nil
	}
	summary := strings.TrimSpace(resp.Content)
	return []ports.Message{{
		Role:    "system",
		Content: summary,
		Source:  ports.MessageSourceUserHistory,
	}}
}

func buildHistorySummaryPrompt(messages []ports.Message) string {
	if len(messages) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("Summarize the intent, assistant responses, tool outputs, and remaining follow-ups from the prior exchanges below. Focus on actionable context relevant to the current task.\n\n")
	for i, msg := range messages {
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			role = "message"
		}
		roleLower := strings.ToLower(role)
		roleLabel := strings.ToUpper(roleLower[:1]) + roleLower[1:]
		builder.WriteString(fmt.Sprintf("%d. %s: ", i+1, roleLabel))
		builder.WriteString(condenseHistoryText(msg.Content, historyComposeSnippetLimit))
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

func condenseHistoryText(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	normalized := normalizeWhitespace(trimmed)
	runes := []rune(normalized)
	if len(runes) <= limit {
		return normalized
	}
	if limit <= 1 {
		return string(runes[:1])
	}
	return string(runes[:limit-1]) + "…"
}

func normalizeWhitespace(value string) string {
	if value == "" {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}

func historyMessagesFromSession(messages []ports.Message) []ports.Message {
	if len(messages) == 0 {
		return nil
	}
	filtered := make([]ports.Message, 0, len(messages))
	for _, msg := range messages {
		if !shouldRecallHistoryMessage(msg) {
			continue
		}
		filtered = append(filtered, cloneHistoryMessage(msg))
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func shouldRecallHistoryMessage(msg ports.Message) bool {
	role := strings.TrimSpace(msg.Role)
	if strings.EqualFold(role, "system") {
		return false
	}
	if msg.Source == ports.MessageSourceSystemPrompt || msg.Source == ports.MessageSourceUserHistory {
		return false
	}
	if strings.TrimSpace(msg.Content) == "" && len(msg.Attachments) == 0 && len(msg.ToolCalls) == 0 && len(msg.ToolResults) == 0 {
		return false
	}
	return true
}

func cloneHistoryMessage(msg ports.Message) ports.Message {
	cloned := msg
	cloned.Role = strings.TrimSpace(cloned.Role)
	if cloned.Role == "" {
		cloned.Role = msg.Role
	}
	cloned.Content = strings.TrimSpace(cloned.Content)
	cloned.Source = ports.MessageSourceUserHistory
	if len(msg.ToolCalls) > 0 {
		cloned.ToolCalls = append([]ports.ToolCall(nil), msg.ToolCalls...)
	}
	if len(msg.ToolResults) > 0 {
		cloned.ToolResults = make([]ports.ToolResult, len(msg.ToolResults))
		for i, result := range msg.ToolResults {
			cloned.ToolResults[i] = cloneHistoryToolResult(result)
		}
	}
	if len(msg.Metadata) > 0 {
		metadata := make(map[string]any, len(msg.Metadata))
		for key, value := range msg.Metadata {
			metadata[key] = value
		}
		cloned.Metadata = metadata
	}
	if len(msg.Attachments) > 0 {
		cloned.Attachments = cloneHistoryAttachments(msg.Attachments)
	}
	return cloned
}

func cloneHistoryToolResult(result ports.ToolResult) ports.ToolResult {
	cloned := result
	if len(result.Metadata) > 0 {
		metadata := make(map[string]any, len(result.Metadata))
		for key, value := range result.Metadata {
			metadata[key] = value
		}
		cloned.Metadata = metadata
	}
	if len(result.Attachments) > 0 {
		cloned.Attachments = cloneHistoryAttachments(result.Attachments)
	}
	return cloned
}

func cloneHistoryAttachments(values map[string]ports.Attachment) map[string]ports.Attachment {
	if len(values) == 0 {
		return nil
	}
	return ports.CloneAttachmentMap(values)
}

func (s *ExecutionPreparationService) shouldSummarizeHistory(messages []ports.Message) bool {
	if len(messages) == 0 {
		return false
	}
	limit := s.config.MaxTokens
	if limit <= 0 {
		return false
	}
	threshold := int(float64(limit) * 0.7)
	if threshold <= 0 {
		return false
	}
	return s.estimateHistoryTokens(messages) > threshold
}

func (s *ExecutionPreparationService) estimateHistoryTokens(messages []ports.Message) int {
	if len(messages) == 0 {
		return 0
	}
	if s.contextMgr != nil {
		if estimate := s.contextMgr.EstimateTokens(messages); estimate > 0 {
			return estimate
		}
	}
	total := 0
	for _, msg := range messages {
		total += len(msg.Content) / 4
	}
	return total
}

func collectSessionAttachments(session *ports.Session) map[string]ports.Attachment {
	attachments := make(map[string]ports.Attachment)
	if session == nil {
		return attachments
	}

	mergeAttachmentMaps(attachments, session.Attachments)
	for _, msg := range session.Messages {
		mergeAttachmentMaps(attachments, msg.Attachments)
	}
	return attachments
}

func mergeAttachmentMaps(target map[string]ports.Attachment, source map[string]ports.Attachment) {
	if len(source) == 0 {
		return
	}
	for key, att := range source {
		name := strings.TrimSpace(key)
		if name == "" {
			name = strings.TrimSpace(att.Name)
		}
		if name == "" {
			continue
		}
		if att.Name == "" {
			att.Name = name
		}
		target[name] = att
	}
}

func collectSessionImportant(session *ports.Session) map[string]ports.ImportantNote {
	notes := make(map[string]ports.ImportantNote)
	if session == nil {
		return notes
	}
	mergeImportantNotes(notes, session.Important)
	return notes
}

func mergeImportantNotes(target map[string]ports.ImportantNote, source map[string]ports.ImportantNote) {
	if len(source) == 0 {
		return
	}
	for key, note := range source {
		id := strings.TrimSpace(key)
		if id == "" {
			id = strings.TrimSpace(note.ID)
		}
		if id == "" {
			continue
		}
		if note.ID == "" {
			note.ID = id
		}
		target[id] = note
	}
}

func buildImportantNotesMessage(notes map[string]ports.ImportantNote) *ports.Message {
	if len(notes) == 0 {
		return nil
	}
	type annotated struct {
		id   string
		note ports.ImportantNote
	}
	items := make([]annotated, 0, len(notes))
	for id, note := range notes {
		content := strings.TrimSpace(note.Content)
		if content == "" {
			continue
		}
		note.Content = content
		items = append(items, annotated{id: id, note: note})
	}
	if len(items) == 0 {
		return nil
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i].note.CreatedAt
		right := items[j].note.CreatedAt
		if !left.Equal(right) {
			return left.Before(right)
		}
		return items[i].id < items[j].id
	})

	var builder strings.Builder
	builder.WriteString("Important session notes (auto-recalled after compression):\n")
	for idx, item := range items {
		builder.WriteString(fmt.Sprintf("%d. %s", idx+1, item.note.Content))
		var metaParts []string
		if len(item.note.Tags) > 0 {
			metaParts = append(metaParts, fmt.Sprintf("tags: %s", strings.Join(item.note.Tags, ",")))
		}
		if source := strings.TrimSpace(item.note.Source); source != "" {
			metaParts = append(metaParts, fmt.Sprintf("source: %s", source))
		}
		if !item.note.CreatedAt.IsZero() {
			metaParts = append(metaParts, fmt.Sprintf("recorded: %s", item.note.CreatedAt.Format(time.RFC3339)))
		}
		if len(metaParts) > 0 {
			builder.WriteString(" (")
			builder.WriteString(strings.Join(metaParts, "; "))
			builder.WriteString(")")
		}
		if idx < len(items)-1 {
			builder.WriteString("\n")
		}
	}

	return &ports.Message{
		Role:    "system",
		Content: builder.String(),
		Source:  ports.MessageSourceImportant,
	}
}

var visionPlaceholderPattern = regexp.MustCompile(`\[([^\[\]]+)\]`)

func taskNeedsVision(task string, attachments map[string]ports.Attachment, userAttachments []ports.Attachment) bool {
	for _, att := range userAttachments {
		if isImageAttachment(att) {
			return true
		}
	}

	if strings.TrimSpace(task) == "" || len(attachments) == 0 {
		return false
	}

	matches := visionPlaceholderPattern.FindAllStringSubmatch(task, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		placeholder := strings.TrimSpace(match[1])
		if placeholder == "" {
			continue
		}
		att, ok := lookupAttachmentByName(attachments, placeholder)
		if !ok {
			continue
		}
		if isImageAttachment(att) {
			return true
		}
	}

	return false
}

func lookupAttachmentByName(attachments map[string]ports.Attachment, name string) (ports.Attachment, bool) {
	if len(attachments) == 0 {
		return ports.Attachment{}, false
	}
	if att, ok := attachments[name]; ok {
		return att, true
	}
	for key, att := range attachments {
		if strings.EqualFold(key, name) || strings.EqualFold(att.Name, name) {
			return att, true
		}
	}
	return ports.Attachment{}, false
}

func isImageAttachment(att ports.Attachment) bool {
	mediaType := strings.ToLower(strings.TrimSpace(att.MediaType))
	if strings.HasPrefix(mediaType, "image/") {
		return true
	}
	name := strings.TrimSpace(att.Name)
	if name == "" {
		return false
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tif", ".tiff":
		return true
	default:
		return false
	}
}

func (s *ExecutionPreparationService) applyInheritedStateSnapshot(state *domain.TaskState, inherited *ports.TaskState) {
	if state == nil || inherited == nil {
		return
	}
	snapshot := ports.CloneTaskState(inherited)
	if trimmed := strings.TrimSpace(snapshot.SystemPrompt); trimmed != "" {
		state.SystemPrompt = trimmed
	}
	if len(snapshot.Messages) > 0 {
		state.Messages = ports.CloneMessages(snapshot.Messages)
	}
	if len(snapshot.Attachments) > 0 {
		if state.Attachments == nil {
			state.Attachments = make(map[string]ports.Attachment)
		}
		mergeAttachmentMaps(state.Attachments, snapshot.Attachments)
	}
	if len(snapshot.Important) > 0 {
		if state.Important == nil {
			state.Important = make(map[string]ports.ImportantNote)
		}
		mergeImportantNotes(state.Important, snapshot.Important)
	}
	if len(snapshot.AttachmentIterations) > 0 {
		if state.AttachmentIterations == nil {
			state.AttachmentIterations = make(map[string]int)
		}
		for key, iter := range snapshot.AttachmentIterations {
			name := strings.TrimSpace(key)
			if name == "" {
				continue
			}
			state.AttachmentIterations[name] = iter
		}
	}
	if len(snapshot.Plans) > 0 {
		state.Plans = ports.ClonePlanNodes(snapshot.Plans)
	}
	if len(snapshot.Beliefs) > 0 {
		state.Beliefs = ports.CloneBeliefs(snapshot.Beliefs)
	}
	if len(snapshot.KnowledgeRefs) > 0 {
		state.KnowledgeRefs = ports.CloneKnowledgeReferences(snapshot.KnowledgeRefs)
	}
	if len(snapshot.WorldState) > 0 {
		state.WorldState = snapshot.WorldState
	}
	if len(snapshot.WorldDiff) > 0 {
		state.WorldDiff = snapshot.WorldDiff
	}
	if len(snapshot.FeedbackSignals) > 0 {
		state.FeedbackSignals = ports.CloneFeedbackSignals(snapshot.FeedbackSignals)
	}
}

// SetEnvironmentSummary updates the environment summary used when preparing prompts.
func (s *ExecutionPreparationService) SetEnvironmentSummary(summary string) {
	s.config.EnvironmentSummary = summary
}

func (s *ExecutionPreparationService) ResolveAgentPreset(ctx context.Context, preset string) string {
	if s.presetResolver == nil {
		return ""
	}
	resolved, _ := s.presetResolver.resolveAgentPreset(ctx, preset)
	return resolved
}

func (s *ExecutionPreparationService) ResolveToolPreset(ctx context.Context, preset string) string {
	if s.presetResolver == nil {
		return ""
	}
	toolMode := presets.ToolMode(strings.TrimSpace(s.config.ToolMode))
	if toolMode == "" {
		toolMode = presets.ToolModeCLI
	}
	resolved, _ := s.presetResolver.resolveToolPreset(ctx, toolMode, preset)
	return resolved
}

func (s *ExecutionPreparationService) loadSession(ctx context.Context, id string) (*ports.Session, error) {
	if id == "" {
		session, err := s.sessionStore.Create(ctx)
		if err != nil {
			s.logger.Error("Failed to create session: %v", err)
		}
		return session, err
	}

	session, err := s.sessionStore.Get(ctx, id)
	if err != nil {
		s.logger.Error("Failed to load session: %v", err)
	}
	return session, err
}

func (s *ExecutionPreparationService) selectToolRegistry(ctx context.Context, toolMode presets.ToolMode, resolvedToolPreset string) ports.ToolRegistry {
	// Handle subagent context filtering first
	registry := s.toolRegistry
	if toolMode == "" {
		toolMode = presets.ToolModeCLI
	}
	configPreset := resolvedToolPreset
	if configPreset == "" && toolMode != presets.ToolModeWeb {
		configPreset = s.config.ToolPreset
	}
	if isSubagentContext(ctx) {
		registry = s.getRegistryWithoutSubagent()
		s.logger.Debug("Using filtered registry (subagent excluded) for nested call")

		// Apply preset configured for subagents (context overrides allowed)
		return s.presetResolver.ResolveToolRegistry(ctx, registry, toolMode, configPreset)
	}

	return s.presetResolver.ResolveToolRegistry(ctx, registry, toolMode, configPreset)
}

func (s *ExecutionPreparationService) getRegistryWithoutSubagent() ports.ToolRegistry {
	type registryWithFilter interface {
		WithoutSubagent() ports.ToolRegistry
	}

	if filtered, ok := s.toolRegistry.(registryWithFilter); ok {
		return filtered.WithoutSubagent()
	}

	return s.toolRegistry
}

func (s *ExecutionPreparationService) preAnalyzeTask(ctx context.Context, session *ports.Session, task string) (*ports.TaskAnalysis, bool) {
	if strings.TrimSpace(task) == "" {
		return nil, false
	}
	provider, model, ok := s.resolveSmallModelConfig()
	if !ok || session == nil {
		return nil, false
	}
	if analysis, preferSmall, ok := quickTriageTask(task); ok {
		clilatency.Printf("[latency] preanalysis=skipped reason=%s\n", analysis.Approach)
		return analysis, preferSmall
	}
	client, err := s.llmFactory.GetIsolatedClient(provider, model, ports.LLMConfig{
		APIKey:  s.config.APIKey,
		BaseURL: s.config.BaseURL,
	})
	if err != nil {
		s.logger.Warn("Task pre-analysis skipped: %v", err)
		return nil, false
	}
	client = s.costDecorator.Wrap(ctx, session.ID, client)

	taskNameRule := `- task_name must be a short single-line title (<= 32 chars), suitable for a session title.` + "\n\n"
	if strings.TrimSpace(session.Metadata["title"]) != "" {
		taskNameRule = `- task_name must be an empty string because the session already has a title.` + "\n\n"
	}

	req := ports.CompletionRequest{
		Messages: []ports.Message{
			{
				Role: "system",
				Content: "You are a fast task triage assistant. Analyze the user's task and decide which model tier is sufficient.\n\n" +
					"Definitions:\n" +
					`- complexity="simple": can be completed quickly with straightforward steps; no deep design/architecture; no large refactors; no ambiguous requirements; no heavy external research.\n` +
					`- complexity="complex": otherwise.\n` +
					`- recommended_model="small": a smaller/cheaper model should handle the entire task reliably.\n` +
					`- recommended_model="default": use the default (stronger) model.\n\n` +
					"Output requirements:\n" +
					`- Respond ONLY with JSON.\n` +
					taskNameRule +
					`Schema: {"complexity":"simple|complex","recommended_model":"small|default","task_name":"...","goal":"...","approach":"...","success_criteria":["..."],` +
					`"steps":[{"description":"...","rationale":"...","needs_external_context":false}],` +
					`"retrieval":{"should_retrieve":false,"local_queries":[],"search_queries":[],"crawl_urls":[],"knowledge_gaps":[],"notes":""}}`,
			},
			{Role: "user", Content: task},
		},
		Temperature: 0.2,
		MaxTokens:   320,
	}

	preanalysisStarted := time.Now()
	analysisCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	streaming, ok := ports.EnsureStreamingClient(client).(ports.StreamingLLMClient)
	if !ok {
		resp, err := client.Complete(analysisCtx, req)
		clilatency.Printf(
			"[latency] preanalysis_ms=%.2f model=%s\n",
			float64(time.Since(preanalysisStarted))/float64(time.Millisecond),
			strings.TrimSpace(model),
		)
		if err != nil || resp == nil {
			s.logger.Warn("Task pre-analysis failed: %v", err)
			return nil, false
		}
		analysis, recommendedModel := parseTaskAnalysis(resp.Content)
		if analysis == nil {
			s.logger.Warn("Task pre-analysis returned unparsable output")
			return nil, false
		}
		preferSmallModel := false
		if strings.EqualFold(recommendedModel, "small") {
			preferSmallModel = true
		} else if strings.EqualFold(recommendedModel, "default") {
			preferSmallModel = false
		} else {
			preferSmallModel = strings.EqualFold(analysis.Complexity, "simple")
		}
		return analysis, preferSmallModel
	}
	resp, err := streaming.StreamComplete(analysisCtx, req, ports.CompletionStreamCallbacks{
		OnContentDelta: func(ports.ContentDelta) {},
	})
	clilatency.Printf(
		"[latency] preanalysis_ms=%.2f model=%s\n",
		float64(time.Since(preanalysisStarted))/float64(time.Millisecond),
		strings.TrimSpace(model),
	)
	if err != nil || resp == nil {
		s.logger.Warn("Task pre-analysis failed: %v", err)
		return nil, false
	}
	analysis, recommendedModel := parseTaskAnalysis(resp.Content)
	if analysis == nil {
		s.logger.Warn("Task pre-analysis returned unparsable output")
		return nil, false
	}
	preferSmallModel := false
	if strings.EqualFold(recommendedModel, "small") {
		preferSmallModel = true
	} else if strings.EqualFold(recommendedModel, "default") {
		preferSmallModel = false
	} else {
		preferSmallModel = strings.EqualFold(analysis.Complexity, "simple")
	}
	return analysis, preferSmallModel
}

func quickTriageTask(task string) (*ports.TaskAnalysis, bool, bool) {
	trimmed := strings.TrimSpace(task)
	if trimmed == "" {
		return nil, false, false
	}
	if strings.Contains(trimmed, "\n") || strings.Contains(trimmed, "\r") {
		return nil, false, false
	}
	runes := []rune(trimmed)
	if len(runes) > 24 {
		return nil, false, false
	}

	switch strings.ToLower(trimmed) {
	case "hi", "hello", "hey", "yo", "nihao", "你好", "您好", "嗨", "在吗", "ping", "pings":
		return &ports.TaskAnalysis{
			Complexity: "simple",
			ActionName: "Greeting",
			Goal:       "",
			Approach:   "greeting",
		}, true, true
	case "thanks", "thank you", "thx", "谢谢", "多谢", "感谢", "ok", "okay", "好的", "收到":
		return &ports.TaskAnalysis{
			Complexity: "simple",
			ActionName: "Acknowledge",
			Goal:       "",
			Approach:   "ack",
		}, true, true
	default:
		return nil, false, false
	}
}

func shouldSkipContextWindow(task string, session *ports.Session) (bool, string) {
	if session == nil {
		return false, ""
	}
	if len(session.Messages) > 0 {
		return false, ""
	}
	analysis, _, ok := quickTriageTask(task)
	if !ok || analysis == nil {
		return false, ""
	}
	switch strings.TrimSpace(strings.ToLower(analysis.Approach)) {
	case "greeting":
		return true, "greeting"
	case "ack":
		return true, "ack"
	default:
		return false, ""
	}
}

func (s *ExecutionPreparationService) resolveSmallModelConfig() (string, string, bool) {
	model := strings.TrimSpace(s.config.LLMSmallModel)
	if model == "" {
		return "", "", false
	}
	provider := strings.TrimSpace(s.config.LLMSmallProvider)
	if provider == "" {
		provider = strings.TrimSpace(s.config.LLMProvider)
	}
	if provider == "" {
		return "", "", false
	}
	return provider, model, true
}

type taskAnalysisPayload struct {
	Complexity       string                     `json:"complexity"`
	RecommendedModel string                     `json:"recommended_model"`
	TaskName         string                     `json:"task_name"`
	ActionName       string                     `json:"action_name"`
	Goal             string                     `json:"goal"`
	Approach         string                     `json:"approach"`
	SuccessCriteria  []string                   `json:"success_criteria"`
	Steps            []taskAnalysisStepPayload  `json:"steps"`
	Retrieval        taskAnalysisRetrievalHints `json:"retrieval"`
}

type taskAnalysisStepPayload struct {
	Description          string `json:"description"`
	Rationale            string `json:"rationale"`
	NeedsExternalContext bool   `json:"needs_external_context"`
}

type taskAnalysisRetrievalHints struct {
	ShouldRetrieve bool     `json:"should_retrieve"`
	LocalQueries   []string `json:"local_queries"`
	SearchQueries  []string `json:"search_queries"`
	CrawlURLs      []string `json:"crawl_urls"`
	KnowledgeGaps  []string `json:"knowledge_gaps"`
	Notes          string   `json:"notes"`
}

func parseTaskAnalysis(raw string) (*ports.TaskAnalysis, string) {
	body := strings.TrimSpace(raw)
	start := strings.Index(body, "{")
	end := strings.LastIndex(body, "}")
	if start < 0 || end <= start {
		return nil, ""
	}
	body = body[start : end+1]

	var payload taskAnalysisPayload
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil, ""
	}

	analysis := &ports.TaskAnalysis{
		Complexity:      normalizeComplexity(payload.Complexity),
		ActionName:      coalesce(payload.TaskName, payload.ActionName),
		Goal:            strings.TrimSpace(payload.Goal),
		Approach:        strings.TrimSpace(payload.Approach),
		SuccessCriteria: compactStrings(payload.SuccessCriteria),
	}

	if len(payload.Steps) > 0 {
		analysis.TaskBreakdown = make([]ports.TaskAnalysisStep, 0, len(payload.Steps))
		for _, step := range payload.Steps {
			if strings.TrimSpace(step.Description) == "" {
				continue
			}
			analysis.TaskBreakdown = append(analysis.TaskBreakdown, ports.TaskAnalysisStep{
				Description:          strings.TrimSpace(step.Description),
				NeedsExternalContext: step.NeedsExternalContext,
				Rationale:            strings.TrimSpace(step.Rationale),
			})
		}
	}

	if analysis.Retrieval.ShouldRetrieve || payload.Retrieval.ShouldRetrieve {
		analysis.Retrieval.ShouldRetrieve = true
	}
	if len(payload.Retrieval.LocalQueries) > 0 {
		analysis.Retrieval.LocalQueries = compactStrings(payload.Retrieval.LocalQueries)
	}
	if len(payload.Retrieval.SearchQueries) > 0 {
		analysis.Retrieval.SearchQueries = compactStrings(payload.Retrieval.SearchQueries)
	}
	if len(payload.Retrieval.CrawlURLs) > 0 {
		analysis.Retrieval.CrawlURLs = compactStrings(payload.Retrieval.CrawlURLs)
	}
	if len(payload.Retrieval.KnowledgeGaps) > 0 {
		analysis.Retrieval.KnowledgeGaps = compactStrings(payload.Retrieval.KnowledgeGaps)
	}
	if note := strings.TrimSpace(payload.Retrieval.Notes); note != "" {
		analysis.Retrieval.Notes = note
	}
	if !analysis.Retrieval.ShouldRetrieve {
		for _, step := range payload.Steps {
			if step.NeedsExternalContext {
				analysis.Retrieval.ShouldRetrieve = true
				break
			}
		}
	}

	return analysis, normalizeRecommendedModel(payload.RecommendedModel)
}

func normalizeComplexity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "simple", "easy":
		return "simple"
	case "complex", "hard":
		return "complex"
	default:
		return ""
	}
}

func normalizeRecommendedModel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "small", "mini":
		return "small"
	case "default", "large":
		return "default"
	default:
		return ""
	}
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func coalesce(values ...string) string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
func buildWorldStateFromWindow(window ports.ContextWindow) (map[string]any, map[string]any) {
	profile := window.Static.World
	envSummary := strings.TrimSpace(window.Static.EnvironmentSummary)
	hasProfile := profile.ID != "" || profile.Environment != "" || len(profile.Capabilities) > 0 || len(profile.Limits) > 0 || len(profile.CostModel) > 0
	if !hasProfile && envSummary == "" {
		return nil, nil
	}
	state := make(map[string]any)
	if hasProfile {
		profileMap := map[string]any{"id": profile.ID}
		if profile.Environment != "" {
			profileMap["environment"] = profile.Environment
		}
		if len(profile.Capabilities) > 0 {
			profileMap["capabilities"] = append([]string(nil), profile.Capabilities...)
		}
		if len(profile.Limits) > 0 {
			profileMap["limits"] = append([]string(nil), profile.Limits...)
		}
		if len(profile.CostModel) > 0 {
			profileMap["cost_model"] = append([]string(nil), profile.CostModel...)
		}
		state["profile"] = profileMap
	}
	if envSummary != "" {
		state["environment_summary"] = envSummary
	}
	var diff map[string]any
	if len(state) > 0 {
		diff = make(map[string]any)
		if profile.ID != "" {
			diff["profile_loaded"] = profile.ID
		}
		if envSummary != "" {
			diff["environment_summary"] = envSummary
		}
		if len(profile.Capabilities) > 0 {
			diff["capabilities"] = append([]string(nil), profile.Capabilities...)
		}
	}
	return state, diff
}
