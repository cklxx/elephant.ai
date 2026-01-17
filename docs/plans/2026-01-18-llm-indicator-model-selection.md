# LLM Indicator Model Selection Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a clickable LLM indicator that can switch between YAML runtime config and CLI subscription models, backed by an internal model listing endpoint.

**Architecture:** Extend the internal config HTTP handler to return CLI model catalogs via provider `/models` APIs, expose this to the UI, and use managed overrides to switch `llm_provider`/`llm_model` without touching YAML. Keep overrides merge-safe by only modifying the relevant keys.

**Tech Stack:** Go (net/http, httptest), Next.js/React, Radix UI dropdowns, Vitest + Testing Library.

---

### Task 1: Export CLI credentials loader (config package)

**Files:**
- Modify: `internal/config/cli_auth.go`
- Modify: `internal/config/loader.go`
- Create: `internal/config/cli_auth_test.go`

**Step 1: Write the failing test**

```go
package config

import (
  "os"
  "path/filepath"
  "testing"
)

func TestLoadCLICredentialsReadsCodexAuth(t *testing.T) {
  t.Parallel()
  tmp := t.TempDir()
  codexDir := filepath.Join(tmp, ".codex")
  if err := os.MkdirAll(codexDir, 0o755); err != nil {
    t.Fatalf("mkdir: %v", err)
  }
  auth := `{"tokens":{"access_token":"tok-123","account_id":"acct"}}`
  if err := os.WriteFile(filepath.Join(codexDir, "auth.json"), []byte(auth), 0o600); err != nil {
    t.Fatalf("write auth: %v", err)
  }
  toml := "model = \"gpt-5-codex\"\n"
  if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte(toml), 0o600); err != nil {
    t.Fatalf("write toml: %v", err)
  }

  creds := LoadCLICredentials(
    WithHomeDir(func() (string, error) { return tmp, nil }),
    WithEnv(func(string) (string, bool) { return "", false }),
  )

  if creds.Codex.APIKey != "tok-123" {
    t.Fatalf("expected codex api key, got %q", creds.Codex.APIKey)
  }
  if creds.Codex.Model != "gpt-5-codex" {
    t.Fatalf("expected codex model, got %q", creds.Codex.Model)
  }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run TestLoadCLICredentialsReadsCodexAuth -v`
Expected: FAIL with `undefined: LoadCLICredentials`

**Step 3: Write minimal implementation**

```go
// internal/config/cli_auth.go

type CLICredential struct {
  Provider string
  APIKey   string
  BaseURL  string
  Model    string
  Source   ValueSource
}

type CLICredentials struct {
  Codex       CLICredential
  Claude      CLICredential
  Antigravity CLICredential
}

func LoadCLICredentials(opts ...Option) CLICredentials {
  options := loadOptions{
    envLookup: DefaultEnvLookup,
    readFile:  os.ReadFile,
    homeDir:   os.UserHomeDir,
  }
  for _, opt := range opts {
    opt(&options)
  }
  return loadCLICredentials(options)
}
```

```go
// internal/config/cli_auth.go (rename internal types)
// cliCredential -> CLICredential
// cliCredentials -> CLICredentials
// loadCLICredentials returns CLICredentials
```

```go
// internal/config/loader.go
// replace cliCredentials/loadCLICredentials with CLICredentials/LoadCLICredentials
cliCreds := CLICredentials{}
if shouldLoadCLICredentials(cfg) {
  cliCreds = LoadCLICredentials(options)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config -run TestLoadCLICredentialsReadsCodexAuth -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/cli_auth.go internal/config/loader.go internal/config/cli_auth_test.go
git commit -m "feat: export cli credential loader"
```

---

### Task 2: Add runtime model parsing + fetch helpers

**Files:**
- Create: `internal/server/http/runtime_models.go`
- Create: `internal/server/http/runtime_models_test.go`

**Step 1: Write the failing test**

```go
package http

import "testing"

func TestParseModelListHandlesDataObjects(t *testing.T) {
  input := []byte(`{"data":[{"id":"model-a"},{"id":"model-b"}]}`)
  models, err := parseModelList(input)
  if err != nil {
    t.Fatalf("parse error: %v", err)
  }
  if len(models) != 2 || models[0] != "model-a" || models[1] != "model-b" {
    t.Fatalf("unexpected models: %#v", models)
  }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/http -run TestParseModelListHandlesDataObjects -v`
