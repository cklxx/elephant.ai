package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	jsonx "alex/internal/shared/json"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
)

// openaiToolCallDelta represents a single tool-call fragment inside a streamed
// SSE chunk from the OpenAI-compatible API.
type openaiToolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// openaiStreamChunk is the top-level JSON structure for one SSE data payload.
type openaiStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string                `json:"content"`
			Reasoning        string                `json:"reasoning"`
			ReasoningContent string                `json:"reasoning_content"`
			Role             string                `json:"role"`
			ToolCalls        []openaiToolCallDelta `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// openaiToolAccumulator collects incremental fragments of a single tool call.
type openaiToolAccumulator struct {
	id        string
	name      string
	arguments strings.Builder
}

// streamProcessResult holds everything accumulated while scanning SSE chunks.
type streamProcessResult struct {
	content          string
	reasoning        string
	reasoningContent string
	usage            ports.TokenUsage
	finishReason     string
	toolAccumulators map[int]*openaiToolAccumulator
	toolOrder        []int
}

// StreamComplete streams incremental completion deltas while constructing the
// final aggregated response.
func (c *openaiClient) StreamComplete(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	requestID, prefix := c.buildLogPrefix(ctx, req.Metadata)
	provider := c.detectProvider()

	oaiReq := c.buildOpenAIRequest(req, true)

	body, err := jsonx.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	logBody := redactDataURIs(body)

	endpoint := c.baseURL + "/chat/completions"
	c.logRequestMeta(prefix, "POST", endpoint)

	c.logger.Debug("%sRequest Body: %s", prefix, logBody)
	utils.LogStreamingRequestPayload(requestID, logBody)

	requestStarted := time.Now()
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
		c.logger.Debug("%sError Response Body: %s", prefix, respBody)
		mappedErr := mapHTTPError(resp.StatusCode, respBody, resp.Header)
		c.logHTTPFailure(prefix, requestID, "stream", provider, endpoint, req, resp.StatusCode, resp.Header, respBody, mappedErr)
		return nil, mappedErr
	}

	scanner := newStreamScanner(resp.Body)

	sr, err := c.processStreamChunks(scanner, callbacks, prefix, requestID, provider, requestStarted)
	if err != nil {
		c.logProcessingFailure(prefix, requestID, "stream", provider, endpoint, "read_stream", req, err)
		return nil, err
	}

	if callbacks.OnContentDelta != nil {
		callbacks.OnContentDelta(ports.ContentDelta{Final: true})
	}

	result := assembleStreamResponse(sr, requestID, prefix, c.logger)

	c.fireUsageCallback(result.Usage, provider)

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

// processStreamChunks reads SSE events from scanner, invokes callbacks for
// content deltas, and returns the accumulated state.
func (c *openaiClient) processStreamChunks(
	scanner *streamScanner,
	callbacks ports.CompletionStreamCallbacks,
	prefix, requestID, provider string,
	requestStarted time.Time,
) (*streamProcessResult, error) {
	toolAccumulators := make(map[int]*openaiToolAccumulator)
	var toolOrder []int

	var contentBuilder strings.Builder
	var reasoningBuilder strings.Builder
	var reasoningContentBuilder strings.Builder
	usage := ports.TokenUsage{}
	finishReason := ""
	loggedTTFB := false

	appendToolCall := func(idx int) *openaiToolAccumulator {
		acc, ok := toolAccumulators[idx]
		if !ok {
			acc = &openaiToolAccumulator{}
			toolAccumulators[idx] = acc
			toolOrder = append(toolOrder, idx)
		}
		return acc
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			break
		}

		if !loggedTTFB {
			loggedTTFB = true
			logCLILatencyf(
				"[latency] llm_stream_ttfb_ms=%.2f provider=%s model=%s request_id=%s\n",
				float64(time.Since(requestStarted))/float64(time.Millisecond),
				provider,
				c.model,
				requestID,
			)
		}

		var chunk openaiStreamChunk
		if err := jsonx.Unmarshal([]byte(payload), &chunk); err != nil {
			c.logger.Debug("%sFailed to decode stream chunk: %v", prefix, err)
			continue
		}

		if chunk.Usage != nil {
			usage.PromptTokens = chunk.Usage.PromptTokens
			usage.CompletionTokens = chunk.Usage.CompletionTokens
			usage.TotalTokens = chunk.Usage.TotalTokens
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			finishReason = *choice.FinishReason
		}

		if text := choice.Delta.Content; text != "" {
			contentBuilder.WriteString(text)
			if callbacks.OnContentDelta != nil {
				callbacks.OnContentDelta(ports.ContentDelta{Delta: text})
			}
		}
		if reasoning := choice.Delta.Reasoning; reasoning != "" {
			reasoningBuilder.WriteString(reasoning)
		}
		if reasoning := choice.Delta.ReasoningContent; reasoning != "" {
			reasoningContentBuilder.WriteString(reasoning)
		}

		for _, tc := range choice.Delta.ToolCalls {
			acc := appendToolCall(tc.Index)
			if tc.ID != "" {
				acc.id = tc.ID
			}
			if tc.Function.Name != "" {
				acc.name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				acc.arguments.WriteString(tc.Function.Arguments)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		c.logger.Debug("%sStream read error: %v", prefix, err)
		return nil, fmt.Errorf("read response stream: %w", err)
	}

	return &streamProcessResult{
		content:          contentBuilder.String(),
		reasoning:        reasoningBuilder.String(),
		reasoningContent: reasoningContentBuilder.String(),
		usage:            usage,
		finishReason:     finishReason,
		toolAccumulators: toolAccumulators,
		toolOrder:        toolOrder,
	}, nil
}

// assembleStreamResponse builds a CompletionResponse from the accumulated
// stream processing result.
func assembleStreamResponse(sr *streamProcessResult, requestID, prefix string, logger logging.Logger) *ports.CompletionResponse {
	result := &ports.CompletionResponse{
		Content:    sr.content,
		StopReason: sr.finishReason,
		Usage:      sr.usage,
		Metadata: map[string]any{
			"request_id": requestID,
		},
	}
	thinking := ports.Thinking{}
	if sr.reasoning != "" {
		appendThinkingText(&thinking, "reasoning", sr.reasoning)
	}
	if sr.reasoningContent != "" {
		appendThinkingText(&thinking, "reasoning_content", sr.reasoningContent)
	}
	if len(thinking.Parts) > 0 {
		result.Thinking = thinking
	}

	for _, idx := range sr.toolOrder {
		acc := sr.toolAccumulators[idx]
		if acc == nil {
			continue
		}
		var args map[string]any
		if acc.arguments.Len() > 0 {
			if err := jsonx.Unmarshal([]byte(acc.arguments.String()), &args); err != nil {
				logger.Debug("%sFailed to parse tool call arguments: %v", prefix, err)
			}
		}
		result.ToolCalls = append(result.ToolCalls, ports.ToolCall{
			ID:        acc.id,
			Name:      acc.name,
			Arguments: args,
		})
	}

	return result
}
