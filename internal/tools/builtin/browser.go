package builtin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"alex/internal/agent/ports"
)

// BrowserConfig wires dependencies for the browser tool.
type BrowserConfig struct {
	LLMFactory     ports.LLMClientFactory
	LLMProvider    string
	LLMModel       string
	LLMVisionModel string
	APIKey         string
	BaseURL        string
	VisionTool     ports.ToolExecutor
	PresetCookies  PresetCookieJar
}

type browserTool struct {
	llmFactory     ports.LLMClientFactory
	llmProvider    string
	llmModel       string
	llmVisionModel string
	apiKey         string
	baseURL        string
	visionTool     ports.ToolExecutor
	runner         playwrightRunner
	viewport       viewport
	presetCookies  PresetCookieJar
}

type viewport struct {
	width  int
	height int
}

// NewBrowser creates a DSL-driven browser automation tool backed by Playwright.
func NewBrowser(cfg BrowserConfig) ports.ToolExecutor {
	presetCookies := cfg.PresetCookies
	if len(presetCookies) == 0 {
		presetCookies = defaultPresetCookieJar()
	}
	tool := &browserTool{
		llmFactory:     cfg.LLMFactory,
		llmProvider:    strings.TrimSpace(cfg.LLMProvider),
		llmModel:       strings.TrimSpace(cfg.LLMModel),
		llmVisionModel: strings.TrimSpace(cfg.LLMVisionModel),
		apiKey:         cfg.APIKey,
		baseURL:        cfg.BaseURL,
		visionTool:     cfg.VisionTool,
		presetCookies:  presetCookies,
		viewport: viewport{
			width:  1280,
			height: 720,
		},
	}
	tool.runner = tool.runPlaywright
	return tool
}

func (t *browserTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "browser",
		Version:  "0.1.0",
		Category: "web",
		Tags: []string{
			"browser", "playwright", "vision", "automation",
		},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Produces:          []string{"text/plain"},
			ProducesArtifacts: []string{"image/png"},
		},
	}
}

func (t *browserTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "browser",
		Description: `Control a headless browser with a compact action DSL and summarize the page via VLM.

Supported statements (one per line):
- open <url>
- click <css_selector>
- scroll <up|down|left|right> <pixels>
- if exists <css_selector>
    ...actions...
  else
    ...actions...
  end

Behavior:
- Compiles the DSL into Playwright, executes it headlessly, and captures a screenshot after every action.
- On the first open, also captures basic page structure (viewport, document height, sampled nodes).
- Uses the configured vision model to summarize what is visible in the screenshots.
- Returns a concise text summary plus screenshots as attachments.

Example:
open https://example.com
if exists #login
  click #login
else
  scroll down 800
end`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"script": {
					Type:        "string",
					Description: "Action DSL to execute (one statement per line).",
				},
			},
			Required: []string{"script"},
		},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Produces:          []string{"text/plain"},
			ProducesArtifacts: []string{"image/png"},
		},
	}
}

