package memory

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/ksuid"
	"gopkg.in/yaml.v3"
)

const (
	entriesDirName       = "entries"
	dailyDirName         = "daily"
	memoryFileName       = "MEMORY.md"
	dailySummaryType     = "daily_summary"
	longTermType         = "long_term"
	longTermKey          = "long_term"
	dailyKeyPrefix       = "daily-"
	defaultDailyLookback = 7
)

// FileStore implements Store by persisting each memory entry as a Markdown file
// with YAML frontmatter. Entries live under entries/, daily summaries under daily/,
// and long-term facts in MEMORY.md.
type FileStore struct {
	rootDir       string
	entriesDir    string
	dailyDir      string
	memoryFile    string
	dailyLookback int
	mu            sync.RWMutex
}

// NewFileStore creates a file-backed memory store rooted at dir.
func NewFileStore(dir string) *FileStore {
	return &FileStore{
		rootDir:       dir,
		entriesDir:    filepath.Join(dir, entriesDirName),
		dailyDir:      filepath.Join(dir, dailyDirName),
		memoryFile:    filepath.Join(dir, memoryFileName),
		dailyLookback: defaultDailyLookback,
	}
}

// EntriesDir returns the raw entry directory path.
func (s *FileStore) EntriesDir() string {
	return s.entriesDir
}

// DailyDir returns the daily summary directory path.
func (s *FileStore) DailyDir() string {
	return s.dailyDir
}

// MemoryFile returns the long-term memory file path.
func (s *FileStore) MemoryFile() string {
	return s.memoryFile
}

// frontmatter is the YAML structure written between --- delimiters.
type frontmatter struct {
	User    string            `yaml:"user"`
	Tags    []string          `yaml:"tags,omitempty"`
	Created string            `yaml:"created"`
	Slots   map[string]string `yaml:"slots,omitempty"`
}

type dailyFrontmatter struct {
	Date       string   `yaml:"date"`
	EntryCount int      `yaml:"entry_count"`
	Sources    []string `yaml:"sources,omitempty"`
}

type dailySummary struct {
	Date       time.Time
	EntryCount int
	Sources    []string
	Content    string
}

// EnsureSchema creates the memory directory structure if it does not exist.
func (s *FileStore) EnsureSchema(_ context.Context) error {
	if err := os.MkdirAll(s.rootDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(s.entriesDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(s.dailyDir, 0o755); err != nil {
		return err
	}
	return s.migrateLegacyEntries()
}

func (s *FileStore) migrateLegacyEntries() error {
	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read memory dir: %w", err)
	}

	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		if name == memoryFileName {
			continue
		}
		src := filepath.Join(s.rootDir, name)
		dst := filepath.Join(s.entriesDir, name)
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("migrate legacy entry: %w", err)
		}
	}
	return nil
}

// Insert writes a memory entry as a .md file and returns the entry with its key set.
func (s *FileStore) Insert(_ context.Context, entry Entry) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.Key == "" {
		entry.Key = ksuid.New().String()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	fm := frontmatter{
		User:    entry.UserID,
		Tags:    entry.Keywords,
		Created: entry.CreatedAt.Format(time.RFC3339),
		Slots:   entry.Slots,
	}
	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return entry, fmt.Errorf("marshal frontmatter: %w", err)
	}

	var buf strings.Builder
	buf.WriteString("---\n")
	buf.Write(fmBytes)
	buf.WriteString("---\n")
	buf.WriteString(entry.Content)
	buf.WriteByte('\n')

	if err := os.MkdirAll(s.entriesDir, 0o755); err != nil {
		return entry, fmt.Errorf("ensure entries dir: %w", err)
	}
	filename := filepath.Join(s.entriesDir, entry.Key+".md")
	if err := os.WriteFile(filename, []byte(buf.String()), 0o644); err != nil {
		return entry, fmt.Errorf("write memory file: %w", err)
	}
	return entry, nil
}

// Search reads layered memory sources, filters by user and keyword/term overlap, and
// returns up to query.Limit matching entries.
func (s *FileStore) Search(_ context.Context, query Query) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	termSet := make(map[string]bool, len(query.Terms)+len(query.Keywords))
	for _, t := range query.Terms {
		termSet[strings.ToLower(t)] = true
	}
	for _, kw := range query.Keywords {
		termSet[strings.ToLower(kw)] = true
	}
	hasTerms := len(termSet) > 0

	var results []Entry
	seen := make(map[string]struct{})

	appendFiltered := func(entries []Entry) bool {
		filtered := filterEntries(entries, query, termSet, hasTerms)
		for _, entry := range filtered {
			if entry.Key == "" {
				continue
			}
			if _, ok := seen[entry.Key]; ok {
				continue
			}
			seen[entry.Key] = struct{}{}
			results = append(results, entry)
			if query.Limit > 0 && len(results) >= query.Limit {
				return true
			}
		}
		return false
	}

	if longTerm, err := s.readLongTermEntry(query.UserID); err != nil {
		return nil, err
	} else if longTerm != nil {
		if appendFiltered([]Entry{*longTerm}) {
			return results, nil
		}
	}

	dailyEntries, err := s.readDailyEntries(query.UserID)
	if err != nil {
		return results, err
	}
	if appendFiltered(dailyEntries) {
		return results, nil
	}

	entries, err := s.readEntries()
	if err != nil {
		return results, err
	}
	_ = appendFiltered(entries)

	return results, nil
}

