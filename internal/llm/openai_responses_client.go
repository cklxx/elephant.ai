package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/httpclient"
	"alex/internal/logging"
	"alex/internal/utils"
	id "alex/internal/utils/id"
)

const defaultOpenAIResponsesBaseURL = "https://api.openai.com/v1"

type openAIResponsesClient struct {
	model         string
	apiKey        string
	baseURL       string
	httpClient    *http.Client
	logger        logging.Logger
	headers       map[string]string
	maxRetries    int
	usageCallback func(usage ports.TokenUsage, model string, provider string)
}

func NewOpenAIResponsesClient(model string, config Config) (ports.LLMClient, error) {
	if config.BaseURL == "" {
		config.BaseURL = defaultOpenAIResponsesBaseURL
	}

	timeout := 120 * time.Second
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}

	logger := utils.NewCategorizedLogger(utils.LogCategoryLLM, "openai-responses")

	return &openAIResponsesClient{
		model:      model,
		apiKey:     config.APIKey,
		baseURL:    strings.TrimRight(config.BaseURL, "/"),
		httpClient: httpclient.New(timeout, logger),
		logger:     logger,
		headers:    config.Headers,
		maxRetries: config.MaxRetries,
	}, nil
}

func (c *openAIResponsesClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	if c.isCodexEndpoint() {
		return c.StreamComplete(ctx, req, ports.CompletionStreamCallbacks{})
	}

	requestID := extractRequestID(req.Metadata)
	if requestID == "" {
		requestID = id.NewRequestID()
	}
	prefix := fmt.Sprintf("[req:%s] ", requestID)

	input := c.buildResponsesInput(req.Messages)
	payload := map[string]any{
		"model":       c.model,
		"input":       input,
		"temperature": req.Temperature,
		"stream":      false,
	}
	if req.MaxTokens > 0 && !c.isCodexEndpoint() {
		payload["max_output_tokens"] = req.MaxTokens
	}
	payload["store"] = false

	if len(req.Tools) > 0 {
		payload["tools"] = convertCodexTools(req.Tools)
		payload["tool_choice"] = "auto"
	}

	if len(req.StopSequences) > 0 {
		payload["stop"] = append([]string(nil), req.StopSequences...)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	logBody := redactDataURIs(body)

	c.logger.Debug("%s=== LLM Request ===", prefix)
	c.logger.Debug("%sURL: POST %s/responses", prefix, c.baseURL)
	c.logger.Debug("%sModel: %s", prefix, c.model)

	endpoint := c.baseURL + "/responses"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if c.maxRetries > 0 {
		httpReq.Header.Set("X-Retry-Limit", strconv.Itoa(c.maxRetries))
	}
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}

	c.logger.Debug("%sRequest Headers:", prefix)
	for k, v := range httpReq.Header {
		if k == "Authorization" {
			c.logger.Debug("%s  %s: Bearer (hidden)", prefix, k)
		} else {
			c.logger.Debug("%s  %s: %s", prefix, k, strings.Join(v, ", "))
		}
	}

	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, logBody, "", "  "); err == nil {
		c.logger.Debug("%sRequest Body:\n%s", prefix, prettyJSON.String())
	} else {
		c.logger.Debug("%sRequest Body: %s", prefix, string(logBody))
	}
	utils.LogStreamingRequestPayload(requestID, append([]byte(nil), logBody...))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Debug("%sHTTP request failed: %v", prefix, err)
		return nil, wrapRequestError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	c.logger.Debug("%s=== LLM Response ===", prefix)
	c.logger.Debug("%sStatus: %d %s", prefix, resp.StatusCode, resp.Status)
	c.logger.Debug("%sResponse Headers:", prefix)
	for k, v := range resp.Header {
		c.logger.Debug("%s  %s: %s", prefix, k, strings.Join(v, ", "))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Debug("%sFailed to read response body: %v", prefix, err)
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.logger.Debug("%sError Response Body: %s", prefix, string(respBody))
		return nil, mapHTTPError(resp.StatusCode, respBody, resp.Header)
	}

	var apiResp responsesResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		c.logger.Debug("%sFailed to decode response: %v", prefix, err)
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if apiResp.Error != nil && apiResp.Error.Message != "" {
		errMsg := apiResp.Error.Message
		if apiResp.Error.Type != "" {
			errMsg = fmt.Sprintf("%s: %s", apiResp.Error.Type, apiResp.Error.Message)
		}
		return nil, mapHTTPError(resp.StatusCode, []byte(errMsg), resp.Header)
	}

	content, toolCalls := parseResponsesOutput(apiResp)

	result := &ports.CompletionResponse{
		Content:    content,
		StopReason: apiResp.Status,
		Usage: ports.TokenUsage{
			PromptTokens:     apiResp.Usage.InputTokens,
			CompletionTokens: apiResp.Usage.OutputTokens,
			TotalTokens:      apiResp.Usage.TotalTokens,
		},
		ToolCalls: toolCalls,
		Metadata: map[string]any{
			"request_id":  requestID,
			"response_id": strings.TrimSpace(apiResp.ID),
		},
	}

	if c.usageCallback != nil {
		c.usageCallback(result.Usage, c.model, "openai")
	}

	var prettyResp bytes.Buffer
	if err := json.Indent(&prettyResp, respBody, "", "  "); err == nil {
		c.logger.Debug("%sResponse Body:\n%s", prefix, prettyResp.String())
	} else {
		c.logger.Debug("%sResponse Body: %s", prefix, string(respBody))
	}
	utils.LogStreamingResponsePayload(requestID, append([]byte(nil), respBody...))

	c.logger.Debug("%s=== LLM Response Summary ===", prefix)
	c.logger.Debug("%sStop Reason: %s", prefix, result.StopReason)
	c.logger.Debug("%sContent Length: %d chars", prefix, len(result.Content))
	c.logger.Debug("%sTool Calls: %d", prefix, len(result.ToolCalls))
	c.logger.Debug("%sUsage: %d prompt + %d completion = %d total tokens",
		prefix,
		result.Usage.PromptTokens,
		result.Usage.CompletionTokens,
		result.Usage.TotalTokens,
	)
	c.logger.Debug("%s==================", prefix)

	return result, nil
}