func (t *browserTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	script, _ := call.Arguments["script"].(string)
	script = strings.TrimSpace(script)
	if script == "" {
		err := errors.New("script is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	commands, err := parseBrowserDSL(script)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	firstURL := firstOpenURL(commands)

	resolver := GetPathResolverFromContext(ctx)
	workdir := t.playwrightWorkdir(resolver)

	runResult, err := t.runner(ctx, commands, workdir, t.viewport)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	defer func() { _ = os.RemoveAll(runResult.artifactDir) }()

	attachments, steps := collectBrowserArtifacts(runResult)

	summary := t.summarize(ctx, script, steps, attachments)
	if summary == "" {
		summary = formatFallbackSummary(steps)
	}
	content := summary
	if reminder := buildBrowserReminder(firstURL); reminder != "" {
		content = summary + "\n\n" + reminder
	}

	metadata := map[string]any{
		"browser": map[string]any{
			"dsl":      script,
			"steps":    steps,
			"viewport": map[string]any{"width": t.viewport.width, "height": t.viewport.height},
			"summary":  summary,
		},
	}
	if firstURL != "" {
		metadata["url"] = firstURL
		metadata["browser"].(map[string]any)["url"] = firstURL
	}

	return &ports.ToolResult{
		CallID:      call.ID,
		Content:     content,
		Metadata:    metadata,
		Attachments: attachments,
	}, nil
}

// playwrightRunner executes compiled Playwright scripts and returns captured steps.
type playwrightRunner func(ctx context.Context, commands []browserCommand, workdir string, size viewport) (playwrightRunResult, error)

type playwrightRunResult struct {
	Steps       []playwrightStep `json:"steps"`
	artifactDir string
}

type playwrightStep struct {
	Label      string         `json:"label"`
	Screenshot string         `json:"screenshot,omitempty"`
	Error      string         `json:"error,omitempty"`
	Meta       map[string]any `json:"meta,omitempty"`
}

func (t *browserTool) runPlaywright(ctx context.Context, commands []browserCommand, workdir string, size viewport) (playwrightRunResult, error) {
	if workdir == "" {
		workdir = "."
	}

	artifactDir, err := os.MkdirTemp(workdir, "browser-shots-")
	if err != nil {
		return playwrightRunResult{}, fmt.Errorf("create artifact dir: %w", err)
	}

	script, err := t.compilePlaywrightScript(commands, artifactDir, size)
	if err != nil {
		_ = os.RemoveAll(artifactDir)
		return playwrightRunResult{}, err
	}

	scriptFile, err := os.CreateTemp(workdir, "browser-script-*.js")
	if err != nil {
		_ = os.RemoveAll(artifactDir)
		return playwrightRunResult{}, fmt.Errorf("create script file: %w", err)
	}
	if _, err := scriptFile.WriteString(script); err != nil {
		_ = os.RemoveAll(artifactDir)
		return playwrightRunResult{}, fmt.Errorf("write script: %w", err)
	}
	_ = scriptFile.Close()
	defer func() { _ = os.Remove(scriptFile.Name()) }()

	runCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(runCtx, "node", scriptFile.Name())
	cmd.Dir = workdir
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(artifactDir)
		combined := strings.TrimSpace(stderr.String())
		if combined == "" {
			combined = strings.TrimSpace(stdout.String())
		}
		if combined == "" {
			combined = err.Error()
		}
		return playwrightRunResult{}, fmt.Errorf("playwright failed: %s", combined)
	}

	output := strings.TrimSpace(stdout.String())
	var parsed struct {
		Steps []playwrightStep `json:"steps"`
	}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		_ = os.RemoveAll(artifactDir)
		return playwrightRunResult{}, fmt.Errorf("parse playwright output: %w", err)
	}

	return playwrightRunResult{Steps: parsed.Steps, artifactDir: artifactDir}, nil
}

func (t *browserTool) playwrightWorkdir(resolver *PathResolver) string {
	candidate := resolver.ResolvePath("web")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate
	}
	return resolver.ResolvePath(".")
}

func collectBrowserArtifacts(run playwrightRunResult) (map[string]ports.Attachment, []map[string]any) {
	if len(run.Steps) == 0 {
		return nil, nil
	}

	attachments := make(map[string]ports.Attachment)
	steps := make([]map[string]any, 0, len(run.Steps))

	for idx, step := range run.Steps {
		entry := map[string]any{
			"label": step.Label,
		}
		if step.Error != "" {
			entry["error"] = step.Error
		}
		if len(step.Meta) > 0 {
			entry["meta"] = step.Meta
		}

		if step.Screenshot != "" {
			name := fmt.Sprintf("browser_step_%02d.png", idx+1)
			path := filepath.Join(run.artifactDir, step.Screenshot)
			if data, err := os.ReadFile(path); err == nil {
				encoded := base64.StdEncoding.EncodeToString(data)
				attachments[name] = ports.Attachment{
					Name:        name,
					MediaType:   "image/png",
					Data:        encoded,
					URI:         fmt.Sprintf("data:image/png;base64,%s", encoded),
					Source:      "browser",
					Description: fmt.Sprintf("Screenshot after: %s", step.Label),
					Format:      "image",
				}
				entry["screenshot"] = name
			} else {
				entry["screenshot_error"] = err.Error()
			}
		}

		steps = append(steps, entry)
	}

	if len(attachments) == 0 {
		return nil, steps
	}
	return attachments, steps
}

