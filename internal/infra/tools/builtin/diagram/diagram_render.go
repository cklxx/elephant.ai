package diagram

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/sandbox"
	"alex/internal/infra/tools/builtin/browser"
	"alex/internal/infra/tools/builtin/shared"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

const (
	toolName        = "diagram_render"
	mermaidVersion  = "10.9.0"
	defaultWidth    = 1200
	defaultHeight   = 800
	defaultScale    = 1.0
	defaultPadding  = 32
	defaultTimeout  = 20 * time.Second
	captureSelector = "#capture"
	svgSelector     = "#diagram svg"
)

type diagramRender struct {
	shared.BaseTool

	mode          string
	localConfig   LocalConfig
	browserMgr    *browser.Manager
	sandboxCfg    SandboxConfig
	sandboxClient *sandbox.Client
}

type LocalConfig struct {
	ChromePath  string
	Headless    bool
	UserDataDir string
	Timeout     time.Duration
}

type SandboxConfig struct {
	BaseURL          string
	Timeout          time.Duration
	MaxResponseBytes int
}

type diagramRenderArgs struct {
	Format  string
	Source  string
	Items   []IconBlockItem
	Title   string
	Theme   string
	Output  string
	Name    string
	Width   int
	Height  int
	Scale   float64
	Padding int
}

func NewDiagramRenderLocal(cfg LocalConfig, mgr *browser.Manager) tools.ToolExecutor {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	cfg.Timeout = timeout
	return &diagramRender{
		BaseTool: shared.NewBaseTool(
			diagramRenderDefinition(),
			ports.ToolMetadata{
				Name:     toolName,
				Version:  "1.0.0",
				Category: "media",
				Tags:     []string{"diagram", "mermaid", "render", "png", "svg"},
				MaterialCapabilities: ports.ToolMaterialCapabilities{
					ProducesArtifacts: []string{"image/png", "image/svg+xml"},
				},
			},
		),
		mode:        "local",
		localConfig: cfg,
		browserMgr:  mgr,
	}
}

func NewDiagramRenderSandbox(cfg SandboxConfig) tools.ToolExecutor {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	cfg.Timeout = timeout
	client := sandbox.NewClient(sandbox.Config{
		BaseURL:          cfg.BaseURL,
		Timeout:          timeout,
		MaxResponseBytes: cfg.MaxResponseBytes,
	})
	return &diagramRender{
		BaseTool: shared.NewBaseTool(
			diagramRenderDefinition(),
			ports.ToolMetadata{
				Name:     toolName,
				Version:  "1.0.0",
				Category: "media",
				Tags:     []string{"diagram", "mermaid", "render", "png", "svg", "sandbox"},
				MaterialCapabilities: ports.ToolMaterialCapabilities{
					ProducesArtifacts: []string{"image/png", "image/svg+xml"},
				},
			},
		),
		mode:          "sandbox",
		sandboxCfg:    cfg,
		sandboxClient: client,
	}
}

func diagramRenderDefinition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        toolName,
		Description: "Render Mermaid diagrams or icon blocks to a beautiful PNG (optionally SVG). Offline Mermaid rendering uses a vendored mermaid.min.js.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"format": {Type: "string", Description: `Render format: "mermaid" or "icon_blocks" (default: mermaid)`, Enum: []any{"mermaid", "icon_blocks"}},
				"source": {Type: "string", Description: "Mermaid source text. Required when format=mermaid; ```mermaid fences are accepted."},
				"items":  {Type: "array", Description: "Icon block items. Required when format=icon_blocks.", Items: &ports.Property{Type: "object"}},
				"title":  {Type: "string", Description: "Optional title for icon blocks."},
				"theme":  {Type: "string", Description: `Theme: "light" or "dark" (default: light)`, Enum: []any{"light", "dark"}},
				"output": {Type: "string", Description: `Output: "png", "svg", or "png_svg" (default: png)`, Enum: []any{"png", "svg", "png_svg"}},
				"name":   {Type: "string", Description: "Output name base or filename. Defaults to diagram (mermaid) or icons (icon_blocks)."},
				"width":  {Type: "integer", Description: fmt.Sprintf("Viewport width in px (default: %d)", defaultWidth)},
				"height": {Type: "integer", Description: fmt.Sprintf("Viewport height in px (default: %d)", defaultHeight)},
				"scale":  {Type: "number", Description: fmt.Sprintf("Device scale factor (default: %.1f)", defaultScale)},
				"padding": {
					Type:        "integer",
					Description: fmt.Sprintf("Outer padding in px (default: %d)", defaultPadding),
				},
			},
		},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			ProducesArtifacts: []string{"image/png", "image/svg+xml"},
		},
	}
}

