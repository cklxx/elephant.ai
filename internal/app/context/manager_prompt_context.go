package context

import (
	"fmt"
	"strings"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
)

const defaultPersonaVoice = "You are ALEX, an enterprise-grade assistant focused on secure, testable software delivery."

func personaVoice(persona agent.PersonaProfile) string {
	if voice := strings.TrimSpace(persona.Voice); voice != "" {
		return voice
	}
	return defaultPersonaVoice
}

func buildIdentityLine(persona agent.PersonaProfile) string {
	return personaVoice(persona)
}

func buildIdentitySection(persona agent.PersonaProfile) string {
	var builder strings.Builder
	builder.WriteString("# Identity & Persona\n\n")
	builder.WriteString(personaVoice(persona))
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

func buildSafetySection() string {
	return formatSection("# Safety", []string{
		"Hard limits enforced by tool policy, approvals, sandboxing, and channel allowlists.",
		"NEVER bypass approval gates or policy boundaries.",
		"NEVER fabricate tool outputs, file contents, or completion claims.",
		"NEVER execute irreversible actions without explicit user consent.",
		"NEVER include secrets, API keys, or credentials in responses.",
		"When blocked, escalate with concrete evidence of the blocker.",
	})
}

// buildHabitStewardshipSection removed — habit recording rules folded into memory/policy config.

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

func buildKnowledgeSection(knowledge []agent.KnowledgeReference, sopSummaryOnly bool) string {
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

		// SOP content handling based on sopSummaryOnly flag
		if len(ref.SOPRefs) > 0 {
			if sopSummaryOnly {
				// Summary-only mode: show references with hint to use read_file
				var refLabels []string
				for _, sopRef := range ref.SOPRefs {
					refLabels = append(refLabels, SOPRefLabel(sopRef))
				}
				lines = append(lines, fmt.Sprintf("  - SOP refs: %s", strings.Join(refLabels, ", ")))
				lines = append(lines, "    (Use read_file tool to access full content when needed)")
			} else {
				// Full inline mode: render resolved SOP content inline when available
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
				} else {
					// Fallback to raw ref strings if resolution didn't happen
					lines = append(lines, fmt.Sprintf("  - SOP refs: %s", strings.Join(ref.SOPRefs, ", ")))
				}
			}
		}

		if len(ref.MemoryKeys) > 0 {
			lines = append(lines, fmt.Sprintf("  - Memory keys: %s", strings.Join(ref.MemoryKeys, ", ")))
		}
	}
	return formatSection("# Knowledge & Experience", lines)
}

func buildMemorySection(snapshot string) string {
	trimmed := strings.TrimSpace(snapshot)
	if trimmed == "" {
		return ""
	}
	return formatSection("# Persistent Memory (Markdown)", []string{
		"USER.md refines collaboration preferences. SOUL.md identity is injected via the Identity section above.",
		trimmed,
	})
}

func buildOKRSection(okrContext string) string {
	trimmed := strings.TrimSpace(okrContext)
	if trimmed == "" {
		return ""
	}
	return "# OKR Goals\n" + trimmed
}

func buildUnattendedOverrideSection() string {
	return `# CRITICAL OVERRIDE — Unattended Autonomous Mode

This session runs in UNATTENDED mode with NO human operator present.
All previous instructions about asking for confirmation, clarification, or user input are SUSPENDED for this session.

Mandatory rules (override all prior instructions):
- NEVER call clarify(), request_user(), or any tool that requires human response.
- NEVER ask "do you want me to…", "should I…", "which option…", or any confirmation question.
- NEVER output A/B choices or yes/no questions directed at a user.
- When facing ambiguity, make the best autonomous decision and document your rationale.
- When blocked, immediately switch to an alternative approach. Record the blocker and your pivot.
- Every turn must produce at least one concrete tool action with verifiable output.
- End with a factual "## Execution Summary" containing: completed work, evidence, decisions, next steps.`
}

// buildSelfUpdateSection removed — too specialized for every-conversation injection.

func buildWorkspaceSection() string {
	return formatSection("# Workspace & Docs", []string{
		"Use active repository root as working directory; default temp files to /tmp.",
		"NEVER write generated files into the repository tree unless explicitly requested.",
		"Primary docs live under ./docs; read them before changing architecture or config contracts.",
	})
}

// buildDocumentationSection removed — merged into buildWorkspaceSection.

func buildWorkspaceFilesSection(records []bootstrapRecord) string {
	if len(records) == 0 {
		return ""
	}
	lines := []string{
		"Bootstrap files injected on the first turn (Global-first, workspace fallback):",
	}
	for _, record := range records {
		name := strings.TrimSpace(record.Name)
		if name == "" {
			continue
		}
		if record.Missing {
			lines = append(lines, fmt.Sprintf("- %s: [missing file marker] %s", name, strings.TrimSpace(record.Path)))
			continue
		}
		label := fmt.Sprintf("- %s (%s): %s", name, record.Source, strings.TrimSpace(record.Path))
		if record.Truncated {
			label += " [TRUNCATED]"
		}
		lines = append(lines, label)
		if content := strings.TrimSpace(record.Content); content != "" {
			lines = append(lines, "  "+content)
		}
	}
	return formatSection("# Workspace Files", lines)
}

// buildSandboxSection removed — sandbox concept retired; tool mode lives in Runtime section.

func buildTimezoneSection(tz string) string {
	zone := strings.TrimSpace(tz)
	if zone == "" {
		zone = time.Now().Location().String()
	}
	return formatSection("# Current Date & Time", []string{
		fmt.Sprintf("Current date: %s", time.Now().Format("2006-01-02")),
		fmt.Sprintf("User timezone: %s", zone),
	})
}

func buildReplyTagsSection(enabled bool) string {
	if !enabled {
		return ""
	}
	return formatSection("# Reply Tags", []string{
		"Use provider-compatible reply tags when a channel explicitly requires tagged output syntax.",
	})
}

// buildHeartbeatSection removed — only relevant in heartbeat mode, not general conversations.

func buildReasoningSection() string {
	return formatSection("# Reasoning", []string{
		"Structure: conclusion → key context → supporting detail. Omit detail tiers that add no decision value.",
		"Match verbosity to channel: concise for chat, thorough for reports and analysis.",
	})
}

func buildChannelFormattingSection(hint string) string {
	if strings.TrimSpace(hint) == "" {
		return ""
	}
	return hint
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
	if len(lines) == 0 {
		return ""
	}
	return formatSection("# Operating Environment", lines)
}

func buildDynamicSection(dynamic agent.DynamicContext) string {
	var lines []string
	// Note: TurnID, LLMTurnSeq, SnapshotTimestamp, WorldState, and Feedback removed
	// - Already implicit in chat history or contain debug info not useful for LLM
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
	if len(lines) == 0 {
		return ""
	}
	return formatSection("# Live Session State", lines)
}

// buildMetaSection removed — persona version alone adds no decision value to the LLM.
