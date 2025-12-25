package context

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/analytics/journal"
	"alex/internal/logging"
	"alex/internal/observability"
	sessionstate "alex/internal/session/state_store"
	"alex/internal/skills"
	"gopkg.in/yaml.v3"
)

type manager struct {
	threshold  float64
	configRoot string
	logger     logging.Logger
	stateStore sessionstate.Store
	metrics    *observability.ContextMetrics
	journal    journal.Writer

	static      *staticRegistry
	preloadOnce sync.Once
	preloadErr  error
}

func (m *manager) compressionThreshold() float64 {
	if m.threshold <= 0 {
		return defaultThreshold
	}
	return m.threshold
}

const (
	defaultThreshold    = 0.8
	defaultStaticTTL    = 30 * time.Minute
	contextConfigEnvVar = "ALEX_CONTEXT_CONFIG_DIR"
)

// Option configures the context manager.
type Option func(*manager)

// WithConfigRoot overrides the directory used for static context files.
func WithConfigRoot(root string) Option {
	return func(m *manager) {
		if root != "" {
			m.configRoot = root
		}
	}
}

// WithStateStore attaches a dynamic state store implementation.
func WithStateStore(store sessionstate.Store) Option {
	return func(m *manager) {
		m.stateStore = store
	}
}

// WithLogger injects a custom logger (used by tests).
func WithLogger(logger logging.Logger) Option {
	return func(m *manager) {
		if !logging.IsNil(logger) {
			m.logger = logger
		}
	}
}

// WithJournalWriter wires a turn journal writer for replay and meta-context jobs.
func WithJournalWriter(writer journal.Writer) Option {
	return func(m *manager) {
		if writer != nil {
			m.journal = writer
		}
	}
}

// WithMetrics allows overriding the metrics recorder.
func WithMetrics(metrics *observability.ContextMetrics) Option {
	return func(m *manager) {
		if metrics != nil {
			m.metrics = metrics
		}
	}
}

// NewManager constructs a layered context manager implementation.
func NewManager(opts ...Option) ports.ContextManager {
	root := resolveContextConfigRoot()

	m := &manager{
		threshold:  defaultThreshold,
		configRoot: root,
		logger:     logging.NewComponentLogger("ContextManager"),
		metrics:    observability.NewContextMetrics(),
		journal:    journal.NopWriter(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}
	if m.static == nil {
		cfgRoot := m.configRoot
		if cfgRoot == "" {
			cfgRoot = root
		}
		m.static = newStaticRegistry(cfgRoot, defaultStaticTTL, m.logger, m.metrics)
	}
	return m
}

// EstimateTokens approximates token usage by dividing rune count.
func (m *manager) EstimateTokens(messages []ports.Message) int {
	count := 0
	for _, msg := range messages {
		count += len(msg.Content) / 4
	}
	return count
}

// ShouldCompress indicates whether the context needs to be compacted.
func (m *manager) ShouldCompress(messages []ports.Message, limit int) bool {
	if limit <= 0 {
		return false
	}
	threshold := m.compressionThreshold()
	return float64(m.EstimateTokens(messages)) > float64(limit)*threshold
}

// AutoCompact applies compression when the configured threshold is exceeded.
// It returns the (possibly) compacted messages alongside a flag indicating
// whether compaction was performed.
func (m *manager) AutoCompact(messages []ports.Message, limit int) ([]ports.Message, bool) {
	if !m.ShouldCompress(messages, limit) {
		return messages, false
	}

	threshold := m.compressionThreshold()
	target := int(float64(limit) * threshold)
	compressed, err := m.Compress(messages, target)
	if err != nil {
		logging.OrNop(m.logger).Warn("Auto compaction failed: %v", err)
		return messages, false
	}

	return compressed, true
}

// Compress preserves all system prompts and summarizes everything else when the
// token budget is exceeded. The summary is inserted where non-system content
// was first removed so that later system prompts stay in their original order.
// This keeps governance instructions intact while still giving the model
// awareness of the trimmed conversation.
func (m *manager) Compress(messages []ports.Message, targetTokens int) ([]ports.Message, error) {
	if targetTokens <= 0 {
		return messages, nil
	}
	current := m.EstimateTokens(messages)
	if current <= targetTokens {
		return messages, nil
	}

	var (
		compressed            []ports.Message
		compressible          []ports.Message
		summaryInsertionIndex = -1
	)

	for _, msg := range messages {
		if msg.Source == ports.MessageSourceSystemPrompt {
			compressed = append(compressed, msg)
			continue
		}
		if summaryInsertionIndex == -1 {
			summaryInsertionIndex = len(compressed)
		}
		compressible = append(compressible, msg)
	}

	if len(compressible) == 0 {
		return messages, nil
	}

	if summary := buildCompressionSummary(compressible); summary != "" {
		compressed = append(compressed, ports.Message{
			Role:    "system",
			Content: summary,
			Source:  ports.MessageSourceSystemPrompt,
		})
		if summaryInsertionIndex >= 0 && summaryInsertionIndex < len(compressed)-1 {
			insert := compressed[len(compressed)-1]
			copy(compressed[summaryInsertionIndex+1:], compressed[summaryInsertionIndex:])
			compressed[summaryInsertionIndex] = insert
		}
	} else {
		compressed = append(compressed, compressible...)
	}

	return compressed, nil
}

func buildCompressionSummary(messages []ports.Message) string {
	if len(messages) == 0 {
		return ""
	}

	var userCount, assistantCount, toolMentions int
	var firstUser, lastUser, firstAssistant, lastAssistant string

	for _, msg := range messages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		snippet := buildCompressionSnippet(msg.Content, 140)
		switch role {
		case "user":
			userCount++
			if firstUser == "" {
				firstUser = snippet
			}
			lastUser = snippet
		case "assistant":
			assistantCount++
			toolMentions += len(msg.ToolCalls)
			if firstAssistant == "" {
				firstAssistant = snippet
			}
			lastAssistant = snippet
		case "tool":
			toolMentions++
		}
		toolMentions += len(msg.ToolResults)
	}

	parts := []string{fmt.Sprintf("Earlier conversation had %d user message(s) and %d assistant response(s)", userCount, assistantCount)}
	if toolMentions > 0 {
		parts = append(parts, fmt.Sprintf("tools were referenced %d time(s)", toolMentions))
	}

	var contextParts []string
	if firstUser != "" {
		contextParts = append(contextParts, fmt.Sprintf("user first asked: %s", firstUser))
	}
	if firstAssistant != "" {
		contextParts = append(contextParts, fmt.Sprintf("assistant first replied: %s", firstAssistant))
	}
	if lastUser != "" && lastUser != firstUser {
		contextParts = append(contextParts, fmt.Sprintf("recent user request: %s", lastUser))
	}
	if lastAssistant != "" && lastAssistant != firstAssistant {
		contextParts = append(contextParts, fmt.Sprintf("recent assistant reply: %s", lastAssistant))
	}
	if len(contextParts) > 0 {
		parts = append(parts, strings.Join(contextParts, " | "))
	}

	return fmt.Sprintf("[Earlier context compressed] %s.", strings.Join(parts, "; "))
}

