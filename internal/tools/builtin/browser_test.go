package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

const fakePlaywrightModule = `
const fs = require('fs');

const png = Buffer.from(
  '89504e470d0a1a0a0000000d4948445200000001000000010806000000' +
  '1f15c4890000000a49444154789c6360000002000154a24f0500000000' +
  '49454e44ae426082',
  'hex'
);

const toElement = (node) => {
  const rect = node.rect || {};
  return {
    tagName: (node.tag || 'div').toUpperCase(),
    id: node.id || '',
    className: node.className || '',
    innerText: node.text || '',
    async fill(text) { (node.filled = node.filled || []).push(text); },
    getBoundingClientRect: () => ({
      width: rect.width || 0,
      height: rect.height || 0,
      top: rect.top || 0,
      left: rect.left || 0,
    }),
  };
};

const matches = (selector, el) => {
  if (selector.startsWith('#')) {
    return el.id === selector.slice(1);
  }
  if (selector.startsWith('.')) {
    return (el.className || '').split(/\s+/).includes(selector.slice(1));
  }
  return el.tagName.toLowerCase() === selector.toLowerCase();
};

const createPage = (state) => {
  const elements = state.nodes.map(toElement);
  const find = (selector) => elements.find((el) => matches(selector, el));
  return {
    async goto(url) { state.lastUrl = url; },
    async setDefaultTimeout() {},
    async setExtraHTTPHeaders(headers) { state.pageHeaders = headers; },
    async waitForTimeout(ms) { state.waits.push(ms); },
    async evaluate(fn) {
      const previousDocument = global.document;
      const previousWindow = global.window;
      const doc = {
        documentElement: { scrollHeight: state.scrollHeight },
        body: {
          scrollHeight: state.scrollHeight,
          querySelectorAll: () => elements,
        },
        querySelectorAll: () => elements,
      };
      const win = { innerWidth: state.viewport.width, innerHeight: state.viewport.height };
      global.document = doc;
      global.window = win;
      try {
        return await fn();
      } finally {
        global.document = previousDocument;
        global.window = previousWindow;
      }
    },
    async $(selector) {
      const element = find(selector);
      return element ? { click: async () => { state.clicked.push(selector); }, fill: async (text) => { state.filled.push([selector, text]); } } : null;
    },
    mouse: {
      wheel: async (dx, dy) => { state.scrolled.push([dx, dy]); },
    },
    keyboard: {
      press: async (key) => { state.pressed.push(key); },
      type: async (text) => { state.typed.push(text); },
    },
    async screenshot(options) {
      fs.writeFileSync(options.path, png);
    },
  };
};

const createContext = (state) => {
  const page = createPage(state);
  return {
    addCookies: async (cookies) => { state.cookies = cookies; },
    addInitScript: async (fn) => { state.initScripts.push(String(fn)); },
    setExtraHTTPHeaders: async (headers) => { state.extraHTTPHeaders = headers; },
    newPage: async () => page,
    close: async () => { state.contextClosed = true; },
  };
};

module.exports = {
  chromium: {
    launch: async (options = {}) => {
      const nodes = JSON.parse(process.env.PLAYWRIGHT_FAKE_NODES || '[]');
      const viewport = JSON.parse(process.env.PLAYWRIGHT_FAKE_VIEWPORT || '{"width":1280,"height":720}');
      const scrollHeight = parseInt(process.env.PLAYWRIGHT_FAKE_SCROLL_HEIGHT || '2000', 10);
      const state = {
        nodes,
        viewport,
        scrollHeight,
        clicked: [],
        scrolled: [],
        pressed: [],
        typed: [],
        filled: [],
        cookies: [],
        waits: [],
        initScripts: [],
        extraHTTPHeaders: null,
        launchOptions: options,
      };
      const contextFactory = createContext(state);
      return {
        newContext: async (ctxOptions = {}) => {
          state.contextOptions = ctxOptions;
          if (ctxOptions.viewport) {
            state.viewport = ctxOptions.viewport;
          }
          return { ...contextFactory };
        },
        close: async () => { state.closed = true; },
      };
    },
  },
};
`

