package subscription

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	runtimeconfig "alex/internal/shared/config"
	jsonx "alex/internal/shared/json"
)

const (
	onboardingStateVersion  = 1
	onboardingStateFilename = "onboarding_state.json"
)

// OnboardingState stores first-run setup completion metadata.
type OnboardingState struct {
	CompletedAt           string `json:"completed_at,omitempty"`
	SelectedProvider      string `json:"selected_provider,omitempty"`
	SelectedModel         string `json:"selected_model,omitempty"`
	UsedSource            string `json:"used_source,omitempty"`
	AdvancedOverridesUsed bool   `json:"advanced_overrides_used,omitempty"`
}

func (s OnboardingState) isZero() bool {
	return strings.TrimSpace(s.CompletedAt) == "" &&
		strings.TrimSpace(s.SelectedProvider) == "" &&
		strings.TrimSpace(s.SelectedModel) == "" &&
		strings.TrimSpace(s.UsedSource) == "" &&
		!s.AdvancedOverridesUsed
}

type onboardingStateDoc struct {
	Version int             `json:"version"`
	State   OnboardingState `json:"state,omitempty"`
}

// ResolveOnboardingStatePath returns the file path used to persist onboarding state.
//
// Priority:
//  1. Explicit ALEX_ONBOARDING_STATE_PATH.
//  2. Sibling to the resolved config path (defaults to ~/.alex/onboarding_state.json).
func ResolveOnboardingStatePath(envLookup runtimeconfig.EnvLookup, homeDir func() (string, error)) string {
	if envLookup == nil {
		envLookup = runtimeconfig.DefaultEnvLookup
	}
	if value, ok := envLookup("ALEX_ONBOARDING_STATE_PATH"); ok {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	configPath, _ := runtimeconfig.ResolveConfigPath(envLookup, homeDir)
	return filepath.Join(filepath.Dir(configPath), onboardingStateFilename)
}

type OnboardingStateStore struct {
	path string
	mu   sync.Mutex
}

func NewOnboardingStateStore(path string) *OnboardingStateStore {
	return &OnboardingStateStore{path: strings.TrimSpace(path)}
}

func (s *OnboardingStateStore) Get(ctx context.Context) (OnboardingState, bool, error) {
	if s == nil {
		return OnboardingState{}, false, nil
	}
	if ctx != nil && ctx.Err() != nil {
		return OnboardingState{}, false, ctx.Err()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	doc, exists, err := s.loadDocLocked(ctx)
	if err != nil {
		return OnboardingState{}, false, err
	}
	if !exists || doc.State.isZero() {
		return OnboardingState{}, false, nil
	}
	return doc.State, true, nil
}

func (s *OnboardingStateStore) Set(ctx context.Context, state OnboardingState) error {
	if s == nil || s.path == "" {
		return fmt.Errorf("onboarding store not configured")
	}
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	normalizeOnboardingState(&state)
	if state.CompletedAt == "" && state.SelectedProvider != "" && state.SelectedModel != "" {
		state.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	doc, _, err := s.loadDocLocked(ctx)
	if err != nil {
		return err
	}
	doc.State = state
	return s.saveDocLocked(ctx, doc)
}

func (s *OnboardingStateStore) Clear(ctx context.Context) error {
	if s == nil || s.path == "" {
		return nil
	}
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove onboarding state: %w", err)
	}
	return nil
}

func normalizeOnboardingState(state *OnboardingState) {
	if state == nil {
		return
	}
	state.CompletedAt = strings.TrimSpace(state.CompletedAt)
	state.SelectedProvider = strings.ToLower(strings.TrimSpace(state.SelectedProvider))
	state.SelectedModel = strings.TrimSpace(state.SelectedModel)
	state.UsedSource = strings.TrimSpace(state.UsedSource)
}

func (s *OnboardingStateStore) loadDocLocked(ctx context.Context) (onboardingStateDoc, bool, error) {
	if s.path == "" {
		return onboardingStateDoc{}, false, fmt.Errorf("onboarding state path not configured")
	}
	if ctx != nil && ctx.Err() != nil {
		return onboardingStateDoc{}, false, ctx.Err()
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return onboardingStateDoc{Version: onboardingStateVersion}, false, nil
		}
		return onboardingStateDoc{}, false, fmt.Errorf("read onboarding state: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return onboardingStateDoc{Version: onboardingStateVersion}, true, nil
	}

	var doc onboardingStateDoc
	if err := decodeOnboardingStateDoc(data, &doc); err != nil {
		return onboardingStateDoc{}, false, fmt.Errorf("parse onboarding state: %w", err)
	}
	if doc.Version == 0 {
		doc.Version = onboardingStateVersion
	}
	if doc.Version != onboardingStateVersion {
		return onboardingStateDoc{}, false, fmt.Errorf("unsupported onboarding state version %d", doc.Version)
	}
	normalizeOnboardingState(&doc.State)
	return doc, true, nil
}

// decodeOnboardingStateDoc parses only the first JSON document.
// This keeps onboarding reads resilient when historical corruption leaves trailing bytes.
func decodeOnboardingStateDoc(data []byte, doc *onboardingStateDoc) error {
	decoder := jsonx.NewDecoder(bytes.NewReader(data))
	return decoder.Decode(doc)
}

func (s *OnboardingStateStore) saveDocLocked(ctx context.Context, doc onboardingStateDoc) error {
	if s.path == "" {
		return fmt.Errorf("onboarding state path not configured")
	}
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}

	doc.Version = onboardingStateVersion
	normalizeOnboardingState(&doc.State)
	if doc.State.isZero() {
		if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove onboarding state: %w", err)
		}
		return nil
	}

	data, err := jsonx.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode onboarding state: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensure onboarding state directory: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write onboarding state temp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("commit onboarding state: %w", err)
	}
	return nil
}