func buildCompressionSnippet(content string, limit int) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || limit <= 0 {
		return trimmed
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:limit])) + "…"
}

func (m *manager) Preload(ctx context.Context) error {
	m.preloadOnce.Do(func() {
		m.preloadErr = m.static.ensure(ctx)
	})
	return m.preloadErr
}

func (m *manager) BuildWindow(ctx context.Context, session *ports.Session, cfg ports.ContextWindowConfig) (ports.ContextWindow, error) {
	if session == nil {
		return ports.ContextWindow{}, fmt.Errorf("session required")
	}
	if err := m.Preload(ctx); err != nil {
		return ports.ContextWindow{}, err
	}

	staticSnapshot, err := m.static.currentSnapshot(ctx)
	if err != nil {
		return ports.ContextWindow{}, err
	}

	persona := selectPersona(cfg.PersonaKey, session, staticSnapshot.Personas)
	goal := selectGoal(cfg.GoalKey, staticSnapshot.Goals)
	world := selectWorld(cfg.WorldKey, session, staticSnapshot.Worlds)
	policies := mapToSlice(staticSnapshot.Policies)
	knowledge := mapToSlice(staticSnapshot.Knowledge)

	messages := append([]ports.Message(nil), session.Messages...)
	if cfg.TokenLimit > 0 {
		if compacted, ok := m.AutoCompact(messages, cfg.TokenLimit); ok {
			messages = compacted
		}
	}

	dyn := ports.DynamicContext{}
	if m.stateStore != nil {
		snap, err := m.stateStore.LatestSnapshot(ctx, session.ID)
		if err == nil {
			dyn = convertSnapshotToDynamic(snap)
		} else if !errors.Is(err, sessionstate.ErrSnapshotNotFound) && m.logger != nil {
			m.logger.Warn("State snapshot read failed: %v", err)
		}
	}

	meta := deriveHistoryAwareMeta(messages, persona.ID)

	window := ports.ContextWindow{
		SessionID: session.ID,
		Messages:  messages,
		Static: ports.StaticContext{
			Persona:            persona,
			Goal:               goal,
			Policies:           policies,
			Knowledge:          knowledge,
			Tools:              buildToolHints(cfg.ToolMode, cfg.ToolPreset),
			World:              world,
			EnvironmentSummary: cfg.EnvironmentSummary,
			Version:            staticSnapshot.Version,
		},
		Dynamic: dyn,
		Meta:    meta,
	}
	omitEnvironment := strings.EqualFold(strings.TrimSpace(cfg.ToolMode), "web")
	if omitEnvironment {
		window.Static.EnvironmentSummary = ""
	}

	window.SystemPrompt = composeSystemPrompt(m.logger, window.Static, window.Dynamic, window.Meta, omitEnvironment)
	return window, nil
}

