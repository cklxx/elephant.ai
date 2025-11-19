package admin

import (
	"testing"

	runtimeconfig "alex/internal/config"
)

func TestDeriveReadinessTasks(t *testing.T) {
	t.Parallel()

	cfg := runtimeconfig.RuntimeConfig{}
	tasks := DeriveReadinessTasks(cfg)
	if len(tasks) != 4 {
		t.Fatalf("expected 4 tasks when config is empty, got %d", len(tasks))
	}

	cfg = runtimeconfig.RuntimeConfig{
		LLMProvider:    "mock",
		LLMModel:       "foo",
		SandboxBaseURL: "http://sandbox",
		TavilyAPIKey:   "tv",
	}
	tasks = DeriveReadinessTasks(cfg)
	if len(tasks) != 0 {
		t.Fatalf("expected no tasks when config complete with mock provider, got %d", len(tasks))
	}

	cfg = runtimeconfig.RuntimeConfig{
		LLMProvider: "openai",
		LLMModel:    "gpt-4",
	}
	tasks = DeriveReadinessTasks(cfg)
	if len(tasks) == 0 {
		t.Fatalf("expected tasks when provider requires api key and sandbox/tavily missing")
	}
	if tasks[0].Severity != TaskSeverityCritical {
		t.Fatalf("expected first task to be critical, got %s", tasks[0].Severity)
	}
}
