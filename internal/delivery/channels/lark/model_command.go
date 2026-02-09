package lark

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/subscription"
	"alex/internal/delivery/channels"
	"alex/internal/infra/tools/builtin/shared"
	runtimeconfig "alex/internal/shared/config"
)

func (g *Gateway) applyPinnedLarkLLMSelection(ctx context.Context, msg *incomingMessage) context.Context {
	if g == nil || ctx == nil || msg == nil {
		return ctx
	}
	if g.llmSelections == nil || g.llmResolver == nil {
		return ctx
	}

	selection, matchedScope, ok, err := g.llmSelections.GetWithFallback(ctx, selectionScopes(msg)...)
	if err != nil {
		g.logger.Warn("Lark LLM selection load failed: %v", err)
		return ctx
	}
	if !ok {
		g.logger.Debug("Lark LLM selection: no selection found for chat=%s", msg.chatID)
		return ctx
	}
	g.logger.Debug("Lark LLM selection found: provider=%s model=%s scope=%s",
		selection.Provider, selection.Model, matchedScope.Channel)

	resolved, ok := g.llmResolver.Resolve(selection)
	if !ok {
		g.logger.Warn("Lark LLM selection resolve failed: provider=%q model=%q", selection.Provider, selection.Model)
		return ctx
	}
	return appcontext.WithLLMSelection(ctx, resolved)
}

func (g *Gateway) handleModelCommand(msg *incomingMessage) {
	if g == nil || msg == nil {
		return
	}

	sessionID := g.memoryIDForChat(msg.chatID)
	execCtx := channels.BuildBaseContext(g.cfg.BaseConfig, "lark", sessionID, msg.senderID, msg.chatID, msg.isGroup)
	execCtx = shared.WithLarkClient(execCtx, g.client)
	execCtx = shared.WithLarkChatID(execCtx, msg.chatID)
	execCtx = shared.WithLarkMessageID(execCtx, msg.messageID)

	trimmed := strings.TrimSpace(msg.content)
	fields := strings.Fields(trimmed)
	sub := ""
	if len(fields) > 1 {
		sub = strings.ToLower(strings.TrimSpace(fields[1]))
	}
	chatOnly := hasFlag(fields, "--chat")

	var reply string
	switch sub {
	case "", "list", "ls":
		_, content := g.buildModelListReply(execCtx, msg)
		g.dispatch(execCtx, msg.chatID, replyTarget(msg.messageID, true), "text", content)
		return
	case "use", "select", "set":
		if len(fields) < 3 {
			reply = modelCommandUsage()
			break
		}
		spec := firstNonFlag(fields[2:])
		if spec == "" {
			reply = modelCommandUsage()
			break
		}
		if err := g.setModelSelection(execCtx, msg, spec, chatOnly); err != nil {
			reply = fmt.Sprintf("设置失败：%v\n\n%s", err, modelCommandUsage())
			break
		}
		reply = g.buildModelStatus(execCtx, msg)
	case "clear", "reset":
		if err := g.clearModelSelection(execCtx, msg, chatOnly); err != nil {
			reply = fmt.Sprintf("清除失败：%v\n\n%s", err, modelCommandUsage())
			break
		}
		reply = "已清除订阅模型选择；后续将使用配置默认值。"
	case "status", "show", "current":
		reply = g.buildModelStatus(execCtx, msg)
	case "help", "-h", "--help":
		reply = modelCommandUsage()
	default:
		reply = modelCommandUsage()
	}

	g.dispatch(execCtx, msg.chatID, replyTarget(msg.messageID, true), "text", textContent(reply))
}

func modelCommandUsage() string {
	return strings.TrimSpace(`
Model command usage:
  /model                                List available subscription models
  /model use <provider>/<model>         Select globally (all Lark chats)
  /model use <provider>/<model> --chat  Select for this chat only (override)
  /model status                         Show current pinned selection
  /model clear                          Clear global selection
  /model clear --chat                   Clear this chat's override only

Examples:
  /model use codex/gpt-5.2-codex
  /model use anthropic/claude-sonnet-4-20250514 --chat
  /model use llama_server/local-model
`)
}

// hasFlag checks whether a flag like "--chat" appears in the fields slice.
func hasFlag(fields []string, flag string) bool {
	for _, f := range fields {
		if strings.EqualFold(f, flag) {
			return true
		}
	}
	return false
}

// firstNonFlag returns the first field that doesn't start with "--".
func firstNonFlag(fields []string) string {
	for _, f := range fields {
		if !strings.HasPrefix(f, "--") {
			return strings.TrimSpace(f)
		}
	}
	return ""
}

// channelScope returns the agent-global (Lark channel-level) selection scope.
func channelScope() subscription.SelectionScope {
	return subscription.SelectionScope{Channel: "lark"}
}

