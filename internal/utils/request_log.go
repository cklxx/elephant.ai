package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	requestLogMu        sync.Mutex
	streamingLogDeduper sync.Map
)

const (
	requestLogEnvVar    = "ALEX_REQUEST_LOG_DIR"
	requestLogSubfolder = "logs/requests"
	requestLogFileName  = "streaming.log"
)

const (
	defaultStreamingLogTTL = 5 * time.Minute
)

// LogStreamingRequestPayload persists the serialized request payload for a streaming request.
// The payload is written to logs/requests/streaming.log (or the directory specified via
// ALEX_REQUEST_LOG_DIR) so it doesn't mix with the general server logs captured by deploy.sh.
func LogStreamingRequestPayload(requestID string, payload []byte) {
	logStreamingPayload(requestID, payload, "request")
}

// LogStreamingResponsePayload persists the serialized response payload for a streaming request.
// The payload is written to logs/requests/streaming.log (or the directory specified via
// ALEX_REQUEST_LOG_DIR) to keep it isolated from the general server logs.
func LogStreamingResponsePayload(requestID string, payload []byte) {
	logStreamingPayload(requestID, payload, "response")
}

// LogStreamingSummary persists the textual or serialized summary for a streaming request.
// The summary is logged separately so operators can quickly review the LLM Streaming Summary
// that is also emitted to the structured logger.
func LogStreamingSummary(requestID string, payload []byte) {
	logStreamingPayload(requestID, payload, "summary")
}

func logStreamingPayload(requestID string, payload []byte, entryType string) {
	if len(payload) == 0 {
		return
	}

	dir, err := ensureRequestLogDir()
	if err != nil {
		log.Printf("request log: failed to prepare directory: %v", err)
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
	entry := fmt.Sprintf("%s [req:%s] [%s] body_bytes=%d\n%s\n\n",
		time.Now().Format(time.RFC3339Nano), trimmedID, entryLabel, len(payload), string(payload))

	path := filepath.Join(dir, requestLogFileName)
	requestLogMu.Lock()
	defer requestLogMu.Unlock()

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("request log: failed to open %s: %v", path, err)
		return
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			log.Printf("request log: failed to close %s: %v", path, cerr)
		}
	}()

	if _, err := file.WriteString(entry); err != nil {
		log.Printf("request log: failed to write entry: %v", err)
	}
}

func ensureRequestLogDir() (string, error) {
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
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
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
			if ts, ok := value.(time.Time); ok && ts == now {
				streamingLogDeduper.Delete(requestID)
			}
		}
	})
	return true
}
