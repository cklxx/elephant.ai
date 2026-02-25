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

func logStreamingPayload(requestID string, payload []byte, entryType string) {
	if len(payload) == 0 {
		return
	}

	dir := resolveRequestLogDir()
	if strings.TrimSpace(dir) == "" {
		log.Printf("request log: resolved log directory empty")
		return
	}

	trimmedID := strings.TrimSpace(requestID)
	if trimmedID == "" {
		trimmedID = "unknown"
	}
	entryKey := fmt.Sprintf("%s:%s", trimmedID, entryType)
	if trimmedID != "unknown" && !shouldLogStreamingEntry(entryKey, defaultStreamingLogTTL) {
		return
	}

	entryLabel := strings.ToLower(strings.TrimSpace(entryType))
	if entryLabel == "" {
		entryLabel = "payload"
	}
	entry := requestLogEntry{
		Timestamp: time.Now().Format(time.RFC3339Nano),
		RequestID: trimmedID,
		LogID:     logIDFromRequestID(trimmedID),
		EntryType: entryLabel,
		BodyBytes: len(payload),
	}
	if json.Valid(payload) {
		entry.Payload = json.RawMessage(payload)
	} else {
		entry.PayloadText = string(payload)
	}

	entryBytes, err := json.Marshal(entry)
	if err != nil {
		log.Printf("request log: failed to encode entry: %v", err)
		return
	}
	entryBytes = append(entryBytes, '\n')

	path := filepath.Join(dir, requestLogFileName)
	enqueueRequestLogWrite(path, entryBytes)
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

type requestLogEntry struct {
	Timestamp   string          `json:"timestamp"`
	RequestID   string          `json:"request_id"`
	LogID       string          `json:"log_id,omitempty"`
	EntryType   string          `json:"entry_type"`
	BodyBytes   int             `json:"body_bytes"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	PayloadText string          `json:"payload_text,omitempty"`
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
