package llm

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	"alex/internal/shared/json"
	alexerrors "alex/internal/shared/errors"
	"alex/internal/shared/utils"
)

const (
	defaultAnthropicBaseURL     = "https://api.anthropic.com/v1"
	defaultAnthropicVersion     = "2023-06-01"
	anthropicToolsBetaHeader    = "tools-2024-04-04"
	anthropicOAuthBetaHeader    = "oauth-2025-04-20"
	anthropicThinkingBetaHeader = "interleaved-thinking-2025-05-14"
	anthropicCodeBetaHeader     = "claude-code-20250219"
	anthropicStreamBetaHeader   = "fine-grained-tool-streaming-2025-05-14"
	anthropicVersionHeaderKey   = "anthropic-version"
	anthropicBetaHeaderKey      = "anthropic-beta"
	anthropicRequestHeaderKey   = "x-api-key"
	anthropicMessagesPath       = "/messages"
	anthropicRequestContentType = "application/json"
	anthropicOAuthUserAgent     = "claude-cli/2.1.75"
)

type anthropicClient struct {
	baseClient
}

func NewAnthropicClient(model string, config Config) (portsllm.LLMClient, error) {
	return &anthropicClient{
		baseClient: newBaseClient(model, config, baseClientOpts{
			defaultBaseURL: defaultAnthropicBaseURL,
			logCategory:    utils.LogCategoryLLM,
			logComponent:   "anthropic",
		}),
	}, nil
}

