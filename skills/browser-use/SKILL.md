---
name: browser-use
description: 浏览器自动化 — 通过 Playwright MCP Extension Relay 控制用户已登录的 Chrome，复用 cookies/session。
triggers:
  intent_patterns:
    - "浏览器|browser|网页|webpage|打开网站|open.*url|截图|screenshot|x\\.com|twitter"
  context_signals:
    keywords: ["浏览器", "browser", "网页", "截图", "打开", "navigate", "snapshot"]
  confidence_threshold: 0.7
priority: 6
requires_tools: [bash]
max_tokens: 4000
cooldown: 10
---

# browser-use

通过 Playwright MCP Extension Relay 控制用户当前 Chrome 浏览器，复用已登录的 session。

## 前置条件

1. Chrome 安装了 [Playwright MCP Bridge](https://chromewebstore.google.com/detail/playwright-mcp-bridge/mmlmfjhmonkocbjadbfplnigmagldckm) 扩展
2. `.env` 中配置了 `ALEX_BROWSER_BRIDGE_TOKEN`（从扩展弹窗复制）

## 调用

```bash
# 导航到 URL
python3 skills/browser-use/run.py navigate --url https://x.com

# 获取页面快照（无障碍树）
python3 skills/browser-use/run.py snapshot

# 点击元素（ref 来自 snapshot）
python3 skills/browser-use/run.py click --ref e44 --element 'Home link'

# 输入文本
python3 skills/browser-use/run.py type --ref e100 --text hello --submit true

# 截图
python3 skills/browser-use/run.py screenshot --filename page.png

# 管理标签页
python3 skills/browser-use/run.py tabs list

# 执行 JavaScript
python3 skills/browser-use/run.py evaluate --function '() => document.title'

# 执行 Playwright 代码
python3 skills/browser-use/run.py run_code --code 'async (page) => await page.title()'

# 按键
python3 skills/browser-use/run.py press_key --key Enter

# 等待文本出现
python3 skills/browser-use/run.py wait_for --text 'Loading complete'
```

## 典型工作流

1. `navigate` → 打开目标页面
2. `snapshot` → 获取页面结构和元素 ref
3. `click` / `type` → 与页面交互
4. `snapshot` → 确认结果