func (m *manager) RecordTurn(ctx context.Context, record ports.ContextTurnRecord) error {
	if record.SessionID == "" {
		return nil
	}
	if m.stateStore != nil {
		snapshot := sessionstate.Snapshot{
			SessionID:     record.SessionID,
			TurnID:        record.TurnID,
			LLMTurnSeq:    record.LLMTurnSeq,
			CreatedAt:     record.Timestamp,
			Summary:       record.Summary,
			Plans:         record.Plans,
			Beliefs:       record.Beliefs,
			World:         record.World,
			Diff:          record.Diff,
			Messages:      record.Messages,
			Feedback:      record.Feedback,
			KnowledgeRefs: record.KnowledgeRefs,
		}
		if snapshot.CreatedAt.IsZero() {
			snapshot.CreatedAt = time.Now()
		}
		if err := m.stateStore.SaveSnapshot(ctx, snapshot); err != nil {
			m.metrics.RecordSnapshotError()
			if m.logger != nil {
				m.logger.Warn("Failed to persist context snapshot: %v", err)
			}
			return err
		}
	}
	if m.journal != nil {
		entry := convertRecordToJournal(record)
		if err := m.journal.Write(ctx, entry); err != nil && m.logger != nil {
			m.logger.Warn("Failed to write turn journal: %v", err)
		}
	}
	return nil
}

// Helper conversions -------------------------------------------------------

func buildToolHints(mode string, preset string) []string {
	mode = strings.TrimSpace(strings.ToLower(mode))
	preset = strings.TrimSpace(preset)
	if mode == "" && preset == "" {
		return nil
	}
	if mode == "" {
		mode = "cli"
	}
	if mode == "web" {
		return []string{"mode=web", "scope=non-local"}
	}
	if preset == "" {
		preset = "full"
	}
	return []string{fmt.Sprintf("mode=%s", mode), fmt.Sprintf("preset=%s", preset)}
}

func convertSnapshotToDynamic(snapshot sessionstate.Snapshot) ports.DynamicContext {
	return ports.DynamicContext{
		TurnID:            snapshot.TurnID,
		LLMTurnSeq:        snapshot.LLMTurnSeq,
		Plans:             snapshot.Plans,
		Beliefs:           snapshot.Beliefs,
		WorldState:        snapshot.World,
		Feedback:          snapshot.Feedback,
		SnapshotTimestamp: snapshot.CreatedAt,
	}
}

func convertRecordToJournal(record ports.ContextTurnRecord) journal.TurnJournalEntry {
	entry := journal.TurnJournalEntry{
		SessionID:     record.SessionID,
		TurnID:        record.TurnID,
		LLMTurnSeq:    record.LLMTurnSeq,
		Summary:       record.Summary,
		Plans:         record.Plans,
		Beliefs:       record.Beliefs,
		World:         record.World,
		Diff:          record.Diff,
		Messages:      record.Messages,
		Feedback:      record.Feedback,
		KnowledgeRefs: record.KnowledgeRefs,
	}
	if record.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	} else {
		entry.Timestamp = record.Timestamp
	}
	return entry
}

const historyTimelineLimit = 8

func deriveHistoryAwareMeta(messages []ports.Message, personaVersion string) ports.MetaContext {
	meta := ports.MetaContext{PersonaVersion: personaVersion}
	if len(messages) == 0 {
		return meta
	}

	var firstSystemSnippet string
	var lastUserSnippet string
	var lastAssistantSnippet string
	var lastToolSnippet string

	for _, msg := range messages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		if firstSystemSnippet == "" && (msg.Source == ports.MessageSourceSystemPrompt || role == "system") {
			firstSystemSnippet = buildCompressionSnippet(msg.Content, 320)
		}
		switch role {
		case "user":
			if snippet := buildCompressionSnippet(msg.Content, 200); snippet != "" {
				lastUserSnippet = snippet
			}
		case "assistant":
			if snippet := buildCompressionSnippet(msg.Content, 200); snippet != "" {
				lastAssistantSnippet = snippet
			}
		case "tool":
			if snippet := buildCompressionSnippet(msg.Content, 200); snippet != "" {
				lastToolSnippet = snippet
			}
		}
	}

	if timeline := buildHistoryTimeline(messages, historyTimelineLimit); len(timeline) > 0 {
		meta.Memories = append(meta.Memories, ports.MemoryFragment{
			Key:       "recent_session_timeline",
			Content:   strings.Join(timeline, "\n"),
			CreatedAt: time.Now(),
			Source:    "session_history",
		})
	}
	if firstSystemSnippet != "" {
		meta.Memories = append(meta.Memories, ports.MemoryFragment{
			Key:       "session_system_prompt",
			Content:   firstSystemSnippet,
			CreatedAt: time.Now(),
			Source:    "session_history",
		})
	}
	if lastUserSnippet != "" {
		meta.Recommendations = append(meta.Recommendations, fmt.Sprintf("Latest user request: %s", lastUserSnippet))
	}
	if lastAssistantSnippet != "" {
		meta.Recommendations = append(meta.Recommendations, fmt.Sprintf("Previous assistant response: %s", lastAssistantSnippet))
	}
	if lastToolSnippet != "" {
		meta.Recommendations = append(meta.Recommendations, fmt.Sprintf("Latest tool insight: %s", lastToolSnippet))
	}
	return meta
}