func (c *openAIResponsesClient) Model() string {
	return c.model
}

func (c *openAIResponsesClient) isCodexEndpoint() bool {
	return strings.Contains(c.baseURL, "/backend-api/codex")
}

func (c *openAIResponsesClient) SetUsageCallback(callback func(usage ports.TokenUsage, model string, provider string)) {
	c.usageCallback = callback
}

func (c *openAIResponsesClient) StreamComplete(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	requestID := extractRequestID(req.Metadata)
	if requestID == "" {
		requestID = id.NewRequestID()
	}
	prefix := fmt.Sprintf("[req:%s] ", requestID)

	input := c.buildResponsesInput(req.Messages)
	payload := map[string]any{
		"model":  c.model,
		"input":  input,
		"stream": true,
		"store":  false,
	}
	if !c.isCodexEndpoint() {
		payload["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 && !c.isCodexEndpoint() {
		payload["max_output_tokens"] = req.MaxTokens
	}
	if len(req.Tools) > 0 {
		payload["tools"] = convertCodexTools(req.Tools)
		payload["tool_choice"] = "auto"
	}
	if len(req.StopSequences) > 0 {
		payload["stop"] = append([]string(nil), req.StopSequences...)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	logBody := redactDataURIs(body)

	c.logger.Debug("%s=== LLM Request ===", prefix)
	c.logger.Debug("%sURL: POST %s/responses", prefix, c.baseURL)
	c.logger.Debug("%sModel: %s", prefix, c.model)

	endpoint := c.baseURL + "/responses"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if c.maxRetries > 0 {
		httpReq.Header.Set("X-Retry-Limit", strconv.Itoa(c.maxRetries))
	}
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}

	c.logger.Debug("%sRequest Headers:", prefix)
	for k, v := range httpReq.Header {
		if k == "Authorization" {
			c.logger.Debug("%s  %s: Bearer (hidden)", prefix, k)
		} else {
			c.logger.Debug("%s  %s: %s", prefix, k, strings.Join(v, ", "))
		}
	}

	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, logBody, "", "  "); err == nil {
		c.logger.Debug("%sRequest Body:\n%s", prefix, prettyJSON.String())
	} else {
		c.logger.Debug("%sRequest Body: %s", prefix, string(logBody))
	}
	utils.LogStreamingRequestPayload(requestID, append([]byte(nil), logBody...))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Debug("%sHTTP request failed: %v", prefix, err)
		return nil, wrapRequestError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	c.logger.Debug("%s=== LLM Streaming Response ===", prefix)
	c.logger.Debug("%sStatus: %d %s", prefix, resp.StatusCode, resp.Status)
	c.logger.Debug("%sResponse Headers:", prefix)
	for k, v := range resp.Header {
		c.logger.Debug("%s  %s: %s", prefix, k, strings.Join(v, ", "))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			c.logger.Debug("%sFailed to read error response: %v", prefix, readErr)
			return nil, fmt.Errorf("read response: %w", readErr)
		}
		c.logger.Debug("%sError Response Body: %s", prefix, string(respBody))
		return nil, mapHTTPError(resp.StatusCode, respBody, resp.Header)
	}

	type responsesStreamEvent struct {
		Type     string `json:"type"`
		Delta    string `json:"delta"`
		Message  string `json:"message"`
		Code     string `json:"code"`
		Response *struct {
			ID    string `json:"id"`
			Usage *struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
				TotalTokens  int `json:"total_tokens"`
			} `json:"usage"`
			IncompleteDetails *struct {
				Reason string `json:"reason"`
			} `json:"incomplete_details"`
		} `json:"response"`
		Item *struct {
			Type      string `json:"type"`
			ID        string `json:"id"`
			CallID    string `json:"call_id"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"item"`
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	var contentBuilder strings.Builder
	var toolCalls []ports.ToolCall
	usage := ports.TokenUsage{}
	stopReason := ""
	responseID := ""

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}

		var evt responsesStreamEvent
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			c.logger.Debug("%sFailed to decode stream event: %v", prefix, err)
			continue
		}

		switch evt.Type {
		case "response.created":
			if evt.Response != nil && evt.Response.ID != "" {
				responseID = evt.Response.ID
			}
		case "response.output_text.delta":
			if evt.Delta != "" {
				contentBuilder.WriteString(evt.Delta)
				if callbacks.OnContentDelta != nil {
					callbacks.OnContentDelta(ports.ContentDelta{Delta: evt.Delta})
				}
			}
		case "response.output_item.done":
			if evt.Item != nil && evt.Item.Type == "function_call" {
				args := parseToolArguments([]byte(evt.Item.Arguments))
				toolID := evt.Item.CallID
				if strings.TrimSpace(toolID) == "" {
					toolID = evt.Item.ID
				}
				toolCalls = append(toolCalls, ports.ToolCall{
					ID:        toolID,
					Name:      evt.Item.Name,
					Arguments: args,
				})
			}
		case "response.completed", "response.incomplete":
			stopReason = evt.Type
			if evt.Response != nil && evt.Response.Usage != nil {
				usage = ports.TokenUsage{
					PromptTokens:     evt.Response.Usage.InputTokens,
					CompletionTokens: evt.Response.Usage.OutputTokens,
					TotalTokens:      evt.Response.Usage.TotalTokens,
				}
			}
		case "error":
			if evt.Message != "" {
				return nil, fmt.Errorf("llm error: %s", evt.Message)
			}
			return nil, fmt.Errorf("llm error: %s", string(data))
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read stream: %w", err)
	}

	if callbacks.OnContentDelta != nil {
		callbacks.OnContentDelta(ports.ContentDelta{Final: true})
	}

	content := contentBuilder.String()
	result := &ports.CompletionResponse{
		Content:    content,
		StopReason: stopReason,
		Usage:      usage,
		ToolCalls:  toolCalls,
		Metadata: map[string]any{
			"request_id":  requestID,
			"response_id": strings.TrimSpace(responseID),
		},
	}

	if result.StopReason == "" {
		result.StopReason = "completed"
	}

	if c.usageCallback != nil {
		c.usageCallback(result.Usage, c.model, "openai")
	}

	c.logger.Debug("%s=== LLM Response Summary ===", prefix)
	c.logger.Debug("%sStop Reason: %s", prefix, result.StopReason)
	c.logger.Debug("%sContent Length: %d chars", prefix, len(result.Content))
	c.logger.Debug("%sTool Calls: %d", prefix, len(result.ToolCalls))
	c.logger.Debug("%sUsage: %d prompt + %d completion = %d total tokens",
		prefix,
		result.Usage.PromptTokens,
		result.Usage.CompletionTokens,
		result.Usage.TotalTokens,
	)
	c.logger.Debug("%s==================", prefix)

	return result, nil
}

