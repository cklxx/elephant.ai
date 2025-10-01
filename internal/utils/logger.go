package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var (
	loggerInstance *Logger
	loggerOnce     sync.Once
)

// Logger provides structured logging to alex-debug.log
type Logger struct {
	file       *os.File
	logger     *log.Logger
	level      LogLevel
	mu         sync.Mutex
	component  string
	enableFile bool
}

// GetLogger returns the singleton logger instance
func GetLogger() *Logger {
	loggerOnce.Do(func() {
		loggerInstance = newLogger("", DEBUG, true)
	})
	return loggerInstance
}

// NewComponentLogger creates a logger for a specific component
func NewComponentLogger(component string) *Logger {
	logger := GetLogger()
	return &Logger{
		file:       logger.file,
		logger:     logger.logger,
		level:      logger.level,
		component:  component,
		enableFile: logger.enableFile,
	}
}

// newLogger creates a new Logger instance
func newLogger(component string, level LogLevel, enableFile bool) *Logger {
	l := &Logger{
		level:      level,
		component:  component,
		enableFile: enableFile,
	}

	if enableFile {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Printf("Failed to get home directory: %v", err)
			return l
		}

		logPath := filepath.Join(home, "alex-debug.log")
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
	logLine := fmt.Sprintf("%s [%s] [%s] %s:%d - %s\n",
		timestamp, levelStr, component, file, line, message)

	if l.logger != nil {
		l.logger.Print(logLine)
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
