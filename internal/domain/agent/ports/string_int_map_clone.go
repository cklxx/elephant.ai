package ports

import "maps"

// CloneStringIntMap returns a deep copy of a map[string]int.
func CloneStringIntMap(src map[string]int) map[string]int {
	if len(src) == 0 {
		return nil
	}
	return maps.Clone(src)
}
