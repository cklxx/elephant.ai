package llm

import (
	"alex/internal/agent/ports"
	"alex/internal/utils"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAI API compatible client
type openaiClient struct {
	model      string
	apiKey     string
	baseURL    string
	httpClient *http.Client
	logger     *utils.Logger
}

func NewOpenAIClient(model string, config Config) (ports.LLMClient, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://openrouter.ai/api/v1"
	}

	return &openaiClient{
		model:   model,
		apiKey:  config.APIKey,
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		logger: utils.NewComponentLogger("llm"),
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

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Debug log: Request headers
	c.logger.Debug("Request Headers:")
	for k, v := range httpReq.Header {
		if k == "Authorization" {
			// Mask API key for security
			c.logger.Debug("  %s: Bearer %s...%s", k, c.apiKey[:8], c.apiKey[len(c.apiKey)-4:])
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
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	// Debug log: Response status
	c.logger.Debug("=== LLM Response ===")
	c.logger.Debug("Status: %d %s", resp.StatusCode, resp.Status)

	// Debug log: Response headers
	c.logger.Debug("Response Headers:")
	for k, v := range resp.Header {
		c.logger.Debug("  %s: %s", k, strings.Join(v, ", "))
	}

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		c.logger.Debug("Error Response Body: %s", string(bodyBytes))
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
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
	}

	// Read response body for both decoding and logging
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Debug("Failed to read response body: %v", err)
		return nil, fmt.Errorf("read response: %w", err)
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

	if len(oaiResp.Choices) == 0 {
		c.logger.Debug("No choices in response")
		return nil, fmt.Errorf("no choices in response")
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
