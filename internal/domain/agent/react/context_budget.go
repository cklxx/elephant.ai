package react

import (
	"encoding/binary"
	"fmt"
	"hash"
	"hash/fnv"
	"sort"
	"strings"

	"alex/internal/domain/agent/ports"
	jsonx "alex/internal/shared/json"
	"alex/internal/shared/modelregistry"
	tokenutil "alex/internal/shared/token"
)

const (
	defaultModelContextWindowTokens = 128000
	gpt5ContextWindowTokens         = 256000
	claudeContextWindowTokens       = 200000
	minContextBudgetTokens          = 4096
	// Reserve a fixed margin for request framing and provider-side formatting.
	contextBudgetRequestSafetyTokens = 1024
	// Keep at least one token for message budget so trim logic can still run.
	minMessageBudgetTokens = 1
)

type contextBudgetSplit struct {
	TotalLimit   int
	MessageLimit int
	ToolTokens   int
}

func (e *ReactEngine) resolveContextTokenLimit(services Services) int {
	if e.completion.contextTokenLimit > 0 {
		return e.completion.contextTokenLimit
	}

	model := ""
	if services.LLM != nil {
		model = services.LLM.Model()
	}
	return deriveContextTokenLimit(model, e.completion.maxTokens)
}

func deriveContextTokenLimit(model string, maxOutputTokens int) int {
	window := modelContextWindowTokens(model)

	reservedForOutput := maxOutputTokens
	if reservedForOutput < 2048 {
		reservedForOutput = 2048
	}

	// Keep a small fixed buffer to reduce edge-case overflows from framing/tool metadata.
	safetyBuffer := window / 100
	if safetyBuffer < 1024 {
		safetyBuffer = 1024
	}

	limit := window - reservedForOutput - safetyBuffer
	if limit < minContextBudgetTokens {
		limit = minContextBudgetTokens
	}
	if limit > window {
		limit = window
	}
	return limit
}

func modelContextWindowTokens(model string) int {
	if info, ok := modelregistry.Lookup(model); ok && info.ContextWindow > 0 {
		return info.ContextWindow
	}

	m := strings.ToLower(strings.TrimSpace(model))
	switch {
	case m == "":
		return defaultModelContextWindowTokens
	case strings.HasPrefix(m, "gpt-5"),
		strings.Contains(m, "gpt-5.2-codex"),
		strings.Contains(m, "gpt-5.3-codex"),
		strings.Contains(m, "codex-spark"):
		return gpt5ContextWindowTokens
	case strings.Contains(m, "claude"):
		return claudeContextWindowTokens
	case strings.Contains(m, "gpt-4"),
		strings.Contains(m, "deepseek"),
		strings.Contains(m, "kimi"),
		strings.Contains(m, "moonshot"),
		strings.Contains(m, "glm"),
		strings.Contains(m, "minimax"):
		return defaultModelContextWindowTokens
	default:
		return defaultModelContextWindowTokens
	}
}

func splitContextBudget(totalLimit int, tools []ports.ToolDefinition) contextBudgetSplit {
	return splitContextBudgetWithEstimator(totalLimit, tools, estimateToolDefinitionTokens)
}

func (e *ReactEngine) splitContextBudget(totalLimit int, tools []ports.ToolDefinition) contextBudgetSplit {
	if e == nil {
		return splitContextBudget(totalLimit, tools)
	}
	return splitContextBudgetWithEstimator(totalLimit, tools, e.estimateToolDefinitionTokensCached)
}

func splitContextBudgetWithEstimator(
	totalLimit int,
	tools []ports.ToolDefinition,
	estimator func([]ports.ToolDefinition) int,
) contextBudgetSplit {
	split := contextBudgetSplit{TotalLimit: totalLimit, MessageLimit: totalLimit}
	if totalLimit <= 0 {
		return split
	}
	if estimator == nil {
		estimator = estimateToolDefinitionTokens
	}

	toolTokens := estimator(tools)
	messageLimit := totalLimit - toolTokens - contextBudgetRequestSafetyTokens
	if messageLimit < minMessageBudgetTokens {
		messageLimit = minMessageBudgetTokens
	}
	if messageLimit > totalLimit {
		messageLimit = totalLimit
	}

	split.ToolTokens = toolTokens
	split.MessageLimit = messageLimit
	return split
}

func estimateToolDefinitionTokens(tools []ports.ToolDefinition) int {
	return estimateToolDefinitionTokensWithMarshal(tools, jsonx.Marshal)
}

