package toolregistry

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
)

// validatingExecutor validates tool call arguments against the tool's
// ParameterSchema before delegating to the inner executor. Checks required
// fields and basic type matching. Lenient: accepts float64 for integer
// (JSON numbers), allows extra fields not in schema.
type validatingExecutor struct {
	delegate tools.ToolExecutor
}

func (v *validatingExecutor) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if v.delegate == nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("tool executor missing")}, nil
	}

	schema := v.delegate.Definition().Parameters
	if err := validateArguments(schema, call.Arguments); err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Invalid arguments: %v", err),
			Error:   fmt.Errorf("argument validation: %w", err),
		}, nil
	}

	return v.delegate.Execute(ctx, call)
}

func (v *validatingExecutor) Definition() ports.ToolDefinition {
	return v.delegate.Definition()
}

func (v *validatingExecutor) Metadata() ports.ToolMetadata {
	return v.delegate.Metadata()
}

func validateArguments(schema ports.ParameterSchema, args map[string]any) error {
	if len(schema.Properties) == 0 {
		return nil
	}

	// Check required fields.
	for _, req := range schema.Required {
		val, ok := args[req]
		if !ok || val == nil {
			return fmt.Errorf("missing required argument %q", req)
		}
	}

	// Check type for each provided argument that has a schema entry.
	for key, val := range args {
		prop, ok := schema.Properties[key]
		if !ok {
			continue // extra fields allowed
		}
		if val == nil {
			continue // nil values skip type check
		}
		if err := checkType(key, prop.Type, val); err != nil {
			return err
		}
	}

	return nil
}

func checkType(key, expectedType string, val any) error {
	if expectedType == "" {
		return nil
	}

	switch strings.ToLower(expectedType) {
	case "string":
		if _, ok := val.(string); !ok {
			return fmt.Errorf("argument %q: expected string, got %T", key, val)
		}
	case "number", "integer":
		switch val.(type) {
		case float64, int, int64, float32:
			// OK — JSON numbers unmarshal as float64
		default:
			return fmt.Errorf("argument %q: expected number, got %T", key, val)
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("argument %q: expected boolean, got %T", key, val)
		}
	case "array":
		if _, ok := val.([]any); !ok {
			return fmt.Errorf("argument %q: expected array, got %T", key, val)
		}
	case "object":
		if _, ok := val.(map[string]any); !ok {
			return fmt.Errorf("argument %q: expected object, got %T", key, val)
		}
	}

	return nil
}

var _ tools.ToolExecutor = (*validatingExecutor)(nil)
