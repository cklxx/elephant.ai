package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"alex/internal/app/subscription"
)

type memoryOnboardingStore struct {
	state subscription.OnboardingState
	ok    bool
}

func (s *memoryOnboardingStore) Get(context.Context) (subscription.OnboardingState, bool, error) {
	if !s.ok {
		return subscription.OnboardingState{}, false, nil
	}
	return s.state, true, nil
}

func (s *memoryOnboardingStore) Set(_ context.Context, state subscription.OnboardingState) error {
	s.state = state
	s.ok = true
	return nil
}

func (s *memoryOnboardingStore) Clear(context.Context) error {
	s.state = subscription.OnboardingState{}
	s.ok = false
	return nil
}

func TestOnboardingStateHandlerGetEmpty(t *testing.T) {
	t.Parallel()

	handler := NewOnboardingStateHandler(&memoryOnboardingStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/internal/onboarding/state", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetOnboardingState(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var payload onboardingStateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Completed {
		t.Fatalf("expected completed=false")
	}
}

func TestOnboardingStateHandlerUpdateAndGet(t *testing.T) {
	t.Parallel()

	store := &memoryOnboardingStore{}
	handler := NewOnboardingStateHandler(store)
	handler.now = func() time.Time {
		return time.Date(2026, 2, 8, 14, 0, 0, 0, time.UTC)
	}

	body := onboardingStateUpdateRequest{
		State: subscription.OnboardingState{
			SelectedProvider: "codex",
			SelectedModel:    "gpt-5.2-codex",
			UsedSource:       "codex_cli",
		},
	}
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/internal/onboarding/state", bytes.NewReader(data))
	rr := httptest.NewRecorder()
	handler.HandleUpdateOnboardingState(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"completed":true`) {
		t.Fatalf("expected completed=true, body=%s", rr.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/internal/onboarding/state", nil)
	getRR := httptest.NewRecorder()
	handler.HandleGetOnboardingState(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", getRR.Code)
	}
	if !strings.Contains(getRR.Body.String(), `"selected_provider":"codex"`) {
		t.Fatalf("unexpected get body: %s", getRR.Body.String())
	}
}

func TestOnboardingStateHandlerUpdateValidation(t *testing.T) {
	t.Parallel()

	handler := NewOnboardingStateHandler(&memoryOnboardingStore{})
	req := httptest.NewRequest(http.MethodPut, "/api/internal/onboarding/state", strings.NewReader(`{"state":{"selected_provider":"codex"}}`))
	rr := httptest.NewRecorder()

	handler.HandleUpdateOnboardingState(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func TestOnboardingStateHandlerUpdateClear(t *testing.T) {
	t.Parallel()

	store := &memoryOnboardingStore{
		ok: true,
		state: subscription.OnboardingState{
			CompletedAt:      "2026-02-08T14:00:00Z",
			SelectedProvider: "codex",
			SelectedModel:    "gpt-5.2-codex",
		},
	}
	handler := NewOnboardingStateHandler(store)

	req := httptest.NewRequest(http.MethodPut, "/api/internal/onboarding/state", strings.NewReader(`{"state":{}}`))
	rr := httptest.NewRecorder()
	handler.HandleUpdateOnboardingState(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if store.ok {
		t.Fatalf("expected store to be cleared")
	}
}
