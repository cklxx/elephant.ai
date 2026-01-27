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
	requestLogFileName  = "streaming.log"

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
		opts.MaxLineBytes = 1 << 20
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

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), opts.MaxLineBytes)

	bytesRead := 0
	for scanner.Scan() {
		line := scanner.Text()
		bytesRead += len(line)
		if strings.Contains(line, logID) {
			snippet.Entries = append(snippet.Entries, line)
			if len(snippet.Entries) >= opts.MaxEntries {
				snippet.Truncated = true
				break
			}
		}
		if bytesRead >= opts.MaxBytes {
			snippet.Truncated = true
			break
		}
	}
	if err := scanner.Err(); err != nil {
		snippet.Error = err.Error()
	}

	return snippet
}

func readRequestLogMatches(path, logID string, opts LogFetchOptions) LogFileSnippet {
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

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), opts.MaxLineBytes)

	var block []string
	bytesRead := 0
	flush := func() {
		if len(block) == 0 {
			return
		}
		entry := strings.Join(block, "\n")
		if strings.Contains(entry, logID) {
			snippet.Entries = append(snippet.Entries, entry)
		}
		block = block[:0]
	}

	for scanner.Scan() {
		line := scanner.Text()
		bytesRead += len(line)
		if strings.TrimSpace(line) == "" {
			flush()
			if len(snippet.Entries) >= opts.MaxEntries {
				snippet.Truncated = true
				break
			}
			if bytesRead >= opts.MaxBytes {
				snippet.Truncated = true
				break
			}
			continue
		}
		block = append(block, line)
	}
	flush()

	if len(snippet.Entries) > opts.MaxEntries {
		snippet.Entries = snippet.Entries[:opts.MaxEntries]
		snippet.Truncated = true
	}

	if err := scanner.Err(); err != nil {
		snippet.Error = err.Error()
	}

	return snippet
}
