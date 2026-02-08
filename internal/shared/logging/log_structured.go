package logging

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// ParsedTextEntry represents a single parsed text log line.
type ParsedTextEntry struct {
	Raw        string `json:"raw"`
	Timestamp  string `json:"timestamp"`
	Level      string `json:"level"`
	Category   string `json:"category"`
	Component  string `json:"component"`
	LogID      string `json:"log_id,omitempty"`
	SourceFile string `json:"source_file,omitempty"`
	SourceLine int    `json:"source_line,omitempty"`
	Message    string `json:"message"`
}

// ParsedRequestEntry represents a single parsed LLM request/response log.
type ParsedRequestEntry struct {
	Raw       string          `json:"raw"`
	Timestamp string          `json:"timestamp"`
	RequestID string          `json:"request_id"`
	LogID     string          `json:"log_id,omitempty"`
	EntryType string          `json:"entry_type"`
	BodyBytes int             `json:"body_bytes"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// StructuredLogSnippet holds parsed text log entries.
type StructuredLogSnippet struct {
	Path      string            `json:"path,omitempty"`
	Entries   []ParsedTextEntry `json:"entries,omitempty"`
	Truncated bool              `json:"truncated,omitempty"`
	Error     string            `json:"error,omitempty"`
}

// StructuredRequestSnippet holds parsed LLM request/response entries.
type StructuredRequestSnippet struct {
	Path      string               `json:"path,omitempty"`
	Entries   []ParsedRequestEntry `json:"entries,omitempty"`
	Truncated bool                 `json:"truncated,omitempty"`
	Error     string               `json:"error,omitempty"`
}

// StructuredLogBundle aggregates all log types with parsed entries.
type StructuredLogBundle struct {
	LogID    string                   `json:"log_id"`
	Service  StructuredLogSnippet     `json:"service"`
	LLM      StructuredLogSnippet     `json:"llm"`
	Latency  StructuredLogSnippet     `json:"latency"`
	Requests StructuredRequestSnippet `json:"requests"`
}

// textLogLineRegexp matches the log format produced by logger.go log() method.
//
// Format with log_id:
//
//	2026-02-08 01:11:57 [INFO] [SERVICE] [Main] [log_id=xxx] lark.go:196 - Message
//
// Format without log_id:
//
//	2026-02-08 01:11:57 [INFO] [SERVICE] [Main] lark.go:196 - Message
//
// Capture groups:
//  1. timestamp  "2026-02-08 01:11:57"
//  2. level      "INFO"
//  3. category   "SERVICE"
//  4. component  "Main"
//  5. log_id     "xxx" (optional, empty if absent)
//  6. source     "lark.go"
//  7. line       "196"
//  8. message    "Message"
var textLogLineRegexp = regexp.MustCompile(
	`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}) \[(\w+)\] \[(\w+)\] \[([^\]]+)\](?: \[log_id=([^\]]+)\])? (\S+):(\d+) - (.*)$`,
)

// parseTextLogLine parses a single text log line into a ParsedTextEntry.
// If the line doesn't match the expected format, it falls back to a raw-only entry.
func parseTextLogLine(line string) ParsedTextEntry {
	m := textLogLineRegexp.FindStringSubmatch(line)
	if m == nil {
		return ParsedTextEntry{
			Raw:     line,
			Message: line,
		}
	}

	sourceLine, _ := strconv.Atoi(m[7])
	return ParsedTextEntry{
		Raw:        line,
		Timestamp:  m[1],
		Level:      m[2],
		Category:   m[3],
		Component:  m[4],
		LogID:      m[5],
		SourceFile: m[6],
		SourceLine: sourceLine,
		Message:    m[8],
	}
}

// parseRequestLogJSON parses a JSONL request/response log line.
// Returns the parsed entry and true on success, or zero value and false on failure.
func parseRequestLogJSON(line string) (ParsedRequestEntry, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return ParsedRequestEntry{}, false
	}

	var raw struct {
		Timestamp string          `json:"timestamp"`
		RequestID string          `json:"request_id"`
		LogID     string          `json:"log_id"`
		EntryType string          `json:"entry_type"`
		BodyBytes int             `json:"body_bytes"`
		Payload   json.RawMessage `json:"payload"`
	}

	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return ParsedRequestEntry{}, false
	}

	logID := strings.TrimSpace(raw.LogID)
	if logID == "" {
		logID = deriveLogIDFromRequestID(raw.RequestID)
	}

	entry := ParsedRequestEntry{
		Raw:       line,
		Timestamp: raw.Timestamp,
		RequestID: raw.RequestID,
		LogID:     logID,
		EntryType: raw.EntryType,
		BodyBytes: raw.BodyBytes,
	}

	if len(raw.Payload) > 0 && string(raw.Payload) != "null" {
		entry.Payload = raw.Payload
	}

	return entry, true
}
