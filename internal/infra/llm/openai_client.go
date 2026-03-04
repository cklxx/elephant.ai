package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	alexerrors "alex/internal/shared/errors"
	"alex/internal/shared/json"
	"alex/internal/shared/utils"
)

// OpenAI API compatible client
type openaiClient struct {
	baseClient
	kimiCompat bool
}

// NewOpenAIClient constructs an LLM client that speaks the OpenAI-compatible
// chat completions API using the provided configuration.
func NewOpenAIClient(model string, config Config) (portsllm.LLMClient, error) {
	return &openaiClient{
		baseClient: newBaseClient(model, config, baseClientOpts{
			defaultBaseURL: "https://openrouter.ai/api/v1",
			logCategory:    utils.LogCategoryLLM,
			logComponent:   "openai",
		}),
		kimiCompat: config.KimiCompat,
	}, nil
}

func (c *openaiClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	requestID, prefix := c.buildLogPrefix(ctx, req.Metadata)
	provider := c.detectProvider()

	oaiReq := c.buildOpenAIRequest(req, false)

	body, err := jsonx.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	logBody := redactDataURIs(body)

	endpoint := c.baseURL + "/chat/completions"
	c.logRequestMeta(prefix, "POST", endpoint)

	c.logger.Debug("%sRequest Body: %s", prefix, logBody)
	utils.LogStreamingRequestPayload(requestID, logBody)

	resp, err := c.doPost(ctx, endpoint, body)
	if err != nil {
		c.logger.Debug("%sHTTP request failed: %v", prefix, err)
		wrapped := wrapRequestError(err)
		c.logTransportFailure(prefix, requestID, "complete", provider, endpoint, req, wrapped)
		return nil, wrapped
	}
	defer func() { _ = resp.Body.Close() }()

	c.logResponseStatus(prefix, resp)

	respBody, err := readResponseBody(resp.Body)
	if err != nil {
		c.logger.Debug("%sFailed to read response body: %v", prefix, err)
		readErr := fmt.Errorf("read response: %w", err)
		c.logProcessingFailure(prefix, requestID, "complete", provider, endpoint, "read_response", req, readErr)
		return nil, readErr
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.logger.Debug("%sError Response Body: %s", prefix, respBody)
		mappedErr := mapHTTPError(resp.StatusCode, respBody, resp.Header)
		c.logHTTPFailure(prefix, requestID, "complete", provider, endpoint, req, resp.StatusCode, resp.Header, respBody, mappedErr)
		return nil, mappedErr
	}

	var oaiResp struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				Reasoning        string `json:"reasoning"`
				ReasoningContent string `json:"reasoning_content"`
				ToolCalls        []struct {
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
			Type    string           `json:"type"`
			Message string           `json:"message"`
			Code    jsonx.RawMessage `json:"code"`
		} `json:"error"`
	}

	c.logger.Debug("%sResponse Body: %s", prefix, respBody)
	utils.LogStreamingResponsePayload(requestID, respBody)

	if err := jsonx.Unmarshal(respBody, &oaiResp); err != nil {
		c.logger.Debug("%sFailed to decode response: %v", prefix, err)
		decodeErr := fmt.Errorf("decode response: %w", err)
		c.logProcessingFailure(prefix, requestID, "complete", provider, endpoint, "decode_response", req, decodeErr)
		return nil, decodeErr
	}

	if oaiResp.Error != nil && oaiResp.Error.Message != "" {
		errMsg := oaiResp.Error.Message
		if oaiResp.Error.Type != "" {
			errMsg = fmt.Sprintf("%s: %s", oaiResp.Error.Type, oaiResp.Error.Message)
		}
		mappedErr := mapHTTPError(resp.StatusCode, []byte(errMsg), resp.Header)
		c.logHTTPFailure(prefix, requestID, "complete", provider, endpoint, req, resp.StatusCode, resp.Header, []byte(errMsg), mappedErr)
		return nil, mappedErr
	}

	if len(oaiResp.Choices) == 0 {
		c.logger.Debug("%sNo choices in response", prefix)
		emptyErr := alexerrors.NewTransientError(errors.New("no choices in response"), "LLM returned an empty response. Please retry.")
		c.logProcessingFailure(prefix, requestID, "complete", provider, endpoint, "empty_choices", req, emptyErr)
		return nil, emptyErr
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
	thinking := ports.Thinking{}
	appendThinkingText(&thinking, "reasoning", oaiResp.Choices[0].Message.Reasoning)
	appendThinkingText(&thinking, "reasoning_content", oaiResp.Choices[0].Message.ReasoningContent)
	if len(thinking.Parts) > 0 {
		result.Thinking = thinking
	}

	c.fireUsageCallback(result.Usage, provider)

	// Convert tool calls
	for _, tc := range oaiResp.Choices[0].Message.ToolCalls {
		var args map[string]any
		if err := jsonx.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			c.logger.Debug("%sFailed to parse tool call arguments: %v", prefix, err)
			continue
		}
		result.ToolCalls = append(result.ToolCalls, ports.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: args,
		})
	}

	c.logResponseSummary(prefix, result)
	return result, nil
}

