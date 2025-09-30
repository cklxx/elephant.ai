package types

import (
	"encoding/json"
	"time"
)

// ToolParameters represents structured parameters for tool execution
// Replaces map[string]interface{} in tool contexts
type ToolParameters map[string]any

// Validate ensures all required parameters are present
func (tp ToolParameters) GetString(key string) (string, bool) {
	if val, exists := tp[key]; exists {
		if str, ok := val.(string); ok {
			return str, true
		}
	}
	return "", false
}

// GetInt safely extracts an integer parameter
func (tp ToolParameters) GetInt(key string) (int, bool) {
	if val, exists := tp[key]; exists {
		switch v := val.(type) {
		case int:
			return v, true
		case float64:
			return int(v), true
		case string:
			// Could add strconv.Atoi here if needed
		}
	}
	return 0, false
}

// GetBool safely extracts a boolean parameter
func (tp ToolParameters) GetBool(key string) (bool, bool) {
	if val, exists := tp[key]; exists {
		if b, ok := val.(bool); ok {
			return b, true
		}
	}
	return false, false
}

// APIResponse represents structured API response data
// Replaces map[string]interface{} for API responses
type APIResponse struct {
	Success  bool            `json:"success"`
	Data     json.RawMessage `json:"data,omitempty"`
	Error    string          `json:"error,omitempty"`
	Metadata ResponseMeta    `json:"metadata,omitempty"`
}

// ResponseMeta contains response metadata
type ResponseMeta struct {
	RequestID    string            `json:"request_id,omitempty"`
	Timestamp    time.Time         `json:"timestamp"`
	Version      string            `json:"version,omitempty"`
	CacheInfo    *CacheInfo        `json:"cache_info,omitempty"`
	CustomFields map[string]string `json:"custom_fields,omitempty"`
}

// CacheInfo represents cache-related metadata
type CacheInfo struct {
	CacheID   string    `json:"cache_id,omitempty"`
	CacheHit  bool      `json:"cache_hit"`
	TTL       int       `json:"ttl,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// ConfigValue represents a typed configuration value
// Replaces interface{} in configuration contexts
type ConfigValue struct {
	StringValue *string  `json:"string_value,omitempty"`
	IntValue    *int     `json:"int_value,omitempty"`
	BoolValue   *bool    `json:"bool_value,omitempty"`
	FloatValue  *float64 `json:"float_value,omitempty"`
	StringArray []string `json:"string_array,omitempty"`
}

// Get returns the actual value based on type
func (cv ConfigValue) Get() any {
	switch {
	case cv.StringValue != nil:
		return *cv.StringValue
	case cv.IntValue != nil:
		return *cv.IntValue
	case cv.BoolValue != nil:
		return *cv.BoolValue
	case cv.FloatValue != nil:
		return *cv.FloatValue
	case cv.StringArray != nil:
		return cv.StringArray
	default:
		return nil
	}
}

// NewStringConfig creates a string configuration value
func NewStringConfig(value string) ConfigValue {
	return ConfigValue{StringValue: &value}
}

// NewIntConfig creates an integer configuration value
func NewIntConfig(value int) ConfigValue {
	return ConfigValue{IntValue: &value}
}

// NewBoolConfig creates a boolean configuration value
func NewBoolConfig(value bool) ConfigValue {
	return ConfigValue{BoolValue: &value}
}

// NewFloatConfig creates a float configuration value
func NewFloatConfig(value float64) ConfigValue {
	return ConfigValue{FloatValue: &value}
}

// NewStringArrayConfig creates a string array configuration value
func NewStringArrayConfig(value []string) ConfigValue {
	return ConfigValue{StringArray: value}
}

// SessionConfig represents typed session configuration
// Replaces map[string]interface{} in session contexts
type SessionConfig struct {
	UserID      string                 `json:"user_id,omitempty"`
	SessionType string                 `json:"session_type,omitempty"`
	Settings    map[string]ConfigValue `json:"settings,omitempty"`
	Features    []string               `json:"features,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// GetSetting safely retrieves a typed setting
func (sc SessionConfig) GetSetting(key string) (ConfigValue, bool) {
	if val, exists := sc.Settings[key]; exists {
		return val, true
	}
	return ConfigValue{}, false
}

// SetSetting sets a typed setting
func (sc *SessionConfig) SetSetting(key string, value ConfigValue) {
	if sc.Settings == nil {
		sc.Settings = make(map[string]ConfigValue)
	}
	sc.Settings[key] = value
}

// ToolResult represents a structured tool execution result
// Replaces interface{} in tool result contexts
type ToolResult[T any] struct {
	Success  bool          `json:"success"`
	Data     T             `json:"data,omitempty"`
	Error    string        `json:"error,omitempty"`
	Metadata ToolMeta      `json:"metadata"`
	Duration time.Duration `json:"duration"`
}

// ToolMeta contains tool execution metadata
type ToolMeta struct {
	ToolName   string            `json:"tool_name"`
	Version    string            `json:"version,omitempty"`
	ExecutedAt time.Time         `json:"executed_at"`
	Parameters ToolParameters    `json:"parameters,omitempty"`
	CustomMeta map[string]string `json:"custom_meta,omitempty"`
}

// NewSuccessResult creates a successful tool result
func NewSuccessResult[T any](toolName string, data T) ToolResult[T] {
	return ToolResult[T]{
		Success: true,
		Data:    data,
		Metadata: ToolMeta{
			ToolName:   toolName,
			ExecutedAt: time.Now(),
		},
	}
}

// NewErrorResult creates an error tool result
func NewErrorResult[T any](toolName string, err error) ToolResult[T] {
	return ToolResult[T]{
		Success: false,
		Error:   err.Error(),
		Metadata: ToolMeta{
			ToolName:   toolName,
			ExecutedAt: time.Now(),
		},
	}
}
