package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	agentdomain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/filestore"
	jsonx "alex/internal/shared/json"
)

// FileEventHistoryStore is a file-backed implementation of EventHistoryStore.
// Events are stored as JSONL files — one file per session under {dir}/events/{session_id}.jsonl.
// Each line is a self-describing JSON record that can reconstruct an AgentEvent.
type FileEventHistoryStore struct {
	dir string
	mu  sync.Mutex // serialises writes to the same session
}

// NewFileEventHistoryStore creates a file-backed event history store.
// dir is the root directory; session files will be at {dir}/events/{session_id}.jsonl.
func NewFileEventHistoryStore(dir string) *FileEventHistoryStore {
	return &FileEventHistoryStore{dir: dir}
}

// EnsureSchema creates the events directory if it does not exist.
func (s *FileEventHistoryStore) EnsureSchema(_ context.Context) error {
	return filestore.EnsureDir(s.eventsDir())
}

// Append serialises the event and appends it as a single JSONL line to the session file.
func (s *FileEventHistoryStore) Append(ctx context.Context, event agent.AgentEvent) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if event == nil {
		return nil
	}

	rec := recordFromAgentEvent(event)
	line, err := jsonx.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal event record: %w", err)
	}
	line = append(line, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.sessionPath(rec.SessionID)
	if err := filestore.EnsureParentDir(path); err != nil {
		return fmt.Errorf("ensure events dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open event file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("write event line: %w", err)
	}
	return nil
}

// Stream reads the session's JSONL file line-by-line, filters by event types,
// and calls fn for each matching event. Events are replayed in append order.
func (s *FileEventHistoryStore) Stream(ctx context.Context, filter EventHistoryFilter, fn func(agent.AgentEvent) error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	path := s.sessionPath(filter.SessionID)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no events for this session
		}
		return fmt.Errorf("open event file: %w", err)
	}
	defer f.Close()

	typeSet := make(map[string]struct{}, len(filter.EventTypes))
	for _, t := range filter.EventTypes {
		typeSet[t] = struct{}{}
	}
	filterByType := len(typeSet) > 0

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 2*1024*1024) // 2 MB max line
	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec eventFileRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue // skip corrupt lines
		}

		if filterByType {
			if _, ok := typeSet[rec.EventType]; !ok {
				continue
			}
		}

		event := agentEventFromRecord(rec)
		if err := fn(event); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// DeleteSession removes the session's event file.
func (s *FileEventHistoryStore) DeleteSession(_ context.Context, sessionID string) error {
	path := s.sessionPath(sessionID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete session events: %w", err)
	}
	return nil
}

