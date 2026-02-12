package kernel

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// InitDocSnapshot captures kernel runtime bootstrap details for INIT.md.
type InitDocSnapshot struct {
	GeneratedAt      time.Time
	KernelID         string
	Schedule         string
	StateDir         string
	StatePath        string
	InitPath         string
	SystemPromptPath string
	TimeoutSeconds   int
	LeaseSeconds     int
	MaxConcurrent    int
	Channel          string
	UserID           string
	ChatID           string
	SeedState        string
	Agents           []AgentConfig
}

// RenderInitMarkdown renders a deterministic INIT.md snapshot.
func RenderInitMarkdown(snapshot InitDocSnapshot) string {
	var b strings.Builder
	b.WriteString("# Kernel Initialization\n")
	b.WriteString(fmt.Sprintf("- generated_at: %s\n", formatTimestamp(snapshot.GeneratedAt)))
	b.WriteString(fmt.Sprintf("- kernel_id: %s\n\n", nonEmpty(snapshot.KernelID)))

	b.WriteString("## Runtime Config\n")
	b.WriteString(fmt.Sprintf("- schedule: %s\n", nonEmpty(snapshot.Schedule)))
	b.WriteString(fmt.Sprintf("- state_dir: %s\n", nonEmpty(snapshot.StateDir)))
	b.WriteString(fmt.Sprintf("- state_path: %s\n", nonEmpty(snapshot.StatePath)))
	b.WriteString(fmt.Sprintf("- init_path: %s\n", nonEmpty(snapshot.InitPath)))
	b.WriteString(fmt.Sprintf("- system_prompt_path: %s\n", nonEmpty(snapshot.SystemPromptPath)))
	b.WriteString(fmt.Sprintf("- timeout_seconds: %d\n", snapshot.TimeoutSeconds))
	b.WriteString(fmt.Sprintf("- lease_seconds: %d\n", snapshot.LeaseSeconds))
	b.WriteString(fmt.Sprintf("- max_concurrent: %d\n", snapshot.MaxConcurrent))
	b.WriteString(fmt.Sprintf("- channel: %s\n", nonEmpty(snapshot.Channel)))
	b.WriteString(fmt.Sprintf("- user_id: %s\n", nonEmpty(snapshot.UserID)))
	b.WriteString(fmt.Sprintf("- chat_id: %s\n\n", nonEmpty(snapshot.ChatID)))

	b.WriteString("## Seed State\n")
	b.WriteString("```md\n")
	b.WriteString(ensureTrailingNewline(snapshot.SeedState))
	b.WriteString("```\n\n")

	b.WriteString("## Agents\n")
	if len(snapshot.Agents) == 0 {
		b.WriteString("(none)\n")
		return b.String()
	}
	for i, agentCfg := range snapshot.Agents {
		b.WriteString(fmt.Sprintf("### %d. %s\n", i+1, nonEmpty(agentCfg.AgentID)))
		b.WriteString(fmt.Sprintf("- enabled: %t\n", agentCfg.Enabled))
		b.WriteString(fmt.Sprintf("- priority: %d\n", agentCfg.Priority))
		if len(agentCfg.Metadata) > 0 {
			keys := make([]string, 0, len(agentCfg.Metadata))
			for key := range agentCfg.Metadata {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			b.WriteString("- metadata:\n")
			for _, key := range keys {
				b.WriteString(fmt.Sprintf("  - %s: %s\n", key, agentCfg.Metadata[key]))
			}
		}
		b.WriteString("- prompt_template:\n")
		b.WriteString("```text\n")
		b.WriteString(ensureTrailingNewline(agentCfg.Prompt))
		b.WriteString("```\n\n")
	}
	return b.String()
}

// RenderSystemPromptMarkdown renders the current effective system prompt used by kernel-dispatched runs.
func RenderSystemPromptMarkdown(systemPrompt string, generatedAt time.Time) string {
	var b strings.Builder
	b.WriteString("# Kernel System Prompt\n")
	b.WriteString(fmt.Sprintf("- generated_at: %s\n", formatTimestamp(generatedAt)))
	b.WriteString("- source: AgentCoordinator.GetSystemPrompt()\n\n")
	b.WriteString("```text\n")
	if strings.TrimSpace(systemPrompt) == "" {
		b.WriteString("(empty)\n")
	} else {
		b.WriteString(ensureTrailingNewline(systemPrompt))
	}
	b.WriteString("```\n")
	return b.String()
}

func formatTimestamp(ts time.Time) string {
	if ts.IsZero() {
		return "unknown"
	}
	return ts.UTC().Format(time.RFC3339)
}

func ensureTrailingNewline(content string) string {
	if strings.HasSuffix(content, "\n") {
		return content
	}
	return content + "\n"
}

func nonEmpty(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "(empty)"
	}
	return trimmed
}
