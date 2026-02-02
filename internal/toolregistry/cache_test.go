package toolregistry

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ports "alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
)

// ---------------------------------------------------------------------------
// stub helpers
// ---------------------------------------------------------------------------

// cacheStubTool is a configurable stub that counts invocations.
type cacheStubTool struct {
	name      string
	dangerous bool
	calls     atomic.Int64
	result    func(call ports.ToolCall) (*ports.ToolResult, error)
}

func newCacheStub(name string) *cacheStubTool {
	return &cacheStubTool{
		name: name,
		result: func(call ports.ToolCall) (*ports.ToolResult, error) {
			return &ports.ToolResult{CallID: call.ID, Content: "result"}, nil
		},
	}
}

func (s *cacheStubTool) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	s.calls.Add(1)
	return s.result(call)
}

func (s *cacheStubTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{Name: s.name}
}

func (s *cacheStubTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{Name: s.name, Dangerous: s.dangerous}
}

var _ tools.ToolExecutor = (*cacheStubTool)(nil)

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestCacheHitReturnsCachedResult(t *testing.T) {
	stub := newCacheStub("web_search")
	executor := NewCacheExecutor(stub, CacheConfig{MaxSize: 8, TTL: time.Minute})

	call := ports.ToolCall{ID: "c1", Name: "web_search", Arguments: map[string]any{"q": "hello"}}
	r1, err := executor.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if r1.Content != "result" {
		t.Fatalf("unexpected content: %s", r1.Content)
	}

	// Second call with different ID but same args should hit cache.
	call2 := ports.ToolCall{ID: "c2", Name: "web_search", Arguments: map[string]any{"q": "hello"}}
	r2, err := executor.Execute(context.Background(), call2)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if r2.Content != "result" {
		t.Fatalf("unexpected cached content: %s", r2.Content)
	}
	if stub.calls.Load() != 1 {
		t.Fatalf("expected delegate called once, got %d", stub.calls.Load())
	}
}

func TestCacheMissDelegatesToUnderlying(t *testing.T) {
	stub := newCacheStub("web_search")
	executor := NewCacheExecutor(stub, CacheConfig{MaxSize: 8, TTL: time.Minute})

	call := ports.ToolCall{ID: "c1", Name: "web_search", Arguments: map[string]any{"q": "hello"}}
	_, err := executor.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("call error: %v", err)
	}
	if stub.calls.Load() != 1 {
		t.Fatalf("expected delegate called once, got %d", stub.calls.Load())
	}
}

func TestCacheTTLExpiration(t *testing.T) {
	stub := newCacheStub("web_search")
	// Use a very short TTL so it expires quickly.
	executor := NewCacheExecutor(stub, CacheConfig{MaxSize: 8, TTL: 10 * time.Millisecond})

	call := ports.ToolCall{ID: "c1", Name: "web_search", Arguments: map[string]any{"q": "hello"}}
	_, err := executor.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}

	// Wait for TTL to expire.
	time.Sleep(20 * time.Millisecond)

	call2 := ports.ToolCall{ID: "c2", Name: "web_search", Arguments: map[string]any{"q": "hello"}}
	_, err = executor.Execute(context.Background(), call2)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if stub.calls.Load() != 2 {
		t.Fatalf("expected delegate called twice after TTL expiry, got %d", stub.calls.Load())
	}
}

func TestCacheExcludedToolsBypassCache(t *testing.T) {
	stub := newCacheStub("shell_exec")
	executor := NewCacheExecutor(stub, CacheConfig{
		MaxSize:      8,
		TTL:          time.Minute,
		ExcludeTools: []string{"shell_exec"},
	})

	call := ports.ToolCall{ID: "c1", Name: "shell_exec", Arguments: map[string]any{"cmd": "ls"}}
	_, _ = executor.Execute(context.Background(), call)

	call2 := ports.ToolCall{ID: "c2", Name: "shell_exec", Arguments: map[string]any{"cmd": "ls"}}
	_, _ = executor.Execute(context.Background(), call2)

	if stub.calls.Load() != 2 {
		t.Fatalf("excluded tool should bypass cache, expected 2 calls, got %d", stub.calls.Load())
	}
}

