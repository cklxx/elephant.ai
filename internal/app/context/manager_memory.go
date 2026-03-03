package context

import (
	appcontext "alex/internal/app/agent/context"
	"alex/internal/shared/utils"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/shared/logging"
	id "alex/internal/shared/utils/id"
	"gopkg.in/yaml.v3"
)

const (
	maxMemorySnapshotChars = 10000
	maxMemorySectionChars  = 4000
	soulFileName           = "SOUL.md"
	userFileName           = "USER.md"
	defaultPersonaConfig   = "configs/context/personas/default.yaml"
)

func (m *manager) memoryEnabled(ctx context.Context) bool {
	if m == nil || m.memoryEngine == nil {
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
	soul, user, soulPath, userPath := m.loadIdentitySnapshot(userID)

	longTerm, _ := m.memoryEngine.LoadLongTerm(ctx, userID)
	today, _ := m.memoryEngine.LoadDaily(ctx, userID, now)
	yesterday, _ := m.memoryEngine.LoadDaily(ctx, userID, now.AddDate(0, 0, -1))

	soul = truncateMemorySection(soul, maxMemorySectionChars)
	user = truncateMemorySection(user, maxMemorySectionChars)
	longTerm = truncateMemorySection(longTerm, maxMemorySectionChars)
	today = truncateMemorySection(today, maxMemorySectionChars)
	yesterday = truncateMemorySection(yesterday, maxMemorySectionChars)

	var sections []string
	if soul != "" {
		sections = append(sections, fmt.Sprintf("## Identity (SOUL.md: %s)\n%s", soulPath, soul))
	}
	if user != "" {
		sections = append(sections, fmt.Sprintf("## Identity (USER.md: %s)\n%s", userPath, user))
	}
	// Daily logs are high-churn runtime notes intended for kernel/autonomous
	// loops. Keep them out of normal sessions to avoid leaking kernel-only
	// directives into regular assistant runs.
	if appcontext.IsUnattendedContext(ctx) {
		if daily := buildKernelDailyLogPromptChunk(now, today, yesterday); daily != "" {
			sections = append(sections, daily)
		}
	}
	if longTerm != "" {
		sections = append(sections, fmt.Sprintf("## Long-term Memory (MEMORY.md)\n%s", longTerm))
	}
	if len(sections) == 0 {
		return ""
	}

	return truncateMemorySection(strings.Join(sections, "\n\n"), maxMemorySnapshotChars)
}

func (m *manager) loadIdentitySnapshot(userID string) (soul string, user string, soulPath string, userPath string) {
	if m == nil || m.memoryEngine == nil {
		return "", "", "", ""
	}
	root := strings.TrimSpace(m.memoryEngine.RootDir())
	if root == "" {
		return "", "", "", ""
	}

	soulPath = filepath.Join(root, soulFileName)
	userPath = filepath.Join(root, userFileName)

	if err := ensureMarkdownFileIfMissing(soulPath, m.renderSoulTemplate); err != nil {
		logging.OrNop(m.logger).Warn("Failed to bootstrap SOUL.md: %v", err)
	}
	if err := ensureMarkdownFileIfMissing(userPath, func() string { return renderUserTemplate(userID) }); err != nil {
		logging.OrNop(m.logger).Warn("Failed to bootstrap USER.md: %v", err)
	}

	soul, _ = readMarkdownFile(soulPath)
	user, _ = readMarkdownFile(userPath)
	return soul, user, soulPath, userPath
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

func buildKernelDailyLogPromptChunk(now time.Time, today, yesterday string) string {
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
		summary := summarizeKernelDailyLog(raw)
		lines = append(lines, fmt.Sprintf("%d | date=%s | summary=%s", index, item.date, summary))
		index++
	}
	if len(lines) == 0 {
		return ""
	}
	lines = append(lines, "Use memory_search/memory_get for full details.")
	return fmt.Sprintf("## Daily Log Digest (Kernel only)\n%s", strings.Join(lines, "\n"))
}

func summarizeKernelDailyLog(content string) string {
	snippet := buildCompressionSnippet(content, historyTimelineSummaryChars)
	if snippet == "" {
		return "daily memory entry available"
	}
	if containsNonASCII(snippet) {
		return "non-English daily memory available (open via memory_search)."
	}
	return snippet
}

func containsNonASCII(value string) bool {
	for _, r := range value {
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		if unicode.IsPrint(r) && r > unicode.MaxASCII {
			return true
		}
	}
	return false
}

func ensureMarkdownFileIfMissing(path string, contentBuilder func() string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path is required")
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if contentBuilder == nil {
		return fmt.Errorf("content builder is required for %s", path)
	}
	content := contentBuilder()
	if utils.IsBlank(content) {
		return fmt.Errorf("file content is required for %s", path)
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644)
}

func readMarkdownFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
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

func truncateMemorySection(content string, limit int) string {
	return truncateWithEllipsis(content, limit)
}
