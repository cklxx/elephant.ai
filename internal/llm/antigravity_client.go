package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"alex/internal/agent/ports"
	alexerrors "alex/internal/errors"
	"alex/internal/httpclient"
	"alex/internal/logging"
	"alex/internal/utils"
	id "alex/internal/utils/id"
)

const antigravityBaseURL = "https://cloudcode-pa.googleapis.com"

// Antigravity client uses Gemini-style requests via cloudcode-pa.
type antigravityClient struct {
	model         string
	apiKey        string
	baseURL       string
	httpClient    *http.Client
	logger        logging.Logger
	headers       map[string]string
	maxRetries    int
	usageCallback func(usage ports.TokenUsage, model string, provider string)
}

// NewAntigravityClient constructs an LLM client for Antigravity (Gemini CLI API).
func NewAntigravityClient(model string, config Config) (ports.LLMClient, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		baseURL = antigravityBaseURL
	}

	timeout := 120 * time.Second
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}

	logger := utils.NewCategorizedLogger(utils.LogCategoryLLM, "antigravity")

	return &antigravityClient{
		model:      model,
		apiKey:     config.APIKey,
		baseURL:    baseURL,
		httpClient: httpclient.New(timeout, logger),
		logger:     logger,
		headers:    config.Headers,
		maxRetries: config.MaxRetries,
	}, nil
}

func (c *antigravityClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	requestID := extractRequestID(req.Metadata)
	if requestID == "" {
		requestID = id.NewRequestID()
	}
	prefix := fmt.Sprintf("[req:%s] ", requestID)

	payload := buildAntigravityPayload(req, requestID, c.model)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	logBody := redactDataURIs(body)

	c.logger.Debug("%s=== LLM Request ===", prefix)
	c.logger.Debug("%sURL: POST %s/v1internal:generateContent", prefix, c.baseURL)
	c.logger.Debug("%sModel: %s", prefix, c.model)

	endpoint := c.baseURL + "/v1internal:generateContent"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if c.maxRetries > 0 {
		httpReq.Header.Set("X-Retry-Limit", fmt.Sprintf("%d", c.maxRetries))
	}
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}

	c.logger.Debug("%sRequest Headers:", prefix)
	for k := range httpReq.Header {
		if k == "Authorization" {
			c.logger.Debug("%s  %s: Bearer (hidden)", prefix, k)
		} else {
			c.logger.Debug("%s  %s: <redacted>", prefix, k)
		}
	}

	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, logBody, "", "  "); err == nil {
		c.logger.Debug("%sRequest Body:\n%s", prefix, prettyJSON.String())
	} else {
		c.logger.Debug("%sRequest Body: %s", prefix, string(logBody))
	}

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

	result, err := parseAntigravityResponse(respBody, requestID)
	if err != nil {
		return nil, err
	}

	if c.usageCallback != nil {
		c.usageCallback(result.Usage, c.model, "antigravity")
	}

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

func (c *antigravityClient) Model() string {
	return c.model
}

// SetUsageCallback implements UsageTrackingClient.
func (c *antigravityClient) SetUsageCallback(callback func(usage ports.TokenUsage, model string, provider string)) {
	c.usageCallback = callback
}

