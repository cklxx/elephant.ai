package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/sandbox"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

type sandboxBrowserDOMTool struct {
	client *sandbox.Client
}

type sandboxDOMStep struct {
	Action     string
	Selector   string
	URL        string
	Text       string
	Key        string
	Attribute  string
	Expression string
	Name       string
	State      string
	TimeoutMs  int
}

type sandboxDOMStepResult struct {
	Action   string `json:"action"`
	Name     string `json:"name,omitempty"`
	Selector string `json:"selector,omitempty"`
	OK       bool   `json:"ok"`
	Value    any    `json:"value,omitempty"`
	Error    string `json:"error,omitempty"`
}

type sandboxDOMElement struct {
	Tag       string `json:"tag"`
	Text      string `json:"text"`
	AriaLabel string `json:"ariaLabel"`
	Name      string `json:"name"`
	ID        string `json:"id"`
	Role      string `json:"role"`
	Type      string `json:"type"`
	Selector  string `json:"selector"`
}

const (
	sandboxDOMToolName        = "sandbox_browser_dom"
	sandboxDOMDefaultMaxHints = 6
	sandboxDOMDefaultMaxElems = 24
	sandboxDOMMaxElemsCap     = 60
)

func NewSandboxBrowserDOM(cfg SandboxConfig) ports.ToolExecutor {
	return &sandboxBrowserDOMTool{client: newSandboxClient(cfg)}
}

func (t *sandboxBrowserDOMTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     sandboxDOMToolName,
		Version:  "0.1.0",
		Category: "web",
		Tags:     []string{"sandbox", "browser", "dom", "automation"},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Produces: []string{"text/plain", "application/json"},
		},
	}
}

func (t *sandboxBrowserDOMTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: sandboxDOMToolName,
		Description: `Use the sandbox browser via CDP for DOM-level automation (Playwright-like).

Provide ordered steps using CSS selectors. Useful for click/fill/wait/query/evaluate without relying on screenshots.`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"steps": {
					Type:        "array",
					Description: "Ordered list of DOM actions (goto, click, fill, type, press, wait_for, get_text, get_html, get_attribute, evaluate).",
					Items:       &ports.Property{Type: "object"},
				},
				"continue_on_error": {
					Type:        "boolean",
					Description: "Continue executing steps after a failure (default false).",
				},
				"inspect": {
					Type:        "object",
					Description: "Optional DOM inspection settings.",
				},
				"return_page": {
					Type:        "boolean",
					Description: "Include current page title and URL in the result (default true).",
				},
			},
			Required: []string{"steps"},
		},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Produces: []string{"text/plain", "application/json"},
		},
	}
}

