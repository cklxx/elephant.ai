package context

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/logging"
	"alex/internal/skills"
)

func composeSystemPrompt(logger logging.Logger, static agent.StaticContext, dynamic agent.DynamicContext, meta agent.MetaContext, omitEnvironment bool) string {
	sections := []string{
		buildUserPersonaSection(static.UserPersona),
		buildIdentitySection(static.Persona),
		buildGoalsSection(static.Goal),
		buildPoliciesSection(static.Policies),
		buildKnowledgeSection(static.Knowledge),
		buildSkillsSection(logger),
	}
	if !omitEnvironment {
		sections = append(sections, buildEnvironmentSection(static))
	}
	sections = append(sections, buildDynamicSection(dynamic), buildMetaSection(meta))
	var compact []string
	for _, section := range sections {
		if trimmed := strings.TrimSpace(section); trimmed != "" {
			compact = append(compact, trimmed)
		}
	}
	return strings.Join(compact, "\n\n")
}

func buildUserPersonaSection(profile *ports.UserPersonaProfile) string {
	if profile == nil {
		return ""
	}

	var lines []string
	lines = append(lines, "This section defines the user's core persona and is the highest-priority reference for proactive behavior.")

	if profile.Summary != "" {
		lines = append(lines, fmt.Sprintf("Persona summary: %s", strings.TrimSpace(profile.Summary)))
	}
	if len(profile.TopDrives) > 0 {
		lines = append(lines, fmt.Sprintf("Primary drives: %s", strings.Join(profile.TopDrives, ", ")))
	}
	if len(profile.InitiativeSources) > 0 {
		lines = append(lines, fmt.Sprintf("Initiative sources: %s", strings.Join(profile.InitiativeSources, ", ")))
	}
	if len(profile.Values) > 0 {
		lines = append(lines, fmt.Sprintf("Core values: %s", strings.Join(profile.Values, ", ")))
	}
	if profile.DecisionStyle != "" {
		lines = append(lines, fmt.Sprintf("Decision style: %s", profile.DecisionStyle))
	}
	if profile.RiskProfile != "" {
		lines = append(lines, fmt.Sprintf("Risk profile: %s", profile.RiskProfile))
	}
	if profile.ConflictStyle != "" {
		lines = append(lines, fmt.Sprintf("Conflict style: %s", profile.ConflictStyle))
	}
	if profile.NonNegotiables != "" {
		lines = append(lines, fmt.Sprintf("Non-negotiables: %s", strings.TrimSpace(profile.NonNegotiables)))
	}

	if len(profile.CoreDrives) > 0 {
		var drives []string
		for _, drive := range profile.CoreDrives {
			if strings.TrimSpace(drive.Label) == "" {
				continue
			}
			drives = append(drives, fmt.Sprintf("%s (%d/5)", drive.Label, drive.Score))
		}
		if len(drives) > 0 {
			lines = append(lines, "Drive intensity:")
			lines = append(lines, prependBullet(drives, 1)...)
		}
	}

	goalLines := []string{}
	if strings.TrimSpace(profile.Goals.CurrentFocus) != "" {
		goalLines = append(goalLines, fmt.Sprintf("Current focus: %s", strings.TrimSpace(profile.Goals.CurrentFocus)))
	}
	if strings.TrimSpace(profile.Goals.OneYear) != "" {
		goalLines = append(goalLines, fmt.Sprintf("1-year goal: %s", strings.TrimSpace(profile.Goals.OneYear)))
	}
	if strings.TrimSpace(profile.Goals.ThreeYear) != "" {
		goalLines = append(goalLines, fmt.Sprintf("3-year goal: %s", strings.TrimSpace(profile.Goals.ThreeYear)))
	}
	if len(goalLines) > 0 {
		lines = append(lines, "Goals:")
		lines = append(lines, prependBullet(goalLines, 1)...)
	}

	if len(profile.Traits) > 0 {
		var traitLines []string
		for key, score := range profile.Traits {
			if strings.TrimSpace(key) == "" || score <= 0 {
				continue
			}
			traitLines = append(traitLines, fmt.Sprintf("%s: %d/7", key, score))
		}
		sort.Strings(traitLines)
		if len(traitLines) > 0 {
			lines = append(lines, "Traits (Big Five):")
			lines = append(lines, prependBullet(traitLines, 1)...)
		}
	}

	if len(profile.KeyChoices) > 0 {
		lines = append(lines, "Key choice defaults:")
		lines = append(lines, prependBullet(profile.KeyChoices, 1)...)
	}

	if len(profile.ConstructionRules) > 0 {
		lines = append(lines, "Construction rules:")
		lines = append(lines, prependBullet(profile.ConstructionRules, 1)...)
	}

	return formatSection("# User Persona Core (Highest Priority)", lines)
}

