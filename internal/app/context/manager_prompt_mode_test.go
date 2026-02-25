package context

import (
	"strings"
	"testing"

	agent "alex/internal/domain/agent/ports/agent"
)

func TestComposeSystemPrompt_NoneModeReturnsIdentityLine(t *testing.T) {
	prompt := composeSystemPrompt(systemPromptInput{
		PromptMode: "none",
		Static: agent.StaticContext{
			Persona: agent.PersonaProfile{
				Voice: "You are a concise assistant.",
			},
		},
	})
	if prompt != "You are a concise assistant." {
		t.Fatalf("unexpected none mode prompt: %q", prompt)
	}
}

func TestComposeSystemPrompt_MinimalOmitHeavySections(t *testing.T) {
	prompt := composeSystemPrompt(systemPromptInput{
		PromptMode: "minimal",
		Static: agent.StaticContext{
			Persona: agent.PersonaProfile{Voice: "persona voice"},
		},
		BootstrapRecords: []bootstrapRecord{
			{Name: "AGENTS.md", Content: "agent rules", Source: "global"},
		},
	})
	for _, required := range []string{"# Tooling", "# Safety", "# Runtime"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("expected minimal prompt to include %q", required)
		}
	}
	for _, forbidden := range []string{"# Workspace Files", "# OpenClaw Self-Update", "# Heartbeats"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("did not expect minimal prompt to include %q", forbidden)
		}
	}
}

func TestComposeSystemPrompt_FullIncludesWorkspaceFiles(t *testing.T) {
	prompt := composeSystemPrompt(systemPromptInput{
		PromptMode: "full",
		Static: agent.StaticContext{
			Persona: agent.PersonaProfile{Voice: "persona voice"},
		},
		BootstrapRecords: []bootstrapRecord{
			{Name: "AGENTS.md", Content: "agent rules", Source: "global", Path: "/tmp/AGENTS.md"},
		},
	})
	if !strings.Contains(prompt, "# Workspace Files") {
		t.Fatalf("expected workspace files section in full mode")
	}
	if !strings.Contains(prompt, "AGENTS.md") {
		t.Fatalf("expected file listing in full mode")
	}
}

