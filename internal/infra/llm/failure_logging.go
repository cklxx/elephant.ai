package llm

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"alex/internal/domain/agent/ports"
	alexerrors "alex/internal/shared/errors"
	"alex/internal/shared/json"
)

const (
	llmFailurePreviewLimit = 320
	llmUnknownField        = "unknown"
)

func extractMetadataString(metadata map[string]any, key string) string {
	if metadata == nil || strings.TrimSpace(key) == "" {
		return ""
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(v)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		data, err := jsonx.Marshal(v)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(data))
	}
}

func extractRequestIntent(metadata map[string]any) string {
	for _, key := range []string{"intent", "operation", "caller", "source"} {
		if value := extractMetadataString(metadata, key); value != "" {
			return value
		}
	}
	return ""
}

func sanitizeLogValue(value string, limit int) string {
	if limit <= 0 {
		limit = llmFailurePreviewLimit
	}
	compact := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if compact == "" {
		return ""
	}
	runes := []rune(compact)
	if len(runes) <= limit {
		return compact
	}
	if limit <= 1 {
		return string(runes[:1])
	}
	return string(runes[:limit-1]) + "…"
}

func normalizeLogField(value string) string {
	trimmed := sanitizeLogValue(value, llmFailurePreviewLimit)
	if trimmed == "" {
		return llmUnknownField
	}
	return trimmed
}

func summarizeBodyPreview(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	return sanitizeLogValue(string(body), llmFailurePreviewLimit)
}

func parseUpstreamError(body []byte) (errType, errCode, errMessage string) {
	if len(body) == 0 {
		return "", "", ""
	}
	var payload struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Code    any    `json:"code"`
		Error   *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Code    any    `json:"code"`
		} `json:"error"`
	}
	if err := jsonx.Unmarshal(body, &payload); err != nil {
		return "", "", ""
	}

	if payload.Error != nil {
		errType = strings.TrimSpace(payload.Error.Type)
		errCode = stringifyErrorCode(payload.Error.Code)
		errMessage = strings.TrimSpace(payload.Error.Message)
	}

	if errType == "" {
		errType = strings.TrimSpace(payload.Type)
	}
	if errCode == "" {
		errCode = stringifyErrorCode(payload.Code)
	}
	if errMessage == "" {
		errMessage = strings.TrimSpace(payload.Message)
	}

	return sanitizeLogValue(errType, 64), sanitizeLogValue(errCode, 64), sanitizeLogValue(errMessage, llmFailurePreviewLimit)
}

func stringifyErrorCode(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case int:
		return strconv.Itoa(v)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		data, err := jsonx.Marshal(v)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(data))
	}
}

func classifyFailureError(err error) string {
	if err == nil {
		return "none"
	}
	switch {
	case alexerrors.IsDegraded(err):
		return "degraded"
	case alexerrors.IsTransient(err):
		return "transient"
	case alexerrors.IsPermanent(err):
		return "permanent"
	default:
		return "unknown"
	}
}

func (c *baseClient) logTransportFailure(prefix, requestID, mode, provider, endpoint string, req ports.CompletionRequest, err error) {
	c.logger.Warn(
		"%sLLM request transport failed: mode=%s provider=%s model=%s endpoint=%s request_id=%s intent=%s error_class=%s error=%v",
		prefix,
		normalizeLogField(mode),
		normalizeLogField(provider),
		normalizeLogField(c.model),
		normalizeLogField(endpoint),
		normalizeLogField(requestID),
		normalizeLogField(extractRequestIntent(req.Metadata)),
		classifyFailureError(err),
		err,
	)
}

func (c *baseClient) logHTTPFailure(
	prefix, requestID, mode, provider, endpoint string,
	req ports.CompletionRequest,
	status int,
	headers http.Header,
	body []byte,
	mappedErr error,
) {
	retryAfter := ""
	if headers != nil {
		retryAfter = strings.TrimSpace(headers.Get("Retry-After"))
	}
	retryAfterSeconds := parseRetryAfter(retryAfter)
	upstreamType, upstreamCode, upstreamMessage := parseUpstreamError(body)
	bodyPreview := summarizeBodyPreview(body)
	if upstreamMessage == "" {
		upstreamMessage = bodyPreview
	}

	c.logger.Warn(
		"%sLLM request rejected: mode=%s provider=%s model=%s endpoint=%s request_id=%s intent=%s status=%d retry_after=%q retry_after_seconds=%d upstream_type=%s upstream_code=%s upstream_message=%q error_class=%s error=%v body_preview=%q",
		prefix,
		normalizeLogField(mode),
		normalizeLogField(provider),
		normalizeLogField(c.model),
		normalizeLogField(endpoint),
		normalizeLogField(requestID),
		normalizeLogField(extractRequestIntent(req.Metadata)),
		status,
		retryAfter,
		retryAfterSeconds,
		normalizeLogField(upstreamType),
		normalizeLogField(upstreamCode),
		upstreamMessage,
		classifyFailureError(mappedErr),
		mappedErr,
		bodyPreview,
	)
}

func (c *baseClient) logProcessingFailure(prefix, requestID, mode, provider, endpoint, stage string, req ports.CompletionRequest, err error) {
	c.logger.Warn(
		"%sLLM request processing failed: mode=%s provider=%s model=%s endpoint=%s request_id=%s intent=%s stage=%s error_class=%s error=%v",
		prefix,
		normalizeLogField(mode),
		normalizeLogField(provider),
		normalizeLogField(c.model),
		normalizeLogField(endpoint),
		normalizeLogField(requestID),
		normalizeLogField(extractRequestIntent(req.Metadata)),
		normalizeLogField(stage),
		classifyFailureError(err),
		err,
	)
}

