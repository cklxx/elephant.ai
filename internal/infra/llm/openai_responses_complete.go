package llm

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	"alex/internal/shared/json"
	"alex/internal/shared/utils"
)

func (c *openAIResponsesClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	if c.isCodexEndpoint() {
		return c.StreamComplete(ctx, req, ports.CompletionStreamCallbacks{})
	}

	requestID, prefix := c.buildLogPrefix(ctx, req.Metadata)

	input, instructions := c.buildResponsesInputAndInstructions(req.Messages)
	var droppedCallIDs []string
	input, droppedCallIDs = pruneOrphanFunctionCallOutputs(input)
	if len(droppedCallIDs) > 0 {
		c.logger.Warn("%sDropped %d orphan function_call_output item(s): %s", prefix, len(droppedCallIDs), strings.Join(droppedCallIDs, ", "))
	}
	payload := map[string]any{
		"model":       c.model,
		"input":       input,
		"temperature": req.Temperature,
		"stream":      false,
	}
	if shouldSendOpenAIReasoning(c.baseURL, c.model, req.Thinking) {
		if reasoning := buildOpenAIReasoningConfig(req.Thinking); reasoning != nil {
			payload["reasoning"] = reasoning
		}
	}
	if req.MaxTokens > 0 && !c.isCodexEndpoint() {
		payload["max_output_tokens"] = req.MaxTokens
	}
	if c.isCodexEndpoint() {
		payload["instructions"] = instructions
	}
	payload["store"] = false

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

	c.logResponseStatus(prefix, resp)

	respBody, err := readResponseBody(resp.Body)
	if err != nil {
		c.logger.Debug("%sFailed to read response body: %v", prefix, err)
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.logger.Debug("%sError Response Body: %s", prefix, string(respBody))
		return nil, mapHTTPError(resp.StatusCode, respBody, resp.Header)
	}

	var apiResp responsesResponse
	if err := jsonx.Unmarshal(respBody, &apiResp); err != nil {
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

	content, toolCalls, thinking := parseResponsesOutput(apiResp)

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
	if len(thinking.Parts) > 0 {
		result.Thinking = thinking
	}

	c.fireUsageCallback(result.Usage, "openai")

	c.logger.Debug("%sResponse Body: %s", prefix, string(respBody))
	utils.LogStreamingResponsePayload(requestID, append([]byte(nil), respBody...))

	c.logResponseSummary(prefix, result)
	return result, nil
}