func (t *diagramRender) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	args, err := parseDiagramRenderArgs(call.Arguments)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	started := time.Now()
	rendered, status, diagErr, err := t.render(ctx, call, args)
	elapsed := time.Since(started)

	metadata := map[string]any{
		"format":       args.Format,
		"theme":        args.Theme,
		"output":       args.Output,
		"viewport":     map[string]any{"width": args.Width, "height": args.Height},
		"scale":        args.Scale,
		"padding":      args.Padding,
		"render_ms":    elapsed.Milliseconds(),
		"diagram":      map[string]any{"status": status, "error": diagErr},
		"tool_runtime": t.mode,
	}
	if args.Format == "mermaid" {
		metadata["mermaid_version"] = mermaidVersion
	}

	if err != nil {
		content := err.Error()
		if diagErr != "" {
			content = fmt.Sprintf("%s (diagram_error=%s)", content, diagErr)
		}
		return &ports.ToolResult{CallID: call.ID, Content: content, Error: err, Metadata: metadata}, nil
	}

	attachments := make(map[string]ports.Attachment, 2)
	if len(rendered.PNG) > 0 {
		name := rendered.PNGName
		attachments[name] = ports.Attachment{
			Name:      name,
			MediaType: "image/png",
			Data:      base64.StdEncoding.EncodeToString(rendered.PNG),
			Source:    call.Name,
		}
	}
	if rendered.SVGName != "" && rendered.SVG != "" {
		name := rendered.SVGName
		attachments[name] = ports.Attachment{
			Name:      name,
			MediaType: "image/svg+xml",
			Data:      base64.StdEncoding.EncodeToString([]byte(rendered.SVG)),
			Source:    call.Name,
		}
	}

	content := fmt.Sprintf("Rendered %s as [%s].", args.Format, rendered.PNGName)
	if rendered.SVGName != "" {
		content = fmt.Sprintf("%s Also produced [%s].", content, rendered.SVGName)
	}

	return &ports.ToolResult{
		CallID:      call.ID,
		Content:     content,
		Metadata:    metadata,
		Attachments: attachments,
	}, nil
}

type renderOutput struct {
	PNG     []byte
	PNGName string
	SVG     string
	SVGName string
}