func (t *sandboxBrowserDOMTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	steps, err := parseSandboxDOMSteps(call.Arguments)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	info, err := fetchSandboxBrowserInfo(ctx, t.client, call.SessionID)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	cdpURL := strings.TrimSpace(info.CDPURL)
	if cdpURL == "" {
		err := errors.New("sandbox browser CDP URL is missing")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(ctx, cdpURL)
	defer cancelAlloc()

	chromeCtx, cancelChrome, targetID := attachSandboxTarget(allocCtx)
	defer cancelChrome()

	if targetID != "" {
		_ = chromedp.Run(chromeCtx, target.ActivateTarget(targetID))
	}

	continueOnError := readBoolWithDefault(call.Arguments, "continue_on_error", false)

	results := make([]sandboxDOMStepResult, 0, len(steps))
	var firstErr error

	for _, step := range steps {
		result := sandboxDOMStepResult{
			Action:   step.Action,
			Name:     step.Name,
			Selector: step.Selector,
		}

		value, execErr := runSandboxDOMStep(chromeCtx, step)
		if execErr != nil {
			result.OK = false
			result.Error = execErr.Error()
			if firstErr == nil {
				firstErr = execErr
			}
		} else {
			result.OK = true
			if value != nil {
				result.Value = value
			}
		}

		results = append(results, result)

		if execErr != nil && !continueOnError {
			break
		}
	}

	pageInfo := map[string]any{}
	if readBoolWithDefault(call.Arguments, "return_page", true) {
		if url, title, err := fetchSandboxDOMPageInfo(chromeCtx); err == nil {
			if url != "" {
				pageInfo["url"] = url
			}
			if title != "" {
				pageInfo["title"] = title
			}
		}
	}

	inspectElements := []sandboxDOMElement{}
	inspectConfig := parseSandboxDOMInspect(call.Arguments)
	if inspectConfig != nil && inspectConfig.IncludeInteractive {
		if elements, err := inspectSandboxDOM(chromeCtx, inspectConfig.MaxElements); err == nil {
			inspectElements = elements
		}
	}

	content := buildSandboxDOMSummary(results, pageInfo, inspectElements, firstErr)

	metadata := map[string]any{
		"sandbox_browser_dom": map[string]any{
			"steps":    steps,
			"results":  results,
			"page":     pageInfo,
			"elements": inspectElements,
		},
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  content,
		Metadata: metadata,
	}, nil
}

func fetchSandboxBrowserInfo(ctx context.Context, client *sandbox.Client, sessionID string) (*sandbox.BrowserInfo, error) {
	var response sandbox.Response[sandbox.BrowserInfo]
	if err := client.DoJSON(ctx, httpMethodGet, "/v1/browser/info", nil, sessionID, &response); err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, fmt.Errorf("sandbox browser info failed: %s", response.Message)
	}
	if response.Data == nil {
		return nil, errors.New("sandbox browser info returned empty data")
	}
	return response.Data, nil
}

func attachSandboxTarget(ctx context.Context) (context.Context, context.CancelFunc, target.ID) {
	targetID, err := pickSandboxTarget(ctx)
	if err != nil || targetID == "" {
		chromeCtx, cancel := chromedp.NewContext(ctx)
		return chromeCtx, cancel, ""
	}
	chromeCtx, cancel := chromedp.NewContext(ctx, chromedp.WithTargetID(targetID))
	return chromeCtx, cancel, targetID
}

func pickSandboxTarget(ctx context.Context) (target.ID, error) {
	targets, err := chromedp.Targets(ctx)
	if err != nil {
		return "", err
	}

	var fallback *target.Info
	for _, info := range targets {
		if info == nil || info.Type != "page" {
			continue
		}
		if info.URL != "" && info.URL != "about:blank" && !strings.HasPrefix(info.URL, "chrome://") {
			return info.TargetID, nil
		}
		if fallback == nil {
			fallback = info
		}
	}
	if fallback != nil {
		return fallback.TargetID, nil
	}
	return "", errors.New("no page targets available")
}

func parseSandboxDOMSteps(args map[string]any) ([]sandboxDOMStep, error) {
	rawSteps, ok := args["steps"].([]any)
	if !ok || len(rawSteps) == 0 {
		return nil, errors.New("steps is required")
	}

	steps := make([]sandboxDOMStep, 0, len(rawSteps))
	for idx, item := range rawSteps {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("steps[%d] must be an object", idx)
		}

		action := strings.ToLower(strings.TrimSpace(stringFromArgs(m, "action")))
		if action == "" {
			action = strings.ToLower(strings.TrimSpace(stringFromArgs(m, "action_type")))
		}
		if action == "" {
			return nil, fmt.Errorf("steps[%d].action is required", idx)
		}

		text := strings.TrimSpace(stringFromArgs(m, "text"))
		if text == "" {
			text = strings.TrimSpace(stringFromArgs(m, "value"))
		}

		timeoutMs, _ := readInt(m, "timeout_ms")
		if timeoutMs < 0 {
			timeoutMs = 0
		}

		step := sandboxDOMStep{
			Action:     action,
			Selector:   strings.TrimSpace(stringFromArgs(m, "selector")),
			URL:        strings.TrimSpace(stringFromArgs(m, "url")),
			Text:       text,
			Key:        strings.TrimSpace(stringFromArgs(m, "key")),
			Attribute:  strings.TrimSpace(stringFromArgs(m, "attribute")),
			Expression: strings.TrimSpace(stringFromArgs(m, "expression")),
			Name:       strings.TrimSpace(stringFromArgs(m, "name")),
			State:      strings.TrimSpace(stringFromArgs(m, "state")),
			TimeoutMs:  timeoutMs,
		}

		steps = append(steps, step)
	}

	return steps, nil
}

