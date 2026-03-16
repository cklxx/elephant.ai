package context

import (
	appcontext "alex/internal/app/agent/context"
	"alex/internal/shared/utils"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/shared/logging"
	id "alex/internal/shared/utils/id"
	"gopkg.in/yaml.v3"
)

const (
	maxMemorySnapshotChars     = 10000
	maxMemorySectionChars      = 4000
	defaultActiveBufferChars   = 7000
	defaultPredictiveBufferPct = 30
	defaultPersonaConfig       = "configs/context/personas/default.yaml"
)

func (m *manager) memoryEnabled(ctx context.Context) bool {
	if m.memoryEngine == nil {
		return false
	}
	if m.memoryGate == nil {
		return true
	}
	return m.memoryGate(ctx)
}

func (m *manager) loadMemorySnapshot(ctx context.Context, session *storage.Session) string {
	if !m.memoryEnabled(ctx) {
		return ""
	}

	userID := resolveMemoryUserID(ctx, session)
	now := time.Now()
	soul, user := m.loadIdentitySnapshot(userID)

	// Load long-term, today, and yesterday memories in parallel — they are
	// independent IO operations against the same engine.
	var longTerm, today, yesterday string
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		longTerm, _ = m.memoryEngine.LoadLongTerm(ctx, userID)
	}()
	go func() {
		defer wg.Done()
		today, _ = m.memoryEngine.LoadDaily(ctx, userID, now)
	}()
	go func() {
		defer wg.Done()
		yesterday, _ = m.memoryEngine.LoadDaily(ctx, userID, now.AddDate(0, 0, -1))
	}()
	wg.Wait()

	soul = ports.TruncateRuneSnippet(soul, maxMemorySectionChars)
	user = ports.TruncateRuneSnippet(user, maxMemorySectionChars)
	longTerm = ports.TruncateRuneSnippet(longTerm, maxMemorySectionChars)
	today = ports.TruncateRuneSnippet(today, maxMemorySectionChars)
	yesterday = ports.TruncateRuneSnippet(yesterday, maxMemorySectionChars)

	var sections []string
	if soul != "" {
		sections = append(sections, fmt.Sprintf("## Identity (SOUL.md)\n%s", soul))
	}
	if user != "" {
		sections = append(sections, fmt.Sprintf("## Identity (USER.md)\n%s", user))
	}
	// Daily logs are high-churn runtime notes intended for unattended/autonomous
	// runs. Keep them out of normal sessions to avoid leaking unattended-only
	// directives into regular assistant runs.
	if appcontext.IsUnattendedContext(ctx) {
		if daily := buildUnattendedDailyLogPromptChunk(now, today, yesterday); daily != "" {
			sections = append(sections, daily)
		}
	}
	if longTerm != "" {
		sections = append(sections, fmt.Sprintf("## Long-term Memory (MEMORY.md)\n%s", longTerm))
	}
	if len(sections) == 0 {
		return ""
	}

	return ports.TruncateRuneSnippet(strings.Join(sections, "\n\n"), maxMemorySnapshotChars)
}

func (m *manager) loadIdentitySnapshot(userID string) (soul, user string) {
	if m.memoryEngine == nil {
		return "", ""
	}
	defaultSoul := m.renderSoulTemplate()
	defaultUser := renderUserTemplate(userID)
	soul, user, err := m.memoryEngine.LoadIdentity(context.Background(), userID, defaultSoul, defaultUser)
	if err != nil {
		logging.OrNop(m.logger).Warn("Failed to load identity: %v", err)
		return "", ""
	}
	return soul, user
}

func (m *manager) renderSoulTemplate() string {
	profile := m.readDefaultPersonaProfile()
	voice := strings.TrimSpace(profile.Voice)
	if voice == "" {
		voice = "You are eli, a pragmatic coding partner for production software."
	}
	return strings.TrimSpace(voice) + "\n"
}

func (m *manager) readDefaultPersonaProfile() agent.PersonaProfile {
	// Prefer using static registry if available (handles voice_path loading)
	if m != nil && m.static != nil {
		snapshot, err := m.static.currentSnapshot(context.Background())
		if err == nil {
			if profile, ok := snapshot.Personas["default"]; ok {
				return profile
			}
		}
	}

	// Fallback: read YAML directly and handle voice_path
	candidates := []string{
		strings.TrimSpace(m.defaultPersonaSourcePath()),
		defaultPersonaConfig,
	}
	seen := make(map[string]struct{}, len(candidates))
	for _, path := range candidates {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var profile agent.PersonaProfile
		if err := yaml.Unmarshal(data, &profile); err != nil {
			continue
		}
		if utils.IsBlank(profile.ID) {
			profile.ID = "default"
		}

		// Load voice from file if voice_path is set and voice is empty
		if profile.VoicePath != "" && profile.Voice == "" {
			configRoot := m.configRoot
			if configRoot == "" {
				configRoot = resolveContextConfigRoot()
			}
			repoRoot := deriveRepoRootFromConfigRoot(configRoot)
			voicePath := filepath.Join(repoRoot, profile.VoicePath)
			voiceData, readErr := os.ReadFile(voicePath)
			if readErr == nil {
				profile.Voice = string(voiceData)
			}
			// If voice loading failed, profile.Voice remains empty and caller will use fallback
		}

		return profile
	}
	return agent.PersonaProfile{
		ID:    "default",
		Tone:  "pragmatic",
		Voice: "You are eli, a pragmatic coding partner for production software.",
	}
}