func (t *diagramRender) render(ctx context.Context, call ports.ToolCall, args diagramRenderArgs) (renderOutput, string, string, error) {
	timeout := defaultTimeout
	switch t.mode {
	case "local":
		timeout = t.localConfig.Timeout
	case "sandbox":
		timeout = t.sandboxCfg.Timeout
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	renderCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	outputBase := sanitizeOutputBase(args.Name, args.Format)
	pngName := outputBase + ".png"
	svgName := outputBase + ".svg"

	wantPNG := args.Output == "png" || args.Output == "png_svg"
	wantSVG := args.Output == "svg" || args.Output == "png_svg"

	if args.Format == "icon_blocks" && wantSVG {
		return renderOutput{}, "", "", errors.New("output svg is not supported for format=icon_blocks")
	}

	var htmlDoc string
	var err error
	switch args.Format {
	case "mermaid":
		normalized, hasInitDirective, err := normalizeMermaidSource(args.Source)
		if err != nil {
			return renderOutput{}, "", "", err
		}
		htmlDoc, err = buildMermaidHTML(mermaidHTMLParams{
			Source:           normalized,
			Theme:            args.Theme,
			Padding:          args.Padding,
			HasInitDirective: hasInitDirective,
		})
	case "icon_blocks":
		htmlDoc, err = buildIconBlocksHTML(iconBlocksHTMLParams{
			Title:   args.Title,
			Items:   args.Items,
			Theme:   args.Theme,
			Padding: args.Padding,
		})
	default:
		return renderOutput{}, "", "", fmt.Errorf("unsupported format %q", args.Format)
	}
	if err != nil {
		return renderOutput{}, "", "", err
	}

	chromeCtx, closeFn, err := t.newChromeContext(renderCtx, call)
	if err != nil {
		return renderOutput{}, "", "", err
	}
	defer closeFn()

	width := args.Width
	height := args.Height
	scale := args.Scale
	if scale <= 0 {
		scale = defaultScale
	}

	var status string
	var diagErr string

	var png []byte
	var svg string

	tasks := []chromedp.Action{
		emulation.SetDeviceMetricsOverride(int64(width), int64(height), scale, false),
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			frameTree, err := page.GetFrameTree().Do(ctx)
			if err != nil {
				return err
			}
			return page.SetDocumentContent(frameTree.Frame.ID, htmlDoc).Do(ctx)
		}),
		chromedp.WaitVisible(captureSelector, chromedp.ByQuery),
	}

	if err := chromedp.Run(chromeCtx, tasks...); err != nil {
		return renderOutput{}, "", "", err
	}

	statusTimeout := timeout - 2*time.Second
	if statusTimeout < 3*time.Second {
		statusTimeout = 3 * time.Second
	}
	status, diagErr, err = waitForDiagramStatus(chromeCtx, statusTimeout)
	if err != nil {
		return renderOutput{}, status, diagErr, err
	}

	if wantSVG && args.Format == "mermaid" {
		if err := chromedp.Run(chromeCtx, chromedp.OuterHTML(svgSelector, &svg, chromedp.ByQuery)); err != nil {
			return renderOutput{}, status, diagErr, err
		}
	}
	if wantPNG {
		if err := chromedp.Run(chromeCtx, chromedp.Screenshot(captureSelector, &png, chromedp.NodeVisible, chromedp.ByQuery)); err != nil {
			return renderOutput{}, status, diagErr, err
		}
	}

	out := renderOutput{}
	if wantPNG {
		out.PNG = png
		out.PNGName = pngName
	}
	if wantSVG && args.Format == "mermaid" {
		out.SVG = svg
		out.SVGName = svgName
	}
	if !wantPNG && wantSVG {
		// Ensure content has at least one attachment name for summary.
		out.PNGName = svgName
	}
	return out, status, diagErr, nil
}

func sanitizeOutputBase(name string, format string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		switch format {
		case "icon_blocks":
			return "icons"
		default:
			return "diagram"
		}
	}
	trimmed = filepath.Base(trimmed)
	ext := strings.ToLower(filepath.Ext(trimmed))
	switch ext {
	case ".png", ".svg":
		trimmed = strings.TrimSuffix(trimmed, ext)
	}
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		switch format {
		case "icon_blocks":
			return "icons"
		default:
			return "diagram"
		}
	}
	if sanitized := sanitizeFilenameComponent(trimmed); sanitized != "" {
		return sanitized
	}
	switch format {
	case "icon_blocks":
		return "icons"
	default:
		return "diagram"
	}
}

func sanitizeFilenameComponent(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	return out
}

