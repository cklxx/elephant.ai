package app

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/agent/presets"
	id "alex/internal/utils/id"
)

const (
	historyComposeSnippetLimit = 600
	historySummaryMaxTokens    = 320
	historySummaryLLMTimeout   = 4 * time.Second
	historySummaryIntent       = "user_history_summary"
	defaultSystemPrompt        = "You are ALEX, a helpful AI coding assistant. Follow Plan → Clearify → ReAct → Finalize. Call plan() before any non-plan/clearify tool call; call clearify() before each task's first action tool call."
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

	session, err := s.loadSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	ids := id.IDsFromContext(ctx)
	if session != nil {
		ids.SessionID = session.ID
	}

	sessionHistory := s.loadSessionHistory(ctx, session)
	rawHistory := ports.CloneMessages(sessionHistory)
	if session != nil {
		session.Messages = sessionHistory
	}

	var inheritedState *ports.TaskState
	if isSubagentContext(ctx) {
		inheritedState = ports.GetTaskStateSnapshot(ctx)
	}

	var (
		initialWorldState map[string]any
		initialWorldDiff  map[string]any
		window            ports.ContextWindow
	)

	toolPreset := s.config.ToolPreset
	if s.presetResolver != nil {
		if resolved, source := s.presetResolver.resolveToolPreset(ctx, toolPreset); resolved != "" {
			toolPreset = resolved
			s.logger.Info("Using tool preset %s (source=%s)", resolved, source)
		}
	} else if toolPreset == "" {
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
		originalCount := len(session.Messages)
		var err error
		window, err = s.contextMgr.BuildWindow(ctx, session, ports.ContextWindowConfig{
			TokenLimit:         s.config.MaxTokens,
			PersonaKey:         personaKey,
			ToolPreset:         toolPreset,
			EnvironmentSummary: s.config.EnvironmentSummary,
		})
		if err != nil {
			return nil, fmt.Errorf("build context window: %w", err)
		}
		session.Messages = window.Messages
		initialWorldState, initialWorldDiff = buildWorldStateFromWindow(window)
		if compressedCount := len(window.Messages); compressedCount < originalCount {
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
	systemPrompt := strings.TrimSpace(window.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt
	}
	if runID := strings.TrimSpace(ids.TaskID); runID != "" {
		systemPrompt = strings.TrimSpace(systemPrompt +
			"\n\n## Runtime Identifiers\n" +
			fmt.Sprintf("- run_id: %s\n", runID) +
			"- Use this exact run_id for plan() and clearify().")
	}

	preloadedAttachments := collectSessionAttachments(session)
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

	effectiveModel := s.config.LLMModel
	if taskNeedsVision(task, preloadedAttachments, GetUserAttachments(ctx)) {
		if visionModel := strings.TrimSpace(s.config.LLMVisionModel); visionModel != "" {
			effectiveModel = visionModel
		}
	}

	s.logger.Debug("Getting isolated LLM client: provider=%s, model=%s", s.config.LLMProvider, effectiveModel)
	// Use GetIsolatedClient to ensure session-level cost tracking isolation
	llmClient, err := s.llmFactory.GetIsolatedClient(s.config.LLMProvider, effectiveModel, ports.LLMConfig{
		APIKey:  s.config.APIKey,
		BaseURL: s.config.BaseURL,
	})
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

	toolRegistry := s.selectToolRegistry(ctx, toolPreset)
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
	resolved, _ := s.presetResolver.resolveToolPreset(ctx, preset)
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

func (s *ExecutionPreparationService) selectToolRegistry(ctx context.Context, resolvedToolPreset string) ports.ToolRegistry {
	// Handle subagent context filtering first
	registry := s.toolRegistry
	configPreset := resolvedToolPreset
	if configPreset == "" {
		configPreset = s.config.ToolPreset
	}
	if isSubagentContext(ctx) {
		registry = s.getRegistryWithoutSubagent()
		s.logger.Debug("Using filtered registry (subagent excluded) for nested call")

		// Apply preset configured for subagents (context overrides allowed)
		return s.presetResolver.ResolveToolRegistry(ctx, registry, configPreset)
	}

	return s.presetResolver.ResolveToolRegistry(ctx, registry, configPreset)
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