Expected: FAIL with `undefined: parseModelList`

**Step 3: Write minimal implementation**

```go
package http

import (
  "encoding/json"
  "sort"
)

func parseModelList(raw []byte) ([]string, error) {
  var payload any
  if err := json.Unmarshal(raw, &payload); err != nil {
    return nil, err
  }

  models := map[string]struct{}{}
  if obj, ok := payload.(map[string]any); ok {
    if list, ok := obj["data"]; ok {
      extractModelIDs(list, models)
    }
    if list, ok := obj["models"]; ok {
      extractModelIDs(list, models)
    }
  }

  out := make([]string, 0, len(models))
  for id := range models {
    out = append(out, id)
  }
  sort.Strings(out)
  return out, nil
}

func extractModelIDs(value any, out map[string]struct{}) {
  list, ok := value.([]any)
  if !ok {
    return
  }
  for _, item := range list {
    switch v := item.(type) {
    case string:
      if v != "" {
        out[v] = struct{}{}
      }
    case map[string]any:
      if id, ok := v["id"].(string); ok && id != "" {
        out[id] = struct{}{}
        continue
      }
      if id, ok := v["model"].(string); ok && id != "" {
        out[id] = struct{}{}
      }
    }
  }
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/server/http -run TestParseModelListHandlesDataObjects -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/server/http/runtime_models.go internal/server/http/runtime_models_test.go
git commit -m "feat: add runtime model parsing helper"
```

---

### Task 3: Add provider fetch logic + tests

**Files:**
- Modify: `internal/server/http/runtime_models.go`
- Modify: `internal/server/http/runtime_models_test.go`

**Step 1: Write the failing test**

```go
func TestFetchProviderModelsUsesBearerAuth(t *testing.T) {
  var gotAuth string
  srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    gotAuth = r.Header.Get("Authorization")
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte(`{"data":[{"id":"model-x"}]}`))
  }))
  defer srv.Close()

  client := srv.Client()
  models, err := fetchProviderModels(context.Background(), client, modelFetchTarget{
    Provider: "codex",
    BaseURL:  srv.URL,
    APIKey:   "tok-abc",
  })
  if err != nil {
    t.Fatalf("fetch error: %v", err)
  }
  if gotAuth != "Bearer tok-abc" {
    t.Fatalf("expected bearer auth, got %q", gotAuth)
  }
  if len(models) != 1 || models[0] != "model-x" {
    t.Fatalf("unexpected models: %#v", models)
  }
}
```

```go
func TestFetchProviderModelsUsesAnthropicOAuthHeaders(t *testing.T) {
  var gotVersion string
  var gotBeta string
  var gotAuth string
  srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    gotVersion = r.Header.Get("anthropic-version")
    gotBeta = r.Header.Get("anthropic-beta")
    gotAuth = r.Header.Get("Authorization")
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte(`{"data":[{"id":"claude-3"}]}`))
  }))
  defer srv.Close()

  client := srv.Client()
  _, err := fetchProviderModels(context.Background(), client, modelFetchTarget{
    Provider: "anthropic",
    BaseURL:  srv.URL,
    APIKey:   "oauth-token",
  })
  if err != nil {
    t.Fatalf("fetch error: %v", err)
  }
  if gotAuth != "Bearer oauth-token" {
    t.Fatalf("expected oauth bearer auth, got %q", gotAuth)
  }
  if gotVersion == "" {
    t.Fatalf("expected anthropic-version header")
  }
  if !strings.Contains(gotBeta, "oauth-2025-04-20") {
    t.Fatalf("expected oauth beta header, got %q", gotBeta)
  }
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/server/http -run TestFetchProviderModels -v`
Expected: FAIL with `undefined: fetchProviderModels`

**Step 3: Write minimal implementation**

