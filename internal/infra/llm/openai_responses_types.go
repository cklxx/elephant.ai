package llm

import "alex/internal/shared/json"

type responsesResponse struct {
	ID         string                 `json:"id"`
	Status     string                 `json:"status"`
	Output     []responseOutputItem   `json:"output"`
	OutputText any                    `json:"output_text"`
	Usage      responsesUsage         `json:"usage"`
	Error      *responsesErrorPayload `json:"error"`
}

type responsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type responsesErrorPayload struct {
	Type    string           `json:"type"`
	Message string           `json:"message"`
	Code    jsonx.RawMessage `json:"code"`
}

type responseOutputItem struct {
	Type      string              `json:"type"`
	ID        string              `json:"id"`
	Role      string              `json:"role"`
	Name      string              `json:"name"`
	Arguments jsonx.RawMessage    `json:"arguments"`
	Content   []responseContent   `json:"content"`
	ToolCalls []responseToolCall  `json:"tool_calls"`
	Metadata  map[string]any      `json:"metadata"`
	Delta     map[string]any      `json:"delta"`
	Item      *responseOutputItem `json:"item"`
	Response  *responsesResponse  `json:"response"`
	OutputIdx int                 `json:"output_index"`
}

type responseContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type responseToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}
