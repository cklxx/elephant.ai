package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToolParameterType(t *testing.T) {
	// Test ToolParameterType constants
	assert.Equal(t, ToolParameterType("string"), StringType)
	assert.Equal(t, ToolParameterType("integer"), IntegerType)
	assert.Equal(t, ToolParameterType("number"), NumberType)
	assert.Equal(t, ToolParameterType("boolean"), BooleanType)
	assert.Equal(t, ToolParameterType("array"), ArrayType)
	assert.Equal(t, ToolParameterType("object"), ObjectType)
}

func TestToolParameterDefinition(t *testing.T) {
	// Test basic ToolParameterDefinition
	param := &ToolParameterDefinition{
		Type:        StringType,
		Description: "Test parameter",
		Required:    true,
		Default:     "default_value",
	}

	assert.Equal(t, StringType, param.Type)
	assert.Equal(t, "Test parameter", param.Description)
	assert.True(t, param.Required)
	assert.Equal(t, "default_value", param.Default)
}

func TestToolParameterDefinitionWithEnum(t *testing.T) {
	// Test ToolParameterDefinition with enum values
	param := &ToolParameterDefinition{
		Type:        StringType,
		Description: "Enum parameter",
		Required:    true,
		Enum:        []any{"option1", "option2", "option3"},
	}

	assert.Equal(t, StringType, param.Type)
	assert.Len(t, param.Enum, 3)
	assert.Contains(t, param.Enum, "option1")
	assert.Contains(t, param.Enum, "option2")
	assert.Contains(t, param.Enum, "option3")
}

func TestToolParameterDefinitionArray(t *testing.T) {
	// Test ToolParameterDefinition for array type
	itemDefinition := &ToolParameterDefinition{
		Type:        StringType,
		Description: "Array item",
	}

	param := &ToolParameterDefinition{
		Type:        ArrayType,
		Description: "Array parameter",
		Items:       itemDefinition,
	}

	assert.Equal(t, ArrayType, param.Type)
	assert.Equal(t, "Array parameter", param.Description)
	assert.NotNil(t, param.Items)
	assert.Equal(t, StringType, param.Items.Type)
	assert.Equal(t, "Array item", param.Items.Description)
}

func TestToolParameterDefinitionObject(t *testing.T) {
	// Test ToolParameterDefinition for object type
	properties := map[string]*ToolParameterDefinition{
		"name": {
			Type:        StringType,
			Description: "Object name",
			Required:    true,
		},
		"age": {
			Type:        IntegerType,
			Description: "Object age",
			Required:    false,
		},
	}

	param := &ToolParameterDefinition{
		Type:        ObjectType,
		Description: "Object parameter",
		Properties:  properties,
	}

	assert.Equal(t, ObjectType, param.Type)
	assert.Equal(t, "Object parameter", param.Description)
	assert.Len(t, param.Properties, 2)
	assert.Equal(t, StringType, param.Properties["name"].Type)
	assert.Equal(t, IntegerType, param.Properties["age"].Type)
	assert.True(t, param.Properties["name"].Required)
	assert.False(t, param.Properties["age"].Required)
}

func TestToolParameterDefinitionStringConstraints(t *testing.T) {
	// Test ToolParameterDefinition with string constraints
	minLength := 5
	maxLength := 100
	param := &ToolParameterDefinition{
		Type:        StringType,
		Description: "Constrained string parameter",
		MinLength:   &minLength,
		MaxLength:   &maxLength,
		Pattern:     "^[a-zA-Z]+$",
	}

	assert.Equal(t, StringType, param.Type)
	assert.NotNil(t, param.MinLength)
	assert.NotNil(t, param.MaxLength)
	assert.Equal(t, 5, *param.MinLength)
	assert.Equal(t, 100, *param.MaxLength)
	assert.Equal(t, "^[a-zA-Z]+$", param.Pattern)
}

func TestToolSchema(t *testing.T) {
	// Test ToolSchema structure
	properties := map[string]*ToolParameterDefinition{
		"name": {
			Type:        StringType,
			Description: "Name parameter",
			Required:    true,
		},
		"optional": {
			Type:        StringType,
			Description: "Optional parameter",
			Required:    false,
		},
	}

	schema := &ToolSchema{
		Type:       "object",
		Properties: properties,
		Required:   []string{"name"},
	}

	assert.Equal(t, "object", schema.Type)
	assert.Len(t, schema.Properties, 2)
	assert.Len(t, schema.Required, 1)
	assert.Contains(t, schema.Required, "name")
	assert.NotContains(t, schema.Required, "optional")
}

