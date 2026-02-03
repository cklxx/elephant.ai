package memory

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	dailyDirName          = "memory"
	memoryFileName        = "MEMORY.md"
	userDirName           = "users"
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
	if e == nil {
		return
	}
	e.indexer = indexer
}

// Name identifies the drainable memory subsystem.
func (e *MarkdownEngine) Name() string {
	return "memory-indexer"
}

// Drain stops the indexer if it is running.
func (e *MarkdownEngine) Drain(ctx context.Context) error {
	if e == nil || e.indexer == nil {
		return nil
	}
	return e.indexer.Drain(ctx)
}

// SetChunkConfig overrides the default chunking configuration.
func (e *MarkdownEngine) SetChunkConfig(tokens, overlap int) {
	if e == nil {
		return
	}
	if tokens > 0 {
		e.chunkTokens = tokens
	}
	if overlap >= 0 {
		e.chunkOverlap = overlap
	}
}

// RootDir returns the configured root directory.
func (e *MarkdownEngine) RootDir() string {
	if e == nil {
		return ""
	}
	return e.rootDir
}

// EnsureSchema creates required directories if missing.
func (e *MarkdownEngine) EnsureSchema(_ context.Context) error {
	if e == nil {
		return fmt.Errorf("memory engine not initialized")
	}
	root := strings.TrimSpace(e.rootDir)
	if root == "" {
		return fmt.Errorf("memory root directory is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	return nil
}

// AppendDaily appends a record to the daily memory log.
func (e *MarkdownEngine) AppendDaily(_ context.Context, userID string, entry DailyEntry) (string, error) {
	if e == nil {
		return "", fmt.Errorf("memory engine not initialized")
	}
	content := strings.TrimSpace(entry.Content)
	if content == "" {
		return "", fmt.Errorf("content is required")
	}
	createdAt := entry.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	root := e.userRoot(userID)
	if root == "" {
		return "", fmt.Errorf("memory root directory is required")
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

	if _, err := f.WriteString(block.String()); err != nil {
		return "", err
	}

	return path, nil
}

// Search scans MEMORY.md + daily logs for the query and returns ranked hits.
func (e *MarkdownEngine) Search(ctx context.Context, userID, query string, maxResults int, minScore float64) ([]SearchHit, error) {
	if e == nil {
		return nil, fmt.Errorf("memory engine not initialized")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if maxResults <= 0 {
		maxResults = defaultSearchMax
	}
	if minScore <= 0 {
		minScore = defaultSearchMinScore
	}

	root := e.userRoot(userID)
	if root == "" {
		return nil, fmt.Errorf("memory root directory is required")
	}

	if e.indexer != nil {
		results, err := e.indexer.Search(ctx, userID, query, maxResults, minScore)
		if err == nil {
			return results, nil
		}
	}
	paths, err := e.collectMemoryFiles(root)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil
	}

	queryTerms := normalizeTerms(tokenize(query))
	queryLower := strings.ToLower(query)
	if len(queryTerms) == 0 && queryLower == "" {
		return nil, nil
	}

	var hits []SearchHit
	for _, file := range paths {
		h, err := searchFile(file, root, queryTerms, queryLower, minScore, e.chunkTokens, e.chunkOverlap)
		if err != nil {
			continue
		}
		hits = append(hits, h...)
	}

	if len(hits) == 0 {
		return nil, nil
	}

	return selectTopHits(hits, maxResults), nil
}

// GetLines returns a slice of lines from the given memory path.
func (e *MarkdownEngine) GetLines(_ context.Context, userID, path string, fromLine, lineCount int) (string, error) {
	if e == nil {
		return "", fmt.Errorf("memory engine not initialized")
	}
	if strings.TrimSpace(e.rootDir) == "" {
		return "", fmt.Errorf("memory root directory is required")
	}
	absPath, err := e.resolvePath(userID, path)
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
func (e *MarkdownEngine) LoadDaily(_ context.Context, userID string, day time.Time) (string, error) {
	if e == nil {
		return "", fmt.Errorf("memory engine not initialized")
	}
	if strings.TrimSpace(e.rootDir) == "" {
		return "", fmt.Errorf("memory root directory is required")
	}
	if day.IsZero() {
		day = time.Now()
	}
	dateStr := day.Format("2006-01-02")
	path := filepath.Join(e.userRoot(userID), dailyDirName, dateStr+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// LoadLongTerm reads MEMORY.md for the user.
func (e *MarkdownEngine) LoadLongTerm(_ context.Context, userID string) (string, error) {
	if e == nil {
		return "", fmt.Errorf("memory engine not initialized")
	}
	if strings.TrimSpace(e.rootDir) == "" {
		return "", fmt.Errorf("memory root directory is required")
	}
	path := filepath.Join(e.userRoot(userID), memoryFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (e *MarkdownEngine) userRoot(userID string) string {
	return ResolveUserRoot(e.rootDir, userID)
}

func sanitizeSegment(input string) string {
	if input == "" {
		return "user"
	}
	var b strings.Builder
	b.Grow(len(input))
	for _, r := range input {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == '@' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	out := strings.Trim(b.String(), "._-")
	if out == "" {
		return "user"
	}
	return out
}

func (e *MarkdownEngine) resolvePath(userID, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	root := e.userRoot(userID)
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

func (e *MarkdownEngine) collectMemoryFiles(root string) ([]string, error) {
	return collectMemoryFilesForRoot(root)
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

func searchFile(path, root string, queryTerms map[string]struct{}, queryLower string, minScore float64, chunkTokens, chunkOverlap int) ([]SearchHit, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, nil
	}

	lineTokens := make([][]string, len(lines))
	lineCounts := make([]int, len(lines))
	for i, line := range lines {
		tokens := tokenize(line)
		lineTokens[i] = tokens
		lineCounts[i] = len(tokens)
	}

	var hits []SearchHit
	if chunkTokens <= 0 {
		chunkTokens = chunkTokenSize
	}
	if chunkOverlap < 0 {
		chunkOverlap = 0
	}
	start := 0
	for start < len(lines) {
		end, _ := nextChunkEnd(start, lineCounts, chunkTokens)
		if end <= start {
			break
		}
		chunkText := strings.Join(lines[start:end], "\n")
		chunkTokens := mergeTokens(lineTokens[start:end])
		score := scoreChunk(queryTerms, queryLower, chunkTokens, chunkText)
		if score >= minScore {
			relPath := path
			if rel, err := filepath.Rel(root, path); err == nil {
				relPath = rel
			}
			snippet := buildSnippet(chunkText)
			source := "memory"
			if filepath.Base(path) == memoryFileName {
				source = "long_term"
			}
			hits = append(hits, SearchHit{
				Path:      relPath,
				StartLine: start + 1,
				EndLine:   end,
				Score:     score,
				Snippet:   snippet,
				Source:    source,
			})
		}
		start = nextChunkStart(start, end, lineCounts, chunkOverlap)
		if start <= 0 {
			break
		}
	}

	return hits, nil
}

func selectTopHits(hits []SearchHit, limit int) []SearchHit {
	if limit <= 0 || len(hits) <= limit {
		return hits
	}
	// Partial selection: keep best `limit` hits by score, then stable sort them.
	best := make([]SearchHit, 0, limit)
	for _, hit := range hits {
		if len(best) < limit {
			best = append(best, hit)
			continue
		}
		minIdx := 0
		for i := 1; i < len(best); i++ {
			if best[i].Score < best[minIdx].Score {
				minIdx = i
			}
		}
		if hit.Score > best[minIdx].Score {
			best[minIdx] = hit
		}
	}
	for i := 1; i < len(best); i++ {
		j := i
		for j > 0 {
			swap := best[j].Score > best[j-1].Score
			if !swap && best[j].Score == best[j-1].Score {
				swap = best[j].Path < best[j-1].Path
			}
			if !swap {
				break
			}
			best[j], best[j-1] = best[j-1], best[j]
			j--
		}
	}
	return best
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
		trimmed := strings.ToLower(strings.TrimSpace(term))
		if trimmed == "" {
			continue
		}
		out[trimmed] = struct{}{}
	}
	return out
}

func scoreChunk(queryTerms map[string]struct{}, queryLower string, chunkTerms []string, chunkText string) float64 {
	if len(queryTerms) == 0 {
		return 0
	}
	chunkSet := make(map[string]struct{}, len(chunkTerms))
	for _, term := range chunkTerms {
		trimmed := strings.ToLower(strings.TrimSpace(term))
		if trimmed == "" {
			continue
		}
		chunkSet[trimmed] = struct{}{}
	}
	matches := 0
	for term := range queryTerms {
		if _, ok := chunkSet[term]; ok {
			matches++
		}
	}
	termScore := float64(matches) / float64(len(queryTerms))
	exactScore := 0.0
	if queryLower != "" {
		textLower := strings.ToLower(chunkText)
		if len(queryLower) >= 3 && strings.Contains(textLower, queryLower) {
			exactScore = 1.0
		}
	}
	return (0.7 * termScore) + (0.3 * exactScore)
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