func TestParseBrowserDSLRequiresScrollAmount(t *testing.T) {
	_, err := parseBrowserDSL("scroll down")
	if err == nil {
		t.Fatalf("expected scroll without amount to fail")
	}
	_, err = parseBrowserDSL("scroll down 0")
	if err == nil {
		t.Fatalf("expected non-positive scroll amount to fail")
	}
	cmds, err := parseBrowserDSL("scroll up 120")
	if err != nil {
		t.Fatalf("expected valid scroll to parse: %v", err)
	}
	if got := cmds[0].label(); got != "scroll up 120" {
		t.Fatalf("expected scroll label to include amount, got %q", got)
	}
}

func TestParseBrowserDSLSupportsTypingAndPress(t *testing.T) {
	cmds, err := parseBrowserDSL(`
press Enter
type hello world into #search
type quick input
`)
	if err != nil {
		t.Fatalf("expected typing and press to parse: %v", err)
	}
	if len(cmds) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(cmds))
	}
	if cmds[0].kind != commandPress || cmds[0].value != "Enter" {
		t.Fatalf("expected press Enter, got %+v", cmds[0])
	}
	if cmds[1].kind != commandType || cmds[1].value != "hello world" || cmds[1].target != "#search" {
		t.Fatalf("expected type into selector, got %+v", cmds[1])
	}
	if cmds[2].label() != "type quick input" {
		t.Fatalf("expected type label without target, got %q", cmds[2].label())
	}
}

func TestCompilePlaywrightScriptInjectsPresetCookies(t *testing.T) {
	cmds, err := parseBrowserDSL("open https://example.com/home")
	if err != nil {
		t.Fatalf("parseBrowserDSL: %v", err)
	}

	tool := &browserTool{
		presetCookies: PresetCookieJar{
			"example.com": {
				{Name: "antibot", Value: "ok", Domain: "example.com"},
			},
		},
	}
	script, err := tool.compilePlaywrightScript(cmds, t.TempDir(), viewport{width: 1024, height: 768})
	if err != nil {
		t.Fatalf("compilePlaywrightScript: %v", err)
	}
	if !strings.Contains(script, `"example.com"`) || !strings.Contains(script, "antibot") {
		t.Fatalf("expected preset cookies to be embedded, got %s", script)
	}
	if !strings.Contains(script, `applyPresetCookies("https://example.com/home")`) {
		t.Fatalf("expected applyPresetCookies call for open command, got %s", script)
	}
}

func TestCompilePlaywrightScriptAddsStealthHeaders(t *testing.T) {
	cmds, err := parseBrowserDSL("open https://www.xiaohongshu.com/explore")
	if err != nil {
		t.Fatalf("parseBrowserDSL: %v", err)
	}

	script, err := (&browserTool{}).compilePlaywrightScript(cmds, t.TempDir(), viewport{width: 1280, height: 720})
	if err != nil {
		t.Fatalf("compilePlaywrightScript: %v", err)
	}
	for _, want := range []string{
		"ignoreDefaultArgs",
		"navigator, 'webdriver'",
		"extraHTTPHeaders: baseHeaders",
		"applyHeaders(\"https://www.xiaohongshu.com/explore\")",
		"waitUntil: 'domcontentloaded'",
		"waitForTimeout(1200)",
		"headers['Referer'] = 'https://www.xiaohongshu.com/'",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("expected script to include %q, got %s", want, script)
		}
	}
}

func TestDefaultPresetCookieJarCoversMajorPlatforms(t *testing.T) {
	jar := defaultPresetCookieJar()
	if len(jar) < 20 {
		t.Fatalf("expected at least 20 preset domains, got %d", len(jar))
	}
	for _, domain := range []string{
		".xiaohongshu.com",
		".google.com",
		".youtube.com",
		".tiktok.com",
		".baidu.com",
		".taobao.com",
		".tmall.com",
		".jd.com",
		".bilibili.com",
		".douyin.com",
		".weibo.com",
		".zhihu.com",
		".qq.com",
		".so.com",
		".sogou.com",
		".bing.com",
		".reddit.com",
		".x.com",
		".douban.com",
		".kuaishou.com",
	} {
		if _, ok := jar[domain]; !ok {
			t.Fatalf("expected preset cookies for %s", domain)
		}
	}
}

