package builtin

import (
	"alex/internal/agent/ports"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCodeExecute_Python(t *testing.T) {
	tool := NewCodeExecute(CodeExecuteConfig{})

	tests := []struct {
		name        string
		code        string
		expectError bool
		contains    string
	}{
		{
			name:     "simple print",
			code:     "print('Hello, World!')",
			contains: "Hello, World!",
		},
		{
			name:     "arithmetic",
			code:     "result = 2 + 2\nprint(f'Result: {result}')",
			contains: "Result: 4",
		},
		{
			name:        "syntax error",
			code:        "print('unclosed string",
			expectError: true,
			contains:    "SyntaxError",
		},
		{
			name:     "import standard library",
			code:     "import math\nprint(math.pi)",
			contains: "3.14159",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), ports.ToolCall{
				ID:   "test-1",
				Name: "code_execute",
				Arguments: map[string]any{
					"language": "python",
					"code":     tt.code,
				},
			})

			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if tt.expectError {
				// Check metadata for success = false
				if success, ok := result.Metadata["success"].(bool); !ok || success {
					t.Error("expected execution failure but got success")
				}
				if !strings.Contains(result.Content, tt.contains) {
					t.Errorf("expected content to contain %q, got %q", tt.contains, result.Content)
				}
			} else {
				// Check metadata for success = true
				if success, ok := result.Metadata["success"].(bool); !ok || !success {
					t.Errorf("unexpected execution failure: %v", result.Content)
				}
				if !strings.Contains(result.Content, tt.contains) {
					t.Errorf("expected content to contain %q, got %q", tt.contains, result.Content)
				}
			}
		})
	}
}

func TestCodeExecute_Go(t *testing.T) {
	tool := NewCodeExecute(CodeExecuteConfig{})

	tests := []struct {
		name        string
		code        string
		expectError bool
		contains    string
	}{
		{
			name: "simple program",
			code: `package main
import "fmt"
func main() {
	fmt.Println("Hello from Go!")
}`,
			contains: "Hello from Go!",
		},
		{
			name: "with math",
			code: `package main
import "fmt"
func main() {
	result := 10 * 5
	fmt.Printf("Result: %d\n", result)
}`,
			contains: "Result: 50",
		},
		{
			name: "syntax error",
			code: `package main
func main() {
	fmt.Println("missing import")
}`,
			expectError: true,
			contains:    "undefined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), ports.ToolCall{
				ID:   "test-1",
				Name: "code_execute",
				Arguments: map[string]any{
					"language": "go",
					"code":     tt.code,
				},
			})

			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if tt.expectError {
				// Check metadata for success = false
				if success, ok := result.Metadata["success"].(bool); !ok || success {
					t.Error("expected execution failure but got success")
				}
				if !strings.Contains(result.Content, tt.contains) {
					t.Errorf("expected content to contain %q, got %q", tt.contains, result.Content)
				}
			} else {
				// Check metadata for success = true
				if success, ok := result.Metadata["success"].(bool); !ok || !success {
					t.Errorf("unexpected execution failure: %v", result.Content)
				}
				if !strings.Contains(result.Content, tt.contains) {
					t.Errorf("expected content to contain %q, got %q", tt.contains, result.Content)
				}
			}
		})
	}
}

func TestCodeExecute_JavaScript(t *testing.T) {
	tool := NewCodeExecute(CodeExecuteConfig{})

	tests := []struct {
		name        string
		code        string
		expectError bool
		contains    string
	}{
		{
			name:     "simple console.log",
			code:     "console.log('Hello from Node.js!');",
			contains: "Hello from Node.js!",
		},
		{
			name:     "arithmetic",
			code:     "const result = 3 * 7;\nconsole.log(`Result: ${result}`);",
			contains: "Result: 21",
		},
		{
			name:        "reference error",
			code:        "console.log(undefinedVariable);",
			expectError: true,
			contains:    "ReferenceError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), ports.ToolCall{
				ID:   "test-1",
				Name: "code_execute",
				Arguments: map[string]any{
					"language": "javascript",
					"code":     tt.code,
				},
			})

			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if tt.expectError {
				// Check metadata for success = false
				if success, ok := result.Metadata["success"].(bool); !ok || success {
					t.Error("expected execution failure but got success")
				}
				if !strings.Contains(result.Content, tt.contains) {
					t.Errorf("expected content to contain %q, got %q", tt.contains, result.Content)
				}
			} else {
				// Check metadata for success = true
				if success, ok := result.Metadata["success"].(bool); !ok || !success {
					t.Errorf("unexpected execution failure: %v", result.Content)
				}
				if !strings.Contains(result.Content, tt.contains) {
					t.Errorf("expected content to contain %q, got %q", tt.contains, result.Content)
				}
			}
		})
	}
}

