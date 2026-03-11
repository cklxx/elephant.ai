package lark

import (
	"context"
	"strings"
	"time"

	"alex/internal/app/agent/llmclient"
	ports "alex/internal/domain/agent/ports"
	runtimeconfig "alex/internal/shared/config"
)

const (
	intentInject = "INJECT"
	intentBTW    = "BTW"

	intentRouterTimeout  = 8 * time.Second
	intentRouterMaxTok   = 8
	intentRouterTemp     = 0.0
)

var intentRouterSystemPrompt = strings.TrimSpace(`
You are an intent classifier. A task is currently running in the background.
Your job is to decide whether a new user message is:
  INJECT — a follow-up, clarification, or correction that belongs to the running task
  BTW    — an independent side-question unrelated to the running task

Reply with exactly one word: INJECT or BTW. No punctuation, no explanation.
`)

// classifyBtwIntent calls a lightweight LLM to decide if the new user message
// should be injected into the running task (INJECT) or forked as a
// side-question (BTW). Falls back to BTW on any error.
func (g *Gateway) classifyBtwIntent(ctx context.Context, taskDesc string, newMsg string) string {
	if g.llmFactory == nil {
		return intentBTW
	}

	profile := g.resolveIntentRouterProfile()
	client, _, err := llmclient.GetClientFromProfile(g.llmFactory, profile, nil, false)
	if err != nil {
		g.logger.Warn("intent router: failed to get LLM client: %v; defaulting to BTW", err)
		return intentBTW
	}

	userPrompt := "Running task: " + strings.TrimSpace(taskDesc) + "\n\nNew message: " + strings.TrimSpace(newMsg)

	llmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), intentRouterTimeout)
	defer cancel()

	resp, err := client.Complete(llmCtx, ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: intentRouterSystemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: intentRouterTemp,
		MaxTokens:   intentRouterMaxTok,
	})
	if err != nil {
		g.logger.Warn("intent router: LLM call failed: %v; defaulting to BTW", err)
		return intentBTW
	}

	result := strings.ToUpper(strings.TrimSpace(resp.Content))
	// Accept prefix-match so "INJECT." or "INJECT\n" still works.
	if strings.HasPrefix(result, intentInject) {
		g.logger.Info("intent router: classified as INJECT (raw=%q)", resp.Content)
		return intentInject
	}
	g.logger.Info("intent router: classified as BTW (raw=%q)", resp.Content)
	return intentBTW
}

// btwIntentRouterEnabled returns whether the LLM-based btw/inject router is on.
func (g *Gateway) btwIntentRouterEnabled() bool {
	if g.cfg.BtwIntentRouterEnabled == nil {
		return false
	}
	return *g.cfg.BtwIntentRouterEnabled
}

// resolveIntentRouterProfile returns the LLM profile to use for intent
// classification, falling back to the gateway default when no override is set.
func (g *Gateway) resolveIntentRouterProfile() runtimeconfig.LLMProfile {
	if g.cfg.BtwIntentRouterModel != "" {
		p := g.llmProfile
		p.Model = g.cfg.BtwIntentRouterModel
		return p
	}
	return g.llmProfile
}
