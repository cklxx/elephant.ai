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
	requestLogEnvVar           = "ALEX_REQUEST_LOG_DIR"
	requestLogSubfolder        = "logs/requests"
	requestLogRequestFileName  = "streaming_request.log"
	requestLogResponseFileName = "streaming_response.log"
)

const (
	defaultStreamingLogTTL = 5 * time.Minute
)

// LogStreamingRequestPayload persists the serialized request payload for a streaming request.
// The payload is written to logs/requests/streaming_request.log (or the directory specified via
// ALEX_REQUEST_LOG_DIR) so it doesn't mix with the general server logs captured by deploy.sh.
func LogStreamingRequestPayload(requestID string, payload []byte) {
	logStreamingPayload(requestID, payload, requestLogRequestFileName)
}

// LogStreamingResponsePayload persists the serialized response payload for a streaming request.
// The payload is written to logs/requests/streaming_response.log (or the directory specified via
// ALEX_REQUEST_LOG_DIR) to keep it isolated from the general server logs.
func LogStreamingResponsePayload(requestID string, payload []byte) {
	logStreamingPayload(requestID, payload, requestLogResponseFileName)
}

func logStreamingPayload(requestID string, payload []byte, fileName string) {
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
	if trimmedID != "unknown" && !shouldLogStreamingEntry(trimmedID+":"+fileName, defaultStreamingLogTTL) {
		return
	}

	entry := fmt.Sprintf("%s [req:%s] body_bytes=%d\n%s\n\n",
		time.Now().Format(time.RFC3339Nano), trimmedID, len(payload), string(payload))

	path := filepath.Join(dir, fileName)
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