func (t *browserTool) summarize(ctx context.Context, dsl string, steps []map[string]any, attachments map[string]ports.Attachment) string {
	if t.visionTool != nil {
		images := collectVisionImagesFromSteps(steps, attachments)
		if len(images) > 0 {
			prompt := buildBrowserVisionPrompt(dsl, steps)
			call := ports.ToolCall{
				ID: fmt.Sprintf("browser-vision-%d", time.Now().UnixNano()),
				Arguments: map[string]any{
					"images": images,
					"prompt": prompt,
				},
			}
			if res, err := t.visionTool.Execute(ctx, call); err == nil && res != nil && strings.TrimSpace(res.Content) != "" {
				return strings.TrimSpace(res.Content)
			}
		}
		return formatFallbackSummary(steps)
	}

	provider := strings.TrimSpace(t.llmProvider)
	model := strings.TrimSpace(t.llmVisionModel)
	if model == "" {
		model = strings.TrimSpace(t.llmModel)
	}
	if provider == "" || model == "" || t.llmFactory == nil {
		return ""
	}

	client, err := t.llmFactory.GetClient(provider, model, ports.LLMConfig{APIKey: t.apiKey, BaseURL: t.baseURL})
	if err != nil {
		return ""
	}

	limited := attachments
	if len(attachments) > 6 {
		limited = make(map[string]ports.Attachment)
		idx := 0
		for name, att := range attachments {
			limited[name] = att
			idx++
			if idx >= 6 {
				break
			}
		}
	}

	var builder strings.Builder
	builder.WriteString("Analyze the browser screenshots and summarize what is visible. Provide actionable highlights and note any errors.")
	if context := extractInitialPageContext(steps); context != "" {
		builder.WriteString("\nInitial page context:\n")
		builder.WriteString(context)
	}
	builder.WriteString("\nDSL:\n")
	builder.WriteString(strings.TrimSpace(dsl))
	builder.WriteString("\n\nScreenshots:\n")
	i := 1
	for _, step := range steps {
		name, _ := step["screenshot"].(string)
		label, _ := step["label"].(string)
		if name == "" || label == "" {
			continue
		}
		builder.WriteString(fmt.Sprintf("%d. %s — [%s]\n", i, label, name))
		i++
	}

	req := ports.CompletionRequest{
		Messages: []ports.Message{
			{
				Role:    "system",
				Content: "You summarize headless browser screenshots clearly and concisely.",
			},
			{
				Role:        "user",
				Content:     builder.String(),
				Attachments: limited,
			},
		},
		Temperature: 0.2,
		MaxTokens:   400,
	}

	resp, err := client.Complete(ctx, req)
	if err != nil || resp == nil {
		return ""
	}
	return strings.TrimSpace(resp.Content)
}

func buildBrowserVisionPrompt(dsl string, steps []map[string]any) string {
	var builder strings.Builder
	builder.WriteString("Analyze these browser screenshots. Describe visible layout, key elements, and any issues or errors.")
	if context := extractInitialPageContext(steps); context != "" {
		builder.WriteString("\nInitial page context:\n")
		builder.WriteString(context)
	}
	builder.WriteString("\nDSL:\n")
	builder.WriteString(strings.TrimSpace(dsl))
	return builder.String()
}

func extractInitialPageContext(steps []map[string]any) string {
	for _, step := range steps {
		label, _ := step["label"].(string)
		if !strings.HasPrefix(label, "open ") {
			continue
		}
		meta, ok := step["meta"].(map[string]any)
		if !ok || len(meta) == 0 {
			continue
		}
		return formatPageContext(meta)
	}
	return ""
}

func formatPageContext(meta map[string]any) string {
	var builder strings.Builder
	if viewport, ok := meta["viewport"].(map[string]any); ok {
		width := intFromAny(viewport["width"])
		height := intFromAny(viewport["height"])
		if width > 0 && height > 0 {
			builder.WriteString(fmt.Sprintf("- viewport: %dx%d\n", width, height))
		}
	}
	if scrollHeight := intFromAny(meta["scrollHeight"]); scrollHeight > 0 {
		builder.WriteString(fmt.Sprintf("- document height: %dpx\n", scrollHeight))
	}
	if nodes, ok := meta["nodes"].([]any); ok && len(nodes) > 0 {
		builder.WriteString(fmt.Sprintf("- sampled nodes (%d total):\n", len(nodes)))
		limit := len(nodes)
		if limit > 8 {
			limit = 8
		}
		for i := 0; i < limit; i++ {
			node, ok := nodes[i].(map[string]any)
			if !ok {
				continue
			}
			label := stringFromAny(node["label"])
			if label == "" {
				tag := stringFromAny(node["tag"])
				id := stringFromAny(node["id"])
				className := stringFromAny(node["class"])
				label = strings.TrimSpace(fmt.Sprintf("%s %s %s", tag, id, className))
			}
			line := fmt.Sprintf("  %d. %s", i+1, strings.TrimSpace(label))
			if height := intFromAny(node["height"]); height > 0 {
				line += fmt.Sprintf(" (h=%d", height)
				if top := intFromAny(node["top"]); top != 0 {
					line += fmt.Sprintf(", top=%d", top)
				}
				line += ")"
			}
			if text := stringFromAny(node["text"]); text != "" {
				line += fmt.Sprintf(" — %s", truncateText(text, 80))
			}
			builder.WriteString(line + "\n")
		}
	}
	return strings.TrimSpace(builder.String())
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		if parsed, err := strconv.Atoi(v.String()); err == nil {
			return parsed
		}
	}
	return 0
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	}
	return ""
}

