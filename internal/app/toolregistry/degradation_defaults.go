package toolregistry

// DefaultRegistryDegradationConfig returns a conservative built-in fallback
// map for production tool routing.
func DefaultRegistryDegradationConfig() DegradationConfig {
	cfg := DefaultDegradationConfig()
	cfg.MaxFallbackAttempts = 1
	cfg.FallbackMap = map[string][]string{
		"grep": {"ripgrep"},
	}
	return cfg
}
