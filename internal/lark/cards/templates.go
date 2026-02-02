package cards

import (
	"fmt"
	"sort"
	"strings"
)

// --- Progress step ---

// ProgressStep represents a single step in a progress card.
type ProgressStep struct {
	Name   string // Human-readable step name.
	Status string // "pending", "active", or "done".
}

// --- Pre-built card templates ---

// ApprovalCard builds a card with Approve and Reject buttons.
// The approvalID is attached to the button values so the handler can identify the request.
func ApprovalCard(title, description, approvalID string) (string, error) {
	return NewCard(CardConfig{Title: title, TitleColor: "orange", EnableForward: false}).
		AddMarkdownSection(description).
		AddDivider().
		AddActionButtons(
			NewPrimaryButton("Approve", "approval_approve").
				WithValue("approval_id", approvalID),
			NewDangerButton("Reject", "approval_reject").
				WithValue("approval_id", approvalID),
		).
		Build()
}

// ConfirmationCard builds a simple yes/no confirmation card.
func ConfirmationCard(title, description string, confirmLabel, cancelLabel string) (string, error) {
	return NewCard(CardConfig{Title: title, TitleColor: "blue", EnableForward: false}).
		AddMarkdownSection(description).
		AddDivider().
		AddActionButtons(
			NewPrimaryButton(confirmLabel, "confirm_yes"),
			NewButton(cancelLabel, "confirm_no"),
		).
		Build()
}

// ProgressCard builds a card showing task progress through a series of steps.
// currentStep is a 0-based index into steps indicating the active step.
func ProgressCard(title string, steps []ProgressStep, currentStep int) (string, error) {
	card := NewCard(CardConfig{Title: title, TitleColor: "green", EnableForward: true})

	var lines []string
	for i, s := range steps {
		icon := statusIcon(s.Status, i, currentStep)
		lines = append(lines, fmt.Sprintf("%s **%s**", icon, s.Name))
	}
	card.AddMarkdownSection(strings.Join(lines, "\n"))

	progress := progressText(currentStep, len(steps))
	card.AddNote(progress)

	return card.Build()
}

// SummaryCard builds a card with key-value sections.
// Keys are sorted alphabetically for deterministic output.
func SummaryCard(title string, sections map[string]string) (string, error) {
	card := NewCard(CardConfig{Title: title, TitleColor: "blue", EnableForward: true})

	keys := make([]string, 0, len(sections))
	for k := range sections {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var lines []string
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("**%s**: %s", k, sections[k]))
	}
	card.AddMarkdownSection(strings.Join(lines, "\n"))

	return card.Build()
}

// --- helpers ---

func statusIcon(status string, index, currentStep int) string {
	switch {
	case status == "done" || index < currentStep:
		return "[done]"
	case status == "active" || index == currentStep:
		return "[active]"
	default:
		return "[pending]"
	}
}

func progressText(current, total int) string {
	if total == 0 {
		return "0/0 steps completed"
	}
	done := current
	if done > total {
		done = total
	}
	if done < 0 {
		done = 0
	}
	return fmt.Sprintf("%d/%d steps completed", done, total)
}
