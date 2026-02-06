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
	larkcards "alex/internal/infra/lark/cards"
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

	scope := subscription.SelectionScope{Channel: "lark", ChatID: msg.chatID, UserID: msg.senderID}
	selection, ok, err := g.llmSelections.Get(ctx, scope)
	if err != nil {
		g.logger.Warn("Lark LLM selection load failed: %v", err)
		return ctx
	}
	if !ok {
		return ctx
	}

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

	var reply string
	replyMsgType := "text"
	replyContent := ""
	switch sub {
	case "", "list", "ls":
		replyMsgType, replyContent = g.buildModelListReply(execCtx, msg)
	case "use", "select", "set":
		if len(fields) < 3 {
			reply = modelCommandUsage()
			break
		}
		if err := g.setModelSelection(execCtx, msg, strings.TrimSpace(fields[2])); err != nil {
			reply = fmt.Sprintf("设置失败：%v\n\n%s", err, modelCommandUsage())
			break
		}
		reply = g.buildModelStatus(execCtx, msg)
	case "clear", "reset":
		if err := g.clearModelSelection(execCtx, msg); err != nil {
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

	if replyContent == "" {
		replyContent = textContent(reply)
	}
	g.dispatch(execCtx, msg.chatID, replyTarget(msg.messageID, true), replyMsgType, replyContent)
}

func modelCommandUsage() string {
	return strings.TrimSpace(`
Model command usage:
  /model                         List available subscription models
  /model use <provider>/<model>  Select a subscription model (pinned)
  /model status                  Show current pinned selection
  /model clear                   Clear pinned selection, revert to defaults

Examples:
  /model use codex/gpt-5.2-codex
  /model use anthropic/claude-sonnet-4-20250514
  /model use ollama/llama3:latest
  /model use llama_server/local-model
`)
}

func (g *Gateway) modelSelectionScope(msg *incomingMessage) subscription.SelectionScope {
	return subscription.SelectionScope{Channel: "lark", ChatID: strings.TrimSpace(msg.chatID), UserID: strings.TrimSpace(msg.senderID)}
}

func (g *Gateway) buildModelStatus(ctx context.Context, msg *incomingMessage) string {
	if g == nil || msg == nil || g.llmSelections == nil {
		return "（模型选择不可用）"
	}
	scope := g.modelSelectionScope(msg)
	selection, ok, err := g.llmSelections.Get(ctx, scope)
	if err != nil {
		return fmt.Sprintf("读取失败：%v", err)
	}
	if !ok {
		return "当前未设置订阅模型选择；后续将使用配置默认值。"
	}
	if g.llmResolver != nil {
		if resolved, ok := g.llmResolver.Resolve(selection); ok {
			source := strings.TrimSpace(resolved.Source)
			if source == "" {
				source = strings.TrimSpace(selection.Source)
			}
			if source != "" {
				return fmt.Sprintf("当前订阅模型选择：%s/%s (%s)", resolved.Provider, resolved.Model, source)
			}
			return fmt.Sprintf("当前订阅模型选择：%s/%s", resolved.Provider, resolved.Model)
		}
	}
	if strings.TrimSpace(selection.Provider) == "" || strings.TrimSpace(selection.Model) == "" {
		return "当前订阅模型选择无效；请重新设置或清除。"
	}
	return fmt.Sprintf("当前订阅模型选择：%s/%s", selection.Provider, selection.Model)
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
	if g == nil || !g.cfg.CardsEnabled {
		return "text", textContent(textReply)
	}
	card, err := buildModelSelectionCard(status, catalog)
	if err != nil {
		g.logger.Warn("Lark model list card build failed: %v", err)
		return "text", textContent(textReply)
	}
	return "interactive", card
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

func buildModelSelectionCard(status string, catalog subscription.Catalog) (string, error) {
	card := larkcards.NewCard(larkcards.CardConfig{
		Title:         "订阅模型选择",
		TitleColor:    "blue",
		EnableForward: false,
	})
	if strings.TrimSpace(status) != "" {
		card.AddMarkdownSection(truncateCardText(status, maxCardReplyChars))
	}

	hasButton := false
	for _, provider := range catalog.Providers {
		name := strings.TrimSpace(provider.Provider)
		if name == "" || len(provider.Models) == 0 {
			continue
		}
		source := strings.TrimSpace(provider.Source)
		if source != "" {
			card.AddDivider()
			card.AddMarkdownSection(fmt.Sprintf("**%s** (%s)", name, source))
		} else {
			card.AddDivider()
			card.AddMarkdownSection(fmt.Sprintf("**%s**", name))
		}

		models := provider.Models
		if len(models) > 10 {
			models = models[:10]
		}
		row := make([]larkcards.Button, 0, 3)
		for _, rawModel := range models {
			model := strings.TrimSpace(rawModel)
			if model == "" {
				continue
			}
			label := model
			if len(label) > 32 {
				label = label[:29] + "..."
			}
			spec := name + "/" + model
			row = append(row, larkcards.NewButton(label, "model_use").
				WithValue("text", "/model use "+spec).
				WithValue("model_spec", spec))
			hasButton = true
			if len(row) == 3 {
				card.AddActionButtons(row...)
				row = make([]larkcards.Button, 0, 3)
			}
		}
		if len(row) > 0 {
			card.AddActionButtons(row...)
		}
	}

	if !hasButton {
		return "", fmt.Errorf("no selectable models")
	}
	card.AddNote("点击模型按钮即可切换当前会话订阅模型。")
	return card.Build()
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

func (g *Gateway) setModelSelection(ctx context.Context, msg *incomingMessage, spec string) error {
	if g == nil || msg == nil || g.llmSelections == nil {
		return fmt.Errorf("selection store not available")
	}
	parts := strings.SplitN(strings.TrimSpace(spec), "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return fmt.Errorf("format: <provider>/<model>")
	}
	provider := strings.ToLower(strings.TrimSpace(parts[0]))
	model := strings.TrimSpace(parts[1])

	creds := runtimeconfig.CLICredentials{}
	if provider != "ollama" {
		creds = runtimeconfig.LoadCLICredentials()
	}
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
	return g.llmSelections.Set(ctx, g.modelSelectionScope(msg), selection)
}

func (g *Gateway) clearModelSelection(ctx context.Context, msg *incomingMessage) error {
	if g == nil || msg == nil || g.llmSelections == nil {
		return fmt.Errorf("selection store not available")
	}
	return g.llmSelections.Clear(ctx, g.modelSelectionScope(msg))
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
	case "ollama":
		return runtimeconfig.CLICredential{
			Provider: "ollama",
			Source:   "ollama",
		}, true
	case "llama_server":
		return runtimeconfig.CLICredential{
			Provider: "llama_server",
			Source:   "llama_server",
		}, true
	}
	return runtimeconfig.CLICredential{}, false
}
