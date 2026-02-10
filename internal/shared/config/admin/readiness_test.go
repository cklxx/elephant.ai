package admin

import (
	"testing"

	runtimeconfig "alex/internal/shared/config"
)

func TestDeriveReadinessTasks(t *testing.T) {
	t.Parallel()

	cfg := runtimeconfig.RuntimeConfig{}
	tasks := DeriveReadinessTasks(cfg)
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks when config is empty, got %d", len(tasks))
	}

	cfg = runtimeconfig.RuntimeConfig{
		LLMProvider:  "mock",
		LLMModel:     "foo",
		TavilyAPIKey: "tv",
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
		t.Fatalf("expected tasks when provider requires api key and tavily missing")
	}
	if tasks[0].Severity != TaskSeverityCritical {
		t.Fatalf("expected first task to be critical, got %s", tasks[0].Severity)
	}

	cfg = runtimeconfig.RuntimeConfig{
		Profile:     runtimeconfig.RuntimeProfileQuickstart,
		LLMProvider: "openai",
		LLMModel:    "gpt-4",
	}
	tasks = DeriveReadinessTasks(cfg)
	var keyTask *ReadinessTask
	for i := range tasks {
		if tasks[i].ID == "llm-api-key" {
			keyTask = &tasks[i]
			break
		}
	}
	if keyTask == nil {
		t.Fatalf("expected llm-api-key readiness task in quickstart profile")
	}
	if keyTask.Severity != TaskSeverityWarning {
		t.Fatalf("expected llm-api-key task to be warning in quickstart profile, got %s", keyTask.Severity)
	}
}