func TestCacheDangerousToolsBypassCache(t *testing.T) {
	stub := newCacheStub("risky_tool")
	stub.dangerous = true
	executor := NewCacheExecutor(stub, CacheConfig{MaxSize: 8, TTL: time.Minute})

	call := ports.ToolCall{ID: "c1", Name: "risky_tool", Arguments: map[string]any{"x": 1}}
	_, _ = executor.Execute(context.Background(), call)

	call2 := ports.ToolCall{ID: "c2", Name: "risky_tool", Arguments: map[string]any{"x": 1}}
	_, _ = executor.Execute(context.Background(), call2)

	if stub.calls.Load() != 2 {
		t.Fatalf("dangerous tool should bypass cache, expected 2 calls, got %d", stub.calls.Load())
	}
}

func TestCacheErrorResultsNotCached(t *testing.T) {
	stub := newCacheStub("flaky_tool")
	callCount := 0
	stub.result = func(call ports.ToolCall) (*ports.ToolResult, error) {
		callCount++
		if callCount == 1 {
			return &ports.ToolResult{CallID: call.ID, Error: errors.New("oops")}, nil
		}
		return &ports.ToolResult{CallID: call.ID, Content: "ok"}, nil
	}
	executor := NewCacheExecutor(stub, CacheConfig{MaxSize: 8, TTL: time.Minute})

	call := ports.ToolCall{ID: "c1", Name: "flaky_tool", Arguments: map[string]any{"x": 1}}
	r1, _ := executor.Execute(context.Background(), call)
	if r1.Error == nil {
		t.Fatal("expected error on first call")
	}

	// Second call same args — should NOT use cache since the first was an error.
	call2 := ports.ToolCall{ID: "c2", Name: "flaky_tool", Arguments: map[string]any{"x": 1}}
	r2, _ := executor.Execute(context.Background(), call2)
	if r2.Error != nil {
		t.Fatalf("expected success on second call, got error: %v", r2.Error)
	}
	if stub.calls.Load() != 2 {
		t.Fatalf("expected 2 delegate calls (error not cached), got %d", stub.calls.Load())
	}
}

func TestCacheNormalizedArgsConsistentKeys(t *testing.T) {
	// Two maps with same keys in different insertion order should produce
	// the same normalised string.
	a := map[string]any{"b": 2, "a": 1}
	b := map[string]any{"a": 1, "b": 2}
	na := normalizeArgs(a)
	nb := normalizeArgs(b)
	if na != nb {
		t.Fatalf("normalizeArgs mismatch:\n  a=%s\n  b=%s", na, nb)
	}
}

func TestCacheDifferentArgsProduceDifferentKeys(t *testing.T) {
	a := map[string]any{"q": "hello"}
	b := map[string]any{"q": "world"}
	if normalizeArgs(a) == normalizeArgs(b) {
		t.Fatal("different arguments should produce different keys")
	}
}

func TestCacheRespectsMaxSize(t *testing.T) {
	stub := newCacheStub("tool")
	executor := NewCacheExecutor(stub, CacheConfig{MaxSize: 2, TTL: time.Minute})

	// Fill cache with 3 distinct entries; the LRU should evict the oldest.
	for i := 0; i < 3; i++ {
		call := ports.ToolCall{
			ID:        fmt.Sprintf("c%d", i),
			Name:      "tool",
			Arguments: map[string]any{"i": i},
		}
		_, _ = executor.Execute(context.Background(), call)
	}
	if stub.calls.Load() != 3 {
		t.Fatalf("expected 3 delegate calls, got %d", stub.calls.Load())
	}

	// Access the first entry again — it should have been evicted, causing
	// a new delegate call.
	call0 := ports.ToolCall{ID: "c0-again", Name: "tool", Arguments: map[string]any{"i": 0}}
	_, _ = executor.Execute(context.Background(), call0)
	if stub.calls.Load() != 4 {
		t.Fatalf("expected 4 delegate calls after eviction, got %d", stub.calls.Load())
	}
}

