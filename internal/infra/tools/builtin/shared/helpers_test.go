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

// ---------------------------------------------------------------------------
// ValidationFramework
// ---------------------------------------------------------------------------

func TestValidationFramework_StringField(t *testing.T) {
	vf := NewValidationFramework().
		AddStringField("title", "The title")

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, vf.Validate(map[string]any{"title": "hello"}))
	})

	t.Run("empty string", func(t *testing.T) {
		err := vf.Validate(map[string]any{"title": ""})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("missing", func(t *testing.T) {
		err := vf.Validate(map[string]any{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "required")
	})
}

func TestValidationFramework_OptionalStringField(t *testing.T) {
	vf := NewValidationFramework().
		AddOptionalStringField("tag", "A tag")

	t.Run("absent is OK", func(t *testing.T) {
		assert.NoError(t, vf.Validate(map[string]any{}))
	})

	t.Run("empty when present fails", func(t *testing.T) {
		err := vf.Validate(map[string]any{"tag": ""})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty when provided")
	})
}

func TestValidationFramework_IntField(t *testing.T) {
	vf := NewValidationFramework().
		AddIntField("count", "Item count", 1, 100)

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, vf.Validate(map[string]any{"count": 50}))
	})

	t.Run("below min", func(t *testing.T) {
		err := vf.Validate(map[string]any{"count": 0})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least")
	})

	t.Run("above max", func(t *testing.T) {
		err := vf.Validate(map[string]any{"count": 200})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at most")
	})

	t.Run("float64 coercion", func(t *testing.T) {
		assert.NoError(t, vf.Validate(map[string]any{"count": float64(10)}))
	})
}

func TestValidationFramework_OptionalIntField(t *testing.T) {
	vf := NewValidationFramework().
		AddOptionalIntField("limit", "Limit", 0, 500)

	t.Run("absent OK", func(t *testing.T) {
		assert.NoError(t, vf.Validate(map[string]any{}))
	})

	t.Run("valid when present", func(t *testing.T) {
		assert.NoError(t, vf.Validate(map[string]any{"limit": 100}))
	})
}

func TestValidationFramework_BoolField(t *testing.T) {
	vf := NewValidationFramework().
		AddBoolField("enabled", "Toggle", true)

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, vf.Validate(map[string]any{"enabled": true}))
	})

	t.Run("missing required", func(t *testing.T) {
		err := vf.Validate(map[string]any{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "required")
	})

	t.Run("wrong type", func(t *testing.T) {
		err := vf.Validate(map[string]any{"enabled": "yes"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "boolean")
	})
}

func TestValidationFramework_OptionalArrayField(t *testing.T) {
	vf := NewValidationFramework().
		AddOptionalArrayField("items", "List of items")

	t.Run("absent OK", func(t *testing.T) {
		assert.NoError(t, vf.Validate(map[string]any{}))
	})

	t.Run("valid array", func(t *testing.T) {
		assert.NoError(t, vf.Validate(map[string]any{"items": []interface{}{"a", "b"}}))
	})

	t.Run("wrong type", func(t *testing.T) {
		err := vf.Validate(map[string]any{"items": "not-array"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "array")
	})
}

func TestValidationFramework_FieldMetadata(t *testing.T) {
	vf := NewValidationFramework().
		AddStringField("name", "User name").
		AddOptionalStringField("bio", "User biography")

	assert.Equal(t, []string{"name"}, vf.GetRequiredFields())
	assert.Equal(t, []string{"bio"}, vf.GetOptionalFields())
	assert.Equal(t, "User name", vf.GetFieldDescription("name"))
	assert.Equal(t, "User biography", vf.GetFieldDescription("bio"))
	assert.Equal(t, "", vf.GetFieldDescription("nonexistent"))
}

func TestValidationFramework_NilArgs(t *testing.T) {
	vf := NewValidationFramework().
		AddStringField("x", "required field")

	err := vf.Validate(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}