func buildSkillsSection(logger logging.Logger) string {
	library, err := skills.DefaultLibrary()
	if err != nil {
		logging.OrNop(logger).Warn("Failed to load skills: %v", err)
		return ""
	}
	return skills.IndexMarkdown(library)
}

func buildIdentitySection(persona agent.PersonaProfile) string {
	var builder strings.Builder
	voice := strings.TrimSpace(persona.Voice)
	if voice == "" {
		voice = "You are ALEX, an enterprise-grade assistant focused on secure, testable software delivery."
	}
	builder.WriteString("# Identity & Persona\n\n")
	builder.WriteString(voice)
	meta := formatBulletList(filterNonEmpty([]string{
		formatKeyValue("Tone", persona.Tone),
		formatKeyValue("Decision Style", persona.DecisionStyle),
		formatKeyValue("Risk Profile", persona.RiskProfile),
	}))
	if meta != "" {
		builder.WriteString("\n")
		builder.WriteString(meta)
	}
	return strings.TrimSpace(builder.String())
}

func buildGoalsSection(goal agent.GoalProfile) string {
	var lines []string
	if len(goal.LongTerm) > 0 {
		lines = append(lines, "Long-term:")
		lines = append(lines, prependBullet(goal.LongTerm, 1)...)
	}
	if len(goal.MidTerm) > 0 {
		lines = append(lines, "Mid-term:")
		lines = append(lines, prependBullet(goal.MidTerm, 1)...)
	}
	if len(goal.SuccessMetrics) > 0 {
		lines = append(lines, "Success metrics:")
		lines = append(lines, prependBullet(goal.SuccessMetrics, 1)...)
	}
	if len(lines) == 0 {
		return ""
	}
	return formatSection("# Mission Objectives", lines)
}

func buildPoliciesSection(policies []agent.PolicyRule) string {
	if len(policies) == 0 {
		return ""
	}
	var lines []string
	for _, policy := range policies {
		if len(policy.HardConstraints) == 0 && len(policy.SoftPreferences) == 0 && len(policy.RewardHooks) == 0 {
			continue
		}
		label := formatPolicyLabel(policy.ID)
		lines = append(lines, fmt.Sprintf("%s:", label))
		lines = append(lines, prependBullet(policy.HardConstraints, 1, "Hard constraints")...)
		lines = append(lines, prependBullet(policy.SoftPreferences, 1, "Soft preferences")...)
		lines = append(lines, prependBullet(policy.RewardHooks, 1, "Reward hooks")...)
	}
	if len(lines) == 0 {
		return ""
	}
	return formatSection("# Guardrails & Policies", lines)
}

func buildKnowledgeSection(knowledge []agent.KnowledgeReference) string {
	if len(knowledge) == 0 {
		return ""
	}
	var lines []string
	for _, ref := range knowledge {
		label := ref.ID
		if label == "" {
			label = ref.Description
		}
		label = strings.TrimSpace(label)
		if label == "" {
			label = "knowledge"
		}
		lines = append(lines, fmt.Sprintf("%s:", label))
		if ref.Description != "" {
			lines = append(lines, fmt.Sprintf("  - Summary: %s", ref.Description))
		}

		// Render resolved SOP content inline when available.
		if len(ref.ResolvedSOPContent) > 0 {
			for _, sopRef := range ref.SOPRefs {
				content, ok := ref.ResolvedSOPContent[sopRef]
				if !ok || content == "" {
					continue
				}
				sopLabel := SOPRefLabel(sopRef)
				lines = append(lines, fmt.Sprintf("  - SOP [%s]:", sopLabel))
				for _, cl := range strings.Split(content, "\n") {
					lines = append(lines, fmt.Sprintf("    %s", cl))
				}
			}
		} else if len(ref.SOPRefs) > 0 {
			// Fallback to raw ref strings if resolution didn't happen.
			lines = append(lines, fmt.Sprintf("  - SOP refs: %s", strings.Join(ref.SOPRefs, ", ")))
		}

		if len(ref.RAGCollections) > 0 {
			lines = append(lines, fmt.Sprintf("  - RAG collections: %s", strings.Join(ref.RAGCollections, ", ")))
		}
		if len(ref.MemoryKeys) > 0 {
			lines = append(lines, fmt.Sprintf("  - Memory keys: %s", strings.Join(ref.MemoryKeys, ", ")))
		}
	}
	return formatSection("# Knowledge & Experience", lines)
}