// Responses input item shapes follow OpenAI Responses API.
// Source: opencode dev branch
// - packages/opencode/src/provider/sdk/openai-compatible/src/responses/openai-responses-api-types.ts
// - packages/opencode/src/provider/sdk/openai-compatible/src/responses/convert-to-openai-responses-input.ts
func (c *openAIResponsesClient) buildResponsesInput(msgs []ports.Message) []map[string]any {
	items := make([]map[string]any, 0, len(msgs))
	for _, msg := range msgs {
		if msg.Source == ports.MessageSourceDebug || msg.Source == ports.MessageSourceEvaluation {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		switch role {
		case "system", "developer":
			if strings.TrimSpace(msg.Content) == "" {
				continue
			}
			items = append(items, map[string]any{
				"role":    role,
				"content": msg.Content,
			})
		case "user":
			parts := buildResponsesUserContent(msg, shouldEmbedAttachmentsInContent(msg))
			if len(parts) == 0 {
				continue
			}
			items = append(items, map[string]any{
				"role":    "user",
				"content": parts,
			})
		case "assistant":
			if strings.TrimSpace(msg.Content) != "" {
				items = append(items, map[string]any{
					"role": "assistant",
					"content": []map[string]any{
						{"type": "output_text", "text": msg.Content},
					},
				})
			}
			for _, call := range msg.ToolCalls {
				if !isValidToolName(call.Name) {
					continue
				}
				callID := strings.TrimSpace(call.ID)
				if callID == "" {
					continue
				}
				args := "{}"
				if len(call.Arguments) > 0 {
					if data, err := json.Marshal(call.Arguments); err == nil {
						args = string(data)
					}
				}
				items = append(items, map[string]any{
					"type":      "function_call",
					"call_id":   callID,
					"name":      call.Name,
					"arguments": args,
				})
			}
		case "tool":
			callID := strings.TrimSpace(msg.ToolCallID)
			if callID == "" {
				continue
			}
			items = append(items, map[string]any{
				"type":    "function_call_output",
				"call_id": callID,
				"output":  msg.Content,
			})
		}
	}
	return items
}

func buildResponsesUserContent(msg ports.Message, embedAttachments bool) []map[string]any {
	if len(msg.Attachments) == 0 || !embedAttachments {
		if msg.Content == "" {
			return nil
		}
		return []map[string]any{
			{"type": "input_text", "text": msg.Content},
		}
	}

	index := buildAttachmentIndex(msg.Attachments)

	var parts []map[string]any
	used := make(map[string]bool)

	appendText := func(text string) {
		if text == "" {
			return
		}
		parts = append(parts, map[string]any{
			"type": "input_text",
			"text": text,
		})
	}

	appendImage := func(url string) {
		if url == "" {
			return
		}
		parts = append(parts, map[string]any{
			"type": "input_image",
			"image_url": url,
		})
	}

	content := msg.Content
	cursor := 0
	matches := attachmentPlaceholderPattern.FindAllStringSubmatchIndex(content, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		if match[0] > cursor {
			appendText(content[cursor:match[0]])
		}
		placeholderToken := content[match[0]:match[1]]
		appendText(placeholderToken)

		name := strings.TrimSpace(content[match[2]:match[3]])
		if name == "" {
			cursor = match[1]
			continue
		}
		if att, key, ok := index.resolve(name); ok && isImageAttachment(att, key) && !used[key] {
			if url := ports.AttachmentReferenceValue(att); url != "" {
				appendImage(url)
				used[key] = true
			}
		}
		cursor = match[1]
	}
	if cursor < len(content) {
		appendText(content[cursor:])
	}

	for _, desc := range orderedImageAttachments(content, msg.Attachments) {
		key := desc.Placeholder
		if key == "" || used[key] {
			continue
		}
		if url := ports.AttachmentReferenceValue(desc.Attachment); url != "" {
			appendText("[" + key + "]")
			appendImage(url)
			used[key] = true
		}
	}

	return parts
}

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
	Type    string          `json:"type"`
	Message string          `json:"message"`
	Code    json.RawMessage `json:"code"`
}

