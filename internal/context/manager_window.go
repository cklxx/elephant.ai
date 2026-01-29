package context

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	storage "alex/internal/agent/ports/storage"
	"alex/internal/analytics/journal"
	sessionstate "alex/internal/session/state_store"
)

func (m *manager) Preload(ctx context.Context) error {
	m.preloadOnce.Do(func() {
		m.preloadErr = m.static.ensure(ctx)
	})
	return m.preloadErr
}

func (m *manager) BuildWindow(ctx context.Context, session *storage.Session, cfg agent.ContextWindowConfig) (agent.ContextWindow, error) {
	if session == nil {
		return agent.ContextWindow{}, fmt.Errorf("session required")
	}
	if err := m.Preload(ctx); err != nil {
		return agent.ContextWindow{}, err
	}

	staticSnapshot, err := m.static.currentSnapshot(ctx)
	if err != nil {
		return agent.ContextWindow{}, err
	}

	persona := selectPersona(cfg.PersonaKey, session, staticSnapshot.Personas)
	goal := selectGoal(cfg.GoalKey, staticSnapshot.Goals)
	world := selectWorld(cfg.WorldKey, session, staticSnapshot.Worlds)
	policies := mapToSlice(staticSnapshot.Policies)
	knowledge := mapToSlice(staticSnapshot.Knowledge)

	if m.sopResolver != nil {
		knowledge = m.sopResolver.ResolveKnowledgeRefs(knowledge)
	}

	messages := append([]ports.Message(nil), session.Messages...)
	if cfg.TokenLimit > 0 {
		if compacted, ok := m.AutoCompact(messages, cfg.TokenLimit); ok {
			messages = compacted
		}
	}

	dyn := agent.DynamicContext{}
	if m.stateStore != nil {
		snap, err := m.stateStore.LatestSnapshot(ctx, session.ID)
		if err == nil {
			dyn = convertSnapshotToDynamic(snap)
		} else if !errors.Is(err, sessionstate.ErrSnapshotNotFound) && m.logger != nil {
			m.logger.Warn("State snapshot read failed: %v", err)
		}
	}

	meta := deriveHistoryAwareMeta(messages, persona.ID)

	window := agent.ContextWindow{
		SessionID: session.ID,
		Messages:  messages,
		Static: agent.StaticContext{
			Persona:            persona,
			Goal:               goal,
			Policies:           policies,
			Knowledge:          knowledge,
			Tools:              buildToolHints(cfg.ToolMode, cfg.ToolPreset),
			World:              world,
			UserPersona:        session.UserPersona,
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

func (m *manager) RecordTurn(ctx context.Context, record agent.ContextTurnRecord) error {
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

func convertSnapshotToDynamic(snapshot sessionstate.Snapshot) agent.DynamicContext {
	return agent.DynamicContext{
		TurnID:            snapshot.TurnID,
		LLMTurnSeq:        snapshot.LLMTurnSeq,
		Plans:             snapshot.Plans,
		Beliefs:           snapshot.Beliefs,
		WorldState:        snapshot.World,
		Feedback:          snapshot.Feedback,
		SnapshotTimestamp: snapshot.CreatedAt,
	}
}

func convertRecordToJournal(record agent.ContextTurnRecord) journal.TurnJournalEntry {
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

func deriveHistoryAwareMeta(messages []ports.Message, personaVersion string) agent.MetaContext {
	meta := agent.MetaContext{PersonaVersion: personaVersion}
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
		meta.Memories = append(meta.Memories, agent.MemoryFragment{
			Key:       "recent_session_timeline",
			Content:   strings.Join(timeline, "\n"),
			CreatedAt: time.Now(),
			Source:    "session_history",
		})
	}
	if firstSystemSnippet != "" {
		meta.Memories = append(meta.Memories, agent.MemoryFragment{
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

func selectPersona(key string, session *storage.Session, personas map[string]agent.PersonaProfile) agent.PersonaProfile {
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
		return agent.PersonaProfile{ID: "default", Tone: "neutral"}
	}
	return personas[keys[0]]
}

func selectGoal(key string, goals map[string]agent.GoalProfile) agent.GoalProfile {
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
		return agent.GoalProfile{ID: "default"}
	}
	return goals[keys[0]]
}

func selectWorld(key string, session *storage.Session, worlds map[string]agent.WorldProfile) agent.WorldProfile {
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
		return agent.WorldProfile{ID: "default"}
	}
	return worlds[keys[0]]
}

func mapToSlice[T any](input map[string]T) []T {
	if len(input) == 0 {
		return nil
	}
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]T, 0, len(keys))
	for _, key := range keys {
		out = append(out, input[key])
	}
	return out
}