```go
type modelFetchTarget struct {
  Provider string
  BaseURL  string
  APIKey   string
}

func fetchProviderModels(ctx context.Context, client *http.Client, target modelFetchTarget) ([]string, error) {
  endpoint := strings.TrimRight(target.BaseURL, "/") + "/models"
  req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
  if err != nil {
    return nil, err
  }

  if target.Provider == "anthropic" || target.Provider == "claude" {
    if isAnthropicOAuthToken(target.APIKey) {
      req.Header.Set("Authorization", "Bearer "+target.APIKey)
      req.Header.Set("anthropic-beta", "oauth-2025-04-20")
    } else if target.APIKey != "" {
      req.Header.Set("x-api-key", target.APIKey)
    }
    req.Header.Set("anthropic-version", "2023-06-01")
  } else if target.APIKey != "" {
    req.Header.Set("Authorization", "Bearer "+target.APIKey)
  }

  resp, err := client.Do(req)
  if err != nil {
    return nil, err
  }
  defer func() { _ = resp.Body.Close() }()

  if resp.StatusCode < 200 || resp.StatusCode >= 300 {
    return nil, fmt.Errorf("model list request failed: %s", resp.Status)
  }

  body, err := io.ReadAll(resp.Body)
  if err != nil {
    return nil, err
  }

  return parseModelList(body)
}

func isAnthropicOAuthToken(token string) bool {
  token = strings.TrimSpace(token)
  if token == "" {
    return false
  }
  return !strings.HasPrefix(strings.ToLower(token), "sk-")
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/http -run TestFetchProviderModels -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/server/http/runtime_models.go internal/server/http/runtime_models_test.go
git commit -m "feat: fetch runtime models from providers"
```

---

### Task 4: Add model listing + handler wiring

**Files:**
- Modify: `internal/server/http/runtime_models.go`
- Modify: `internal/server/http/config_handler.go`
- Modify: `internal/server/http/router.go`
- Modify: `internal/server/http/config_handler_test.go`

**Step 1: Write the failing test**

```go
func TestConfigHandlerHandleGetRuntimeModels(t *testing.T) {
  t.Parallel()

  manager := configadmin.NewManager(&memoryStore{}, runtimeconfig.Overrides{})
  resolver := func(context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error) {
    return runtimeconfig.RuntimeConfig{}, runtimeconfig.Metadata{}, nil
  }

  handler := NewConfigHandler(manager, resolver)
  handler.modelLister = func(context.Context) []runtimeModelProvider {
    return []runtimeModelProvider{
      {Provider: "codex", Source: "codex_cli", Models: []string{"m1"}},
    }
  }

  req := httptest.NewRequest(http.MethodGet, "/api/internal/config/runtime/models", nil)
  rr := httptest.NewRecorder()

  handler.HandleGetRuntimeModels(rr, req)

  if rr.Code != http.StatusOK {
    t.Fatalf("expected status 200, got %d", rr.Code)
  }

  var payload runtimeModelsResponse
  if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
    t.Fatalf("decode response: %v", err)
  }
  if len(payload.Providers) != 1 || payload.Providers[0].Provider != "codex" {
    t.Fatalf("unexpected payload: %#v", payload)
  }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/http -run TestConfigHandlerHandleGetRuntimeModels -v`
Expected: FAIL with `handler.modelLister undefined` or `undefined: HandleGetRuntimeModels`

**Step 3: Write minimal implementation**

```go
// internal/server/http/config_handler.go

type RuntimeModelLister func(context.Context) []runtimeModelProvider

type ConfigHandler struct {
  manager     *configadmin.Manager
  resolver    RuntimeConfigResolver
  modelLister RuntimeModelLister
}

func NewConfigHandler(manager *configadmin.Manager, resolver RuntimeConfigResolver) *ConfigHandler {
  if manager == nil || resolver == nil {
    return nil
  }
  return &ConfigHandler{
    manager:     manager,
    resolver:    resolver,
    modelLister: defaultRuntimeModelLister,
  }
}

func (h *ConfigHandler) HandleGetRuntimeModels(w http.ResponseWriter, r *http.Request) {
  if h == nil {
    http.NotFound(w, r)
    return
  }
  lister := h.modelLister
  if lister == nil {
    lister = defaultRuntimeModelLister
  }
  payload := runtimeModelsResponse{Providers: lister(r.Context())}
  writeJSON(w, http.StatusOK, payload)
}
```

