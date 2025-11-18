# Tool Development Guide
> Last updated: 2025-11-18


## Overview

This guide covers how to develop custom tools for the Deep Coding Agent. Tools are the primary way the agent interacts with the external world, from file operations to command execution.

## Tool Architecture

### Core Components

1. **Tool Interface**: All tools must implement the `Tool` interface
2. **Parameter Schema**: JSON Schema defining tool parameters
3. **Validation**: Input validation and security checks
4. **Execution**: The core tool functionality
5. **Result**: Structured output with metadata

### Tool Interface

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]interface{}
    Validate(args map[string]interface{}) error
    Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error)
}
```

## Creating a Custom Tool

### Step 1: Define the Tool Structure

```go
type MyCustomTool struct {
    // Tool-specific fields
    configManager *config.Manager
    cache         *cache.Cache
}

func NewMyCustomTool(configManager *config.Manager) *MyCustomTool {
    return &MyCustomTool{
        configManager: configManager,
        cache:         cache.New(),
    }
}
```

### Step 2: Implement Required Methods

#### Name Method
```go
func (t *MyCustomTool) Name() string {
    return "my_custom_tool"
}
```

#### Description Method
```go
func (t *MyCustomTool) Description() string {
    return "Performs custom operations with specific functionality"
}
```

#### Parameters Method
```go
func (t *MyCustomTool) Parameters() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "input_data": map[string]interface{}{
                "type":        "string",
                "description": "Input data to process",
            },
            "options": map[string]interface{}{
                "type":        "object",
                "description": "Processing options",
                "properties": map[string]interface{}{
                    "format": map[string]interface{}{
                        "type": "string",
                        "enum": []string{"json", "text", "xml"},
                        "default": "json",
                    },
                    "validate": map[string]interface{}{
                        "type": "boolean",
                        "default": true,
                    },
                },
            },
        },
        "required": []string{"input_data"},
    }
}
```

#### Validation Method
```go
func (t *MyCustomTool) Validate(args map[string]interface{}) error {
    // Check required parameters
    inputData, ok := args["input_data"].(string)
    if !ok || inputData == "" {
        return fmt.Errorf("input_data is required and must be a non-empty string")
    }

    // Validate options if provided
    if options, ok := args["options"]; ok {
        if optionsMap, ok := options.(map[string]interface{}); ok {
            if format, ok := optionsMap["format"]; ok {
                if formatStr, ok := format.(string); ok {
                    validFormats := []string{"json", "text", "xml"}
                    valid := false
                    for _, vf := range validFormats {
                        if formatStr == vf {
                            valid = true
                            break
                        }
                    }
                    if !valid {
                        return fmt.Errorf("invalid format: %s", formatStr)
                    }
                }
            }
        }
    }

    return nil
}
```

#### Execute Method
```go
func (t *MyCustomTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
    inputData := args["input_data"].(string)
    
    // Parse options
    format := "json"
    validate := true
    
    if options, ok := args["options"].(map[string]interface{}); ok {
        if f, ok := options["format"].(string); ok {
            format = f
        }
        if v, ok := options["validate"].(bool); ok {
            validate = v
        }
    }

    // Perform validation if requested
    if validate {
        if err := t.validateInputData(inputData); err != nil {
            return nil, fmt.Errorf("input validation failed: %w", err)
        }
    }

    // Process the data
    result, err := t.processData(ctx, inputData, format)
    if err != nil {
        return nil, fmt.Errorf("processing failed: %w", err)
    }

    // Return structured result
    return &ToolResult{
        Content: result.Summary,
        Data: map[string]interface{}{
            "format":      format,
            "input_size":  len(inputData),
            "output_size": len(result.Output),
            "validated":   validate,
            "processing_time": result.Duration.Milliseconds(),
        },
        Files: result.GeneratedFiles,
    }, nil
}
```

### Step 3: Implement Helper Methods

```go
func (t *MyCustomTool) validateInputData(data string) error {
    // Implement validation logic
    if len(data) > 1000000 { // 1MB limit
        return fmt.Errorf("input data too large (max 1MB)")
    }
    
    // Additional validation rules
    return nil
}

func (t *MyCustomTool) processData(ctx context.Context, data, format string) (*ProcessResult, error) {
    startTime := time.Now()
    
    // Check for context cancellation
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // Implement core processing logic
    var output string
    var files []string
    
    switch format {
    case "json":
        output, files, err = t.processAsJSON(data)
    case "text":
        output, files, err = t.processAsText(data)
    case "xml":
        output, files, err = t.processAsXML(data)
    default:
        return nil, fmt.Errorf("unsupported format: %s", format)
    }
    
    if err != nil {
        return nil, err
    }

    return &ProcessResult{
        Output:         output,
        GeneratedFiles: files,
        Duration:       time.Since(startTime),
        Summary:        fmt.Sprintf("Processed %d bytes as %s", len(data), format),
    }, nil
}