func buildHistoryTimeline(messages []ports.Message, limit int) []string {
	if len(messages) == 0 || limit <= 0 {
		return nil
	}
	start := len(messages) - limit
	if start < 0 {
		start = 0
	}
	timeline := make([]string, 0, len(messages)-start)
	for idx := start; idx < len(messages); idx++ {
		msg := messages[idx]
		snippet := buildCompressionSnippet(msg.Content, 160)
		if snippet == "" {
			snippet = "(no visible content)"
		}
		label := normalizeHistoryLabel(msg)
		timeline = append(timeline, fmt.Sprintf("%02d. %s: %s", idx-start+1, label, snippet))
	}
	return timeline
}

func normalizeHistoryLabel(msg ports.Message) string {
	if msg.Source == ports.MessageSourceSystemPrompt {
		return "system"
	}
	if msg.Source == ports.MessageSourceUserInput {
		return "user"
	}
	if msg.Source == ports.MessageSourceAssistantReply {
		return "assistant"
	}
	if msg.Source == ports.MessageSourceToolResult {
		if msg.ToolCallID != "" {
			return fmt.Sprintf("tool[%s]", msg.ToolCallID)
		}
		return "tool"
	}
	role := strings.ToLower(strings.TrimSpace(msg.Role))
	if role == "" {
		return "message"
	}
	return role
}

func composeSystemPrompt(logger logging.Logger, static ports.StaticContext, dynamic ports.DynamicContext, meta ports.MetaContext, omitEnvironment bool) string {
	sections := []string{
		buildIdentitySection(static.Persona),
		buildGoalsSection(static.Goal),
		buildPoliciesSection(static.Policies),
		buildKnowledgeSection(static.Knowledge),
		buildSkillsSection(logger),
	}
	if !omitEnvironment {
		sections = append(sections, buildEnvironmentSection(static))
	}
	sections = append(sections, buildDynamicSection(dynamic), buildMetaSection(meta))
	var compact []string
	for _, section := range sections {
		if trimmed := strings.TrimSpace(section); trimmed != "" {
			compact = append(compact, trimmed)
		}
	}
	return strings.Join(compact, "\n\n")
}

func buildSkillsSection(logger logging.Logger) string {
	library, err := skills.DefaultLibrary()
	if err != nil {
		logging.OrNop(logger).Warn("Failed to load skills: %v", err)
		return ""
	}
	return skills.IndexMarkdown(library)
}

func buildIdentitySection(persona ports.PersonaProfile) string {
	var builder strings.Builder
	voice := strings.TrimSpace(persona.Voice)
	if voice == "" {
		voice = "You are ALEX, an enterprise-grade assistant focused on secure, testable software delivery."
	}
	builder.WriteString("# Identity & Persona\n\n")
	builder.WriteString(voice)
	meta := formatBulletList(filterNonEmpty([]string{
		formatKeyValue("Tone", persona.Tone),
		formatKeyValue("Decision Style", persona.DecisionStyle),
		formatKeyValue("Risk Profile", persona.RiskProfile),
	}))
	if meta != "" {
		builder.WriteString("\n")
		builder.WriteString(meta)
	}
	return strings.TrimSpace(builder.String())
}

func buildGoalsSection(goal ports.GoalProfile) string {
	var lines []string
	if len(goal.LongTerm) > 0 {
		lines = append(lines, "Long-term:")
		lines = append(lines, prependBullet(goal.LongTerm, 1)...)
	}
	if len(goal.MidTerm) > 0 {
		lines = append(lines, "Mid-term:")
		lines = append(lines, prependBullet(goal.MidTerm, 1)...)
	}
	if len(goal.SuccessMetrics) > 0 {
		lines = append(lines, "Success metrics:")
		lines = append(lines, prependBullet(goal.SuccessMetrics, 1)...)
	}
	if len(lines) == 0 {
		return ""
	}
	return formatSection("# Mission Objectives", lines)
}

