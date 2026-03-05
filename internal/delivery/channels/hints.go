package channels

// ChannelHints maps channel names to their prompt formatting hints.
// Each channel gateway registers its hint at startup.
type ChannelHints map[string]string

// LarkFormattingHint is the prompt section injected when the reply channel is Lark.
const LarkFormattingHint = `# Reply Formatting (Lark Channel)
Current reply channel is Lark; Lark text messages do not render Markdown.
For long-running or parallel execution, proactively send intermediate checkpoints via shell_exec + skills/feishu-cli/run.py so users can see progress.
Follow these formatting rules:
- Do not use Markdown syntax: avoid **bold**, *italic*, ## heading, - list, > quote, [link](url), and ` + "```" + `code` + "```" + ` fences.
- Use plain text formatting: separate paragraphs with newlines and use numbered lists for structure.
- For code snippets: keep content unchanged but do not wrap with ` + "```" + ` fences; inline short snippets.
- For links: paste raw URLs directly; do not use [text](url).
- For hierarchy: use numeric ordering (1. 2. 3.) instead of unordered bullets.
- For emphasis: use quoted keywords (e.g., "keyword") or prefix with ->.`

// DefaultHints returns the built-in channel hint registry.
func DefaultHints() ChannelHints {
	return ChannelHints{
		"lark": LarkFormattingHint,
	}
}