// StreamComplete streams incremental completion deltas while constructing the
// final aggregated response.
func (c *openaiClient) StreamComplete(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	requestID, prefix := c.buildLogPrefix(ctx, req.Metadata)
	provider := c.detectProvider()

	oaiReq := c.buildOpenAIRequest(req, true)

	body, err := jsonx.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	logBody := redactDataURIs(body)

	endpoint := c.baseURL + "/chat/completions"
	c.logRequestMeta(prefix, "POST", endpoint)

	c.logger.Debug("%sRequest Body: %s", prefix, logBody)
	utils.LogStreamingRequestPayload(requestID, logBody)

	requestStarted := time.Now()
	resp, err := c.doPost(ctx, endpoint, body)
	if err != nil {
		c.logger.Debug("%sHTTP request failed: %v", prefix, err)
		wrapped := wrapRequestError(err)
		c.logTransportFailure(prefix, requestID, "stream", provider, endpoint, req, wrapped)
		return nil, wrapped
	}
	defer func() { _ = resp.Body.Close() }()

	c.logger.Debug("%s=== LLM Streaming Response ===", prefix)
	c.logResponseStatus(prefix, resp)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, readErr := readResponseBody(resp.Body)
		if readErr != nil {
			c.logger.Debug("%sFailed to read error response: %v", prefix, readErr)
			errRead := fmt.Errorf("read response: %w", readErr)
			c.logProcessingFailure(prefix, requestID, "stream", provider, endpoint, "read_error_response", req, errRead)
			return nil, errRead
		}
		c.logger.Debug("%sError Response Body: %s", prefix, respBody)
		mappedErr := mapHTTPError(resp.StatusCode, respBody, resp.Header)
		c.logHTTPFailure(prefix, requestID, "stream", provider, endpoint, req, resp.StatusCode, resp.Header, respBody, mappedErr)
		return nil, mappedErr
	}

	scanner := newStreamScanner(resp.Body)

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
				Content          string          `json:"content"`
				Reasoning        string          `json:"reasoning"`
				ReasoningContent string          `json:"reasoning_content"`
				Role             string          `json:"role"`
				ToolCalls        []toolCallDelta `json:"tool_calls"`
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
	var reasoningBuilder strings.Builder
	var reasoningContentBuilder strings.Builder
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
		if err := jsonx.Unmarshal([]byte(payload), &chunk); err != nil {
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
		if reasoning := choice.Delta.Reasoning; reasoning != "" {
			reasoningBuilder.WriteString(reasoning)
		}
		if reasoning := choice.Delta.ReasoningContent; reasoning != "" {
			reasoningContentBuilder.WriteString(reasoning)
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
		streamErr := fmt.Errorf("read response stream: %w", err)
		c.logProcessingFailure(prefix, requestID, "stream", provider, endpoint, "read_stream", req, streamErr)
		return nil, streamErr
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
	thinking := ports.Thinking{}
	if reasoningBuilder.Len() > 0 {
		appendThinkingText(&thinking, "reasoning", reasoningBuilder.String())
	}
	if reasoningContentBuilder.Len() > 0 {
		appendThinkingText(&thinking, "reasoning_content", reasoningContentBuilder.String())
	}
	if len(thinking.Parts) > 0 {
		result.Thinking = thinking
	}

	for _, idx := range toolOrder {
		acc := toolAccumulators[idx]
		if acc == nil {
			continue
		}
		var args map[string]any
		if acc.arguments.Len() > 0 {
			if err := jsonx.Unmarshal([]byte(acc.arguments.String()), &args); err != nil {
				c.logger.Debug("%sFailed to parse tool call arguments: %v", prefix, err)
			}
		}
		result.ToolCalls = append(result.ToolCalls, ports.ToolCall{
			ID:        acc.id,
			Name:      acc.name,
			Arguments: args,
		})
	}

	c.fireUsageCallback(result.Usage, provider)

	if respPayload, err := jsonx.Marshal(map[string]any{
		"content":     result.Content,
		"stop_reason": result.StopReason,
		"tool_calls":  result.ToolCalls,
		"usage":       result.Usage,
	}); err != nil {
		c.logger.Debug("%sFailed to marshal streaming response payload: %v", prefix, err)
	} else {
		utils.LogStreamingResponsePayload(requestID, respPayload)
	}

	c.logResponseSummary(prefix, result)
	return result, nil
}

func (c *openaiClient) buildOpenAIRequest(req ports.CompletionRequest, stream bool) map[string]any {
	oaiReq := map[string]any{
		"model":       c.model,
		"messages":    c.convertMessages(req.Messages),
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
		"stream":      stream,
	}

	if shouldSendArkReasoning(c.baseURL, req.Thinking) {
		effort := strings.TrimSpace(req.Thinking.Effort)
		if effort == "" {
			effort = "medium"
		}
		oaiReq["reasoning_effort"] = effort
		if req.MaxTokens > 0 {
			delete(oaiReq, "max_tokens")
			oaiReq["max_completion_tokens"] = req.MaxTokens
		}
	} else if shouldSendOpenAIReasoning(c.baseURL, c.model, req.Thinking) {
		if reasoning := buildOpenAIReasoningConfig(req.Thinking); reasoning != nil {
			oaiReq["reasoning"] = reasoning
		}
	}

	if len(req.Tools) > 0 {
		oaiReq["tools"] = c.convertTools(req.Tools)
		oaiReq["tool_choice"] = "auto"
	}

	if stream && len(req.StopSequences) > 0 {
		oaiReq["stop"] = append([]string(nil), req.StopSequences...)
	}

	return oaiReq
}

// SetUsageCallback implements UsageTrackingClient
func (c *openaiClient) SetUsageCallback(callback func(usage ports.TokenUsage, model string, provider string)) {
	c.usageCallback = callback
}

// detectProvider infers the provider name from the base URL.
func (c *openaiClient) detectProvider() string {
	switch {
	case strings.Contains(c.baseURL, "api.openai.com"):
		return "openai"
	case strings.Contains(c.baseURL, "api.deepseek.com"):
		return "deepseek"
	case strings.Contains(strings.ToLower(c.baseURL), "ark"):
		return "ark"
	default:
		return "openrouter"
	}
}

func (c *openaiClient) convertMessages(msgs []ports.Message) []map[string]any {
	result := make([]map[string]any, 0, len(msgs))
	isKimi := c.kimiCompat
	seenToolCalls := make(map[string]struct{}, 8)
	droppedToolOutputs := make(map[string]struct{}, 4)
	droppedCallIDs := make([]string, 0, 4)
	recordDroppedToolOutput := func(callID string) {
		callID = strings.TrimSpace(callID)
		if callID == "" {
			callID = "<empty_call_id>"
		}
		if _, exists := droppedToolOutputs[callID]; exists {
			return
		}
		droppedToolOutputs[callID] = struct{}{}
		droppedCallIDs = append(droppedCallIDs, callID)
	}
	embedMask := attachmentEmbeddingMask(msgs)
	for idx, msg := range msgs {
		if msg.Source == ports.MessageSourceDebug || msg.Source == ports.MessageSourceEvaluation {
			continue
		}
		role := utils.TrimLower(msg.Role)
		if role == "tool" {
			callID := strings.TrimSpace(msg.ToolCallID)
			if callID == "" {
				recordDroppedToolOutput(callID)
				continue
			}
			if _, ok := seenToolCalls[callID]; !ok {
				recordDroppedToolOutput(callID)
				continue
			}
		}
		entry := map[string]any{"role": msg.Role}
		content := buildMessageContent(msg, embedMask[idx])
		entry["content"] = content
		// Kimi rejects empty user messages and empty assistant messages
		// (no content AND no tool_calls). Empty assistants arise from
		// checkpoint recovery where MessageState drops ToolCalls.
		if isKimi && isEmptyContent(content) {
			if msg.Role == "user" {
				continue
			}
			if msg.Role == "assistant" && len(msg.ToolCalls) == 0 {
				continue
			}
		}
		if msg.ToolCallID != "" {
			entry["tool_call_id"] = msg.ToolCallID
		}
		if len(msg.ToolCalls) > 0 {
			historyCalls := buildToolCallHistory(msg.ToolCalls)
			if len(historyCalls) > 0 {
				entry["tool_calls"] = historyCalls
				for _, call := range historyCalls {
					callID, _ := call["id"].(string)
					callID = strings.TrimSpace(callID)
					if callID == "" {
						continue
					}
					seenToolCalls[callID] = struct{}{}
				}
			}
		}
		// Kimi requires reasoning_content in assistant messages with tool_calls when thinking is enabled.
		// The field must always be present (even as "") to satisfy the API contract; omitting it
		// causes: "thinking is enabled but reasoning_content is missing in assistant tool call message".
		if isKimi && msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			reasoningContent := extractThinkingText(msg.Thinking)
			entry["reasoning_content"] = reasoningContent
		}
		result = append(result, entry)
	}
	if len(droppedCallIDs) > 0 && c.logger != nil {
		c.logger.Warn("Dropped %d orphan/invalid tool message(s) from chat history: %s", len(droppedCallIDs), strings.Join(droppedCallIDs, ", "))
	}
	return result
}

