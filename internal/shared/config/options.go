package config

import "os"

// Option customises the loader behaviour.
type Option func(*loadOptions)

type loadOptions struct {
	envLookup  EnvLookup
	readFile   func(string) ([]byte, error)
	homeDir    func() (string, error)
	overrides  Overrides
	configPath string
}

// WithEnv supplies a custom environment lookup implementation.
func WithEnv(lookup EnvLookup) Option {
	return func(o *loadOptions) {
		o.envLookup = lookup
	}
}

// WithOverrides applies caller overrides that take highest precedence.
func WithOverrides(overrides Overrides) Option {
	return func(o *loadOptions) {
		o.overrides = overrides
	}
}

// WithConfigPath forces the loader to read configuration from a specific file.
func WithConfigPath(path string) Option {
	return func(o *loadOptions) {
		o.configPath = path
	}
}

// WithFileReader injects a custom reader, used primarily for tests.
func WithFileReader(reader func(string) ([]byte, error)) Option {
	return func(o *loadOptions) {
		o.readFile = reader
	}
}

// WithHomeDir overrides how the loader resolves the user's home directory.
func WithHomeDir(resolver func() (string, error)) Option {
	return func(o *loadOptions) {
		o.homeDir = resolver
	}
}

// DefaultEnvLookup delegates to os.LookupEnv.
func DefaultEnvLookup(key string) (string, bool) {
	return os.LookupEnv(key)
}

// Load constructs the runtime configuration by merging defaults, file, env, and overrides.
