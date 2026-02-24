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

func (s *configOverrideStore) PopPending() *agent.ConfigOverride {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == nil {
		return nil
	}
	p := s.pending
	s.pending = nil
	return p
}

func (s *configOverrideStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending = nil
}

// mergeOverride overwrites non-nil fields in dst with deep-copied values from src.
func mergeOverride(dst *agent.ConfigOverride, src agent.ConfigOverride) {
	if src.Provider != nil {
		v := *src.Provider
		dst.Provider = &v
	}
	if src.Model != nil {
		v := *src.Model
		dst.Model = &v
	}
	if src.Temperature != nil {
		v := *src.Temperature
		dst.Temperature = &v
	}
	if src.TopP != nil {
		v := *src.TopP
		dst.TopP = &v
	}
	if src.MaxTokens != nil {
		v := *src.MaxTokens
		dst.MaxTokens = &v
	}
	if src.MaxIterations != nil {
		v := *src.MaxIterations
		dst.MaxIterations = &v
	}
	if src.StopSequences != nil {
		dst.StopSequences = append([]string(nil), src.StopSequences...)
	}
}

// maxTokensUpperBound is the upper limit for max_tokens validation.
// Increase when models support larger output windows.
const maxTokensUpperBound = 128000

// validateConfigOverride checks that every non-nil field falls within
// acceptable bounds.
func validateConfigOverride(o agent.ConfigOverride) error {
	// Provider and model must be specified together.
	if (o.Provider != nil) != (o.Model != nil) {
		return fmt.Errorf("provider and model must be specified together")
	}
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
		if *o.MaxTokens < 1 || *o.MaxTokens > maxTokensUpperBound {
			return fmt.Errorf("max_tokens must be in [1, %d], got %d", maxTokensUpperBound, *o.MaxTokens)
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
//
// SAFETY: This mutates engine fields directly. This is safe because the
// coordinator creates a fresh ReactEngine per ExecuteTask call (it is never
// shared across concurrent tasks). If engine reuse is ever introduced, these
// fields must be moved to reactRuntime-local copies.
func (r *reactRuntime) applyPendingConfigOverrides() {
	if r.configOverrides == nil {
		return
	}
	pending := r.configOverrides.PopPending()
	if pending == nil {
		return
	}

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