func TestCodeExecute_CodePath(t *testing.T) {
	tool := NewCodeExecute(CodeExecuteConfig{})

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "hello.py")
	if err := os.WriteFile(scriptPath, []byte("print('hi from file')"), 0644); err != nil {
		t.Fatalf("failed to write temp script: %v", err)
	}

	ctx := WithWorkingDir(context.Background(), tmpDir)
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID:   "test-code-path",
		Name: "code_execute",
		Arguments: map[string]any{
			"language":  "python",
			"code_path": scriptPath,
		},
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if success, ok := result.Metadata["success"].(bool); !ok || !success {
		t.Fatalf("expected success, got metadata %v", result.Metadata)
	}
	if !strings.Contains(result.Content, "hi from file") {
		t.Fatalf("expected output to contain script contents, got %q", result.Content)
	}
	if got := result.Metadata["code_path"]; got != scriptPath {
		t.Fatalf("expected metadata.code_path %q, got %v", scriptPath, got)
	}
}

func TestCodeExecute_DataURI(t *testing.T) {
	tool := NewCodeExecute(CodeExecuteConfig{})
	payload := base64.StdEncoding.EncodeToString([]byte("print('from data uri')"))
	dataURI := "data:text/plain;base64," + payload

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "test-data-uri",
		Name: "code_execute",
		Arguments: map[string]any{
			"language": "python",
			"code":     dataURI,
		},
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if success, ok := result.Metadata["success"].(bool); !ok || !success {
		t.Fatalf("expected success, got metadata %v", result.Metadata)
	}
	if !strings.Contains(result.Content, "from data uri") {
		t.Fatalf("expected decoded output, got %q", result.Content)
	}
	if prov, _ := result.Metadata["code_provenance"].(string); prov != "attachment" {
		t.Fatalf("expected metadata.code_provenance to be attachment, got %v", prov)
	}
}

func TestCodeExecute_Bash(t *testing.T) {
	tool := NewCodeExecute(CodeExecuteConfig{})

	tests := []struct {
		name        string
		code        string
		expectError bool
		contains    string
	}{
		{
			name:     "simple echo",
			code:     "echo 'Hello from Bash!'",
			contains: "Hello from Bash!",
		},
		{
			name:     "multiple commands",
			code:     "X=10\nY=20\necho $((X + Y))",
			contains: "30",
		},
		{
			name:        "command not found",
			code:        "nonexistentcommand",
			expectError: true,
			contains:    "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), ports.ToolCall{
				ID:   "test-1",
				Name: "code_execute",
				Arguments: map[string]any{
					"language": "bash",
					"code":     tt.code,
				},
			})

			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if tt.expectError {
				// Check metadata for success = false
				if success, ok := result.Metadata["success"].(bool); !ok || success {
					t.Error("expected execution failure but got success")
				}
				if !strings.Contains(result.Content, tt.contains) {
					t.Errorf("expected content to contain %q, got %q", tt.contains, result.Content)
				}
			} else {
				// Check metadata for success = true
				if success, ok := result.Metadata["success"].(bool); !ok || !success {
					t.Errorf("unexpected execution failure: %v", result.Content)
				}
				if !strings.Contains(result.Content, tt.contains) {
					t.Errorf("expected content to contain %q, got %q", tt.contains, result.Content)
				}
			}
		})
	}
}

func TestCodeExecute_Timeout(t *testing.T) {
	tool := NewCodeExecute(CodeExecuteConfig{})

	// Python infinite loop with short timeout
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "test-1",
		Name: "code_execute",
		Arguments: map[string]any{
			"language": "python",
			"code":     "import time\nwhile True:\n    time.sleep(0.1)",
			"timeout":  1.0,
		},
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Check metadata for success = false
	if success, ok := result.Metadata["success"].(bool); !ok || success {
		t.Error("expected execution failure but got success")
	}

	if !strings.Contains(result.Content, "timed out") {
		t.Errorf("expected content to contain 'timed out', got %q", result.Content)
	}
}

func TestCodeExecute_MissingParameters(t *testing.T) {
	tool := NewCodeExecute(CodeExecuteConfig{})

	tests := []struct {
		name      string
		arguments map[string]any
		wantError bool
	}{
		{
			name: "missing language",
			arguments: map[string]any{
				"code": "print('test')",
			},
			wantError: true,
		},
		{
			name: "missing code",
			arguments: map[string]any{
				"language": "python",
			},
			wantError: true,
		},
		{
			name: "empty language",
			arguments: map[string]any{
				"language": "",
				"code":     "print('test')",
			},
			wantError: true,
		},
		{
			name: "empty code",
			arguments: map[string]any{
				"language": "python",
				"code":     "",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), ports.ToolCall{
				ID:        "test-1",
				Name:      "code_execute",
				Arguments: tt.arguments,
			})

			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if tt.wantError && result.Error == nil {
				t.Error("expected error in result but got none")
			}
		})
	}
}