func runSandboxDOMStep(ctx context.Context, step sandboxDOMStep) (any, error) {
	action := step.Action
	switch action {
	case "goto", "navigate":
		if step.URL == "" {
			return nil, errors.New("goto requires url")
		}
		return nil, runDOMTasks(ctx, step.TimeoutMs,
			chromedp.Navigate(step.URL),
			chromedp.WaitReady("body", chromedp.ByQuery),
		)
	case "click":
		if step.Selector == "" {
			return nil, errors.New("click requires selector")
		}
		return nil, runDOMTasks(ctx, step.TimeoutMs,
			chromedp.Click(step.Selector, chromedp.NodeVisible),
		)
	case "hover":
		if step.Selector == "" {
			return nil, errors.New("hover requires selector")
		}
		script := fmt.Sprintf(`(() => {
  const el = document.querySelector(%q);
  if (!el) return false;
  el.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
  return true;
})()`, step.Selector)
		var ok bool
		err := runDOMTasks(ctx, step.TimeoutMs, chromedp.Evaluate(script, &ok))
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("hover target not found")
		}
		return nil, nil
	case "focus":
		if step.Selector == "" {
			return nil, errors.New("focus requires selector")
		}
		return nil, runDOMTasks(ctx, step.TimeoutMs,
			chromedp.Focus(step.Selector),
		)
	case "fill":
		if step.Selector == "" {
			return nil, errors.New("fill requires selector")
		}
		return nil, runDOMTasks(ctx, step.TimeoutMs,
			chromedp.Focus(step.Selector),
			chromedp.SetValue(step.Selector, ""),
			chromedp.SendKeys(step.Selector, step.Text),
		)
	case "type":
		if step.Selector == "" {
			return nil, errors.New("type requires selector")
		}
		return nil, runDOMTasks(ctx, step.TimeoutMs,
			chromedp.SendKeys(step.Selector, step.Text),
		)
	case "press":
		if step.Key == "" {
			return nil, errors.New("press requires key")
		}
		if step.Selector != "" {
			return nil, runDOMTasks(ctx, step.TimeoutMs,
				chromedp.SendKeys(step.Selector, step.Key),
			)
		}
		return nil, runDOMTasks(ctx, step.TimeoutMs, chromedp.KeyEvent(step.Key))
	case "select":
		if step.Selector == "" {
			return nil, errors.New("select requires selector")
		}
		return nil, runDOMTasks(ctx, step.TimeoutMs,
			chromedp.SetValue(step.Selector, step.Text),
		)
	case "wait_for":
		if step.Selector == "" {
			return nil, errors.New("wait_for requires selector")
		}
		state := strings.ToLower(strings.TrimSpace(step.State))
		switch state {
		case "", "visible":
			return nil, runDOMTasks(ctx, step.TimeoutMs,
				chromedp.WaitVisible(step.Selector, chromedp.ByQuery),
			)
		case "hidden":
			return nil, runDOMTasks(ctx, step.TimeoutMs,
				chromedp.WaitNotVisible(step.Selector, chromedp.ByQuery),
			)
		case "attached", "ready":
			return nil, runDOMTasks(ctx, step.TimeoutMs,
				chromedp.WaitReady(step.Selector, chromedp.ByQuery),
			)
		default:
			return nil, fmt.Errorf("wait_for state %q not supported", state)
		}
	case "get_text":
		if step.Selector == "" {
			return nil, errors.New("get_text requires selector")
		}
		var text string
		err := runDOMTasks(ctx, step.TimeoutMs,
			chromedp.Text(step.Selector, &text, chromedp.NodeVisible),
		)
		return strings.TrimSpace(text), err
	case "get_html":
		if step.Selector == "" {
			return nil, errors.New("get_html requires selector")
		}
		var html string
		err := runDOMTasks(ctx, step.TimeoutMs,
			chromedp.OuterHTML(step.Selector, &html),
		)
		return strings.TrimSpace(html), err
	case "get_attribute":
		if step.Selector == "" || step.Attribute == "" {
			return nil, errors.New("get_attribute requires selector and attribute")
		}
		var value string
		var found bool
		err := runDOMTasks(ctx, step.TimeoutMs,
			chromedp.AttributeValue(step.Selector, step.Attribute, &value, &found),
		)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("attribute %q not found", step.Attribute)
		}
		return strings.TrimSpace(value), nil
	case "evaluate":
		if step.Expression == "" {
			return nil, errors.New("evaluate requires expression")
		}
		var value any
		err := runDOMTasks(ctx, step.TimeoutMs,
			chromedp.Evaluate(step.Expression, &value),
		)
		return value, err
	default:
		return nil, fmt.Errorf("unsupported action %q", action)
	}
}

