# OpenAI Responses Input Format Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Align openai-responses/codex request payloads with the OpenAI Responses API input item schema, including tool calls/results, roles, and tool definitions, and document rule sources in code.

**Architecture:** Build a dedicated Responses input converter that emits itemized inputs (`role` messages plus `function_call` / `function_call_output` items) and reuse it for both codex and non-codex endpoints. System/developer messages become input items, user messages become `input_text`/`input_image` parts, assistant tool calls become `function_call`, and tool results become `function_call_output`. Update tool serialization to the flat Responses tool schema.

**Tech Stack:** Go (`internal/llm`), Go tests with in-process HTTP servers.

### Task 1: Add failing test for tool call + tool result conversion

**Files:**
- Modify: `internal/llm/openai_responses_client_test.go`

**Step 1: Write the failing test**

```go
func TestOpenAIResponsesClientConvertsToolMessagesToFunctionCallOutput(t *testing.T) {
	t.Parallel()

	var input []any

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		raw, ok := payload["input"].([]any)
		if !ok {
			t.Fatalf("expected input list, got %#v", payload["input"])
		}
		input = raw

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("expected http.Flusher")
		}
		events := []string{
			`{"type":"response.output_text.delta","item_id":"item-1","delta":"ok"}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
			`[DONE]`,
		}
		for _, evt := range events {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", evt); err != nil {
				t.Fatalf("write event: %v", err)
			}
			flusher.Flush()
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/backend-api/codex",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "user", Content: "hi"},
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []ports.ToolCall{
					{
						ID:   "call-1",
						Name: "plan",
						Arguments: map[string]any{
							"foo": "bar",
						},
					},
				},
			},
			{Role: "tool", Content: "ok", ToolCallID: "call-1"},
		},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	var sawFunctionCall bool
	var sawFunctionOutput bool
	var sawToolRole bool
	for _, item := range input {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if entry["role"] == "tool" {
			sawToolRole = true
		}
		if entry["type"] == "function_call" {
			sawFunctionCall = true
		}
		if entry["type"] == "function_call_output" {
			sawFunctionOutput = true
		}
	}

	if sawToolRole {
		t.Fatalf("expected tool role to be omitted for responses input")
	}
	if !sawFunctionCall {
		t.Fatalf("expected function_call input item")
	}
	if !sawFunctionOutput {
		t.Fatalf("expected function_call_output input item")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/llm -run TestOpenAIResponsesClientConvertsToolMessagesToFunctionCallOutput`

Expected: FAIL due to tool role and missing function_call/function_call_output items.

### Task 2: Add failing test for system message placement

**Files:**
- Modify: `internal/llm/openai_responses_client_test.go`

**Step 1: Write the failing test**

```go
func TestOpenAIResponsesClientKeepsSystemMessageInInput(t *testing.T) {
	t.Parallel()

	var input []any

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		raw, ok := payload["input"].([]any)
		if !ok {
			t.Fatalf("expected input list, got %#v", payload["input"])
		}
		input = raw

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":     "resp-1",
			"status": "completed",
			"output": []any{},
			"usage": map[string]any{
				"input_tokens":  1,
				"output_tokens": 1,
				"total_tokens":  2,
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: "system instructions"},
			{Role: "user", Content: "hi"},
		},
		MaxTokens: 64,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if len(input) != 2 {
		t.Fatalf("expected 2 input entries, got %d", len(input))
	}
	first, ok := input[0].(map[string]any)
	if !ok {
		t.Fatalf("expected input entry map, got %#v", input[0])
	}
	if first["role"] != "system" {
		t.Fatalf("expected system role, got %#v", first["role"])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/llm -run TestOpenAIResponsesClientKeepsSystemMessageInInput`

Expected: FAIL because current code moves system messages to `instructions`.

### Task 3: Implement Responses input conversion (minimal)

**Files:**
- Modify: `internal/llm/openai_responses_client.go`

**Step 1: Write minimal implementation**

```go
// Responses input item shapes follow OpenAI Responses API.
// Source: opencode dev branch
// - packages/opencode/src/provider/sdk/openai-compatible/src/responses/openai-responses-api-types.ts
// - packages/opencode/src/provider/sdk/openai-compatible/src/responses/convert-to-openai-responses-input.ts
func (c *openAIResponsesClient) buildResponsesInput(msgs []ports.Message) []map[string]any {
	items := make([]map[string]any, 0, len(msgs))
	for _, msg := range msgs {
		if msg.Source == ports.MessageSourceDebug || msg.Source == ports.MessageSourceEvaluation {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		switch role {
		case "system", "developer":
			if strings.TrimSpace(msg.Content) == "" {
				continue
			}
			items = append(items, map[string]any{
				"role":    role,
				"content": msg.Content,
			})
		case "user":
			parts := buildResponsesUserContent(msg, shouldEmbedAttachmentsInContent(msg))
			if len(parts) == 0 {
				continue
			}
			items = append(items, map[string]any{
				"role":    "user",
				"content": parts,
			})
		case "assistant":
			if strings.TrimSpace(msg.Content) != "" {
				items = append(items, map[string]any{
					"role": "assistant",
					"content": []map[string]any{
						{"type": "output_text", "text": msg.Content},
					},
				})
			}
			for _, call := range msg.ToolCalls {
				if !isValidToolName(call.Name) {
					continue
				}
				args := "{}"
				if len(call.Arguments) > 0 {
					if data, err := json.Marshal(call.Arguments); err == nil {
						args = string(data)
					}
				}
				if strings.TrimSpace(call.ID) == "" {
					continue
				}
				items = append(items, map[string]any{
					"type":      "function_call",
					"call_id":   call.ID,
					"name":      call.Name,
					"arguments": args,
				})
			}
		case "tool":
			if strings.TrimSpace(msg.ToolCallID) == "" {
				continue
			}
			items = append(items, map[string]any{
				"type":    "function_call_output",
				"call_id": msg.ToolCallID,
				"output":  msg.Content,
			})
		}
	}
	return items
}
```

**Step 2: Update request building to use `buildResponsesInput` and remove `instructions`**

```go
input := c.buildResponsesInput(req.Messages)
payload := map[string]any{
  "model":  c.model,
  "input":  input,
  "stream": true, // or false
  "store":  false,
}
```

**Step 3: Update user content builder to emit Responses parts**

```go
func buildResponsesUserContent(msg ports.Message, embedAttachments bool) []map[string]any {
  // returns []{"type":"input_text","text":...} and []{"type":"input_image","image_url":...}
}
```

**Step 4: Run tests**

Run: `go test ./internal/llm -run TestOpenAIResponsesClientConvertsToolMessagesToFunctionCallOutput`
Expected: PASS

Run: `go test ./internal/llm -run TestOpenAIResponsesClientKeepsSystemMessageInInput`
Expected: PASS

### Task 4: Align tool serialization with Responses schema

**Files:**
- Modify: `internal/llm/openai_responses_client.go`
- Modify: `internal/llm/openai_responses_client_test.go`

**Step 1: Update tool serialization to flat Responses tools**

```go
if len(req.Tools) > 0 {
  payload["tools"] = convertCodexTools(req.Tools)
  payload["tool_choice"] = "auto"
}
```

**Step 2: Add test for non-codex tool shape**

```go
func TestOpenAIResponsesClientUsesFlatToolsForResponses(t *testing.T) {
  // assert tools[0].name exists at top-level and no "function" wrapper
}
```

**Step 3: Run test to verify pass**

Run: `go test ./internal/llm -run TestOpenAIResponsesClientUsesFlatToolsForResponses`

### Task 5: Full verification

**Step 1: Run full Go tests**

Run: `go test ./...`

Expected: PASS

**Step 2: Run web lint + tests**

Run: `npm run lint` in `web/`

Run: `npm test` in `web/`

Expected: PASS (baseline-browser-mapping warnings are acceptable)

### Task 6: Commit

```bash
git add internal/llm/openai_responses_client.go internal/llm/openai_responses_client_test.go
git commit -m "fix: align responses input with OpenAI spec"
```
