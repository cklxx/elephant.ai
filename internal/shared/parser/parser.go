package parser

import (
	"encoding/json"
	"fmt"
	"regexp"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
)

// Pre-compiled regexes for hot path performance (avoid recompilation per call)
var (
	toolCallRe    = regexp.MustCompile(`<tool_call>(.*?)</tool_call>`)
	toolNameRe    = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)
	cleanPatterns = []*regexp.Regexp{
		regexp.MustCompile(`<\|tool_call_begin\|>.*?(?:<\|tool_call_end\|>|$)`),
		regexp.MustCompile(`user<\|tool_call_begin\|>.*`),
		regexp.MustCompile(`functions\.[\w_]+:\d+\(.*?\)`),
	}
)

type parser struct{}

func New() tools.FunctionCallParser {
	return &parser{}
}

func (p *parser) Parse(content string) ([]ports.ToolCall, error) {
	// Clean content first: remove incomplete tool call markers
	cleaned := p.cleanLeakedMarkers(content)

	// Parse XML-style tool calls: <tool_call>{...}</tool_call>
	matches := toolCallRe.FindAllStringSubmatch(cleaned, -1)

	var calls []ports.ToolCall
	for _, match := range matches {
		var call struct {
			Name string         `json:"name"`
			Args map[string]any `json:"args"`
		}
		if err := json.Unmarshal([]byte(match[1]), &call); err != nil {
			continue
		}

		// Validate tool name (must be alphanumeric + underscore)
		if !isValidToolName(call.Name) {
			continue
		}

		calls = append(calls, ports.ToolCall{
			ID:        fmt.Sprintf("call_%d", len(calls)),
			Name:      call.Name,
			Arguments: call.Args,
		})
	}

	return calls, nil
}

// cleanLeakedMarkers removes incomplete or malformed tool call markers
func (p *parser) cleanLeakedMarkers(content string) string {
	cleaned := content
	for _, re := range cleanPatterns {
		cleaned = re.ReplaceAllString(cleaned, "")
	}
	return cleaned
}

// isValidToolName checks if tool name is valid (alphanumeric + underscore only)
func isValidToolName(name string) bool {
	return toolNameRe.MatchString(name)
}

func (p *parser) Validate(call ports.ToolCall, definition ports.ToolDefinition) error {
	for _, required := range definition.Parameters.Required {
		if _, ok := call.Arguments[required]; !ok {
			return fmt.Errorf("missing required parameter: %s", required)
		}
	}
	return nil
}
