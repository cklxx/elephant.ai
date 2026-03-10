package http

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"alex/internal/app/subscription"
	"alex/internal/shared/utils"
)

type onboardingStateStore interface {
	Get(context.Context) (subscription.OnboardingState, bool, error)
	Set(context.Context, subscription.OnboardingState) error
	Clear(context.Context) error
}

// OnboardingStateHandler serves internal onboarding state APIs.
type OnboardingStateHandler struct {
	store onboardingStateStore
}

func NewOnboardingStateHandler(store onboardingStateStore) *OnboardingStateHandler {
	if store == nil {
		return nil
	}
	return &OnboardingStateHandler{store: store}
}

type onboardingStateResponse struct {
	State     subscription.OnboardingState `json:"state"`
	Completed bool                         `json:"completed"`
}

type onboardingStateUpdateRequest struct {
	State subscription.OnboardingState `json:"state"`
}

func (h *OnboardingStateHandler) HandleGetOnboardingState(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.NotFound(w, r)
		return
	}
	state, ok, err := h.store.Get(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, onboardingStateResponse{
			State:     subscription.OnboardingState{},
			Completed: false,
		})
		return
	}
	writeJSON(w, http.StatusOK, onboardingStateResponse{
		State:     state,
		Completed: utils.HasContent(state.CompletedAt),
	})
}

func (h *OnboardingStateHandler) HandleUpdateOnboardingState(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.NotFound(w, r)
		return
	}
	var body onboardingStateUpdateRequest
	if !decodeJSONRequest(w, r, &body, "invalid JSON payload") {
		return
	}
	state := subscription.NormalizeOnboardingState(body.State)
	if err := validateOnboardingState(state); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if state.IsZero() {
		if err := h.store.Clear(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, onboardingStateResponse{
			State:     subscription.OnboardingState{},
			Completed: false,
		})
		return
	}
	// CompletedAt auto-populate is handled by the store's Set method.
	if err := h.store.Set(r.Context(), state); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Re-read from store to get the canonical state (with auto-populated CompletedAt).
	stored, ok, _ := h.store.Get(r.Context())
	if !ok {
		stored = state
	}
	writeJSON(w, http.StatusOK, onboardingStateResponse{
		State:     stored,
		Completed: utils.HasContent(stored.CompletedAt),
	})
}

func validateOnboardingState(state subscription.OnboardingState) error {
	provider := strings.TrimSpace(state.SelectedProvider)
	model := strings.TrimSpace(state.SelectedModel)
	switch {
	case provider == "" && model != "":
		return fmt.Errorf("state.selected_provider is required when selected_model is set")
	case provider != "" && model == "":
		return fmt.Errorf("state.selected_model is required when selected_provider is set")
	}
	if utils.HasContent(state.CompletedAt) {
		if _, err := time.Parse(time.RFC3339, state.CompletedAt); err != nil {
			return fmt.Errorf("state.completed_at must be RFC3339 timestamp")
		}
	}
	if state.SelectedRuntimeMode != "" {
		switch state.SelectedRuntimeMode {
		case "cli", "lark", "full-dev":
		default:
			return fmt.Errorf("state.selected_runtime_mode must be one of cli|lark|full-dev")
		}
	}
	if state.PersistenceMode != "" {
		switch state.PersistenceMode {
		case "file", "memory":
		default:
			return fmt.Errorf("state.persistence_mode must be one of file|memory")
		}
	}
	return nil
}
