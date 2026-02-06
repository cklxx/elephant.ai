package journal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
)

// Writer persists structured turn journal entries so downstream analytics and
// replay systems can inspect what changed during every reasoning loop.
type Writer interface {
	Write(ctx context.Context, entry TurnJournalEntry) error
}

// WriterFunc allows ordinary functions to satisfy the Writer interface.
type WriterFunc func(ctx context.Context, entry TurnJournalEntry) error

// Write invokes the wrapped function.
func (f WriterFunc) Write(ctx context.Context, entry TurnJournalEntry) error {
	if f == nil {
		return nil
	}
	return f(ctx, entry)
}

// NopWriter returns a Writer that drops all entries. It is used when journal
// persistence has not been configured by the caller but the context manager
// still wants to call into a Writer implementation unconditionally.
func NopWriter() Writer {
	return WriterFunc(func(context.Context, TurnJournalEntry) error { return nil })
}

// TurnJournalEntry captures the structured state that should be emitted for
// each turn after the LLM has responded or tools have completed. It mirrors the
// schema described in docs/design/agent_context_framework.md so Meta-context
// jobs can process compact JSON lines instead of replaying entire sessions.
type TurnJournalEntry struct {
	SessionID     string                     `json:"session_id"`
	TurnID        int                        `json:"turn_id"`
	LLMTurnSeq    int                        `json:"llm_turn_seq"`
	Timestamp     time.Time                  `json:"timestamp"`
	Summary       string                     `json:"summary"`
	Plans         []agent.PlanNode           `json:"plans"`
	Beliefs       []agent.Belief             `json:"beliefs"`
	World         map[string]any             `json:"world_state"`
	Diff          map[string]any             `json:"diff"`
	Messages      []ports.Message            `json:"messages"`
	Feedback      []agent.FeedbackSignal     `json:"feedback"`
	KnowledgeRefs []agent.KnowledgeReference `json:"knowledge_refs"`
}

// FileWriter appends JSONL entries per session so that CLI operators can pull
// turn journals without querying a database. Files are named <session>.jsonl.
type FileWriter struct {
	dir string
	mu  sync.Mutex
}

// NewFileWriter creates a writer that appends to JSONL files stored within dir.
func NewFileWriter(dir string) (*FileWriter, error) {
	if dir == "" {
		return nil, fmt.Errorf("journal directory required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create journal dir: %w", err)
	}
	return &FileWriter{dir: dir}, nil
}

// Write appends the entry to the session-specific JSONL file.
func (w *FileWriter) Write(_ context.Context, entry TurnJournalEntry) error {
	if w == nil {
		return fmt.Errorf("nil file writer")
	}
	if entry.SessionID == "" {
		return fmt.Errorf("session id required")
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal journal entry: %w", err)
	}

	path := filepath.Join(w.dir, fmt.Sprintf("%s.jsonl", entry.SessionID))
	w.mu.Lock()
	defer w.mu.Unlock()

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open journal file: %w", err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "journal: close %s: %v\n", path, cerr)
		}
	}()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("append journal entry: %w", err)
	}
	return nil
}
