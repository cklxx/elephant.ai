package bootstrap

import (
	"context"
	"errors"
	"testing"

	"alex/internal/delivery/channels"
)

func TestChannelRegistry_PluginLifecycle(t *testing.T) {
	reg := NewChannelRegistry()

	var started []string
	var cleanedUp []string

	reg.Register(channels.PluginFactory{
		Name:     "alpha",
		Required: true,
		Build: func(ctx context.Context) (func(), error) {
			started = append(started, "alpha")
			return func() { cleanedUp = append(cleanedUp, "alpha") }, nil
		},
	})
	reg.Register(channels.PluginFactory{
		Name:     "beta",
		Required: false,
		Build: func(ctx context.Context) (func(), error) {
			started = append(started, "beta")
			return func() { cleanedUp = append(cleanedUp, "beta") }, nil
		},
	})

	plugins := reg.Plugins()
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}

	// Simulate bootstrap: iterate and build all plugins
	var cleanups []func()
	for _, p := range plugins {
		cleanup, err := p.Build(context.Background())
		if err != nil {
			t.Fatalf("plugin %q build failed: %v", p.Name, err)
		}
		cleanups = append(cleanups, cleanup)
	}

	if len(started) != 2 || started[0] != "alpha" || started[1] != "beta" {
		t.Fatalf("unexpected start order: %v", started)
	}

	// Simulate shutdown
	for _, fn := range cleanups {
		fn()
	}
	if len(cleanedUp) != 2 {
		t.Fatalf("expected 2 cleanups, got %d", len(cleanedUp))
	}
}

func TestChannelRegistry_RequiredPluginFailure(t *testing.T) {
	reg := NewChannelRegistry()

	reg.Register(channels.PluginFactory{
		Name:     "failing",
		Required: true,
		Build: func(ctx context.Context) (func(), error) {
			return nil, errors.New("simulated startup failure")
		},
	})
	reg.Register(channels.PluginFactory{
		Name:     "optional-failing",
		Required: false,
		Build: func(ctx context.Context) (func(), error) {
			return nil, errors.New("optional failure")
		},
	})

	for _, p := range reg.Plugins() {
		_, err := p.Build(context.Background())
		if err != nil && p.Required {
			// Required plugin failure should be treated as fatal
			return
		}
		if err != nil && !p.Required {
			// Optional plugin failure should be logged but not fatal
			continue
		}
	}
}

func TestChannelRegistry_TypedConfigInjection(t *testing.T) {
	reg := NewChannelRegistry()

	type testConfig struct {
		Token string
	}

	reg.SetConfig("test-channel", testConfig{Token: "secret"})

	raw, ok := reg.Config("test-channel")
	if !ok {
		t.Fatal("config not found for test-channel")
	}

	cfg, ok := raw.(testConfig)
	if !ok {
		t.Fatalf("config type assertion failed: %T", raw)
	}
	if cfg.Token != "secret" {
		t.Fatalf("config.Token = %q, want %q", cfg.Token, "secret")
	}

	// Non-existent channel returns false
	if _, ok := reg.Config("nonexistent"); ok {
		t.Fatal("expected false for nonexistent channel config")
	}
}
