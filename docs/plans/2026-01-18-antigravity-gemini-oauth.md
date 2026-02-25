# Antigravity Gemini OAuth Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Support Antigravity subscription model listing and real LLM calls using `~/.gemini/oauth_creds.json` credentials.

**Architecture:** Load Gemini OAuth access tokens as Antigravity CLI credentials, fetch models via `v1internal:fetchAvailableModels`, and add a native Antigravity LLM client that converts OpenAI-style messages/tools into Gemini request payloads. Keep changes isolated to config loading, subscription catalog, and a new LLM client.

**Tech Stack:** Go, internal config loader, HTTP client (`internal/httpclient`), Go testing (`go test`).

### Task 1: Load Gemini OAuth creds as Antigravity CLI credentials

**Files:**
- Modify: `internal/config/cli_auth.go`
- Modify: `internal/config/cli_auth_test.go`

**Step 1: Write the failing test**

```go
func TestLoadCLICredentialsReadsGeminiOAuthForAntigravity(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	geminiDir := filepath.Join(tmp, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	oauth := `{"access_token":"ag-access","refresh_token":"ag-refresh","expiry_date":123}`
	if err := os.WriteFile(filepath.Join(geminiDir, "oauth_creds.json"), []byte(oauth), 0o600); err != nil {
		t.Fatalf("write oauth: %v", err)
	}

	creds := LoadCLICredentials(
		WithHomeDir(func() (string, error) { return tmp, nil }),
		WithEnv(func(string) (string, bool) { return "", false }),
	)

	if creds.Antigravity.APIKey != "ag-access" {
		t.Fatalf("expected antigravity api key, got %q", creds.Antigravity.APIKey)
	}
	if creds.Antigravity.BaseURL != "https://cloudcode-pa.googleapis.com" {
		t.Fatalf("expected antigravity base url, got %q", creds.Antigravity.BaseURL)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run TestLoadCLICredentialsReadsGeminiOAuthForAntigravity -v`
Expected: FAIL (antigravity API key empty).

**Step 3: Write minimal implementation**

- Add a Gemini OAuth creds struct.
- Read `~/.gemini/oauth_creds.json` in `loadAntigravityCLIAuth`.
- Use `access_token` as `APIKey`, default `BaseURL` to `https://cloudcode-pa.googleapis.com`.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config -run TestLoadCLICredentialsReadsGeminiOAuthForAntigravity -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/cli_auth.go internal/config/cli_auth_test.go
git commit -m "feat: load gemini oauth creds for antigravity"
```

### Task 2: Use Antigravity model listing endpoint

**Files:**
- Modify: `internal/subscription/catalog.go`
- Modify: `internal/subscription/catalog_test.go`

**Step 1: Write the failing test**

```go
func TestFetchProviderModelsUsesAntigravityEndpoint(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":{"gemini-3-pro-high":{},"nanobanana-pro":{}}}`))
	}))
	defer srv.Close()

	models, err := fetchProviderModels(context.Background(), srv.Client(), fetchTarget{
		provider: "antigravity",
		baseURL:  srv.URL,
		apiKey:   "tok-abc",
	})
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %q", gotMethod)
	}
	if gotPath != "/v1internal:fetchAvailableModels" {
		t.Fatalf("expected antigravity endpoint, got %q", gotPath)
	}
	if gotAuth != "Bearer tok-abc" {
		t.Fatalf("expected bearer auth, got %q", gotAuth)
	}
	if len(models) != 2 {
		t.Fatalf("unexpected models: %#v", models)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/subscription -run TestFetchProviderModelsUsesAntigravityEndpoint -v`
Expected: FAIL (GET /models).

**Step 3: Write minimal implementation**

- Branch `fetchProviderModels` to call `POST {base_url}/v1internal:fetchAvailableModels` when provider is `antigravity`.
- Parse response `models` object keys into model list.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/subscription -run TestFetchProviderModelsUsesAntigravityEndpoint -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/subscription/catalog.go internal/subscription/catalog_test.go
git commit -m "feat: list antigravity models via fetchAvailableModels"
```

### Task 3: Add Antigravity LLM client (Gemini payload)

**Files:**
- Create: `internal/llm/antigravity_client.go`
- Modify: `internal/llm/factory.go`
- Create: `internal/llm/antigravity_client_test.go`

**Step 1: Write the failing tests**

```go
func TestAntigravityClientBuildsGeminiPayload(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":2,"totalTokenCount":3}}`))
	}))
	defer srv.Close()

	client, err := NewAntigravityClient("gemini-3-pro-high", Config{APIKey: "tok", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("client error: %v", err)
	}

	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
		Tools: []ports.ToolDefinition{{
			Name: "tool",
			Description: "desc",
			Parameters: ports.ParameterSchema{Type: "object", Properties: map[string]ports.Property{}},
		}},
		Metadata: map[string]any{"request_id": "req-1"},
	})
	if err != nil {
		t.Fatalf("complete error: %v", err)
	}
	if gotPath != "/v1internal:generateContent" {
		t.Fatalf("expected generateContent path, got %q", gotPath)
	}
	if gotAuth != "Bearer tok" {
		t.Fatalf("expected bearer auth, got %q", gotAuth)
	}
	if resp.Content != "ok" {
		t.Fatalf("expected response content, got %q", resp.Content)
	}

	request, _ := gotBody["request"].(map[string]any)
	if request == nil {
		t.Fatalf("expected request payload")
	}
	if request["toolConfig"] == nil {
		t.Fatalf("expected toolConfig")
	}
	if request["tools"] == nil {
		t.Fatalf("expected tools")
	}
}

func TestAntigravityClientConvertsToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"functionCall":{"name":"tool","args":{"x":1}}}]},"finishReason":"STOP"}]}`))
	}))
	defer srv.Close()

	client, _ := NewAntigravityClient("gemini-3-pro-high", Config{APIKey: "tok", BaseURL: srv.URL})
	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("complete error: %v", err)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "tool" {
		t.Fatalf("unexpected tool calls: %#v", resp.ToolCalls)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/llm -run TestAntigravityClient -v`
Expected: FAIL (client not implemented).

**Step 3: Write minimal implementation**

- Build Antigravity payload:
  - POST `{base_url}/v1internal:generateContent`
  - JSON body: `project`, `requestId`, `request` (contents/systemInstruction/generationConfig/tools/toolConfig/safetySettings), `model`, `requestType=agent`.
- Convert messages to Gemini contents, tool calls to functionCall parts, tool results to functionResponse parts.
- Convert tools into `functionDeclarations` with `parametersJsonSchema`.
- Parse response `candidates` into `ports.CompletionResponse` with `ToolCalls` and token usage from `usageMetadata`.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/llm -run TestAntigravityClient -v`
Expected: PASS

**Step 5: Wire factory**

- Use `NewAntigravityClient` for provider `antigravity` in `internal/llm/factory.go`.

**Step 6: Commit**

```bash
git add internal/llm/antigravity_client.go internal/llm/antigravity_client_test.go internal/llm/factory.go
git commit -m "feat: add antigravity gemini client"
```

### Task 4: Full validation

**Step 1: Run Go tests**

Run: `go test ./...`
Expected: PASS

**Step 2: Run web lint and tests**

Run: `npm run lint` in `web/`
Expected: PASS (baseline-browser-mapping warning acceptable if already present)

Run: `npm test` in `web/`
Expected: PASS
