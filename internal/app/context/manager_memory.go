package context

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	if today != "" {
		sections = append(sections, fmt.Sprintf("## Daily Log (%s)\n%s", now.Format("2006-01-02"), today))
	}
	if yesterday != "" {
		sections = append(sections, fmt.Sprintf("## Daily Log (%s)\n%s", now.AddDate(0, 0, -1).Format("2006-01-02"), yesterday))
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

	var lines []string
	lines = append(lines,
		"# SOUL",
		"",
		"This file defines who the assistant is.",
		"",
		fmt.Sprintf("- Canonical source: `%s`", defaultPersonaConfig),
	)
	if sourcePath := strings.TrimSpace(m.defaultPersonaSourcePath()); sourcePath != "" && sourcePath != defaultPersonaConfig {
		lines = append(lines, fmt.Sprintf("- Resolved source path: `%s`", sourcePath))
	}
	lines = append(lines,
		"- Bootstrap behavior: if missing, this file is auto-created from the persona source.",
		"",
		"## Voice",
		voice,
	)
	if tone := strings.TrimSpace(profile.Tone); tone != "" {
		lines = append(lines, "", fmt.Sprintf("- Tone: %s", tone))
	}
	if style := strings.TrimSpace(profile.DecisionStyle); style != "" {
		lines = append(lines, fmt.Sprintf("- Decision style: %s", style))
	}
	if risk := strings.TrimSpace(profile.RiskProfile); risk != "" {
		lines = append(lines, fmt.Sprintf("- Risk profile: %s", risk))
	}
	return strings.Join(lines, "\n") + "\n"
}

func (m *manager) readDefaultPersonaProfile() agent.PersonaProfile {
	candidates := []string{
		strings.TrimSpace(m.defaultPersonaSourcePath()),
		defaultPersonaConfig,
	}
	for _, path := range candidates {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var profile agent.PersonaProfile
		if err := yaml.Unmarshal(data, &profile); err != nil {
			continue
		}
		if strings.TrimSpace(profile.ID) == "" {
			profile.ID = "default"
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
	if m == nil {
		return ""
	}
	root := strings.TrimSpace(m.configRoot)
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
	if strings.TrimSpace(content) == "" {
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
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || limit <= 0 {
		return trimmed
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:limit])) + "..."
}
