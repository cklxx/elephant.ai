package llm

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	"alex/internal/jsonx"
	"alex/internal/utils"
)

func (c *openAIResponsesClient) StreamComplete(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	requestID, prefix := c.buildLogPrefix(ctx, req.Metadata)

	input, instructions := c.buildResponsesInputAndInstructions(req.Messages)
	payload := map[string]any{
		"model":  c.model,
		"input":  input,
		"stream": true,
		"store":  false,
	}
	if shouldSendOpenAIReasoning(c.baseURL, c.model, req.Thinking) {
		if reasoning := buildOpenAIReasoningConfig(req.Thinking); reasoning != nil {
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
		return nil, wrapRequestError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	c.logger.Debug("%s=== LLM Streaming Response ===", prefix)
	c.logResponseStatus(prefix, resp)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, readErr := readResponseBody(resp.Body)
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
			Content   string `json:"content"`
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
		case "response.reasoning.delta", "response.thinking.delta":
			if evt.Delta != "" {
				thinkingBuilder.WriteString(evt.Delta)
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
			} else if evt.Item != nil && (evt.Item.Type == "reasoning" || evt.Item.Type == "thinking") {
				if evt.Item.Content != "" {
					thinkingBuilder.WriteString(evt.Item.Content)
				}
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
	if thinkingText := strings.TrimSpace(thinkingBuilder.String()); thinkingText != "" {
		result.Thinking = ports.Thinking{
			Parts: []ports.ThinkingPart{{Kind: "reasoning", Text: thinkingText}},
		}
	}

	if result.StopReason == "" {
		result.StopReason = "completed"
	}

	c.fireUsageCallback(result.Usage, "openai")

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