func parseDiagramRenderArgs(args map[string]any) (diagramRenderArgs, error) {
	format := strings.ToLower(strings.TrimSpace(shared.StringArg(args, "format")))
	if format == "" {
		format = "mermaid"
	}
	theme := normalizeTheme(shared.StringArg(args, "theme"))
	output := strings.ToLower(strings.TrimSpace(shared.StringArg(args, "output")))
	if output == "" {
		output = "png"
	}
	switch output {
	case "png", "svg", "png_svg":
	default:
		return diagramRenderArgs{}, fmt.Errorf("unsupported output %q", output)
	}

	width := defaultWidth
	if v, ok := shared.IntArg(args, "width"); ok && v > 0 {
		width = clampInt(v, 600, 2400)
	}
	height := defaultHeight
	if v, ok := shared.IntArg(args, "height"); ok && v > 0 {
		height = clampInt(v, 400, 2400)
	}
	scale := defaultScale
	if v, ok := shared.FloatArg(args, "scale"); ok && v > 0 {
		if v > 2.0 {
			v = 2.0
		}
		scale = v
	}
	padding := defaultPadding
	if v, ok := shared.IntArg(args, "padding"); ok {
		padding = clampInt(v, 0, 128)
	}

	name := strings.TrimSpace(shared.StringArg(args, "name"))

	switch format {
	case "mermaid":
		source := shared.StringArg(args, "source")
		if strings.TrimSpace(source) == "" {
			return diagramRenderArgs{}, errors.New("source is required for format=mermaid")
		}
		return diagramRenderArgs{
			Format:  format,
			Source:  source,
			Theme:   theme,
			Output:  output,
			Name:    name,
			Width:   width,
			Height:  height,
			Scale:   scale,
			Padding: padding,
		}, nil

	case "icon_blocks":
		items, err := parseIconBlockItems(args["items"])
		if err != nil {
			return diagramRenderArgs{}, err
		}
		if len(items) == 0 {
			return diagramRenderArgs{}, errors.New("items is required for format=icon_blocks")
		}
		title := shared.StringArg(args, "title")
		return diagramRenderArgs{
			Format:  format,
			Items:   items,
			Title:   title,
			Theme:   theme,
			Output:  output,
			Name:    name,
			Width:   width,
			Height:  height,
			Scale:   scale,
			Padding: padding,
		}, nil

	default:
		return diagramRenderArgs{}, fmt.Errorf("unsupported format %q", format)
	}
}

func parseIconBlockItems(raw any) ([]IconBlockItem, error) {
	if raw == nil {
		return nil, nil
	}
	list, ok := raw.([]any)
	if !ok {
		if typed, ok := raw.([]map[string]any); ok {
			list = make([]any, 0, len(typed))
			for _, item := range typed {
				list = append(list, item)
			}
		} else {
			return nil, errors.New("items must be an array")
		}
	}

	items := make([]IconBlockItem, 0, len(list))
	for i, entry := range list {
		m, ok := entry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("items[%d] must be an object", i)
		}
		icon := strings.TrimSpace(fmt.Sprint(coalesceMapValue(m, "icon")))
		title := strings.TrimSpace(fmt.Sprint(coalesceMapValue(m, "title")))
		if icon == "" {
			return nil, fmt.Errorf("items[%d].icon is required", i)
		}
		if title == "" {
			return nil, fmt.Errorf("items[%d].title is required", i)
		}
		desc := strings.TrimSpace(fmt.Sprint(coalesceMapValue(m, "description")))
		items = append(items, IconBlockItem{Icon: icon, Title: title, Description: desc})
	}
	return items, nil
}

func coalesceMapValue(m map[string]any, key string) any {
	if m == nil {
		return ""
	}
	value, ok := m[key]
	if !ok || value == nil {
		return ""
	}
	return value
}

func waitForDiagramStatus(ctx context.Context, timeout time.Duration) (status string, diagErr string, err error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	deadline := time.Now().Add(timeout)

	for {
		select {
		case <-ctx.Done():
			return status, diagErr, ctx.Err()
		default:
		}

		var s string
		var e string
		_ = chromedp.Run(ctx,
			chromedp.Evaluate(`document.documentElement.dataset.diagramStatus || ""`, &s),
			chromedp.Evaluate(`document.documentElement.dataset.diagramError || ""`, &e),
		)
		status = strings.TrimSpace(s)
		diagErr = strings.TrimSpace(e)

		switch status {
		case "ready":
			return status, diagErr, nil
		case "error":
			if diagErr == "" {
				diagErr = "unknown error"
			}
			return status, diagErr, fmt.Errorf("diagram render failed: %s", truncate(diagErr, 200))
		}

		if time.Now().After(deadline) {
			return status, diagErr, fmt.Errorf("diagram render timed out (status=%q)", status)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func truncate(value string, max int) string {
	if max <= 0 {
		return ""
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) <= max {
		return trimmed
	}
	return string(runes[:max]) + "..."
}
