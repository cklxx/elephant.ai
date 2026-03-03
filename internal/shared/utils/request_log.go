package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	requestLogOnce      sync.Once
	requestLogQueue     chan requestLogWrite
	requestLogPending   atomic.Int64
	streamingLogDeduper sync.Map
)

const (
	requestLogEnvVar    = "ALEX_REQUEST_LOG_DIR"
	requestLogSubfolder = "logs/requests"
	requestLogFileName  = "llm.jsonl"
)

const (
	defaultStreamingLogTTL = 5 * time.Minute
	requestLogQueueSize    = 256
)

// LLMErrorLogDetails captures normalized failure metadata for request log error entries.
type LLMErrorLogDetails struct {
	Mode       string `json:"mode,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
	Intent     string `json:"intent,omitempty"`
	Stage      string `json:"stage,omitempty"`
	ErrorClass string `json:"error_class,omitempty"`
	Error      string `json:"error,omitempty"`
	LatencyMS  int64  `json:"latency_ms,omitempty"`
}

// LogStreamingRequestPayload persists the serialized request payload as a JSONL log entry.
// The payload is written to logs/requests/llm.jsonl (or the directory specified via
// ALEX_REQUEST_LOG_DIR) so it doesn't mix with the general server logs.
func LogStreamingRequestPayload(requestID string, payload []byte) {
	logStreamingPayload(requestID, payload, "request")
}

// LogStreamingResponsePayload persists the serialized response payload as a JSONL log entry.
// The payload is written to logs/requests/llm.jsonl (or the directory specified via
// ALEX_REQUEST_LOG_DIR) to keep it isolated from the general server logs.
func LogStreamingResponsePayload(requestID string, payload []byte) {
	logStreamingPayload(requestID, payload, "response")
}

// LogStreamingErrorPayload persists normalized LLM failure metadata as an error entry.
// The entry is written to logs/requests/llm.jsonl with entry_type=error.
func LogStreamingErrorPayload(requestID string, details LLMErrorLogDetails) {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		log.Printf("request log: failed to encode error details: %v", err)
		return
	}

	entry := buildRequestLogEntry(requestID, "error")
	entry.BodyBytes = len(detailsJSON)
	entry.Mode = strings.TrimSpace(details.Mode)
	entry.Provider = strings.TrimSpace(details.Provider)
	entry.Model = strings.TrimSpace(details.Model)
	entry.Intent = strings.TrimSpace(details.Intent)
	entry.Stage = strings.TrimSpace(details.Stage)
	entry.ErrorClass = strings.TrimSpace(details.ErrorClass)
	entry.Error = strings.TrimSpace(details.Error)
	if details.LatencyMS > 0 {
		entry.LatencyMS = details.LatencyMS
	}

	// Error payloads are small enough to write inline via the separate payload line.
	writeRequestLogEntryWithPayload(entry, detailsJSON)
}

func logStreamingPayload(requestID string, payload []byte, entryType string) {
	if len(payload) == 0 {
		return
	}
	entry := buildRequestLogEntry(requestID, entryType)
	entry.BodyBytes = len(payload)

	// Write the entry metadata (without full payload) as a JSONL line,
	// then append the raw payload on a separate line. This avoids
	// json.Marshal re-serializing the entire payload into the entry,
	// which was a major source of O(N²) memory growth during long runs.
	writeRequestLogEntryWithPayload(entry, payload)
}

func resolveRequestLogDir() string {
	var dir string
	if value, ok := os.LookupEnv(requestLogEnvVar); ok {
		dir = strings.TrimSpace(value)
	}
	if dir == "" {
		base, err := os.Getwd()
		if err != nil {
			base = "."
		}
		dir = filepath.Join(base, requestLogSubfolder)
	}
	return dir
}

func shouldLogStreamingEntry(requestID string, ttl time.Duration) bool {
	now := time.Now()
	if ttl <= 0 {
		ttl = defaultStreamingLogTTL
	}
	if value, ok := streamingLogDeduper.Load(requestID); ok {
		if ts, ok := value.(time.Time); ok {
			if now.Sub(ts) < ttl {
				return false
			}
		}
	}
	streamingLogDeduper.Store(requestID, now)
	time.AfterFunc(ttl, func() {
		if value, ok := streamingLogDeduper.Load(requestID); ok {
			if ts, ok := value.(time.Time); ok && ts.Equal(now) {
				streamingLogDeduper.Delete(requestID)
			}
		}
	})
	return true
}

type requestLogWrite struct {
	path  string
	entry []byte
}

// payloadPreviewBytes is the max number of bytes from the raw payload to
// include inline in the metadata entry for quick debugging.
const payloadPreviewBytes = 2048

type requestLogEntry struct {
	Timestamp      string `json:"timestamp"`
	RequestID      string `json:"request_id"`
	LogID          string `json:"log_id,omitempty"`
	EntryType      string `json:"entry_type"`
	BodyBytes      int    `json:"body_bytes"`
	Mode           string `json:"mode,omitempty"`
	Provider       string `json:"provider,omitempty"`
	Model          string `json:"model,omitempty"`
	Intent         string `json:"intent,omitempty"`
	Stage          string `json:"stage,omitempty"`
	ErrorClass     string `json:"error_class,omitempty"`
	Error          string `json:"error,omitempty"`
	LatencyMS      int64  `json:"latency_ms,omitempty"`
	PayloadPreview string `json:"payload_preview,omitempty"` // first N bytes of payload
	// NOTE: full Payload field removed to prevent json.Marshal from
	// re-serializing the entire LLM request/response body inside the
	// entry. The full payload is written as a separate JSONL line.
}

func buildRequestLogEntry(requestID string, entryType string) requestLogEntry {
	trimmedID := strings.TrimSpace(requestID)
	if trimmedID == "" {
		trimmedID = "unknown"
	}
	entryLabel := TrimLower(entryType)
	if entryLabel == "" {
		entryLabel = "payload"
	}
	return requestLogEntry{
		Timestamp: time.Now().Format(time.RFC3339Nano),
		RequestID: trimmedID,
		LogID:     logIDFromRequestID(trimmedID),
		EntryType: entryLabel,
	}
}

// writeRequestLogEntryWithPayload writes the entry metadata and full payload
// as two consecutive JSONL lines. The entry gets a truncated preview; the
// full payload follows on the next line so that json.Marshal never needs to
// re-encode the (potentially multi-MB) body.
func writeRequestLogEntryWithPayload(entry requestLogEntry, payload []byte) {
	dir := resolveRequestLogDir()
	if IsBlank(dir) {
		log.Printf("request log: resolved log directory empty")
		return
	}

	entryKey := fmt.Sprintf("%s:%s", entry.RequestID, entry.EntryType)
	if entry.RequestID != "unknown" && !shouldLogStreamingEntry(entryKey, defaultStreamingLogTTL) {
		return
	}

	// Attach a truncated preview for quick debugging.
	if len(payload) > 0 {
		preview := payload
		if len(preview) > payloadPreviewBytes {
			preview = preview[:payloadPreviewBytes]
		}
		entry.PayloadPreview = string(preview)
	}

	entryBytes, err := json.Marshal(entry)
	if err != nil {
		log.Printf("request log: failed to encode entry: %v", err)
		return
	}

	// Build a single write buffer: metadata line + payload line.
	// This keeps the two lines atomic from the writer's perspective.
	buf := make([]byte, 0, len(entryBytes)+1+len(payload)+1)
	buf = append(buf, entryBytes...)
	buf = append(buf, '\n')
	if len(payload) > 0 {
		buf = append(buf, payload...)
		buf = append(buf, '\n')
	}

	enqueueRequestLogWrite(filepath.Join(dir, requestLogFileName), buf)
}

func logIDFromRequestID(requestID string) string {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return ""
	}
	marker := ":llm-"
	if idx := strings.LastIndex(requestID, marker); idx > 0 {
		return requestID[:idx]
	}
	return ""
}

func enqueueRequestLogWrite(path string, entry []byte) {
	initRequestLogWriter()
	requestLogPending.Add(1)
	select {
	case requestLogQueue <- requestLogWrite{path: path, entry: entry}:
	default:
		requestLogPending.Add(-1)
		log.Printf("request log: queue full, dropping entry")
	}
}

func initRequestLogWriter() {
	requestLogOnce.Do(func() {
		requestLogQueue = make(chan requestLogWrite, requestLogQueueSize)
		go requestLogWriter()
	})
}

func requestLogWriter() {
	for item := range requestLogQueue {
		if err := appendRequestLogEntry(item.path, item.entry); err != nil {
			log.Printf("request log: failed to write entry: %v", err)
		}
		requestLogPending.Add(-1)
	}
}

func appendRequestLogEntry(path string, entry []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	if _, err := file.Write(entry); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// WaitForRequestLogQueueDrain waits for async request log writes to finish or timeout.
// Intended for tests that need to read log files after logging.
func WaitForRequestLogQueueDrain(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if requestLogPending.Load() == 0 {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return requestLogPending.Load() == 0
}
