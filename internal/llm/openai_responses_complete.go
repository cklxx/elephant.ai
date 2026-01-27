package llm

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"alex/internal/agent/ports"
	"alex/internal/jsonx"
	"alex/internal/utils"
	id "alex/internal/utils/id"
)

func (c *openAIResponsesClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	if c.isCodexEndpoint() {
		return c.StreamComplete(ctx, req, ports.CompletionStreamCallbacks{})
	}

	requestID := extractRequestID(req.Metadata)
	if requestID == "" {
		requestID = id.NewRequestIDWithLogID(id.LogIDFromContext(ctx))
	}
	prefix := fmt.Sprintf("[req:%s] ", requestID)

	input, instructions := c.buildResponsesInputAndInstructions(req.Messages)
	payload := map[string]any{
		"model":       c.model,
		"input":       input,
		"temperature": req.Temperature,
		"stream":      false,
	}
	if req.MaxTokens > 0 && !c.isCodexEndpoint() {
		payload["max_output_tokens"] = req.MaxTokens
	}
	// Codex responses require instructions; opencode sets SystemPrompt.instructions() for codex.
	// Sources:
	// - packages/opencode/src/session/portsllm.ts (isCodex -> options.instructions)
	// - packages/opencode/src/session/system.ts (SystemPrompt.instructions)
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
		var loggedValue string
		switch strings.ToLower(k) {
		case "authorization":
			loggedValue = "Bearer (hidden)"
		case "cookie", "set-cookie", "x-api-key", "x-auth-token", "x-amz-security-token", "x-amz-security-token-expires":
			loggedValue = "(hidden)"
		default:
			loggedValue = strings.Join(v, ", ")
		}
		c.logger.Debug("%s  %s: %s", prefix, k, loggedValue)
	}

	c.logger.Debug("%sRequest Body: %s", prefix, string(logBody))
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

	c.logger.Debug("%sResponse Body: %s", prefix, string(respBody))
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
