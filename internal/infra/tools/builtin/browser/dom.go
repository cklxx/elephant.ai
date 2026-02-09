package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"

	"github.com/chromedp/chromedp"
)

type browserDOMTool struct {
	shared.BaseTool
	manager *Manager
}

type browserDOMStep struct {
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

type browserDOMStepResult struct {
	Action   string `json:"action"`
	Name     string `json:"name,omitempty"`
	Selector string `json:"selector,omitempty"`
	OK       bool   `json:"ok"`
	Value    any    `json:"value,omitempty"`
	Error    string `json:"error,omitempty"`
}

type browserDOMElement struct {
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
	browserDOMToolName        = "browser_dom"
	browserDOMDefaultMaxHints = 6
	browserDOMDefaultMaxElems = 24
	browserDOMMaxElemsCap     = 60
)

// NewBrowserDOM returns a local browser_dom tool backed by chromedp.
func NewBrowserDOM(manager *Manager) tools.ToolExecutor {
	return &browserDOMTool{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: browserDOMToolName,
				Description: `Use the browser via CDP for selector-based DOM automation (Playwright-like).

Provide ordered steps using CSS selectors for click/fill/wait/query/evaluate flows. If reliable selectors are unavailable and coordinate interaction is required, use browser_action.`,
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
			},
			ports.ToolMetadata{
				Name:     browserDOMToolName,
				Version:  "0.1.0",
				Category: "web",
				Tags:     []string{"browser", "dom", "automation"},
				MaterialCapabilities: ports.ToolMaterialCapabilities{
					Produces: []string{"text/plain", "application/json"},
				},
			},
		),
		manager: manager,
	}
}

