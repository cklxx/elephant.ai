package parser

import (
	"alex/internal/agent/ports"
	"encoding/json"
	"fmt"
	"regexp"
)

type parser struct{}

func New() ports.FunctionCallParser {
	return &parser{}
}

func (p *parser) Parse(content string) ([]ports.ToolCall, error) {
	var calls []ports.ToolCall

	// Clean content first: remove incomplete tool call markers
	cleaned := p.cleanLeakedMarkers(content)

	// Parse XML-style tool calls: <tool_call>{...}</tool_call>
	re := regexp.MustCompile(`<tool_call>(.*?)</tool_call>`)
	matches := re.FindAllStringSubmatch(cleaned, -1)

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
	patterns := []string{
		`<\|tool_call_begin\|>.*?(?:<\|tool_call_end\|>|$)`,
		`user<\|tool_call_begin\|>.*`,
		`functions\.[\w_]+:\d+\(.*?\)`,
	}

	cleaned := content
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		cleaned = re.ReplaceAllString(cleaned, "")
	}

	return cleaned
}

// isValidToolName checks if tool name is valid (alphanumeric + underscore only)
func isValidToolName(name string) bool {
	re := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)
	return re.MatchString(name)
}

func (p *parser) Validate(call ports.ToolCall, definition ports.ToolDefinition) error {
	for _, required := range definition.Parameters.Required {
		if _, ok := call.Arguments[required]; !ok {
			return fmt.Errorf("missing required parameter: %s", required)
		}
	}
	return nil
}
