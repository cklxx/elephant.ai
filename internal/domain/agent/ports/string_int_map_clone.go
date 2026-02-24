package ports

// CloneStringIntMap returns a deep copy of a map[string]int.
func CloneStringIntMap(src map[string]int) map[string]int {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]int, len(src))
	for key, value := range src {
		cloned[key] = value
	}
	return cloned
}
