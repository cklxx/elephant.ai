package react

import (
	"fmt"
	"sync"

	agent "alex/internal/domain/agent/ports/agent"
)

// configOverrideStore is the concrete, mutex-protected implementation of
// agent.ConfigOverrideStore used inside reactRuntime.
type configOverrideStore struct {
	mu      sync.Mutex
	pending *agent.ConfigOverride
}

var _ agent.ConfigOverrideStore = (*configOverrideStore)(nil)

func newConfigOverrideStore() *configOverrideStore {
	return &configOverrideStore{}
}

func (s *configOverrideStore) Stage(override agent.ConfigOverride) error {
	if err := validateConfigOverride(override); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pending == nil {
		s.pending = &agent.ConfigOverride{}
	}
	mergeOverride(s.pending, override)
	return nil
}

func (s *configOverrideStore) Pending() *agent.ConfigOverride {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == nil {
		return nil
	}
	return cloneOverride(s.pending)
}

// cloneOverride performs a deep copy of a ConfigOverride so callers cannot
// mutate the store's internal state through returned pointers.
func cloneOverride(src *agent.ConfigOverride) *agent.ConfigOverride {
	cp := &agent.ConfigOverride{}
	if src.Provider != nil {
		v := *src.Provider
		cp.Provider = &v
	}
	if src.Model != nil {
		v := *src.Model
		cp.Model = &v
	}
	if src.Temperature != nil {
		v := *src.Temperature
		cp.Temperature = &v
	}
	if src.TopP != nil {
		v := *src.TopP
		cp.TopP = &v
	}
	if src.MaxTokens != nil {
		v := *src.MaxTokens
		cp.MaxTokens = &v
	}
	if src.MaxIterations != nil {
		v := *src.MaxIterations
		cp.MaxIterations = &v
	}
	if src.StopSequences != nil {
		cp.StopSequences = append([]string(nil), src.StopSequences...)
	}
	return cp
}

func (s *configOverrideStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending = nil
}

// mergeOverride overwrites non-nil fields in dst with values from src.
func mergeOverride(dst *agent.ConfigOverride, src agent.ConfigOverride) {
	if src.Provider != nil {
		dst.Provider = src.Provider
	}
	if src.Model != nil {
		dst.Model = src.Model
	}
	if src.Temperature != nil {
		dst.Temperature = src.Temperature
	}
	if src.TopP != nil {
		dst.TopP = src.TopP
	}
	if src.MaxTokens != nil {
		dst.MaxTokens = src.MaxTokens
	}
	if src.MaxIterations != nil {
		dst.MaxIterations = src.MaxIterations
	}
	if src.StopSequences != nil {
		dst.StopSequences = append([]string(nil), src.StopSequences...)
	}
}

// validateConfigOverride checks that every non-nil field falls within
// acceptable bounds.
func validateConfigOverride(o agent.ConfigOverride) error {
	if o.Temperature != nil {
		if *o.Temperature < 0 || *o.Temperature > 2 {
			return fmt.Errorf("temperature must be in [0, 2], got %v", *o.Temperature)
		}
	}
	if o.TopP != nil {
		if *o.TopP < 0 || *o.TopP > 1 {
			return fmt.Errorf("top_p must be in [0, 1], got %v", *o.TopP)
		}
	}
	if o.MaxTokens != nil {
		if *o.MaxTokens < 1 || *o.MaxTokens > 128000 {
			return fmt.Errorf("max_tokens must be in [1, 128000], got %d", *o.MaxTokens)
		}
	}
	if o.MaxIterations != nil {
		if *o.MaxIterations < 1 || *o.MaxIterations > 200 {
			return fmt.Errorf("max_iterations must be in [1, 200], got %d", *o.MaxIterations)
		}
	}
	return nil
}

// applyPendingConfigOverrides reads and clears the pending override, then
// mutates the engine's completion config and maxIterations. If provider/model
// changed, it calls the llmRebuilder to swap the LLM client.
func (r *reactRuntime) applyPendingConfigOverrides() {
	if r.configOverrides == nil {
		return
	}
	pending := r.configOverrides.Pending()
	if pending == nil {
		return
	}
	r.configOverrides.Clear()

	eng := r.engine

	if pending.Temperature != nil {
		eng.completion.temperature = *pending.Temperature
	}
	if pending.TopP != nil {
		eng.completion.topP = *pending.TopP
	}
	if pending.MaxTokens != nil {
		eng.completion.maxTokens = *pending.MaxTokens
	}
	if pending.StopSequences != nil {
		eng.completion.stopSequences = append([]string(nil), pending.StopSequences...)
	}
	if pending.MaxIterations != nil {
		eng.maxIterations = *pending.MaxIterations
	}

	// Rebuild LLM client if provider or model changed.
	if pending.Provider != nil && pending.Model != nil && eng.llmRebuilder != nil {
		newClient, err := eng.llmRebuilder(*pending.Provider, *pending.Model)
		if err != nil {
			eng.logger.Warn("Failed to rebuild LLM client for %s/%s: %v", *pending.Provider, *pending.Model, err)
			return
		}
		r.services.LLM = newClient
		eng.logger.Info("LLM client switched to %s/%s via update_config", *pending.Provider, *pending.Model)
	}
}
