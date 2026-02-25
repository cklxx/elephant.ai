package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func migrateLegacyUsers(root string) error {
	var errs []string
	legacyRoot := filepath.Join(root, legacyUserDirName)
	legacyEntries, err := os.ReadDir(legacyRoot)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		legacyEntries = nil
	}
	for _, entry := range legacyEntries {
		if !entry.IsDir() {
			continue
		}
		legacyUserID := entry.Name()
		legacyPath := filepath.Join(legacyRoot, legacyUserID)
		if err := mergeLegacyUserDir(root, legacyPath); err != nil {
			errs = append(errs, fmt.Sprintf("merge users/%s: %v", legacyUserID, err))
		}
	}
	if err := removeIfEmpty(legacyRoot); err != nil {
		errs = append(errs, err.Error())
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if isReservedUserDirName(name) {
			continue
		}
		sourcePath := filepath.Join(root, name)
		if err := mergeLegacyUserDir(root, sourcePath); err != nil {
			errs = append(errs, fmt.Sprintf("merge %s: %v", name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("legacy memory migration errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

func mergeLegacyUserDir(root, legacyRoot string) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	if err := mergeLegacyMarkdownFile(root, legacyRoot, memoryFileName); err != nil {
		return err
	}
	if err := mergeLegacyMarkdownFile(root, legacyRoot, userFileName); err != nil {
		return err
	}
	if err := mergeLegacyDailyLogs(root, legacyRoot); err != nil {
		return err
	}
	return os.RemoveAll(legacyRoot)
}

func mergeLegacyMarkdownFile(root, sourceRoot, fileName string) error {
	legacyPath := filepath.Join(sourceRoot, fileName)
	if _, err := os.Stat(legacyPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	newPath := filepath.Join(root, fileName)
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		return os.Rename(legacyPath, newPath)
	}
	legacyContent, err := os.ReadFile(legacyPath)
	if err != nil {
		return err
	}
	trimmed := stripLeadingHeader(string(legacyContent))
	if strings.TrimSpace(trimmed) == "" {
		return os.Remove(legacyPath)
	}
	if err := appendLegacySection(newPath, "Legacy Import", trimmed); err != nil {
		return err
	}
	return os.Remove(legacyPath)
}

func mergeLegacyDailyLogs(newRoot, legacyRoot string) error {
	legacyDaily := filepath.Join(legacyRoot, dailyDirName)
	entries, err := os.ReadDir(legacyDaily)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	newDaily := filepath.Join(newRoot, dailyDirName)
	if err := os.MkdirAll(newDaily, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		legacyPath := filepath.Join(legacyDaily, entry.Name())
		newPath := filepath.Join(newDaily, entry.Name())
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			if err := os.Rename(legacyPath, newPath); err != nil {
				return err
			}
			continue
		}
		legacyContent, err := os.ReadFile(legacyPath)
		if err != nil {
			return err
		}
		trimmed := stripLeadingHeader(string(legacyContent))
		if strings.TrimSpace(trimmed) == "" {
			if err := os.Remove(legacyPath); err != nil {
				return err
			}
			continue
		}
		if err := appendLegacySection(newPath, "Legacy Import", trimmed); err != nil {
			return err
		}
		if err := os.Remove(legacyPath); err != nil {
			return err
		}
	}
	return removeIfEmpty(legacyDaily)
}

func stripLeadingHeader(content string) string {
	lines := strings.Split(content, "\n")
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	if start < len(lines) {
		line := strings.TrimSpace(lines[start])
		if strings.HasPrefix(line, "#") {
			start++
			for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
				start++
			}
		}
	}
	return strings.TrimSpace(strings.Join(lines[start:], "\n"))
}

func appendLegacySection(path, title, content string) error {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	var b strings.Builder
	if needsLeadingNewline(path) {
		b.WriteString("\n")
	}
	dateTag := time.Now().Format("2006-01-02")
	b.WriteString(fmt.Sprintf("\n## %s (%s)\n", title, dateTag))
	b.WriteString(content)
	b.WriteString("\n")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(b.String())
	return err
}

func removeIfEmpty(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(entries) == 0 {
		return os.Remove(dir)
	}
	return nil
}
