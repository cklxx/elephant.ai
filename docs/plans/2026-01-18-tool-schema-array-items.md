# Tool Schema Array Items Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ensure all tool definitions, including optional tools like explore and vision_analyze, declare array item schemas so OpenAI-compatible providers accept tool schemas.

**Architecture:** Add a registry test that registers optional tools and enforces array items. Update tool definitions to include `items` for any array-typed parameters (explore scopes, seedream vision images). Keep changes localized to tool schema definitions.

**Tech Stack:** Go, internal tool registry, Go testing (`go test`).

### Task 1: Add failing test for optional tool schemas

**Files:**
- Modify: `internal/toolregistry/registry_test.go`

**Step 1: Write the failing test**

```go
func TestToolDefinitionsArrayItemsIncludesOptionalTools(t *testing.T) {
	registry, err := NewRegistry(Config{
		MemoryService:       newTestMemoryService(),
		SeedreamVisionModel: "seedream-vision",
	})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	registry.RegisterSubAgent(stubCoordinator{})

	defs := registry.List()
	for _, def := range defs {
		for name, prop := range def.Parameters.Properties {
			if prop.Type != "array" {
				continue
			}
			if prop.Items == nil {
				t.Fatalf("tool %s property %s missing items schema", def.Name, name)
			}
		}
	}
}

type stubCoordinator struct{}

func (stubCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener ports.EventListener) (*ports.TaskResult, error) {
	return nil, nil
}

func (stubCoordinator) PrepareExecution(ctx context.Context, task string, sessionID string) (*ports.ExecutionEnvironment, error) {
	return nil, nil
}

func (stubCoordinator) SaveSessionAfterExecution(ctx context.Context, session *ports.Session, result *ports.TaskResult) error {
	return nil
}

func (stubCoordinator) ListSessions(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (stubCoordinator) GetConfig() ports.AgentConfig {
	return ports.AgentConfig{}
}

func (stubCoordinator) GetLLMClient() (ports.LLMClient, error) {
	return nil, nil
}

func (stubCoordinator) GetToolRegistryWithoutSubagent() ports.ToolRegistry {
	return nil
}

func (stubCoordinator) GetParser() ports.FunctionCallParser {
	return nil
}

func (stubCoordinator) GetContextManager() ports.ContextManager {
	return nil
}

func (stubCoordinator) GetSystemPrompt() string {
	return ""
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/toolregistry -run TestToolDefinitionsArrayItemsIncludesOptionalTools -v`
Expected: FAIL with a missing items schema error (e.g., `tool explore property local_scope missing items schema`).

### Task 2: Add missing array item schemas for optional tools

**Files:**
- Modify: `internal/tools/builtin/explore.go`
- Modify: `internal/tools/builtin/seedream.go`

**Step 1: Write minimal implementation**

```go
"local_scope": {
	Type:        "array",
	Description: "Specific local/codebase areas to inspect.",
	Items:       &ports.Property{Type: "string"},
},
"web_scope": {
	Type:        "array",
	Description: "Web research focus areas.",
	Items:       &ports.Property{Type: "string"},
},
"custom_tasks": {
	Type:        "array",
	Description: "Additional custom subtasks to run.",
	Items:       &ports.Property{Type: "string"},
},
```

```go
"images": {
	Type:        "array",
	Description: "List of image URLs or data URIs to analyze.",
	Items:       &ports.Property{Type: "string"},
},
```

**Step 2: Run test to verify it passes**

Run: `go test ./internal/toolregistry -run TestToolDefinitionsArrayItemsIncludesOptionalTools -v`
Expected: PASS

**Step 3: Run full test/lint validation**

Run: `go test ./...`
Expected: PASS

Run: `npm run lint`
Expected: PASS (warnings acceptable if already present)

Run: `npm test`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/toolregistry/registry_test.go internal/tools/builtin/explore.go internal/tools/builtin/seedream.go
git commit -m "fix: add array item schemas for optional tools"
```
