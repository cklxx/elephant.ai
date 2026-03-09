package preparation

import (
	"strings"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/presets"
	runtimeconfig "alex/internal/shared/config"
	utils "alex/internal/shared/utils"
)

// buildSystemPrompt assembles the final system prompt, appending tool-mode
// specific instructions when applicable.
func (s *ExecutionPreparationService) buildSystemPrompt(pc *prepareContext) string {
	systemPrompt := strings.TrimSpace(pc.window.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = DefaultSystemPrompt
	}
	promptMode := utils.TrimLower(s.config.Proactive.Prompt.Mode)
	if promptMode != "none" {
		if pc.toolMode == presets.ToolModeCLI {
			systemPrompt = strings.TrimSpace(systemPrompt + `

## File Outputs
- When producing long-form deliverables (reports, articles, specs), write them to a Markdown file via write_file.
- Use /tmp as the default location for temporary/generated files unless the user requests another path.
- Always execute first. Exhaust all safe deterministic attempts before asking follow-up questions.
- If intent is unclear, inspect memory and thread context first (memory_search, then memory_get/memory_related, then local chat context snapshots when available).
- Ask only after all viable attempts fail and missing critical input still blocks progress.
- In Lark chats, use shell_exec + skill CLIs (for example skills/feishu-cli/run.py) for both text updates and file delivery.
- Provide a short summary in the final answer and point the user to the generated file path instead of pasting the full content.`)
		} else {
			systemPrompt = strings.TrimSpace(systemPrompt + `

## Artifacts & Attachments
- When producing long-form deliverables (reports, articles, specs), write them to a Markdown artifact via artifacts_write.
- Provide a short summary in the final answer and point the user to the generated file instead of pasting the full content.
- Keep attachment placeholders out of the main body; list them at the end of the final answer.
- If you want clients to render an attachment card, reference the file with a placeholder like [report.md].`)
		}
	}
	return systemPrompt
}

// buildSkillsConfig converts the runtime skills configuration into the
// domain-level SkillsConfig used by the context manager.
func buildSkillsConfig(cfg runtimeconfig.SkillsConfig) agent.SkillsConfig {
	return agent.SkillsConfig{
		AutoActivation: agent.SkillAutoActivationConfig{
			Enabled:             cfg.AutoActivation.Enabled,
			MaxActivated:        cfg.AutoActivation.MaxActivated,
			TokenBudget:         cfg.AutoActivation.TokenBudget,
			ConfidenceThreshold: cfg.AutoActivation.ConfidenceThreshold,
			CacheTTLSeconds:     cfg.CacheTTLSeconds,
			FallbackToIndex:     true,
		},
		Feedback: agent.SkillFeedbackConfig{
			Enabled:   cfg.Feedback.Enabled,
			StorePath: cfg.Feedback.StorePath,
		},
		MetaOrchestratorEnabled:  cfg.MetaOrchestratorEnabled,
		SoulAutoEvolutionEnabled: cfg.SoulAutoEvolutionEnabled,
		ProactiveLevel:           cfg.ProactiveLevel,
		PolicyPath:               cfg.PolicyPath,
	}
}
