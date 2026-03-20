package tape

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// EntryKind classifies a tape entry.
type EntryKind string

const (
	KindMessage    EntryKind = "message"
	KindSystem     EntryKind = "system"
	KindAnchor     EntryKind = "anchor"
	KindToolCall   EntryKind = "tool_call"
	KindToolResult EntryKind = "tool_result"
	KindError      EntryKind = "error"
	KindEvent      EntryKind = "event"
	KindThinking   EntryKind = "thinking"
	KindCheckpoint EntryKind = "checkpoint"
)

// EntryMeta contains tracking metadata for a tape entry.
type EntryMeta struct {
	SessionID     string `json:"session_id,omitempty"`
	RunID         string `json:"run_id,omitempty"`
	ParentRunID   string `json:"parent_run_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Seq           int64  `json:"seq"`
	Model         string `json:"model,omitempty"`
	TokensUsed    int    `json:"tokens_used,omitempty"`
}

// TapeEntry is a single immutable record in the tape audit trail.
// Each entry represents a single event in the agent lifecycle.
type TapeEntry struct {
	ID      string         `json:"id"`
	Kind    EntryKind      `json:"kind"`
	Payload map[string]any `json:"payload,omitempty"`
	Meta    EntryMeta      `json:"meta"`
	Date    time.Time      `json:"date"`
}

// generateID returns a random 16-byte hex-encoded identifier.
func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// newEntry creates a TapeEntry with the given kind, payload, and meta.
func newEntry(kind EntryKind, payload map[string]any, meta EntryMeta) TapeEntry {
	return TapeEntry{
		ID:      generateID(),
		Kind:    kind,
		Payload: payload,
		Meta:    meta,
		Date:    time.Now(),
	}
}

// NewMessage creates a message tape entry.
func NewMessage(role, content string, meta EntryMeta) TapeEntry {
	return newEntry(KindMessage, map[string]any{
		"role":    role,
		"content": content,
	}, meta)
}

// NewSystem creates a system tape entry.
func NewSystem(content string, meta EntryMeta) TapeEntry {
	return newEntry(KindSystem, map[string]any{
		"content": content,
	}, meta)
}

// NewAnchor creates an anchor tape entry used as a query boundary marker.
func NewAnchor(label string, meta EntryMeta) TapeEntry {
	return newEntry(KindAnchor, map[string]any{
		"label": label,
	}, meta)
}

// NewToolCall creates a tool_call tape entry.
func NewToolCall(name string, args map[string]any, meta EntryMeta) TapeEntry {
	return newEntry(KindToolCall, map[string]any{
		"name": name,
		"args": args,
	}, meta)
}

// NewToolResult creates a tool_result tape entry.
func NewToolResult(name string, result any, isError bool, meta EntryMeta) TapeEntry {
	return newEntry(KindToolResult, map[string]any{
		"name":     name,
		"result":   result,
		"is_error": isError,
	}, meta)
}

// NewError creates an error tape entry.
func NewError(err string, code string, meta EntryMeta) TapeEntry {
	return newEntry(KindError, map[string]any{
		"error": err,
		"code":  code,
	}, meta)
}

// NewEvent creates a generic event tape entry.
func NewEvent(eventType string, data map[string]any, meta EntryMeta) TapeEntry {
	return newEntry(KindEvent, map[string]any{
		"type": eventType,
		"data": data,
	}, meta)
}

// NewThinking creates a thinking tape entry for chain-of-thought traces.
func NewThinking(content string, meta EntryMeta) TapeEntry {
	return newEntry(KindThinking, map[string]any{
		"content": content,
	}, meta)
}

// NewCheckpoint creates a checkpoint tape entry for state snapshots.
func NewCheckpoint(label string, state map[string]any, meta EntryMeta) TapeEntry {
	return newEntry(KindCheckpoint, map[string]any{
		"label": label,
		"state": state,
	}, meta)
}
