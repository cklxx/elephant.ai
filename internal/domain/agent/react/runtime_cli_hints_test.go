package react

import (
	"errors"
	"strings"
	"testing"

	agent "alex/internal/domain/agent/ports/agent"
)

func TestFormatExternalInputRequestMessage_UsesExplicitTeamCLICommands(t *testing.T) {
	msg := formatExternalInputRequestMessage(agent.InputRequest{
		TaskID:    "task-123",
		RequestID: "req-456",
		AgentType: "codex",
		Summary:   "Need approval for shell command",
	})

	if !strings.Contains(msg, "alex team reply --task-id \"task-123\" --request-id \"req-456\" --decision approve|reject") {
		t.Fatalf("expected reply guidance in message, got %q", msg)
	}
	if !strings.Contains(msg, "alex team inject --task-id \"task-123\" --message") {
		t.Fatalf("expected inject guidance in message, got %q", msg)
	}
	if strings.Contains(msg, "--approved=true|false") {
		t.Fatalf("expected legacy approved flag guidance removed, got %q", msg)
	}
}

func TestClassifyNonRetryableToolFailure_TemplateHintUsesTemplatesCommand(t *testing.T) {
	got, ok := classifyNonRetryableToolFailure(errors.New("template \"missing\" not found"))
	if !ok {
		t.Fatal("expected classification hit")
	}
	if got.signature != "template_not_found" {
		t.Fatalf("signature = %q, want template_not_found", got.signature)
	}
	if !strings.Contains(got.hint, "alex team templates") {
		t.Fatalf("expected templates command hint, got %q", got.hint)
	}
	if strings.Contains(got.hint, "--template list") {
		t.Fatalf("expected legacy template-list hint removed, got %q", got.hint)
	}
}