func TestCodeExecute_UnsupportedLanguage(t *testing.T) {
	tool := NewCodeExecute(CodeExecuteConfig{})

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "test-1",
		Name: "code_execute",
		Arguments: map[string]any{
			"language": "ruby",
			"code":     "puts 'Hello'",
		},
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Error == nil {
		t.Error("expected error for unsupported language")
	}

	if result.Content != "" {
		t.Logf("Content: %q", result.Content)
	}
}

func TestCodeExecute_TimeoutBounds(t *testing.T) {
	tool := NewCodeExecute(CodeExecuteConfig{})

	tests := []struct {
		name            string
		timeout         float64
		expectedBounded bool
	}{
		{
			name:            "timeout too large",
			timeout:         1000.0,
			expectedBounded: true, // Should be capped at 300
		},
		{
			name:            "timeout too small",
			timeout:         0.1,
			expectedBounded: true, // Should be at least 1
		},
		{
			name:            "valid timeout",
			timeout:         30.0,
			expectedBounded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := tool.Execute(ctx, ports.ToolCall{
				ID:   "test-1",
				Name: "code_execute",
				Arguments: map[string]any{
					"language": "python",
					"code":     "print('test')",
					"timeout":  tt.timeout,
				},
			})

			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			// Check metadata for execution time
			if result.Metadata != nil {
				execTime, ok := result.Metadata["execution_time"].(int64)
				if ok && execTime > 300000 { // 300 seconds in milliseconds
					t.Error("execution time exceeded max timeout bound")
				}
			}
		})
	}
}

func TestCodeExecute_Definition(t *testing.T) {
	tool := NewCodeExecute(CodeExecuteConfig{})
	def := tool.Definition()

	if def.Name != "code_execute" {
		t.Errorf("expected name 'code_execute', got %q", def.Name)
	}

	if def.Description == "" {
		t.Error("expected non-empty description")
	}

	if def.Parameters.Type != "object" {
		t.Errorf("expected parameters type 'object', got %q", def.Parameters.Type)
	}

	requiredParams := []string{"language", "code"}
	if len(def.Parameters.Required) != len(requiredParams) {
		t.Errorf("expected %d required parameters, got %d", len(requiredParams), len(def.Parameters.Required))
	}

	for _, param := range requiredParams {
		found := false
		for _, req := range def.Parameters.Required {
			if req == param {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected required parameter %q not found", param)
		}
	}
}

func TestCodeExecute_Metadata(t *testing.T) {
	tool := NewCodeExecute(CodeExecuteConfig{})
	meta := tool.Metadata()

	if meta.Name != "code_execute" {
		t.Errorf("expected name 'code_execute', got %q", meta.Name)
	}

	if meta.Version == "" {
		t.Error("expected non-empty version")
	}

	if meta.Category != "execution" {
		t.Errorf("expected category 'execution', got %q", meta.Category)
	}

	if !meta.Dangerous {
		t.Error("expected tool to be marked as dangerous")
	}

	if len(meta.Tags) == 0 {
		t.Error("expected non-empty tags")
	}
}

func TestCodeExecute_ConcurrentExecution(t *testing.T) {
	tool := NewCodeExecute(CodeExecuteConfig{})

	// Run multiple code executions concurrently
	const numGoroutines = 5
	results := make(chan *ports.ToolResult, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			result, err := tool.Execute(context.Background(), ports.ToolCall{
				ID:   "test-1",
				Name: "code_execute",
				Arguments: map[string]any{
					"language": "python",
					"code":     fmt.Sprintf("print('Concurrent execution %d')", id),
				},
			})
			results <- result
			errors <- err
		}(i)
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		err := <-errors
		if err != nil {
			t.Errorf("goroutine %d failed: %v", i, err)
		}

		result := <-results
		if success, ok := result.Metadata["success"].(bool); !ok || !success {
			t.Errorf("goroutine %d execution failed: %v", i, result.Content)
		}
	}
}

func TestCodeExecute_ContextCancellation(t *testing.T) {
	tool := NewCodeExecute(CodeExecuteConfig{})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID:   "test-1",
		Name: "code_execute",
		Arguments: map[string]any{
			"language": "python",
			"code":     "import time\ntime.sleep(10)",
		},
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Check metadata for success = false
	if success, ok := result.Metadata["success"].(bool); !ok || success {
		t.Error("expected execution failure but got success")
	}
}
