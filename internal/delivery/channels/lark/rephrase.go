package lark

import (
	"context"

	"alex/internal/shared/utils"
)

type rephraseKind int

const (
	rephraseBackground rephraseKind = iota
	rephraseForeground

	rephraseMaxInput = 2000
)

const rephraseBackgroundSystemPrompt = `把后台任务完成结果改写为简洁自然的中文消息。
去掉 task_id、status、merge 等技术字段，保留关键结论、具体成果、耗时。
2-5 句话，不要使用 markdown 格式。`

const rephraseForegroundSystemPrompt = `把 AI 回答改写为更简洁易读的版本。
结构：结论/结果在第一句，关键上下文在后，细节只保留必要的。
保留所有关键信息和文件路径，去除冗余推理过程和重复陈述。
不要使用 markdown 格式（如 **加粗**、## 标题、- 列表、` + "`" + `代码` + "`" + `、[链接](url)），输出纯文本。`

func (g *Gateway) rephraseForUser(ctx context.Context, rawText string, kind rephraseKind) string {
	if g == nil || g.llmFactory == nil {
		return rawText
	}
	if utils.IsBlank(g.llmProfile.Provider) || utils.IsBlank(g.llmProfile.Model) {
		return rawText
	}
	if g.cfg.RephraseEnabled != nil && !*g.cfg.RephraseEnabled {
		return rawText
	}

	input := truncateForLark(rawText, rephraseMaxInput)
	if utils.IsBlank(input) {
		return rawText
	}

	var systemPrompt string
	switch kind {
	case rephraseBackground:
		systemPrompt = rephraseBackgroundSystemPrompt
	case rephraseForeground:
		systemPrompt = rephraseForegroundSystemPrompt
	default:
		return rawText
	}

	result, err := g.narrateWithLLM(ctx, systemPrompt, input, narrateOpts{
		temperature: 0.3,
		maxTokens:   400,
	})
	if err != nil || result == "" {
		return rawText
	}
	return result
}
