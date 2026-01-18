package llm

import (
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
	requestID := extractRequestID(req.Metadata)
	if requestID == "" {
		requestID = id.NewRequestID()
	}
	prefix := fmt.Sprintf("[req:%s] ", requestID)

	input, instructions := c.convertMessages(req.Messages)
	payload := map[string]any{
		"model":             c.model,
		"input":             input,
		"temperature":       req.Temperature,
		"max_output_tokens": req.MaxTokens,
		"stream":            false,
	}
	if instructions != "" {
		payload["instructions"] = instructions
	}

	if len(req.Tools) > 0 {
		payload["tools"] = convertTools(req.Tools)
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

func (c *openAIResponsesClient) SetUsageCallback(callback func(usage ports.TokenUsage, model string, provider string)) {
	c.usageCallback = callback
}

func (c *openAIResponsesClient) convertMessages(msgs []ports.Message) ([]map[string]any, string) {
	result := make([]map[string]any, 0, len(msgs))
	var instructionsParts []string
	for _, msg := range msgs {
		if msg.Source == ports.MessageSourceDebug || msg.Source == ports.MessageSourceEvaluation {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		if role == "system" {
			if strings.TrimSpace(msg.Content) != "" {
				instructionsParts = append(instructionsParts, msg.Content)
			}
			continue
		}
		entry := map[string]any{"role": msg.Role}
		entry["content"] = buildResponsesMessageContent(msg, shouldEmbedAttachmentsInContent(msg))
		if msg.ToolCallID != "" {
			entry["tool_call_id"] = msg.ToolCallID
		}
		if len(msg.ToolCalls) > 0 {
			entry["tool_calls"] = buildToolCallHistory(msg.ToolCalls)
		}
		result = append(result, entry)
	}
	return result, strings.Join(instructionsParts, "\n\n")
}

func buildResponsesMessageContent(msg ports.Message, embedAttachments bool) any {
	if len(msg.Attachments) == 0 || !embedAttachments {
		return msg.Content
	}

	index := buildAttachmentIndex(msg.Attachments)

	var parts []map[string]any
	used := make(map[string]bool)
	hasImage := false

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
		hasImage = true
		parts = append(parts, map[string]any{
			"type": "input_image",
			"image_url": map[string]any{
				"url": url,
			},
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

	if !hasImage {
		return msg.Content
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