func (c *anthropicClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	requestID, prefix := c.buildLogPrefix(ctx, req.Metadata)
	provider := "anthropic"

	// Detect OAuth mode from API key.
	usesOAuth := isAnthropicOAuthToken(c.getAPIKey())

	messages, system := c.convertMessages(req.Messages)
	payload := map[string]any{
		"model":      c.model,
		"max_tokens": req.MaxTokens,
		"messages":   messages,
	}

	thinkingEnabled := false
	if shouldSendAnthropicThinking(c.model, req.Thinking) {
		if usesOAuth {
			// OAuth tokens require adaptive thinking (not manual "enabled" mode).
			payload["thinking"] = map[string]any{"type": "adaptive"}
			payload["output_config"] = map[string]any{"effort": "medium"}
			thinkingEnabled = true
		} else if thinking := buildAnthropicThinkingConfig(req.Thinking); thinking != nil {
			payload["thinking"] = thinking
			thinkingEnabled = true
		}
	}

	// System prompt: OAuth requires list format with Claude Code identity prefix.
	if system != "" {
		if usesOAuth {
			payload["system"] = []map[string]any{
				{"type": "text", "text": "You are Claude Code, Anthropic's official CLI for Claude."},
				{"type": "text", "text": system},
			}
		} else {
			payload["system"] = system
		}
	} else if usesOAuth {
		payload["system"] = []map[string]any{
			{"type": "text", "text": "You are Claude Code, Anthropic's official CLI for Claude."},
		}
	}

	// OAuth mode: omit temperature entirely (Anthropic rejects it with adaptive thinking).
	// API key mode: force temperature=1 for manual thinking, otherwise use requested value.
	if usesOAuth {
		// temperature omitted
	} else if thinkingEnabled {
		payload["temperature"] = 1
	} else {
		payload["temperature"] = req.Temperature
	}

	// OAuth mode always streams.
	if usesOAuth {
		payload["stream"] = true
	}

	if len(req.StopSequences) > 0 {
		payload["stop_sequences"] = append([]string(nil), req.StopSequences...)
	}
	if len(req.Tools) > 0 {
		payload["tools"] = convertAnthropicToolsDef(req.Tools)
	}

	body, err := jsonx.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	logBody := redactDataURIs(body)

	endpoint := c.baseURL + anthropicMessagesPath
	c.logRequestMeta(prefix, "POST", endpoint)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", anthropicRequestContentType)
	if c.maxRetries > 0 {
		httpReq.Header.Set("X-Retry-Limit", fmt.Sprintf("%d", c.maxRetries))
	}
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}

	// Auth headers.
	hasAuthorization := utils.HasContent(httpReq.Header.Get("Authorization"))
	hasAPIKeyHeader := utils.HasContent(httpReq.Header.Get(anthropicRequestHeaderKey))
	if !hasAuthorization && !hasAPIKeyHeader {
		if key := c.getAPIKey(); key != "" {
			if isAnthropicOAuthToken(key) {
				httpReq.Header.Set("Authorization", "Bearer "+key)
			} else {
				httpReq.Header.Set(anthropicRequestHeaderKey, key)
			}
		}
	}

	// OAuth mode: impersonate Claude Code CLI (required by Anthropic for OAuth tokens).
	if usesOAuth {
		httpReq.Header.Set("User-Agent", anthropicOAuthUserAgent)
		httpReq.Header.Set("X-App", "cli")
		httpReq.Header.Set("Accept", "application/json")
		httpReq.Header.Set("anthropic-dangerous-direct-browser-access", "true")
	}

	if httpReq.Header.Get(anthropicVersionHeaderKey) == "" {
		httpReq.Header.Set(anthropicVersionHeaderKey, defaultAnthropicVersion)
	}

	// Beta headers.
	var betaValues []string
	if usesOAuth {
		betaValues = append(betaValues, anthropicCodeBetaHeader, anthropicOAuthBetaHeader, anthropicStreamBetaHeader, anthropicThinkingBetaHeader)
	} else {
		if len(req.Tools) > 0 {
			betaValues = append(betaValues, anthropicToolsBetaHeader)
		}
		if thinkingEnabled {
			betaValues = append(betaValues, anthropicThinkingBetaHeader)
		}
	}
	if len(betaValues) > 0 {
		httpReq.Header.Set(
			anthropicBetaHeaderKey,
			mergeAnthropicBetaValues(httpReq.Header.Get(anthropicBetaHeaderKey), betaValues...),
		)
	}

	c.logRequestHeaders(prefix, httpReq.Header)

	c.logger.Debug("%sRequest Body: %s", prefix, logBody)
	utils.LogStreamingRequestPayload(requestID, logBody)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Debug("%sHTTP request failed: %v", prefix, err)
		wrapped := wrapRequestError(err)
		c.logTransportFailure(prefix, requestID, "complete", provider, endpoint, req, wrapped)
		return nil, wrapped
	}
	defer func() { _ = resp.Body.Close() }()

	c.logResponseStatus(prefix, resp)

	// OAuth mode uses streaming — consume SSE events and reconstruct the response.
	if usesOAuth && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result, sseErr := c.consumeAnthropicSSE(resp.Body)
		if sseErr != nil {
			c.logProcessingFailure(prefix, requestID, "complete", provider, endpoint, "sse_consume", req, sseErr)
			return nil, sseErr
		}
		if result.Metadata == nil {
			result.Metadata = map[string]any{}
		}
		result.Metadata["request_id"] = requestID
		c.fireUsageCallback(result.Usage, "anthropic")
		c.logger.Debug("%sResponse (SSE): content=%d bytes, tool_calls=%d", prefix, len(result.Content), len(result.ToolCalls))
		utils.LogStreamingResponsePayload(requestID, []byte(result.Content))
		c.logResponseSummary(prefix, result)
		return result, nil
	}

	respBody, err := readResponseBody(resp.Body)
	if err != nil {
		c.logger.Debug("%sFailed to read response body: %v", prefix, err)
		readErr := fmt.Errorf("read response: %w", err)
		c.logProcessingFailure(prefix, requestID, "complete", provider, endpoint, "read_response", req, readErr)
		return nil, readErr
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.logger.Warn("%sRequest rejected (HTTP %d): response=%s request=%s", prefix, resp.StatusCode, respBody, logBody)
		mappedErr := mapHTTPError(resp.StatusCode, respBody, resp.Header)
		c.logHTTPFailure(prefix, requestID, "complete", provider, endpoint, req, resp.StatusCode, resp.Header, respBody, mappedErr)
		return nil, mappedErr
	}

	var apiResp anthropicResponse
	if err := jsonx.Unmarshal(respBody, &apiResp); err != nil {
		c.logger.Debug("%sFailed to decode response: %v", prefix, err)
		decodeErr := fmt.Errorf("decode response: %w", err)
		c.logProcessingFailure(prefix, requestID, "complete", provider, endpoint, "decode_response", req, decodeErr)
		return nil, decodeErr
	}

	if apiResp.Error != nil && apiResp.Error.Message != "" {
		errMsg := apiResp.Error.Message
		if apiResp.Error.Type != "" {
			errMsg = fmt.Sprintf("%s: %s", apiResp.Error.Type, apiResp.Error.Message)
		}
		mappedErr := mapHTTPError(resp.StatusCode, []byte(errMsg), resp.Header)
		c.logHTTPFailure(prefix, requestID, "complete", provider, endpoint, req, resp.StatusCode, resp.Header, []byte(errMsg), mappedErr)
		return nil, mappedErr
	}

	content, toolCalls, thinking := parseAnthropicContent(apiResp.Content)
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
	if len(thinking.Parts) > 0 {
		result.Thinking = thinking
	}

	c.fireUsageCallback(result.Usage, "anthropic")

	c.logger.Debug("%sResponse Body: %s", prefix, respBody)
	utils.LogStreamingResponsePayload(requestID, respBody)

	c.logResponseSummary(prefix, result)
	return result, nil
}

