package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// AppendDaily appends a record to the daily memory log.
func (e *MarkdownEngine) AppendDaily(_ context.Context, _ string, entry DailyEntry) (string, error) {
	content := strings.TrimSpace(entry.Content)
	if content == "" {
		return "", fmt.Errorf("content is required")
	}
	createdAt := entry.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	root, err := e.requireRoot()
	if err != nil {
		return "", err
	}
	dailyDir := filepath.Join(root, dailyDirName)
	if err := os.MkdirAll(dailyDir, 0o755); err != nil {
		return "", err
	}

	dateStr := createdAt.Format("2006-01-02")
	path := filepath.Join(dailyDir, dateStr+".md")
	if err := ensureDailyHeader(path, dateStr); err != nil {
		return "", err
	}

	title := strings.TrimSpace(entry.Title)
	if title == "" {
		title = "Note"
	}
	timeStr := createdAt.Format("3:04 PM")

	block := strings.Builder{}
	if needsLeadingNewline(path) {
		block.WriteString("\n")
	}
	block.WriteString(fmt.Sprintf("## %s - %s\n", timeStr, title))
	block.WriteString(content)
	block.WriteString("\n")

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return "", fmt.Errorf("flock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck

	if _, err := f.WriteString(block.String()); err != nil {
		return "", err
	}

	return path, nil
}

// GetLines returns a slice of lines from the given memory path.
func (e *MarkdownEngine) GetLines(_ context.Context, _ string, path string, fromLine, lineCount int) (string, error) {
	if _, err := e.requireRoot(); err != nil {
		return "", err
	}
	absPath, err := e.resolvePath(path)
	if err != nil {
		return "", err
	}
	if fromLine <= 0 {
		fromLine = 1
	}
	if lineCount <= 0 {
		lineCount = 20
	}

	lines, err := readLines(absPath)
	if err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", nil
	}
	start := fromLine - 1
	if start >= len(lines) {
		return "", fmt.Errorf("start line out of range")
	}
	end := start + lineCount
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], "\n"), nil
}

// LoadDaily reads the daily log for the given day (local time).
func (e *MarkdownEngine) LoadDaily(_ context.Context, _ string, day time.Time) (string, error) {
	root, err := e.requireRoot()
	if err != nil {
		return "", err
	}
	if day.IsZero() {
		day = time.Now()
	}
	dateStr := day.Format("2006-01-02")
	path := filepath.Join(root, dailyDirName, dateStr+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// LoadIdentity returns the soul and user identity markdown content.
// Missing files are bootstrapped from the provided defaults.
func (e *MarkdownEngine) LoadIdentity(_ context.Context, _ string, defaultSoul, defaultUser string) (soul, user string, err error) {
	root := strings.TrimSpace(e.rootDir)
	if root == "" {
		return "", "", nil
	}
	soulPath := filepath.Join(root, soulFileName)
	userPath := filepath.Join(root, userFileName)

	if err := ensureFileWithDefault(soulPath, defaultSoul); err != nil {
		return "", "", fmt.Errorf("ensure soul identity: %w", err)
	}
	if err := ensureFileWithDefault(userPath, defaultUser); err != nil {
		return "", "", fmt.Errorf("ensure user identity: %w", err)
	}

	soulBytes, err := os.ReadFile(soulPath)
	if err != nil && !os.IsNotExist(err) {
		return "", "", fmt.Errorf("read soul identity: %w", err)
	}
	userBytes, err := os.ReadFile(userPath)
	if err != nil && !os.IsNotExist(err) {
		return "", "", fmt.Errorf("read user identity: %w", err)
	}
	return strings.TrimSpace(string(soulBytes)), strings.TrimSpace(string(userBytes)), nil
}

// ListDailyEntries returns all daily memory entries sorted newest-first.
func (e *MarkdownEngine) ListDailyEntries(_ context.Context, _ string) ([]DailySnapshot, error) {
	root := strings.TrimSpace(e.rootDir)
	if root == "" {
		return nil, nil
	}
	dailyDir := filepath.Join(root, dailyDirName)
	entries, err := os.ReadDir(dailyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		files = append(files, entry.Name())
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i] > files[j]
	})

	snapshots := make([]DailySnapshot, 0, len(files))
	for _, name := range files {
		content, readErr := os.ReadFile(filepath.Join(dailyDir, name))
		if readErr != nil {
			return nil, readErr
		}
		date := strings.TrimSuffix(name, ".md")
		relPath := filepath.ToSlash(filepath.Join(dailyDirName, name))
		snapshots = append(snapshots, DailySnapshot{
			Date:    date,
			Path:    relPath,
			Content: strings.TrimSpace(string(content)),
		})
	}
	return snapshots, nil
}

// LoadLongTerm reads MEMORY.md for the user.
func (e *MarkdownEngine) LoadLongTerm(_ context.Context, _ string) (string, error) {
	root, err := e.requireRoot()
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, memoryFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// SavePredictions overwrites predictions.md with the given prediction strings.
func (e *MarkdownEngine) SavePredictions(_ context.Context, _ string, predictions []string) error {
	root, err := e.requireRoot()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	path := filepath.Join(root, predictionsFileName)

	var b strings.Builder
	b.WriteString("# Predictions\n")
	b.WriteString(fmt.Sprintf("Updated: %s\n\n", time.Now().Format("2006-01-02 15:04")))
	for _, p := range predictions {
		line := strings.TrimSpace(p)
		if line == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// LoadPredictions reads and parses bullet lines from predictions.md.
func (e *MarkdownEngine) LoadPredictions(_ context.Context, _ string) ([]string, error) {
	root, err := e.requireRoot()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, predictionsFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var predictions []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			text := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if text != "" {
				predictions = append(predictions, text)
			}
		}
	}
	return predictions, nil
}