func runDOMTasks(ctx context.Context, timeoutMs int, tasks ...chromedp.Action) error {
	runCtx := ctx
	var cancel context.CancelFunc
	if timeoutMs > 0 {
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()
	}
	return chromedp.Run(runCtx, tasks...)
}

func fetchSandboxDOMPageInfo(ctx context.Context) (string, string, error) {
	var url string
	var title string
	err := chromedp.Run(ctx,
		chromedp.Location(&url),
		chromedp.Title(&title),
	)
	return strings.TrimSpace(url), strings.TrimSpace(title), err
}

type sandboxDOMInspectConfig struct {
	IncludeInteractive bool
	MaxElements        int
}

func parseSandboxDOMInspect(args map[string]any) *sandboxDOMInspectConfig {
	raw, ok := args["inspect"]
	if !ok || raw == nil {
		return nil
	}
	switch value := raw.(type) {
	case bool:
		if !value {
			return nil
		}
		return &sandboxDOMInspectConfig{IncludeInteractive: true, MaxElements: sandboxDOMDefaultMaxElems}
	case map[string]any:
		include := readBoolWithDefault(value, "include_interactive", true)
		maxElements, ok := readInt(value, "max_elements")
		if !ok || maxElements <= 0 {
			maxElements = sandboxDOMDefaultMaxElems
		}
		if maxElements > sandboxDOMMaxElemsCap {
			maxElements = sandboxDOMMaxElemsCap
		}
		return &sandboxDOMInspectConfig{
			IncludeInteractive: include,
			MaxElements:        maxElements,
		}
	default:
		return nil
	}
}

func inspectSandboxDOM(ctx context.Context, maxElements int) ([]sandboxDOMElement, error) {
	if maxElements <= 0 {
		maxElements = sandboxDOMDefaultMaxElems
	}
	if maxElements > sandboxDOMMaxElemsCap {
		maxElements = sandboxDOMMaxElemsCap
	}
	script := fmt.Sprintf(`(() => {
  const max = %d;
  const elements = Array.from(document.querySelectorAll('a, button, input, select, textarea, [role="button"], [role="link"], [role="textbox"]'));
  const results = [];
  const seen = new Set();
  for (const el of elements) {
    if (results.length >= max) break;
    const text = (el.innerText || el.value || '').trim();
    const ariaLabel = (el.getAttribute('aria-label') || '').trim();
    const name = (el.getAttribute('name') || '').trim();
    const id = (el.id || '').trim();
    const role = (el.getAttribute('role') || '').trim();
    const type = (el.getAttribute('type') || '').trim();
    const label = text || ariaLabel || name || id;
    if (!label) continue;
    const key = el.tagName + '|' + label + '|' + id + '|' + name + '|' + ariaLabel;
    if (seen.has(key)) continue;
    seen.add(key);
    let selector = '';
    if (id) selector = '#' + id;
    else if (ariaLabel) selector = '[aria-label=' + JSON.stringify(ariaLabel) + ']';
    else if (name) selector = '[name=' + JSON.stringify(name) + ']';
    results.push({ tag: el.tagName.toLowerCase(), text, ariaLabel, name, id, role, type, selector });
  }
  return results;
})()`, maxElements)

	var elements []sandboxDOMElement
	if err := chromedp.Run(ctx, chromedp.Evaluate(script, &elements)); err != nil {
		return nil, err
	}
	return elements, nil
}