// chatScope returns the per-chat override selection scope.
func chatScope(msg *incomingMessage) subscription.SelectionScope {
	if msg == nil {
		return subscription.SelectionScope{Channel: "lark"}
	}
	return subscription.SelectionScope{Channel: "lark", ChatID: strings.TrimSpace(msg.chatID)}
}

// legacyChatUserScope returns the historical chat+user scope shape.
func legacyChatUserScope(msg *incomingMessage) (subscription.SelectionScope, bool) {
	if msg == nil {
		return subscription.SelectionScope{}, false
	}
	chatID := strings.TrimSpace(msg.chatID)
	userID := strings.TrimSpace(msg.senderID)
	if chatID == "" || userID == "" {
		return subscription.SelectionScope{}, false
	}
	return subscription.SelectionScope{Channel: "lark", ChatID: chatID, UserID: userID}, true
}

// selectionScopes builds lookup scopes from most specific to least specific.
// Order: chat-level -> legacy chat+user (DM-only compatibility) -> channel-level.
func selectionScopes(msg *incomingMessage) []subscription.SelectionScope {
	scopes := make([]subscription.SelectionScope, 0, 3)
	if msg != nil {
		if chatID := strings.TrimSpace(msg.chatID); chatID != "" {
			scopes = append(scopes, subscription.SelectionScope{Channel: "lark", ChatID: chatID})
		}
		// Group chats should be strictly chat-scoped to avoid sender-specific drift.
		if !msg.isGroup {
			if legacy, ok := legacyChatUserScope(msg); ok {
				scopes = append(scopes, legacy)
			}
		}
	}
	scopes = append(scopes, channelScope())
	return scopes
}

func (g *Gateway) buildModelStatus(ctx context.Context, msg *incomingMessage) string {
	if g == nil || msg == nil || g.llmSelections == nil {
		return "（模型选择不可用）"
	}
	selection, matchedScope, ok, err := g.llmSelections.GetWithFallback(ctx, selectionScopes(msg)...)
	if err != nil {
		return fmt.Sprintf("读取失败：%v", err)
	}
	if !ok {
		return "当前未设置订阅模型选择；后续将使用配置默认值。"
	}

	scopeLabel := "[全局]"
	if matchedScope.ChatID != "" {
		scopeLabel = "[当前会话]"
	}

	if g.llmResolver != nil {
		if resolved, ok := g.llmResolver.Resolve(selection); ok {
			source := strings.TrimSpace(resolved.Source)
			if source == "" {
				source = strings.TrimSpace(selection.Source)
			}
			if source != "" {
				return fmt.Sprintf("当前订阅模型选择 %s：%s/%s (%s)", scopeLabel, resolved.Provider, resolved.Model, source)
			}
			return fmt.Sprintf("当前订阅模型选择 %s：%s/%s", scopeLabel, resolved.Provider, resolved.Model)
		}
	}
	if strings.TrimSpace(selection.Provider) == "" || strings.TrimSpace(selection.Model) == "" {
		return "当前订阅模型选择无效；请重新设置或清除。"
	}
	return fmt.Sprintf("当前订阅模型选择 %s：%s/%s", scopeLabel, selection.Provider, selection.Model)
}

func (g *Gateway) buildModelList(ctx context.Context, msg *incomingMessage) string {
	status := g.buildModelStatus(ctx, msg)
	catalog := g.loadModelCatalog(ctx)
	return formatModelListText(status, catalog)
}

func (g *Gateway) buildModelListReply(ctx context.Context, msg *incomingMessage) (string, string) {
	status := g.buildModelStatus(ctx, msg)
	catalog := g.loadModelCatalog(ctx)
	textReply := formatModelListText(status, catalog)
	return "text", textContent(textReply)
}

func formatModelListText(status string, catalog subscription.Catalog) string {
	lines := []string{status, "", "可用的订阅模型:"}
	if len(catalog.Providers) == 0 {
		lines = append(lines, "", "未发现可用的订阅模型。", "", modelCommandUsage())
		return strings.Join(lines, "\n")
	}

	for _, p := range catalog.Providers {
		if strings.TrimSpace(p.Provider) == "" {
			continue
		}
		header := fmt.Sprintf("- %s (%s)", p.Provider, p.Source)
		if strings.TrimSpace(p.Error) != "" {
			header = header + fmt.Sprintf(" — %s", strings.TrimSpace(p.Error))
		}
		lines = append(lines, header)
		models := p.Models
		if len(models) > 10 {
			models = models[:10]
		}
		for _, m := range models {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("  - %s", m))
		}
		if len(p.Models) > 10 {
			lines = append(lines, fmt.Sprintf("  - … (%d more)", len(p.Models)-10))
		}
	}

	lines = append(lines, "", modelCommandUsage())
	return strings.Join(lines, "\n")
}