func TestNewStringParameter(t *testing.T) {
	// Test NewStringParameter helper function
	param := NewStringParameter("Test string parameter", true)

	assert.NotNil(t, param)
	assert.Equal(t, StringType, param.Type)
	assert.Equal(t, "Test string parameter", param.Description)
	assert.True(t, param.Required)
}

func TestNewStringParameterOptional(t *testing.T) {
	// Test NewStringParameter helper function for optional parameter
	param := NewStringParameter("Optional string parameter", false)

	assert.NotNil(t, param)
	assert.Equal(t, StringType, param.Type)
	assert.Equal(t, "Optional string parameter", param.Description)
	assert.False(t, param.Required)
}

func TestNewIntegerParameter(t *testing.T) {
	// Test NewIntegerParameter helper function
	param := NewIntegerParameter("Test integer parameter", true)

	assert.NotNil(t, param)
	assert.Equal(t, IntegerType, param.Type)
	assert.Equal(t, "Test integer parameter", param.Description)
	assert.True(t, param.Required)
}

func TestNewIntegerParameterOptional(t *testing.T) {
	// Test NewIntegerParameter helper function for optional parameter
	param := NewIntegerParameter("Optional integer parameter", false)

	assert.NotNil(t, param)
	assert.Equal(t, IntegerType, param.Type)
	assert.Equal(t, "Optional integer parameter", param.Description)
	assert.False(t, param.Required)
}

func TestParameterDefinitionNilFields(t *testing.T) {
	// Test ToolParameterDefinition with nil optional fields
	param := &ToolParameterDefinition{
		Type:        StringType,
		Description: "Simple parameter",
		Required:    false,
	}

	assert.Equal(t, StringType, param.Type)
	assert.Equal(t, "Simple parameter", param.Description)
	assert.False(t, param.Required)
	assert.Nil(t, param.Default)
	assert.Nil(t, param.Enum)
	assert.Nil(t, param.Items)
	assert.Nil(t, param.Properties)
	assert.Nil(t, param.MinLength)
	assert.Nil(t, param.MaxLength)
	assert.Empty(t, param.Pattern)
}

func TestComplexNestedSchema(t *testing.T) {
	// Test complex nested schema structure
	addressProperties := map[string]*ToolParameterDefinition{
		"street": NewStringParameter("Street address", true),
		"city":   NewStringParameter("City name", true),
		"zip":    NewStringParameter("ZIP code", false),
	}

	addressParam := &ToolParameterDefinition{
		Type:        ObjectType,
		Description: "Address object",
		Properties:  addressProperties,
	}

	tagsParam := &ToolParameterDefinition{
		Type:        ArrayType,
		Description: "List of tags",
		Items:       NewStringParameter("Tag", false),
	}

	schema := &ToolSchema{
		Type: "object",
		Properties: map[string]*ToolParameterDefinition{
			"name":    NewStringParameter("Full name", true),
			"age":     NewIntegerParameter("Age in years", false),
			"address": addressParam,
			"tags":    tagsParam,
		},
		Required: []string{"name"},
	}

	assert.Equal(t, "object", schema.Type)
	assert.Len(t, schema.Properties, 4)
	assert.Len(t, schema.Required, 1)

	// Test address object
	address := schema.Properties["address"]
	assert.Equal(t, ObjectType, address.Type)
	assert.Len(t, address.Properties, 3)
	assert.Equal(t, StringType, address.Properties["street"].Type)
	assert.True(t, address.Properties["street"].Required)

	// Test tags array
	tags := schema.Properties["tags"]
	assert.Equal(t, ArrayType, tags.Type)
	assert.NotNil(t, tags.Items)
	assert.Equal(t, StringType, tags.Items.Type)
}

// Benchmark tests for performance
func BenchmarkNewStringParameter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewStringParameter("Test parameter", true)
	}
}

func BenchmarkNewIntegerParameter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewIntegerParameter("Test parameter", false)
	}
}