func truncateText(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return strings.TrimSpace(text[:limit]) + "..."
}

func collectVisionImagesFromSteps(steps []map[string]any, attachments map[string]ports.Attachment) []string {
	if len(attachments) == 0 || len(steps) == 0 {
		return nil
	}
	images := make([]string, 0, len(steps))
	for _, step := range steps {
		name, _ := step["screenshot"].(string)
		if name == "" {
			continue
		}
		att, ok := attachments[name]
		if !ok {
			continue
		}
		if uri := strings.TrimSpace(att.URI); uri != "" {
			images = append(images, uri)
			continue
		}
		if att.Data != "" {
			mediaType := att.MediaType
			if mediaType == "" {
				mediaType = "image/png"
			}
			images = append(images, fmt.Sprintf("data:%s;base64,%s", mediaType, att.Data))
		}
	}
	return images
}

func formatFallbackSummary(steps []map[string]any) string {
	if len(steps) == 0 {
		return "No browser actions were captured."
	}
	var builder strings.Builder
	builder.WriteString("Browser actions executed:\n")
	for idx, step := range steps {
		label, _ := step["label"].(string)
		if label == "" {
			label = "(unnamed action)"
		}
		builder.WriteString(fmt.Sprintf("%d. %s", idx+1, label))
		if errText, _ := step["error"].(string); errText != "" {
			builder.WriteString(fmt.Sprintf(" (error: %s)", errText))
		}
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

type browserCommand struct {
	kind      commandKind
	value     string
	amount    int
	thenBlock []browserCommand
	elseBlock []browserCommand
}

func (c browserCommand) label() string {
	switch c.kind {
	case commandOpen:
		return fmt.Sprintf("open %s", c.value)
	case commandClick:
		return fmt.Sprintf("click %s", c.value)
	case commandScroll:
		return fmt.Sprintf("scroll %s %d", c.value, c.amount)
	case commandIfExists:
		return fmt.Sprintf("if exists %s", c.value)
	default:
		return string(c.kind)
	}
}

type commandKind string

const (
	commandOpen     commandKind = "open"
	commandClick    commandKind = "click"
	commandScroll   commandKind = "scroll"
	commandIfExists commandKind = "if"
)

type blockEndReason int

const (
	reasonEOF blockEndReason = iota
	reasonEnd
	reasonElse
)

func parseBrowserDSL(input string) ([]browserCommand, error) {
	lines := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
	trimmed := make([]string, 0, len(lines))
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		trimmed = append(trimmed, t)
	}

	idx := 0
	cmds, reason, err := parseBlock(trimmed, &idx)
	if err != nil {
		return nil, err
	}
	if reason == reasonEnd {
		return nil, fmt.Errorf("unexpected end without matching if")
	}
	if idx < len(trimmed) {
		return nil, fmt.Errorf("unable to parse DSL near line %d", idx+1)
	}
	return cmds, nil
}

func parseBlock(lines []string, idx *int) ([]browserCommand, blockEndReason, error) {
	var cmds []browserCommand
	for *idx < len(lines) {
		line := lines[*idx]
		lower := strings.ToLower(line)
		switch lower {
		case "end":
			*idx++
			return cmds, reasonEnd, nil
		case "else":
			*idx++
			return cmds, reasonElse, nil
		default:
			if strings.HasPrefix(lower, "if ") {
				condition := strings.TrimSpace(line[2:])
				selector, err := parseExistsCondition(condition)
				if err != nil {
					return nil, reasonEOF, fmt.Errorf("line %d: %w", *idx+1, err)
				}
				*idx++
				thenBlock, reason, err := parseBlock(lines, idx)
				if err != nil {
					return nil, reasonEOF, err
				}

				var elseBlock []browserCommand
				switch reason {
				case reasonElse:
					elseBlock, reason, err = parseBlock(lines, idx)
					if err != nil {
						return nil, reasonEOF, err
					}
					if reason != reasonEnd {
						return nil, reasonEOF, fmt.Errorf("line %d: missing end for if exists %s", *idx, selector)
					}
				case reasonEnd:
				case reasonEOF:
					return nil, reasonEOF, fmt.Errorf("line %d: missing end for if exists %s", *idx, selector)
				}

				cmds = append(cmds, browserCommand{kind: commandIfExists, value: selector, thenBlock: thenBlock, elseBlock: elseBlock})
			} else {
				cmd, err := parseAction(line)
				if err != nil {
					return nil, reasonEOF, fmt.Errorf("line %d: %w", *idx+1, err)
				}
				*idx++
				cmds = append(cmds, cmd)
			}
		}
	}
	return cmds, reasonEOF, nil
}

func parseExistsCondition(raw string) (string, error) {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if !strings.HasPrefix(lower, "exists ") {
		return "", fmt.Errorf("unsupported condition: %s", raw)
	}
	selector := strings.TrimSpace(raw[len("exists "):])
	selector = strings.Trim(selector, "[]")
	if selector == "" {
		return "", fmt.Errorf("selector is required for exists condition")
	}
	return selector, nil
}

func parseAction(line string) (browserCommand, error) {
	lower := strings.ToLower(line)
	switch {
	case strings.HasPrefix(lower, "open "):
		target := strings.TrimSpace(line[len("open "):])
		target = strings.Trim(target, "[]")
		if target == "" {
			return browserCommand{}, fmt.Errorf("url is required")
		}
		if parsed, err := url.Parse(target); err != nil || parsed.Scheme == "" {
			return browserCommand{}, fmt.Errorf("invalid url: %s", target)
		}
		return browserCommand{kind: commandOpen, value: target}, nil

	case strings.HasPrefix(lower, "click "):
		selector := strings.TrimSpace(line[len("click "):])
		selector = strings.Trim(selector, "[]")
		if selector == "" {
			return browserCommand{}, fmt.Errorf("selector is required for click")
		}
		return browserCommand{kind: commandClick, value: selector}, nil

	case strings.HasPrefix(lower, "scroll "):
		rest := strings.TrimSpace(line[len("scroll "):])
		fields := strings.Fields(rest)
		if len(fields) < 2 {
			return browserCommand{}, fmt.Errorf("scroll requires a direction and pixel amount, e.g. scroll down 400")
		}
		direction := strings.ToLower(fields[0])
		amount, err := strconv.Atoi(fields[1])
		if err != nil || amount <= 0 {
			return browserCommand{}, fmt.Errorf("scroll amount must be a positive integer")
		}
		switch direction {
		case "up", "down", "left", "right":
			return browserCommand{kind: commandScroll, value: direction, amount: amount}, nil
		default:
			return browserCommand{}, fmt.Errorf("unknown scroll direction: %s", direction)
		}
	default:
		return browserCommand{}, fmt.Errorf("unsupported statement: %s", line)
	}
}

func (t *browserTool) compilePlaywrightScript(commands []browserCommand, artifactDir string, size viewport) (string, error) {
	if artifactDir == "" {
		return "", fmt.Errorf("artifact directory is empty")
	}

	cookiesJSON := "{}"
	if len(t.presetCookies) > 0 {
		encoded, err := json.Marshal(t.presetCookies)
		if err != nil {
			return "", fmt.Errorf("marshal preset cookies: %w", err)
		}
		cookiesJSON = string(encoded)
	}

	var builder strings.Builder
	builder.WriteString("const { chromium } = require('playwright');\n")
	builder.WriteString("const fs = require('fs');\n")
	builder.WriteString("const path = require('path');\n")
	builder.WriteString(fmt.Sprintf("const presetCookies = %s;\n", cookiesJSON))
	builder.WriteString(fmt.Sprintf("const artifactDir = %q;\n", artifactDir))
	builder.WriteString("const userAgent = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36';\n")
	builder.WriteString("const launchArgs = [\n")
	builder.WriteString("  '--disable-blink-features=AutomationControlled',\n")
	builder.WriteString("  '--disable-infobars',\n")
	builder.WriteString(fmt.Sprintf("  '--window-size=%d,%d',\n", size.width, size.height))
	builder.WriteString("];\n")
	builder.WriteString("const baseHeaders = {\n")
	builder.WriteString("  'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8',\n")
	builder.WriteString("  'User-Agent': userAgent,\n")
	builder.WriteString("};\n")
	builder.WriteString("const contextOptions = {\n")
	builder.WriteString(fmt.Sprintf("  viewport: { width: %d, height: %d },\n", size.width, size.height))
	builder.WriteString("  userAgent,\n")
	builder.WriteString("  locale: 'zh-CN',\n")
	builder.WriteString("  timezoneId: 'Asia/Shanghai',\n")
	builder.WriteString("  bypassCSP: true,\n")
	builder.WriteString("  extraHTTPHeaders: baseHeaders,\n")
	builder.WriteString("};\n")
	builder.WriteString("fs.mkdirSync(artifactDir, { recursive: true });\n")
	builder.WriteString("async function run() {\n")
	builder.WriteString("  const browser = await chromium.launch({\n")
	builder.WriteString("    headless: true,\n")
	builder.WriteString("    args: launchArgs,\n")
	builder.WriteString("    chromiumSandbox: false,\n")
	builder.WriteString("    ignoreDefaultArgs: ['--enable-automation'],\n")
	builder.WriteString("  });\n")
	builder.WriteString("  const context = await browser.newContext(contextOptions);\n")
	builder.WriteString("  await context.addInitScript(() => {\n")
	builder.WriteString("    Object.defineProperty(navigator, 'webdriver', { get: () => undefined });\n")
	builder.WriteString("    window.chrome = window.chrome || { runtime: {} };\n")
	builder.WriteString("    Object.defineProperty(navigator, 'languages', { get: () => ['zh-CN', 'zh', 'en'] });\n")
	builder.WriteString("    Object.defineProperty(navigator, 'plugins', { get: () => [1, 2, 3, 4, 5] });\n")
	builder.WriteString("    const originalQuery = navigator.permissions && navigator.permissions.query;\n")
	builder.WriteString("    if (originalQuery) {\n")
	builder.WriteString("      navigator.permissions.query = (parameters) => parameters.name === 'notifications'\n")
	builder.WriteString("        ? Promise.resolve({ state: 'denied' })\n")
	builder.WriteString("        : originalQuery(parameters);\n")
	builder.WriteString("    }\n")
	builder.WriteString("  });\n")
	builder.WriteString("  const page = await context.newPage();\n")
	builder.WriteString("  const resolvePresetCookies = (targetUrl) => {\n")
	builder.WriteString("    try {\n")
	builder.WriteString("      const { hostname } = new URL(targetUrl);\n")
	builder.WriteString("      const host = hostname.toLowerCase();\n")
	builder.WriteString("      const segments = host.split('.').filter(Boolean);\n")
	builder.WriteString("      const domains = new Set([host]);\n")
	builder.WriteString("      for (let i = 0; i < segments.length - 1; i++) {\n")
	builder.WriteString("        const candidate = segments.slice(i).join('.');\n")
	builder.WriteString("        domains.add(candidate);\n")
	builder.WriteString("        domains.add('.' + candidate);\n")
	builder.WriteString("      }\n")
	builder.WriteString("      const resolved = [];\n")
	builder.WriteString("      for (const domain of domains) {\n")
	builder.WriteString("        const entries = presetCookies[domain];\n")
	builder.WriteString("        if (!entries || !entries.length) continue;\n")
	builder.WriteString("        for (const cookie of entries) {\n")
	builder.WriteString("          resolved.push({ ...cookie, domain: cookie.domain || domain, path: cookie.path || '/' });\n")
	builder.WriteString("        }\n")
	builder.WriteString("      }\n")
	builder.WriteString("      return resolved;\n")
	builder.WriteString("    } catch (err) {\n")
	builder.WriteString("      return [];\n")
	builder.WriteString("    }\n")
	builder.WriteString("  };\n")
	builder.WriteString("  const applyPresetCookies = async (targetUrl) => {\n")
	builder.WriteString("    const cookies = resolvePresetCookies(targetUrl);\n")
	builder.WriteString("    if (!cookies.length) return;\n")
	builder.WriteString("    await context.addCookies(cookies);\n")
	builder.WriteString("  };\n")
	builder.WriteString("  const resolveHeaders = (targetUrl) => {\n")
	builder.WriteString("    try {\n")
	builder.WriteString("      const { hostname } = new URL(targetUrl);\n")
	builder.WriteString("      const normalized = hostname.toLowerCase();\n")
	builder.WriteString("      const headers = { ...baseHeaders };\n")
	builder.WriteString("      if (normalized.endsWith('xiaohongshu.com')) {\n")
	builder.WriteString("        headers['Referer'] = 'https://www.xiaohongshu.com/';\n")
	builder.WriteString("      }\n")
	builder.WriteString("      return headers;\n")
	builder.WriteString("    } catch (err) {\n")
	builder.WriteString("      return baseHeaders;\n")
	builder.WriteString("    }\n")
	builder.WriteString("  };\n")
	builder.WriteString("  const applyHeaders = async (targetUrl) => {\n")
	builder.WriteString("    await page.setExtraHTTPHeaders(resolveHeaders(targetUrl));\n")
	builder.WriteString("  };\n")
	builder.WriteString("  page.setDefaultTimeout(15000);\n")
	builder.WriteString("  const steps = [];\n")
	builder.WriteString("  let shotCount = 0;\n")
	builder.WriteString("  const snap = async (label, meta) => {\n")
	builder.WriteString("    shotCount += 1;\n")
	builder.WriteString("    const name = `step-${shotCount}.png`;\n")
	builder.WriteString("    const filePath = path.join(artifactDir, name);\n")
	builder.WriteString("    await page.screenshot({ path: filePath, fullPage: true });\n")
	builder.WriteString("    const entry = { label, screenshot: name };\n")
	builder.WriteString("    if (meta) { entry.meta = meta; }\n")
	builder.WriteString("    steps.push(entry);\n")
	builder.WriteString("  };\n")
	builder.WriteString("  try {\n")
	emitCommands(&builder, commands, "    ", &emitContext{})
	builder.WriteString("  } catch (err) {\n")
	builder.WriteString("    steps.push({ label: 'error', error: err?.message || String(err) });\n")
	builder.WriteString("  }\n")
	builder.WriteString("  await context.close();\n")
	builder.WriteString("  await browser.close();\n")
	builder.WriteString("  console.log(JSON.stringify({ steps }));\n")
	builder.WriteString("}\nrun();\n")

	return builder.String(), nil
}

type emitContext struct {
	pageContextCaptured bool
}

func emitCommands(builder *strings.Builder, commands []browserCommand, indent string, ctx *emitContext) {
	for _, cmd := range commands {
		switch cmd.kind {
		case commandOpen:
			builder.WriteString(fmt.Sprintf("%sawait applyPresetCookies(%s);\n", indent, strconv.Quote(cmd.value)))
			builder.WriteString(fmt.Sprintf("%sawait applyHeaders(%s);\n", indent, strconv.Quote(cmd.value)))
			builder.WriteString(fmt.Sprintf("%sawait page.goto(%s, { waitUntil: 'domcontentloaded' });\n", indent, strconv.Quote(cmd.value)))
			builder.WriteString(fmt.Sprintf("%sawait page.waitForTimeout(1200);\n", indent))
			if !ctx.pageContextCaptured {
				builder.WriteString(fmt.Sprintf("%sconst pageContext = await page.evaluate(() => {\n", indent))
				builder.WriteString(fmt.Sprintf("%s  const doc = document.documentElement || {};\n", indent))
				builder.WriteString(fmt.Sprintf("%s  const body = document.body || {};\n", indent))
				builder.WriteString(fmt.Sprintf("%s  const nodes = Array.from(document.querySelectorAll('body *')).slice(0, 40).map((el) => {\n", indent))
				builder.WriteString(fmt.Sprintf("%s    const rect = el.getBoundingClientRect();\n", indent))
				builder.WriteString(fmt.Sprintf("%s    const tag = (el.tagName || '').toLowerCase();\n", indent))
				builder.WriteString(fmt.Sprintf("%s    const id = el.id ? `#${el.id}` : '';\n", indent))
				builder.WriteString(fmt.Sprintf("%s    const className = typeof el.className === 'string' ? el.className.trim() : '';\n", indent))
				builder.WriteString(fmt.Sprintf("%s    const classLabel = className ? '.' + className.split(/\\s+/).filter(Boolean).join('.') : '';\n", indent))
				builder.WriteString(fmt.Sprintf("%s    const label = `${tag}${id}${classLabel}`;\n", indent))
				builder.WriteString(fmt.Sprintf("%s    return {\n", indent))
				builder.WriteString(fmt.Sprintf("%s      tag,\n", indent))
				builder.WriteString(fmt.Sprintf("%s      id: el.id || undefined,\n", indent))
				builder.WriteString(fmt.Sprintf("%s      class: className || undefined,\n", indent))
				builder.WriteString(fmt.Sprintf("%s      label,\n", indent))
				builder.WriteString(fmt.Sprintf("%s      height: rect.height,\n", indent))
				builder.WriteString(fmt.Sprintf("%s      width: rect.width,\n", indent))
				builder.WriteString(fmt.Sprintf("%s      top: rect.top,\n", indent))
				builder.WriteString(fmt.Sprintf("%s      left: rect.left,\n", indent))
				builder.WriteString(fmt.Sprintf("%s      text: (el.innerText || '').slice(0, 120),\n", indent))
				builder.WriteString(fmt.Sprintf("%s    };\n", indent))
				builder.WriteString(fmt.Sprintf("%s  });\n", indent))
				builder.WriteString(fmt.Sprintf("%s  return {\n", indent))
				builder.WriteString(fmt.Sprintf("%s    viewport: { width: window.innerWidth, height: window.innerHeight },\n", indent))
				builder.WriteString(fmt.Sprintf("%s    scrollHeight: doc.scrollHeight || body.scrollHeight || 0,\n", indent))
				builder.WriteString(fmt.Sprintf("%s    nodes,\n", indent))
				builder.WriteString(fmt.Sprintf("%s  };\n", indent))
				builder.WriteString(fmt.Sprintf("%s});\n", indent))
				builder.WriteString(fmt.Sprintf("%sawait snap(%s, pageContext);\n", indent, strconv.Quote(cmd.label())))
				ctx.pageContextCaptured = true
			} else {
				builder.WriteString(fmt.Sprintf("%sawait snap(%s);\n", indent, strconv.Quote(cmd.label())))
			}

		case commandClick:
			builder.WriteString(fmt.Sprintf("%s{\n", indent))
			builder.WriteString(fmt.Sprintf("%s  const handle = await page.$(%s);\n", indent, strconv.Quote(cmd.value)))
			builder.WriteString(fmt.Sprintf("%s  if (handle) { await handle.click(); } else { steps.push({ label: %s, error: 'selector not found' }); }\n", indent, strconv.Quote(cmd.label())))
			builder.WriteString(fmt.Sprintf("%s  await snap(%s);\n", indent, strconv.Quote(cmd.label())))
			builder.WriteString(fmt.Sprintf("%s}\n", indent))

		case commandScroll:
			dx, dy := scrollDelta(cmd.value, cmd.amount)
			builder.WriteString(fmt.Sprintf("%sawait page.mouse.wheel(%d, %d);\n", indent, dx, dy))
			builder.WriteString(fmt.Sprintf("%sawait snap(%s);\n", indent, strconv.Quote(cmd.label())))

		case commandIfExists:
			builder.WriteString(fmt.Sprintf("%sif (await page.$(%s)) {\n", indent, strconv.Quote(cmd.value)))
			emitCommands(builder, cmd.thenBlock, indent+"  ", ctx)
			builder.WriteString(fmt.Sprintf("%s} else {\n", indent))
			emitCommands(builder, cmd.elseBlock, indent+"  ", ctx)
			builder.WriteString(fmt.Sprintf("%s}\n", indent))
		}
	}
}

func scrollDelta(direction string, amount int) (int, int) {
	switch direction {
	case "up":
		return 0, -amount
	case "down":
		return 0, amount
	case "left":
		return -amount, 0
	case "right":
		return amount, 0
	default:
		return 0, 0
	}
}

func buildBrowserReminder(openURL string) string {
	if strings.TrimSpace(openURL) == "" {
		return ""
	}
	return fmt.Sprintf("<system-reminder>The browser page stays live until the task ends. Reuse the existing page at %s instead of reopening.</system-reminder>", strings.TrimSpace(openURL))
}

func firstOpenURL(commands []browserCommand) string {
	for _, cmd := range commands {
		switch cmd.kind {
		case commandOpen:
			return cmd.value
		case commandIfExists:
			if url := firstOpenURL(cmd.thenBlock); url != "" {
				return url
			}
			if url := firstOpenURL(cmd.elseBlock); url != "" {
				return url
			}
		}
	}
	return ""
}
