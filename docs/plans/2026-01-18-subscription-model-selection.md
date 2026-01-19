# Subscription Model Selection (Client-Scoped) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make model selection client-scoped (per browser) by using local selection + request-scoped overrides, without writing managed overrides on the server.

**Architecture:** Add a subscription catalog service that lists CLI models and a selection resolver that injects per-request LLM overrides via context. The UI stores selection in localStorage, shows YAML vs CLI sources, and attaches `llm_selection` to task creation. Execution uses the resolved provider/model and bypasses small/vision switching when pinned.

**Tech Stack:** Go (net/http, context), React/Next.js, Vitest, Testing Library.

---

### Task 1: Add subscription catalog parsing + fetch helpers

**Files:**
- Create: `internal/subscription/catalog.go`
- Create: `internal/subscription/catalog_test.go`

**Step 1: Write the failing tests**

```go
package subscription

import (
  "context"
  "net/http"
  "net/http/httptest"
  "strings"
  "testing"
)

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

func TestFetchProviderModelsUsesBearerAuth(t *testing.T) {
  var gotAuth string
  srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    gotAuth = r.Header.Get("Authorization")
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte(`{"data":[{"id":"model-x"}]}`))
  }))
  defer srv.Close()

  client := srv.Client()
  models, err := fetchProviderModels(context.Background(), client, fetchTarget{
    provider: "codex",
    baseURL:  srv.URL,
    apiKey:   "tok-abc",
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
  _, err := fetchProviderModels(context.Background(), client, fetchTarget{
    provider: "anthropic",
    baseURL:  srv.URL,
    apiKey:   "oauth-token",
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

**Step 2: Run test to verify it fails**

Run: `go test ./internal/subscription -run TestParseModelListHandlesDataObjects -v`
Expected: FAIL with undefined symbols (package missing).

**Step 3: Write minimal implementation**

```go
package subscription