// consumeAnthropicSSE reads an SSE stream from the Anthropic Messages API and
// reconstructs a CompletionResponse. Used when OAuth mode forces stream=true.
func (c *anthropicClient) consumeAnthropicSSE(body io.Reader) (*ports.CompletionResponse, error) {
	var (
		contentBuilder strings.Builder
		toolCalls      []ports.ToolCall
		stopReason     string
		usage          ports.TokenUsage
		messageID      string
		// Track in-flight tool_use blocks by index.
		toolByIndex = map[int]*ports.ToolCall{}
		argsByIndex = map[int]*strings.Builder{}
	)

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := line[6:]
		if data == "[DONE]" {
			break
		}

		var event map[string]any
		if err := jsonx.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event["type"] {
		case "message_start":
			if msg, ok := event["message"].(map[string]any); ok {
				messageID, _ = msg["id"].(string)
				if u, ok := msg["usage"].(map[string]any); ok {
					if v, ok := u["input_tokens"].(float64); ok {
						usage.PromptTokens = int(v)
					}
				}
			}
		case "content_block_start":
			idx := int(getFloat(event, "index"))
			if cb, ok := event["content_block"].(map[string]any); ok {
				switch cb["type"] {
				case "tool_use":
					tc := &ports.ToolCall{
						ID:        getString(cb, "id"),
						Name:      getString(cb, "name"),
						Arguments: map[string]any{},
					}
					toolByIndex[idx] = tc
					argsByIndex[idx] = &strings.Builder{}
				}
			}
		case "content_block_delta":
			idx := int(getFloat(event, "index"))
			if delta, ok := event["delta"].(map[string]any); ok {
				switch delta["type"] {
				case "text_delta":
					contentBuilder.WriteString(getString(delta, "text"))
				case "thinking_delta":
					// Thinking is not returned to the caller; skip.
				case "input_json_delta":
					if ab, ok := argsByIndex[idx]; ok {
						ab.WriteString(getString(delta, "partial_json"))
					}
				case "signature_delta":
					// Thinking is not returned; skip signature.
				}
			}
		case "content_block_stop":
			idx := int(getFloat(event, "index"))
			if tc, ok := toolByIndex[idx]; ok {
				if ab, ok := argsByIndex[idx]; ok {
					raw := ab.String()
					if raw != "" {
						var args map[string]any
						if err := jsonx.Unmarshal([]byte(raw), &args); err == nil {
							tc.Arguments = args
						}
					}
				}
				toolCalls = append(toolCalls, *tc)
				delete(toolByIndex, idx)
				delete(argsByIndex, idx)
			}
		case "message_delta":
			if delta, ok := event["delta"].(map[string]any); ok {
				if sr, ok := delta["stop_reason"].(string); ok {
					stopReason = sr
				}
			}
			if u, ok := event["usage"].(map[string]any); ok {
				if v, ok := u["output_tokens"].(float64); ok {
					usage.CompletionTokens = int(v)
				}
			}
		case "error":
			if errObj, ok := event["error"].(map[string]any); ok {
				errType, _ := errObj["type"].(string)
				errMsg, _ := errObj["message"].(string)
				if errMsg == "" {
					errMsg = errType
				}
				rawErr := fmt.Errorf("anthropic stream error: %s: %s", errType, errMsg)
				if errType == "overloaded_error" {
					terr := alexerrors.NewTransientError(rawErr, "Server overloaded (529). Retrying request.")
					terr.StatusCode = 529
					return nil, terr
				}
				return nil, rawErr
			}
			return nil, fmt.Errorf("anthropic stream error: %s", data)
		}
	}

	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens

	result := &ports.CompletionResponse{
		Content:    contentBuilder.String(),
		StopReason: stopReason,
		ToolCalls:  toolCalls,
		Usage:      usage,
		Metadata: map[string]any{
			"message_id": strings.TrimSpace(messageID),
		},
	}
	return result, scanner.Err()
}

