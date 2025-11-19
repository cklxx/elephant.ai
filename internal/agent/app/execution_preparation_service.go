package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	id "alex/internal/utils/id"
)

const (
	defaultBudgetTarget        = 5.0
	historyComposeSnippetLimit = 600
	historySummaryMaxTokens    = 320
	historySummaryLLMTimeout   = 4 * time.Second
	historySummaryIntent       = "user_history_summary"
	defaultSystemPrompt        = "You are ALEX, a helpful AI coding assistant. Use available tools to help solve the user's task."
)

// ExecutionPreparationDeps enumerates the dependencies required by the preparation service.
type ExecutionPreparationDeps struct {
	LLMFactory     ports.LLMClientFactory
	ToolRegistry   ports.ToolRegistry
	SessionStore   ports.SessionStore
	ContextMgr     ports.ContextManager
	Parser         ports.FunctionCallParser
	Config         Config
	Logger         ports.Logger
	Clock          ports.Clock
	Analysis       *TaskAnalysisService
	CostDecorator  *CostTrackingDecorator
	PresetResolver *PresetResolver // Optional: if nil, one will be created
	EventEmitter   ports.EventListener
	CostTracker    ports.CostTracker
	RAGGate        ports.RAGGate
}

// ExecutionPreparationService prepares everything needed before executing a task.
type ExecutionPreparationService struct {
	llmFactory     ports.LLMClientFactory
	toolRegistry   ports.ToolRegistry
	sessionStore   ports.SessionStore
	contextMgr     ports.ContextManager
	parser         ports.FunctionCallParser
	config         Config
	logger         ports.Logger
	clock          ports.Clock
	analysis       *TaskAnalysisService
	costDecorator  *CostTrackingDecorator
	presetResolver *PresetResolver
	eventEmitter   ports.EventListener
	costTracker    ports.CostTracker
	ragGate        ports.RAGGate
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

	analysis := deps.Analysis
	if analysis == nil {
		analysis = NewTaskAnalysisService(logger)
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
		parser:         deps.Parser,
		config:         deps.Config,
		logger:         logger,
		clock:          clock,
		analysis:       analysis,
		costDecorator:  costDecorator,
		presetResolver: presetResolver,
		eventEmitter:   eventEmitter,
		costTracker:    deps.CostTracker,
		ragGate:        deps.RAGGate,
	}
}

