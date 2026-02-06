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

// PlanReviewParams controls the plan review card output.
type PlanReviewParams struct {
	Title                string
	Goal                 string
	PlanMarkdown         string
	RunID                string
	RequireConfirmation  bool
	IncludeFeedbackInput bool
}

// ResultParams controls the summary/result card output.
type ResultParams struct {
	Title         string
	Summary       string
	Footer        string
	TitleColor    string
	EnableForward bool
}

// AttachmentAsset describes an uploaded asset for attachment cards.
type AttachmentAsset struct {
	Name        string
	Kind        string // "image" or "file"
	ImageKey    string
	FileKey     string
	FileName    string
	ButtonTag   string
	ShowPreview bool
}

// AttachmentCardParams controls the attachment card output.
type AttachmentCardParams struct {
	Title      string
	Summary    string
	Footer     string
	TitleColor string
	Assets     []AttachmentAsset
}

// AwaitChoiceCard builds a user-input selection card for await_user_input prompts.
func AwaitChoiceCard(question string, options []string) (string, error) {
	trimmedQuestion := strings.TrimSpace(question)
	if trimmedQuestion == "" {
		return "", fmt.Errorf("question is required")
	}

	uniqueOptions := make([]string, 0, len(options))
	seen := make(map[string]struct{})
	for _, raw := range options {
		option := strings.TrimSpace(raw)
		if option == "" {
			continue
		}
		if _, exists := seen[option]; exists {
			continue
		}
		seen[option] = struct{}{}
		uniqueOptions = append(uniqueOptions, option)
	}
	if len(uniqueOptions) == 0 {
		return "", fmt.Errorf("at least one option is required")
	}

	card := NewCard(CardConfig{
		Title:         "请选择方案",
		TitleColor:    "orange",
		EnableForward: false,
	})
	card.AddMarkdownSection(trimmedQuestion)
	card.AddDivider()

	row := make([]Button, 0, 3)
	for _, option := range uniqueOptions {
		label := option
		if len(label) > 32 {
			label = label[:29] + "..."
		}
		row = append(row, NewButton(label, "await_choice_select").WithValue("text", option))
		if len(row) == 3 {
			card.AddActionButtons(row...)
			row = make([]Button, 0, 3)
		}
	}
	if len(row) > 0 {
		card.AddActionButtons(row...)
	}
	card.AddNote("点击一个选项继续。")
	return card.Build()
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

// PlanReviewCard builds a card used for plan review approvals.
func PlanReviewCard(params PlanReviewParams) (string, error) {
	title := strings.TrimSpace(params.Title)
	if title == "" {
		title = "计划确认"
	}
	card := NewCard(CardConfig{Title: title, TitleColor: "orange", EnableForward: false})

	var lines []string
	if goal := strings.TrimSpace(params.Goal); goal != "" {
		lines = append(lines, fmt.Sprintf("**目标**: %s", goal))
	}
	if plan := strings.TrimSpace(params.PlanMarkdown); plan != "" {
		lines = append(lines, fmt.Sprintf("**计划**:\n%s", plan))
	}
	if len(lines) > 0 {
		card.AddMarkdownSection(strings.Join(lines, "\n\n"))
	}
	if params.RequireConfirmation {
		card.AddNote("请确认执行或提交修改意见。")
	}
	if params.IncludeFeedbackInput {
		card.AddInput(InputConfig{
			Name:        "plan_feedback",
			Label:       "修改意见",
			Placeholder: "如需调整计划，请填写修改点（可选）",
		})
	}
	card.AddDivider()
	card.AddActionButtons(
		NewPrimaryButton("确认执行", "plan_review_approve").WithValue("run_id", params.RunID),
		NewButton("提交修改", "plan_review_request_changes").WithValue("run_id", params.RunID),
	)

	return card.Build()
}

// ResultCard builds a completion summary card.
func ResultCard(params ResultParams) (string, error) {
	title := strings.TrimSpace(params.Title)
	if title == "" {
		title = "任务完成"
	}
	color := strings.TrimSpace(params.TitleColor)
	if color == "" {
		color = "green"
	}
	card := NewCard(CardConfig{Title: title, TitleColor: color, EnableForward: params.EnableForward})
	if summary := strings.TrimSpace(params.Summary); summary != "" {
		card.AddMarkdownSection(summary)
	}
	if footer := strings.TrimSpace(params.Footer); footer != "" {
		card.AddNote(footer)
	}
	return card.Build()
}

// ErrorCard builds a failure summary card.
func ErrorCard(title, summary string) (string, error) {
	return ResultCard(ResultParams{
		Title:      title,
		Summary:    summary,
		TitleColor: "red",
	})
}

// AttachmentCard builds a completion summary card with attachment buttons.
func AttachmentCard(params AttachmentCardParams) (string, error) {
	title := strings.TrimSpace(params.Title)
	if title == "" {
		title = "任务完成"
	}
	color := strings.TrimSpace(params.TitleColor)
	if color == "" {
		color = "green"
	}
	card := NewCard(CardConfig{Title: title, TitleColor: color, EnableForward: true})
	if summary := strings.TrimSpace(params.Summary); summary != "" {
		card.AddMarkdownSection(summary)
	}
	if len(params.Assets) > 0 {
		card.AddDivider()
		card.AddMarkdownSection("**附件**")
		for _, asset := range params.Assets {
			name := strings.TrimSpace(asset.Name)
			if name == "" {
				name = strings.TrimSpace(asset.FileName)
			}
			if name == "" {
				name = "attachment"
			}
			if asset.Kind == "image" && asset.ImageKey != "" && asset.ShowPreview {
				card.AddImage(asset.ImageKey, name)
			}
			buttonLabel := "发送 " + name
			buttonTag := strings.TrimSpace(asset.ButtonTag)
			if buttonTag == "" {
				buttonTag = "attachment_send"
			}
			card.AddActionButtons(NewButton(buttonLabel, buttonTag).
				WithValue("attachment_name", name).
				WithValue("attachment_kind", asset.Kind).
				WithValue("image_key", asset.ImageKey).
				WithValue("file_key", asset.FileKey).
				WithValue("file_name", asset.FileName))
		}
	}
	if footer := strings.TrimSpace(params.Footer); footer != "" {
		card.AddNote(footer)
	}
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
