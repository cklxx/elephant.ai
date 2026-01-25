package llm

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/httpclient"
	"alex/internal/jsonx"
	"alex/internal/logging"
	"alex/internal/utils"
	id "alex/internal/utils/id"
)

const (
	defaultAnthropicBaseURL     = "https://api.anthropic.com/v1"
	defaultAnthropicVersion     = "2023-06-01"
	anthropicToolsBetaHeader    = "tools-2024-04-04"
	anthropicOAuthBetaHeader    = "oauth-2025-04-20"
	anthropicVersionHeaderKey   = "anthropic-version"
	anthropicBetaHeaderKey      = "anthropic-beta"
	anthropicRequestHeaderKey   = "x-api-key"
	anthropicMessagesPath       = "/messages"
	anthropicRequestContentType = "application/json"
)

type anthropicClient struct {
	model         string
	apiKey        string
	baseURL       string
	httpClient    *http.Client
	logger        logging.Logger
	headers       map[string]string
	maxRetries    int
	usageCallback func(usage ports.TokenUsage, model string, provider string)
}

func NewAnthropicClient(model string, config Config) (ports.LLMClient, error) {
	if config.BaseURL == "" {
		config.BaseURL = defaultAnthropicBaseURL
	}

	timeout := 120 * time.Second
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}

	logger := utils.NewCategorizedLogger(utils.LogCategoryLLM, "anthropic")

	return &anthropicClient{
		model:      model,
		apiKey:     config.APIKey,
		baseURL:    strings.TrimRight(config.BaseURL, "/"),
		httpClient: httpclient.New(timeout, logger),
		logger:     logger,
		headers:    config.Headers,
		maxRetries: config.MaxRetries,
	}, nil
}

