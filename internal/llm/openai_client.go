package llm

import (
	"alex/internal/agent/ports"
	alexerrors "alex/internal/errors"
	"alex/internal/utils"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// OpenAI API compatible client
type openaiClient struct {
	model         string
	apiKey        string
	baseURL       string
	httpClient    *http.Client
	logger        *utils.Logger
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

	return &openaiClient{
		model:   model,
		apiKey:  config.APIKey,
		baseURL: strings.TrimRight(config.BaseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger:     utils.NewComponentLogger("llm"),
		headers:    config.Headers,
		maxRetries: config.MaxRetries,
	}, nil
}

func (c *openaiClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
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

	// Debug log: Request details
	c.logger.Debug("=== LLM Request ===")
	c.logger.Debug("URL: POST %s/chat/completions", c.baseURL)
	c.logger.Debug("Model: %s", c.model)

	endpoint := c.baseURL + "/chat/completions"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	if c.maxRetries > 0 {
		httpReq.Header.Set("X-Retry-Limit", strconv.Itoa(c.maxRetries))
	}

	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}

	// Debug log: Request headers
	c.logger.Debug("Request Headers:")
	for k, v := range httpReq.Header {
		if k == "Authorization" {
			// Mask API key for security
			c.logger.Debug("  %s: Bearer (hidden)", k)
		} else {
			c.logger.Debug("  %s: %s", k, strings.Join(v, ", "))
		}
	}

	// Debug log: Request body (pretty print)
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, body, "", "  "); err == nil {
		c.logger.Debug("Request Body:\n%s", prettyJSON.String())
	} else {
		c.logger.Debug("Request Body: %s", string(body))
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Debug("HTTP request failed: %v", err)
		return nil, c.wrapRequestError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Debug log: Response status
	c.logger.Debug("=== LLM Response ===")
	c.logger.Debug("Status: %d %s", resp.StatusCode, resp.Status)

	// Debug log: Response headers
	c.logger.Debug("Response Headers:")
	for k, v := range resp.Header {
		c.logger.Debug("  %s: %s", k, strings.Join(v, ", "))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Debug("Failed to read response body: %v", err)
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.logger.Debug("Error Response Body: %s", string(respBody))
		return nil, c.mapHTTPError(resp.StatusCode, respBody, resp.Header)
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
			Type    string `json:"type"`
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	// Debug log: Response body (pretty print)
	var prettyResp bytes.Buffer
	if err := json.Indent(&prettyResp, respBody, "", "  "); err == nil {
		c.logger.Debug("Response Body:\n%s", prettyResp.String())
	} else {
		c.logger.Debug("Response Body: %s", string(respBody))
	}

	if err := json.Unmarshal(respBody, &oaiResp); err != nil {
		c.logger.Debug("Failed to decode response: %v", err)
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if oaiResp.Error != nil && oaiResp.Error.Message != "" {
		errMsg := oaiResp.Error.Message
		if oaiResp.Error.Type != "" {
			errMsg = fmt.Sprintf("%s: %s", oaiResp.Error.Type, oaiResp.Error.Message)
		}
		return nil, c.mapHTTPError(resp.StatusCode, []byte(errMsg), resp.Header)
	}

	if len(oaiResp.Choices) == 0 {
		c.logger.Debug("No choices in response")
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
	}

	// Trigger usage callback if set
	if c.usageCallback != nil {
		provider := "openrouter"
		if strings.Contains(c.baseURL, "api.openai.com") {
			provider = "openai"
		} else if strings.Contains(c.baseURL, "api.deepseek.com") {
			provider = "deepseek"
		}
		c.usageCallback(result.Usage, c.model, provider)
	}

	// Convert tool calls
	for _, tc := range oaiResp.Choices[0].Message.ToolCalls {
		var args map[string]any
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			c.logger.Debug("Failed to parse tool call arguments: %v", err)
			continue
		}
		result.ToolCalls = append(result.ToolCalls, ports.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: args,
		})
	}

	// Debug log: Summary
	c.logger.Debug("=== LLM Response Summary ===")
	c.logger.Debug("Stop Reason: %s", result.StopReason)
	c.logger.Debug("Content Length: %d chars", len(result.Content))
	c.logger.Debug("Tool Calls: %d", len(result.ToolCalls))
	c.logger.Debug("Usage: %d prompt + %d completion = %d total tokens",
		result.Usage.PromptTokens,
		result.Usage.CompletionTokens,
		result.Usage.TotalTokens)
	c.logger.Debug("==================")

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
	result := make([]map[string]any, len(msgs))
	for i, msg := range msgs {
		result[i] = map[string]any{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}
	return result
}

func (c *openaiClient) convertTools(tools []ports.ToolDefinition) []map[string]any {
	result := make([]map[string]any, len(tools))
	for i, tool := range tools {
		result[i] = map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.Parameters,
			},
		}
	}
	return result
}

func (c *openaiClient) wrapRequestError(err error) error {
	if errors.Is(err, context.Canceled) {
		return err
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return alexerrors.NewTransientError(err, "Request to LLM provider timed out. Please retry.")
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return alexerrors.NewTransientError(err, "Request to LLM provider timed out. Please retry.")
	}

	return alexerrors.NewTransientError(err, "Failed to reach LLM provider. Please retry shortly.")
}

func (c *openaiClient) mapHTTPError(status int, body []byte, headers http.Header) error {
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = http.StatusText(status)
	}

	baseErr := fmt.Errorf("status %d: %s", status, message)

	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		perr := alexerrors.NewPermanentError(baseErr, "Authentication failed. Please verify your API key.")
		perr.StatusCode = status
		return perr
	case status == http.StatusTooManyRequests:
		terr := alexerrors.NewTransientError(baseErr, "Rate limit reached. The system will retry automatically.")
		terr.StatusCode = status
		if retryAfter := parseRetryAfter(headers.Get("Retry-After")); retryAfter > 0 {
			terr.RetryAfter = retryAfter
		}
		return terr
	case status == http.StatusRequestTimeout || status == http.StatusGatewayTimeout:
		terr := alexerrors.NewTransientError(baseErr, "Upstream service timed out. Please retry.")
		terr.StatusCode = status
		return terr
	case status >= 500:
		terr := alexerrors.NewTransientError(baseErr, "Upstream service temporarily unavailable. Please retry.")
		terr.StatusCode = status
		return terr
	case status >= 400:
		perr := alexerrors.NewPermanentError(baseErr, "Request was rejected by the upstream service.")
		perr.StatusCode = status
		return perr
	default:
		terr := alexerrors.NewTransientError(baseErr, "Unexpected response from upstream service. Please retry.")
		terr.StatusCode = status
		return terr
	}
}

func parseRetryAfter(value string) int {
	if value == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			return 0
		}
		return seconds
	}

	if t, err := http.ParseTime(value); err == nil {
		delta := int(time.Until(t).Seconds())
		if delta < 0 {
			return 0
		}
		return delta
	}

	return 0
}
