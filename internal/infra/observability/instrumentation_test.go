package observability

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeToolArgumentsRecursive(t *testing.T) {
	args := map[string]any{
		"safe": "value",
		"nested": map[string]any{
			"headers": map[string]any{
				"Authorization": "Bearer top-secret-token",
				"X-Request-ID":  "request-1",
				"x-api-key":     "abc1234567890abcdefghijklmnop",
			},
			"items": []any{
				map[string]any{
					"password": "password-123",
				},
				map[string]any{
					"meta": map[string]any{
						"refresh_token": "refresh-token-value",
					},
				},
				"plain",
			},
		},
	}

	sanitized := sanitizeToolArguments(args)
	require.NotNil(t, sanitized)

	assert.Equal(t, "value", sanitized["safe"])

	nested, ok := sanitized["nested"].(map[string]any)
	require.True(t, ok)

	headers, ok := nested["headers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "***REDACTED***", headers["Authorization"])
	assert.Equal(t, "***REDACTED***", headers["x-api-key"])
	assert.Equal(t, "request-1", headers["X-Request-ID"])

	items, ok := nested["items"].([]any)
	require.True(t, ok)

	item0, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "***REDACTED***", item0["password"])

	item1, ok := items[1].(map[string]any)
	require.True(t, ok)
	meta, ok := item1["meta"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "***REDACTED***", meta["refresh_token"])
}

func TestSanitizeToolArgumentsPreservesOriginalInput(t *testing.T) {
	args := map[string]any{
		"token": "keep-original",
		"nested": map[string]any{
			"authorization": "Bearer keep-original-too",
		},
	}

	_ = sanitizeToolArguments(args)

	assert.Equal(t, "keep-original", args["token"])
	nested, ok := args["nested"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Bearer keep-original-too", nested["authorization"])
}