func estimateToolDefinitionTokensWithMarshal(
	tools []ports.ToolDefinition,
	marshal func(v any) ([]byte, error),
) int {
	if len(tools) == 0 {
		return 0
	}
	if marshal == nil {
		marshal = jsonx.Marshal
	}

	total := 16 // list wrapper overhead
	for _, tool := range tools {
		total += 24 // per-tool wrapper overhead
		total += tokenutil.CountTokens(strings.TrimSpace(tool.Name))
		total += tokenutil.CountTokens(strings.TrimSpace(tool.Description))
		payload, err := marshal(tool.Parameters)
		if err != nil {
			continue
		}
		total += tokenutil.CountTokens(string(payload))
	}
	return total
}

func (e *ReactEngine) estimateToolDefinitionTokensCached(tools []ports.ToolDefinition) int {
	if len(tools) == 0 {
		return 0
	}
	if e == nil {
		return estimateToolDefinitionTokens(tools)
	}

	signature := toolDefinitionsSignature(tools)
	if tokens, ok := e.toolTokenCache.load(signature); ok {
		return tokens
	}

	e.toolTokenCache.mu.Lock()
	defer e.toolTokenCache.mu.Unlock()
	if e.toolTokenCache.ready && e.toolTokenCache.signature == signature {
		return e.toolTokenCache.tokens
	}

	total := estimateToolDefinitionTokensWithMarshal(tools, e.toolParameterMarshaler())
	e.toolTokenCache.signature = signature
	e.toolTokenCache.tokens = total
	e.toolTokenCache.ready = true
	return total
}

func (e *ReactEngine) toolParameterMarshaler() func(v any) ([]byte, error) {
	if e != nil && e.toolParameterMarshal != nil {
		return e.toolParameterMarshal
	}
	return jsonx.Marshal
}

func toolDefinitionsSignature(tools []ports.ToolDefinition) uint64 {
	hasher := fnv.New64a()
	hashUint64(hasher, uint64(len(tools)))
	for _, tool := range tools {
		hashString(hasher, strings.TrimSpace(tool.Name))
		hashString(hasher, strings.TrimSpace(tool.Description))
		hashParameterSchema(hasher, tool.Parameters)
		hashStringSlice(hasher, tool.MaterialCapabilities.Consumes)
		hashStringSlice(hasher, tool.MaterialCapabilities.Produces)
		hashStringSlice(hasher, tool.MaterialCapabilities.ProducesArtifacts)
	}
	return hasher.Sum64()
}

func hashParameterSchema(hasher hash.Hash64, schema ports.ParameterSchema) {
	hashString(hasher, strings.TrimSpace(schema.Type))
	keys := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	hashUint64(hasher, uint64(len(keys)))
	for _, name := range keys {
		hashString(hasher, name)
		hashProperty(hasher, schema.Properties[name])
	}
	required := append([]string(nil), schema.Required...)
	sort.Strings(required)
	hashStringSlice(hasher, required)
}

func hashProperty(hasher hash.Hash64, prop ports.Property) {
	hashString(hasher, strings.TrimSpace(prop.Type))
	hashString(hasher, strings.TrimSpace(prop.Description))
	hashUint64(hasher, uint64(len(prop.Enum)))
	for _, enumValue := range prop.Enum {
		hashAny(hasher, enumValue)
	}
	if prop.Items == nil {
		hashUint64(hasher, 0)
		return
	}
	hashUint64(hasher, 1)
	hashProperty(hasher, *prop.Items)
}

func hashAny(hasher hash.Hash64, value any) {
	switch v := value.(type) {
	case nil:
		hashString(hasher, "nil")
	case string:
		hashString(hasher, "string")
		hashString(hasher, v)
	case bool:
		hashString(hasher, "bool")
		if v {
			hashUint64(hasher, 1)
		} else {
			hashUint64(hasher, 0)
		}
	case float64:
		hashString(hasher, "float64")
		hashString(hasher, fmt.Sprintf("%g", v))
	default:
		// Handles []any, map[string]any, and any unexpected types via JSON.
		hashString(hasher, fmt.Sprintf("%T", value))
		payload, err := jsonx.Marshal(value)
		if err != nil {
			hashString(hasher, fmt.Sprintf("%v", value))
			return
		}
		hashString(hasher, string(payload))
	}
}

func hashStringSlice(hasher hash.Hash64, values []string) {
	hashUint64(hasher, uint64(len(values)))
	for _, value := range values {
		hashString(hasher, value)
	}
}

func hashString(hasher hash.Hash64, value string) {
	hashUint64(hasher, uint64(len(value)))
	if value == "" {
		return
	}
	_, _ = hasher.Write([]byte(value))
}

func hashUint64(hasher hash.Hash64, value uint64) {
	var bytes [8]byte
	binary.LittleEndian.PutUint64(bytes[:], value)
	_, _ = hasher.Write(bytes[:])
}

func (c *toolDefinitionTokenCache) load(signature uint64) (int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.ready || c.signature != signature {
		return 0, false
	}
	return c.tokens, true
}