func buildAntigravityPayload(req ports.CompletionRequest, requestID, model string) map[string]any {
	contents, systemInstruction := convertAntigravityMessages(req.Messages)
	request := map[string]any{
		"contents": contents,
	}
	if systemInstruction != nil {
		request["systemInstruction"] = systemInstruction
	}

	generationConfig := map[string]any{}
	if req.Temperature != 0 {
		generationConfig["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		generationConfig["maxOutputTokens"] = req.MaxTokens
	}
	if req.TopP > 0 {
		generationConfig["topP"] = req.TopP
	}
	if len(req.StopSequences) > 0 {
		generationConfig["stopSequences"] = append([]string(nil), req.StopSequences...)
	}
	if len(generationConfig) > 0 {
		request["generationConfig"] = generationConfig
	}

	tools := convertAntigravityTools(req.Tools)
	if len(tools) > 0 {
		request["tools"] = tools
		request["toolConfig"] = map[string]any{
			"functionCallingConfig": map[string]any{"mode": "AUTO"},
		}
	}

	request["safetySettings"] = defaultAntigravitySafetySettings()

	return map[string]any{
		"project":     generateAntigravityProjectID(),
		"requestId":   requestID,
		"request":     request,
		"model":       model,
		"userAgent":   "antigravity",
		"requestType": "agent",
	}
}

func convertAntigravityMessages(msgs []ports.Message) ([]map[string]any, map[string]any) {
	contents := make([]map[string]any, 0, len(msgs))
	systemTexts := make([]string, 0)
	toolNames := map[string]string{}

	for _, msg := range msgs {
		if msg.Source == ports.MessageSourceDebug || msg.Source == ports.MessageSourceEvaluation {
			continue
		}
		for _, call := range msg.ToolCalls {
			if isValidToolName(call.Name) {
				toolNames[call.ID] = call.Name
			}
		}
	}

	for _, msg := range msgs {
		if msg.Source == ports.MessageSourceDebug || msg.Source == ports.MessageSourceEvaluation {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		if role == "system" {
			text := strings.TrimSpace(msg.Content)
			if text != "" {
				systemTexts = append(systemTexts, text)
			}
			continue
		}

		parts := convertAntigravityContentParts(msg)
		switch role {
		case "assistant":
			parts = appendAntigravityToolCalls(parts, msg.ToolCalls)
			if len(parts) > 0 {
				contents = append(contents, map[string]any{
					"role":  "model",
					"parts": parts,
				})
			}
		case "tool":
			if msg.ToolCallID == "" {
				continue
			}
			name := toolNames[msg.ToolCallID]
			part := buildAntigravityToolResponse(msg.ToolCallID, name, msg.Content)
			contents = append(contents, map[string]any{
				"role":  "user",
				"parts": []map[string]any{part},
			})
		default:
			if len(parts) > 0 {
				contents = append(contents, map[string]any{
					"role":  "user",
					"parts": parts,
				})
			}
		}
	}

	var systemInstruction map[string]any
	if len(systemTexts) > 0 {
		systemInstruction = map[string]any{
			"role": "user",
			"parts": []map[string]any{{
				"text": strings.Join(systemTexts, "\n\n"),
			}},
		}
	}

	return contents, systemInstruction
}

func convertAntigravityContentParts(msg ports.Message) []map[string]any {
	content := buildMessageContent(msg, shouldEmbedAttachmentsInContent(msg))
	switch value := content.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return nil
		}
		return []map[string]any{{"text": value}}
	case []map[string]any:
		parts := make([]map[string]any, 0, len(value))
		for _, item := range value {
			kind, _ := item["type"].(string)
			switch kind {
			case "text":
				text, _ := item["text"].(string)
				if strings.TrimSpace(text) != "" {
					parts = append(parts, map[string]any{"text": text})
				}
			case "image_url":
				url := ""
				if imageURL, ok := item["image_url"].(map[string]any); ok {
					url, _ = imageURL["url"].(string)
				}
				if url == "" {
					continue
				}
				mimeType, data, ok := parseAntigravityDataURL(url)
				if ok {
					parts = append(parts, map[string]any{
						"inlineData": map[string]any{
							"mimeType": mimeType,
							"data":     data,
						},
					})
				} else {
					parts = append(parts, map[string]any{"text": url})
				}
			}
		}
		return parts
	default:
		return nil
	}
}

func appendAntigravityToolCalls(parts []map[string]any, calls []ports.ToolCall) []map[string]any {
	for _, call := range calls {
		if !isValidToolName(call.Name) {
			continue
		}
		args := call.Arguments
		if args == nil {
			args = map[string]any{}
		}
		functionCall := map[string]any{
			"name": call.Name,
			"args": args,
		}
		if call.ID != "" {
			functionCall["id"] = call.ID
		}
		parts = append(parts, map[string]any{
			"functionCall": functionCall,
		})
	}
	return parts
}

func buildAntigravityToolResponse(callID, name, content string) map[string]any {
	resultValue := parseJSONValue(content)
	response := map[string]any{
		"result": resultValue,
	}
	functionResponse := map[string]any{
		"name":     name,
		"response": response,
	}
	if callID != "" {
		functionResponse["id"] = callID
	}
	return map[string]any{"functionResponse": functionResponse}
}

func convertAntigravityTools(tools []ports.ToolDefinition) []map[string]any {
	if len(tools) == 0 {
		return nil
	}
	declarations := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		if !isValidToolName(tool.Name) {
			continue
		}
		var schema map[string]any
		if data, err := json.Marshal(tool.Parameters); err == nil {
			_ = json.Unmarshal(data, &schema)
		}
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		declarations = append(declarations, map[string]any{
			"name":                 tool.Name,
			"description":          tool.Description,
			"parametersJsonSchema": schema,
		})
	}
	if len(declarations) == 0 {
		return nil
	}
	return []map[string]any{{"functionDeclarations": declarations}}
}

