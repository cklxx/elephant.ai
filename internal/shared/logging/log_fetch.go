package logging

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

const (
	logDirEnvVar        = "ALEX_LOG_DIR"
	requestLogEnvVar    = "ALEX_REQUEST_LOG_DIR"
	requestLogSubfolder = "logs/requests"
	requestLogFileName  = "llm.jsonl"

	serviceLogFileName = "alex-service.log"
	llmLogFileName     = "alex-llm.log"
	latencyLogFileName = "alex-latency.log"
)

// LogFileSnippet captures matched log lines for a single file.
type LogFileSnippet struct {
	Path      string   `json:"path,omitempty"`
	Entries   []string `json:"entries,omitempty"`
	Truncated bool     `json:"truncated,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// LogBundle aggregates log snippets across log categories for a single log id.
type LogBundle struct {
	LogID    string         `json:"log_id"`
	Service  LogFileSnippet `json:"service"`
	LLM      LogFileSnippet `json:"llm"`
	Latency  LogFileSnippet `json:"latency"`
	Requests LogFileSnippet `json:"requests"`
}

// LogFetchOptions tunes how much log data is returned.
type LogFetchOptions struct {
	MaxEntries   int
	MaxBytes     int
	MaxLineBytes int
}

// FetchLogBundle returns log lines that match the provided log id across known log files.
func FetchLogBundle(logID string, opts LogFetchOptions) LogBundle {
	logID = strings.TrimSpace(logID)
	bundle := LogBundle{LogID: logID}
	if logID == "" {
		errMsg := "log_id is required"
		bundle.Service.Error = errMsg
		bundle.LLM.Error = errMsg
		bundle.Latency.Error = errMsg
		bundle.Requests.Error = errMsg
		return bundle
	}

	opts = normalizeLogFetchOptions(opts)
	logDir := resolveLogDirectory()
	requestDir := resolveRequestLogDirectory()

	bundle.Service = readLogMatches(filepath.Join(logDir, serviceLogFileName), logID, opts)
	bundle.LLM = readLogMatches(filepath.Join(logDir, llmLogFileName), logID, opts)
	bundle.Latency = readLogMatches(filepath.Join(logDir, latencyLogFileName), logID, opts)
	bundle.Requests = readRequestLogMatches(filepath.Join(requestDir, requestLogFileName), logID, opts)

	return bundle
}

func normalizeLogFetchOptions(opts LogFetchOptions) LogFetchOptions {
	if opts.MaxEntries <= 0 {
		opts.MaxEntries = 200
	}
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = 1 << 20
	}
	if opts.MaxLineBytes <= 0 {
		opts.MaxLineBytes = 8 << 20
	}
	return opts
}

func resolveLogDirectory() string {
	if value, ok := os.LookupEnv(logDirEnvVar); ok {
		if override := strings.TrimSpace(value); override != "" {
			return override
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "."
	}
	return home
}

func resolveRequestLogDirectory() string {
	if value, ok := os.LookupEnv(requestLogEnvVar); ok {
		if override := strings.TrimSpace(value); override != "" {
			return override
		}
	}
	base, err := os.Getwd()
	if err != nil || strings.TrimSpace(base) == "" {
		base = "."
	}
	return filepath.Join(base, requestLogSubfolder)
}

func readLogMatches(path, logID string, opts LogFetchOptions) LogFileSnippet {
	opts = normalizeLogFetchOptions(opts)
	snippet := LogFileSnippet{Path: path}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			snippet.Error = "not_found"
		} else {
			snippet.Error = err.Error()
		}
		return snippet
	}
	defer func() { _ = file.Close() }()

	reader := bufio.NewReaderSize(file, 64*1024)
	matchedBytes := 0
	for {
		line, err := readLineString(reader, opts.MaxLineBytes)
		if err != nil {
			break
		}
		if line == "" {
			continue
		}
		if strings.Contains(line, logID) {
			snippet.Entries = append(snippet.Entries, line)
			matchedBytes += len(line)
			if len(snippet.Entries) >= opts.MaxEntries {
				snippet.Truncated = true
				break
			}
			if matchedBytes >= opts.MaxBytes {
				snippet.Truncated = true
				break
			}
		}
	}

	return snippet
}

// readLineString reads a single newline-terminated line from reader.
// Lines longer than maxBytes are silently skipped (drained and discarded).
// Returns ("", io.EOF) at end of input.
func readLineString(reader *bufio.Reader, maxBytes int) (string, error) {
	var buf []byte
	oversize := false
	for {
		segment, isPrefix, err := reader.ReadLine()
		if err != nil {
			if len(buf) > 0 && !oversize {
				return string(buf), nil
			}
			return "", err
		}
		if oversize {
			if !isPrefix {
				oversize = false
			}
			continue
		}
		buf = append(buf, segment...)
		if len(buf) > maxBytes {
			buf = nil
			if isPrefix {
				oversize = true
			}
			// Line exceeds limit — skip entirely.
			continue
		}
		if !isPrefix {
			return string(buf), nil
		}
	}
}

func readRequestLogMatches(path, logID string, opts LogFetchOptions) LogFileSnippet {
	return readLogMatches(path, logID, opts)
}