func buildPoliciesSection(policies []ports.PolicyRule) string {
	if len(policies) == 0 {
		return ""
	}
	var lines []string
	for _, policy := range policies {
		label := formatPolicyLabel(policy.ID)
		lines = append(lines, fmt.Sprintf("%s:", label))
		lines = append(lines, prependBullet(policy.HardConstraints, 1, "Hard constraints")...)
		lines = append(lines, prependBullet(policy.SoftPreferences, 1, "Soft preferences")...)
		lines = append(lines, prependBullet(policy.RewardHooks, 1, "Reward hooks")...)
	}
	return formatSection("# Guardrails & Policies", lines)
}

func buildKnowledgeSection(knowledge []ports.KnowledgeReference) string {
	if len(knowledge) == 0 {
		return ""
	}
	var lines []string
	for _, ref := range knowledge {
		label := ref.ID
		if label == "" {
			label = ref.Description
		}
		label = strings.TrimSpace(label)
		if label == "" {
			label = "knowledge"
		}
		lines = append(lines, fmt.Sprintf("%s:", label))
		if ref.Description != "" {
			lines = append(lines, fmt.Sprintf("  - Summary: %s", ref.Description))
		}
		if len(ref.SOPRefs) > 0 {
			lines = append(lines, fmt.Sprintf("  - SOP refs: %s", strings.Join(ref.SOPRefs, ", ")))
		}
		if len(ref.RAGCollections) > 0 {
			lines = append(lines, fmt.Sprintf("  - RAG collections: %s", strings.Join(ref.RAGCollections, ", ")))
		}
		if len(ref.MemoryKeys) > 0 {
			lines = append(lines, fmt.Sprintf("  - Memory keys: %s", strings.Join(ref.MemoryKeys, ", ")))
		}
	}
	return formatSection("# Knowledge & Experience", lines)
}

func buildEnvironmentSection(static ports.StaticContext) string {
	var lines []string
	if env := strings.TrimSpace(static.EnvironmentSummary); env != "" {
		lines = append(lines, fmt.Sprintf("Environment summary: %s", env))
	}
	if world := strings.TrimSpace(static.World.Environment); world != "" {
		lines = append(lines, fmt.Sprintf("World: %s", world))
	}
	if len(static.World.Capabilities) > 0 {
		lines = append(lines, fmt.Sprintf("Capabilities: %s", strings.Join(static.World.Capabilities, ", ")))
	}
	if len(static.World.Limits) > 0 {
		lines = append(lines, fmt.Sprintf("Limits: %s", strings.Join(static.World.Limits, ", ")))
	}
	if len(static.World.CostModel) > 0 {
		lines = append(lines, fmt.Sprintf("Cost awareness: %s", strings.Join(static.World.CostModel, ", ")))
	}
	if len(static.Tools) > 0 {
		lines = append(lines, fmt.Sprintf("Tool access: %s", strings.Join(static.Tools, ", ")))
	}
	if len(lines) == 0 {
		return ""
	}
	return formatSection("# Operating Environment", lines)
}

func buildDynamicSection(dynamic ports.DynamicContext) string {
	var lines []string
	if dynamic.TurnID > 0 || dynamic.LLMTurnSeq > 0 {
		lines = append(lines, fmt.Sprintf("Turn: %d (llm_seq=%d)", dynamic.TurnID, dynamic.LLMTurnSeq))
	}
	if !dynamic.SnapshotTimestamp.IsZero() {
		lines = append(lines, fmt.Sprintf("Snapshot captured: %s", dynamic.SnapshotTimestamp.Format(time.RFC3339)))
	}
	if len(dynamic.Plans) > 0 {
		lines = append(lines, "Plans:")
		lines = append(lines, formatPlanTree(dynamic.Plans, 1)...)
	}
	if len(dynamic.Beliefs) > 0 {
		beliefs := make([]string, 0, len(dynamic.Beliefs))
		for _, belief := range dynamic.Beliefs {
			beliefs = append(beliefs, fmt.Sprintf("%s (confidence %.2f)", belief.Statement, belief.Confidence))
		}
		lines = append(lines, "Beliefs:")
		lines = append(lines, prependBullet(beliefs, 1)...)
	}
	if len(dynamic.WorldState) > 0 {
		lines = append(lines, "World state summary:")
		lines = append(lines, summarizeMap(dynamic.WorldState, 1)...)
	}
	if len(dynamic.Feedback) > 0 {
		lines = append(lines, "Feedback signals:")
		var feedback []string
		for _, signal := range dynamic.Feedback {
			feedback = append(feedback, fmt.Sprintf("%s — %s (%.2f)", signal.Kind, signal.Message, signal.Value))
		}
		lines = append(lines, prependBullet(feedback, 1)...)
	}
	if len(lines) == 0 {
		return ""
	}
	return formatSection("# Live Session State", lines)
}

