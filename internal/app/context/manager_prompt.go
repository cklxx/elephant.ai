package context

import (
	"alex/internal/shared/utils"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/logging"
)

type systemPromptInput struct {
	Logger                 logging.Logger
	Static                 agent.StaticContext
	Dynamic                agent.DynamicContext
	Meta                   agent.MetaContext
	Memory                 string
	OmitEnvironment        bool
	TaskInput              string
	Messages               []ports.Message
	SessionID              string
	PromptMode             string
	PromptTimezone         string
	ReplyTagsEnabled       bool
	BootstrapRecords       []bootstrapRecord
	ToolMode               string
	SkillsConfig           agent.SkillsConfig
	OKRContext             string
	KernelAlignmentContext string
	SOPSummaryOnly         bool // If true, only show SOP references without full content
	Unattended             bool // If true, inject autonomous behavior override (no user interaction)
}

const (
	promptModeFull               = "full"
	promptModeMinimal            = "minimal"
	promptModeNone               = "none"
	maxComposedSystemPromptChars = 32000
)

func composeSystemPrompt(input systemPromptInput) string {
	mode := normalizePromptMode(input.PromptMode)
	if mode == promptModeNone {
		return clampSystemPromptSize(buildIdentityLine(input.Static.Persona))
	}

	fullSections := []string{
		buildIdentitySection(input.Static.Persona),
		buildToolingSection(input.Static.Tools),
		buildToolRoutingSection(),
		buildSafetySection(),
		buildHabitStewardshipSection(),
		buildGoalsSection(input.Static.Goal),
		buildPoliciesSection(input.Static.Policies),
		buildKnowledgeSection(input.Static.Knowledge, input.SOPSummaryOnly),
		buildMemorySection(input.Memory),
		buildOKRSection(input.OKRContext),
		buildKernelAlignmentSection(input.KernelAlignmentContext),
		buildSkillsSection(input.Logger, input.TaskInput, input.Messages, input.SessionID, input.SkillsConfig),
		buildSelfUpdateSection(),
		buildWorkspaceSection(),
		buildDocumentationSection(),
		buildWorkspaceFilesSection(input.BootstrapRecords),
		buildSandboxSection(input.ToolMode, input.OmitEnvironment),
		buildTimezoneSection(input.PromptTimezone),
		buildReplyTagsSection(input.ReplyTagsEnabled),
		buildHeartbeatSection(),
		buildRuntimeSection(input.Static.Tools, input.ToolMode),
		buildReasoningSection(),
	}
	if !input.OmitEnvironment {
		fullSections = append(fullSections, buildEnvironmentSection(input.Static))
	}
	fullSections = append(fullSections, buildDynamicSection(input.Dynamic), buildMetaSection(input.Meta))
	if input.Unattended {
		fullSections = append(fullSections, buildUnattendedOverrideSection())
	}

	minimalSections := []string{
		buildIdentitySection(input.Static.Persona),
		buildToolingSection(input.Static.Tools),
		buildToolRoutingSection(),
		buildSafetySection(),
		buildGoalsSection(input.Static.Goal),
		buildKernelAlignmentSection(input.KernelAlignmentContext),
		buildPoliciesSection(input.Static.Policies),
		buildWorkspaceSection(),
		buildDocumentationSection(),
		buildTimezoneSection(input.PromptTimezone),
		buildRuntimeSection(input.Static.Tools, input.ToolMode),
		buildReasoningSection(),
	}
	if !input.OmitEnvironment {
		minimalSections = append(minimalSections, buildEnvironmentSection(input.Static))
	}
	if input.Unattended {
		minimalSections = append(minimalSections, buildUnattendedOverrideSection())
	}

	var selected []string
	if mode == promptModeMinimal {
		selected = minimalSections
	} else {
		selected = fullSections
	}

	var compact []string
	for _, section := range selected {
		if trimmed := strings.TrimSpace(section); trimmed != "" {
			compact = append(compact, trimmed)
		}
	}
	return clampSystemPromptSize(strings.Join(compact, "\n\n"))
}

func normalizePromptMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case promptModeMinimal:
		return promptModeMinimal
	case promptModeNone:
		return promptModeNone
	default:
		return promptModeFull
	}
}

func formatSection(title string, lines []string) string {
	var builder strings.Builder
	if title != "" {
		builder.WriteString(title)
		builder.WriteString("\n")
	}
	for _, line := range lines {
		if utils.IsBlank(line) {
			continue
		}
		builder.WriteString(line)
		if !strings.HasSuffix(line, "\n") {
			builder.WriteString("\n")
		}
	}
	return strings.TrimSpace(builder.String())
}

func formatBulletList(items []string) string {
	if len(items) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		builder.WriteString("- ")
		builder.WriteString(trimmed)
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

func prependBullet(items []string, depth int, prefix ...string) []string {
	var lines []string
	if len(prefix) > 0 {
		lines = append(lines, strings.Repeat("  ", depth-1)+prefix[0]+":")
	}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		lines = append(lines, strings.Repeat("  ", depth)+"- "+trimmed)
	}
	return lines
}

func filterNonEmpty(items []string) []string {
	var result []string
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func formatKeyValue(key, value string) string {
	if utils.IsBlank(value) {
		return ""
	}
	return fmt.Sprintf("%s: %s", key, value)
}

func fallbackString(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func clampSystemPromptSize(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" || maxComposedSystemPromptChars <= 0 {
		return trimmed
	}
	runes := []rune(trimmed)
	if len(runes) <= maxComposedSystemPromptChars {
		return trimmed
	}
	clipped := strings.TrimSpace(string(runes[:maxComposedSystemPromptChars]))
	return clipped + "\n\n[System prompt truncated for model context safety. Use tools (skills/memory/read_file) for detailed lookup.]"
}

func formatPlanTree(nodes []agent.PlanNode, depth int) []string {
	var lines []string
	for _, node := range nodes {
		title := strings.TrimSpace(node.Title)
		if title == "" {
			title = node.ID
		}
		entry := title
		if node.Status != "" {
			entry = fmt.Sprintf("%s [%s]", entry, node.Status)
		}
		if node.Description != "" {
			entry = fmt.Sprintf("%s — %s", entry, node.Description)
		}
		lines = append(lines, strings.Repeat("  ", depth)+"- "+strings.TrimSpace(entry))
		if len(node.Children) > 0 {
			lines = append(lines, formatPlanTree(node.Children, depth+1)...)
		}
	}
	return lines
}

func formatPolicyLabel(id string) string {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return "Policy"
	}
	runes := []rune(trimmed)
	if len(runes) == 0 {
		return "Policy"
	}
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	return string(runes)
}
