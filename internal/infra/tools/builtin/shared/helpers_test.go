package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// StringArg
// ---------------------------------------------------------------------------

func TestStringArg(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		key  string
		want string
	}{
		{"nil map", nil, "k", ""},
		{"missing key", map[string]any{"other": "v"}, "k", ""},
		{"nil value", map[string]any{"k": nil}, "k", ""},
		{"string value", map[string]any{"k": "hello"}, "k", "hello"},
		{"non-string value (int)", map[string]any{"k": 42}, "k", "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, StringArg(tt.args, tt.key))
		})
	}
}

// ---------------------------------------------------------------------------
// StringSliceArg
// ---------------------------------------------------------------------------

func TestStringSliceArg(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		key  string
		want []string
	}{
		{"missing key", map[string]any{}, "k", nil},
		{"[]string input", map[string]any{"k": []string{"a", "b"}}, "k", []string{"a", "b"}},
		{
			"[]any with trimming and empty filtering",
			map[string]any{"k": []any{" hello ", "", "  ", "world"}},
			"k",
			[]string{"hello", "world"},
		},
		{"single string input", map[string]any{"k": "solo"}, "k", []string{"solo"}},
		{"empty string input", map[string]any{"k": ""}, "k", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, StringSliceArg(tt.args, tt.key))
		})
	}
}

// ---------------------------------------------------------------------------
// StringMapArg
// ---------------------------------------------------------------------------

func TestStringMapArg(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		key  string
		want map[string]string
	}{
		{"nil map", nil, "k", nil},
		{"missing key", map[string]any{}, "k", nil},
		{"non-map value", map[string]any{"k": "not-a-map"}, "k", nil},
		{
			"empty keys and values filtered",
			map[string]any{"k": map[string]any{
				"":     "val",
				"key":  "",
				"  ":   "val2",
				"good": "  ",
			}},
			"k",
			nil, // all entries filtered out
		},
		{
			"valid map",
			map[string]any{"k": map[string]any{
				"alpha": "one",
				"beta":  " two ",
			}},
			"k",
			map[string]string{"alpha": "one", "beta": "two"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, StringMapArg(tt.args, tt.key))
		})
	}
}

// ---------------------------------------------------------------------------
// IntArg
// ---------------------------------------------------------------------------

func TestIntArg(t *testing.T) {
	tests := []struct {
		name   string
		args   map[string]any
		key    string
		wantV  int
		wantOK bool
	}{
		{"nil map", nil, "k", 0, false},
		{"missing key", map[string]any{}, "k", 0, false},
		{"int value", map[string]any{"k": 7}, "k", 7, true},
		{"int64 value", map[string]any{"k": int64(99)}, "k", 99, true},
		{"float64 value", map[string]any{"k": float64(3.9)}, "k", 3, true},
		{"invalid type (string)", map[string]any{"k": "nope"}, "k", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, ok := IntArg(tt.args, tt.key)
			assert.Equal(t, tt.wantV, v)
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}

// ---------------------------------------------------------------------------
// FloatArg
// ---------------------------------------------------------------------------

func TestFloatArg(t *testing.T) {
	tests := []struct {
		name   string
		args   map[string]any
		key    string
		wantV  float64
		wantOK bool
	}{
		{"float64 value", map[string]any{"k": 3.14}, "k", 3.14, true},
		{"int value", map[string]any{"k": 5}, "k", 5.0, true},
		{"int64 value", map[string]any{"k": int64(10)}, "k", 10.0, true},
		{"invalid type (string)", map[string]any{"k": "bad"}, "k", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, ok := FloatArg(tt.args, tt.key)
			assert.Equal(t, tt.wantV, v)
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}

// ---------------------------------------------------------------------------
// Uint64Arg
// ---------------------------------------------------------------------------

func TestUint64Arg(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		key  string
		want uint64
	}{
		{"nil map", nil, "k", 0},
		{"int value", map[string]any{"k": 42}, "k", 42},
		{"negative int returns 0", map[string]any{"k": -1}, "k", 0},
		{"float64 value", map[string]any{"k": float64(7.5)}, "k", 7},
		{"uint64 value", map[string]any{"k": uint64(123)}, "k", 123},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Uint64Arg(tt.args, tt.key))
		})
	}
}

// ---------------------------------------------------------------------------
// PreviewProfile
// ---------------------------------------------------------------------------

func TestPreviewProfile(t *testing.T) {
	tests := []struct {
		name      string
		mediaType string
		format    string
		want      string
	}{
		{"a2ui media type", "application/a2ui", "", "document.a2ui"},
		{"a2ui format", "", "a2ui", "document.a2ui"},
		{"markdown media type", "text/markdown", "", "document.markdown"},
		{"md format", "", "md", "document.markdown"},
		{"html format", "", "html", "document.html"},
		{"image/png", "image/png", "", "document.image"},
		{"unknown", "application/octet-stream", "", "document"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, PreviewProfile(tt.mediaType, tt.format))
		})
	}
}

// ---------------------------------------------------------------------------
// ToolError
// ---------------------------------------------------------------------------

func TestToolError(t *testing.T) {
	result, err := ToolError("call-1", "something %s", "broke")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "call-1", result.CallID)
	assert.Equal(t, "something broke", result.Content)
	assert.EqualError(t, result.Error, "something broke")
}

// ---------------------------------------------------------------------------
// RequireStringArg
// ---------------------------------------------------------------------------

func TestRequireStringArg(t *testing.T) {
	t.Run("missing key", func(t *testing.T) {
		_, res := RequireStringArg(map[string]any{}, "c1", "name")
		require.NotNil(t, res)
		assert.Contains(t, res.Content, "missing")
	})

	t.Run("blank string", func(t *testing.T) {
		_, res := RequireStringArg(map[string]any{"name": "   "}, "c1", "name")
		require.NotNil(t, res)
		assert.Contains(t, res.Content, "cannot be empty")
	})

	t.Run("valid string", func(t *testing.T) {
		val, res := RequireStringArg(map[string]any{"name": " alice "}, "c1", "name")
		assert.Nil(t, res)
		assert.Equal(t, "alice", val)
	})
}
