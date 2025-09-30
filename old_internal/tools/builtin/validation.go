package builtin

import (
	"fmt"
	"reflect"
)

// ValidationRule defines a single validation rule for a parameter
type ValidationRule struct {
	FieldName   string
	Type        reflect.Type
	Required    bool
	Validator   func(interface{}) error
	Description string
}

// ValidationFramework provides common validation functionality
type ValidationFramework struct {
	rules []ValidationRule
}

// NewValidationFramework creates a new validation framework
func NewValidationFramework() *ValidationFramework {
	return &ValidationFramework{
		rules: make([]ValidationRule, 0),
	}
}

// AddRule adds a validation rule
func (vf *ValidationFramework) AddRule(rule ValidationRule) *ValidationFramework {
	vf.rules = append(vf.rules, rule)
	return vf
}

// AddStringField adds a required string field validation
func (vf *ValidationFramework) AddStringField(fieldName, description string) *ValidationFramework {
	return vf.AddRule(ValidationRule{
		FieldName:   fieldName,
		Type:        reflect.TypeOf(""),
		Required:    true,
		Description: description,
		Validator: func(value interface{}) error {
			if str, ok := value.(string); ok {
				if str == "" {
					return fmt.Errorf("%s cannot be empty", fieldName)
				}
				return nil
			}
			return fmt.Errorf("%s must be a string", fieldName)
		},
	})
}

// AddRequiredStringField adds a required string field that can be empty
func (vf *ValidationFramework) AddRequiredStringField(fieldName, description string) *ValidationFramework {
	return vf.AddRule(ValidationRule{
		FieldName:   fieldName,
		Type:        reflect.TypeOf(""),
		Required:    true,
		Description: description,
		Validator: func(value interface{}) error {
			if _, ok := value.(string); !ok {
				return fmt.Errorf("%s must be a string", fieldName)
			}
			return nil
		},
	})
}

// AddOptionalStringField adds an optional string field validation
func (vf *ValidationFramework) AddOptionalStringField(fieldName, description string) *ValidationFramework {
	return vf.AddRule(ValidationRule{
		FieldName:   fieldName,
		Type:        reflect.TypeOf(""),
		Required:    false,
		Description: description,
		Validator: func(value interface{}) error {
			if value == nil {
				return nil // Optional field
			}
			if str, ok := value.(string); ok {
				if str == "" {
					return fmt.Errorf("%s cannot be empty when provided", fieldName)
				}
				return nil
			}
			return fmt.Errorf("%s must be a string", fieldName)
		},
	})
}

// AddIntField adds a required integer field validation
func (vf *ValidationFramework) AddIntField(fieldName, description string, min, max int) *ValidationFramework {
	return vf.AddRule(ValidationRule{
		FieldName:   fieldName,
		Type:        reflect.TypeOf(0),
		Required:    true,
		Description: description,
		Validator: func(value interface{}) error {
			var intVal int
			switch v := value.(type) {
			case int:
				intVal = v
			case float64:
				intVal = int(v)
			default:
				return fmt.Errorf("%s must be an integer", fieldName)
			}

			if intVal < min {
				return fmt.Errorf("%s must be at least %d", fieldName, min)
			}
			if max > 0 && intVal > max {
				return fmt.Errorf("%s must be at most %d", fieldName, max)
			}
			return nil
		},
	})
}

// AddOptionalIntField adds an optional integer field validation
func (vf *ValidationFramework) AddOptionalIntField(fieldName, description string, min, max int) *ValidationFramework {
	return vf.AddRule(ValidationRule{
		FieldName:   fieldName,
		Type:        reflect.TypeOf(0),
		Required:    false,
		Description: description,
		Validator: func(value interface{}) error {
			if value == nil {
				return nil // Optional field
			}

			var intVal int
			switch v := value.(type) {
			case int:
				intVal = v
			case float64:
				intVal = int(v)
			default:
				return fmt.Errorf("%s must be an integer", fieldName)
			}

			if intVal < min {
				return fmt.Errorf("%s must be at least %d", fieldName, min)
			}
			if max > 0 && intVal > max {
				return fmt.Errorf("%s must be at most %d", fieldName, max)
			}
			return nil
		},
	})
}

// AddBoolField adds a boolean field validation
func (vf *ValidationFramework) AddBoolField(fieldName, description string, required bool) *ValidationFramework {
	return vf.AddRule(ValidationRule{
		FieldName:   fieldName,
		Type:        reflect.TypeOf(true),
		Required:    required,
		Description: description,
		Validator: func(value interface{}) error {
			if value == nil && !required {
				return nil // Optional field
			}
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("%s must be a boolean", fieldName)
			}
			return nil
		},
	})
}