type ProcessResult struct {
    Output         string
    GeneratedFiles []string
    Duration       time.Duration
    Summary        string
}
```

## Security Considerations

### Input Validation

Always validate all inputs thoroughly:

```go
func (t *MyCustomTool) validateSecurely(args map[string]interface{}) error {
    // 1. Check parameter types
    // 2. Validate ranges and limits
    // 3. Sanitize string inputs
    // 4. Check for injection attacks
    // 5. Verify file paths are safe
    return nil
}
```

### Path Sanitization

```go
func sanitizePath(path string) (string, error) {
    // Clean the path
    cleanPath := filepath.Clean(path)
    
    // Check for path traversal
    if strings.Contains(cleanPath, "..") {
        return "", fmt.Errorf("path traversal not allowed")
    }
    
    // Ensure absolute path
    absPath, err := filepath.Abs(cleanPath)
    if err != nil {
        return "", fmt.Errorf("invalid path: %w", err)
    }
    
    return absPath, nil
}
```

### Resource Limits

```go
func (t *MyCustomTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
    // Set execution timeout
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    // Limit memory usage
    if err := t.checkMemoryUsage(); err != nil {
        return nil, err
    }
    
    // Continue with execution...
}
```

## Error Handling

### Error Types

Define specific error types for better error handling:

```go
type ToolError struct {
    Type    string `json:"type"`
    Message string `json:"message"`
    Code    string `json:"code"`
    Details map[string]interface{} `json:"details,omitempty"`
}

func (e *ToolError) Error() string {
    return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Error constructors
func NewValidationError(message string, details map[string]interface{}) *ToolError {
    return &ToolError{
        Type:    "ValidationError",
        Message: message,
        Code:    "VALIDATION_FAILED",
        Details: details,
    }
}

func NewExecutionError(message string, cause error) *ToolError {
    details := map[string]interface{}{}
    if cause != nil {
        details["cause"] = cause.Error()
    }
    return &ToolError{
        Type:    "ExecutionError",
        Message: message,
        Code:    "EXECUTION_FAILED",
        Details: details,
    }
}
```

## Testing Tools

### Unit Tests

```go
func TestMyCustomTool_Execute(t *testing.T) {
    tool := NewMyCustomTool(nil)
    
    tests := []struct {
        name    string
        args    map[string]interface{}
        want    *ToolResult
        wantErr bool
    }{
        {
            name: "valid input",
            args: map[string]interface{}{
                "input_data": "test data",
                "options": map[string]interface{}{
                    "format": "json",
                },
            },
            want: &ToolResult{
                Content: "Processed 9 bytes as json",
                Data: map[string]interface{}{
                    "format":     "json",
                    "input_size": 9,
                },
            },
            wantErr: false,
        },
        {
            name: "invalid format",
            args: map[string]interface{}{
                "input_data": "test data",
                "options": map[string]interface{}{
                    "format": "invalid",
                },
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := tool.Execute(context.Background(), tt.args)
            if (err != nil) != tt.wantErr {
                t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !tt.wantErr && !reflect.DeepEqual(got.Data["format"], tt.want.Data["format"]) {
                t.Errorf("Execute() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Integration Tests

```go
func TestMyCustomTool_Integration(t *testing.T) {
    // Test with real configuration
    configManager := config.NewManager()
    tool := NewMyCustomTool(configManager)
    
    // Test with realistic inputs
    args := map[string]interface{}{
        "input_data": generateLargeTestData(),
        "options": map[string]interface{}{
            "format":   "json",
            "validate": true,
        },
    }
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    result, err := tool.Execute(ctx, args)
    assert.NoError(t, err)
    assert.NotNil(t, result)
    assert.Greater(t, len(result.Content), 0)
}
```

## Tool Registration

### Adding to Registry

```go
// In internal/tools/registry.go
func (r *Registry) RegisterBuiltinTools() {
    // Register your custom tool
    r.Register(NewMyCustomTool(r.configManager))
}
```

### Configuration

Add tool configuration options:

```go
// In pkg/types/config.go
type ToolConfig struct {
    MyCustomTool *MyCustomToolConfig `json:"myCustomTool,omitempty"`
}

type MyCustomToolConfig struct {
    Enabled     bool   `json:"enabled"`
    MaxSize     int    `json:"maxSize"`
    DefaultFormat string `json:"defaultFormat"`
}
```

## Best Practices

### Performance

1. **Caching**: Cache expensive operations
2. **Streaming**: Use streaming for large data
3. **Concurrency**: Support concurrent execution where safe
4. **Resource Management**: Clean up resources properly

### Usability

1. **Clear Documentation**: Provide clear parameter descriptions
2. **Meaningful Errors**: Return actionable error messages
3. **Progress Feedback**: For long operations, provide progress updates
4. **Sensible Defaults**: Use reasonable default values

### Maintainability

1. **Modular Design**: Break complex tools into smaller components
2. **Interface Segregation**: Use specific interfaces for dependencies
3. **Configuration**: Make behavior configurable
4. **Logging**: Add appropriate logging for debugging

## Examples

### File Processing Tool

See `internal/tools/builtin/file_tools.go` for examples of:
- File system operations
- Path validation
- Content processing
- Error handling

### Command Execution Tool

See `internal/tools/builtin/bash_tools.go` for examples of:
- Command validation
- Security checks
- Process management
- Output handling

---

*For more examples and detailed implementation patterns, refer to the existing tools in `internal/tools/builtin/`.*
