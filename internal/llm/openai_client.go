package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"alex/internal/agent/ports"
	alexerrors "alex/internal/errors"
	"alex/internal/httpclient"
	"alex/internal/logging"
	"alex/internal/utils"
	id "alex/internal/utils/id"
)

// OpenAI API compatible client
type openaiClient struct {
	model         string
	apiKey        string
	baseURL       string
	httpClient    *http.Client
	logger        logging.Logger
	headers       map[string]string
	maxRetries    int
	usageCallback func(usage ports.TokenUsage, model string, provider string)
}

// NewOpenAIClient constructs an LLM client that speaks the OpenAI-compatible
// chat completions API using the provided configuration.
func NewOpenAIClient(model string, config Config) (ports.LLMClient, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://openrouter.ai/api/v1"
	}

	timeout := 120 * time.Second
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}

	logger := utils.NewCategorizedLogger(utils.LogCategoryLLM, "openai")

	return &openaiClient{
		model:      model,
		apiKey:     config.APIKey,
		baseURL:    strings.TrimRight(config.BaseURL, "/"),
		httpClient: httpclient.New(timeout, logger),
		logger:     logger,
		headers:    config.Headers,
		maxRetries: config.MaxRetries,
	}, nil
}

func (c *openaiClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	requestID := extractRequestID(req.Metadata)
	if requestID == "" {
		requestID = id.NewRequestID()
	}
	prefix := fmt.Sprintf("[req:%s] ", requestID)

	// Convert to OpenAI format
	oaiReq := map[string]any{
		"model":       c.model,
		"messages":    c.convertMessages(req.Messages),
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
		"stream":      false,
	}

	if len(req.Tools) > 0 {
		oaiReq["tools"] = c.convertTools(req.Tools)
		oaiReq["tool_choice"] = "auto"
	}

	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	logBody := redactDataURIs(body)

	// Debug log: Request details
	c.logger.Debug("%s=== LLM Request ===", prefix)
	c.logger.Debug("%sURL: POST %s/chat/completions", prefix, c.baseURL)
	c.logger.Debug("%sModel: %s", prefix, c.model)

	endpoint := c.baseURL + "/chat/completions"

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

	// Debug log: Request headers
	c.logger.Debug("%sRequest Headers:", prefix)
	for k, v := range httpReq.Header {
		if k == "Authorization" {
			// Mask API key for security
			c.logger.Debug("%s  %s: Bearer (hidden)", prefix, k)
		} else {
			c.logger.Debug("%s  %s: %s", prefix, k, strings.Join(v, ", "))
		}
	}

	// Debug log: Request body (pretty print)
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

	// Debug log: Response status
	c.logger.Debug("%s=== LLM Response ===", prefix)
	c.logger.Debug("%sStatus: %d %s", prefix, resp.StatusCode, resp.Status)

	// Debug log: Response headers
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

	var oaiResp struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
		Error *struct {
			Type    string          `json:"type"`
			Message string          `json:"message"`
			Code    json.RawMessage `json:"code"`
		} `json:"error"`
	}

	// Debug log: Response body (pretty print)
	var prettyResp bytes.Buffer
	if err := json.Indent(&prettyResp, respBody, "", "  "); err == nil {
		c.logger.Debug("%sResponse Body:\n%s", prefix, prettyResp.String())
	} else {
		c.logger.Debug("%sResponse Body: %s", prefix, string(respBody))
	}
	utils.LogStreamingResponsePayload(requestID, append([]byte(nil), respBody...))

	if err := json.Unmarshal(respBody, &oaiResp); err != nil {
		c.logger.Debug("%sFailed to decode response: %v", prefix, err)
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if oaiResp.Error != nil && oaiResp.Error.Message != "" {
		errMsg := oaiResp.Error.Message
		if oaiResp.Error.Type != "" {
			errMsg = fmt.Sprintf("%s: %s", oaiResp.Error.Type, oaiResp.Error.Message)
		}
		return nil, mapHTTPError(resp.StatusCode, []byte(errMsg), resp.Header)
	}

	if len(oaiResp.Choices) == 0 {
		c.logger.Debug("%sNo choices in response", prefix)
		return nil, alexerrors.NewTransientError(errors.New("no choices in response"), "LLM returned an empty response. Please retry.")
	}

	result := &ports.CompletionResponse{
		Content:    oaiResp.Choices[0].Message.Content,
		StopReason: oaiResp.Choices[0].FinishReason,
		Usage: ports.TokenUsage{
			PromptTokens:     oaiResp.Usage.PromptTokens,
			CompletionTokens: oaiResp.Usage.CompletionTokens,
			TotalTokens:      oaiResp.Usage.TotalTokens,
		},
		Metadata: map[string]any{
			"request_id": requestID,
		},
	}

	// Trigger usage callback if set
	if c.usageCallback != nil {
		provider := "openrouter"
		switch {
		case strings.Contains(c.baseURL, "api.openai.com"):
			provider = "openai"
		case strings.Contains(c.baseURL, "api.deepseek.com"):
			provider = "deepseek"
		case strings.Contains(strings.ToLower(c.baseURL), "antigravity"):
			provider = "antigravity"
		}
		c.usageCallback(result.Usage, c.model, provider)
	}

	// Convert tool calls
	for _, tc := range oaiResp.Choices[0].Message.ToolCalls {
		var args map[string]any
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			c.logger.Debug("%sFailed to parse tool call arguments: %v", prefix, err)
			continue
		}
		result.ToolCalls = append(result.ToolCalls, ports.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: args,
		})
	}

	// Debug log: Summary
	c.logger.Debug("%s=== LLM Response Summary ===", prefix)
	c.logger.Debug("%sStop Reason: %s", prefix, result.StopReason)
	c.logger.Debug("%sContent Length: %d chars", prefix, len(result.Content))
	c.logger.Debug("%sTool Calls: %d", prefix, len(result.ToolCalls))
	c.logger.Debug("%sUsage: %d prompt + %d completion = %d total tokens",
		prefix,
		result.Usage.PromptTokens,
		result.Usage.CompletionTokens,
		result.Usage.TotalTokens)
	c.logger.Debug("%s==================", prefix)

	return result, nil
}

