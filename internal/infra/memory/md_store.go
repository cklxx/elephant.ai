package memory

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"alex/internal/shared/utils"
)

const (
	defaultSearchMax      = 6
	defaultSearchMinScore = 0.35
	chunkTokenSize        = 400
	chunkTokenOverlap     = 80
	maxSnippetChars       = 600
)

// MarkdownEngine implements Engine using Markdown files on disk.
type MarkdownEngine struct {
	rootDir string
	indexer *Indexer

	chunkTokens  int
	chunkOverlap int
}

// NewMarkdownEngine constructs a Markdown engine rooted at dir.
func NewMarkdownEngine(dir string) *MarkdownEngine {
	return &MarkdownEngine{
		rootDir:      dir,
		chunkTokens:  chunkTokenSize,
		chunkOverlap: chunkTokenOverlap,
	}
}

// SetIndexer attaches an indexer for hybrid memory search.
func (e *MarkdownEngine) SetIndexer(indexer *Indexer) {
	e.indexer = indexer
}

// Name identifies the drainable memory subsystem.
func (e *MarkdownEngine) Name() string {
	return "memory-indexer"
}

// Drain stops the indexer if it is running.
func (e *MarkdownEngine) Drain(ctx context.Context) error {
	if e.indexer == nil {
		return nil
	}
	return e.indexer.Drain(ctx)
}

// SetChunkConfig overrides the default chunking configuration.
func (e *MarkdownEngine) SetChunkConfig(tokens, overlap int) {
	if tokens > 0 {
		e.chunkTokens = tokens
	}
	if overlap >= 0 {
		e.chunkOverlap = overlap
	}
}

// RootDir returns the configured root directory.
func (e *MarkdownEngine) RootDir() string {
	return e.rootDir
}

// EnsureSchema creates required directories if missing.
func (e *MarkdownEngine) EnsureSchema(_ context.Context) error {
	root, err := e.requireRoot()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	if err := migrateLegacyUsers(root); err != nil {
		return err
	}
	return nil
}

func (e *MarkdownEngine) userRoot() string {
	return ResolveUserRoot(e.rootDir, "")
}

func (e *MarkdownEngine) requireRoot() (string, error) {
	root := e.userRoot()
	if root == "" {
		return "", fmt.Errorf("memory root directory is required")
	}
	return root, nil
}

func (e *MarkdownEngine) resolvePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	root, err := e.requireRoot()
	if err != nil {
		return "", err
	}
	var abs string
	if filepath.IsAbs(path) {
		abs = filepath.Clean(path)
	} else {
		abs = filepath.Clean(filepath.Join(root, path))
	}
	if !strings.HasPrefix(abs, root+string(os.PathSeparator)) && abs != root {
		return "", fmt.Errorf("path outside memory root")
	}
	return abs, nil
}

func collectMemoryFilesForRoot(root string) ([]string, error) {
	var paths []string
	memoryPath := filepath.Join(root, memoryFileName)
	if _, err := os.Stat(memoryPath); err == nil {
		paths = append(paths, memoryPath)
	}
	dailyDir := filepath.Join(root, dailyDirName)
	entries, err := os.ReadDir(dailyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return paths, nil
		}
		return paths, err
	}
	for _, de := range entries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
			continue
		}
		paths = append(paths, filepath.Join(dailyDir, de.Name()))
	}
	return paths, nil
}

// ensureFileWithDefault creates a file with the given default content if it doesn't exist.
func ensureFileWithDefault(path, defaultContent string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return nil // already exists
	}
	defaultContent = strings.TrimSpace(defaultContent)
	if defaultContent == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	return os.WriteFile(path, []byte(defaultContent+"\n"), 0o644)
}

func ensureDailyHeader(path, dateStr string) error {
	info, err := os.Stat(path)
	if err == nil {
		if info.Size() > 0 {
			return nil
		}
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "# %s\n\n", dateStr)
	return err
}

func needsLeadingNewline(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() == 0 {
		return false
	}
	if _, err := f.Seek(-1, io.SeekEnd); err != nil {
		return false
	}
	buf := make([]byte, 1)
	if _, err := f.Read(buf); err != nil {
		return false
	}
	return buf[0] != '\n'
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lines := make([]string, 0, 64)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func nextChunkEnd(start int, counts []int, maxTokens int) (int, int) {
	if start < 0 || start >= len(counts) {
		return start, 0
	}
	if maxTokens <= 0 {
		maxTokens = chunkTokenSize
	}
	tokens := 0
	end := start
	for end < len(counts) {
		tokens += counts[end]
		end++
		if tokens >= maxTokens {
			break
		}
	}
	if end == start {
		end = start + 1
	}
	return end, tokens
}

func nextChunkStart(start, end int, counts []int, overlapTokens int) int {
	if end >= len(counts) {
		return len(counts)
	}
	if overlapTokens <= 0 {
		return end
	}
	overlap := 0
	nextStart := end
	for i := end - 1; i >= start; i-- {
		overlap += counts[i]
		if overlap >= overlapTokens {
			nextStart = i
			break
		}
	}
	if nextStart <= start {
		return end
	}
	return nextStart
}

func mergeTokens(lines [][]string) []string {
	var out []string
	for _, tokens := range lines {
		out = append(out, tokens...)
	}
	return out
}

func normalizeTerms(terms []string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, term := range terms {
		trimmed := utils.TrimLower(term)
		if trimmed == "" {
			continue
		}
		out[trimmed] = struct{}{}
	}
	return out
}

func buildSnippet(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= maxSnippetChars {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:maxSnippetChars]) + "..."
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
