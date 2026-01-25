package preparation

import (
	"strings"
	"testing"
	"time"

	"alex/internal/agent/ports"
)

func TestBuildImportantNotesMessageFormatsChronologically(t *testing.T) {
	now := time.Now()
	notes := map[string]ports.ImportantNote{
		"n2": {
			ID:        "n2",
			Content:   "User prefers dark mode.",
			Source:    "attention",
			Tags:      []string{"preference"},
			CreatedAt: now.Add(2 * time.Minute),
		},
		"n1": {
			ID:        "n1",
			Content:   "Primary IDE: VS Code.",
			Source:    "attention",
			Tags:      []string{"identity"},
			CreatedAt: now.Add(time.Minute),
		},
	}

	msg := buildImportantNotesMessage(notes)
	if msg == nil {
		t.Fatalf("expected message to be built")
	}
	if msg.Source != ports.MessageSourceImportant {
		t.Fatalf("expected message source to be important, got %q", msg.Source)
	}
	if msg.Role != "system" {
		t.Fatalf("expected role to be system, got %q", msg.Role)
	}
	if !strings.Contains(msg.Content, "1. Primary IDE: VS Code.") {
		t.Fatalf("expected chronological ordering, got %q", msg.Content)
	}
	if !strings.Contains(msg.Content, "2. User prefers dark mode.") {
		t.Fatalf("expected second entry, got %q", msg.Content)
	}
	if !strings.Contains(msg.Content, "recorded:") {
		t.Fatalf("expected recorded timestamp to be included, got %q", msg.Content)
	}
}
