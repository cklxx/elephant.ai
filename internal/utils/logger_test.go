package utils

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/fatih/color"
)

func TestComponentLogger_Log(t *testing.T) {
	// 捕获日志输出
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)

	// 创建测试日志器
	logger := NewComponentLogger(ComponentLoggerConfig{
		ComponentName: "TEST",
		Color:         color.FgRed,
		EnabledLevels: []LogLevel{INFO, ERROR},
	})

	// 测试启用的日志级别
	logger.Info("test info message")
	output := buf.String()
	if !strings.Contains(output, "[TEST]") {
		t.Errorf("Expected component name in output, got: %s", output)
	}
	if !strings.Contains(output, "test info message") {
		t.Errorf("Expected message in output, got: %s", output)
	}

	// 清空缓冲区
	buf.Reset()

	// 测试未启用的日志级别（DEBUG未启用）
	logger.Debug("test debug message")
	if buf.Len() > 0 {
		t.Errorf("Expected no output for disabled level, got: %s", buf.String())
	}

	// 测试错误级别
	logger.Error("test error message")
	output = buf.String()
	if !strings.Contains(output, "test error message") {
		t.Errorf("Expected error message in output, got: %s", output)
	}
}

func TestComponentLogger_LevelMethods(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)

	logger := NewComponentLogger(ComponentLoggerConfig{
		ComponentName: "TEST",
		EnabledLevels: []LogLevel{DEBUG, INFO, WARN, ERROR},
	})

	tests := []struct {
		method   func(string, ...interface{})
		message  string
		expected string
	}{
		{logger.Debug, "debug message", "debug message"},
		{logger.Info, "info message", "info message"},
		{logger.Warn, "warn message", "warn message"},
		{logger.Error, "error message", "error message"},
	}

	for _, test := range tests {
		buf.Reset()
		test.method(test.message)
		output := buf.String()
		if !strings.Contains(output, test.expected) {
			t.Errorf("Expected '%s' in output, got: %s", test.expected, output)
		}
	}
}

func TestLoggerFactory_GetLogger(t *testing.T) {
	factory := &LoggerFactory{}

	tests := []struct {
		component string
		expected  *ComponentLogger
	}{
		{"REACT", ReactLogger},
		{"TOOL", ToolLogger},
		{"SUB-AGENT", SubAgentLogger},
		{"CORE", CoreLogger},
		{"LLM", LLMLogger},
	}

	for _, test := range tests {
		logger := factory.GetLogger(test.component)
		if logger != test.expected {
			t.Errorf("Expected %v for component %s, got %v", test.expected, test.component, logger)
		}
	}

	// 测试未知组件
	unknownLogger := factory.GetLogger("UNKNOWN")
	if unknownLogger == nil {
		t.Error("Expected logger for unknown component, got nil")
	}
}

func TestConvenienceFunctions(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)

	// 测试便利函数
	LogInfo("TEST", "test message")
	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected message in convenience function output, got: %s", output)
	}

	buf.Reset()
	LogError("TEST", "error message")
	output = buf.String()
	if !strings.Contains(output, "error message") {
		t.Errorf("Expected error message in convenience function output, got: %s", output)
	}
}

func TestComponentLoggerConfig_DefaultLevels(t *testing.T) {
	// 测试默认启用所有级别
	logger := NewComponentLogger(ComponentLoggerConfig{
		ComponentName: "TEST",
	})

	// 检查所有级别都应该启用
	expectedLevels := []LogLevel{DEBUG, INFO, WARN, ERROR}
	for _, level := range expectedLevels {
		if !logger.enabled[level] {
			t.Errorf("Expected level %s to be enabled by default", level)
		}
	}
}

func BenchmarkComponentLogger_Log(b *testing.B) {
	logger := NewComponentLogger(ComponentLoggerConfig{
		ComponentName: "BENCH",
		EnabledLevels: []LogLevel{INFO},
	})

	// 设置输出到丢弃
	log.SetOutput(&bytes.Buffer{})
	defer log.SetOutput(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message %d", i)
	}
}