func parseAntigravityResponse(body []byte, requestID string) (*ports.CompletionResponse, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	resp := payload
	if wrapped, ok := payload["response"].(map[string]any); ok {
		resp = wrapped
	}

	candidates, ok := resp["candidates"].([]any)
	if !ok || len(candidates) == 0 {
		return nil, alexerrors.NewTransientError(errors.New("no candidates in response"), "LLM returned an empty response. Please retry.")
	}

	first, ok := candidates[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid candidate payload")
	}

	content := ""
	toolCalls := make([]ports.ToolCall, 0)
	if contentObj, ok := first["content"].(map[string]any); ok {
		if parts, ok := contentObj["parts"].([]any); ok {
			for _, part := range parts {
				partObj, ok := part.(map[string]any)
				if !ok {
					continue
				}
				if thought, ok := partObj["thought"].(bool); ok && thought {
					continue
				}
				if text, ok := partObj["text"].(string); ok {
					content += text
				}
				if fc, ok := partObj["functionCall"].(map[string]any); ok {
					name, _ := fc["name"].(string)
					args := map[string]any{}
					switch v := fc["args"].(type) {
					case map[string]any:
						args = v
					case string:
						_ = json.Unmarshal([]byte(v), &args)
					}
					callID := ""
					if idValue, ok := fc["id"].(string); ok {
						callID = idValue
					}
					if callID == "" {
						callID = fmt.Sprintf("call_%s", id.NewKSUID())
					}
					toolCalls = append(toolCalls, ports.ToolCall{ID: callID, Name: name, Arguments: args})
				}
				if inline, ok := partObj["inlineData"].(map[string]any); ok {
					data, _ := inline["data"].(string)
					mimeType, _ := inline["mimeType"].(string)
					if data != "" {
						if mimeType == "" {
							mimeType = "image/png"
						}
						if content != "" {
							content += "\n\n"
						}
						content += fmt.Sprintf("![image](data:%s;base64,%s)", mimeType, data)
					}
				}
			}
		}
	}

	finishReason := ""
	if fr, ok := first["finishReason"].(string); ok {
		switch strings.ToUpper(strings.TrimSpace(fr)) {
		case "STOP":
			finishReason = "stop"
		case "MAX_TOKENS":
			finishReason = "length"
		case "SAFETY", "RECITATION":
			finishReason = "content_filter"
		default:
			finishReason = "stop"
		}
	}
	if finishReason == "" && len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	usage := ports.TokenUsage{}
	if meta, ok := resp["usageMetadata"].(map[string]any); ok {
		usage.PromptTokens = readInt(meta["promptTokenCount"])
		usage.CompletionTokens = readInt(meta["candidatesTokenCount"])
		usage.TotalTokens = readInt(meta["totalTokenCount"])
	}

	return &ports.CompletionResponse{
		Content:    content,
		ToolCalls:  toolCalls,
		StopReason: finishReason,
		Usage:      usage,
		Metadata: map[string]any{
			"request_id": requestID,
		},
	}, nil
}

func parseAntigravityDataURL(value string) (string, string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", "", false
	}
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "data:") {
		return "", "", false
	}
	comma := strings.Index(trimmed, ",")
	if comma < 0 {
		return "", "", false
	}
	meta := trimmed[5:comma]
	parts := strings.Split(meta, ";")
	mimeType := strings.TrimSpace(parts[0])
	data := strings.TrimSpace(trimmed[comma+1:])
	if data == "" {
		return "", "", false
	}
	if mimeType == "" {
		mimeType = "image/png"
	}
	return mimeType, data, true
}

func parseJSONValue(content string) any {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return map[string]any{}
	}
	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
		return parsed
	}
	return trimmed
}

func readInt(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return int(parsed)
		}
	}
	return 0
}

func defaultAntigravitySafetySettings() []map[string]any {
	return []map[string]any{
		{"category": "HARM_CATEGORY_HARASSMENT", "threshold": "OFF"},
		{"category": "HARM_CATEGORY_HATE_SPEECH", "threshold": "OFF"},
		{"category": "HARM_CATEGORY_SEXUALLY_EXPLICIT", "threshold": "OFF"},
		{"category": "HARM_CATEGORY_DANGEROUS_CONTENT", "threshold": "OFF"},
		{"category": "HARM_CATEGORY_CIVIC_INTEGRITY", "threshold": "BLOCK_NONE"},
	}
}

func generateAntigravityProjectID() string {
	return fmt.Sprintf("project-%s", id.NewKSUID())
}
