package agent

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"alex/internal/llm"
	"alex/internal/session"
)

// LLMHandler handles all LLM-related operations
type LLMHandler struct {
	streamCallback StreamCallback
	sessionManager *session.Manager
}

// NewLLMHandler creates a new LLM handler
func NewLLMHandler(sessionManager *session.Manager, streamCallback StreamCallback) *LLMHandler {
	return &LLMHandler{
		streamCallback: streamCallback,
		sessionManager: sessionManager,
	}
}

// isNetworkError checks if an error is network-related and should not be retried
func (h *LLMHandler) isNetworkError(err error) bool {
	errStr := err.Error()

	// Extract HTTP status code if present
	if strings.Contains(errStr, "HTTP error ") {
		// Look for pattern "HTTP error XXX:"
		parts := strings.Split(errStr, "HTTP error ")
		if len(parts) > 1 {
			statusPart := strings.Split(parts[1], ":")
			if len(statusPart) > 0 {
				if statusCode, parseErr := strconv.Atoi(statusPart[0]); parseErr == nil {
					// Network-related HTTP status codes that shouldn't be retried
					switch statusCode {
					case 400, 401, 403, 404, 405, 406, 408, 409, 410, 411, 412, 413, 414, 415, 416, 417, 418, 421, 422, 423, 424, 425, 426, 428, 429, 431, 451:
						// Client errors (4xx) - usually indicate request issues, not transient network problems
						return true
					case 500: // Server error - request format issue
						return true
						// Note: 502, 503, 504 are temporary server issues and should be retried
					}
				}
			}
		}
	}

	// Check for common network error patterns
	networkErrorPatterns := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"dial timeout",
		"read timeout",
		"write timeout",
		"network is unreachable",
		"no route to host",
		"host is down",
		"dns lookup failed",
		"tls handshake timeout",
		"certificate verify failed",
		"ssl handshake failed",
	}

	lowerErr := strings.ToLower(errStr)
	for _, pattern := range networkErrorPatterns {
		if strings.Contains(lowerErr, pattern) {
			return true
		}
	}

	return false
}

// isRetriableError checks if an error should be retried (opposite of network error)
func (h *LLMHandler) isRetriableError(err error) bool {
	errStr := err.Error()

	// Timeout errors from HTTP client should be retried
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return true
	}

	// Extract HTTP status code if present
	if strings.Contains(errStr, "HTTP error ") {
		parts := strings.Split(errStr, "HTTP error ")
		if len(parts) > 1 {
			statusPart := strings.Split(parts[1], ":")
			if len(statusPart) > 0 {
				if statusCode, parseErr := strconv.Atoi(statusPart[0]); parseErr == nil {
					// Temporary server issues that should be retried
					switch statusCode {
					case 502, 503, 504, 429: // Bad Gateway, Service Unavailable, Gateway Timeout, Rate Limited
						return true
					}
				}
			}
		}
	}

	// Temporary network issues that should be retried
	retriablePatterns := []string{
		"temporary failure",
		"server temporarily unavailable",
		"connection reset by peer",
		"broken pipe",
		"EOF",
	}

	lowerErr := strings.ToLower(errStr)
	for _, pattern := range retriablePatterns {
		if strings.Contains(lowerErr, pattern) {
			return true
		}
	}

	return false
}

// callLLMWithRetry - 带重试机制的非流式LLM调用
func (h *LLMHandler) callLLMWithRetry(ctx context.Context, client llm.Client, request *llm.ChatRequest, maxRetries int) (*llm.ChatResponse, error) {
	return h.callLLMWithRetryAndBackoff(ctx, client, request, maxRetries, nil)
}

// callLLMWithRetryAndBackoff - 带重试机制和可配置退避策略的非流式LLM调用
func (h *LLMHandler) callLLMWithRetryAndBackoff(ctx context.Context, client llm.Client, request *llm.ChatRequest, maxRetries int, backoffFunc func(int) time.Duration) (*llm.ChatResponse, error) {
	var lastErr error
	sessionID, _ := h.sessionManager.GetSessionID()
	// 默认的快速重试策略（100ms 间隔）
	if backoffFunc == nil {
		backoffFunc = func(attempt int) time.Duration {
			return 100 * time.Millisecond
		}
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// 使用非流式调用
		response, err := client.Chat(ctx, request, sessionID)
		if err != nil {
			lastErr = err
			log.Printf("[WARN] LLMHandler: Chat call failed (attempt %d): %v", attempt, err)

			// 检查是否是永久性网络错误（如认证错误），如果是，不要重试
			if h.isNetworkError(err) && !h.isRetriableError(err) {
				log.Printf("[ERROR] LLMHandler: Permanent network error detected, not retrying: %v", err)
				return nil, fmt.Errorf("permanent network error - not retrying: %w", err)
			}

			// 如果是可重试的错误（如超时、临时服务器错误），则重试
			if attempt < maxRetries && (h.isRetriableError(err) || !h.isNetworkError(err)) {
				backoffDuration := backoffFunc(attempt)
				log.Printf("[WARN] LLMHandler: Retrying in %v (retriable: %v, network: %v)",
					backoffDuration, h.isRetriableError(err), h.isNetworkError(err))
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoffDuration):
					continue
				}
			}
			continue
		}

		// 直接返回完整响应
		if response != nil {
			// 如果有回调，可以一次性发送完整内容
			if h.streamCallback != nil && len(response.Choices) > 0 {
				h.streamCallback(StreamChunk{
					Type:     "llm_content",
					Content:  response.Choices[0].Message.Content,
					Metadata: map[string]any{"streaming": false},
				})
			}
			return response, nil
		}

		lastErr = fmt.Errorf("received nil response")
		log.Printf("[WARN] LLMHandler: Received nil response (attempt %d)", attempt)

		if attempt < maxRetries {
			backoffDuration := backoffFunc(attempt)
			log.Printf("[WARN] LLMHandler: Retrying in %v", backoffDuration)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoffDuration):
				continue
			}
		}
	}

	return nil, fmt.Errorf("LLM call failed after %d attempts: %w", maxRetries, lastErr)
}

// validateLLMRequest - 验证LLM请求参数
func (h *LLMHandler) validateLLMRequest(request *llm.ChatRequest) error {
	if request == nil {
		return fmt.Errorf("request is nil")
	}

	if len(request.Messages) == 0 {
		return fmt.Errorf("no messages in request")
	}

	if request.Config == nil {
		return fmt.Errorf("config is nil")
	}

	return nil
}
