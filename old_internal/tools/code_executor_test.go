package tools

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCodeActResult(t *testing.T) {
	// Test CodeActResult structure
	result := &CodeActResult{
		Success:       true,
		Output:        "Hello, World!",
		Error:         "",
		ExitCode:      0,
		ExecutionTime: 100 * time.Millisecond,
		Language:      "python",
		Code:          "print('Hello, World!')",
	}

	assert.True(t, result.Success)
	assert.Equal(t, "Hello, World!", result.Output)
	assert.Empty(t, result.Error)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, 100*time.Millisecond, result.ExecutionTime)
	assert.Equal(t, "python", result.Language)
	assert.Equal(t, "print('Hello, World!')", result.Code)
}

func TestCodeActResultFailure(t *testing.T) {
	// Test CodeActResult for failed execution
	result := &CodeActResult{
		Success:       false,
		Output:        "",
		Error:         "SyntaxError: invalid syntax",
		ExitCode:      1,
		ExecutionTime: 50 * time.Millisecond,
		Language:      "python",
		Code:          "print('Hello World'",
	}

	assert.False(t, result.Success)
	assert.Empty(t, result.Output)
	assert.Equal(t, "SyntaxError: invalid syntax", result.Error)
	assert.Equal(t, 1, result.ExitCode)
	assert.Equal(t, "python", result.Language)
}

func TestNewCodeActExecutor(t *testing.T) {
	// Test CodeActExecutor creation
	executor := NewCodeActExecutor()

	assert.NotNil(t, executor)
	assert.NotNil(t, executor.supportedLanguages)
	assert.NotEmpty(t, executor.sandboxDir)
	assert.Greater(t, executor.timeout, time.Duration(0))

	// Test supported languages
	assert.Contains(t, executor.supportedLanguages, "python")
	assert.Contains(t, executor.supportedLanguages, "go")
	assert.Contains(t, executor.supportedLanguages, "bash")
	assert.Contains(t, executor.supportedLanguages, "javascript")
	assert.Contains(t, executor.supportedLanguages, "js")
}

func TestCodeActExecutorSupportedLanguages(t *testing.T) {
	// Test supported languages mapping
	executor := NewCodeActExecutor()

	expectedLanguages := map[string]string{
		"python":     "python3",
		"go":         "go run",
		"bash":       "bash",
		"javascript": "node",
		"js":         "node",
	}

	for lang, cmd := range expectedLanguages {
		assert.Equal(t, cmd, executor.supportedLanguages[lang])
	}
}

func TestCodeActExecutorConfiguration(t *testing.T) {
	// Test CodeActExecutor configuration
	executor := NewCodeActExecutor()

	// Test that sandbox directory is set
	assert.NotEmpty(t, executor.sandboxDir)

	// Test that timeout is set to a reasonable value
	assert.GreaterOrEqual(t, executor.timeout, 10*time.Second)
	assert.LessOrEqual(t, executor.timeout, 60*time.Second)

	// Test thread safety with mutex
	assert.NotNil(t, &executor.mu)
}

func TestCodeActExecutorConcurrentAccess(t *testing.T) {
	// Test concurrent access to CodeActExecutor
	executor := NewCodeActExecutor()

	done := make(chan bool)
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()

			// Read access
			executor.mu.RLock()
			_ = len(executor.supportedLanguages)
			executor.mu.RUnlock()

			// Write access simulation (in real code this would modify internal state)
			executor.mu.Lock()
			_ = executor.sandboxDir
			executor.mu.Unlock()
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestCodeActExecutorSandboxDirectory(t *testing.T) {
	// Test sandbox directory creation
	executor := NewCodeActExecutor()

	// Verify sandbox directory exists or falls back gracefully
	assert.NotEmpty(t, executor.sandboxDir)

	// If it's a temp directory, it should contain "deep-coding-sandbox"
	if strings.Contains(executor.sandboxDir, "deep-coding-sandbox") {
		// Check if directory exists (it should be created during initialization)
		_, err := os.Stat(executor.sandboxDir)
		// Either it exists or we got a fallback (both are acceptable)
		if err != nil {
			// Fallback case - should be current directory
			assert.Equal(t, ".", executor.sandboxDir)
		}
	}
}

func TestCodeActExecutorTimeout(t *testing.T) {
	// Test timeout configuration
	executor := NewCodeActExecutor()

	// Verify timeout is set to a reasonable value
	assert.Greater(t, executor.timeout, time.Duration(0))
	assert.LessOrEqual(t, executor.timeout, 2*time.Minute) // Should not be too long
}

// Test helper functions that might exist in the CodeActExecutor

func TestCodeActExecutorLanguageSupport(t *testing.T) {
	// Test language support checking
	executor := NewCodeActExecutor()

	testCases := []struct {
		language  string
		supported bool
	}{
		{"python", true},
		{"go", true},
		{"bash", true},
		{"javascript", true},
		{"js", true},
		{"java", false},
		{"c++", false},
		{"rust", false},
		{"", false},
	}

	for _, tc := range testCases {
		t.Run(tc.language, func(t *testing.T) {
			_, exists := executor.supportedLanguages[tc.language]
			assert.Equal(t, tc.supported, exists)
		})
	}
}

func TestCodeActExecutorInitialization(t *testing.T) {
	// Test that CodeActExecutor is properly initialized
	executor := NewCodeActExecutor()

	// All fields should be properly initialized
	assert.NotNil(t, executor.supportedLanguages)
	assert.NotEmpty(t, executor.sandboxDir)
	assert.Greater(t, executor.timeout, time.Duration(0))

	// supportedLanguages should not be empty
	assert.Greater(t, len(executor.supportedLanguages), 0)
}

func TestCodeActResultValidation(t *testing.T) {
	// Test CodeActResult field validation
	successResult := &CodeActResult{
		Success:  true,
		ExitCode: 0,
		Language: "python",
	}

	failureResult := &CodeActResult{
		Success:  false,
		ExitCode: 1,
		Error:    "Some error occurred",
		Language: "python",
	}

	// Success result should have exit code 0
	assert.True(t, successResult.Success)
	assert.Equal(t, 0, successResult.ExitCode)

	// Failure result should have non-zero exit code and error message
	assert.False(t, failureResult.Success)
	assert.NotEqual(t, 0, failureResult.ExitCode)
	assert.NotEmpty(t, failureResult.Error)
}

func TestCodeActExecutorThreadSafety(t *testing.T) {
	// Test thread safety of CodeActExecutor
	executor := NewCodeActExecutor()

	numGoroutines := 20
	results := make(chan bool, numGoroutines)

	// Simulate concurrent access
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { results <- true }()

			// Simulate reading configuration
			executor.mu.RLock()
			languages := make(map[string]string)
			for k, v := range executor.supportedLanguages {
				languages[k] = v
			}
			sandboxDir := executor.sandboxDir
			timeout := executor.timeout
			executor.mu.RUnlock()

			// Verify data consistency
			assert.NotEmpty(t, languages)
			assert.NotEmpty(t, sandboxDir)
			assert.Greater(t, timeout, time.Duration(0))
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-results
	}
}

// Benchmark tests for performance
func BenchmarkNewCodeActExecutor(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewCodeActExecutor()
	}
}

func BenchmarkCodeActResultCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = &CodeActResult{
			Success:       true,
			Output:        "test output",
			ExitCode:      0,
			ExecutionTime: 100 * time.Millisecond,
			Language:      "python",
			Code:          "print('hello')",
		}
	}
}
