package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const logDirEnvVar = "ALEX_LOG_DIR"

// LogLevel represents the severity of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

type LogCategory string

const (
	LogCategoryService LogCategory = "service"
	LogCategoryLLM     LogCategory = "llm"
	LogCategoryLatency LogCategory = "latency"
)

var (
	loggerInstance  *Logger
	loggerOnce      sync.Once
	categoryMu      sync.Mutex
	categoryLoggers = make(map[LogCategory]*Logger)
)

// Logger provides structured logging to alex-debug.log
type Logger struct {
	file       *os.File
	logger     *log.Logger
	level      LogLevel
	mu         sync.Mutex
	component  string
	enableFile bool
	category   LogCategory
	logID      string
}

// GetLogger returns the singleton logger instance
func GetLogger() *Logger {
	return getOrCreateCategoryLogger(LogCategoryService)
}

// NewComponentLogger creates a logger for a specific component
func NewComponentLogger(component string) *Logger {
	return NewCategorizedLogger(LogCategoryService, component)
}

// NewLatencyLogger creates a logger dedicated to latency instrumentation output.
func NewLatencyLogger(component string) *Logger {
	return NewCategorizedLogger(LogCategoryLatency, component)
}

// NewCategorizedLogger creates a logger for a specific category and component.
func NewCategorizedLogger(category LogCategory, component string) *Logger {
	base := getOrCreateCategoryLogger(category)
	return &Logger{
		file:       base.file,
		logger:     base.logger,
		level:      base.level,
		component:  component,
		enableFile: base.enableFile,
		category:   category,
	}
}

func getOrCreateCategoryLogger(category LogCategory) *Logger {
	if category == LogCategoryService {
		loggerOnce.Do(func() {
			loggerInstance = newLogger("", DEBUG, true, category)
		})
		return loggerInstance
	}

	categoryMu.Lock()
	defer categoryMu.Unlock()

	if logger, ok := categoryLoggers[category]; ok {
		return logger
	}

	logger := newLogger("", DEBUG, true, category)
	categoryLoggers[category] = logger
	return logger
}

// newLogger creates a new Logger instance
func newLogger(component string, level LogLevel, enableFile bool, category LogCategory) *Logger {
	l := &Logger{
		level:      level,
		component:  component,
		enableFile: enableFile,
		category:   category,
	}

	if enableFile {
		logDir, err := resolveLogDirectory()
		if err != nil {
			log.Printf("Failed to resolve log directory: %v", err)
			return l
		}
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			log.Printf("Failed to create log directory %s: %v", logDir, err)
			return l
		}

		logPath := filepath.Join(logDir, logFileName(category))
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Printf("Failed to open log file: %v", err)
			return l
		}

		l.file = file
		l.logger = log.New(file, "", 0) // We'll format ourselves
	}

	return l
}

func resolveLogDirectory() (string, error) {
	if override := strings.TrimSpace(os.Getenv(logDirEnvVar)); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return home, nil
}

func logFileName(category LogCategory) string {
	switch category {
	case LogCategoryLLM:
		return "alex-llm.log"
	case LogCategoryLatency:
		return "alex-latency.log"
	default:
		return "alex-service.log"
	}
}

// OpenLogFile opens (or creates) the log file for the given category.
func OpenLogFile(category LogCategory) (*os.File, error) {
	logDir, err := resolveLogDirectory()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}

	logPath := filepath.Join(logDir, logFileName(category))
	return os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// Close closes the log file
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// WithLogID returns a shallow copy of the logger that tags log lines with a log id.
func (l *Logger) WithLogID(logID string) *Logger {
	if l == nil {
		return nil
	}
	if strings.TrimSpace(logID) == "" {
		return l
	}
	return &Logger{
		file:       l.file,
		logger:     l.logger,
		level:      l.level,
		component:  l.component,
		enableFile: l.enableFile,
		category:   l.category,
		logID:      logID,
	}
}

// log is the internal logging function
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level || !l.enableFile {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Get caller info
	_, file, line, ok := runtime.Caller(2)
	if ok {
		file = filepath.Base(file)
	} else {
		file = "???"
		line = 0
	}

	// Format: 2025-09-30 12:34:56 [INFO] [ComponentName] file.go:123 - Message
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelStr := levelToString(level)
	component := l.component
	if component == "" {
		component = "ALEX"
	}

	message := fmt.Sprintf(format, args...)
	category := strings.ToUpper(string(l.category))
	if category == "" {
		category = "SERVICE"
	}
	logID := strings.TrimSpace(l.logID)
	if logID != "" {
		logLine := fmt.Sprintf("%s [%s] [%s] [%s] [log_id=%s] %s:%d - %s\n",
			timestamp, levelStr, category, component, logID, file, line, message)
		sanitizedLine := sanitizeLogLine(logLine)
		if l.logger != nil {
			l.logger.Print(sanitizedLine)
		}
		if os.Getenv("ALEX_SERVER_MODE") == "deploy" {
			fmt.Print(sanitizedLine)
		}
		return
	}

	logLine := fmt.Sprintf("%s [%s] [%s] [%s] %s:%d - %s\n",
		timestamp, levelStr, category, component, file, line, message)

	sanitizedLine := sanitizeLogLine(logLine)
	if l.logger != nil {
		l.logger.Print(sanitizedLine)
	}
	if os.Getenv("ALEX_SERVER_MODE") == "deploy" {
		fmt.Print(sanitizedLine)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// levelToString converts LogLevel to string
func levelToString(level LogLevel) string {
	switch level {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Helper functions for global logging
func Debug(format string, args ...interface{}) {
	GetLogger().Debug(format, args...)
}

func Info(format string, args ...interface{}) {
	GetLogger().Info(format, args...)
}

func Warn(format string, args ...interface{}) {
	GetLogger().Warn(format, args...)
}

func Error(format string, args ...interface{}) {
	GetLogger().Error(format, args...)
}

func sanitizeLogLine(line string) string {
	return line
}
