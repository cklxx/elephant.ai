package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	alexerrors "alex/internal/shared/errors"
	jsonx "alex/internal/shared/json"
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