// AddOptionalBooleanField adds an optional boolean field validation
func (vf *ValidationFramework) AddOptionalBooleanField(fieldName, description string) *ValidationFramework {
	return vf.AddBoolField(fieldName, description, false)
}

// AddOptionalArrayField adds an optional array field validation
func (vf *ValidationFramework) AddOptionalArrayField(fieldName, description string) *ValidationFramework {
	return vf.AddRule(ValidationRule{
		FieldName:   fieldName,
		Type:        reflect.TypeOf([]interface{}{}),
		Required:    false,
		Description: description,
		Validator: func(value interface{}) error {
			if value == nil {
				return nil // Optional field
			}
			if _, ok := value.([]interface{}); !ok {
				return fmt.Errorf("%s must be an array", fieldName)
			}
			return nil
		},
	})
}

// AddCustomValidator adds a custom validation rule
func (vf *ValidationFramework) AddCustomValidator(fieldName, description string, required bool, validator func(interface{}) error) *ValidationFramework {
	return vf.AddRule(ValidationRule{
		FieldName:   fieldName,
		Required:    required,
		Description: description,
		Validator:   validator,
	})
}

// Validate validates arguments against all rules
func (vf *ValidationFramework) Validate(args map[string]interface{}) error {
	if args == nil {
		return fmt.Errorf("arguments cannot be nil")
	}

	for _, rule := range vf.rules {
		value, exists := args[rule.FieldName]

		// Check required fields
		if rule.Required && !exists {
			return fmt.Errorf("%s is required", rule.FieldName)
		}

		// Skip validation for optional fields that are not present
		if !exists && !rule.Required {
			continue
		}

		// Run custom validator if provided
		if rule.Validator != nil {
			if err := rule.Validator(value); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetRequiredFields returns a list of required field names
func (vf *ValidationFramework) GetRequiredFields() []string {
	var required []string
	for _, rule := range vf.rules {
		if rule.Required {
			required = append(required, rule.FieldName)
		}
	}
	return required
}

// GetOptionalFields returns a list of optional field names
func (vf *ValidationFramework) GetOptionalFields() []string {
	var optional []string
	for _, rule := range vf.rules {
		if !rule.Required {
			optional = append(optional, rule.FieldName)
		}
	}
	return optional
}

// GetFieldDescription returns the description for a field
func (vf *ValidationFramework) GetFieldDescription(fieldName string) string {
	for _, rule := range vf.rules {
		if rule.FieldName == fieldName {
			return rule.Description
		}
	}
	return ""
}

// Common validation helpers

// ValidateFilePath validates that a file path is not empty and is a string
func ValidateFilePath(value interface{}) error {
	if value == nil {
		return fmt.Errorf("file_path is required")
	}

	filePath, ok := value.(string)
	if !ok {
		return fmt.Errorf("file_path must be a string")
	}

	if filePath == "" {
		return fmt.Errorf("file_path cannot be empty")
	}

	return nil
}

// ValidateCommand validates that a command is not empty and is a string
func ValidateCommand(value interface{}) error {
	if value == nil {
		return fmt.Errorf("command is required")
	}

	command, ok := value.(string)
	if !ok {
		return fmt.Errorf("command must be a string")
	}

	if command == "" {
		return fmt.Errorf("command cannot be empty")
	}

	return nil
}

// ValidatePattern validates that a search pattern is not empty and is a string
func ValidatePattern(value interface{}) error {
	if value == nil {
		return fmt.Errorf("pattern is required")
	}

	pattern, ok := value.(string)
	if !ok {
		return fmt.Errorf("pattern must be a string")
	}

	if pattern == "" {
		return fmt.Errorf("pattern cannot be empty")
	}

	return nil
}

// ValidateOptionalString validates an optional string field
func ValidateOptionalString(fieldName string) func(interface{}) error {
	return func(value interface{}) error {
		if value == nil {
			return nil // Optional field
		}

		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s must be a string", fieldName)
		}

		if str == "" {
			return fmt.Errorf("%s cannot be empty when provided", fieldName)
		}

		return nil
	}
}

// ValidatePositiveInt validates that a value is a positive integer
func ValidatePositiveInt(fieldName string) func(interface{}) error {
	return func(value interface{}) error {
		if value == nil {
			return nil // Optional field
		}

		var intVal int
		switch v := value.(type) {
		case int:
			intVal = v
		case float64:
			intVal = int(v)
		default:
			return fmt.Errorf("%s must be an integer", fieldName)
		}

		if intVal <= 0 {
			return fmt.Errorf("%s must be a positive integer", fieldName)
		}

		return nil
	}
}
