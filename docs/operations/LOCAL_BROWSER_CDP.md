# Local Browser via CDP (Reuse Cookies / Logged-in Sessions)

This repo already has local `browser_*` tools backed by CDP (chromedp). To make them reuse your **existing cookies / logged-in sessions**, the simplest MVP is to run your browser with a **DevTools remote debugging port** and point `runtime.browser.cdp_url` at it.

> This is **not** the OpenClaw/extension-style “attach to a normal running Chrome without remote-debugging”. That may be added later.

## 1) Start Chrome (or Atlas) with remote debugging

### macOS

Quit the app first (Cmd+Q) so `--args` takes effect, then run:

```bash
./scripts/browser/start-cdp.sh --app chrome --port 9222
```

For Atlas (best-effort; depends on whether Atlas supports `--remote-debugging-port`):

```bash
./scripts/browser/start-cdp.sh --app atlas --port 9223
```

The script prints a DevTools endpoint like:

```text
http://127.0.0.1:9222
```

### Linux

```bash
BIN=google-chrome ./scripts/browser/start-cdp.sh --port 9222 --user-data-dir ~/.config/google-chrome
```

## 2) Configure ALEX to use local toolset + your CDP endpoint

In `~/.alex/config.yaml` (or `ALEX_CONFIG_PATH=/path/to/config.yaml`):

```yaml
runtime:
  toolset: "local"  # alias of "lark-local"
  browser:
    # can be ws://... (webSocketDebuggerUrl) OR http://127.0.0.1:<port>
    cdp_url: "http://127.0.0.1:9222"
```

Notes:
- When `cdp_url` is set, ALEX connects to that running browser. `headless/chrome_path/user_data_dir` are ignored in this mode.
- If you want ALEX to launch its own Chrome instead, leave `cdp_url` empty and configure `chrome_path/headless/user_data_dir`.

## 3) Use `browser_*` tools

With `runtime.toolset: local`, both `alex` (CLI) and `alex-server` (Web) will register local `browser_action/browser_dom/browser_screenshot/browser_info`.

If Atlas CDP doesn’t work, you can still use the `$atlas` skill for tab focus/open + bookmarks/history, but it won’t provide full DOM/click/type automation.

## Security note (CDP is powerful)

DevTools remote debugging effectively grants full control of that browser profile. Keep the port bound to localhost and shut it down when not needed.