func (t *browserDOMTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	steps, err := parseBrowserDOMSteps(call.Arguments)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if t.manager == nil {
		err := errors.New("browser manager not configured")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	session, err := t.manager.Session(call.SessionID)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	continueOnError := shared.BoolArgWithDefault(call.Arguments, "continue_on_error", false)
	returnPage := shared.BoolArgWithDefault(call.Arguments, "return_page", true)
	inspectConfig := parseBrowserDOMInspect(call.Arguments)

	results := make([]browserDOMStepResult, 0, len(steps))
	var firstErr error
	pageInfo := map[string]any{}
	inspectElements := []browserDOMElement{}

	if err := session.withRunContext(ctx, t.manager.Config().timeoutOrDefault(), func(runCtx context.Context) error {
		for _, step := range steps {
			result := browserDOMStepResult{
				Action:   step.Action,
				Name:     step.Name,
				Selector: step.Selector,
			}

			value, execErr := runBrowserDOMStep(runCtx, step)
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

		if returnPage {
			if url, title, err := fetchBrowserDOMPageInfo(runCtx); err == nil {
				if url != "" {
					pageInfo["url"] = url
				}
				if title != "" {
					pageInfo["title"] = title
				}
			}
		}

		if inspectConfig != nil && inspectConfig.IncludeInteractive {
			if elements, err := inspectBrowserDOM(runCtx, inspectConfig.MaxElements); err == nil {
				inspectElements = elements
			}
		}
		return nil
	}); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	content := buildBrowserDOMSummary(results, pageInfo, inspectElements, firstErr)
	metadata := map[string]any{
		"browser_dom": map[string]any{
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

func parseBrowserDOMSteps(args map[string]any) ([]browserDOMStep, error) {
	rawSteps, ok := args["steps"].([]any)
	if !ok || len(rawSteps) == 0 {
		return nil, errors.New("steps is required")
	}

	steps := make([]browserDOMStep, 0, len(rawSteps))
	for idx, item := range rawSteps {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("steps[%d] must be an object", idx)
		}

		action := strings.ToLower(strings.TrimSpace(shared.StringArgStrict(m, "action")))
		if action == "" {
			action = strings.ToLower(strings.TrimSpace(shared.StringArgStrict(m, "action_type")))
		}
		if action == "" {
			return nil, fmt.Errorf("steps[%d].action is required", idx)
		}

		text := strings.TrimSpace(shared.StringArgStrict(m, "text"))
		if text == "" {
			text = strings.TrimSpace(shared.StringArgStrict(m, "value"))
		}

		timeoutMs, _ := shared.IntArg(m, "timeout_ms")
		if timeoutMs < 0 {
			timeoutMs = 0
		}

		step := browserDOMStep{
			Action:     action,
			Selector:   strings.TrimSpace(shared.StringArgStrict(m, "selector")),
			URL:        strings.TrimSpace(shared.StringArgStrict(m, "url")),
			Text:       text,
			Key:        strings.TrimSpace(shared.StringArgStrict(m, "key")),
			Attribute:  strings.TrimSpace(shared.StringArgStrict(m, "attribute")),
			Expression: strings.TrimSpace(shared.StringArgStrict(m, "expression")),
			Name:       strings.TrimSpace(shared.StringArgStrict(m, "name")),
			State:      strings.TrimSpace(shared.StringArgStrict(m, "state")),
			TimeoutMs:  timeoutMs,
		}

		steps = append(steps, step)
	}

	return steps, nil
}

func runBrowserDOMStep(ctx context.Context, step browserDOMStep) (any, error) {
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

func fetchBrowserDOMPageInfo(ctx context.Context) (string, string, error) {
	var url string
	var title string
	err := chromedp.Run(ctx,
		chromedp.Location(&url),
		chromedp.Title(&title),
	)
	return strings.TrimSpace(url), strings.TrimSpace(title), err
}

type browserDOMInspectConfig struct {
	IncludeInteractive bool
	MaxElements        int
}

func parseBrowserDOMInspect(args map[string]any) *browserDOMInspectConfig {
	raw, ok := args["inspect"]
	if !ok || raw == nil {
		return nil
	}
	switch value := raw.(type) {
	case bool:
		if !value {
			return nil
		}
		return &browserDOMInspectConfig{IncludeInteractive: true, MaxElements: browserDOMDefaultMaxElems}
	case map[string]any:
		include := shared.BoolArgWithDefault(value, "include_interactive", true)
		maxElements, ok := shared.IntArg(value, "max_elements")
		if !ok || maxElements <= 0 {
			maxElements = browserDOMDefaultMaxElems
		}
		if maxElements > browserDOMMaxElemsCap {
			maxElements = browserDOMMaxElemsCap
		}
		return &browserDOMInspectConfig{
			IncludeInteractive: include,
			MaxElements:        maxElements,
		}
	default:
		return nil
	}
}

func inspectBrowserDOM(ctx context.Context, maxElements int) ([]browserDOMElement, error) {
	if maxElements <= 0 {
		maxElements = browserDOMDefaultMaxElems
	}
	if maxElements > browserDOMMaxElemsCap {
		maxElements = browserDOMMaxElemsCap
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

	var elements []browserDOMElement
	if err := chromedp.Run(ctx, chromedp.Evaluate(script, &elements)); err != nil {
		return nil, err
	}
	return elements, nil
}

func buildBrowserDOMSummary(results []browserDOMStepResult, page map[string]any, elements []browserDOMElement, firstErr error) string {
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

	if values := summarizeBrowserDOMValues(results, browserDOMDefaultMaxHints); values != "" {
		parts = append(parts, fmt.Sprintf("Results: %s.", values))
	}

	if summary := summarizeBrowserDOMElements(elements, browserDOMDefaultMaxHints); summary != "" {
		parts = append(parts, fmt.Sprintf("Interactive elements: %s.", summary))
	}

	if firstErr != nil {
		parts = append(parts, fmt.Sprintf("Stopped after error: %s.", firstErr.Error()))
	}

	return strings.Join(parts, " ")
}

func summarizeBrowserDOMValues(results []browserDOMStepResult, limit int) string {
	if limit <= 0 {
		limit = browserDOMDefaultMaxHints
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
		value := formatBrowserDOMValue(result.Value)
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

func formatBrowserDOMValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return truncateBrowserDOMString(v, 120)
	case float64, float32, int, int64, bool:
		return fmt.Sprintf("%v", v)
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return truncateBrowserDOMString(string(encoded), 160)
	}
}

func truncateBrowserDOMString(value string, max int) string {
	trimmed := strings.TrimSpace(value)
	if max <= 0 || len(trimmed) <= max {
		return trimmed
	}
	return trimmed[:max] + "..."
}

func summarizeBrowserDOMElements(elements []browserDOMElement, limit int) string {
	if len(elements) == 0 {
		return ""
	}
	if limit <= 0 {
		limit = browserDOMDefaultMaxHints
	}
	entries := make([]string, 0, limit)
	for _, el := range elements {
		label := pickBrowserDOMElementLabel(el)
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
		entries = append(entries, truncateBrowserDOMString(entry, 140))
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

func pickBrowserDOMElementLabel(el browserDOMElement) string {
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