func buildMetaSection(meta ports.MetaContext) string {
	var lines []string
	if meta.PersonaVersion != "" {
		lines = append(lines, fmt.Sprintf("Persona version: %s", meta.PersonaVersion))
	}
	if len(meta.Memories) > 0 {
		lines = append(lines, "Memories:")
		var memoLines []string
		for _, memory := range meta.Memories {
			stamp := memory.CreatedAt.Format("2006-01-02")
			memoLines = append(memoLines, fmt.Sprintf("%s — %s (%s)", memory.Content, memory.Key, stamp))
		}
		lines = append(lines, prependBullet(memoLines, 1)...)
	}
	if len(meta.Recommendations) > 0 {
		lines = append(lines, "Recommendations:")
		lines = append(lines, prependBullet(meta.Recommendations, 1)...)
	}
	if len(lines) == 0 {
		return ""
	}
	return formatSection("# Meta Stewardship Directives", lines)
}

func formatSection(title string, lines []string) string {
	var builder strings.Builder
	if title != "" {
		builder.WriteString(title)
		builder.WriteString("\n")
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		builder.WriteString(line)
		if !strings.HasSuffix(line, "\n") {
			builder.WriteString("\n")
		}
	}
	return strings.TrimSpace(builder.String())
}

func formatBulletList(items []string) string {
	if len(items) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		builder.WriteString("- ")
		builder.WriteString(trimmed)
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

func prependBullet(items []string, depth int, prefix ...string) []string {
	var lines []string
	if len(prefix) > 0 {
		lines = append(lines, strings.Repeat("  ", depth-1)+prefix[0]+":")
	}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		lines = append(lines, strings.Repeat("  ", depth)+"- "+trimmed)
	}
	return lines
}

func filterNonEmpty(items []string) []string {
	var result []string
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func formatKeyValue(key, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return fmt.Sprintf("%s: %s", key, value)
}

func formatPlanTree(nodes []ports.PlanNode, depth int) []string {
	var lines []string
	for _, node := range nodes {
		title := strings.TrimSpace(node.Title)
		if title == "" {
			title = node.ID
		}
		entry := title
		if node.Status != "" {
			entry = fmt.Sprintf("%s [%s]", entry, node.Status)
		}
		if node.Description != "" {
			entry = fmt.Sprintf("%s — %s", entry, node.Description)
		}
		lines = append(lines, strings.Repeat("  ", depth)+"- "+strings.TrimSpace(entry))
		if len(node.Children) > 0 {
			lines = append(lines, formatPlanTree(node.Children, depth+1)...)
		}
	}
	return lines
}

func summarizeMap(data map[string]any, depth int) []string {
	if len(data) == 0 {
		return nil
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var lines []string
	for _, key := range keys {
		value := data[key]
		lines = append(lines, strings.Repeat("  ", depth)+fmt.Sprintf("- %s: %v", key, value))
	}
	return lines
}

func formatPolicyLabel(id string) string {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return "Policy"
	}
	runes := []rune(trimmed)
	if len(runes) == 0 {
		return "Policy"
	}
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	return string(runes)
}

func selectPersona(key string, session *ports.Session, personas map[string]ports.PersonaProfile) ports.PersonaProfile {
	if key == "" && session != nil && session.Metadata != nil {
		key = session.Metadata["persona"]
	}
	if persona, ok := personas[key]; ok {
		return persona
	}
	if persona, ok := personas["default"]; ok {
		return persona
	}
	// Fallback to deterministic order
	keys := make([]string, 0, len(personas))
	for id := range personas {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ports.PersonaProfile{ID: "default", Tone: "neutral"}
	}
	return personas[keys[0]]
}

func selectGoal(key string, goals map[string]ports.GoalProfile) ports.GoalProfile {
	if goal, ok := goals[key]; ok {
		return goal
	}
	if goal, ok := goals["default"]; ok {
		return goal
	}
	keys := make([]string, 0, len(goals))
	for id := range goals {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ports.GoalProfile{ID: "default"}
	}
	return goals[keys[0]]
}

func selectWorld(key string, session *ports.Session, worlds map[string]ports.WorldProfile) ports.WorldProfile {
	if key == "" && session != nil && session.Metadata != nil {
		key = session.Metadata["world"]
	}
	if world, ok := worlds[key]; ok {
		return world
	}
	if world, ok := worlds["default"]; ok {
		return world
	}
	keys := make([]string, 0, len(worlds))
	for id := range worlds {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ports.WorldProfile{ID: "default"}
	}
	return worlds[keys[0]]
}

func mapToSlice[T any](input map[string]T) []T {
	if len(input) == 0 {
		return nil
	}
	keys := make([]string, 0, len(input))
	for k := range input {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]T, 0, len(keys))
	for _, k := range keys {
		result = append(result, input[k])
	}
	return result
}