func buildEnvironmentSection(static agent.StaticContext) string {
	var lines []string
	if env := strings.TrimSpace(static.EnvironmentSummary); env != "" {
		lines = append(lines, fmt.Sprintf("Environment summary: %s", env))
	}
	if world := strings.TrimSpace(static.World.Environment); world != "" {
		lines = append(lines, fmt.Sprintf("World: %s", world))
	}
	if len(static.World.Capabilities) > 0 {
		lines = append(lines, fmt.Sprintf("Capabilities: %s", strings.Join(static.World.Capabilities, ", ")))
	}
	if len(static.World.Limits) > 0 {
		lines = append(lines, fmt.Sprintf("Limits: %s", strings.Join(static.World.Limits, ", ")))
	}
	if len(static.World.CostModel) > 0 {
		lines = append(lines, fmt.Sprintf("Cost awareness: %s", strings.Join(static.World.CostModel, ", ")))
	}
	if len(static.Tools) > 0 {
		lines = append(lines, fmt.Sprintf("Tool access: %s", strings.Join(static.Tools, ", ")))
	}
	if len(lines) == 0 {
		return ""
	}
	return formatSection("# Operating Environment", lines)
}

func buildDynamicSection(dynamic agent.DynamicContext) string {
	var lines []string
	if dynamic.TurnID > 0 || dynamic.LLMTurnSeq > 0 {
		lines = append(lines, fmt.Sprintf("Turn: %d (llm_seq=%d)", dynamic.TurnID, dynamic.LLMTurnSeq))
	}
	if !dynamic.SnapshotTimestamp.IsZero() {
		lines = append(lines, fmt.Sprintf("Snapshot captured: %s", dynamic.SnapshotTimestamp.Format(time.RFC3339)))
	}
	if len(dynamic.Plans) > 0 {
		lines = append(lines, "Plans:")
		lines = append(lines, formatPlanTree(dynamic.Plans, 1)...)
	}
	if len(dynamic.Beliefs) > 0 {
		beliefs := make([]string, 0, len(dynamic.Beliefs))
		for _, belief := range dynamic.Beliefs {
			beliefs = append(beliefs, fmt.Sprintf("%s (confidence %.2f)", belief.Statement, belief.Confidence))
		}
		lines = append(lines, "Beliefs:")
		lines = append(lines, prependBullet(beliefs, 1)...)
	}
	if len(dynamic.WorldState) > 0 {
		lines = append(lines, "World state summary:")
		lines = append(lines, summarizeMap(dynamic.WorldState, 1)...)
	}
	if len(dynamic.Feedback) > 0 {
		lines = append(lines, "Feedback signals:")
		var feedback []string
		for _, signal := range dynamic.Feedback {
			feedback = append(feedback, fmt.Sprintf("%s — %s (%.2f)", signal.Kind, signal.Message, signal.Value))
		}
		lines = append(lines, prependBullet(feedback, 1)...)
	}
	if len(lines) == 0 {
		return ""
	}
	return formatSection("# Live Session State", lines)
}

func buildMetaSection(meta agent.MetaContext) string {
	var lines []string
	if meta.PersonaVersion != "" {
		lines = append(lines, fmt.Sprintf("Persona version: %s", meta.PersonaVersion))
	}
	if len(meta.Memories) > 0 {
		lines = append(lines, "Memories:")
		var memoLines []string
		for _, memory := range meta.Memories {
			stamp := memory.CreatedAt.Format("2006-01-02")
			memoLines = append(memoLines, fmt.Sprintf("%s — %s (%s)", memory.Content, memory.Key, stamp))
		}
		lines = append(lines, prependBullet(memoLines, 1)...)
	}
	if len(meta.Recommendations) > 0 {
		lines = append(lines, "Recommendations:")
		lines = append(lines, prependBullet(meta.Recommendations, 1)...)
	}
	if len(lines) == 0 {
		return ""
	}
	return formatSection("# Meta Stewardship Directives", lines)
}

func formatSection(title string, lines []string) string {
	var builder strings.Builder
	if title != "" {
		builder.WriteString(title)
		builder.WriteString("\n")
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
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
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return fmt.Sprintf("%s: %s", key, value)
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

func summarizeMap(data map[string]any, depth int) []string {
	if len(data) == 0 {
		return nil
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var lines []string
	for _, key := range keys {
		value := data[key]
		lines = append(lines, strings.Repeat("  ", depth)+fmt.Sprintf("- %s: %v", key, value))
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
