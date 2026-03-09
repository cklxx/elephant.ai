package config

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed schema/config-schema.json
var schemaFS embed.FS

// ValidateConfigSchema validates raw YAML config bytes against the embedded
// JSON Schema (draft-07). It returns human-readable warnings for each
// violation. It never panics — callers should log warnings as appropriate.
func ValidateConfigSchema(yamlData []byte) []string {
	schemaBytes, err := schemaFS.ReadFile("schema/config-schema.json")
	if err != nil {
		return []string{fmt.Sprintf("config schema validation: load schema: %v", err)}
	}

	var schema map[string]any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return []string{fmt.Sprintf("config schema validation: parse schema: %v", err)}
	}

	var doc map[string]any
	if err := yaml.Unmarshal(yamlData, &doc); err != nil {
		return []string{fmt.Sprintf("config schema validation: parse YAML: %v", err)}
	}

	defs, _ := schema["$defs"].(map[string]any)
	return schemaValidateObject(doc, schema, defs, "")
}

// schemaValidateValue validates a single value against a schema node.
func schemaValidateValue(value any, schema map[string]any, defs map[string]any, path string) []string {
	// Resolve $ref.
	if ref, ok := schema["$ref"].(string); ok {
		resolved := schemaResolveRef(ref, defs)
		if resolved == nil {
			return []string{fmt.Sprintf("%s: unresolved $ref %q", path, ref)}
		}
		return schemaValidateValue(value, resolved, defs, path)
	}

	typeName, _ := schema["type"].(string)
	if value == nil {
		return nil
	}

	switch typeName {
	case "object":
		m, ok := schemaToMap(value)
		if !ok {
			return []string{fmt.Sprintf("%s: expected object, got %s", path, schemaDescribeType(value))}
		}
		return schemaValidateObject(m, schema, defs, path)

	case "array":
		arr, ok := schemaToSlice(value)
		if !ok {
			return []string{fmt.Sprintf("%s: expected array, got %s", path, schemaDescribeType(value))}
		}
		itemSchema, _ := schema["items"].(map[string]any)
		var warnings []string
		for i, elem := range arr {
			elemPath := fmt.Sprintf("%s[%d]", path, i)
			if itemSchema != nil {
				warnings = append(warnings, schemaValidateValue(elem, itemSchema, defs, elemPath)...)
			}
		}
		return warnings

	case "string":
		if !schemaIsString(value) {
			return []string{fmt.Sprintf("%s: expected string, got %s", path, schemaDescribeType(value))}
		}

	case "integer":
		if !schemaIsInteger(value) {
			return []string{fmt.Sprintf("%s: expected integer, got %s", path, schemaDescribeType(value))}
		}

	case "number":
		if !schemaIsNumber(value) {
			return []string{fmt.Sprintf("%s: expected number, got %s", path, schemaDescribeType(value))}
		}

	case "boolean":
		if !schemaIsBool(value) {
			return []string{fmt.Sprintf("%s: expected boolean, got %s", path, schemaDescribeType(value))}
		}
	}

	return nil
}

// schemaValidateObject checks required fields and property types.
func schemaValidateObject(obj map[string]any, schema map[string]any, defs map[string]any, path string) []string {
	var warnings []string

	if required, ok := schema["required"].([]any); ok {
		for _, r := range required {
			fieldName, _ := r.(string)
			if fieldName == "" {
				continue
			}
			if _, exists := obj[fieldName]; !exists {
				warnings = append(warnings, fmt.Sprintf("%s: required field missing", schemaJoinPath(path, fieldName)))
			}
		}
	}

	properties, _ := schema["properties"].(map[string]any)
	for key, val := range obj {
		fieldPath := schemaJoinPath(path, key)
		propSchema, hasProp := properties[key]
		if !hasProp {
			continue
		}
		propSchemaMap, ok := propSchema.(map[string]any)
		if !ok {
			continue
		}
		warnings = append(warnings, schemaValidateValue(val, propSchemaMap, defs, fieldPath)...)
	}

	return warnings
}

func schemaResolveRef(ref string, defs map[string]any) map[string]any {
	const prefix = "#/$defs/"
	if !strings.HasPrefix(ref, prefix) {
		return nil
	}
	name := ref[len(prefix):]
	def, ok := defs[name]
	if !ok {
		return nil
	}
	m, _ := def.(map[string]any)
	return m
}

func schemaJoinPath(base, field string) string {
	if base == "" {
		return field
	}
	return base + "." + field
}

func schemaToMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		result := make(map[string]any, len(m))
		for k, val := range m {
			result[fmt.Sprint(k)] = val
		}
		return result, true
	}
	return nil, false
}

func schemaToSlice(v any) ([]any, bool) {
	s, ok := v.([]any)
	return s, ok
}

func schemaIsString(v any) bool {
	_, ok := v.(string)
	return ok
}

func schemaIsInteger(v any) bool {
	switch n := v.(type) {
	case int:
		return true
	case int64:
		return true
	case float64:
		return n == float64(int64(n))
	}
	return false
}

func schemaIsNumber(v any) bool {
	switch v.(type) {
	case int, int64, float64:
		return true
	}
	return false
}

func schemaIsBool(v any) bool {
	_, ok := v.(bool)
	return ok
}

func schemaDescribeType(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case bool:
		return "bool"
	case int, int64:
		return "int"
	case float64:
		return "float"
	case map[string]any, map[any]any:
		return "object"
	case []any:
		return "array"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", v)
	}
}
