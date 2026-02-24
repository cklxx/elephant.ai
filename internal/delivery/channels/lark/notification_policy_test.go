package lark

import (
	"testing"
	"time"

	domain "alex/internal/domain/agent"
	agentports "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
)

func TestNotificationPolicyProgressSuppressesNodeStartedWhenEnabled(t *testing.T) {
	policy := newNotificationPolicy(true)
	if policy.allowProgressEvent(types.EventNodeStarted) {
		t.Fatalf("expected node_started to be suppressed")
	}
	if !policy.allowProgressEvent(types.EventToolStarted) {
		t.Fatalf("expected tool_started to pass")
	}
}

func TestNotificationPolicyPlanClarifyBlockingOnlyWhenEnabled(t *testing.T) {
	enabled := newNotificationPolicy(true)
	if enabled.allowPlanClarify(planClarifyPayload{message: "plan update", needsInput: false}) {
		t.Fatalf("expected non-blocking plan clarify to be suppressed when policy is enabled")
	}
	if !enabled.allowPlanClarify(planClarifyPayload{message: "need input", needsInput: true}) {
		t.Fatalf("expected blocking clarify prompt to pass when policy is enabled")
	}

	disabled := newNotificationPolicy(false)
	if !disabled.allowPlanClarify(planClarifyPayload{message: "plan update", needsInput: false}) {
		t.Fatalf("expected non-blocking plan clarify to pass when policy is disabled")
	}
}

func TestNotificationPolicyBackgroundProgressFiltersHeartbeat(t *testing.T) {
	policy := newNotificationPolicy(true)

	heartbeat := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agentports.LevelCore, "sess", "run", "", time.Now()),
		Event:     types.EventExternalAgentProgress,
		Payload: map[string]any{
			"current_tool": "__heartbeat__",
		},
	}
	if policy.allowBackgroundProgress(heartbeat) {
		t.Fatalf("expected heartbeat progress to be filtered")
	}

	meaningful := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agentports.LevelCore, "sess", "run", "", time.Now()),
		Event:     types.EventExternalAgentProgress,
		Payload: map[string]any{
			"current_tool": "edit_file",
		},
	}
	if !policy.allowBackgroundProgress(meaningful) {
		t.Fatalf("expected meaningful progress to pass")
	}
}

func TestNotificationPolicyClassifiesClarifyNeedsInputAsBlocking(t *testing.T) {
	policy := newNotificationPolicy(true)
	env := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agentports.LevelCore, "sess", "run", "", time.Now()),
		Event:     types.EventToolCompleted,
		Payload: map[string]any{
			"tool_name": "clarify",
			"metadata": map[string]any{
				"needs_user_input": true,
			},
		},
	}
	if got := policy.modeForEvent(env); got != notificationModeBlocking {
		t.Fatalf("expected blocking mode, got %s", got)
	}
}