// HasSessionEvents checks whether a session's event file exists and is non-empty.
func (s *FileEventHistoryStore) HasSessionEvents(_ context.Context, sessionID string) (bool, error) {
	info, err := os.Stat(s.sessionPath(sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.Size() > 0, nil
}

// --- path helpers ---

func (s *FileEventHistoryStore) eventsDir() string {
	return filepath.Join(s.dir, "events")
}

func (s *FileEventHistoryStore) sessionPath(sessionID string) string {
	safe := sanitiseSessionID(sessionID)
	return filepath.Join(s.eventsDir(), safe+".jsonl")
}

func sanitiseSessionID(id string) string {
	id = strings.ReplaceAll(id, "/", "_")
	id = strings.ReplaceAll(id, "..", "_")
	id = strings.ReplaceAll(id, "\\", "_")
	return id
}

// --- serialisation ---

// eventFileRecord is the on-disk JSON line format for a single event.
type eventFileRecord struct {
	// Common metadata (extracted via AgentEvent interface methods).
	RecordType  string    `json:"record_type"` // "event" or "envelope"
	EventType   string    `json:"event_type"`
	SessionID   string    `json:"session_id"`
	RunID       string    `json:"run_id"`
	ParentRunID string    `json:"parent_run_id,omitempty"`
	AgentLevel  string    `json:"agent_level"`
	Timestamp   time.Time `json:"timestamp"`
	Seq         uint64    `json:"seq,omitempty"`

	// For Event (RecordType="event"): Kind + Data JSON.
	Kind string          `json:"kind,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`

	// For WorkflowEventEnvelope (RecordType="envelope").
	Version        int             `json:"version,omitempty"`
	WorkflowID     string          `json:"workflow_id,omitempty"`
	NodeID         string          `json:"node_id,omitempty"`
	NodeKind       string          `json:"node_kind,omitempty"`
	IsSubtask      bool            `json:"is_subtask,omitempty"`
	SubtaskIndex   int             `json:"subtask_index,omitempty"`
	TotalSubtasks  int             `json:"total_subtasks,omitempty"`
	SubtaskPreview string          `json:"subtask_preview,omitempty"`
	MaxParallel    int             `json:"max_parallel,omitempty"`
	Payload        json.RawMessage `json:"payload,omitempty"`
}

func recordFromAgentEvent(event agent.AgentEvent) eventFileRecord {
	rec := eventFileRecord{
		EventType:   event.EventType(),
		SessionID:   event.GetSessionID(),
		RunID:       event.GetRunID(),
		ParentRunID: event.GetParentRunID(),
		AgentLevel:  string(event.GetAgentLevel()),
		Timestamp:   event.Timestamp(),
		Seq:         event.GetSeq(),
	}

	switch e := event.(type) {
	case *agentdomain.WorkflowEventEnvelope:
		rec.RecordType = "envelope"
		rec.Version = e.Version
		rec.WorkflowID = e.WorkflowID
		rec.NodeID = e.NodeID
		rec.NodeKind = e.NodeKind
		rec.IsSubtask = e.IsSubtask
		rec.SubtaskIndex = e.SubtaskIndex
		rec.TotalSubtasks = e.TotalSubtasks
		rec.SubtaskPreview = e.SubtaskPreview
		rec.MaxParallel = e.MaxParallel
		if e.Payload != nil {
			if data, err := jsonx.Marshal(e.Payload); err == nil {
				rec.Payload = data
			}
		}
	case *agentdomain.Event:
		rec.RecordType = "event"
		rec.Kind = e.Kind
		if data, err := jsonx.Marshal(e.Data); err == nil {
			rec.Data = data
		}
	default:
		rec.RecordType = "event"
		rec.Kind = event.EventType()
	}

	return rec
}

func agentEventFromRecord(rec eventFileRecord) agent.AgentEvent {
	base := agentdomain.NewBaseEventFull(
		agent.AgentLevel(rec.AgentLevel),
		rec.SessionID,
		rec.RunID,
		rec.ParentRunID,
		"", "", // correlationID, causationID — not persisted
		rec.Seq,
		rec.Timestamp,
	)

	switch rec.RecordType {
	case "envelope":
		env := &agentdomain.WorkflowEventEnvelope{
			BaseEvent:      base,
			Version:        rec.Version,
			Event:          rec.EventType,
			WorkflowID:     rec.WorkflowID,
			RunID:          rec.RunID,
			NodeID:         rec.NodeID,
			NodeKind:       rec.NodeKind,
			IsSubtask:      rec.IsSubtask,
			SubtaskIndex:   rec.SubtaskIndex,
			TotalSubtasks:  rec.TotalSubtasks,
			SubtaskPreview: rec.SubtaskPreview,
			MaxParallel:    rec.MaxParallel,
		}
		if len(rec.Payload) > 0 {
			var payload map[string]any
			if err := json.Unmarshal(rec.Payload, &payload); err == nil {
				env.Payload = payload
			}
		}
		return env

	default: // "event" or unknown
		evt := &agentdomain.Event{
			BaseEvent: base,
			Kind:      rec.Kind,
		}
		if len(rec.Data) > 0 {
			var data agentdomain.EventData
			if err := json.Unmarshal(rec.Data, &data); err == nil {
				evt.Data = data
			}
		}
		return evt
	}
}

// Compile-time interface check.
var _ EventHistoryStore = (*FileEventHistoryStore)(nil)
