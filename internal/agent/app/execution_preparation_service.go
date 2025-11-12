package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	id "alex/internal/utils/id"
)

const defaultBudgetTarget = 5.0

// ExecutionPreparationDeps enumerates the dependencies required by the preparation service.
type ExecutionPreparationDeps struct {
	LLMFactory     ports.LLMClientFactory
	ToolRegistry   ports.ToolRegistry
	SessionStore   ports.SessionStore
	ContextMgr     ports.ContextManager
	Parser         ports.FunctionCallParser
	PromptLoader   ports.PromptLoader
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
	promptLoader   ports.PromptLoader
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

	promptLoader := deps.PromptLoader
	// PromptLoader is now a required dependency - must be provided by caller

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
			PromptLoader: promptLoader,
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
		promptLoader:   promptLoader,
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

	if s.contextMgr.ShouldCompress(session.Messages, s.config.MaxTokens) {
		s.logger.Info("Context limit reached, compressing...")
		originalCount := len(session.Messages)
		compressed, err := s.contextMgr.Compress(session.Messages, s.config.MaxTokens*80/100)
		if err != nil {
			return nil, fmt.Errorf("failed to compress context: %w", err)
		}
		compressedCount := len(compressed)
		s.logger.Info("Compression complete: %d -> %d messages (%.1f%% retained)",
			originalCount, compressedCount, float64(compressedCount)/float64(originalCount)*100.0)

		// Emit compression metrics event
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

		session.Messages = compressed
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
	var analysisInfo *ports.TaskAnalysisInfo
	var taskAnalysis *ports.TaskAnalysis
	if analysis != nil && analysis.ActionName != "" {
		s.logger.Debug("Task pre-analysis: action=%s, goal=%s", analysis.ActionName, analysis.Goal)
		analysisInfo = &ports.TaskAnalysisInfo{
			Action:   analysis.ActionName,
			Goal:     analysis.Goal,
			Approach: analysis.Approach,
		}
		taskAnalysis = &ports.TaskAnalysis{
			ActionName: analysis.ActionName,
			Goal:       analysis.Goal,
			Approach:   analysis.Approach,
		}
	} else {
		s.logger.Debug("Task pre-analysis skipped or failed")
	}

	systemPrompt := s.presetResolver.ResolveSystemPrompt(ctx, task, analysisInfo, s.config.AgentPreset)
	summary := strings.TrimSpace(s.config.EnvironmentSummary)
	if summary != "" {
		if trimmed := strings.TrimSpace(systemPrompt); trimmed != "" {
			systemPrompt = trimmed + "\n\n" + summary
		} else {
			systemPrompt = summary
		}
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
	state := &domain.TaskState{
		SystemPrompt:         systemPrompt,
		Messages:             append([]domain.Message(nil), session.Messages...),
		SessionID:            session.ID,
		TaskID:               ids.TaskID,
		ParentTaskID:         ids.ParentTaskID,
		Attachments:          preloadedAttachments,
		AttachmentIterations: make(map[string]int),
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

	ragDirectives := s.evaluateRAGDirectives(ctx, session, task, toolRegistry)

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

func (s *ExecutionPreparationService) evaluateRAGDirectives(ctx context.Context, session *ports.Session, task string, registry ports.ToolRegistry) *ports.RAGDirectives {
	if s.ragGate == nil {
		return nil
	}
	query := strings.TrimSpace(task)
	if query == "" {
		return nil
	}

	signals := s.buildRAGSignals(ctx, session, query, registry)
	directives := s.ragGate.Evaluate(ctx, signals)

	if directives.Justification == nil {
		directives.Justification = map[string]float64{}
	}
	s.recordRAGDirectiveMetadata(session, directives, signals)
	s.emitRAGDirectiveEvent(ctx, session, directives, signals)

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

func (s *ExecutionPreparationService) buildRAGSignals(ctx context.Context, session *ports.Session, query string, registry ports.ToolRegistry) ports.RAGSignals {
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
	if !signals.AllowSearch && len(searchSeeds) > 0 {
		signals.SearchSeeds = nil
	}
	if !signals.AllowCrawl && len(crawlSeeds) > 0 {
		signals.CrawlSeeds = nil
	}

	return signals
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
