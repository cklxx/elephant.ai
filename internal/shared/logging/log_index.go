package logging

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	defaultLogIndexLimit = 80
	maxLogIndexLimit     = 500
)

var textLogIDPattern = regexp.MustCompile(`log_id=([^\]\s]+)`)

// LogIndexOptions configures recent log index scanning behavior.
type LogIndexOptions struct {
	Limit        int
	Offset       int
	MaxLineBytes int
}

// LogIndexEntry summarizes one log chain keyed by log_id.
type LogIndexEntry struct {
	LogID        string    `json:"log_id"`
	LastSeen     time.Time `json:"last_seen"`
	ServiceCount int       `json:"service_count"`
	LLMCount     int       `json:"llm_count"`
	LatencyCount int       `json:"latency_count"`
	RequestCount int       `json:"request_count"`
	TotalCount   int       `json:"total_count"`
	Sources      []string  `json:"sources,omitempty"`
}

type logIndexAggregate struct {
	LogIndexEntry
	sourceSet map[string]struct{}
}

// FetchRecentLogIndex scans known log files and returns recent log_id summaries.
func FetchRecentLogIndex(opts LogIndexOptions) []LogIndexEntry {
	opts = normalizeLogIndexOptions(opts)

	aggregates := map[string]*logIndexAggregate{}
	logDir := resolveLogDirectory()
	requestDir := resolveRequestLogDirectory()

	scanTextLogIndex(filepath.Join(logDir, serviceLogFileName), "service", opts, aggregates)
	scanTextLogIndex(filepath.Join(logDir, llmLogFileName), "llm", opts, aggregates)
	scanTextLogIndex(filepath.Join(logDir, latencyLogFileName), "latency", opts, aggregates)
	scanRequestLogIndex(filepath.Join(requestDir, requestLogFileName), opts, aggregates)

	entries := make([]LogIndexEntry, 0, len(aggregates))
	for _, aggregate := range aggregates {
		entry := aggregate.LogIndexEntry
		// Filter noise: entries with very few lines and no LLM/request activity
		// are typically HTTP middleware artifacts (log_id + latency only).
		if entry.TotalCount <= 2 && entry.LLMCount == 0 && entry.RequestCount == 0 {
			continue
		}
		entry.Sources = mapKeysSorted(aggregate.sourceSet)
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		left := entries[i]
		right := entries[j]
		if !left.LastSeen.Equal(right.LastSeen) {
			return left.LastSeen.After(right.LastSeen)
		}
		if left.TotalCount != right.TotalCount {
			return left.TotalCount > right.TotalCount
		}
		return left.LogID < right.LogID
	})

	if opts.Offset > 0 {
		if opts.Offset >= len(entries) {
			return nil
		}
		entries = entries[opts.Offset:]
	}
	if len(entries) > opts.Limit {
		entries = entries[:opts.Limit]
	}
	return entries
}

func normalizeLogIndexOptions(opts LogIndexOptions) LogIndexOptions {
	if opts.Limit <= 0 {
		opts.Limit = defaultLogIndexLimit
	}
	if opts.Limit > maxLogIndexLimit {
		opts.Limit = maxLogIndexLimit
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	if opts.MaxLineBytes <= 0 {
		opts.MaxLineBytes = 8 << 20
	}
	return opts
}

func scanTextLogIndex(path, source string, opts LogIndexOptions, aggregates map[string]*logIndexAggregate) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	reader := bufio.NewReaderSize(file, 64*1024)
	for {
		line, err := readLineString(reader, opts.MaxLineBytes)
		if err != nil {
			return
		}
		logID := extractLogIDFromTextLine(line)
		if logID == "" {
			continue
		}
		updateLogIndexAggregate(aggregates, logID, source, parseTimestampFromTextLog(line))
	}
}

func scanRequestLogIndex(path string, opts LogIndexOptions, aggregates map[string]*logIndexAggregate) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	reader := bufio.NewReaderSize(file, 64*1024)
	for {
		line, err := readLineString(reader, opts.MaxLineBytes)
		if err != nil {
			return
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		logID, ts := parseRequestLogLine(line)
		if logID == "" {
			continue
		}
		updateLogIndexAggregate(aggregates, logID, "requests", ts)
	}
}

func parseRequestLogLine(line string) (string, time.Time) {
	var entry struct {
		LogID     string `json:"log_id"`
		RequestID string `json:"request_id"`
		Timestamp string `json:"timestamp"`
	}
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return "", time.Time{}
	}

	logID := strings.TrimSpace(entry.LogID)
	if logID == "" {
		logID = deriveLogIDFromRequestID(entry.RequestID)
	}
	if logID == "" {
		return "", time.Time{}
	}

	ts, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(entry.Timestamp))
	if err != nil {
		return logID, time.Time{}
	}
	return logID, ts
}

func deriveLogIDFromRequestID(requestID string) string {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return ""
	}
	if idx := strings.LastIndex(requestID, ":llm-"); idx > 0 {
		return requestID[:idx]
	}
	return ""
}

func extractLogIDFromTextLine(line string) string {
	matches := textLogIDPattern.FindStringSubmatch(line)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func parseTimestampFromTextLog(line string) time.Time {
	if len(line) < 19 {
		return time.Time{}
	}
	raw := strings.TrimSpace(line[:19])
	if raw == "" {
		return time.Time{}
	}
	ts, err := time.ParseInLocation("2006-01-02 15:04:05", raw, time.Local)
	if err != nil {
		return time.Time{}
	}
	return ts
}

func updateLogIndexAggregate(aggregates map[string]*logIndexAggregate, logID, source string, ts time.Time) {
	logID = strings.TrimSpace(logID)
	if logID == "" {
		return
	}

	agg := aggregates[logID]
	if agg == nil {
		agg = &logIndexAggregate{
			LogIndexEntry: LogIndexEntry{LogID: logID},
			sourceSet:     map[string]struct{}{},
		}
		aggregates[logID] = agg
	}

	agg.TotalCount++
	switch source {
	case "service":
		agg.ServiceCount++
	case "llm":
		agg.LLMCount++
	case "latency":
		agg.LatencyCount++
	case "requests":
		agg.RequestCount++
	}
	agg.sourceSet[source] = struct{}{}

	if ts.IsZero() {
		return
	}
	if agg.LastSeen.IsZero() || ts.After(agg.LastSeen) {
		agg.LastSeen = ts
	}
}

func mapKeysSorted(sourceSet map[string]struct{}) []string {
	if len(sourceSet) == 0 {
		return nil
	}
	out := make([]string, 0, len(sourceSet))
	for key := range sourceSet {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
