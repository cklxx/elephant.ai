package config

import (
	"strings"
	"testing"
)

func TestValidateConfigSchema_ValidConfig(t *testing.T) {
	yaml := `
runtime:
  llm_provider: "openai"
  llm_model: "gpt-4o"
  api_key: "sk-test"
  base_url: "https://api.openai.com/v1"
  max_tokens: 8192
  verbose: false
  temperature: 0.7
`
	warnings := ValidateConfigSchema([]byte(yaml))
	if len(warnings) > 0 {
		t.Errorf("expected no warnings for valid config, got: %v", warnings)
	}
}

func TestValidateConfigSchema_MissingRequiredRuntime(t *testing.T) {
	yaml := `
session:
  dir: "/tmp/sessions"
`
	warnings := ValidateConfigSchema([]byte(yaml))
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "runtime") && strings.Contains(w, "required") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning about missing required 'runtime', got: %v", warnings)
	}
}

func TestValidateConfigSchema_MissingRequiredRuntimeFields(t *testing.T) {
	yaml := `
runtime:
  verbose: true
`
	warnings := ValidateConfigSchema([]byte(yaml))
	var missingProvider, missingModel bool
	for _, w := range warnings {
		if strings.Contains(w, "llm_provider") && strings.Contains(w, "required") {
			missingProvider = true
		}
		if strings.Contains(w, "llm_model") && strings.Contains(w, "required") {
			missingModel = true
		}
	}
	if !missingProvider {
		t.Errorf("expected warning about missing runtime.llm_provider, got: %v", warnings)
	}
	if !missingModel {
		t.Errorf("expected warning about missing runtime.llm_model, got: %v", warnings)
	}
}

func TestValidateConfigSchema_TypeMismatchString(t *testing.T) {
	yaml := `
runtime:
  llm_provider: 123
  llm_model: "gpt-4o"
`
	warnings := ValidateConfigSchema([]byte(yaml))
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "llm_provider") && strings.Contains(w, "expected string") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected type mismatch warning for llm_provider, got: %v", warnings)
	}
}

func TestValidateConfigSchema_TypeMismatchBoolean(t *testing.T) {
	yaml := `
runtime:
  llm_provider: "openai"
  llm_model: "gpt-4o"
  verbose: "yes"
`
	warnings := ValidateConfigSchema([]byte(yaml))
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "verbose") && strings.Contains(w, "expected boolean") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected type mismatch warning for verbose, got: %v", warnings)
	}
}

func TestValidateConfigSchema_TypeMismatchInteger(t *testing.T) {
	yaml := `
runtime:
  llm_provider: "openai"
  llm_model: "gpt-4o"
  max_tokens: "not_a_number"
`
	warnings := ValidateConfigSchema([]byte(yaml))
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "max_tokens") && strings.Contains(w, "expected integer") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected type mismatch warning for max_tokens, got: %v", warnings)
	}
}

func TestValidateConfigSchema_NestedObjectValidation(t *testing.T) {
	yaml := `
runtime:
  llm_provider: "openai"
  llm_model: "gpt-4o"
  browser:
    headless: "not_bool"
    timeout_seconds: true
`
	warnings := ValidateConfigSchema([]byte(yaml))
	var headlessWarn, timeoutWarn bool
	for _, w := range warnings {
		if strings.Contains(w, "headless") && strings.Contains(w, "expected boolean") {
			headlessWarn = true
		}
		if strings.Contains(w, "timeout_seconds") && strings.Contains(w, "expected integer") {
			timeoutWarn = true
		}
	}
	if !headlessWarn {
		t.Errorf("expected type warning for browser.headless, got: %v", warnings)
	}
	if !timeoutWarn {
		t.Errorf("expected type warning for browser.timeout_seconds, got: %v", warnings)
	}
}

func TestValidateConfigSchema_ArrayItemValidation(t *testing.T) {
	yaml := `
runtime:
  llm_provider: "openai"
  llm_model: "gpt-4o"
  llm_fallback_rules:
    - fallback_provider: "ark"
      fallback_model: "model-x"
`
	warnings := ValidateConfigSchema([]byte(yaml))
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "model") && strings.Contains(w, "required") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning about missing required 'model' in fallback rule, got: %v", warnings)
	}
}

func TestValidateConfigSchema_InvalidYAML(t *testing.T) {
	// Use a YAML that fails to decode as map[string]any (tab indentation error).
	data := []byte("key:\n\t- bad")
	warnings := ValidateConfigSchema(data)
	if len(warnings) == 0 {
		t.Error("expected warning for invalid YAML")
	}
	if !strings.Contains(warnings[0], "parse YAML") {
		t.Errorf("expected YAML parse error, got: %s", warnings[0])
	}
}

func TestValidateConfigSchema_FullValidConfig(t *testing.T) {
	yaml := `
runtime:
  llm_provider: "openai"
  llm_model: "gpt-4o"
  api_key: "sk-test"
  base_url: "https://api.openai.com/v1"
  max_tokens: 8192
  verbose: false
  temperature: 0.7
  top_p: 1.0
  max_iterations: 100
  proactive:
    enabled: true
    prompt:
      mode: "full"
      timezone: "America/Los_Angeles"
    scheduler:
      enabled: false
      triggers:
        - name: "daily"
          schedule: "0 9 * * *"
          task: "hello"
  external_agents:
    max_parallel_agents: 4
    claude_code:
      enabled: false
      binary: "claude"
  llm_fallback_rules:
    - model: "claude-sonnet-4-6"
      fallback_provider: "openai"
      fallback_model: "gpt-4o"
session:
  dir: "/tmp/sessions"
`
	warnings := ValidateConfigSchema([]byte(yaml))
	if len(warnings) > 0 {
		t.Errorf("expected no warnings for full valid config, got: %v", warnings)
	}
}
