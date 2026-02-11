package logging

import (
	"reflect"

	"alex/internal/shared/utils"
)

// Logger defines a minimal, printf-style logging contract.
//
// It intentionally matches the agent domain logger interface so code can depend
// on this package without importing internal/agent/ports.
type Logger interface {
	Debug(format string, args ...any)
	Info(format string, args ...any)
	Warn(format string, args ...any)
	Error(format string, args ...any)
}

type nopLogger struct{}

func (nopLogger) Debug(string, ...any) {}
func (nopLogger) Info(string, ...any)  {}
func (nopLogger) Warn(string, ...any)  {}
func (nopLogger) Error(string, ...any) {}

// Nop returns a logger that discards all output.
func Nop() Logger {
	return nopLogger{}
}

// IsNil reports whether logger is nil or wraps a nil pointer receiver.
func IsNil(logger Logger) bool {
	if logger == nil {
		return true
	}
	val := reflect.ValueOf(logger)
	switch val.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Func:
		return val.IsNil()
	default:
		return false
	}
}

// OrNop returns logger when non-nil, otherwise a no-op logger.
func OrNop(logger Logger) Logger {
	if IsNil(logger) {
		return Nop()
	}
	return logger
}

// NewComponentLogger returns the default application logger scoped to a component.
func NewComponentLogger(component string) Logger {
	return utils.NewComponentLogger(component)
}

// NewLatencyLogger returns a logger dedicated to latency instrumentation output.
func NewLatencyLogger(component string) Logger {
	return utils.NewLatencyLogger(component)
}

// NewLLMLogger returns a logger that writes to the dedicated LLM log file (alex-llm.log).
func NewLLMLogger(component string) Logger {
	return utils.NewCategorizedLogger(utils.LogCategoryLLM, component)
}

// NewKernelLogger returns a logger that writes to the dedicated kernel log file (alex-kernel.log).
func NewKernelLogger(component string) Logger {
	return utils.NewCategorizedLogger(utils.LogCategoryKernel, component)
}

// FromUtils adapts the legacy utils logger to the Logger interface.
func FromUtils(logger *utils.Logger) Logger {
	if logger == nil {
		return Nop()
	}
	return logger
}

type multiLogger struct {
	loggers []Logger
}

// Multi returns a logger fan-out that calls every non-nil logger in order.
func Multi(loggers ...Logger) Logger {
	flattened := make([]Logger, 0, len(loggers))
	for _, logger := range loggers {
		if IsNil(logger) {
			continue
		}
		if ml, ok := logger.(*multiLogger); ok {
			flattened = append(flattened, ml.loggers...)
			continue
		}
		flattened = append(flattened, logger)
	}
	if len(flattened) == 0 {
		return Nop()
	}
	if len(flattened) == 1 {
		return flattened[0]
	}
	return &multiLogger{loggers: flattened}
}

func (l *multiLogger) Debug(format string, args ...any) {
	for _, logger := range l.loggers {
		logger.Debug(format, args...)
	}
}

func (l *multiLogger) Info(format string, args ...any) {
	for _, logger := range l.loggers {
		logger.Info(format, args...)
	}
}

func (l *multiLogger) Warn(format string, args ...any) {
	for _, logger := range l.loggers {
		logger.Warn(format, args...)
	}
}

func (l *multiLogger) Error(format string, args ...any) {
	for _, logger := range l.loggers {
		logger.Error(format, args...)
	}
}