// StreamComplete streams incremental completion deltas while constructing the
// final aggregated response.
func (c *openaiClient) StreamComplete(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	requestID := extractRequestID(req.Metadata)
	if requestID == "" {
		requestID = id.NewRequestID()
	}
	prefix := fmt.Sprintf("[req:%s] ", requestID)
	provider := "openrouter"
	switch {
	case strings.Contains(c.baseURL, "api.openai.com"):
		provider = "openai"
	case strings.Contains(c.baseURL, "api.deepseek.com"):
		provider = "deepseek"
	case strings.Contains(strings.ToLower(c.baseURL), "antigravity"):
		provider = "antigravity"
	}

	oaiReq := map[string]any{
		"model":       c.model,
		"messages":    c.convertMessages(req.Messages),
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
		"stream":      true,
	}

	if len(req.Tools) > 0 {
		oaiReq["tools"] = c.convertTools(req.Tools)
		oaiReq["tool_choice"] = "auto"
	}

	if len(req.StopSequences) > 0 {
		oaiReq["stop"] = append([]string(nil), req.StopSequences...)
	}

	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	logBody := redactDataURIs(body)

	c.logger.Debug("%s=== LLM Request ===", prefix)
	c.logger.Debug("%sURL: POST %s/chat/completions", prefix, c.baseURL)
	c.logger.Debug("%sModel: %s", prefix, c.model)

	endpoint := c.baseURL + "/chat/completions"

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

	requestStarted := time.Now()
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

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	type toolCallDelta struct {
		Index    int    `json:"index"`
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}

	type streamChunk struct {
		Choices []struct {
			Delta struct {
				Content   string          `json:"content"`
				Role      string          `json:"role"`
				ToolCalls []toolCallDelta `json:"tool_calls"`
			} `json:"delta"`
			FinishReason *string `json:"finish_reason"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	type toolAccumulator struct {
		id        string
		name      string
		arguments strings.Builder
	}

	toolAccumulators := make(map[int]*toolAccumulator)
	var toolOrder []int

	var contentBuilder strings.Builder
	usage := ports.TokenUsage{}
	finishReason := ""
	loggedTTFB := false

	appendToolCall := func(idx int) *toolAccumulator {
		acc, ok := toolAccumulators[idx]
		if !ok {
			acc = &toolAccumulator{}
			toolAccumulators[idx] = acc
			toolOrder = append(toolOrder, idx)
		}
		return acc
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			break
		}

		if !loggedTTFB {
			loggedTTFB = true
			logCLILatencyf(
				"[latency] llm_stream_ttfb_ms=%.2f provider=%s model=%s request_id=%s\n",
				float64(time.Since(requestStarted))/float64(time.Millisecond),
				provider,
				c.model,
				requestID,
			)
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			c.logger.Debug("%sFailed to decode stream chunk: %v", prefix, err)
			continue
		}

		if chunk.Usage != nil {
			usage.PromptTokens = chunk.Usage.PromptTokens
			usage.CompletionTokens = chunk.Usage.CompletionTokens
			usage.TotalTokens = chunk.Usage.TotalTokens
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			finishReason = *choice.FinishReason
		}

		if text := choice.Delta.Content; text != "" {
			contentBuilder.WriteString(text)
			if callbacks.OnContentDelta != nil {
				callbacks.OnContentDelta(ports.ContentDelta{Delta: text})
			}
		}

		for _, tc := range choice.Delta.ToolCalls {
			acc := appendToolCall(tc.Index)
			if tc.ID != "" {
				acc.id = tc.ID
			}
			if tc.Function.Name != "" {
				acc.name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				acc.arguments.WriteString(tc.Function.Arguments)
			}
		}

	}

	if err := scanner.Err(); err != nil {
		c.logger.Debug("%sStream read error: %v", prefix, err)
		return nil, fmt.Errorf("read response stream: %w", err)
	}

	if callbacks.OnContentDelta != nil {
		callbacks.OnContentDelta(ports.ContentDelta{Final: true})
	}

	result := &ports.CompletionResponse{
		Content:    contentBuilder.String(),
		StopReason: finishReason,
		Usage:      usage,
		Metadata: map[string]any{
			"request_id": requestID,
		},
	}

	for _, idx := range toolOrder {
		acc := toolAccumulators[idx]
		if acc == nil {
			continue
		}
		var args map[string]any
		if acc.arguments.Len() > 0 {
			if err := json.Unmarshal([]byte(acc.arguments.String()), &args); err != nil {
				c.logger.Debug("%sFailed to parse tool call arguments: %v", prefix, err)
			}
		}
		result.ToolCalls = append(result.ToolCalls, ports.ToolCall{
			ID:        acc.id,
			Name:      acc.name,
			Arguments: args,
		})
	}

	if c.usageCallback != nil {
		c.usageCallback(result.Usage, c.model, provider)
	}

	if respPayload, err := json.Marshal(map[string]any{
		"content":     result.Content,
		"stop_reason": result.StopReason,
		"tool_calls":  result.ToolCalls,
		"usage":       result.Usage,
	}); err != nil {
		c.logger.Debug("%sFailed to marshal streaming response payload: %v", prefix, err)
	} else {
		utils.LogStreamingResponsePayload(requestID, respPayload)
	}

	summaryBuilder := &strings.Builder{}
	summaryBuilder.WriteString("=== LLM Streaming Summary ===\n")
	fmt.Fprintf(summaryBuilder, "Stop Reason: %s\n", result.StopReason)
	fmt.Fprintf(summaryBuilder, "Content Length: %d chars\n", len(result.Content))
	fmt.Fprintf(summaryBuilder, "Tool Calls: %d\n", len(result.ToolCalls))
	fmt.Fprintf(summaryBuilder, "Usage: %d prompt + %d completion = %d total tokens\n",
		result.Usage.PromptTokens,
		result.Usage.CompletionTokens,
		result.Usage.TotalTokens,
	)
	summaryBuilder.WriteString("==================")
	utils.LogStreamingSummary(requestID, []byte(summaryBuilder.String()))

	c.logger.Debug("%s=== LLM Streaming Summary ===", prefix)
	c.logger.Debug("%sStop Reason: %s", prefix, result.StopReason)
	c.logger.Debug("%sContent Length: %d chars", prefix, len(result.Content))
	c.logger.Debug("%sTool Calls: %d", prefix, len(result.ToolCalls))
	c.logger.Debug("%sUsage: %d prompt + %d completion = %d total tokens",
		prefix,
		result.Usage.PromptTokens,
		result.Usage.CompletionTokens,
		result.Usage.TotalTokens)
	c.logger.Debug("%s==================", prefix)

	return result, nil
}

func (c *openaiClient) Model() string {
	return c.model
}

// SetUsageCallback implements UsageTrackingClient
func (c *openaiClient) SetUsageCallback(callback func(usage ports.TokenUsage, model string, provider string)) {
	c.usageCallback = callback
}

func (c *openaiClient) convertMessages(msgs []ports.Message) []map[string]any {
	result := make([]map[string]any, 0, len(msgs))
	for _, msg := range msgs {
		if msg.Source == ports.MessageSourceDebug || msg.Source == ports.MessageSourceEvaluation {
			continue
		}
		entry := map[string]any{"role": msg.Role}
		entry["content"] = buildMessageContent(msg, shouldEmbedAttachmentsInContent(msg))
		if msg.ToolCallID != "" {
			entry["tool_call_id"] = msg.ToolCallID
		}
		if len(msg.ToolCalls) > 0 {
			entry["tool_calls"] = buildToolCallHistory(msg.ToolCalls)
		}
		result = append(result, entry)
	}
	return result
}

func extractRequestID(metadata map[string]any) string {
	if metadata == nil {
		return ""
	}
	if value, ok := metadata["request_id"]; ok {
		switch v := value.(type) {
		case string:
			return strings.TrimSpace(v)
		case fmt.Stringer:
			return strings.TrimSpace(v.String())
		}
	}
	return ""
}

func buildMessageContent(msg ports.Message, embedAttachments bool) any {
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
			"type": "text",
			"text": text,
		})
	}

	appendImage := func(url string) {
		if url == "" {
			return
		}
		hasImage = true
		parts = append(parts, map[string]any{
			"type": "image_url",
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

func (c *openaiClient) convertTools(tools []ports.ToolDefinition) []map[string]any {
	converted := convertTools(tools)
	if len(converted) != len(tools) {
		for _, tool := range tools {
			if !isValidToolName(tool.Name) {
				c.logger.Warn("Skipping tool with invalid function name for OpenAI: %s", tool.Name)
			}
		}
	}
	return converted
}
