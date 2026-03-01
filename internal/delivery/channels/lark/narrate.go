package lark

import (
	"context"
	"strings"
	"time"

	"alex/internal/app/agent/llmclient"
	ports "alex/internal/domain/agent/ports"
)

type narrateOpts struct {
	timeout     time.Duration
	maxTokens   int
	temperature float64
	maxReply    int // rune-level truncation on final output; 0 = no truncation
}

func (o narrateOpts) withDefaults() narrateOpts {
	if o.timeout <= 0 {
		o.timeout = 6 * time.Second
	}
	if o.maxTokens <= 0 {
		o.maxTokens = 300
	}
	if o.temperature <= 0 {
		o.temperature = 0.2
	}
	return o
}

// narrateWithLLM sends a system+user prompt pair to the gateway's LLM profile
// and returns the trimmed response. On any error it returns ("", err) so the
// caller can fall back to a template.
func (g *Gateway) narrateWithLLM(ctx context.Context, systemPrompt, userPrompt string, opts narrateOpts) (string, error) {
	opts = opts.withDefaults()

	client, _, err := llmclient.GetClientFromProfile(g.llmFactory, g.llmProfile, nil, false)
	if err != nil {
		return "", err
	}

	llmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), opts.timeout)
	defer cancel()

	resp, err := client.Complete(llmCtx, ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: opts.temperature,
		MaxTokens:   opts.maxTokens,
	})
	if err != nil {
		return "", err
	}

	result := strings.TrimSpace(resp.Content)
	if result == "" {
		return "", nil
	}
	if opts.maxReply > 0 {
		result = truncateForLark(result, opts.maxReply)
	}
	return result, nil
}

const narrateCycleSystemPrompt = `你是团队调度播报员。把内核调度周期结果用自然中文汇报，2-4 句话。
先说结论（全部成功/部分失败），再逐个概括每个 agent 做了什么。不使用 Markdown 和 emoji。`

// NarrateCycleNotification converts a raw structured cycle notification into
// natural Chinese via LLM. Returns ("", err) on failure so caller can fall back.
func (g *Gateway) NarrateCycleNotification(ctx context.Context, rawText string) (string, error) {
	return g.narrateWithLLM(ctx, narrateCycleSystemPrompt, rawText, narrateOpts{
		timeout:   8 * time.Second,
		maxTokens: 300,
	})
}
