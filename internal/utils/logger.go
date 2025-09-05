package utils

import (
	"fmt"
	"log"
	"strings"

	"github.com/fatih/color"
)

// LogLevel represents the severity level of a log message
type LogLevel string

const (
	DEBUG LogLevel = "DEBUG"
	INFO  LogLevel = "INFO"
	WARN  LogLevel = "WARN"
	ERROR LogLevel = "ERROR"
)

// ComponentLogger provides component-specific logging with colors and prefixes
type ComponentLogger struct {
	componentName string
	colorFunc     func(...interface{}) string
	enabled       map[LogLevel]bool
}

// ComponentLoggerConfig configures a component logger
type ComponentLoggerConfig struct {
	ComponentName string
	Color         color.Attribute
	EnabledLevels []LogLevel
}

var (
	// Predefined color functions for different components
	defaultColor = color.New(color.Reset).SprintFunc()
	errorColor   = color.New(color.FgRed).SprintFunc()
	warnColor    = color.New(color.FgYellow).SprintFunc()
	debugColor   = color.New(color.FgCyan).SprintFunc()
)

// NewComponentLogger creates a new component-specific logger
func NewComponentLogger(config ComponentLoggerConfig) *ComponentLogger {
	logger := &ComponentLogger{
		componentName: config.ComponentName,
		enabled:       make(map[LogLevel]bool),
	}

	// Set up color function
	if config.Color != 0 {
		logger.colorFunc = color.New(config.Color).SprintFunc()
	} else {
		logger.colorFunc = defaultColor
	}

	// Enable specified log levels (default: all levels enabled)
	if len(config.EnabledLevels) == 0 {
		config.EnabledLevels = []LogLevel{DEBUG, INFO, WARN, ERROR}
	}
	for _, level := range config.EnabledLevels {
		logger.enabled[level] = true
	}

	return logger
}

// Log logs a message with the specified level
func (cl *ComponentLogger) Log(level LogLevel, format string, args ...interface{}) {
	if !cl.enabled[level] {
		return
	}

	var levelColor func(...interface{}) string
	switch level {
	case ERROR:
		levelColor = errorColor
	case WARN:
		levelColor = warnColor
	case DEBUG:
		levelColor = debugColor
	default:
		levelColor = defaultColor
	}

	prefix := cl.colorFunc(fmt.Sprintf("[%s]", cl.componentName))
	levelStr := levelColor(string(level))
	message := fmt.Sprintf(format, args...)

	log.Printf("%s [%s] %s", prefix, levelStr, message)
}

// Debug logs a debug message
func (cl *ComponentLogger) Debug(format string, args ...interface{}) {
	cl.Log(DEBUG, format, args...)
}

// Info logs an info message
func (cl *ComponentLogger) Info(format string, args ...interface{}) {
	cl.Log(INFO, format, args...)
}

// Warn logs a warning message
func (cl *ComponentLogger) Warn(format string, args ...interface{}) {
	cl.Log(WARN, format, args...)
}

// Error logs an error message
func (cl *ComponentLogger) Error(format string, args ...interface{}) {
	cl.Log(ERROR, format, args...)
}

// Global component loggers
var (
	ReactLogger    *ComponentLogger
	ToolLogger     *ComponentLogger
	SubAgentLogger *ComponentLogger
	CoreLogger     *ComponentLogger
	LLMLogger      *ComponentLogger
)

// Initialize global loggers
func init() {
	ReactLogger = NewComponentLogger(ComponentLoggerConfig{
		ComponentName: "REACT-AGENT",
		Color:         color.FgBlue,
	})

	ToolLogger = NewComponentLogger(ComponentLoggerConfig{
		ComponentName: "TOOL-EXEC",
		Color:         color.FgGreen,
	})

	SubAgentLogger = NewComponentLogger(ComponentLoggerConfig{
		ComponentName: "SUB-AGENT",
		Color:         color.FgMagenta,
	})

	CoreLogger = NewComponentLogger(ComponentLoggerConfig{
		ComponentName: "REACT-CORE",
		Color:         color.FgCyan,
	})

	LLMLogger = NewComponentLogger(ComponentLoggerConfig{
		ComponentName: "LLM-HANDLER",
		Color:         color.FgYellow,
	})
}

// LoggerFactory provides easy access to component loggers
type LoggerFactory struct{}

// GetLogger returns a logger for the specified component
func (lf *LoggerFactory) GetLogger(component string) *ComponentLogger {
	component = strings.ToUpper(component)
	switch component {
	case "REACT", "REACT-AGENT":
		return ReactLogger
	case "TOOL", "TOOL-EXEC", "TOOL-EXECUTOR":
		return ToolLogger
	case "SUB-AGENT", "SUBAGENT":
		return SubAgentLogger
	case "CORE", "REACT-CORE":
		return CoreLogger
	case "LLM", "LLM-HANDLER":
		return LLMLogger
	default:
		return NewComponentLogger(ComponentLoggerConfig{
			ComponentName: component,
			Color:         color.Reset,
		})
	}
}

// Global logger factory instance
var Logger = &LoggerFactory{}

// Convenience functions for backward compatibility
func LogDebug(component, format string, args ...interface{}) {
	Logger.GetLogger(component).Debug(format, args...)
}

func LogInfo(component, format string, args ...interface{}) {
	Logger.GetLogger(component).Info(format, args...)
}

func LogWarn(component, format string, args ...interface{}) {
	Logger.GetLogger(component).Warn(format, args...)
}

func LogError(component, format string, args ...interface{}) {
	Logger.GetLogger(component).Error(format, args...)
}