func getString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func getFloat(m map[string]any, key string) float64 {
	v, _ := m[key].(float64)
	return v
}

func (c *anthropicClient) SetUsageCallback(callback func(usage ports.TokenUsage, model string, provider string)) {
	c.usageCallback = callback
}

func (c *anthropicClient) convertMessages(msgs []ports.Message) ([]anthropicMessage, string) {
	messages := make([]anthropicMessage, 0, len(msgs))
	var systemParts []string

	embedMask := attachmentEmbeddingMask(msgs)
	for idx, msg := range msgs {
		if msg.Source == ports.MessageSourceDebug || msg.Source == ports.MessageSourceEvaluation {
			continue
		}

		role := utils.TrimLower(msg.Role)
		if role == "" {
			continue
		}

		switch role {
		case "system":
			if utils.HasContent(msg.Content) {
				systemParts = append(systemParts, msg.Content)
			}
			continue
		case "tool":
			if utils.IsBlank(msg.ToolCallID) {
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

		contentBlocks := buildAnthropicMessageContent(msg, embedMask[idx])
		for _, call := range msg.ToolCalls {
			contentBlocks = append(contentBlocks, anthropicContentBlock{
				Type:  "tool_use",
				ID:    call.ID,
				Name:  call.Name,
				Input: normalizeToolArguments(call.Arguments),
			})
		}

		if len(contentBlocks) == 0 && utils.IsBlank(msg.Content) {
			continue
		}

		messages = append(messages, anthropicMessage{
			Role:    role,
			Content: contentBlocks,
		})
	}

	messages = normalizeAnthropicMessages(messages)

	return messages, strings.Join(systemParts, "\n\n")
}

// normalizeAnthropicMessages enforces the Anthropic Messages API constraints:
//  1. Consecutive messages with the same role are merged.
//  2. The first message must have role "user".
//  3. The last message must have role "user" (no assistant prefill).
func normalizeAnthropicMessages(messages []anthropicMessage) []anthropicMessage {
	if len(messages) == 0 {
		return messages
	}

	// Merge consecutive same-role messages.
	merged := make([]anthropicMessage, 0, len(messages))
	for _, msg := range messages {
		if len(merged) > 0 && merged[len(merged)-1].Role == msg.Role {
			merged[len(merged)-1].Content = append(merged[len(merged)-1].Content, msg.Content...)
		} else {
			merged = append(merged, anthropicMessage{
				Role:    msg.Role,
				Content: append([]anthropicContentBlock(nil), msg.Content...),
			})
		}
	}

	// Ensure first message is "user".
	if len(merged) > 0 && merged[0].Role != "user" {
		merged = append([]anthropicMessage{{
			Role:    "user",
			Content: []anthropicContentBlock{{Type: "text", Text: "Continue."}},
		}}, merged...)
	}

	// Ensure last message is "user" (prevent assistant prefill rejection).
	if len(merged) > 0 && merged[len(merged)-1].Role != "user" {
		merged = append(merged, anthropicMessage{
			Role:    "user",
			Content: []anthropicContentBlock{{Type: "text", Text: "Continue."}},
		})
	}

	return merged
}

func buildAnthropicMessageContent(msg ports.Message, embedAttachments bool) []anthropicContentBlock {
	// Anthropic extended thinking: thinking parts with signatures must be
	// emitted as proper "thinking" content blocks (not converted to text)
	// so they round-trip correctly during multi-turn tool use.
	thinkingBlocks, thinkingFallbackText := buildAnthropicThinkingBlocks(msg.Thinking)

	hasThinking := len(thinkingBlocks) > 0 || thinkingFallbackText != ""

	if len(msg.Attachments) == 0 || !embedAttachments {
		if utils.IsBlank(msg.Content) && !hasThinking {
			return nil
		}
		var blocks []anthropicContentBlock
		blocks = append(blocks, thinkingBlocks...)
		if utils.HasContent(msg.Content) {
			blocks = append(blocks, anthropicContentBlock{Type: "text", Text: msg.Content})
		}
		if thinkingFallbackText != "" {
			blocks = append(blocks, anthropicContentBlock{Type: "text", Text: thinkingFallbackText})
		}
		return blocks
	}

	var parts []anthropicContentBlock
	// Thinking blocks go first (Anthropic requires thinking before text/tool_use).
	parts = append(parts, thinkingBlocks...)

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

	appendBase64Image := func(att ports.Attachment, placeholder string) bool {
		data := ports.AttachmentInlineBase64(att)
		if data == "" {
			return false
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
		return true
	}

	embedAttachmentImages(msg.Content, msg.Attachments, appendText,
		appendBase64Image,
		func(att ports.Attachment, key string) bool {
			appendText("[" + key + "]")
			return appendBase64Image(att, key)
		},
	)

	if !hasImage {
		if utils.IsBlank(msg.Content) && !hasThinking {
			return nil
		}
		var blocks []anthropicContentBlock
		blocks = append(blocks, thinkingBlocks...)
		if utils.HasContent(msg.Content) {
			blocks = append(blocks, anthropicContentBlock{Type: "text", Text: msg.Content})
		}
		if thinkingFallbackText != "" {
			blocks = append(blocks, anthropicContentBlock{Type: "text", Text: thinkingFallbackText})
		}
		return blocks
	}

	if thinkingFallbackText != "" {
		parts = append(parts, anthropicContentBlock{Type: "text", Text: thinkingFallbackText})
	}

	return parts
}

// buildAnthropicThinkingBlocks splits thinking parts into proper Anthropic
// content blocks (for parts with signatures that must round-trip) and a
// fallback text representation (for parts without signatures).
func buildAnthropicThinkingBlocks(thinking ports.Thinking) (blocks []anthropicContentBlock, fallbackText string) {
	if len(thinking.Parts) == 0 {
		return nil, ""
	}

	var unsignedParts []ports.ThinkingPart
	for _, part := range thinking.Parts {
		if part.Signature != "" {
			// Proper thinking block with signature — must be passed back
			// unchanged for Anthropic extended thinking round-tripping.
			blocks = append(blocks, anthropicContentBlock{
				Type:      "thinking",
				Thinking:  part.Text,
				Signature: part.Signature,
			})
		} else {
			unsignedParts = append(unsignedParts, part)
		}
	}

	// Parts without signatures are converted to text prompt (non-Anthropic
	// thinking or thinking from providers that don't use signatures).
	fallbackText = thinkingPromptText(ports.Thinking{Parts: unsignedParts})
	return blocks, fallbackText
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



type anthropicMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

type anthropicContentBlock struct {
	Type      string                `json:"type"`
	Text      string                `json:"text,omitempty"`
	Thinking  string                `json:"thinking,omitempty"`
	ID        string                `json:"id,omitempty"`
	Name      string                `json:"name,omitempty"`
	Input     map[string]any        `json:"input,omitempty"`
	ToolUseID string                `json:"tool_use_id,omitempty"`
	Content   any                   `json:"content,omitempty"`
	Signature string                `json:"signature,omitempty"`
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

func parseAnthropicContent(blocks []anthropicContentBlock) (string, []ports.ToolCall, ports.Thinking) {
	var contentBuilder strings.Builder
	var toolCalls []ports.ToolCall
	var thinking ports.Thinking

	for _, block := range blocks {
		switch utils.TrimLower(block.Type) {
		case "text":
			contentBuilder.WriteString(block.Text)
		case "tool_use":
			toolCalls = append(toolCalls, ports.ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: normalizeToolArguments(block.Input),
			})
		case "thinking", "redacted_thinking":
			text := block.Thinking
			if text == "" {
				text = block.Text
			}
			appendThinkingPart(&thinking, ports.ThinkingPart{
				Kind:      utils.TrimLower(block.Type),
				Text:      text,
				Signature: block.Signature,
			})
		}
	}

	return contentBuilder.String(), toolCalls, thinking
}

func isAnthropicOAuthToken(token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	lower := strings.ToLower(token)
	// sk-ant-oat = OAuth Access Token from Claude Code CLI setup.
	if strings.HasPrefix(lower, "sk-ant-oat") {
		return true
	}
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