// Prepare builds the execution environment for a task.
func (s *ExecutionPreparationService) Prepare(ctx context.Context, task string, sessionID string) (*ports.ExecutionEnvironment, error) {
	s.logger.Info("PrepareExecution called: task='%s', sessionID='%s'", task, sessionID)

	session, err := s.loadSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	ids := id.IDsFromContext(ctx)
	if session != nil {
		ids.SessionID = session.ID
	}

	var (
		initialWorldState map[string]any
		initialWorldDiff  map[string]any
		window            ports.ContextWindow
	)
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
			ToolPreset:         s.config.ToolPreset,
			EnvironmentSummary: s.config.EnvironmentSummary,
		})
		if err != nil {
			return nil, fmt.Errorf("build context window: %w", err)
		}
		session.Messages = window.Messages
		initialWorldState, initialWorldDiff = buildWorldStateFromWindow(window)
		if compressedCount := len(window.Messages); compressedCount < originalCount {
			compressionEvent := domain.NewContextCompressionEvent(
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

	s.logger.Debug("Getting isolated LLM client: provider=%s, model=%s", s.config.LLMProvider, s.config.LLMModel)
	// Use GetIsolatedClient to ensure session-level cost tracking isolation
	llmClient, err := s.llmFactory.GetIsolatedClient(s.config.LLMProvider, s.config.LLMModel, ports.LLMConfig{
		APIKey:  s.config.APIKey,
		BaseURL: s.config.BaseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get LLM client: %w", err)
	}
	s.logger.Debug("Isolated LLM client obtained successfully")

	// Use Wrap instead of Attach to avoid modifying shared client state
	llmClient = s.costDecorator.Wrap(ctx, session.ID, llmClient)

	analysis := s.analysis.Analyze(ctx, task, llmClient)
	if (analysis == nil || strings.TrimSpace(analysis.ActionName) == "") && strings.TrimSpace(task) != "" {
		if fallback := fallbackTaskAnalysis(task); fallback != nil {
			s.logger.Debug("Task pre-analysis fallback used")
			analysis = fallback
		}
	}
	history := s.recallUserHistory(ctx, llmClient, task, analysis, session)
	var taskAnalysis *ports.TaskAnalysis
	if analysis != nil && analysis.ActionName != "" {
		s.logger.Debug("Task pre-analysis: action=%s, goal=%s", analysis.ActionName, analysis.Goal)
		taskAnalysis = &ports.TaskAnalysis{
			ActionName:      analysis.ActionName,
			Goal:            analysis.Goal,
			Approach:        analysis.Approach,
			SuccessCriteria: append([]string(nil), analysis.Criteria...),
			TaskBreakdown:   cloneTaskAnalysisSteps(analysis.Steps),
			Retrieval:       cloneTaskRetrievalPlan(analysis.Retrieval),
		}
	} else {
		s.logger.Debug("Task pre-analysis skipped or failed")
	}
	planNodes := buildPlanNodesFromTaskAnalysis(taskAnalysis)
	beliefs := deriveBeliefsFromTaskAnalysis(taskAnalysis)
	knowledgeRefs := buildKnowledgeRefsFromTaskAnalysis(taskAnalysis)

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
	stateMessages := append([]domain.Message(nil), session.Messages...)
	if history != nil && len(history.messages) > 0 {
		stateMessages = trimSessionHistoryMessages(stateMessages)
	}

	state := &domain.TaskState{
		SystemPrompt:         systemPrompt,
		Messages:             stateMessages,
		SessionID:            session.ID,
		TaskID:               ids.TaskID,
		ParentTaskID:         ids.ParentTaskID,
		Attachments:          preloadedAttachments,
		AttachmentIterations: make(map[string]int),
		Plans:                planNodes,
		Beliefs:              beliefs,
		KnowledgeRefs:        knowledgeRefs,
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

	if history != nil && len(history.messages) > 0 {
		state.Messages = append(state.Messages, history.messages...)
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

	toolRegistry := s.selectToolRegistry(ctx)
	services := domain.Services{
		LLM:          llmClient,
		ToolExecutor: toolRegistry,
		Parser:       s.parser,
		Context:      s.contextMgr,
	}

	ragDirectives := s.evaluateRAGDirectives(ctx, session, task, toolRegistry, analysis, history)

	s.logger.Info("Execution environment prepared successfully")

	return &ports.ExecutionEnvironment{
		State:         state,
		Services:      services,
		Session:       session,
		SystemPrompt:  systemPrompt,
		TaskAnalysis:  taskAnalysis,
		RAGDirectives: ragDirectives,
	}, nil
}

func (s *ExecutionPreparationService) evaluateRAGDirectives(ctx context.Context, session *ports.Session, task string, registry ports.ToolRegistry, analysis *TaskAnalysis, history *historyRecall) *ports.RAGDirectives {
	if s.ragGate == nil {
		return nil
	}
	baseQuery := strings.TrimSpace(task)
	if analysis != nil {
		if trimmed := strings.TrimSpace(analysis.Goal); trimmed != "" {
			baseQuery = trimmed
		}
		if len(analysis.Retrieval.LocalQueries) > 0 {
			if candidate := strings.TrimSpace(analysis.Retrieval.LocalQueries[0]); candidate != "" {
				baseQuery = candidate
			}
		}
		if len(analysis.Retrieval.SearchQueries) > 0 {
			if baseQuery == "" {
				if candidate := strings.TrimSpace(analysis.Retrieval.SearchQueries[0]); candidate != "" {
					baseQuery = candidate
				}
			}
		}
	}
	query := baseQuery
	if query == "" {
		return nil
	}

	signals := s.buildRAGSignals(ctx, session, query, registry, analysis)
	directives := s.ragGate.Evaluate(ctx, signals)

	if directives.Justification == nil {
		directives.Justification = map[string]float64{}
	}
	s.recordRAGDirectiveMetadata(session, directives, signals)
	s.emitRAGDirectiveEvent(ctx, session, directives, signals)

	if analysis != nil {
		directives.Query = query
		directives.SearchSeeds = appendUniqueStrings(directives.SearchSeeds, analysis.Retrieval.SearchQueries...)
		directives.CrawlSeeds = appendUniqueStrings(directives.CrawlSeeds, analysis.Retrieval.CrawlURLs...)
	}

	if total, ok := directives.Justification["total_score"]; ok {
		s.logger.Info("RAG gate directives: retrieval=%t search=%t crawl=%t (score=%.2f)", directives.UseRetrieval, directives.UseSearch, directives.UseCrawl, total)
	} else {
		s.logger.Info("RAG gate directives: retrieval=%t search=%t crawl=%t", directives.UseRetrieval, directives.UseSearch, directives.UseCrawl)
	}

	return &directives
}

func (s *ExecutionPreparationService) recordRAGDirectiveMetadata(session *ports.Session, directives ports.RAGDirectives, signals ports.RAGSignals) {
	if session == nil {
		return
	}
	if session.Metadata == nil {
		session.Metadata = make(map[string]string)
	}
	session.Metadata["rag_last_directives"] = encodeDirectiveSummary(directives)
	if score, ok := directives.Justification["total_score"]; ok {
		session.Metadata["rag_last_plan_score"] = formatFloat(score)
	} else {
		delete(session.Metadata, "rag_last_plan_score")
	}
	session.Metadata["rag_last_hit_rate"] = formatFloat(signals.RetrievalHitRate)
	if signals.BudgetRemaining >= 0 {
		session.Metadata["rag_budget_remaining"] = formatFloat(signals.BudgetRemaining)
	}
	if signals.BudgetTarget > 0 {
		session.Metadata["rag_budget_target"] = formatFloat(signals.BudgetTarget)
	}
}

func (s *ExecutionPreparationService) emitRAGDirectiveEvent(ctx context.Context, session *ports.Session, directives ports.RAGDirectives, signals ports.RAGSignals) {
	if s.eventEmitter == nil || ports.IsSilentMode(ctx) {
		return
	}
	level := ports.GetOutputContext(ctx).Level
	sessionID := ""
	if session != nil {
		sessionID = session.ID
	}
	event := domain.NewRAGDirectivesEvaluatedEvent(
		level,
		sessionID,
		id.TaskIDFromContext(ctx),
		id.ParentTaskIDFromContext(ctx),
		directives,
		signals,
		s.clock.Now(),
	)
	s.eventEmitter.OnEvent(event)
}

func (s *ExecutionPreparationService) buildRAGSignals(ctx context.Context, session *ports.Session, query string, registry ports.ToolRegistry, analysis *TaskAnalysis) ports.RAGSignals {
	searchSeeds, crawlSeeds := s.extractSeeds(session)
	lower := strings.ToLower(query)
	isMarketing := containsAny(lower, marketingKeywords)
	isCode := containsAny(lower, codeKeywords)
	freshnessTerms := containsAny(lower, freshnessKeywords) || containsRecentYear(lower)

	signals := ports.RAGSignals{
		Query:             query,
		CanRetrieve:       true,
		SearchSeeds:       searchSeeds,
		CrawlSeeds:        crawlSeeds,
		RetrievalHitRate:  s.estimateRetrievalHitRate(session),
		FreshnessGapHours: estimateFreshnessGap(lower, isMarketing, isCode, freshnessTerms),
		IntentConfidence:  estimateIntentConfidence(lower, isMarketing, isCode),
		AllowSearch:       registryProvides(registry, "web_search"),
		AllowCrawl:        registryProvides(registry, "web_fetch") || registryProvides(registry, "browser"),
	}

	remaining, target := s.estimateBudget(ctx, session)
	signals.BudgetRemaining = remaining
	signals.BudgetTarget = target
	if analysis != nil {
		signals.SearchSeeds = appendUniqueStrings(signals.SearchSeeds, analysis.Retrieval.SearchQueries...)
		signals.CrawlSeeds = appendUniqueStrings(signals.CrawlSeeds, analysis.Retrieval.CrawlURLs...)
		if analysis.Retrieval.ShouldRetrieve {
			if signals.IntentConfidence < 0.7 {
				signals.IntentConfidence = 0.7
			}
		}
	}
	if !signals.AllowSearch && len(searchSeeds) > 0 {
		signals.SearchSeeds = nil
	}
	if !signals.AllowCrawl && len(crawlSeeds) > 0 {
		signals.CrawlSeeds = nil
	}

	return signals
}

func (s *ExecutionPreparationService) recallUserHistory(ctx context.Context, llm ports.LLMClient, task string, analysis *TaskAnalysis, session *ports.Session) *historyRecall {
	if session == nil || len(session.Messages) == 0 {
		return nil
	}

	rawMessages := historyMessagesFromSession(session.Messages)
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

func trimSessionHistoryMessages(messages []ports.Message) []ports.Message {
	if len(messages) == 0 {
		return nil
	}
	trimmed := make([]ports.Message, 0, len(messages))
	removed := false
	for _, msg := range messages {
		if shouldRecallHistoryMessage(msg) {
			removed = true
			continue
		}
		trimmed = append(trimmed, msg)
	}
	if !removed {
		return messages
	}
	if len(trimmed) == 0 {
		return nil
	}
	return trimmed
}

func shouldRecallHistoryMessage(msg ports.Message) bool {
	role := strings.TrimSpace(msg.Role)
	if strings.EqualFold(role, "system") {
		return false
	}
	if msg.Source == ports.MessageSourceSystemPrompt || msg.Source == ports.MessageSourceUserHistory {
		return false
	}
	if strings.TrimSpace(msg.Content) == "" && len(msg.Attachments) == 0 && len(msg.ToolResults) == 0 {
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

func buildPlanNodesFromTaskAnalysis(analysis *ports.TaskAnalysis) []ports.PlanNode {
	if analysis == nil {
		return nil
	}
	rootTitle := strings.TrimSpace(analysis.ActionName)
	if rootTitle == "" {
		rootTitle = strings.TrimSpace(analysis.Goal)
	}
	if rootTitle == "" && len(analysis.TaskBreakdown) == 0 {
		return nil
	}
	root := ports.PlanNode{
		ID:          "task_plan_root",
		Title:       rootTitle,
		Status:      "pending",
		Description: strings.TrimSpace(analysis.Approach),
	}
	if len(analysis.TaskBreakdown) == 0 {
		return []ports.PlanNode{root}
	}
	children := make([]ports.PlanNode, 0, len(analysis.TaskBreakdown))
	for idx, step := range analysis.TaskBreakdown {
		desc := strings.TrimSpace(step.Description)
		if desc == "" {
			continue
		}
		status := "pending"
		if step.NeedsExternalContext {
			status = "needs_context"
		}
		child := ports.PlanNode{
			ID:          fmt.Sprintf("plan_step_%d", idx+1),
			Title:       desc,
			Status:      status,
			Description: strings.TrimSpace(step.Rationale),
		}
		children = append(children, child)
	}
	if len(children) == 0 {
		return []ports.PlanNode{root}
	}
	root.Children = children
	return []ports.PlanNode{root}
}

func deriveBeliefsFromTaskAnalysis(analysis *ports.TaskAnalysis) []ports.Belief {
	if analysis == nil {
		return nil
	}
	beliefs := make([]ports.Belief, 0, len(analysis.SuccessCriteria)+len(analysis.Retrieval.KnowledgeGaps))
	for _, criterion := range analysis.SuccessCriteria {
		statement := strings.TrimSpace(criterion)
		if statement == "" {
			continue
		}
		beliefs = append(beliefs, ports.Belief{
			Statement:  statement,
			Confidence: 0.7,
			Source:     "success_criteria",
		})
	}
	for _, gap := range analysis.Retrieval.KnowledgeGaps {
		trimmed := strings.TrimSpace(gap)
		if trimmed == "" {
			continue
		}
		beliefs = append(beliefs, ports.Belief{
			Statement:  fmt.Sprintf("Unresolved gap: %s", trimmed),
			Confidence: 0.35,
			Source:     "retrieval_plan",
		})
	}
	if len(beliefs) == 0 {
		return nil
	}
	return beliefs
}

func buildKnowledgeRefsFromTaskAnalysis(analysis *ports.TaskAnalysis) []ports.KnowledgeReference {
	if analysis == nil {
		return nil
	}
	plan := analysis.Retrieval
	if !plan.ShouldRetrieve && len(plan.LocalQueries) == 0 && len(plan.SearchQueries) == 0 && len(plan.CrawlURLs) == 0 && len(plan.KnowledgeGaps) == 0 {
		return nil
	}
	ref := ports.KnowledgeReference{
		ID:          "task_analysis_retrieval",
		Description: strings.TrimSpace(plan.Notes),
	}
	ref.SOPRefs = appendUniqueStrings(nil, plan.LocalQueries...)
	ref.RAGCollections = appendUniqueStrings(nil, plan.SearchQueries...)
	ref.RAGCollections = appendUniqueStrings(ref.RAGCollections, plan.CrawlURLs...)
	ref.MemoryKeys = appendUniqueStrings(nil, plan.KnowledgeGaps...)
	if ref.Description == "" && plan.ShouldRetrieve {
		ref.Description = "Auto-generated retrieval plan"
	}
	if len(ref.SOPRefs) == 0 && len(ref.RAGCollections) == 0 && len(ref.MemoryKeys) == 0 && strings.TrimSpace(ref.Description) == "" {
		return nil
	}
	return []ports.KnowledgeReference{ref}
}

func cloneTaskAnalysisSteps(steps []ports.TaskAnalysisStep) []ports.TaskAnalysisStep {
	if len(steps) == 0 {
		return nil
	}
	cloned := make([]ports.TaskAnalysisStep, 0, len(steps))
	for _, step := range steps {
		if strings.TrimSpace(step.Description) == "" {
			continue
		}
		cloned = append(cloned, ports.TaskAnalysisStep{
			Description:          strings.TrimSpace(step.Description),
			NeedsExternalContext: step.NeedsExternalContext,
			Rationale:            strings.TrimSpace(step.Rationale),
		})
	}
	if len(cloned) == 0 {
		return nil
	}
	return cloned
}

func cloneTaskRetrievalPlan(plan ports.TaskRetrievalPlan) ports.TaskRetrievalPlan {
	cloned := ports.TaskRetrievalPlan{
		ShouldRetrieve: plan.ShouldRetrieve,
		Notes:          strings.TrimSpace(plan.Notes),
	}
	cloned.LocalQueries = append([]string(nil), plan.LocalQueries...)
	cloned.SearchQueries = append([]string(nil), plan.SearchQueries...)
	cloned.CrawlURLs = append([]string(nil), plan.CrawlURLs...)
	cloned.KnowledgeGaps = append([]string(nil), plan.KnowledgeGaps...)
	if !cloned.ShouldRetrieve {
		if len(cloned.LocalQueries) > 0 || len(cloned.SearchQueries) > 0 || len(cloned.CrawlURLs) > 0 {
			cloned.ShouldRetrieve = true
		}
	}
	return cloned
}

func appendUniqueStrings(base []string, values ...string) []string {
	if len(values) == 0 {
		if len(base) == 0 {
			return nil
		}
		return base
	}
	seen := make(map[string]struct{}, len(base)+len(values))
	result := make([]string, 0, len(base)+len(values))
	for _, value := range base {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func (s *ExecutionPreparationService) estimateRetrievalHitRate(session *ports.Session) float64 {
	if session == nil {
		return 0.55
	}
	if rate := parseMetadataFloat(session.Metadata, "rag_last_hit_rate"); rate >= 0 {
		return clamp01(rate)
	}

	var attempts float64
	var successes float64
	for _, msg := range session.Messages {
		if len(msg.ToolResults) == 0 {
			continue
		}
		for _, result := range msg.ToolResults {
			if result.Metadata == nil {
				continue
			}
			if _, ok := result.Metadata["repo_path"]; !ok {
				continue
			}
			attempts++
			if count, ok := convertToFloat64(result.Metadata["result_count"]); ok {
				if count > 0 {
					successes++
					continue
				}
			}
			if strings.TrimSpace(result.Content) != "" {
				successes++
			}
		}
	}

	if attempts == 0 {
		return 0.55
	}
	return clamp01(successes / attempts)
}

func (s *ExecutionPreparationService) estimateBudget(ctx context.Context, session *ports.Session) (float64, float64) {
	target := defaultBudgetTarget
	remaining := target

	if session != nil && session.Metadata != nil {
		if metaTarget := parseMetadataFloat(session.Metadata, "rag_budget_target"); metaTarget > 0 {
			target = metaTarget
			remaining = metaTarget
		}
		if metaRemaining := parseMetadataFloat(session.Metadata, "rag_budget_remaining"); metaRemaining >= 0 {
			remaining = metaRemaining
			if target <= 0 {
				target = metaRemaining
			}
		}
	}

	if session == nil || session.ID == "" || s.costTracker == nil {
		if remaining < 0 {
			remaining = 0
		}
		if target <= 0 {
			target = defaultBudgetTarget
		}
		return remaining, target
	}

	stats, err := s.costTracker.GetSessionStats(ctx, session.ID)
	if err != nil {
		s.logger.Debug("Failed to compute session budget from tracker: %v", err)
		if remaining < 0 {
			remaining = 0
		}
		if target <= 0 {
			target = defaultBudgetTarget
		}
		return remaining, target
	}

	spent := stats.TotalCost
	if spent >= target {
		return 0, target
	}
	calculated := target - spent
	if remaining >= 0 && remaining < calculated {
		return remaining, target
	}
	return calculated, target
}

func (s *ExecutionPreparationService) extractSeeds(session *ports.Session) ([]string, []string) {
	if session == nil || len(session.Metadata) == 0 {
		return nil, nil
	}
	var searchSeeds []string
	var crawlSeeds []string

	if raw := session.Metadata["rag_search_seeds"]; strings.TrimSpace(raw) != "" {
		searchSeeds = splitSeedList(raw)
	}
	if raw := session.Metadata["rag_crawl_seeds"]; strings.TrimSpace(raw) != "" {
		crawlSeeds = splitSeedList(raw)
	}
	return searchSeeds, crawlSeeds
}

func splitSeedList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	seeds := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			seeds = append(seeds, trimmed)
		}
	}
	if len(seeds) == 0 {
		return nil
	}
	return seeds
}

func encodeDirectiveSummary(directives ports.RAGDirectives) string {
	if !directives.UseRetrieval && !directives.UseSearch && !directives.UseCrawl {
		return "skip"
	}
	actions := make([]string, 0, 3)
	if directives.UseRetrieval {
		actions = append(actions, "retrieve")
	}
	if directives.UseSearch {
		actions = append(actions, "search")
	}
	if directives.UseCrawl {
		actions = append(actions, "crawl")
	}
	return strings.Join(actions, "+")
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 4, 64)
}

func registryProvides(registry ports.ToolRegistry, name string) bool {
	if registry == nil {
		return false
	}
	for _, def := range registry.List() {
		if strings.EqualFold(def.Name, name) {
			return true
		}
	}
	return false
}

func containsRecentYear(lower string) bool {
	for year := 2022; year <= 2026; year++ {
		if strings.Contains(lower, strconv.Itoa(year)) {
			return true
		}
	}
	return false
}

func convertToFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func parseMetadataFloat(metadata map[string]string, key string) float64 {
	if metadata == nil {
		return -1
	}
	raw := strings.TrimSpace(metadata[key])
	if raw == "" {
		return -1
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return -1
	}
	return value
}

func clamp01(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 1:
		return 1
	default:
		return value
	}
}

var marketingKeywords = []string{
	"marketing", "campaign", "seo", "brand", "audience", "growth", "trend", "insight", "competitor", "market", "persona", "content",
}

var codeKeywords = []string{
	"bug", "panic", "goroutine", "function", "struct", "interface", "compile", "error", "stack trace", "module", "unit test", "refactor",
}

var freshnessKeywords = []string{
	"latest", "current", "today", "recent", "update", "news", "release", "announcement", "breaking",
}

func containsAny(haystack string, keywords []string) bool {
	if haystack == "" {
		return false
	}
	for _, keyword := range keywords {
		if strings.Contains(haystack, keyword) {
			return true
		}
	}
	return false
}

func estimateFreshnessGap(lower string, marketing, code, freshness bool) float64 {
	gap := 48.0
	if code {
		gap = 24.0
	}
	if marketing && gap < 120.0 {
		gap = 120.0
	}
	if freshness {
		if marketing {
			if gap < 240.0 {
				gap = 240.0
			}
		} else if gap < 168.0 {
			gap = 168.0
		}
	}
	if containsRecentYear(lower) && gap < 168.0 {
		gap = 168.0
	}
	return gap
}

func estimateIntentConfidence(lower string, marketing, code bool) float64 {
	switch {
	case marketing:
		return 0.85
	case code:
		return 0.2
	case containsAny(lower, []string{"research", "analysis", "benchmark", "insight"}):
		return 0.65
	default:
		return 0.45
	}
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

// SetEnvironmentSummary updates the environment summary used when preparing prompts.
func (s *ExecutionPreparationService) SetEnvironmentSummary(summary string) {
	s.config.EnvironmentSummary = summary
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

func (s *ExecutionPreparationService) selectToolRegistry(ctx context.Context) ports.ToolRegistry {
	// Handle subagent context filtering first
	registry := s.toolRegistry
	if isSubagentContext(ctx) {
		registry = s.getRegistryWithoutSubagent()
		s.logger.Debug("Using filtered registry (subagent excluded) for nested call")
	}

	// Apply preset-based filtering
	return s.presetResolver.ResolveToolRegistry(ctx, registry, s.config.ToolPreset)
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
