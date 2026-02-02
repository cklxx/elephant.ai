package toolregistry

import (
	"context"
	"fmt"

	ports "alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
)

// ToolLookup resolves a tool executor by name. It returns false when the
// requested tool is not available.
type ToolLookup func(name string) (tools.ToolExecutor, bool)

// DegradationConfig controls the fallback/degradation behaviour of
// degradationExecutor.
type DegradationConfig struct {
	// FallbackMap maps a tool name to an ordered list of fallback tool names.
	// When the primary tool fails, the executor walks this list in order and
	// attempts each alternative until one succeeds.
	FallbackMap map[string][]string

	// EnableUserPrompt, when true, causes the executor to return a structured
	// user-prompt result as a last resort after all fallbacks have been
	// exhausted.
	EnableUserPrompt bool

	// MaxFallbackAttempts caps how many fallback tools will be tried. A value
	// of 0 means "use the default" (2).
	MaxFallbackAttempts int
}

const defaultMaxFallbackAttempts = 2

// DefaultDegradationConfig returns a sensible zero-value config with an empty
// fallback map, user-prompt disabled, and the default attempt cap.
func DefaultDegradationConfig() DegradationConfig {
	return DegradationConfig{
		FallbackMap:         make(map[string][]string),
		EnableUserPrompt:    false,
		MaxFallbackAttempts: defaultMaxFallbackAttempts,
	}
}

// degradationExecutor is a ToolExecutor decorator that attempts fallback
// strategies when the primary execution fails.
//
// Fallback order:
//  1. Walk FallbackMap entries for the failing tool (up to MaxFallbackAttempts).
//  2. If EnableUserPrompt is true, return a structured prompt asking the user
//     to supply the result manually.
//  3. Otherwise, return the original error from the primary tool.
type degradationExecutor struct {
	delegate tools.ToolExecutor
	lookup   ToolLookup
	config   DegradationConfig
}

// NewDegradationExecutor wraps delegate with automatic degradation logic.
// lookup is used to resolve fallback tool names at execution time.
func NewDegradationExecutor(delegate tools.ToolExecutor, lookup ToolLookup, config DegradationConfig) tools.ToolExecutor {
	if config.MaxFallbackAttempts <= 0 {
		config.MaxFallbackAttempts = defaultMaxFallbackAttempts
	}
	if config.FallbackMap == nil {
		config.FallbackMap = make(map[string][]string)
	}
	return &degradationExecutor{
		delegate: delegate,
		lookup:   lookup,
		config:   config,
	}
}

func (d *degradationExecutor) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	// Attempt the primary execution.
	result, err := d.delegate.Execute(ctx, call)
	if err == nil && (result == nil || result.Error == nil) {
		return result, nil
	}

	// Capture the original error for later reporting.
	originalErr := err
	if originalErr == nil && result != nil {
		originalErr = result.Error
	}

	toolName := d.delegate.Metadata().Name

	// --- fallback chain ---
	fallbacks, hasFallbacks := d.config.FallbackMap[toolName]
	if hasFallbacks {
		limit := d.config.MaxFallbackAttempts
		if limit > len(fallbacks) {
			limit = len(fallbacks)
		}
		for i := 0; i < limit; i++ {
			if ctx.Err() != nil {
				break
			}
			fbName := fallbacks[i]
			fbExecutor, ok := d.lookup(fbName)
			if !ok {
				continue
			}
			fbResult, fbErr := fbExecutor.Execute(ctx, call)
			if fbErr == nil && (fbResult == nil || fbResult.Error == nil) {
				// Fallback succeeded -- annotate the result.
				if fbResult == nil {
					fbResult = &ports.ToolResult{CallID: call.ID}
				}
				if fbResult.Metadata == nil {
					fbResult.Metadata = make(map[string]any)
				}
				fbResult.Metadata["degraded_from"] = toolName
				fbResult.Metadata["degraded_to"] = fbName
				return fbResult, nil
			}
		}
	}

	// --- user prompt fallback ---
	if d.config.EnableUserPrompt {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("[degradation] Tool %q failed. Please provide the expected result manually.", toolName),
			Metadata: map[string]any{
				"degraded_from": toolName,
				"user_prompt":   true,
			},
		}, nil
	}

	// --- no fallbacks available -- return original error ---
	if result != nil {
		return result, originalErr
	}
	return &ports.ToolResult{CallID: call.ID, Error: originalErr}, originalErr
}

func (d *degradationExecutor) Definition() ports.ToolDefinition {
	return d.delegate.Definition()
}

func (d *degradationExecutor) Metadata() ports.ToolMetadata {
	return d.delegate.Metadata()
}

// Compile-time interface check.
var _ tools.ToolExecutor = (*degradationExecutor)(nil)
