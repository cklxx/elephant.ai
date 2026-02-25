# Local Browser via CDP (Reuse Login Sessions)

Updated: 2026-02-10

Use local browser mode when you need the agent to reuse your local Chrome profile/cookies.

## 1) Start browser with remote debugging

On macOS, fully quit Chrome first, then run:

```bash
./scripts/browser/start-cdp.sh --app chrome --port 9222
```

## 2) Configure runtime (YAML)

```yaml
runtime:
  toolset: "local"
  browser:
    cdp_url: "http://127.0.0.1:9222"
    headless: false
```

`toolset: local` makes platform tools use local implementations for browser/file/shell execution.

## 3) Verify

Run a browser action from CLI:

```bash
./alex "Open browser to example.com and click the first link using browser_action."
```

## 4) Notes

- Current public browser tool is unified as `browser_action`.
- Older split names (`browser_info`, `browser_screenshot`, `browser_dom`) are not part of the default registry surface.
- If CDP is unreachable, check port exposure and ensure the browser process was started with debugging args.
