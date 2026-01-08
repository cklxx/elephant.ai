package parser

import (
	"alex/internal/agent/ports"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

type parser struct{}

func New() ports.FunctionCallParser {
	return &parser{}
}

func (p *parser) Parse(content string) ([]ports.ToolCall, error) {
	if calls := parseFunctionGemmaCalls(content); len(calls) > 0 {
		return calls, nil
	}

	// Clean content first: remove incomplete tool call markers
	cleaned := p.cleanLeakedMarkers(content)

	// Parse XML-style tool calls: <tool_call>{...}</tool_call>
	re := regexp.MustCompile(`<tool_call>(.*?)</tool_call>`)
	matches := re.FindAllStringSubmatch(cleaned, -1)

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

const (
	functionCallStart = "<start_function_call>call:"
	functionCallEnd   = "<end_function_call>"
	escapeToken       = "<escape>"
)

func parseFunctionGemmaCalls(content string) []ports.ToolCall {
	var calls []ports.ToolCall
	rest := content
	for {
		startIdx := strings.Index(rest, functionCallStart)
		if startIdx == -1 {
			break
		}
		rest = rest[startIdx+len(functionCallStart):]
		nameEnd := strings.Index(rest, "{")
		if nameEnd == -1 {
			break
		}
		name := strings.TrimSpace(rest[:nameEnd])
		if name == "" {
			break
		}
		argBlock, consumed, ok := extractArgumentBlock(rest[nameEnd+1:])
		if !ok {
			break
		}
		afterArgs := rest[nameEnd+1+consumed:]
		endIdx := strings.Index(afterArgs, functionCallEnd)
		if endIdx == -1 {
			break
		}

		args, err := parseFunctionGemmaArgs(argBlock)
		if err == nil && isValidToolName(name) {
			calls = append(calls, ports.ToolCall{
				ID:        fmt.Sprintf("call_%d", len(calls)),
				Name:      name,
				Arguments: args,
			})
		}
		rest = afterArgs[endIdx+len(functionCallEnd):]
	}
	return calls
}

func extractArgumentBlock(input string) (string, int, bool) {
	depth := 1
	i := 0
	for i < len(input) {
		if strings.HasPrefix(input[i:], escapeToken) {
			i += len(escapeToken)
			end := strings.Index(input[i:], escapeToken)
			if end == -1 {
				return "", 0, false
			}
			i += end + len(escapeToken)
			continue
		}
		switch input[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return input[:i], i + 1, true
			}
		}
		i++
	}
	return "", 0, false
}

type argParser struct {
	input string
	pos   int
}

func parseFunctionGemmaArgs(input string) (map[string]any, error) {
	parser := &argParser{input: strings.TrimSpace(input)}
	if parser.input == "" {
		return map[string]any{}, nil
	}
	return parser.parseObject()
}

func (p *argParser) parseObject() (map[string]any, error) {
	obj := make(map[string]any)
	for {
		p.skipSpaces()
		if p.eof() {
			break
		}
		if p.peek() == '}' {
			p.pos++
			break
		}
		key, err := p.parseKey()
		if err != nil {
			return nil, err
		}
		p.skipSpaces()
		if !p.consume(':') {
			return nil, errors.New("missing ':' after key")
		}
		p.skipSpaces()
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		obj[key] = value
		p.skipSpaces()
		if p.consume(',') {
			continue
		}
		if p.peek() == '}' {
			p.pos++
			break
		}
		if p.eof() {
			break
		}
	}
	return obj, nil
}

func (p *argParser) parseArray() ([]any, error) {
	var out []any
	for {
		p.skipSpaces()
		if p.eof() {
			return out, nil
		}
		if p.peek() == ']' {
			p.pos++
			break
		}
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		out = append(out, val)
		p.skipSpaces()
		if p.consume(',') {
			continue
		}
		if p.peek() == ']' {
			p.pos++
			break
		}
	}
	return out, nil
}

func (p *argParser) parseKey() (string, error) {
	if strings.HasPrefix(p.input[p.pos:], escapeToken) {
		return p.parseEscapedString()
	}
	start := p.pos
	for !p.eof() {
		r := rune(p.input[p.pos])
		if r == ':' || r == ',' || r == '}' || r == ']' || unicode.IsSpace(r) {
			break
		}
		p.pos++
	}
	key := strings.TrimSpace(p.input[start:p.pos])
	if key == "" {
		return "", errors.New("empty key")
	}
	return key, nil
}

func (p *argParser) parseValue() (any, error) {
	if p.eof() {
		return nil, nil
	}
	if strings.HasPrefix(p.input[p.pos:], escapeToken) {
		return p.parseEscapedString()
	}
	switch p.peek() {
	case '{':
		p.pos++
		return p.parseObject()
	case '[':
		p.pos++
		return p.parseArray()
	}

	token := p.readBareToken()
	if token == "" {
		return nil, nil
	}
	switch token {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null":
		return nil, nil
	}
	if i, err := strconv.ParseInt(token, 10, 64); err == nil {
		return i, nil
	}
	if f, err := strconv.ParseFloat(token, 64); err == nil {
		return f, nil
	}
	return token, nil
}

func (p *argParser) parseEscapedString() (string, error) {
	if !strings.HasPrefix(p.input[p.pos:], escapeToken) {
		return "", errors.New("expected escape token")
	}
	p.pos += len(escapeToken)
	end := strings.Index(p.input[p.pos:], escapeToken)
	if end == -1 {
		return "", errors.New("unterminated escape token")
	}
	value := p.input[p.pos : p.pos+end]
	p.pos += end + len(escapeToken)
	return value, nil
}

func (p *argParser) readBareToken() string {
	start := p.pos
	for !p.eof() {
		r := rune(p.input[p.pos])
		if r == ',' || r == '}' || r == ']' || unicode.IsSpace(r) {
			break
		}
		p.pos++
	}
	return strings.TrimSpace(p.input[start:p.pos])
}

func (p *argParser) skipSpaces() {
	for !p.eof() {
		if !unicode.IsSpace(rune(p.input[p.pos])) {
			return
		}
		p.pos++
	}
}

func (p *argParser) consume(ch byte) bool {
	if p.eof() || p.input[p.pos] != ch {
		return false
	}
	p.pos++
	return true
}

func (p *argParser) peek() byte {
	if p.eof() {
		return 0
	}
	return p.input[p.pos]
}

func (p *argParser) eof() bool {
	return p.pos >= len(p.input)
}
