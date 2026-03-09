package context

import (
	"fmt"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/skills"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
)

const (
	maxActivatedSkillBodyChars = 2200
	maxAvailableSkillsEntries  = 24
	maxSkillDescriptionChars   = 120
)

func buildToolingSection(hints []string) string {
	lines := []string{
		"Tools are policy-governed; available set varies by channel/session.",
		"ALWAYS inspect tool definitions and argument schemas before side-effectful calls.",
		"NEVER assume a tool exists without checking; NEVER pass undocumented parameters.",
	}
	if len(hints) > 0 {
		lines = append(lines, "Runtime tool hints: "+strings.Join(hints, ", "))
	}
	return formatSection("# Tooling", lines)
}

// buildToolRoutingSection uses pseudocode decision tree + ALWAYS/NEVER binary rules
// instead of formatSection, because the multi-section structure (code block + sub-headings)
// doesn't fit formatSection's flat-line model.
// NOTE: Some NEVER rules (secrets, irreversible consent) intentionally overlap with
// buildSafetySection — redundancy in safety rules reinforces model compliance.
func buildToolRoutingSection() string {
	var sb strings.Builder
	sb.WriteString("# Tool Routing Guardrails\n")
	// Decision tree in pseudocode — model compliance is higher with structured if/else
	sb.WriteString("```\n")
	sb.WriteString("IF task_has_explicit_operation(replace, read, send, check):\n")
	sb.WriteString("  execute with concrete tool immediately\n")
	sb.WriteString("ELIF task_is_read_only_inspection(view, check, list, inspect):\n")
	sb.WriteString("  execute with read_file/list_dir/shell_exec; report findings\n")
	sb.WriteString("ELIF intent_is_unclear:\n")
	sb.WriteString("  inspect thread context + memory files via read_file/shell_exec\n")
	sb.WriteString("  IF still_unclear AND critical_input_missing:\n")
	sb.WriteString("    ask_user(action=clarify, needs_user_input=true) with ONE minimal question\n")
	sb.WriteString("ELIF user_delegates(\"you decide\", \"anything works\"):\n")
	sb.WriteString("  choose sensible default for low-risk reversible action; execute and report\n")
	sb.WriteString("ELIF needs_human_gate(login, 2FA, CAPTCHA, external confirmation):\n")
	sb.WriteString("  ask_user(action=request) with clear steps; wait\n")
	sb.WriteString("```\n")
	// ALWAYS rules — binary, no ambiguity
	sb.WriteString("## ALWAYS\n")
	sb.WriteString("- ALWAYS exhaust deterministic tools (read_file, shell_exec, bash) before asking the user.\n")
	sb.WriteString("- ALWAYS use read_file for workspace/repo files and memory files.\n")
	sb.WriteString("- ALWAYS use shell_exec for CLI commands and code snippets/scripts/computation.\n")
	sb.WriteString("- ALWAYS use write_file to create; replace_in_file to edit in place; artifacts_write for durable outputs.\n")
	sb.WriteString("- ALWAYS use web_search for URL discovery; web_fetch after URL is known.\n")
	sb.WriteString("- ALWAYS avoid interactive browser click-flow assumptions unless explicit browser automation tools are available.\n")
	sb.WriteString("- ALWAYS use shell_exec to run skill CLIs for channel delivery (e.g. python3 skills/feishu-cli/run.py).\n")
	sb.WriteString("- ALWAYS use skills for complex workflows (deep research, media generation, slide decks).\n")
	sb.WriteString("- ALWAYS default temp/generated files to /tmp with deterministic names.\n")
	sb.WriteString("- ALWAYS probe capabilities (command -v, --version) before declaring unavailable.\n")
	sb.WriteString("- ALWAYS inject runtime facts (cwd, OS, toolchain) before irreversible decisions.\n")
	// NEVER rules — explicit bans are more effective than vague positive instructions
	sb.WriteString("## NEVER\n")
	sb.WriteString("- NEVER use ask_user for explicit operational asks; execute with the concrete tool.\n")
	sb.WriteString("- NEVER ask for reconfirmation on explicit read_only_inspection requests.\n")
	sb.WriteString("- NEVER use plan for one-step operational actions (send message, run command).\n")
	sb.WriteString("- NEVER use browser/calendar tools for pure computation; use shell_exec.\n")
	sb.WriteString("- NEVER expose secrets in prompts/outputs; redact sensitive tokens by default.\n")
	sb.WriteString("- NEVER skip user consent for high-impact, irreversible, or external actions.\n")
	sb.WriteString("- NEVER declare a tool unavailable without probing first; search/install from trusted sources before escalating.\n")
	return strings.TrimSpace(sb.String())
}