func TestCacheCachedResultGetsNewCallID(t *testing.T) {
	stub := newCacheStub("tool")
	executor := NewCacheExecutor(stub, CacheConfig{MaxSize: 8, TTL: time.Minute})

	call1 := ports.ToolCall{ID: "original-id", Name: "tool", Arguments: map[string]any{"x": 1}}
	r1, _ := executor.Execute(context.Background(), call1)
	if r1.CallID != "original-id" {
		t.Fatalf("expected original-id, got %s", r1.CallID)
	}

	call2 := ports.ToolCall{ID: "new-id", Name: "tool", Arguments: map[string]any{"x": 1}}
	r2, _ := executor.Execute(context.Background(), call2)
	if r2.CallID != "new-id" {
		t.Fatalf("cached result should adopt new call ID, got %s", r2.CallID)
	}
}

func TestCacheConcurrentAccessSafety(t *testing.T) {
	stub := newCacheStub("tool")
	executor := NewCacheExecutor(stub, CacheConfig{MaxSize: 64, TTL: time.Minute})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			call := ports.ToolCall{
				ID:        fmt.Sprintf("c%d", idx),
				Name:      "tool",
				Arguments: map[string]any{"i": idx % 10},
			}
			_, err := executor.Execute(context.Background(), call)
			if err != nil {
				t.Errorf("concurrent call %d failed: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()
	// No panics or races means success. The delegate should have been called
	// at most once per distinct argument set (10 unique) but under concurrency
	// a few duplicates are acceptable — we just verify no crash.
}

func TestCacheNilDelegateReturnsNil(t *testing.T) {
	executor := NewCacheExecutor(nil, DefaultCacheConfig())
	if executor != nil {
		t.Fatal("expected nil executor for nil delegate")
	}
}

func TestCacheDefinitionAndMetadataPassthrough(t *testing.T) {
	stub := newCacheStub("my_tool")
	executor := NewCacheExecutor(stub, DefaultCacheConfig())

	if executor.Definition().Name != "my_tool" {
		t.Fatalf("expected definition name my_tool, got %s", executor.Definition().Name)
	}
	if executor.Metadata().Name != "my_tool" {
		t.Fatalf("expected metadata name my_tool, got %s", executor.Metadata().Name)
	}
}

func TestCacheErrorFromDelegateNotCached(t *testing.T) {
	stub := newCacheStub("err_tool")
	stub.result = func(call ports.ToolCall) (*ports.ToolResult, error) {
		return nil, errors.New("delegate error")
	}
	executor := NewCacheExecutor(stub, CacheConfig{MaxSize: 8, TTL: time.Minute})

	call := ports.ToolCall{ID: "c1", Name: "err_tool", Arguments: map[string]any{"x": 1}}
	_, err := executor.Execute(context.Background(), call)
	if err == nil {
		t.Fatal("expected error from delegate")
	}

	// Second call should also delegate (no caching of errors).
	call2 := ports.ToolCall{ID: "c2", Name: "err_tool", Arguments: map[string]any{"x": 1}}
	_, _ = executor.Execute(context.Background(), call2)
	if stub.calls.Load() != 2 {
		t.Fatalf("expected 2 delegate calls, got %d", stub.calls.Load())
	}
}

func TestCacheNestedArgsNormalization(t *testing.T) {
	a := map[string]any{"outer": map[string]any{"b": 2, "a": 1}}
	b := map[string]any{"outer": map[string]any{"a": 1, "b": 2}}
	if normalizeArgs(a) != normalizeArgs(b) {
		t.Fatal("nested maps with same content should produce identical normalized args")
	}
}

func TestCacheEmptyArgs(t *testing.T) {
	result := normalizeArgs(nil)
	if result != "{}" {
		t.Fatalf("expected {}, got %s", result)
	}
	result2 := normalizeArgs(map[string]any{})
	if result2 != "{}" {
		t.Fatalf("expected {}, got %s", result2)
	}
}