import (
  "context"
  "encoding/json"
  "fmt"
  "io"
  "net/http"
  "sort"
  "strings"
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

type fetchTarget struct {
  provider  string
  baseURL   string
  apiKey    string
  accountID string
}

func fetchProviderModels(ctx context.Context, client *http.Client, target fetchTarget) ([]string, error) {
  endpoint := strings.TrimRight(target.baseURL, "/") + "/models"
  req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
  if err != nil {
    return nil, err
  }
  if target.accountID != "" {
    req.Header.Set("ChatGPT-Account-Id", target.accountID)
  }
  if target.provider == "anthropic" || target.provider == "claude" {
    if isAnthropicOAuthToken(target.apiKey) {
      req.Header.Set("Authorization", "Bearer "+target.apiKey)
      req.Header.Set("anthropic-beta", "oauth-2025-04-20")
    } else if target.apiKey != "" {
      req.Header.Set("x-api-key", target.apiKey)
    }
    req.Header.Set("anthropic-version", "2023-06-01")
  } else if target.apiKey != "" {
    req.Header.Set("Authorization", "Bearer "+target.apiKey)
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

Run: `go test ./internal/subscription -run TestParseModelListHandlesDataObjects -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/subscription/catalog.go internal/subscription/catalog_test.go
git commit -m "feat: add subscription catalog parsing helpers"
```

---

### Task 2: Add catalog service with Codex CLI fallback + cache

**Files:**
- Modify: `internal/subscription/catalog.go`
- Modify: `internal/subscription/catalog_test.go`

**Step 1: Write the failing test**

```go
func TestCatalogServiceUsesCodexFallbackWithoutNetwork(t *testing.T) {
  client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
    t.Fatalf("unexpected network request")
    return nil, nil
  })}

  svc := NewCatalogService(func() CLICredentials {
    return CLICredentials{
      Codex: CLICredential{
        Provider: "codex",
        APIKey:   "tok-abc",
        Model:    "gpt-5.2-codex",
        BaseURL:  "https://chatgpt.com/backend-api/codex",
        Source:   "codex_cli",
      },
    }
  }, client, 0)

  catalog := svc.Catalog(context.Background())
  if len(catalog.Providers) != 1 {
    t.Fatalf("expected one provider, got %d", len(catalog.Providers))
  }
  got := catalog.Providers[0]
  if got.Error != "" {
    t.Fatalf("expected no error, got %q", got.Error)
  }
  if len(got.Models) == 0 || got.Models[0] == "" {
    t.Fatalf("expected fallback models, got %#v", got.Models)
  }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/subscription -run TestCatalogServiceUsesCodexFallbackWithoutNetwork -v`
Expected: FAIL with undefined symbols.

**Step 3: Write minimal implementation**

```go
type CatalogProvider struct {
  Provider string   `json:"provider"`
  Source   string   `json:"source"`
  BaseURL  string   `json:"base_url,omitempty"`
  Models   []string `json:"models,omitempty"`
  Error    string   `json:"error,omitempty"`
}

type Catalog struct {
  Providers []CatalogProvider `json:"providers"`
}

type CLICredential struct {
  Provider  string
  APIKey    string
  AccountID string
  BaseURL   string
  Model     string
  Source    string
}

type CLICredentials struct {
  Codex       CLICredential
  Claude      CLICredential
  Antigravity CLICredential
}

type CatalogService struct {
  loadCreds func() CLICredentials
  client    *http.Client
  ttl       time.Duration
  mu        sync.Mutex
  cached    Catalog
  cachedAt  time.Time
}

func NewCatalogService(load func() CLICredentials, client *http.Client, ttl time.Duration) *CatalogService {
  return &CatalogService{loadCreds: load, client: client, ttl: ttl}
}

func (s *CatalogService) Catalog(ctx context.Context) Catalog {
  s.mu.Lock()
  defer s.mu.Unlock()
  if s.ttl > 0 && time.Since(s.cachedAt) < s.ttl {
    return s.cached
  }

  creds := s.loadCreds()
  catalog := Catalog{Providers: listProviders(ctx, creds, s.client)}
  s.cached = catalog
  s.cachedAt = time.Now()
  return catalog
}

func listProviders(ctx context.Context, creds CLICredentials, client *http.Client) []CatalogProvider {
  // build providers from creds, use codex fallback for codex_cli
}

func codexFallbackModels(cliModel string) []string {
  // static allowlist + optional cliModel, sorted
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/subscription -run TestCatalogServiceUsesCodexFallbackWithoutNetwork -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/subscription/catalog.go internal/subscription/catalog_test.go
git commit -m "feat: add subscription catalog service"
```

---

### Task 3: Add selection resolver

**Files:**
- Create: `internal/subscription/selection.go`
- Create: `internal/subscription/selection_test.go`

**Step 1: Write the failing tests**

```go
package subscription

import "testing"

func TestResolveSelectionForCodexCLI(t *testing.T) {
  resolver := NewSelectionResolver(func() CLICredentials {
    return CLICredentials{
      Codex: CLICredential{
        Provider:  "codex",
        APIKey:    "tok-abc",
        AccountID: "acct-1",
        BaseURL:   "https://chatgpt.com/backend-api/codex",
        Source:    "codex_cli",
      },
    }
  })

  selection := Selection{Mode: "cli", Provider: "codex", Model: "gpt-5.2-codex", Source: "codex_cli"}
  resolved, ok := resolver.Resolve(selection)
  if !ok {
    t.Fatalf("expected selection to resolve")
  }
  if resolved.Provider != "codex" || resolved.Model != "gpt-5.2-codex" {
    t.Fatalf("unexpected resolution: %#v", resolved)
  }
  if resolved.APIKey != "tok-abc" || resolved.BaseURL == "" {
    t.Fatalf("expected api key + base url")
  }
  if resolved.Headers["ChatGPT-Account-Id"] != "acct-1" {
    t.Fatalf("expected account header")
  }
}

func TestResolveSelectionIgnoresYamlMode(t *testing.T) {
  resolver := NewSelectionResolver(func() CLICredentials { return CLICredentials{} })
  if _, ok := resolver.Resolve(Selection{Mode: "yaml"}); ok {
    t.Fatalf("expected yaml selection to be ignored")
  }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/subscription -run TestResolveSelectionForCodexCLI -v`
Expected: FAIL with undefined symbols.

**Step 3: Write minimal implementation**

```go
package subscription

import "strings"

type Selection struct {
  Mode     string `json:"mode"`
  Provider string `json:"provider"`
  Model    string `json:"model"`
  Source   string `json:"source"`
}

type ResolvedSelection struct {
  Provider string
  Model    string
  APIKey   string
  BaseURL  string
  Headers  map[string]string
  Source   string
  Pinned   bool
}

type SelectionResolver struct {
  loadCreds func() CLICredentials
}

func NewSelectionResolver(load func() CLICredentials) *SelectionResolver {
  return &SelectionResolver{loadCreds: load}
}

func (r *SelectionResolver) Resolve(selection Selection) (ResolvedSelection, bool) {
  if strings.ToLower(strings.TrimSpace(selection.Mode)) != "cli" {
    return ResolvedSelection{}, false
  }
  provider := strings.TrimSpace(strings.ToLower(selection.Provider))
  model := strings.TrimSpace(selection.Model)
  if provider == "" || model == "" {
    return ResolvedSelection{}, false
  }
  creds := r.loadCreds()
  switch provider {
  case creds.Codex.Provider:
    if creds.Codex.APIKey == "" {
      return ResolvedSelection{}, false
    }
    headers := map[string]string{}
    if creds.Codex.AccountID != "" {
      headers["ChatGPT-Account-Id"] = creds.Codex.AccountID
    }
    return ResolvedSelection{
      Provider: provider,
      Model:    model,
      APIKey:   creds.Codex.APIKey,
      BaseURL:  creds.Codex.BaseURL,
      Headers:  headers,
      Source:   string(creds.Codex.Source),
      Pinned:   true,
    }, true
  case creds.Claude.Provider:
    if creds.Claude.APIKey == "" {
      return ResolvedSelection{}, false
    }
    return ResolvedSelection{
      Provider: provider,
      Model:    model,
      APIKey:   creds.Claude.APIKey,
      BaseURL:  creds.Claude.BaseURL,
      Source:   string(creds.Claude.Source),
      Pinned:   true,
    }, true
  case creds.Antigravity.Provider:
    if creds.Antigravity.APIKey == "" {
      return ResolvedSelection{}, false
    }
    return ResolvedSelection{
      Provider: provider,
      Model:    model,
      APIKey:   creds.Antigravity.APIKey,
      BaseURL:  creds.Antigravity.BaseURL,
      Source:   string(creds.Antigravity.Source),
      Pinned:   true,
    }, true
  default:
    return ResolvedSelection{}, false
  }
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/subscription -run TestResolveSelectionForCodexCLI -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/subscription/selection.go internal/subscription/selection_test.go
git commit -m "feat: add subscription selection resolver"
```

---

### Task 4: Inject selection into request context

**Files:**
- Create: `internal/agent/app/llm_selection.go`
- Modify: `internal/server/http/api_handler.go`
- Modify: `internal/server/http/api_handler_test.go`

**Step 1: Write the failing test**

```go
type selectionAwareCoordinator struct {
  selection subscription.ResolvedSelection
  got       chan struct{}
}

func (c *selectionAwareCoordinator) GetSession(ctx context.Context, id string) (*agentPorts.Session, error) {
  if id == "" {
    id = "stub"
  }
  return &agentPorts.Session{ID: id, Metadata: map[string]string{}}, nil
}

func (c *selectionAwareCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener agentPorts.EventListener) (*agentPorts.TaskResult, error) {
  if sel, ok := app.GetLLMSelection(ctx); ok {
    c.selection = sel
  }
  close(c.got)
  return &agentPorts.TaskResult{SessionID: sessionID}, nil
}

func (c *selectionAwareCoordinator) GetConfig() agentPorts.AgentConfig { return agentPorts.AgentConfig{} }
func (c *selectionAwareCoordinator) PreviewContextWindow(ctx context.Context, sessionID string) (agentPorts.ContextWindowPreview, error) {
  return agentPorts.ContextWindowPreview{}, nil
}

func TestHandleCreateTaskInjectsSelection(t *testing.T) {
  coord := &selectionAwareCoordinator{got: make(chan struct{})}
  server := app.NewServerCoordinator(coord, app.NewEventBroadcaster(), nil, app.NewInMemoryTaskStore(), nil)
  handler := NewAPIHandler(server, app.NewHealthChecker(), false, WithSelectionResolver(subscription.NewSelectionResolver(func() subscription.CLICredentials {
    return subscription.CLICredentials{Codex: subscription.CLICredential{Provider: "codex", APIKey: "tok", BaseURL: "https://chatgpt.com/backend-api/codex", Source: "codex_cli"}}
  })))

  body := `{"task":"hi","llm_selection":{"mode":"cli","provider":"codex","model":"gpt-5.2-codex","source":"codex_cli"}}`
  req := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(body))
  rr := httptest.NewRecorder()

  handler.HandleCreateTask(rr, req)
  <-coord.got

  if coord.selection.Provider != "codex" || coord.selection.Model != "gpt-5.2-codex" {
    t.Fatalf("selection not injected: %#v", coord.selection)
  }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/http -run TestHandleCreateTaskInjectsSelection -v`
Expected: FAIL with undefined symbols.

**Step 3: Write minimal implementation**

```go
// internal/agent/app/llm_selection.go
package app

import (
  "context"

  "alex/internal/subscription"
)

type llmSelectionKey struct{}

func WithLLMSelection(ctx context.Context, selection subscription.ResolvedSelection) context.Context {
  return context.WithValue(ctx, llmSelectionKey{}, selection)
}

func GetLLMSelection(ctx context.Context) (subscription.ResolvedSelection, bool) {
  if ctx == nil {
    return subscription.ResolvedSelection{}, false
  }
  selection, ok := ctx.Value(llmSelectionKey{}).(subscription.ResolvedSelection)
  return selection, ok
}
```

```go
// internal/server/http/api_handler.go
import "alex/internal/subscription"
import agentapp "alex/internal/agent/app"

// CreateTaskRequest
LLMSelection *subscription.Selection `json:"llm_selection,omitempty"`

// APIHandler
selectionResolver *subscription.SelectionResolver

// option
func WithSelectionResolver(resolver *subscription.SelectionResolver) APIHandlerOption { ... }

// in NewAPIHandler
if handler.selectionResolver == nil {
  handler.selectionResolver = subscription.NewSelectionResolver(runtimeconfig.LoadCLICredentials)
}

// in HandleCreateTask
ctx := r.Context()
if req.LLMSelection != nil && h.selectionResolver != nil {
  if resolved, ok := h.selectionResolver.Resolve(*req.LLMSelection); ok {
    ctx = agentapp.WithLLMSelection(ctx, resolved)
  }
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/http -run TestHandleCreateTaskInjectsSelection -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/app/llm_selection.go internal/server/http/api_handler.go internal/server/http/api_handler_test.go
git commit -m "feat: inject llm selection into task context"
```

---

### Task 5: Use selection inside execution preparation

**Files:**
- Modify: `internal/agent/app/execution_preparation_service.go`
- Modify: `internal/agent/app/task_preanalysis_routing_test.go`

**Step 1: Write the failing test**

```go
func TestPrepareUsesPinnedSelectionAndSkipsSmallModel(t *testing.T) {
  session := &ports.Session{ID: "session-pinned", Messages: nil, Metadata: map[string]string{}}
  store := &stubSessionStore{session: session}
  factory := &recordingLLMFactory{}

  service := NewExecutionPreparationService(ExecutionPreparationDeps{
    LLMFactory:   factory,
    ToolRegistry: &registryWithList{defs: []ports.ToolDefinition{{Name: "shell"}}},
    SessionStore: store,
    ContextMgr:   stubContextManager{},
    Parser:       stubParser{},
    Config: Config{
      LLMProvider:      "mock-default",
      LLMModel:         "default-model",
      LLMSmallProvider: "mock-small",
      LLMSmallModel:    "small-model",
      MaxIterations:    3,
    },
    Logger:       ports.NoopLogger{},
    EventEmitter: ports.NoopEventListener{},
  })

  ctx := WithLLMSelection(context.Background(), subscription.ResolvedSelection{
    Provider: "codex",
    Model:    "gpt-5.2-codex",
    APIKey:   "tok",
    BaseURL:  "https://chatgpt.com/backend-api/codex",
    Headers:  map[string]string{"ChatGPT-Account-Id": "acct"},
    Pinned:   true,
  })

  _, err := service.Prepare(ctx, "Do the thing", session.ID)
  if err != nil {
    t.Fatalf("prepare failed: %v", err)
  }

  modelCalls := factory.CallModels()
  if len(modelCalls) != 1 || modelCalls[0] != "codex|gpt-5.2-codex" {
    t.Fatalf("expected pinned model only, got %v", modelCalls)
  }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/app -run TestPrepareUsesPinnedSelectionAndSkipsSmallModel -v`
Expected: FAIL with wrong model call.

**Step 3: Write minimal implementation**

```go
selection, hasSelection := GetLLMSelection(ctx)
selectionPinned := hasSelection && selection.Pinned && selection.Provider != "" && selection.Model != ""

var taskAnalysis *ports.TaskAnalysis
var preferSmallModel bool
if selectionPinned {
  if analysis, _, ok := quickTriageTask(task); ok {
    taskAnalysis = analysis
  }
} else {
  taskAnalysis, preferSmallModel = s.preAnalyzeTask(ctx, session, task)
}

// existing session title logic stays

if selectionPinned {
  effectiveProvider = selection.Provider
  effectiveModel = selection.Model
} else if preferSmallModel && strings.TrimSpace(s.config.LLMSmallModel) != "" {
  ...
}
if !selectionPinned && taskNeedsVision(...) { ... }

llmConfig := ports.LLMConfig{APIKey: s.config.APIKey, BaseURL: s.config.BaseURL}
if selectionPinned {
  llmConfig.APIKey = selection.APIKey
  llmConfig.BaseURL = selection.BaseURL
  llmConfig.Headers = selection.Headers
}
llmClient, err := s.llmFactory.GetIsolatedClient(effectiveProvider, effectiveModel, llmConfig)
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/agent/app -run TestPrepareUsesPinnedSelectionAndSkipsSmallModel -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/app/execution_preparation_service.go internal/agent/app/task_preanalysis_routing_test.go
git commit -m "feat: honor pinned llm selection during preparation"
```

---

### Task 6: Catalog endpoint + runtime models compatibility

**Files:**
- Modify: `internal/server/http/config_handler.go`
- Modify: `internal/server/http/router.go`
- Modify: `internal/server/http/runtime_models_test.go`

**Step 1: Write the failing test**

```go
func TestHandleGetSubscriptionCatalogUsesCatalogService(t *testing.T) {
  handler := NewConfigHandler(configadmin.NewManager(&memoryStore{}, runtimeconfig.Overrides{}), func(context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error) {
    return runtimeconfig.RuntimeConfig{}, runtimeconfig.Metadata{}, nil
  })
  handler.catalogService = &stubCatalogService{catalog: subscription.Catalog{Providers: []subscription.CatalogProvider{{Provider: "codex"}}}}

  req := httptest.NewRequest(http.MethodGet, "/api/internal/subscription/catalog", nil)
  rr := httptest.NewRecorder()
  handler.HandleGetSubscriptionCatalog(rr, req)

  if rr.Code != http.StatusOK {
    t.Fatalf("expected 200, got %d", rr.Code)
  }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/http -run TestHandleGetSubscriptionCatalogUsesCatalogService -v`
Expected: FAIL with undefined symbols.

**Step 3: Write minimal implementation**

```go
// config_handler.go
import "alex/internal/subscription"

type CatalogService interface {
  Catalog(ctx context.Context) subscription.Catalog
}

type ConfigHandler struct {
  ...
  catalogService CatalogService
}

func (h *ConfigHandler) HandleGetSubscriptionCatalog(w http.ResponseWriter, r *http.Request) {
  if h == nil {
    http.NotFound(w, r)
    return
  }
  svc := h.catalogService
  if svc == nil {
    svc = subscription.NewCatalogService(runtimeconfig.LoadCLICredentials, httpclient.New(20*time.Second, logging.NewComponentLogger("SubscriptionCatalog")), 15*time.Second)
  }
  writeJSON(w, http.StatusOK, svc.Catalog(r.Context()))
}

func (h *ConfigHandler) HandleGetRuntimeModels(w http.ResponseWriter, r *http.Request) {
  // call HandleGetSubscriptionCatalog for backward compatibility
}
```

```go
// router.go
mux.Handle("/api/internal/subscription/catalog", routeHandler("/api/internal/subscription/catalog", wrap(http.HandlerFunc(configHandler.HandleGetSubscriptionCatalog))))
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/http -run TestHandleGetSubscriptionCatalogUsesCatalogService -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/server/http/config_handler.go internal/server/http/router.go internal/server/http/runtime_models_test.go
git commit -m "feat: expose subscription catalog endpoint"
```

---

### Task 7: Frontend selection storage + UI + request injection

**Files:**
- Create: `web/lib/llmSelection.ts`
- Modify: `web/lib/types.ts`
- Modify: `web/lib/api.ts`
- Modify: `web/hooks/useTaskExecution.ts`
- Modify: `web/hooks/__tests__/useTaskExecution.test.tsx`
- Modify: `web/components/agent/LLMIndicator.tsx`
- Modify: `web/components/agent/__tests__/LLMIndicator.test.tsx`
- Modify: `web/lib/__tests__/runtime-models.test.ts`

**Step 1: Write the failing tests**

```ts
// web/hooks/__tests__/useTaskExecution.test.tsx
it('includes llm_selection from localStorage', async () => {
  localStorage.setItem('alex-llm-selection', JSON.stringify({
    mode: 'cli',
    provider: 'codex',
    model: 'gpt-5.2-codex',
    source: 'codex_cli',
  }));
  vi.mocked(apiClient.createTask).mockResolvedValue(mockResponse);

  renderHook(() => useTaskExecution(), { wrapper: createWrapper() });
  await act(async () => {
    await result.current.mutateAsync({ task: 'hello' });
  });

  expect(apiClient.createTask).toHaveBeenCalledWith(expect.objectContaining({
    llm_selection: {
      mode: 'cli',
      provider: 'codex',
      model: 'gpt-5.2-codex',
      source: 'codex_cli',
    },
  }));
});
```

```ts
// web/components/agent/__tests__/LLMIndicator.test.tsx
vi.mock('@/lib/api', () => ({
  getRuntimeConfigSnapshot: vi.fn().mockResolvedValue({
    effective: { llm_provider: 'codex', llm_model: 'model-a' },
    overrides: {},
    sources: { api_key: 'codex_cli', llm_model: 'file' },
  }),
  getSubscriptionCatalog: vi.fn().mockResolvedValue({
    providers: [{ provider: 'codex', source: 'codex_cli', models: ['model-a'] }],
  }),
}));

it('stores selection locally when a model is chosen', async () => {
  render(<LLMIndicator />);
  const trigger = await screen.findByRole('button', { name: /llm/i });
  await userEvent.click(trigger);
  await userEvent.click(await screen.findByRole('menuitem', { name: /model-a/i }));
  const stored = JSON.parse(localStorage.getItem('alex-llm-selection') || '{}');
  expect(stored.model).toBe('model-a');
});
```

**Step 2: Run tests to verify they fail**

Run: `npm test -- --runTestsByPath web/hooks/__tests__/useTaskExecution.test.tsx web/components/agent/__tests__/LLMIndicator.test.tsx`
Expected: FAIL with missing selection logic.

**Step 3: Write minimal implementation**

```ts
// web/lib/llmSelection.ts
export type LLMSelectionMode = 'yaml' | 'cli';
export type LLMSelection = {
  mode: LLMSelectionMode;
  provider: string;
  model: string;
  source: string;
};

const STORAGE_KEY = 'alex-llm-selection';

export function loadLLMSelection(): LLMSelection | null {
  if (typeof window === 'undefined') return null;
  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as LLMSelection;
  } catch {
    return null;
  }
}

export function saveLLMSelection(selection: LLMSelection) {
  if (typeof window === 'undefined') return;
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(selection));
}

export function clearLLMSelection() {
  if (typeof window === 'undefined') return;
  window.localStorage.removeItem(STORAGE_KEY);
}
```

```ts
// web/lib/types.ts
export interface LLMSelection {
  mode: 'yaml' | 'cli';
  provider: string;
  model: string;
  source: string;
}

export interface CreateTaskRequest {
  task: string;
  session_id?: string;
  parent_task_id?: string;
  attachments?: AttachmentUpload[];
  llm_selection?: LLMSelection;
}
```

```ts
// web/lib/api.ts
export async function getSubscriptionCatalog(): Promise<RuntimeModelCatalog> {
  return fetchAPI<RuntimeModelCatalog>('/api/internal/subscription/catalog');
}
```

```tsx
// web/hooks/useTaskExecution.ts
import { loadLLMSelection } from '@/lib/llmSelection';

mutationFn: async (request) => {
  const selection = request.llm_selection ?? loadLLMSelection();
  const response = await apiClient.createTask({
    ...request,
    ...(selection ? { llm_selection: selection } : {}),
  });
  return response;
}
```

```tsx
// web/components/agent/LLMIndicator.tsx
import { getSubscriptionCatalog } from '@/lib/api';
import { clearLLMSelection, loadLLMSelection, saveLLMSelection } from '@/lib/llmSelection';

// load selection on mount
const [selection, setSelection] = useState<LLMSelection | null>(null);
useEffect(() => {
  setSelection(loadLLMSelection());
}, []);

const handleSelectYaml = () => {
  clearLLMSelection();
  setSelection(null);
};

const handleSelectModel = (providerEntry, modelId) => {
  const next = { mode: 'cli', provider: providerEntry.provider, model: modelId, source: providerEntry.source };
  saveLLMSelection(next);
  setSelection(next);
};

// use selection to compute displayProvider/model + source
```

```tsx
// LLMIndicator button class
className="fixed bottom-4 left-4 z-40 flex items-center gap-2 rounded-full border border-border/80 bg-background px-3 py-2 text-xs text-muted-foreground shadow-md transition hover:text-foreground"
```

**Step 4: Run tests to verify they pass**

Run: `npm test -- --runTestsByPath web/hooks/__tests__/useTaskExecution.test.tsx web/components/agent/__tests__/LLMIndicator.test.tsx`
Expected: PASS

**Step 5: Commit**

```bash
git add web/lib/llmSelection.ts web/lib/types.ts web/lib/api.ts web/hooks/useTaskExecution.ts web/hooks/__tests__/useTaskExecution.test.tsx web/components/agent/LLMIndicator.tsx web/components/agent/__tests__/LLMIndicator.test.tsx web/lib/__tests__/runtime-models.test.ts
git commit -m "feat(web): add client-scoped llm selection"
```

---

## Final Validation
- Run: `go test ./...`
- Run: `npm run lint` (from `web`)
- Run: `npm test` (from `web`)

## Notes
- Keep managed overrides YAML untouched.
- Ensure localStorage stores only non-secret selection data.