// Delete removes entries by key.
func (s *FileStore) Delete(_ context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}

		if trimmed == longTermKey {
			if err := os.Remove(s.memoryFile); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("delete memory file: %w", err)
			}
			continue
		}

		if strings.HasPrefix(trimmed, dailyKeyPrefix) {
			date := strings.TrimPrefix(trimmed, dailyKeyPrefix)
			if date != "" {
				filename := filepath.Join(s.dailyDir, date+".md")
				if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("delete daily file: %w", err)
				}
			}
			continue
		}

		filename := filepath.Join(s.entriesDir, trimmed+".md")
		if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete memory file: %w", err)
		}
		legacy := filepath.Join(s.rootDir, trimmed+".md")
		if err := os.Remove(legacy); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete legacy memory file: %w", err)
		}
	}
	return nil
}

// Prune removes expired entries based on the retention policy.
func (s *FileStore) Prune(_ context.Context, policy RetentionPolicy) ([]string, error) {
	if !policy.HasRules() {
		return nil, nil
	}

	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := s.readEntries()
	if err != nil {
		return nil, err
	}

	var deleted []string
	for _, entry := range entries {
		if !policy.IsExpired(entry, now) {
			continue
		}
		if entry.Key == "" {
			continue
		}
		filename := filepath.Join(s.entriesDir, entry.Key+".md")
		if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
			return deleted, fmt.Errorf("delete memory file: %w", err)
		}
		legacy := filepath.Join(s.rootDir, entry.Key+".md")
		if err := os.Remove(legacy); err != nil && !os.IsNotExist(err) {
			return deleted, fmt.Errorf("delete legacy memory file: %w", err)
		}
		deleted = append(deleted, entry.Key)
	}
	return deleted, nil
}

func filterEntries(entries []Entry, query Query, termSet map[string]bool, hasTerms bool) []Entry {
	if len(entries) == 0 {
		return nil
	}

	results := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if query.UserID != "" && entry.UserID != query.UserID {
			continue
		}
		if !matchSlots(entry.Slots, query.Slots) {
			continue
		}
		if hasTerms {
			if !matchesTerms(entry.Terms, termSet) &&
				!matchesKeywords(entry.Keywords, termSet) &&
				!containsAnyCaseInsensitive(entry.Content, query.Keywords) {
				continue
			}
		}
		results = append(results, entry)
	}
	return results
}

// matchesKeywords checks if any of the entry's stored keywords appear in the query term set.
func matchesKeywords(entryKeywords []string, queryTerms map[string]bool) bool {
	for _, kw := range entryKeywords {
		if queryTerms[strings.ToLower(kw)] {
			return true
		}
	}
	return false
}

// containsAnyCaseInsensitive checks if any keyword appears as a substring in
// content, ignoring case.
func containsAnyCaseInsensitive(content string, keywords []string) bool {
	if content == "" || len(keywords) == 0 {
		return false
	}
	lower := strings.ToLower(content)
	for _, kw := range keywords {
		trimmed := strings.TrimSpace(kw)
		if trimmed != "" && strings.Contains(lower, strings.ToLower(trimmed)) {
			return true
		}
	}
	return false
}

func (s *FileStore) readEntries() ([]Entry, error) {
	entries, err := s.readEntriesFromDir(s.entriesDir, nil)
	if err != nil {
		return nil, err
	}
	legacy, err := s.readEntriesFromDir(s.rootDir, map[string]bool{memoryFileName: true})
	if err != nil {
		return nil, err
	}
	entries = append(entries, legacy...)

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})
	return entries, nil
}

func (s *FileStore) readEntriesFromDir(dir string, skip map[string]bool) ([]Entry, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read memory dir: %w", err)
	}

	var entries []Entry
	for _, de := range dirEntries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
			continue
		}
		if skip != nil && skip[de.Name()] {
			continue
		}
		path := filepath.Join(dir, de.Name())
		entry, err := s.parseEntryFile(path)
		if err != nil {
			continue
		}
		entry.Key = strings.TrimSuffix(de.Name(), ".md")
		entry.Terms = collectTerms(entry.Content, entry.Keywords, entry.Slots)
		entries = append(entries, entry)
	}
	return entries, nil
}