func (g *Gateway) loadModelCatalog(ctx context.Context) subscription.Catalog {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 20 * time.Second}
	loadCreds := func() runtimeconfig.CLICredentials {
		return runtimeconfig.LoadCLICredentials()
	}
	if g != nil && g.cliCredsLoader != nil {
		loadCreds = g.cliCredsLoader
	}
	llamaResolver := func(context.Context) (subscription.LlamaServerTarget, bool) {
		return resolveLlamaServerTarget(runtimeconfig.DefaultEnvLookup)
	}
	if g != nil && g.llamaResolver != nil {
		llamaResolver = g.llamaResolver
	}

	svc := subscription.NewCatalogService(
		func() runtimeconfig.CLICredentials { return loadCreds() },
		client,
		0,
		subscription.WithLlamaServerTargetResolver(llamaResolver),
	)
	return svc.Catalog(ctx)
}

func (g *Gateway) setModelSelection(ctx context.Context, msg *incomingMessage, spec string, chatOnly bool) error {
	if g == nil || msg == nil || g.llmSelections == nil {
		return fmt.Errorf("selection store not available")
	}
	parts := strings.SplitN(strings.TrimSpace(spec), "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return fmt.Errorf("format: <provider>/<model>")
	}
	provider := strings.ToLower(strings.TrimSpace(parts[0]))
	model := strings.TrimSpace(parts[1])

	creds := runtimeconfig.LoadCLICredentials()
	cred, ok := matchSubscriptionCredential(creds, provider)
	if !ok {
		return fmt.Errorf("no subscription credential found for %q", provider)
	}

	selection := subscription.Selection{
		Mode:     "cli",
		Provider: provider,
		Model:    model,
		Source:   string(cred.Source),
	}
	scope := channelScope()
	if chatOnly {
		scope = chatScope(msg)
	}
	if err := g.llmSelections.Set(ctx, scope, selection); err != nil {
		return err
	}
	if err := markLarkOnboardingComplete(ctx, selection); err != nil {
		g.logger.Warn("Lark onboarding state update failed: %v", err)
	}
	return nil
}

func (g *Gateway) clearModelSelection(ctx context.Context, msg *incomingMessage, chatOnly bool) error {
	if g == nil || msg == nil || g.llmSelections == nil {
		return fmt.Errorf("selection store not available")
	}

	if !chatOnly {
		return g.llmSelections.Clear(ctx, channelScope())
	}

	if err := g.llmSelections.Clear(ctx, chatScope(msg)); err != nil {
		return err
	}
	if legacyScope, ok := legacyChatUserScope(msg); ok {
		if err := g.llmSelections.Clear(ctx, legacyScope); err != nil {
			return err
		}
	}
	return nil
}

func resolveLlamaServerTarget(lookup runtimeconfig.EnvLookup) (subscription.LlamaServerTarget, bool) {
	if lookup == nil {
		lookup = runtimeconfig.DefaultEnvLookup
	}

	baseURL := ""
	source := ""
	if value, ok := lookup("LLAMA_SERVER_BASE_URL"); ok {
		baseURL = strings.TrimSpace(value)
		if baseURL != "" {
			source = string(runtimeconfig.SourceEnv)
		}
	}
	if baseURL == "" {
		if host, ok := lookup("LLAMA_SERVER_HOST"); ok {
			host = strings.TrimSpace(host)
			if host != "" {
				if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
					baseURL = host
				} else {
					baseURL = "http://" + host
				}
				source = string(runtimeconfig.SourceEnv)
			}
		}
	}
	if source == "" {
		source = "llama_server"
	}
	return subscription.LlamaServerTarget{
		BaseURL: baseURL,
		Source:  source,
	}, true
}

func matchSubscriptionCredential(creds runtimeconfig.CLICredentials, provider string) (runtimeconfig.CLICredential, bool) {
	switch provider {
	case creds.Codex.Provider:
		if creds.Codex.APIKey != "" {
			return creds.Codex, true
		}
	case creds.Claude.Provider:
		if creds.Claude.APIKey != "" {
			return creds.Claude, true
		}
	case "llama_server":
		return runtimeconfig.CLICredential{
			Provider: "llama_server",
			Source:   "llama_server",
		}, true
	}
	return runtimeconfig.CLICredential{}, false
}

func markLarkOnboardingComplete(ctx context.Context, selection subscription.Selection) error {
	state := subscription.OnboardingState{
		SelectedProvider: selection.Provider,
		SelectedModel:    selection.Model,
		UsedSource:       selection.Source,
	}
	path := subscription.ResolveOnboardingStatePath(runtimeconfig.DefaultEnvLookup, nil)
	store := subscription.NewOnboardingStateStore(path)
	return store.Set(ctx, state)
}
