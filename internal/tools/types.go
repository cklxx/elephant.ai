package tools

// ToolParameterType represents the type of a tool parameter
type ToolParameterType string

const (
	StringType  ToolParameterType = "string"
	IntegerType ToolParameterType = "integer"
	NumberType  ToolParameterType = "number"
	BooleanType ToolParameterType = "boolean"
	ArrayType   ToolParameterType = "array"
	ObjectType  ToolParameterType = "object"
)

// ToolParameterDefinition defines a parameter for a tool
// Replaces the nested map[string]interface{} structures
type ToolParameterDefinition struct {
	Type        ToolParameterType                   `json:"type"`
	Description string                              `json:"description"`
	Required    bool                                `json:"required,omitempty"`
	Default     any                                 `json:"default,omitempty"`
	Enum        []any                               `json:"enum,omitempty"`
	Items       *ToolParameterDefinition            `json:"items,omitempty"`      // For array types
	Properties  map[string]*ToolParameterDefinition `json:"properties,omitempty"` // For object types
	MinLength   *int                                `json:"minLength,omitempty"`
	MaxLength   *int                                `json:"maxLength,omitempty"`
	Pattern     string                              `json:"pattern,omitempty"`
}

// ToolSchema represents the complete parameter schema for a tool
// Replaces map[string]interface{} for tool.Parameters()
type ToolSchema struct {
	Type       string                              `json:"type"`
	Properties map[string]*ToolParameterDefinition `json:"properties"`
	Required   []string                            `json:"required"`
}

// NewStringParameter creates a string parameter definition
func NewStringParameter(description string, required bool) *ToolParameterDefinition {
	return &ToolParameterDefinition{
		Type:        StringType,
		Description: description,
		Required:    required,
	}
}

// NewIntegerParameter creates an integer parameter definition
func NewIntegerParameter(description string, required bool) *ToolParameterDefinition {
	return &ToolParameterDefinition{
		Type:        IntegerType,
		Description: description,
		Required:    required,
	}
}

// NewBooleanParameter creates a boolean parameter definition
func NewBooleanParameter(description string, required bool) *ToolParameterDefinition {
	return &ToolParameterDefinition{
		Type:        BooleanType,
		Description: description,
		Required:    required,
	}
}

// NewArrayParameter creates an array parameter definition
func NewArrayParameter(description string, required bool, items *ToolParameterDefinition) *ToolParameterDefinition {
	return &ToolParameterDefinition{
		Type:        ArrayType,
		Description: description,
		Required:    required,
		Items:       items,
	}
}

// NewObjectParameter creates an object parameter definition
func NewObjectParameter(description string, required bool, properties map[string]*ToolParameterDefinition) *ToolParameterDefinition {
	return &ToolParameterDefinition{
		Type:        ObjectType,
		Description: description,
		Required:    required,
		Properties:  properties,
	}
}

// WithPattern adds a regex pattern constraint to a string parameter
func (tpd *ToolParameterDefinition) WithPattern(pattern string) *ToolParameterDefinition {
	tpd.Pattern = pattern
	return tpd
}

// WithEnum adds enumeration constraints to a parameter
func (tpd *ToolParameterDefinition) WithEnum(values ...any) *ToolParameterDefinition {
	tpd.Enum = values
	return tpd
}

// WithDefault sets a default value for the parameter
func (tpd *ToolParameterDefinition) WithDefault(value any) *ToolParameterDefinition {
	tpd.Default = value
	return tpd
}

// WithLengthConstraints adds min/max length constraints to string or array parameters
func (tpd *ToolParameterDefinition) WithLengthConstraints(min, max *int) *ToolParameterDefinition {
	tpd.MinLength = min
	tpd.MaxLength = max
	return tpd
}

// ToLegacyMap converts the typed schema to the legacy map[string]interface{} format
// This allows gradual migration while maintaining backward compatibility
func (ts *ToolSchema) ToLegacyMap() map[string]interface{} {
	properties := make(map[string]interface{})

	for name, param := range ts.Properties {
		properties[name] = paramToMap(param)
	}

	return map[string]interface{}{
		"type":       ts.Type,
		"properties": properties,
		"required":   ts.Required,
	}
}

// paramToMap recursively converts parameter definitions to maps
func paramToMap(param *ToolParameterDefinition) map[string]interface{} {
	result := map[string]interface{}{
		"type":        string(param.Type),
		"description": param.Description,
	}

	if param.Default != nil {
		result["default"] = param.Default
	}

	if param.Enum != nil {
		result["enum"] = param.Enum
	}

	if param.Items != nil {
		result["items"] = paramToMap(param.Items)
	}

	if param.Properties != nil {
		props := make(map[string]interface{})
		for k, v := range param.Properties {
			props[k] = paramToMap(v)
		}
		result["properties"] = props
	}

	if param.MinLength != nil {
		result["minLength"] = *param.MinLength
	}

	if param.MaxLength != nil {
		result["maxLength"] = *param.MaxLength
	}

	if param.Pattern != "" {
		result["pattern"] = param.Pattern
	}

	return result
}

// NewToolSchema creates a new tool schema with the specified parameters
func NewToolSchema() *ToolSchema {
	return &ToolSchema{
		Type:       "object",
		Properties: make(map[string]*ToolParameterDefinition),
		Required:   []string{},
	}
}

// AddParameter adds a parameter to the schema
func (ts *ToolSchema) AddParameter(name string, param *ToolParameterDefinition) *ToolSchema {
	ts.Properties[name] = param
	if param.Required {
		ts.Required = append(ts.Required, name)
	}
	return ts
}

// AddRequiredParameter adds a required parameter to the schema
func (ts *ToolSchema) AddRequiredParameter(name string, param *ToolParameterDefinition) *ToolSchema {
	param.Required = true
	return ts.AddParameter(name, param)
}