// Static registry ----------------------------------------------------------

type staticRegistry struct {
	root    string
	ttl     time.Duration
	logger  logging.Logger
	metrics *observability.ContextMetrics

	mu       sync.RWMutex
	snapshot staticSnapshot
	expires  time.Time
}

type staticSnapshot struct {
	Version   string
	LoadedAt  time.Time
	Personas  map[string]ports.PersonaProfile
	Goals     map[string]ports.GoalProfile
	Policies  map[string]ports.PolicyRule
	Knowledge map[string]ports.KnowledgeReference
	Worlds    map[string]ports.WorldProfile
}

func newStaticRegistry(root string, ttl time.Duration, logger logging.Logger, metrics *observability.ContextMetrics) *staticRegistry {
	if ttl <= 0 {
		ttl = defaultStaticTTL
	}
	if logging.IsNil(logger) {
		logger = logging.NewComponentLogger("ContextStaticRegistry")
	}
	if metrics == nil {
		metrics = observability.NewContextMetrics()
	}
	return &staticRegistry{
		root:    root,
		ttl:     ttl,
		logger:  logger,
		metrics: metrics,
	}
}

func (r *staticRegistry) ensure(ctx context.Context) error {
	_, err := r.currentSnapshot(ctx)
	return err
}

func (r *staticRegistry) currentSnapshot(ctx context.Context) (staticSnapshot, error) {
	r.mu.RLock()
	if !r.snapshot.LoadedAt.IsZero() && time.Now().Before(r.expires) {
		snap := r.snapshot
		r.mu.RUnlock()
		return snap, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.snapshot.LoadedAt.IsZero() && time.Now().Before(r.expires) {
		return r.snapshot, nil
	}

	if r.metrics != nil {
		r.metrics.RecordStaticCacheMiss()
	}
	snap, err := r.load(ctx)
	if err != nil {
		return staticSnapshot{}, err
	}
	r.snapshot = snap
	r.expires = time.Now().Add(r.ttl)
	return snap, nil
}

func (r *staticRegistry) load(_ context.Context) (staticSnapshot, error) {
	personas, err := loadPersonas(filepath.Join(r.root, "personas"))
	if err != nil {
		return staticSnapshot{}, err
	}
	goals, err := loadGoals(filepath.Join(r.root, "goals"))
	if err != nil {
		return staticSnapshot{}, err
	}
	policies, err := loadPolicies(filepath.Join(r.root, "policies"))
	if err != nil {
		return staticSnapshot{}, err
	}
	knowledge, err := loadKnowledge(filepath.Join(r.root, "knowledge"))
	if err != nil {
		return staticSnapshot{}, err
	}
	worlds, err := loadWorlds(filepath.Join(r.root, "worlds"))
	if err != nil {
		return staticSnapshot{}, err
	}

	version := hashStaticSnapshot(personas, goals, policies, knowledge, worlds)
	snap := staticSnapshot{
		Version:   version,
		LoadedAt:  time.Now(),
		Personas:  personas,
		Goals:     goals,
		Policies:  policies,
		Knowledge: knowledge,
		Worlds:    worlds,
	}
	if r.logger != nil {
		r.logger.Info("Static context cache refreshed (personas=%d goals=%d)", len(personas), len(goals))
	}
	return snap, nil
}

// YAML loaders -------------------------------------------------------------

func loadPersonas(dir string) (map[string]ports.PersonaProfile, error) {
	entries, err := readYAMLDir(dir)
	if err != nil {
		return nil, err
	}
	personas := make(map[string]ports.PersonaProfile, len(entries))
	for _, content := range entries {
		var profile ports.PersonaProfile
		if err := yaml.Unmarshal(content, &profile); err != nil {
			return nil, fmt.Errorf("decode persona: %w", err)
		}
		if profile.ID == "" {
			profile.ID = filepath.Base(dir)
		}
		personas[profile.ID] = profile
	}
	if len(personas) == 0 {
		personas["default"] = ports.PersonaProfile{ID: "default", Tone: "neutral"}
	}
	return personas, nil
}

func loadGoals(dir string) (map[string]ports.GoalProfile, error) {
	entries, err := readYAMLDir(dir)
	if err != nil {
		return nil, err
	}
	goals := make(map[string]ports.GoalProfile, len(entries))
	for _, content := range entries {
		var profile ports.GoalProfile
		if err := yaml.Unmarshal(content, &profile); err != nil {
			return nil, fmt.Errorf("decode goal: %w", err)
		}
		if profile.ID == "" {
			profile.ID = filepath.Base(dir)
		}
		goals[profile.ID] = profile
	}
	if len(goals) == 0 {
		goals["default"] = ports.GoalProfile{ID: "default"}
	}
	return goals, nil
}

func loadPolicies(dir string) (map[string]ports.PolicyRule, error) {
	entries, err := readYAMLDir(dir)
	if err != nil {
		return nil, err
	}
	policies := make(map[string]ports.PolicyRule, len(entries))
	for _, content := range entries {
		var policy ports.PolicyRule
		if err := yaml.Unmarshal(content, &policy); err != nil {
			return nil, fmt.Errorf("decode policy: %w", err)
		}
		if policy.ID == "" {
			policy.ID = filepath.Base(dir)
		}
		policies[policy.ID] = policy
	}
	return policies, nil
}

func loadKnowledge(dir string) (map[string]ports.KnowledgeReference, error) {
	entries, err := readYAMLDir(dir)
	if err != nil {
		return nil, err
	}
	knowledge := make(map[string]ports.KnowledgeReference, len(entries))
	for _, content := range entries {
		var ref ports.KnowledgeReference
		if err := yaml.Unmarshal(content, &ref); err != nil {
			return nil, fmt.Errorf("decode knowledge pack: %w", err)
		}
		if ref.ID == "" {
			ref.ID = filepath.Base(dir)
		}
		knowledge[ref.ID] = ref
	}
	return knowledge, nil
}

func loadWorlds(dir string) (map[string]ports.WorldProfile, error) {
	entries, err := readYAMLDir(dir)
	if err != nil {
		return nil, err
	}
	worlds := make(map[string]ports.WorldProfile, len(entries))
	for _, content := range entries {
		var profile ports.WorldProfile
		if err := yaml.Unmarshal(content, &profile); err != nil {
			return nil, fmt.Errorf("decode world profile: %w", err)
		}
		if profile.ID == "" {
			profile.ID = filepath.Base(dir)
		}
		worlds[profile.ID] = profile
	}
	if len(worlds) == 0 {
		worlds["default"] = ports.WorldProfile{ID: "default"}
	}
	return worlds, nil
}

func readYAMLDir(dir string) ([][]byte, error) {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("context directory %s missing", dir)
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	var blobs [][]byte
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".yaml") && !strings.HasSuffix(d.Name(), ".yml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		blobs = append(blobs, data)
		return nil
	})
	return blobs, err
}

