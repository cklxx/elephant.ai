package ports

import "maps"

// CloneStringMap returns a shallow clone of map[string]string.
func CloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	return maps.Clone(src)
}

// CloneAnyMap returns a shallow clone of map[string]any.
func CloneAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	return maps.Clone(src)
}