func buildSandboxDOMSummary(results []sandboxDOMStepResult, page map[string]any, elements []sandboxDOMElement, firstErr error) string {
	parts := []string{fmt.Sprintf("Executed %d DOM step(s).", len(results))}

	if title, ok := page["title"].(string); ok && strings.TrimSpace(title) != "" {
		if url, ok := page["url"].(string); ok && strings.TrimSpace(url) != "" {
			parts = append(parts, fmt.Sprintf("Page: %s (%s).", title, url))
		} else {
			parts = append(parts, fmt.Sprintf("Page: %s.", title))
		}
	} else if url, ok := page["url"].(string); ok && strings.TrimSpace(url) != "" {
		parts = append(parts, fmt.Sprintf("Page: %s.", url))
	}

	if values := summarizeSandboxDOMValues(results, sandboxDOMDefaultMaxHints); values != "" {
		parts = append(parts, fmt.Sprintf("Results: %s.", values))
	}

	if summary := summarizeSandboxDOMElements(elements, sandboxDOMDefaultMaxHints); summary != "" {
		parts = append(parts, fmt.Sprintf("Interactive elements: %s.", summary))
	}

	if firstErr != nil {
		parts = append(parts, fmt.Sprintf("Stopped after error: %s.", firstErr.Error()))
	}

	return strings.Join(parts, " ")
}

func summarizeSandboxDOMValues(results []sandboxDOMStepResult, limit int) string {
	if limit <= 0 {
		limit = sandboxDOMDefaultMaxHints
	}
	entries := make([]string, 0, limit)
	for _, result := range results {
		if result.Value == nil {
			continue
		}
		label := result.Name
		if label == "" {
			label = result.Action
		}
		value := formatSandboxDOMValue(result.Value)
		if value == "" {
			continue
		}
		entries = append(entries, fmt.Sprintf("%s=%s", label, value))
		if len(entries) >= limit {
			break
		}
	}
	if len(entries) == 0 {
		return ""
	}
	return strings.Join(entries, "; ")
}

func formatSandboxDOMValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return truncateSandboxDOMString(v, 120)
	case float64, float32, int, int64, bool:
		return fmt.Sprintf("%v", v)
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return truncateSandboxDOMString(string(encoded), 160)
	}
}

func truncateSandboxDOMString(value string, max int) string {
	trimmed := strings.TrimSpace(value)
	if max <= 0 || len(trimmed) <= max {
		return trimmed
	}
	return trimmed[:max] + "..."
}

func summarizeSandboxDOMElements(elements []sandboxDOMElement, limit int) string {
	if len(elements) == 0 {
		return ""
	}
	if limit <= 0 {
		limit = sandboxDOMDefaultMaxHints
	}
	entries := make([]string, 0, limit)
	for _, el := range elements {
		label := pickSandboxDOMElementLabel(el)
		if label == "" {
			continue
		}
		entry := label
		if el.Selector != "" {
			entry = fmt.Sprintf("%s (%s)", label, el.Selector)
		}
		if el.Tag != "" {
			entry = fmt.Sprintf("%s %s", el.Tag, entry)
		}
		entries = append(entries, truncateSandboxDOMString(entry, 140))
		if len(entries) >= limit {
			break
		}
	}
	if len(entries) == 0 {
		return ""
	}
	if len(elements) > limit {
		entries = append(entries, fmt.Sprintf("...and %d more", len(elements)-limit))
	}
	return strings.Join(entries, "; ")
}

func pickSandboxDOMElementLabel(el sandboxDOMElement) string {
	if strings.TrimSpace(el.Text) != "" {
		return strings.TrimSpace(el.Text)
	}
	if strings.TrimSpace(el.AriaLabel) != "" {
		return strings.TrimSpace(el.AriaLabel)
	}
	if strings.TrimSpace(el.Name) != "" {
		return strings.TrimSpace(el.Name)
	}
	if strings.TrimSpace(el.ID) != "" {
		return strings.TrimSpace(el.ID)
	}
	return ""
}
