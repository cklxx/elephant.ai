package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"alex/internal/app/subscription"
)

type onboardingStateStore interface {
	Get(context.Context) (subscription.OnboardingState, bool, error)
	Set(context.Context, subscription.OnboardingState) error
	Clear(context.Context) error
}

// OnboardingStateHandler serves internal onboarding state APIs.
type OnboardingStateHandler struct {
	store onboardingStateStore
	now   func() time.Time
}

func NewOnboardingStateHandler(store onboardingStateStore) *OnboardingStateHandler {
	if store == nil {
		return nil
	}
	return &OnboardingStateHandler{
		store: store,
		now:   time.Now,
	}
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
		Completed: strings.TrimSpace(state.CompletedAt) != "",
	})
}

func (h *OnboardingStateHandler) HandleUpdateOnboardingState(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.NotFound(w, r)
		return
	}
	var body onboardingStateUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}
	state := normalizeOnboardingState(body.State)
	if err := validateOnboardingState(state); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if stateIsEmpty(state) {
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
	if strings.TrimSpace(state.CompletedAt) == "" && state.SelectedProvider != "" && state.SelectedModel != "" {
		now := h.now
		if now == nil {
			now = time.Now
		}
		state.CompletedAt = now().UTC().Format(time.RFC3339)
	}
	if err := h.store.Set(r.Context(), state); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, onboardingStateResponse{
		State:     state,
		Completed: strings.TrimSpace(state.CompletedAt) != "",
	})
}

func normalizeOnboardingState(state subscription.OnboardingState) subscription.OnboardingState {
	state.CompletedAt = strings.TrimSpace(state.CompletedAt)
	state.SelectedProvider = strings.ToLower(strings.TrimSpace(state.SelectedProvider))
	state.SelectedModel = strings.TrimSpace(state.SelectedModel)
	state.SelectedRuntimeMode = strings.ToLower(strings.TrimSpace(state.SelectedRuntimeMode))
	state.PersistenceMode = strings.ToLower(strings.TrimSpace(state.PersistenceMode))
	state.UsedSource = strings.TrimSpace(state.UsedSource)
	return state
}

func validateOnboardingState(state subscription.OnboardingState) error {
	provider := strings.TrimSpace(state.SelectedProvider)
	model := strings.TrimSpace(state.SelectedModel)
	switch {
	case provider == "" && model != "":
		return httpError("state.selected_provider is required when selected_model is set")
	case provider != "" && model == "":
		return httpError("state.selected_model is required when selected_provider is set")
	}
	if strings.TrimSpace(state.CompletedAt) != "" {
		if _, err := time.Parse(time.RFC3339, state.CompletedAt); err != nil {
			return httpError("state.completed_at must be RFC3339 timestamp")
		}
	}
	if state.SelectedRuntimeMode != "" {
		switch state.SelectedRuntimeMode {
		case "cli", "lark", "full-dev":
		default:
			return httpError("state.selected_runtime_mode must be one of cli|lark|full-dev")
		}
	}
	if state.PersistenceMode != "" {
		switch state.PersistenceMode {
		case "file", "memory":
		default:
			return httpError("state.persistence_mode must be one of file|memory")
		}
	}
	return nil
}

func stateIsEmpty(state subscription.OnboardingState) bool {
	return strings.TrimSpace(state.CompletedAt) == "" &&
		strings.TrimSpace(state.SelectedProvider) == "" &&
		strings.TrimSpace(state.SelectedModel) == "" &&
		strings.TrimSpace(state.SelectedRuntimeMode) == "" &&
		strings.TrimSpace(state.PersistenceMode) == "" &&
		!state.LarkConfigured &&
		strings.TrimSpace(state.UsedSource) == "" &&
		!state.AdvancedOverridesUsed
}

type validationErr struct {
	msg string
}

func (e validationErr) Error() string {
	return e.msg
}

func httpError(msg string) error {
	return validationErr{msg: msg}
}
