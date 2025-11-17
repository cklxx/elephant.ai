package app

import (
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

func TestResultAutoReviewerFlagsEmptyAnswer(t *testing.T) {
	reviewer := NewResultAutoReviewer(&AutoReviewOptions{Enabled: true, MinPassingScore: 0.5})
	assessment := reviewer.Review(&ports.TaskResult{Answer: ""})
	if assessment == nil || !assessment.NeedsRework {
		t.Fatalf("expected empty answer to require rework, got %+v", assessment)
	}
	if assessment.Grade != "F" {
		t.Fatalf("expected grade F, got %s", assessment.Grade)
	}
}

func TestResultAutoReviewerPassesDetailedAnswer(t *testing.T) {
	reviewer := NewResultAutoReviewer(&AutoReviewOptions{Enabled: true, MinPassingScore: 0.5})
	answer := strings.Repeat("Detailed explanation with concrete steps and references to fixes. ", 8)
	assessment := reviewer.Review(&ports.TaskResult{Answer: answer, Iterations: 3})
	if assessment == nil {
		t.Fatalf("expected assessment")
	}
	if assessment.NeedsRework {
		t.Fatalf("expected solid answer to pass, got assessment %+v", assessment)
	}
	if assessment.Score <= 0.5 {
		t.Fatalf("expected score above threshold, got %f", assessment.Score)
	}
}

func TestBuildReworkPromptIncludesNotes(t *testing.T) {
	assessment := &ports.ResultAssessment{Grade: "D", Score: 0.3, Notes: []string{"final answer is empty"}}
	prompt := buildReworkPrompt("Fix the bug", strings.Repeat("a", 2000), assessment, 1)
	if !strings.Contains(prompt, "automated reviewer") {
		t.Fatalf("expected reviewer context in prompt: %s", prompt)
	}
	if !strings.Contains(prompt, "attempt #2") {
		t.Fatalf("expected attempt counter in prompt: %s", prompt)
	}
	if len(prompt) < 100 {
		t.Fatalf("prompt unexpectedly short: %s", prompt)
	}
	if strings.Count(prompt, "...") == 0 {
		t.Fatalf("expected truncated prior answer indicator")
	}
}
