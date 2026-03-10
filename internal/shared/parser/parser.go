package parser

import (
	"encoding/json"
	"fmt"
	"regexp"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
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

func New() agent.FunctionCallParser {
	return &parser{}
}

func (p *parser) Parse(content string) ([]ports.ToolCall, error) {
	cleaned := cleanLeakedMarkers(content)
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
// that would interfere with XML-style <tool_call> extraction.
func cleanLeakedMarkers(content string) string {
	cleaned := content
	for _, re := range cleanPatterns {
		cleaned = re.ReplaceAllString(cleaned, "")
	}
	return cleaned
}

func isValidToolName(name string) bool {
	return toolNameRe.MatchString(name)
}