type responseOutputItem struct {
	Type      string              `json:"type"`
	ID        string              `json:"id"`
	Role      string              `json:"role"`
	Name      string              `json:"name"`
	Arguments json.RawMessage     `json:"arguments"`
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

func parseResponsesOutput(resp responsesResponse) (string, []ports.ToolCall) {
	var contentBuilder strings.Builder
	var toolCalls []ports.ToolCall

	for _, item := range resp.Output {
		switch strings.ToLower(strings.TrimSpace(item.Type)) {
		case "message":
			for _, part := range item.Content {
				kind := strings.ToLower(strings.TrimSpace(part.Type))
				if kind == "output_text" || kind == "text" {
					contentBuilder.WriteString(part.Text)
				}
			}
			for _, tc := range item.ToolCalls {
				args := parseToolArguments([]byte(tc.Function.Arguments))
				toolCalls = append(toolCalls, ports.ToolCall{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: args,
				})
			}
		case "tool_call", "function_call":
			args := parseToolArguments(item.Arguments)
			toolCalls = append(toolCalls, ports.ToolCall{
				ID:        item.ID,
				Name:      item.Name,
				Arguments: args,
			})
		}
	}

	content := contentBuilder.String()
	if strings.TrimSpace(content) == "" {
		if text := flattenOutputText(resp.OutputText); text != "" {
			content = text
		}
	}

	return content, toolCalls
}

func flattenOutputText(raw any) string {
	switch v := raw.(type) {
	case string:
		return v
	case []any:
		var builder strings.Builder
		for _, item := range v {
			if s, ok := item.(string); ok {
				builder.WriteString(s)
			}
		}
		return builder.String()
	default:
		return ""
	}
}

func parseToolArguments(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}

	var args map[string]any
	if err := json.Unmarshal(raw, &args); err == nil {
		return args
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err != nil {
		return nil
	}

	if err := json.Unmarshal([]byte(asString), &args); err != nil {
		return nil
	}
	return args
}