func TestRunPlaywrightScriptExecutesActions(t *testing.T) {
	dsl := `
open https://example.com
if exists #primary
  click #primary
else
  click #secondary
end
scroll up 120
if exists .missing
  click .missing
else
  scroll down 240
end
press Enter
type demo query into #primary
type final input
`
	commands, err := parseBrowserDSL(dsl)
	if err != nil {
		t.Fatalf("parseBrowserDSL: %v", err)
	}

	workdir := t.TempDir()
	writeFakePlaywrightModule(t, workdir)
	t.Setenv("NODE_PATH", filepath.Join(workdir, "node_modules"))
	t.Setenv("PLAYWRIGHT_FAKE_NODES", `[{"id":"primary","tag":"button","className":"cta","text":"Primary call to action","rect":{"height":48,"width":160,"top":12,"left":16}},{"tag":"section","className":"secondary","text":"Secondary panel","rect":{"height":640,"width":900,"top":200,"left":0}}]`)
	t.Setenv("PLAYWRIGHT_FAKE_SCROLL_HEIGHT", "2200")

	runResult, err := (&browserTool{}).runPlaywright(context.Background(), commands, workdir, viewport{width: 1280, height: 720})
	if err != nil {
		t.Fatalf("runPlaywright: %v", err)
	}

	if len(runResult.Steps) != 7 {
		t.Fatalf("expected 7 steps, got %d", len(runResult.Steps))
	}

	first := runResult.Steps[0]
	if first.Label == "" || len(first.Meta) == 0 {
		t.Fatalf("expected first step to include meta context, got %+v", first)
	}
	if height := intFromAny(first.Meta["scrollHeight"]); height != 2200 {
		t.Fatalf("expected scrollHeight=2200, got %d", height)
	}
	if nodes, ok := first.Meta["nodes"].([]any); !ok || len(nodes) == 0 {
		t.Fatalf("expected sampled nodes in meta, got %+v", first.Meta["nodes"])
	}

	if runResult.Steps[1].Label != "click #primary" {
		t.Fatalf("expected click step, got %q", runResult.Steps[1].Label)
	}
	if runResult.Steps[2].Label != "scroll up 120" {
		t.Fatalf("expected scroll up label, got %q", runResult.Steps[2].Label)
	}
	if runResult.Steps[3].Label != "scroll down 240" {
		t.Fatalf("expected scroll down label, got %q", runResult.Steps[3].Label)
	}
	if runResult.Steps[4].Label != "press Enter" {
		t.Fatalf("expected press label, got %q", runResult.Steps[4].Label)
	}
	if runResult.Steps[5].Label != "type demo query into #primary" {
		t.Fatalf("expected type into selector label, got %q", runResult.Steps[5].Label)
	}
	if runResult.Steps[6].Label != "type final input" {
		t.Fatalf("expected type label, got %q", runResult.Steps[6].Label)
	}
}

func TestBrowserExecuteAddsReminderAndMetadata(t *testing.T) {
	workdir := t.TempDir()
	tool := &browserTool{
		runner: func(ctx context.Context, commands []browserCommand, _ string, _ viewport) (playwrightRunResult, error) {
			return playwrightRunResult{
				Steps:       []playwrightStep{{Label: "open https://example.com"}},
				artifactDir: workdir,
			}, nil
		},
		viewport: viewport{width: 1280, height: 720},
	}

	ctx := WithWorkingDir(context.Background(), workdir)
	call := ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"script": "open https://example.com",
		},
	}

	res, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if res == nil {
		t.Fatalf("expected result, got nil")
	}
	if !strings.Contains(res.Content, "<system-reminder>") {
		t.Fatalf("expected reminder in content, got %q", res.Content)
	}
	if !strings.Contains(res.Content, "https://example.com") {
		t.Fatalf("expected URL in reminder, got %q", res.Content)
	}
	if res.Metadata == nil || res.Metadata["url"] != "https://example.com" {
		t.Fatalf("expected metadata url to be set, got %+v", res.Metadata)
	}
	browserMeta, ok := res.Metadata["browser"].(map[string]any)
	if !ok {
		t.Fatalf("expected browser metadata map, got %+v", res.Metadata["browser"])
	}
	if browserMeta["url"] != "https://example.com" {
		t.Fatalf("expected browser metadata url, got %+v", browserMeta["url"])
	}
}

func writeFakePlaywrightModule(t *testing.T, root string) {
	t.Helper()
	moduleDir := filepath.Join(root, "node_modules", "playwright")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("create module dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "index.js"), []byte(strings.TrimSpace(fakePlaywrightModule)), 0o644); err != nil {
		t.Fatalf("write fake playwright: %v", err)
	}
}