func buildRuntimeSection(toolHints []string, mode string) string {
	lines := []string{
		fmt.Sprintf("Tool mode: %s", fallbackString(strings.TrimSpace(mode), "cli")),
	}
	if len(toolHints) > 0 {
		lines = append(lines, "Tool hints: "+strings.Join(toolHints, ", "))
	}
	lines = append(lines, "A <ctx/> tag per turn reports token budget and phase; adapt verbosity accordingly.")
	lines = append(lines, "Runtime profile should be inferred from channel + config, not guessed.")
	return formatSection("# Runtime", lines)
}

func buildSkillsSection(logger logging.Logger, taskInput string, messages []ports.Message, sessionID string, cfg agent.SkillsConfig) string {
	library, err := skills.CachedLibrary(time.Duration(cfg.AutoActivation.CacheTTLSeconds) * time.Second)
	if err != nil {
		logging.OrNop(logger).Warn("Failed to load skills: %v", err)
		return ""
	}

	autoCfg := skills.AutoActivationConfig{
		Enabled:             cfg.AutoActivation.Enabled,
		MaxActivated:        cfg.AutoActivation.MaxActivated,
		TokenBudget:         cfg.AutoActivation.TokenBudget,
		ConfidenceThreshold: cfg.AutoActivation.ConfidenceThreshold,
		FallbackToIndex:     cfg.AutoActivation.FallbackToIndex,
	}

	feedbackStore := skills.NewFeedbackStore(skills.FeedbackConfig{
		Enabled:   cfg.Feedback.Enabled,
		StorePath: cfg.Feedback.StorePath,
	})

	var matches []skills.MatchResult
	if autoCfg.Enabled && utils.HasContent(taskInput) {
		matcher := skills.NewSkillMatcher(&library, skills.MatcherOptions{FeedbackStore: feedbackStore})
		matches = matcher.Match(skills.MatchContext{
			TaskInput:   taskInput,
			RecentTools: extractRecentTools(messages, 8),
			SessionID:   sessionID,
		}, autoCfg)
		matches = skills.ApplyActivationLimits(matches, autoCfg)
		matcher.MarkActivated(sessionID, matches)
		if feedbackStore != nil {
			feedbackStore.RecordActivations(matches)
		}
	}

	var orchestrationSummary string
	if cfg.MetaOrchestratorEnabled {
		metaPolicy, policyErr := skills.LoadMetaPolicy(cfg.PolicyPath)
		if policyErr != nil {
			logging.OrNop(logger).Warn("Failed to load meta skill policy %q: %v", cfg.PolicyPath, policyErr)
			metaPolicy = skills.DefaultMetaPolicy()
		}
		metaPlan := skills.OrchestrateMatches(matches, skills.OrchestrationConfig{
			Enabled:                  cfg.MetaOrchestratorEnabled,
			SoulAutoEvolutionEnabled: cfg.SoulAutoEvolutionEnabled,
			ProactiveLevel:           cfg.ProactiveLevel,
		}, metaPolicy)
		matches = metaPlan.Selected
		orchestrationSummary = strings.TrimSpace(skills.RenderOrchestrationSummary(metaPlan))
	}

	var sb strings.Builder
	if orchestrationSummary != "" {
		sb.WriteString(orchestrationSummary)
		sb.WriteString("\n\n")
	}
	if len(matches) > 0 {
		sb.WriteString("# Activated Skills\n\n")
		sb.WriteString("The following skills were automatically activated based on the current task. Follow their workflow instructions.\n\n")
		for _, match := range matches {
			body := match.Skill.Body
			if len(match.Skill.Chain) > 0 {
				if resolved, err := library.ResolveChain(skills.SkillChain{Steps: match.Skill.Chain}); err == nil {
					body = resolved
				}
			}
			body = truncateSkillPromptText(body, maxActivatedSkillBodyChars)
			if utils.IsBlank(body) {
				continue
			}
			sb.WriteString(fmt.Sprintf("## Skill: %s (confidence: %.0f%%)\n\n", match.Skill.Name, match.Score*100))
			sb.WriteString(body)
			sb.WriteString("\n\n---\n\n")
		}
	}

	metadata := strings.TrimSpace(renderCompactAvailableSkillsXML(library, maxAvailableSkillsEntries))
	recentlyUsedSkills := hasRecentToolUsage(messages, "skills", 12)
	if metadata != "" && !recentlyUsedSkills {
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString("# Available Skills\n\n")
		sb.WriteString(metadata)
	}

	if !recentlyUsedSkills && (autoCfg.FallbackToIndex || len(matches) == 0) {
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString("# Skill Discovery\n\n")
		sb.WriteString("Use the `skills` tool to load playbooks on demand (action=list|search|show).\n")
		sb.WriteString("For `runner=py` entries, execute via `shell_exec` and follow each skill's invocation contract in SKILL.md.\n")
	}

	return strings.TrimSpace(sb.String())
}

