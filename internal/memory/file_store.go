package memory

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/ksuid"
	"gopkg.in/yaml.v3"
)

// FileStore implements Store by persisting each memory entry as a Markdown file
// with YAML frontmatter. Files are stored in a flat directory structure.
type FileStore struct {
	dir string
	mu  sync.RWMutex
}

// NewFileStore creates a file-backed memory store rooted at dir.
func NewFileStore(dir string) *FileStore {
	return &FileStore{dir: dir}
}

// frontmatter is the YAML structure written between --- delimiters.
type frontmatter struct {
	User     string   `yaml:"user"`
	Tags     []string `yaml:"tags,omitempty"`
	Created  string   `yaml:"created"`
	Slots    map[string]string `yaml:"slots,omitempty"`
}

// EnsureSchema creates the memory directory if it does not exist.
func (s *FileStore) EnsureSchema(_ context.Context) error {
	return os.MkdirAll(s.dir, 0o755)
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

	filename := filepath.Join(s.dir, entry.Key+".md")
	if err := os.WriteFile(filename, []byte(buf.String()), 0o644); err != nil {
		return entry, fmt.Errorf("write memory file: %w", err)
	}
	return entry, nil
}

// Search reads all .md files, filters by user and keyword/term overlap, and
// returns up to query.Limit matching entries sorted newest-first.
func (s *FileStore) Search(_ context.Context, query Query) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := s.readAll()
	if err != nil {
		return nil, err
	}

	// Build term set from both explicit terms and keywords for robust matching.
	termSet := make(map[string]bool, len(query.Terms)+len(query.Keywords))
	for _, t := range query.Terms {
		termSet[strings.ToLower(t)] = true
	}
	for _, kw := range query.Keywords {
		termSet[strings.ToLower(kw)] = true
	}

	var results []Entry
	for _, entry := range entries {
		if query.UserID != "" && entry.UserID != query.UserID {
			continue
		}
		if !matchSlots(entry.Slots, query.Slots) {
			continue
		}
		if len(termSet) == 0 {
			continue
		}
		if matchesTerms(entry.Terms, termSet) ||
			matchesKeywords(entry.Keywords, termSet) ||
			containsAnyCaseInsensitive(entry.Content, query.Keywords) {
			results = append(results, entry)
		}
		if query.Limit > 0 && len(results) >= query.Limit {
			break
		}
	}
	return results, nil
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

// readAll parses every .md file in the directory and returns entries sorted
// newest-first by CreatedAt.
func (s *FileStore) readAll() ([]Entry, error) {
	dirEntries, err := os.ReadDir(s.dir)
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
		entry, err := s.parseFile(filepath.Join(s.dir, de.Name()))
		if err != nil {
			continue // skip malformed files
		}
		entry.Key = strings.TrimSuffix(de.Name(), ".md")
		entry.Terms = collectTerms(entry.Content, entry.Keywords, entry.Slots)
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})
	return entries, nil
}

// parseFile reads a single .md file and returns an Entry.
func (s *FileStore) parseFile(path string) (Entry, error) {
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
