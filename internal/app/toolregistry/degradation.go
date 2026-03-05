package toolregistry

import (
	"context"
	"fmt"
	"strings"

	ports "alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	toolspolicy "alex/internal/infra/tools"
)

// ToolLookup resolves a tool executor by name. It returns false when the
// requested tool is not available.
type ToolLookup func(name string) (tools.ToolExecutor, bool)

// DegradationSLARouter provides SLA-based tool ordering and health checks for
// degradation routing decisions.
type DegradationSLARouter interface {
	RankTools(toolNames []string) []toolspolicy.ToolSLAProfile
	IsHealthy(toolName string) bool
}

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

	// SLARouter, when set, is used to rank fallback candidates by tool health.
	SLARouter DegradationSLARouter

	// PreRouteWhenPrimaryUnhealthy enables pre-routing to fallback candidates
	// before attempting the primary tool when SLA indicates the primary is
	// unhealthy.
	PreRouteWhenPrimaryUnhealthy bool

	// ArgumentAdapter transforms the original arguments before passing them to
	// a fallback tool. This allows mapping parameters between tools with
	// different schemas. When nil, arguments are passed through unmodified.
	ArgumentAdapter func(fromTool, toTool string, args map[string]any) map[string]any
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
	toolName := strings.TrimSpace(d.delegate.Metadata().Name)
	if toolName == "" {
		toolName = strings.TrimSpace(call.Name)
	}
	fallbacks := d.orderedFallbacks(toolName)
	fallbackTried := false

	if d.shouldPreRoute(toolName, fallbacks) {
		if fbResult, ok := d.tryFallbacks(ctx, call, toolName, fallbacks); ok {
			return fbResult, nil
		}
		fallbackTried = true
	}

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

	// --- fallback chain ---
	if !fallbackTried {
		if fbResult, ok := d.tryFallbacks(ctx, call, toolName, fallbacks); ok {
			return fbResult, nil
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

func (d *degradationExecutor) shouldPreRoute(toolName string, fallbacks []string) bool {
	if !d.config.PreRouteWhenPrimaryUnhealthy || d.config.SLARouter == nil || len(fallbacks) == 0 {
		return false
	}
	return !d.config.SLARouter.IsHealthy(toolName)
}

func (d *degradationExecutor) orderedFallbacks(toolName string) []string {
	raw, ok := d.config.FallbackMap[toolName]
	if !ok || len(raw) == 0 {
		return nil
	}

	candidates := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, name := range raw {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" || trimmed == toolName {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		candidates = append(candidates, trimmed)
	}
	if len(candidates) <= 1 || d.config.SLARouter == nil {
		return candidates
	}

	ranked := d.config.SLARouter.RankTools(candidates)
	if len(ranked) == 0 {
		return candidates
	}

	ordered := make([]string, 0, len(candidates))
	added := make(map[string]struct{}, len(candidates))
	for _, profile := range ranked {
		name := strings.TrimSpace(profile.ToolName)
		if _, allowed := seen[name]; !allowed {
			continue
		}
		if _, exists := added[name]; exists {
			continue
		}
		added[name] = struct{}{}
		ordered = append(ordered, name)
	}
	for _, name := range candidates {
		if _, exists := added[name]; exists {
			continue
		}
		ordered = append(ordered, name)
	}
	return ordered
}

func (d *degradationExecutor) tryFallbacks(ctx context.Context, call ports.ToolCall, toolName string, fallbacks []string) (*ports.ToolResult, bool) {
	if len(fallbacks) == 0 {
		return nil, false
	}
	limit := d.config.MaxFallbackAttempts
	if limit > len(fallbacks) {
		limit = len(fallbacks)
	}
	for i := 0; i < limit; i++ {
		if ctx.Err() != nil {
			break
		}
		fbName := fallbacks[i]
		if fbName == toolName {
			continue
		}
		fbExecutor, ok := d.lookup(fbName)
		if !ok || fbExecutor == nil {
			continue
		}
		fbCall := call
		fbCall.Name = fbName
		if d.config.ArgumentAdapter != nil {
			fbCall.Arguments = d.config.ArgumentAdapter(toolName, fbName, fbCall.Arguments)
		}
		fbResult, fbErr := fbExecutor.Execute(ctx, fbCall)
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
			return fbResult, true
		}
	}
	return nil, false
}

// Compile-time interface check.
var _ tools.ToolExecutor = (*degradationExecutor)(nil)