// isEmptyContent checks if the message content is empty (string or array).
func isEmptyContent(content any) bool {
	switch v := content.(type) {
	case string:
		return utils.IsBlank(v)
	case []map[string]any:
		return len(v) == 0
	case nil:
		return true
	default:
		return false
	}
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
	contentWithThinking := appendThinkingToText(msg.Content, msg.Thinking)
	if len(msg.Attachments) == 0 || !embedAttachments {
		return contentWithThinking
	}

	var parts []map[string]any
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

	appendURLImage := func(att ports.Attachment, _ string) bool {
		url := ports.AttachmentReferenceValue(att)
		if url == "" {
			return false
		}
		hasImage = true
		parts = append(parts, map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": url},
		})
		return true
	}

	embedAttachmentImages(msg.Content, msg.Attachments, appendText,
		appendURLImage,
		func(att ports.Attachment, key string) bool {
			url := ports.AttachmentReferenceValue(att)
			if url == "" {
				return false
			}
			appendText("[" + key + "]")
			hasImage = true
			parts = append(parts, map[string]any{
				"type":      "image_url",
				"image_url": map[string]any{"url": url},
			})
			return true
		},
	)

	if !hasImage {
		return contentWithThinking
	}

	if thinkingText := thinkingPromptText(msg.Thinking); thinkingText != "" {
		appendText(thinkingText)
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