func (m *manager) defaultPersonaSourcePath() string {
	var root string
	if m != nil {
		root = strings.TrimSpace(m.configRoot)
	}
	if root == "" {
		root = strings.TrimSpace(resolveContextConfigRoot())
	}
	if root == "" {
		return ""
	}
	return filepath.Join(root, "personas", "default.yaml")
}

func renderUserTemplate(userID string) string {
	displayUserID := strings.TrimSpace(userID)
	if displayUserID == "" {
		displayUserID = "(default user)"
	}
	lines := []string{
		"# USER",
		"",
		"This file defines who the assistant is helping.",
		"",
		fmt.Sprintf("- User ID: %s", displayUserID),
		"- Location: `~/.alex/memory/USER.md`.",
		"- Bootstrap behavior: if missing, this file is auto-created at session boot.",
		"",
		"## Working Profile",
		"- Add stable preferences, constraints, priorities, and collaboration style here.",
	}
	return strings.Join(lines, "\n") + "\n"
}

func buildUnattendedDailyLogPromptChunk(now time.Time, today, yesterday string) string {
	type dailyItem struct {
		date    string
		content string
	}
	items := []dailyItem{
		{date: now.Format("2006-01-02"), content: today},
		{date: now.AddDate(0, 0, -1).Format("2006-01-02"), content: yesterday},
	}

	lines := make([]string, 0, len(items)+1)
	index := 1
	for _, item := range items {
		raw := strings.TrimSpace(item.content)
		if raw == "" {
			continue
		}
		summary := summarizeUnattendedDailyLog(raw)
		lines = append(lines, fmt.Sprintf("%d | date=%s | summary=%s", index, item.date, summary))
		index++
	}
	if len(lines) == 0 {
		return ""
	}
	lines = append(lines, "Use memory_search/memory_get for full details.")
	return fmt.Sprintf("## Daily Log Digest (Unattended only)\n%s", strings.Join(lines, "\n"))
}

func summarizeUnattendedDailyLog(content string) string {
	snippet := ports.TruncateRuneSnippet(content, historyTimelineSummaryChars)
	if snippet == "" {
		return "daily memory entry available"
	}
	return snippet
}

func resolveMemoryUserID(ctx context.Context, session *storage.Session) string {
	if ctx != nil {
		if uid := strings.TrimSpace(id.UserIDFromContext(ctx)); uid != "" {
			return uid
		}
	}
	if session != nil && session.Metadata != nil {
		if uid := strings.TrimSpace(session.Metadata["user_id"]); uid != "" {
			return uid
		}
	}
	if session != nil && strings.HasPrefix(session.ID, "lark-") {
		return session.ID
	}
	return ""
}

// loadPredictiveBuffer loads predictions from the last session and searches
// memory for relevant context, returning a formatted string within budget.
func (m *manager) loadPredictiveBuffer(ctx context.Context, session *storage.Session) string {
	if !m.predictionCfg.Enabled || m.memoryEngine == nil {
		return ""
	}
	if !m.memoryEnabled(ctx) {
		return ""
	}

	userID := resolveMemoryUserID(ctx, session)
	predictions, err := m.memoryEngine.LoadPredictions(ctx, userID)
	if err != nil || len(predictions) == 0 {
		return ""
	}

	bufferPct := m.predictionCfg.PredictiveBufferPct
	if bufferPct <= 0 {
		bufferPct = defaultPredictiveBufferPct
	}
	maxChars := maxMemorySnapshotChars * bufferPct / 100

	// Search memory for each prediction and collect unique snippets.
	type hitKey struct {
		path      string
		startLine int
	}
	seen := make(map[hitKey]bool)
	var snippets []string
	totalChars := 0

	for _, prediction := range predictions {
		hits, searchErr := m.memoryEngine.Search(ctx, userID, prediction, 2, 0.4)
		if searchErr != nil {
			continue
		}
		for _, hit := range hits {
			key := hitKey{path: hit.Path, startLine: hit.StartLine}
			if seen[key] {
				continue
			}
			seen[key] = true
			snippet := strings.TrimSpace(hit.Snippet)
			if snippet == "" {
				continue
			}
			if totalChars+len(snippet) > maxChars {
				break
			}
			snippets = append(snippets, snippet)
			totalChars += len(snippet)
		}
	}

	if len(snippets) == 0 {
		// Even without search hits, surface the predictions themselves.
		var b strings.Builder
		for _, p := range predictions {
			b.WriteString("- ")
			b.WriteString(p)
			b.WriteString("\n")
		}
		return ports.TruncateRuneSnippet(b.String(), maxChars)
	}

	var b strings.Builder
	b.WriteString("Predicted needs:\n")
	for _, p := range predictions {
		b.WriteString("- ")
		b.WriteString(p)
		b.WriteString("\n")
	}
	b.WriteString("\nRelevant context:\n")
	for _, s := range snippets {
		b.WriteString(s)
		b.WriteString("\n\n")
	}
	return ports.TruncateRuneSnippet(strings.TrimSpace(b.String()), maxChars)
}
