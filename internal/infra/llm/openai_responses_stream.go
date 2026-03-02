package llm

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	"alex/internal/shared/json"
	"alex/internal/shared/utils"
)

func (c *openAIResponsesClient) StreamComplete(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	requestID, prefix := c.buildLogPrefix(ctx, req.Metadata)
	provider := "openai-responses"
	if c.isCodexEndpoint() {
		provider = "codex"
	}

	input, instructions := c.buildResponsesInputAndInstructions(req.Messages)
	var droppedCallIDs []string
	input, droppedCallIDs = pruneOrphanFunctionCallOutputs(input)
	if len(droppedCallIDs) > 0 {
		c.logger.Warn("%sDropped %d orphan function_call_output item(s): %s", prefix, len(droppedCallIDs), strings.Join(droppedCallIDs, ", "))
	}
	if len(input) == 0 {
		if fallbackInput, ok := synthesizeFallbackResponsesInput(req.Messages, instructions); ok {
			input = fallbackInput
			c.logger.Warn("%sResponses input was empty after conversion; synthesized fallback input from %d message(s).", prefix, len(req.Messages))
		}
	}
	if len(input) == 0 {
		emptyErr := fmt.Errorf("responses input is empty after converting %d message(s) — nothing to send", len(req.Messages))
		c.logger.Warn("%s%v", prefix, emptyErr)
		return nil, emptyErr
	}
	payload := map[string]any{
		"model":  c.model,
		"input":  input,
		"stream": true,
		"store":  false,
	}
	if shouldSendOpenAIReasoning(c.baseURL, c.model, req.Thinking) {
		if reasoning := buildOpenAIReasoningConfig(req.Thinking); reasoning != nil {
			if c.isCodexEndpoint() {
				reasoning = applyCodexReasoningDefaults(reasoning)
			}
			payload["reasoning"] = reasoning
		}
	}
	if !c.isCodexEndpoint() {
		payload["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 && !c.isCodexEndpoint() {
		payload["max_output_tokens"] = req.MaxTokens
	}
	if c.isCodexEndpoint() {
		payload["instructions"] = instructions
	}
	if len(req.Tools) > 0 {
		payload["tools"] = convertCodexTools(req.Tools)
		payload["tool_choice"] = "auto"
	}
	if len(req.StopSequences) > 0 {
		payload["stop"] = append([]string(nil), req.StopSequences...)
	}

	body, err := jsonx.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	logBody := redactDataURIs(body)

	endpoint := c.baseURL + "/responses"
	c.logRequestMeta(prefix, "POST", endpoint)

	c.logger.Debug("%sRequest Body: %s", prefix, string(logBody))
	utils.LogStreamingRequestPayload(requestID, append([]byte(nil), logBody...))

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
		c.logger.Debug("%sError Response Body: %s", prefix, string(respBody))
		mappedErr := mapHTTPError(resp.StatusCode, respBody, resp.Header)
		c.logHTTPFailure(prefix, requestID, "stream", provider, endpoint, req, resp.StatusCode, resp.Header, respBody, mappedErr)
		return nil, mappedErr
	}

	type responsesStreamEvent struct {
		Type     string             `json:"type"`
		Delta    string             `json:"delta"`
		Message  string             `json:"message"`
		Code     string             `json:"code"`
		Response *responsesResponse `json:"response"`
		Item     *struct {
			Type      string `json:"type"`
			ID        string `json:"id"`
			CallID    string `json:"call_id"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
			Content   any    `json:"content"`
			Text      any    `json:"text"`
			Summary   any    `json:"summary"`
		} `json:"item"`
	}

	scanner := newStreamScanner(resp.Body)

	var contentBuilder strings.Builder
	var thinkingBuilder strings.Builder
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
		if err := jsonx.Unmarshal([]byte(data), &evt); err != nil {
			c.logger.Debug("%sFailed to decode stream event: %v", prefix, err)
			continue
		}

		thinkingDelta := ""
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
		case "response.reasoning.delta", "response.thinking.delta", "response.reasoning_summary.delta", "response.reasoning_summary_text.delta":
			thinkingDelta = evt.Delta
		case "response.output_item.done":
			if evt.Item != nil && evt.Item.Type == "function_call" {
				args := parseToolArguments([]byte(evt.Item.Arguments))
				toolID := evt.Item.CallID
				if utils.IsBlank(toolID) {
					toolID = evt.Item.ID
				}
				toolCalls = append(toolCalls, ports.ToolCall{
					ID:        toolID,
					Name:      evt.Item.Name,
					Arguments: args,
				})
			} else if evt.Item != nil && (evt.Item.Type == "reasoning" || evt.Item.Type == "thinking") {
				thinkingDelta = firstNonBlankString(
					textFromAny(evt.Item.Content),
					textFromAny(evt.Item.Text),
					textFromAny(evt.Item.Summary),
				)
			}
		case "response.completed", "response.incomplete":
			stopReason = evt.Type
			if evt.Response != nil {
				usage = ports.TokenUsage{
					PromptTokens:     evt.Response.Usage.InputTokens,
					CompletionTokens: evt.Response.Usage.OutputTokens,
					TotalTokens:      evt.Response.Usage.TotalTokens,
				}
				// Some providers include final output/thinking only inside the
				// response.completed payload rather than delta events.
				completedContent, completedToolCalls, completedThinking := parseResponsesOutput(*evt.Response)
				if contentBuilder.Len() == 0 && completedContent != "" {
					contentBuilder.WriteString(completedContent)
				}
				if len(toolCalls) == 0 && len(completedToolCalls) > 0 {
					toolCalls = completedToolCalls
				}
				if thinkingText := strings.TrimSpace(extractThinkingText(completedThinking)); thinkingText != "" {
					appendReasoningDelta(&thinkingBuilder, thinkingText)
				}
			}
		case "error":
			if evt.Message != "" {
				streamErr := fmt.Errorf("llm error: %s", evt.Message)
				c.logProcessingFailure(prefix, requestID, "stream", provider, endpoint, "stream_error_event", req, streamErr)
				return nil, streamErr
			}
			streamErr := fmt.Errorf("llm error: %s", string(data))
			c.logProcessingFailure(prefix, requestID, "stream", provider, endpoint, "stream_error_event", req, streamErr)
			return nil, streamErr
		}

		if utils.IsBlank(thinkingDelta) {
			thinkingDelta = extractResponsesReasoningDelta(evt.Type, data)
		}
		if thinkingDelta != "" {
			appendReasoningDelta(&thinkingBuilder, thinkingDelta)
		}
	}

	if err := scanner.Err(); err != nil {
		streamErr := fmt.Errorf("read stream: %w", err)
		c.logProcessingFailure(prefix, requestID, "stream", provider, endpoint, "read_stream", req, streamErr)
		return nil, streamErr
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
	if thinkingText := strings.TrimSpace(thinkingBuilder.String()); thinkingText != "" {
		result.Thinking = ports.Thinking{
			Parts: []ports.ThinkingPart{{Kind: "reasoning", Text: thinkingText}},
		}
	}

	if result.StopReason == "" {
		result.StopReason = "completed"
	}

	c.fireUsageCallback(result.Usage, "openai")

	respPayloadData := map[string]any{
		"content":     result.Content,
		"stop_reason": result.StopReason,
		"tool_calls":  result.ToolCalls,
		"usage":       result.Usage,
	}
	if len(result.Thinking.Parts) > 0 {
		respPayloadData["thinking"] = result.Thinking
	}
	if respPayload, err := jsonx.Marshal(respPayloadData); err != nil {
		c.logger.Debug("%sFailed to marshal streaming response payload: %v", prefix, err)
	} else {
		utils.LogStreamingResponsePayload(requestID, respPayload)
	}

	c.logResponseSummary(prefix, result)
	return result, nil
}

func extractResponsesReasoningDelta(eventType, rawEvent string) string {
	kind := utils.TrimLower(eventType)
	if kind == "" || (!strings.Contains(kind, "reasoning") && !strings.Contains(kind, "thinking")) {
		return ""
	}

	var payload map[string]any
	if err := jsonx.Unmarshal([]byte(rawEvent), &payload); err != nil {
		return ""
	}

	if delta := textFromAny(payload["delta"]); delta != "" {
		return delta
	}

	if item, ok := payload["item"].(map[string]any); ok {
		if text := firstNonBlankString(
			textFromAny(item["content"]),
			textFromAny(item["text"]),
			textFromAny(item["summary"]),
		); text != "" {
			return text
		}
	}

	if part, ok := payload["part"].(map[string]any); ok {
		if text := firstNonBlankString(
			textFromAny(part["content"]),
			textFromAny(part["text"]),
			textFromAny(part["summary"]),
		); text != "" {
			return text
		}
	}

	if summary := textFromAny(payload["summary"]); summary != "" {
		return summary
	}

	return ""
}

func firstNonBlankString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// appendReasoningDelta appends streaming reasoning text while skipping obvious
// duplicate blocks emitted by multiple reasoning event variants.
func appendReasoningDelta(builder *strings.Builder, delta string) {
	if builder == nil {
		return
	}
	normalized := strings.TrimSpace(delta)
	if normalized == "" {
		return
	}
	current := builder.String()
	if current != "" {
		if strings.HasSuffix(current, delta) {
			return
		}
		if len([]rune(normalized)) >= 24 && strings.Contains(current, normalized) {
			return
		}
	}
	builder.WriteString(delta)
}

func textFromAny(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case []any:
		var builder strings.Builder
		for _, item := range v {
			if text := textFromAny(item); text != "" {
				builder.WriteString(text)
			}
		}
		return strings.TrimSpace(builder.String())
	case map[string]any:
		return firstNonBlankString(
			textFromAny(v["content"]),
			textFromAny(v["text"]),
			textFromAny(v["summary"]),
			textFromAny(v["value"]),
			textFromAny(v["delta"]),
			textFromAny(v["output_text"]),
		)
	default:
		return ""
	}
}