func (c *anthropicClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	requestID := extractRequestID(req.Metadata)
	if requestID == "" {
		requestID = id.NewRequestID()
	}
	prefix := fmt.Sprintf("[req:%s] ", requestID)

	messages, system := c.convertMessages(req.Messages)
	payload := map[string]any{
		"model":      c.model,
		"max_tokens": req.MaxTokens,
		"messages":   messages,
	}
	if system != "" {
		payload["system"] = system
	}
	payload["temperature"] = req.Temperature
	if len(req.StopSequences) > 0 {
		payload["stop_sequences"] = append([]string(nil), req.StopSequences...)
	}
	if len(req.Tools) > 0 {
		payload["tools"] = convertAnthropicTools(req.Tools)
	}

	body, err := jsonx.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	logBody := redactDataURIs(body)

	c.logger.Debug("%s=== LLM Request ===", prefix)
	c.logger.Debug("%sURL: POST %s%s", prefix, c.baseURL, anthropicMessagesPath)
	c.logger.Debug("%sModel: %s", prefix, c.model)

	endpoint := c.baseURL + anthropicMessagesPath
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", anthropicRequestContentType)
	if c.maxRetries > 0 {
		httpReq.Header.Set("X-Retry-Limit", strconv.Itoa(c.maxRetries))
	}
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}

	hasAuthorization := strings.TrimSpace(httpReq.Header.Get("Authorization")) != ""
	hasAPIKeyHeader := strings.TrimSpace(httpReq.Header.Get(anthropicRequestHeaderKey)) != ""
	usesOAuth := hasAuthorization
	if !hasAuthorization && !hasAPIKeyHeader && c.apiKey != "" {
		if isAnthropicOAuthToken(c.apiKey) {
			httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
			usesOAuth = true
		} else {
			httpReq.Header.Set(anthropicRequestHeaderKey, c.apiKey)
		}
	}

	if httpReq.Header.Get(anthropicVersionHeaderKey) == "" {
		httpReq.Header.Set(anthropicVersionHeaderKey, defaultAnthropicVersion)
	}

	var betaValues []string
	if usesOAuth {
		betaValues = append(betaValues, anthropicOAuthBetaHeader)
	}
	if len(req.Tools) > 0 {
		betaValues = append(betaValues, anthropicToolsBetaHeader)
	}
	if len(betaValues) > 0 {
		httpReq.Header.Set(
			anthropicBetaHeaderKey,
			mergeAnthropicBetaValues(httpReq.Header.Get(anthropicBetaHeaderKey), betaValues...),
		)
	}

	c.logger.Debug("%sRequest Headers:", prefix)
	for k, v := range httpReq.Header {
		if strings.EqualFold(k, anthropicRequestHeaderKey) || strings.EqualFold(k, "Authorization") {
			c.logger.Debug("%s  %s: (hidden)", prefix, k)
		} else {
			// Avoid logging potentially sensitive header values; only indicate presence.
			_ = v
			c.logger.Debug("%s  %s: (present)", prefix, k)
		}
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

	var apiResp anthropicResponse
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

	content, toolCalls := parseAnthropicContent(apiResp.Content)
	usage := ports.TokenUsage{
		PromptTokens:     apiResp.Usage.InputTokens,
		CompletionTokens: apiResp.Usage.OutputTokens,
		TotalTokens:      apiResp.Usage.InputTokens + apiResp.Usage.OutputTokens,
	}

	result := &ports.CompletionResponse{
		Content:    content,
		StopReason: apiResp.StopReason,
		ToolCalls:  toolCalls,
		Usage:      usage,
		Metadata: map[string]any{
			"request_id": requestID,
			"message_id": strings.TrimSpace(apiResp.ID),
		},
	}

	if c.usageCallback != nil {
		c.usageCallback(result.Usage, c.model, "anthropic")
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

func (c *anthropicClient) Model() string {
	return c.model
}

func (c *anthropicClient) SetUsageCallback(callback func(usage ports.TokenUsage, model string, provider string)) {
	c.usageCallback = callback
}

func (c *anthropicClient) convertMessages(msgs []ports.Message) ([]anthropicMessage, string) {
	messages := make([]anthropicMessage, 0, len(msgs))
	var systemParts []string

	for _, msg := range msgs {
		if msg.Source == ports.MessageSourceDebug || msg.Source == ports.MessageSourceEvaluation {
			continue
		}

		role := strings.ToLower(strings.TrimSpace(msg.Role))
		if role == "" {
			continue
		}

		switch role {
		case "system":
			if strings.TrimSpace(msg.Content) != "" {
				systemParts = append(systemParts, msg.Content)
			}
			continue
		case "tool":
			if strings.TrimSpace(msg.ToolCallID) == "" {
				continue
			}
			block := anthropicContentBlock{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   msg.Content,
			}
			messages = append(messages, anthropicMessage{
				Role:    "user",
				Content: []anthropicContentBlock{block},
			})
			continue
		}

		contentBlocks := buildAnthropicMessageContent(msg, shouldEmbedAttachmentsInContent(msg))
		if len(msg.ToolCalls) > 0 {
			for _, call := range msg.ToolCalls {
				contentBlocks = append(contentBlocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    call.ID,
					Name:  call.Name,
					Input: normalizeToolArguments(call.Arguments),
				})
			}
		}

		if len(contentBlocks) == 0 && strings.TrimSpace(msg.Content) == "" {
			continue
		}

		messages = append(messages, anthropicMessage{
			Role:    role,
			Content: contentBlocks,
		})
	}

	return messages, strings.Join(systemParts, "\n\n")
}

func buildAnthropicMessageContent(msg ports.Message, embedAttachments bool) []anthropicContentBlock {
	if len(msg.Attachments) == 0 || !embedAttachments {
		if strings.TrimSpace(msg.Content) == "" {
			return nil
		}
		return []anthropicContentBlock{{Type: "text", Text: msg.Content}}
	}

	index := buildAttachmentIndex(msg.Attachments)

	var parts []anthropicContentBlock
	used := make(map[string]bool)
	hasImage := false

	appendText := func(text string) {
		if text == "" {
			return
		}
		parts = append(parts, anthropicContentBlock{
			Type: "text",
			Text: text,
		})
	}

	appendImage := func(att ports.Attachment, placeholder string) {
		data := ports.AttachmentInlineBase64(att)
		if data == "" {
			return
		}
		hasImage = true
		parts = append(parts, anthropicContentBlock{
			Type: "image",
			Source: &anthropicImageSource{
				Type:      "base64",
				MediaType: inferAttachmentMediaType(att, placeholder),
				Data:      data,
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
			appendImage(att, key)
			used[key] = true
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
		appendText("[" + key + "]")
		appendImage(desc.Attachment, key)
		used[key] = true
	}

	if !hasImage {
		if strings.TrimSpace(msg.Content) == "" {
			return nil
		}
		return []anthropicContentBlock{{Type: "text", Text: msg.Content}}
	}

	return parts
}

func inferAttachmentMediaType(att ports.Attachment, placeholder string) string {
	if mediaType := strings.TrimSpace(att.MediaType); mediaType != "" {
		return mediaType
	}
	name := strings.TrimSpace(att.Name)
	if name == "" {
		name = strings.TrimSpace(placeholder)
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext != "" {
		if guessed := mime.TypeByExtension(ext); guessed != "" {
			return guessed
		}
		switch ext {
		case ".png":
			return "image/png"
		case ".jpg", ".jpeg":
			return "image/jpeg"
		case ".gif":
			return "image/gif"
		case ".webp":
			return "image/webp"
		}
	}
	return "image/png"
}

func normalizeToolArguments(args map[string]any) map[string]any {
	if args == nil {
		return map[string]any{}
	}
	return args
}

func convertAnthropicTools(tools []ports.ToolDefinition) []map[string]any {
	result := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		if !isValidToolName(tool.Name) {
			continue
		}
		result = append(result, map[string]any{
			"name":         tool.Name,
			"description":  tool.Description,
			"input_schema": tool.Parameters,
		})
	}
	return result
}

type anthropicMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

type anthropicContentBlock struct {
	Type      string                `json:"type"`
	Text      string                `json:"text,omitempty"`
	ID        string                `json:"id,omitempty"`
	Name      string                `json:"name,omitempty"`
	Input     map[string]any        `json:"input,omitempty"`
	ToolUseID string                `json:"tool_use_id,omitempty"`
	Content   any                   `json:"content,omitempty"`
	Source    *anthropicImageSource `json:"source,omitempty"`
}

type anthropicImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

type anthropicResponse struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type"`
	Role       string                  `json:"role"`
	Content    []anthropicContentBlock `json:"content"`
	StopReason string                  `json:"stop_reason"`
	Usage      anthropicUsage          `json:"usage"`
	Error      *anthropicError         `json:"error"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func parseAnthropicContent(blocks []anthropicContentBlock) (string, []ports.ToolCall) {
	var contentBuilder strings.Builder
	var toolCalls []ports.ToolCall

	for _, block := range blocks {
		switch strings.ToLower(strings.TrimSpace(block.Type)) {
		case "text":
			contentBuilder.WriteString(block.Text)
		case "tool_use":
			toolCalls = append(toolCalls, ports.ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: normalizeToolArguments(block.Input),
			})
		}
	}

	return contentBuilder.String(), toolCalls
}

func isAnthropicOAuthToken(token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	lower := strings.ToLower(token)
	return !strings.HasPrefix(lower, "sk-")
}

func mergeAnthropicBetaValues(existing string, values ...string) string {
	seen := map[string]struct{}{}
	var merged []string

	appendValue := func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		merged = append(merged, trimmed)
	}

	for _, part := range strings.Split(existing, ",") {
		appendValue(part)
	}
	for _, value := range values {
		appendValue(value)
	}

	return strings.Join(merged, ",")
}
