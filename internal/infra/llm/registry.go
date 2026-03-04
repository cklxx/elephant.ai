package llm

import (
	"sort"
	"strings"
	"sync"

	portsllm "alex/internal/domain/agent/ports/llm"
)

// ProviderDescriptor describes a registered LLM provider.
type ProviderDescriptor struct {
	Name          string // canonical name: "openai", "anthropic", "kimi", etc.
	Family        string // "openai-compat", "codex-compat", "anthropic", "llamacpp", "mock"
	ClientFactory func(model string, cfg Config) (portsllm.LLMClient, error)
	ConfigMutator func(cfg *Config) // optional: mutate config before client creation
}

// Registry holds registered provider descriptors and alias mappings.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]*ProviderDescriptor
	aliases   map[string]string // alias → canonical name
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]*ProviderDescriptor),
		aliases:   make(map[string]string),
	}
}

// Register adds a provider descriptor to the registry.
func (r *Registry) Register(desc *ProviderDescriptor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[strings.TrimSpace(strings.ToLower(desc.Name))] = desc
}

// RegisterAlias maps an alternative name to a canonical provider name.
func (r *Registry) RegisterAlias(alias, canonical string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.aliases[strings.TrimSpace(strings.ToLower(alias))] = strings.TrimSpace(strings.ToLower(canonical))
}

// Get returns the provider descriptor for the given name or alias.
// The lookup is case-insensitive.
func (r *Registry) Get(name string) (*ProviderDescriptor, bool) {
	key := strings.TrimSpace(strings.ToLower(name))
	r.mu.RLock()
	defer r.mu.RUnlock()

	if desc, ok := r.providers[key]; ok {
		return desc, true
	}
	if canonical, ok := r.aliases[key]; ok {
		if desc, ok := r.providers[canonical]; ok {
			return desc, true
		}
	}
	return nil, false
}

// List returns all registered provider descriptors sorted by name.
func (r *Registry) List() []*ProviderDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*ProviderDescriptor, 0, len(r.providers))
	for _, desc := range r.providers {
		out = append(out, desc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// NewDefaultRegistry creates a registry pre-populated with all built-in providers.
func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	registerBuiltinProviders(r)
	return r
}

func registerBuiltinProviders(r *Registry) {
	// OpenAI-compatible family
	openAIFactory := func(model string, cfg Config) (portsllm.LLMClient, error) {
		return NewOpenAIClient(model, cfg)
	}
	for _, name := range []string{"openai", "openrouter", "deepseek", "kimi", "glm", "minimax"} {
		r.Register(&ProviderDescriptor{
			Name:          name,
			Family:        "openai-compat",
			ClientFactory: openAIFactory,
		})
	}

	// Codex/Responses family
	responsesFactory := func(model string, cfg Config) (portsllm.LLMClient, error) {
		return NewOpenAIResponsesClient(model, cfg)
	}
	for _, name := range []string{"openai-responses", "codex"} {
		r.Register(&ProviderDescriptor{
			Name:          name,
			Family:        "codex-compat",
			ClientFactory: responsesFactory,
		})
	}

	// Anthropic
	r.Register(&ProviderDescriptor{
		Name:   "anthropic",
		Family: "anthropic",
		ClientFactory: func(model string, cfg Config) (portsllm.LLMClient, error) {
			return NewAnthropicClient(model, cfg)
		},
	})

	// LlamaCpp
	r.Register(&ProviderDescriptor{
		Name:   "llama.cpp",
		Family: "llamacpp",
		ClientFactory: func(model string, cfg Config) (portsllm.LLMClient, error) {
			return NewLlamaCppClient(model, cfg)
		},
	})

	// Mock
	r.Register(&ProviderDescriptor{
		Name:   "mock",
		Family: "mock",
		ClientFactory: func(_ string, _ Config) (portsllm.LLMClient, error) {
			return NewMockClient(), nil
		},
	})

	// Aliases
	r.RegisterAlias("responses", "openai-responses")
	r.RegisterAlias("claude", "anthropic")
	r.RegisterAlias("llama-cpp", "llama.cpp")
	r.RegisterAlias("llamacpp", "llama.cpp")
}
