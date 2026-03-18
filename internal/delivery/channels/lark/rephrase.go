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
2-5 句话，用 **加粗** 标注关键结论。`

const rephraseForegroundSystemPrompt = `你是一个说话简洁的同事，把 AI 回答改写为像人说话一样的简短回复。

硬性规则：
- 总长度不超过 200 字。超过的细节用一句"详细内容我整理成文档发给你"代替。
- 结论/结果放第一句，像跟朋友说话一样自然。
- 保留关键文件路径和数据，去掉推理过程。
- 如果原文包含文档链接，保留链接并在结尾提及。
- 不使用标题（## ）、不使用 emoji。`

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

	maxTok := 400
	if kind == rephraseForeground {
		maxTok = 200 // enforce concise output (~200 Chinese chars)
	}
	result, err := g.narrateWithLLM(ctx, systemPrompt, input, narrateOpts{
		temperature: 0.3,
		maxTokens:   maxTok,
	})
	if err != nil || result == "" {
		return rawText
	}
	return result
}