```go
// internal/server/http/runtime_models.go

func defaultRuntimeModelLister(ctx context.Context) []runtimeModelProvider {
  creds := runtimeconfig.LoadCLICredentials()
  logger := logging.NewComponentLogger("RuntimeModels")
  client := httpclient.New(20*time.Second, logger)
  return listRuntimeModels(ctx, creds, client)
}

func listRuntimeModels(ctx context.Context, creds runtimeconfig.CLICredentials, client *http.Client) []runtimeModelProvider {
  targets := []runtimeModelProvider{}

  if creds.Codex.APIKey != "" {
    targets = append(targets, runtimeModelProvider{
      Provider: creds.Codex.Provider,
      Source:   string(creds.Codex.Source),
      BaseURL:  creds.Codex.BaseURL,
    })
  }
  if creds.Antigravity.APIKey != "" {
    targets = append(targets, runtimeModelProvider{
      Provider: creds.Antigravity.Provider,
      Source:   string(creds.Antigravity.Source),
      BaseURL:  creds.Antigravity.BaseURL,
    })
  }
  if creds.Claude.APIKey != "" {
    targets = append(targets, runtimeModelProvider{
      Provider: creds.Claude.Provider,
      Source:   string(creds.Claude.Source),
      BaseURL:  creds.Claude.BaseURL,
    })
  }

  for i := range targets {
    target := &targets[i]
    models, err := fetchProviderModels(ctx, client, modelFetchTarget{
      Provider: target.Provider,
      BaseURL:  target.BaseURL,
      APIKey:   pickAPIKey(creds, target.Provider),
    })
    if err != nil {
      target.Error = err.Error()
      continue
    }
    target.Models = models
  }

  return targets
}

func pickAPIKey(creds runtimeconfig.CLICredentials, provider string) string {
  switch provider {
  case creds.Codex.Provider:
    return creds.Codex.APIKey
  case creds.Antigravity.Provider:
    return creds.Antigravity.APIKey
  case creds.Claude.Provider:
    return creds.Claude.APIKey
  default:
    return ""
  }
}
```

```go
// internal/server/http/router.go
if (internalMode || devMode) && configHandler != nil {
  // existing runtime config handlers
  mux.Handle("/api/internal/config/runtime/models", routeHandler("/api/internal/config/runtime/models", wrap(http.HandlerFunc(configHandler.HandleGetRuntimeModels))))
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/server/http -run TestConfigHandlerHandleGetRuntimeModels -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/server/http/runtime_models.go internal/server/http/config_handler.go internal/server/http/router.go internal/server/http/config_handler_test.go
git commit -m "feat: add runtime model list endpoint"
```

---

### Task 5: Add web API/types for model catalog

**Files:**
- Modify: `web/lib/types.ts`
- Modify: `web/lib/api.ts`

**Step 1: Write the failing test**

```ts
import { describe, it, expect, vi } from "vitest";
import { getRuntimeModelCatalog } from "@/lib/api";

vi.mock("@/lib/api-base", () => ({ buildApiUrl: (path: string) => path }));

global.fetch = vi.fn().mockResolvedValue({
  ok: true,
  status: 200,
  headers: new Headers(),
  json: async () => ({ providers: [] }),
}) as any;

describe("getRuntimeModelCatalog", () => {
  it("fetches the runtime model catalog", async () => {
    const result = await getRuntimeModelCatalog();
    expect(result.providers).toEqual([]);
    expect(fetch).toHaveBeenCalledWith(
      "/api/internal/config/runtime/models",
      expect.objectContaining({ credentials: "include" }),
    );
  });
});
```

**Step 2: Run test to verify it fails**

Run: `cd web && npm test -- --runTestsByPath lib/__tests__/runtime-models.test.ts`
Expected: FAIL with `Cannot find module` or `getRuntimeModelCatalog is not a function`

**Step 3: Write minimal implementation**

```ts
// web/lib/types.ts
export interface RuntimeModelProvider {
  provider: string;
  source: string;
  base_url?: string;
  models?: string[];
  error?: string;
}

export interface RuntimeModelCatalog {
  providers: RuntimeModelProvider[];
}
```

```ts
// web/lib/api.ts
export async function getRuntimeModelCatalog(): Promise<RuntimeModelCatalog> {
  return fetchAPI<RuntimeModelCatalog>("/api/internal/config/runtime/models");
}
```

**Step 4: Run test to verify it passes**

Run: `cd web && npm test -- --runTestsByPath lib/__tests__/runtime-models.test.ts`
Expected: PASS

**Step 5: Commit**

