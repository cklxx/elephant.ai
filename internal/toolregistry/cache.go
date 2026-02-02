package toolregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	ports "alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"

	lru "github.com/hashicorp/golang-lru/v2"
)

const (
	defaultCacheMaxSize = 256
	defaultCacheTTL     = 5 * time.Minute
)

// CacheConfig configures the tool result cache behaviour.
type CacheConfig struct {
	// MaxSize is the maximum number of entries in the LRU cache.
	MaxSize int
	// TTL is how long a cached result remains valid.
	TTL time.Duration
	// ExcludeTools lists tool names that should never be cached.
	ExcludeTools []string
}

// DefaultCacheConfig returns sensible defaults for tool result caching.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		MaxSize: defaultCacheMaxSize,
		TTL:     defaultCacheTTL,
		ExcludeTools: []string{
			"shell_exec",
			"file_write",
			"file_edit",
			"write_file",
			"replace_in_file",
			"bash",
			"code_execute",
			"execute_code",
			"memory_write",
			"todo_update",
			"lark_send_message",
			"lark_calendar_create",
			"lark_calendar_update",
			"lark_calendar_delete",
			"lark_task_manage",
			"okr_write",
			"write_attachment",
		},
	}
}

// cacheEntry holds a cached tool result along with the timestamp it was stored.
type cacheEntry struct {
	content   string
	metadata  map[string]any
	storedAt  time.Time
}

// cacheExecutor is a ToolExecutor wrapper that caches tool results keyed by
// (toolName + normalised arguments). It sits in the executor wrapper chain
// like every other decorator (retry, approval, id-aware, SLA).
type cacheExecutor struct {
	delegate     tools.ToolExecutor
	cache        *lru.Cache[string, cacheEntry]
	ttl          time.Duration
	excludeTools map[string]bool
}

// NewCacheExecutor wraps delegate with an LRU result cache.
// If config values are zero they fall back to DefaultCacheConfig defaults.
func NewCacheExecutor(delegate tools.ToolExecutor, config CacheConfig) tools.ToolExecutor {
	if delegate == nil {
		return nil
	}
	if config.MaxSize <= 0 {
		config.MaxSize = defaultCacheMaxSize
	}
	if config.TTL <= 0 {
		config.TTL = defaultCacheTTL
	}
	cache, err := lru.New[string, cacheEntry](config.MaxSize)
	if err != nil {
		// lru.New only errors on non-positive size which we guard above.
		return delegate
	}
	exclude := make(map[string]bool, len(config.ExcludeTools))
	for _, name := range config.ExcludeTools {
		exclude[strings.TrimSpace(name)] = true
	}
	return &cacheExecutor{
		delegate:     delegate,
		cache:        cache,
		ttl:          config.TTL,
		excludeTools: exclude,
	}
}

func (c *cacheExecutor) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if c.shouldSkip(call) {
		return c.delegate.Execute(ctx, call)
	}

	key := c.cacheKey(call)

	if entry, ok := c.cache.Get(key); ok {
		if time.Since(entry.storedAt) < c.ttl {
			// Cache hit — return a shallow copy with the current call's ID.
			return &ports.ToolResult{
				CallID:   call.ID,
				Content:  entry.content,
				Metadata: cloneMetadata(entry.metadata),
			}, nil
		}
		// Expired — evict so the LRU bookkeeping stays clean.
		c.cache.Remove(key)
	}

	result, err := c.delegate.Execute(ctx, call)
	if err != nil {
		return result, err
	}
	// Do not cache error results.
	if result != nil && result.Error != nil {
		return result, nil
	}
	if result != nil {
		c.cache.Add(key, cacheEntry{
			content:  result.Content,
			metadata: cloneMetadata(result.Metadata),
			storedAt: time.Now(),
		})
	}
	return result, nil
}

func (c *cacheExecutor) Definition() ports.ToolDefinition {
	return c.delegate.Definition()
}

func (c *cacheExecutor) Metadata() ports.ToolMetadata {
	return c.delegate.Metadata()
}

// shouldSkip returns true when caching must be bypassed for this call.
func (c *cacheExecutor) shouldSkip(call ports.ToolCall) bool {
	name := strings.TrimSpace(call.Name)
	if name == "" {
		name = strings.TrimSpace(c.delegate.Metadata().Name)
	}
	if c.excludeTools[name] {
		return true
	}
	if c.delegate.Metadata().Dangerous {
		return true
	}
	return false
}

// cacheKey produces a deterministic string key from tool name + arguments.
func (c *cacheExecutor) cacheKey(call ports.ToolCall) string {
	name := strings.TrimSpace(call.Name)
	if name == "" {
		name = strings.TrimSpace(c.delegate.Metadata().Name)
	}
	return fmt.Sprintf("%s:%s", name, normalizeArgs(call.Arguments))
}

// normalizeArgs serialises a map[string]any into a deterministic JSON string
// by sorting keys at every level.
func normalizeArgs(args map[string]any) string {
	if len(args) == 0 {
		return "{}"
	}
	data, err := json.Marshal(sortedMap(args))
	if err != nil {
		return "{}"
	}
	return string(data)
}

// sortedMap returns a representation of m that json.Marshal will serialise
// with keys in sorted order (Go maps iterate in random order, but
// json.Marshal sorts keys by default since Go 1.12, so we only need to
// handle nested maps by converting them to the same concrete type).
func sortedMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m[k]
		if nested, ok := v.(map[string]any); ok {
			v = sortedMap(nested)
		}
		out[k] = v
	}
	return out
}

// cloneMetadata performs a shallow copy of metadata so cached entries do not
// alias caller maps.
func cloneMetadata(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	cp := make(map[string]any, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

var _ tools.ToolExecutor = (*cacheExecutor)(nil)