func hashStaticSnapshot(
	personas map[string]ports.PersonaProfile,
	goals map[string]ports.GoalProfile,
	policies map[string]ports.PolicyRule,
	knowledge map[string]ports.KnowledgeReference,
	worlds map[string]ports.WorldProfile,
) string {
	h := sha256.New()
	encodeMapForHash(h, "personas", personas)
	encodeMapForHash(h, "goals", goals)
	encodeMapForHash(h, "policies", policies)
	encodeMapForHash(h, "knowledge", knowledge)
	encodeMapForHash(h, "worlds", worlds)
	return hex.EncodeToString(h.Sum(nil))
}

func encodeMapForHash[T any](h hash.Hash, label string, entries map[string]T) {
	if h == nil {
		return
	}
	h.Write([]byte(label))
	if len(entries) == 0 {
		h.Write([]byte{0})
		return
	}
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k))
		data, err := json.Marshal(entries[k])
		if err != nil {
			if _, writeErr := fmt.Fprintf(h, "%v", entries[k]); writeErr != nil {
				_, _ = io.WriteString(h, fmt.Sprint(entries[k]))
			}
			continue
		}
		h.Write(data)
	}
}

func resolveContextConfigRoot() string {
	if envRoot, ok := os.LookupEnv(contextConfigEnvVar); ok {
		if trimmed := strings.TrimSpace(envRoot); trimmed != "" {
			return trimmed
		}
	}
	if resolved := locateExistingContextRoot(); resolved != "" {
		return resolved
	}
	return filepath.Join("configs", "context")
}

func locateExistingContextRoot() string {
	var starts []string
	if wd, err := os.Getwd(); err == nil && wd != "" {
		starts = append(starts, filepath.Clean(wd))
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		exeDir := filepath.Clean(filepath.Dir(exe))
		starts = append(starts, exeDir)
	}
	seen := make(map[string]struct{}, len(starts))
	for _, start := range starts {
		if start == "" {
			continue
		}
		if _, ok := seen[start]; ok {
			continue
		}
		seen[start] = struct{}{}
		if resolved := searchContextRootFromDir(start); resolved != "" {
			return resolved
		}
	}
	return ""
}

func searchContextRootFromDir(start string) string {
	dir := filepath.Clean(start)
	if dir == "" {
		return ""
	}
	for {
		candidate := filepath.Join(dir, "configs", "context")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir || parent == "" {
			break
		}
		dir = parent
	}
	candidate := filepath.Join(dir, "configs", "context")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate
	}
	return ""
}
