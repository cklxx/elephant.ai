package meta

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/analytics/journal"
)

// ReplayConfig controls the meta-steward batch pipeline.
// It replays journal entries to derive persona-aware meta memories and
// recommendations, enforcing a minimal schema before persisting updates.
type ReplayConfig struct {
	InputDir       string
	OutputPath     string
	PersonaID      string
	PersonaVersion string
}

// Steward replays turn journals into stewarded meta profiles.
type Steward struct{}

// NewSteward constructs a default steward implementation.
func NewSteward() *Steward { return &Steward{} }

// Run loads every session journal in InputDir, derives a merged MetaContext and
// writes it to OutputPath (YAML). The generated context can be dropped under
// configs/context/meta to be picked up by the context manager.
func (s *Steward) Run(ctx context.Context, cfg ReplayConfig) (ports.MetaContext, error) {
	if strings.TrimSpace(cfg.InputDir) == "" {
		return ports.MetaContext{}, fmt.Errorf("input dir required")
	}
	personaID := strings.TrimSpace(cfg.PersonaID)
	if personaID == "" {
		return ports.MetaContext{}, fmt.Errorf("persona id required for stewarding")
	}
	reader, err := journal.NewFileReader(cfg.InputDir)
	if err != nil {
		return ports.MetaContext{}, err
	}

	sessions, err := discoverSessions(cfg.InputDir)
	if err != nil {
		return ports.MetaContext{}, err
	}
	if len(sessions) == 0 {
		return ports.MetaContext{}, fmt.Errorf("no sessions to replay in %s", cfg.InputDir)
	}

	recommendations := map[string]struct{}{}
	memories := []ports.MemoryFragment{}

	for _, session := range sessions {
		entries, err := reader.ReadAll(ctx, session)
		if err != nil {
			return ports.MetaContext{}, fmt.Errorf("read session %s: %w", session, err)
		}
		for _, entry := range entries {
			if strings.TrimSpace(entry.Summary) != "" {
				memories = append(memories, ports.MemoryFragment{
					Key:       fmt.Sprintf("%s:%d", entry.SessionID, entry.TurnID),
					Content:   entry.Summary,
					CreatedAt: entry.Timestamp,
					Source:    "journal",
				})
			}
			for _, fb := range entry.Feedback {
				text := strings.TrimSpace(fb.Note)
				if text == "" {
					continue
				}
				recommendations[text] = struct{}{}
			}
		}
	}

	// Ensure deterministic ordering for repeatability and hashing.
	sort.SliceStable(memories, func(i, j int) bool { return memories[i].Key < memories[j].Key })

	recs := make([]string, 0, len(recommendations))
	for rec := range recommendations {
		recs = append(recs, rec)
	}
	sort.Strings(recs)

	meta := ports.MetaContext{
		Memories:        memories,
		Recommendations: recs,
		PersonaVersion:  cfg.PersonaVersion,
	}

	if cfg.OutputPath != "" {
		if err := persistMetaContext(cfg.OutputPath, personaID, meta); err != nil {
			return ports.MetaContext{}, err
		}
	}
	return meta, nil
}

func discoverSessions(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read journal dir: %w", err)
	}
	sessions := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		sessions = append(sessions, strings.TrimSuffix(name, ".jsonl"))
	}
	sort.Strings(sessions)
	return sessions, nil
}

func persistMetaContext(path, personaID string, meta ports.MetaContext) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create meta file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	wrapped := map[string]any{personaID: meta}
	if err := encoder.Encode(wrapped); err != nil {
		return fmt.Errorf("encode meta: %w", err)
	}
	return nil
}

// StreamRecords allows streaming journal entries without loading all in memory.
func StreamRecords(ctx context.Context, dir, sessionID string, fn func(journal.TurnJournalEntry) error) error {
	reader, err := journal.NewFileReader(dir)
	if err != nil {
		return err
	}
	return reader.Stream(ctx, sessionID, fn)
}

// ValidateOutput ensures the generated meta context meets a minimal schema.
func ValidateOutput(meta ports.MetaContext) error {
	for i, mem := range meta.Memories {
		if strings.TrimSpace(mem.Content) == "" {
			return fmt.Errorf("memory %d missing content", i)
		}
		if mem.CreatedAt.IsZero() {
			mem.CreatedAt = time.Now()
		}
	}
	return nil
}

// LoadMetaContext opens a persisted meta context file for inspection.
func LoadMetaContext(path string) (map[string]ports.MetaContext, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()
	dec := json.NewDecoder(bufio.NewReader(f))
	out := map[string]ports.MetaContext{}
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}
