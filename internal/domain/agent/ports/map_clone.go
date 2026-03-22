package ports

import "maps"

// CloneStringMap returns a shallow clone of map[string]string.
func CloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	return maps.Clone(src)
}

// DeepCloneAnyMap deep copies a map[string]any, recursively cloning nested
// maps, slices and other composite values so the result shares no references
// with the original.
func DeepCloneAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(src))
	for key, value := range src {
		cloned[key] = deepCloneValue(value)
	}
	return cloned
}

func deepCloneValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return DeepCloneAnyMap(v)
	case []map[string]any:
		if len(v) == 0 {
			return nil
		}
		cloned := make([]map[string]any, len(v))
		for i := range v {
			cloned[i] = DeepCloneAnyMap(v[i])
		}
		return cloned
	case []string:
		return append([]string(nil), v...)
	case []any:
		if len(v) == 0 {
			return nil
		}
		cloned := make([]any, len(v))
		for i := range v {
			cloned[i] = deepCloneValue(v[i])
		}
		return cloned
	default:
		return v
	}
}

// CloneAnyMap is an alias for DeepCloneAnyMap for backward compatibility.
// Deprecated: Use DeepCloneAnyMap instead.
func CloneAnyMap(src map[string]any) map[string]any {
	return DeepCloneAnyMap(src)
}