func extractRecentTools(messages []ports.Message, limit int) []string {
	if limit <= 0 {
		return nil
	}
	seen := make(map[string]bool, limit)
	var tools []string
	for i := len(messages) - 1; i >= 0 && len(tools) < limit; i-- {
		msg := messages[i]
		if msg.Role == "assistant" {
			for _, call := range msg.ToolCalls {
				name := strings.TrimSpace(call.Name)
				if name == "" || seen[name] {
					continue
				}
				seen[name] = true
				tools = append(tools, name)
				if len(tools) >= limit {
					break
				}
			}
		}
		if msg.Role == "tool" {
			for _, result := range msg.ToolResults {
				name := extractToolNameFromMetadata(result.Metadata)
				if name == "" || seen[name] {
					continue
				}
				seen[name] = true
				tools = append(tools, name)
				if len(tools) >= limit {
					break
				}
			}
		}
	}
	return tools
}

func hasRecentToolUsage(messages []ports.Message, toolName string, limit int) bool {
	target := strings.TrimSpace(toolName)
	if target == "" {
		return false
	}
	recent := extractRecentTools(messages, limit)
	for _, name := range recent {
		if strings.EqualFold(strings.TrimSpace(name), target) {
			return true
		}
	}
	return false
}

func extractToolNameFromMetadata(metadata map[string]any) string {
	if len(metadata) == 0 {
		return ""
	}
	for _, key := range []string{"tool_name", "tool", "name"} {
		if raw, ok := metadata[key]; ok {
			if name, ok := raw.(string); ok {
				return strings.TrimSpace(name)
			}
		}
	}
	return ""
}

func truncateSkillPromptText(content string, maxChars int) string {
	trimmed := truncateSkillInlineText(content, maxChars)
	if trimmed == "" {
		return ""
	}
	if trimmed == strings.TrimSpace(content) {
		return trimmed
	}
	return trimmed + "\n\n[Skill instructions truncated for prompt budget. Use skills(action=show,name=...) for full workflow.]"
}

func renderCompactAvailableSkillsXML(library skills.Library, maxEntries int) string {
	skillList := library.List()
	if len(skillList) == 0 {
		return ""
	}
	if maxEntries <= 0 || maxEntries > len(skillList) {
		maxEntries = len(skillList)
	}

	var sb strings.Builder
	sb.WriteString("<available_skills>\n")
	sb.WriteString("- format: name | description | governance | activation | runner\n")
	sb.WriteString("- runner=py -> shell_exec python3 skills/<skill-name>/run.py ... (see SKILL.md for args)\n")
	for _, skill := range skillList[:maxEntries] {
		name := compactSkillField(skill.Name, "(unnamed)")
		desc := compactSkillField(truncateSkillInlineText(skill.Description, maxSkillDescriptionChars), "(no description)")
		level := compactSkillField(skill.GovernanceLevel, "-")
		mode := compactSkillField(skill.ActivationMode, "-")
		runner := "md"
		if skill.HasRunScript {
			runner = "py"
		}
		sb.WriteString(fmt.Sprintf("- %s | %s | %s | %s | %s\n", escapeSkillXML(name), escapeSkillXML(desc), escapeSkillXML(level), escapeSkillXML(mode), escapeSkillXML(runner)))
	}
	if len(skillList) > maxEntries {
		sb.WriteString(fmt.Sprintf("- ... (%d more; use skills(action=list|search|show))\n", len(skillList)-maxEntries))
	}
	sb.WriteString("</available_skills>")
	return strings.TrimSpace(sb.String())
}

var truncateSkillInlineText = ports.TruncateRuneSnippet

func compactSkillField(value string, fallback string) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if normalized == "" {
		return fallback
	}
	return normalized
}

func escapeSkillXML(value string) string {
	var builder strings.Builder
	for _, r := range value {
		switch r {
		case '&':
			builder.WriteString("&amp;")
		case '<':
			builder.WriteString("&lt;")
		case '>':
			builder.WriteString("&gt;")
		case '"':
			builder.WriteString("&quot;")
		case '\'':
			builder.WriteString("&apos;")
		default:
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
