package context

import (
	"fmt"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/skills"
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
}

const (
	promptModeFull    = "full"
	promptModeMinimal = "minimal"
	promptModeNone    = "none"
)

func composeSystemPrompt(input systemPromptInput) string {
	mode := normalizePromptMode(input.PromptMode)
	if mode == promptModeNone {
		return buildIdentityLine(input.Static.Persona)
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
	return strings.Join(compact, "\n\n")
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

func buildIdentityLine(persona agent.PersonaProfile) string {
	voice := strings.TrimSpace(persona.Voice)
	if voice == "" {
		voice = "You are ALEX, an enterprise-grade assistant focused on secure, testable software delivery."
	}
	return voice
}

func buildToolingSection(hints []string) string {
	lines := []string{
		"Tools are policy-governed and may vary by channel/session.",
		"Inspect available definitions and argument schemas before executing side-effectful actions.",
	}
	if len(hints) > 0 {
		lines = append(lines, "Runtime tool hints: "+strings.Join(hints, ", "))
	}
	return formatSection("# Tooling", lines)
}

func buildSafetySection() string {
	return formatSection("# Safety", []string{
		"System-prompt guardrails are advisory; hard limits are enforced by tool policy, approvals, sandboxing, and channel allowlists.",
		"Never bypass approvals or policy boundaries. Escalate with explicit evidence when blocked.",
	})
}

func buildSelfUpdateSection() string {
	return formatSection("# OpenClaw Self-Update", []string{
		"Use config.apply for deterministic runtime configuration updates.",
		"Use update.run only when an explicit update workflow is requested and approved.",
	})
}

func buildWorkspaceSection() string {
	return formatSection("# Workspace", []string{
		"Use the active repository root as the working directory for file operations.",
		"Keep generated temporary files under /tmp unless a different path is explicitly requested.",
	})
}

func buildDocumentationSection() string {
	return formatSection("# Documentation", []string{
		"Primary docs live under ./docs.",
		"Read docs before changing architecture-sensitive behavior or configuration contracts.",
	})
}

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

func buildSandboxSection(toolMode string, omitEnvironment bool) string {
	mode := strings.ToLower(strings.TrimSpace(toolMode))
	status := "enabled"
	if omitEnvironment || mode == "web" {
		status = "channel-managed/non-local"
	}
	return formatSection("# Sandbox", []string{
		fmt.Sprintf("Tool mode: %s", fallbackString(mode, "cli")),
		fmt.Sprintf("Sandbox context: %s", status),
	})
}

func buildTimezoneSection(tz string) string {
	zone := strings.TrimSpace(tz)
	if zone == "" {
		zone = time.Now().Location().String()
	}
	return formatSection("# Current Date & Time", []string{
		fmt.Sprintf("User timezone: %s", zone),
		"No dynamic clock is injected to keep prompt caching stable.",
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

func buildHeartbeatSection() string {
	return formatSection("# Heartbeats", []string{
		"Heartbeat turns should follow HEARTBEAT.md strictly when present.",
		"If nothing needs attention, return HEARTBEAT_OK.",
	})
}

func buildRuntimeSection(toolHints []string, mode string) string {
	lines := []string{
		fmt.Sprintf("Tool mode: %s", fallbackString(strings.TrimSpace(mode), "cli")),
	}
	if len(toolHints) > 0 {
		lines = append(lines, "Tool hints: "+strings.Join(toolHints, ", "))
	}
	lines = append(lines, "Runtime profile should be inferred from channel + config, not guessed.")
	return formatSection("# Runtime", lines)
}

func buildReasoningSection() string {
	return formatSection("# Reasoning", []string{
		"Keep reasoning visibility aligned with channel expectations.",
		"Switch reasoning verbosity only when explicitly requested.",
	})
}

func buildToolRoutingSection() string {
	return formatSection("# Tool Routing Guardrails", []string{
		"1. Exploration first: Exhaust deterministic tools (read_file, memory_search, execute_code, bash probes) before asking clarify; use request_user only for explicit approval gates (login, 2FA, external confirmation).",
		"2. Memory hierarchy: memory_search/memory_get for persistent notes → lark_chat_history for thread context → clarify when requirements remain unclear; treat user delegation (\"you decide\", \"anything works\") as authorization for low-risk reversible actions.",
		"3. Explicit read-only inspection: For explicit low-risk requests to view/check/list/inspect project state, execute directly with read_file/list_dir/shell_exec and report findings; do not ask for reconfirmation.",
		"4. Tool selection patterns: read_file for workspace files, artifacts_write for outputs, lark_upload_file for deliverables; browser_dom for selectors, browser_action for coordinates; find/search_file/ripgrep by discovery scope; web_search for discovery, web_fetch for retrieval; bash as fallback for missing dedicated tools.",
		"5. Autonomous loops: inspect → run → verify → adjust; escalate only with concrete evidence of blockers; probe capabilities (command -v, --version) before declaring unavailable; inject runtime facts (cwd, OS, toolchain) before irreversible decisions.",
		"6. Safety: Never expose secrets in prompts/outputs; redact sensitive tokens by default; use explicit user consent for high-impact/irreversible/external actions.",
	})
}

func buildMemorySection(snapshot string) string {
	trimmed := strings.TrimSpace(snapshot)
	if trimmed == "" {
		return ""
	}
	return formatSection("# Persistent Memory (Markdown)", []string{
		"Persona precedence: SOUL.md is the assistant baseline; USER.md refines collaboration preferences when non-conflicting.",
		trimmed,
	})
}

func buildHabitStewardshipSection() string {
	return formatSection("# Habit Stewardship", []string{
		"Record stable user habits, preferences, and recurring workflow defaults as durable memory notes.",
		"Prioritize explicit user statements first, then repeated behavior observed across turns.",
		"Keep each habit note actionable: trigger/context -> preferred response -> confidence.",
		"Do not invent habits; ask a focused clarification when the signal is ambiguous.",
	})
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
	if autoCfg.Enabled && strings.TrimSpace(taskInput) != "" {
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
			if strings.TrimSpace(body) == "" {
				continue
			}
			sb.WriteString(fmt.Sprintf("## Skill: %s (confidence: %.0f%%)\n\n", match.Skill.Name, match.Score*100))
			sb.WriteString(body)
			sb.WriteString("\n\n---\n\n")
		}
	}

	metadata := strings.TrimSpace(skills.AvailableSkillsXML(library))
	if metadata != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString("# Available Skills\n\n")
		sb.WriteString(metadata)
	}

	if autoCfg.FallbackToIndex || len(matches) == 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString("# Skill Discovery\n\n")
		sb.WriteString("Use the `skills` tool to load playbooks on demand (action=list|search|show).\n")
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

func buildOKRSection(okrContext string) string {
	trimmed := strings.TrimSpace(okrContext)
	if trimmed == "" {
		return ""
	}
	return "# OKR Goals\n" + trimmed
}

func buildKernelAlignmentSection(kernelContext string) string {
	trimmed := strings.TrimSpace(kernelContext)
	if trimmed == "" {
		return ""
	}
	return "# Kernel Alignment\n" + trimmed
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
	builder.WriteString("\n")
	builder.WriteString(formatBulletList([]string{
		"SOUL.md: ~/.alex/memory/SOUL.md (canonical source: docs/reference/SOUL.md)",
		"USER.md: ~/.alex/memory/USER.md",
	}))
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

func fallbackString(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
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