func (s *FileStore) parseEntryFile(path string) (Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return Entry{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var (
		inFrontmatter bool
		fmLines       []string
		bodyLines     []string
		fmDone        bool
	)

	for scanner.Scan() {
		line := scanner.Text()
		if !fmDone && strings.TrimSpace(line) == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			inFrontmatter = false
			fmDone = true
			continue
		}
		if inFrontmatter {
			fmLines = append(fmLines, line)
		} else if fmDone {
			bodyLines = append(bodyLines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return Entry{}, err
	}

	var fm frontmatter
	if len(fmLines) > 0 {
		if err := yaml.Unmarshal([]byte(strings.Join(fmLines, "\n")), &fm); err != nil {
			return Entry{}, fmt.Errorf("parse frontmatter: %w", err)
		}
	}

	createdAt, _ := time.Parse(time.RFC3339, fm.Created)

	return Entry{
		UserID:    fm.User,
		Keywords:  fm.Tags,
		Slots:     fm.Slots,
		Content:   strings.TrimSpace(strings.Join(bodyLines, "\n")),
		CreatedAt: createdAt,
	}, nil
}

func (s *FileStore) readDailyEntries(userID string) ([]Entry, error) {
	summaries, err := s.readDailySummaries()
	if err != nil {
		return nil, err
	}
	if len(summaries) == 0 {
		return nil, nil
	}

	entries := make([]Entry, 0, len(summaries))
	for _, summary := range summaries {
		date := summary.Date.Format("2006-01-02")
		slots := map[string]string{
			"type": dailySummaryType,
			"date": date,
		}
		if summary.EntryCount > 0 {
			slots["entry_count"] = strconv.Itoa(summary.EntryCount)
		}
		entry := Entry{
			Key:       dailyKeyPrefix + date,
			UserID:    userID,
			Content:   summary.Content,
			Keywords:  summary.Sources,
			Slots:     slots,
			CreatedAt: summary.Date,
		}
		entry.Terms = collectTerms(entry.Content, entry.Keywords, entry.Slots)
		entries = append(entries, entry)
	}

	return entries, nil
}

func (s *FileStore) readDailySummaries() ([]dailySummary, error) {
	dirEntries, err := os.ReadDir(s.dailyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read daily dir: %w", err)
	}

	var summaries []dailySummary
	cutoff := time.Time{}
	if s.dailyLookback > 0 {
		now := time.Now().UTC()
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		cutoff = start.AddDate(0, 0, -(s.dailyLookback - 1))
	}

	for _, de := range dirEntries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
			continue
		}
		dateStr := strings.TrimSuffix(de.Name(), ".md")
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if !cutoff.IsZero() && date.Before(cutoff) {
			continue
		}
		path := filepath.Join(s.dailyDir, de.Name())
		summary, err := s.parseDailyFile(path, date)
		if err != nil {
			continue
		}
		summaries = append(summaries, summary)
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Date.After(summaries[j].Date)
	})
	return summaries, nil
}

func (s *FileStore) parseDailyFile(path string, fallbackDate time.Time) (dailySummary, error) {
	f, err := os.Open(path)
	if err != nil {
		return dailySummary{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var (
		inFrontmatter bool
		fmLines       []string
		bodyLines     []string
		fmDone        bool
	)

	for scanner.Scan() {
		line := scanner.Text()
		if !fmDone && strings.TrimSpace(line) == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			inFrontmatter = false
			fmDone = true
			continue
		}
		if inFrontmatter {
			fmLines = append(fmLines, line)
		} else if fmDone {
			bodyLines = append(bodyLines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return dailySummary{}, err
	}

	var fm dailyFrontmatter
	if len(fmLines) > 0 {
		if err := yaml.Unmarshal([]byte(strings.Join(fmLines, "\n")), &fm); err != nil {
			return dailySummary{}, fmt.Errorf("parse daily frontmatter: %w", err)
		}
	}

	date := fallbackDate
	if fm.Date != "" {
		if parsed, err := time.Parse("2006-01-02", fm.Date); err == nil {
			date = parsed
		}
	}

	return dailySummary{
		Date:       date,
		EntryCount: fm.EntryCount,
		Sources:    fm.Sources,
		Content:    strings.TrimSpace(strings.Join(bodyLines, "\n")),
	}, nil
}

func (s *FileStore) readLongTermEntry(userID string) (*Entry, error) {
	data, err := os.ReadFile(s.memoryFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read memory file: %w", err)
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil, nil
	}

	createdAt := parseUpdatedTimestamp(content)
	if createdAt.IsZero() {
		if info, err := os.Stat(s.memoryFile); err == nil {
			createdAt = info.ModTime()
		}
	}

	entry := Entry{
		Key:       longTermKey,
		UserID:    userID,
		Content:   content,
		Slots:     map[string]string{"type": longTermType},
		CreatedAt: createdAt,
	}
	entry.Terms = collectTerms(entry.Content, entry.Keywords, entry.Slots)
	return &entry, nil
}

func parseUpdatedTimestamp(content string) time.Time {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Updated:") {
			ts := strings.TrimSpace(strings.TrimPrefix(trimmed, "Updated:"))
			if ts == "" {
				continue
			}
			if parsed, err := time.Parse("2006-01-02 15:04", ts); err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}
