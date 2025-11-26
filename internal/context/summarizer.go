package context

import (
	"fmt"
	"strings"
	"text/template"

	"alex/internal/agent/ports"
	"gopkg.in/yaml.v3"
)

// Citation references the origin of a summarized fact.
// Ref is intentionally stable (message index based) to allow lookup in persisted logs.
type Citation struct {
	Source ports.MessageSource `json:"source"`
	Ref    string              `json:"ref"`
}

// SummaryBullet holds a compressed statement plus its citations.
type SummaryBullet struct {
	Text      string     `json:"text"`
	Citations []Citation `json:"citations"`
}

// HistorySummary is a structured output for dynamic history compression.
// Bullets contain citations, and the last raw turn is preserved verbatim for
// quote-bait prompts such as "你的上一句是什么".
type HistorySummary struct {
	Bullets     []SummaryBullet `json:"bullets"`
	LastRawTurn ports.Message   `json:"last_raw_turn"`
}

// HistorySummarizer builds structured summaries with citations while keeping
// the most recent turn verbatim.
type HistorySummarizer struct {
	templates HistoryTemplates
}

// NewHistorySummarizer constructs a summarizer instance.
func NewHistorySummarizer() *HistorySummarizer {
	return NewHistorySummarizerWithTemplates(DefaultHistoryTemplates())
}

// NewHistorySummarizerWithTemplates injects custom rendering templates.
func NewHistorySummarizerWithTemplates(tpls HistoryTemplates) *HistorySummarizer {
	if tpls.User == nil || tpls.Assistant == nil || tpls.Tool == nil {
		tpls = DefaultHistoryTemplates()
	}
	return &HistorySummarizer{templates: tpls}
}

// Summarize condenses prior turns into citation-backed bullets and returns the
// most recent turn uncompressed.
func (s *HistorySummarizer) Summarize(messages []ports.Message) HistorySummary {
	if len(messages) == 0 {
		return HistorySummary{}
	}

	lastIdx := len(messages) - 1
	last := messages[lastIdx]

	bullets := buildRoleBullets(messages[:lastIdx], s.templates)

	return HistorySummary{Bullets: bullets, LastRawTurn: last}
}

func buildRoleBullets(messages []ports.Message, tpls HistoryTemplates) []SummaryBullet {
	if len(messages) == 0 {
		return nil
	}

	var (
		bullets   []SummaryBullet
		userRefs  []Citation
		agentRefs []Citation
		toolRefs  []Citation
		userSnip  string
		agentSnip string
		toolSnip  string
	)

	for idx, msg := range messages {
		source := deriveSource(msg)
		ref := Citation{Source: source, Ref: fmt.Sprintf("msg_%d", idx)}
		snippet := summarizeContent(msg.Content)
		switch source {
		case ports.MessageSourceUserInput, ports.MessageSourceUserHistory:
			if snippet != "" {
				userSnip = snippet
				userRefs = append(userRefs, ref)
			}
		case ports.MessageSourceAssistantReply:
			if snippet != "" {
				agentSnip = snippet
				agentRefs = append(agentRefs, ref)
			}
		case ports.MessageSourceToolResult:
			if snippet != "" {
				toolSnip = snippet
				toolRefs = append(toolRefs, ref)
			}
		}
	}

	if userSnip != "" {
		bullets = append(bullets, SummaryBullet{
			Text:      tpls.RenderUser(userSnip),
			Citations: userRefs,
		})
	}
	if agentSnip != "" {
		bullets = append(bullets, SummaryBullet{
			Text:      tpls.RenderAssistant(agentSnip),
			Citations: agentRefs,
		})
	}
	if toolSnip != "" {
		bullets = append(bullets, SummaryBullet{
			Text:      tpls.RenderTool(toolSnip),
			Citations: toolRefs,
		})
	}

	return bullets
}

func summarizeContent(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	const limit = 160
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return string(runes[:limit]) + "…"
}

func deriveSource(msg ports.Message) ports.MessageSource {
	if msg.Source != ports.MessageSourceUnknown {
		return msg.Source
	}
	role := strings.ToLower(strings.TrimSpace(msg.Role))
	switch role {
	case "user":
		return ports.MessageSourceUserInput
	case "assistant":
		return ports.MessageSourceAssistantReply
	case "tool":
		return ports.MessageSourceToolResult
	default:
		return msg.Source
	}
}

// HistoryTemplates defines renderers for each summary bullet type.
type HistoryTemplates struct {
	User      *template.Template
	Assistant *template.Template
	Tool      *template.Template
	LastRaw   *template.Template
}

// DefaultHistoryTemplates provides built-in sentence templates.
func DefaultHistoryTemplates() HistoryTemplates {
	return mustCompileHistoryTemplates(HistoryTemplateStrings{
		User:      "Recent user intent: {{.Snippet}}",
		Assistant: "Assistant reply: {{.Snippet}}",
		Tool:      "Tool signals: {{.Snippet}}",
		LastRaw:   "Last raw turn preserved ({{.Provenance}}): {{.Snippet}}",
	})
}

// HistoryTemplateStrings exposes YAML-friendly string forms.
type HistoryTemplateStrings struct {
	User      string `yaml:"user"`
	Assistant string `yaml:"assistant"`
	Tool      string `yaml:"tool"`
	LastRaw   string `yaml:"last_raw"`
}

// LoadHistoryTemplates parses YAML template strings from a file path.
func LoadHistoryTemplates(data []byte) (HistoryTemplates, error) {
	var t HistoryTemplateStrings
	if err := yaml.Unmarshal(data, &t); err != nil {
		return HistoryTemplates{}, err
	}
	if strings.TrimSpace(t.User) == "" || strings.TrimSpace(t.Assistant) == "" || strings.TrimSpace(t.Tool) == "" {
		return DefaultHistoryTemplates(), nil
	}
	return mustCompileHistoryTemplates(t), nil
}

func mustCompileHistoryTemplates(strings HistoryTemplateStrings) HistoryTemplates {
	compile := func(name, text string) *template.Template {
		tpl, err := template.New(name).Parse(text)
		if err != nil {
			// Fallback to a minimal template on parse errors to keep summarization resilient.
			tpl = template.Must(template.New(name).Parse("{{.Snippet}}"))
		}
		return tpl
	}
	return HistoryTemplates{
		User:      compile("user", strings.User),
		Assistant: compile("assistant", strings.Assistant),
		Tool:      compile("tool", strings.Tool),
		LastRaw:   compile("last_raw", strings.LastRaw),
	}
}

func (t HistoryTemplates) RenderUser(snippet string) string {
	return executeTemplate(t.User, snippet, "")
}

func (t HistoryTemplates) RenderAssistant(snippet string) string {
	return executeTemplate(t.Assistant, snippet, "")
}

func (t HistoryTemplates) RenderTool(snippet string) string {
	return executeTemplate(t.Tool, snippet, "")
}

func (t HistoryTemplates) RenderLastRaw(snippet, provenance string) string {
	return executeTemplate(t.LastRaw, snippet, provenance)
}

func executeTemplate(tpl *template.Template, snippet, provenance string) string {
	if tpl == nil {
		return snippet
	}
	var sb strings.Builder
	_ = tpl.Execute(&sb, map[string]string{"Snippet": snippet, "Provenance": provenance})
	return sb.String()
}