```bash
git add web/lib/types.ts web/lib/api.ts web/lib/__tests__/runtime-models.test.ts
git commit -m "feat(web): add runtime model catalog api"
```

---

### Task 6: Update LLM indicator UI + tests

**Files:**
- Modify: `web/components/agent/LLMIndicator.tsx`
- Create: `web/components/agent/__tests__/LLMIndicator.test.tsx`

**Step 1: Write the failing test**

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { LLMIndicator } from "@/components/agent/LLMIndicator";

vi.mock("@/lib/api", () => ({
  getRuntimeConfigSnapshot: vi.fn().mockResolvedValue({
    effective: { llm_provider: "codex", llm_model: "model-a" },
    overrides: {},
    sources: { api_key: "codex_cli" },
  }),
  updateRuntimeConfig: vi.fn().mockResolvedValue({
    effective: { llm_provider: "codex", llm_model: "model-a" },
    overrides: {},
    sources: { api_key: "codex_cli" },
  }),
  getRuntimeModelCatalog: vi.fn().mockResolvedValue({
    providers: [{ provider: "codex", source: "codex_cli", models: ["model-a"] }],
  }),
}));

describe("LLMIndicator", () => {
  it("loads CLI models when opened", async () => {
    render(<LLMIndicator />);
    const trigger = await screen.findByRole("button", { name: /llm/i });
    await userEvent.click(trigger);
    expect(await screen.findByText("model-a")).toBeInTheDocument();
  });
});
```

**Step 2: Run test to verify it fails**

Run: `cd web && npm test -- --runTestsByPath components/agent/__tests__/LLMIndicator.test.tsx`
Expected: FAIL with missing dropdown markup or missing role

**Step 3: Write minimal implementation**

```tsx
// web/components/agent/LLMIndicator.tsx
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { getRuntimeConfigSnapshot, getRuntimeModelCatalog, updateRuntimeConfig } from "@/lib/api";

// add state for menu + models
const [menuOpen, setMenuOpen] = useState(false);
const [modelsState, setModelsState] = useState<"idle" | "loading" | "error">("idle");
const [models, setModels] = useState<RuntimeModelProvider[]>([]);

const loadModels = useCallback(async () => {
  setModelsState("loading");
  try {
    const data = await getRuntimeModelCatalog();
    setModels(data.providers ?? []);
    setModelsState("idle");
  } catch (error) {
    setModelsState("error");
  }
}, []);

const handleSelectYaml = useCallback(async () => {
  if (!snapshot) return;
  const next: RuntimeConfigOverrides = { ...(snapshot.overrides ?? {}) };
  delete next.llm_provider;
  delete next.llm_model;
  const payload = await updateRuntimeConfig({ overrides: next });
  setSnapshot(payload);
}, [snapshot]);

const handleSelectModel = useCallback(async (provider: string, model: string) => {
  if (!snapshot) return;
  const next: RuntimeConfigOverrides = {
    ...(snapshot.overrides ?? {}),
    llm_provider: provider,
    llm_model: model,
  };
  const payload = await updateRuntimeConfig({ overrides: next });
  setSnapshot(payload);
}, [snapshot]);

// render DropdownMenu wrapping the chip
```

**Step 4: Run test to verify it passes**

Run: `cd web && npm test -- --runTestsByPath components/agent/__tests__/LLMIndicator.test.tsx`
Expected: PASS

**Step 5: Commit**

```bash
git add web/components/agent/LLMIndicator.tsx web/components/agent/__tests__/LLMIndicator.test.tsx
git commit -m "feat(web): allow model selection from indicator"
```

---

### Task 7: Full validation

**Files:**
- None

**Step 1: Run Go tests**

Run: `go test ./...`
Expected: PASS

**Step 2: Run web lint + tests**

Run: `cd web && npm run lint`
Expected: PASS

Run: `cd web && npm test`
Expected: PASS

**Step 3: Commit if needed**

```bash
git status -sb
```
Expected: clean working tree

---

Plan complete and saved to `docs/plans/2026-01-18-llm-indicator-model-selection.md`. Two execution options:

1. Subagent-Driven (this session) - I dispatch fresh subagent per task, review between tasks, fast iteration
2. Parallel Session (separate) - Open new session with executing-plans, batch execution with checkpoints

Which approach?
