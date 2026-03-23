---
name: browser-use
description: When you need to interact with web pages using the user's logged-in browser → control Chrome via @playwright/cli daemon, reusing cookies/session with persistent tabs.
triggers:
  intent_patterns:
    - "浏览器|browser|网页|webpage|打开网站|open.*url|截图|screenshot"
    - "x\\.com|twitter|小红书|xiaohongshu|redbook|微博|weibo|抖音|douyin|tiktok"
    - "reddit|hacker.?news|github\\.com|youtube|bilibili|B站|知乎|zhihu|豆瓣|douban"
    - "instagram|linkedin|facebook|threads|bluesky|mastodon|telegram"
    - "登录.*页面|login.*page|填写表单|fill.*form|submit.*form|提交表单"
    - "看看这个网站|帮我打开|check.*this.*site|visit.*page|go to"
    - "页面.*内容|page.*content|抓取|scrape|爬取|extract.*from.*page"
    - "点击|click|输入|type.*into|scroll|滚动|翻页|next.*page"
    - "网站.*截图|capture.*page|保存.*网页|save.*page"
    - "帮我.*操作|automate.*web|自动化.*网页|web.*automation"
    - "查看.*网页|look.*at|看一下|帮我看看|打开看看"
    - "发.*推|post.*tweet|发.*帖|create.*post|点赞|like|转发|retweet|repost"
    - "刷.*首页|browse.*feed|看.*热搜|trending|搜索.*用户|search.*user"
    - "评论|comment|关注|follow|收藏|bookmark|分享|share"
    - "下单|checkout|加购|add.*cart|购物|shopping"
    - "复用.*tab|reuse.*tab|之前.*标签页|切换.*标签"
  context_signals:
    keywords: ["浏览器", "browser", "网页", "截图", "打开", "navigate", "snapshot", "tab", "标签页", "页面", "网站", "click", "点击", "form", "表单", "scrape", "抓取", "小红书", "微博", "知乎", "x.com", "twitter", "发帖", "评论", "点赞"]
  confidence_threshold: 0.6
priority: 6
requires_tools: [bash]
max_tokens: 4000
cooldown: 10
---

# browser-use

通过 `@playwright/cli` daemon 连接用户真实 Chrome 浏览器（extension 模式），复用已登录的 session。
**Tab 跨调用持久化** — daemon 进程常驻，tab、cookies、登录状态在多次调用间存活。

## 前置条件

1. Chrome 安装了 [Playwright MCP Bridge](https://chromewebstore.google.com/detail/playwright-mcp-bridge/mmlmfjhmonkocbjadbfplnigmagldckm) 扩展
2. `.env` 中配置了 `ALEX_BROWSER_BRIDGE_TOKEN`（从扩展弹窗复制）

## 核心概念

- **Daemon 持久会话**：首次调用自动启动 daemon（`open --extension`），后续所有调用复用同一会话
- **Snapshot → ref → 操作**：`snapshot` 获取页面无障碍树，每个元素有 `ref`（如 `e44`），用 `click`/`fill`/`type` 操作该 ref
- **不抢前台**：extension 模式连接已有 Chrome，不会弹出新窗口或抢占焦点
- **Token 节省**：snapshot 输出到文件（`.playwright-cli/` 目录），不灌入 stdout

## 调用

```bash
# 首次打开 — 连接 Chrome 扩展（自动启动 daemon）
python3 skills/browser-use/run.py open

# 导航到 URL（自动确保 session 存在）
python3 skills/browser-use/run.py navigate --url https://x.com

# 获取页面快照（无障碍树 + 元素 ref）
python3 skills/browser-use/run.py snapshot

# 点击元素（ref 来自 snapshot）
python3 skills/browser-use/run.py click --ref e44

# 填充输入框（定位到 ref 再填入文本）
python3 skills/browser-use/run.py fill --ref e100 --text "hello world"

# 输入文本到当前焦点元素
python3 skills/browser-use/run.py type --text "search query" --submit true

# 悬停
python3 skills/browser-use/run.py hover --ref e50

# 下拉选择
python3 skills/browser-use/run.py select --ref e80 --value "option1"

# 截图
python3 skills/browser-use/run.py screenshot --filename page.png
python3 skills/browser-use/run.py screenshot --ref e44 --filename element.png
python3 skills/browser-use/run.py screenshot --full_page true --filename full.png

# 按键
python3 skills/browser-use/run.py press_key --key Enter
python3 skills/browser-use/run.py press_key --key "Control+a"

# 导航
python3 skills/browser-use/run.py go_back
python3 skills/browser-use/run.py go_forward
python3 skills/browser-use/run.py reload

# 执行 JavaScript
python3 skills/browser-use/run.py evaluate --function '() => document.title'
python3 skills/browser-use/run.py evaluate --function '(el) => el.textContent' --ref e44

# 执行 Playwright 代码
python3 skills/browser-use/run.py run_code --code 'async (page) => await page.title()'

# Tab 管理（跨调用持久！）
python3 skills/browser-use/run.py tabs                        # 列出所有 tab
python3 skills/browser-use/run.py tab_select --index 0        # 切换到 tab 0
python3 skills/browser-use/run.py tab_new --url https://...   # 新开 tab
python3 skills/browser-use/run.py tab_close --index 1         # 关闭 tab

# 调试
python3 skills/browser-use/run.py console                     # 查看 console 日志
python3 skills/browser-use/run.py network                     # 查看网络请求
python3 skills/browser-use/run.py pdf                         # 导出 PDF

# 关闭会话
python3 skills/browser-use/run.py close
```

## Tab 复用工作流

Tab 在 daemon 会话中持久化。跨调用复用示例：

```bash
# 调用 1: 打开 x.com
python3 skills/browser-use/run.py navigate --url https://x.com

# 调用 2: 直接 snapshot（还在 x.com 页面上！）
python3 skills/browser-use/run.py snapshot

# 调用 3: 操作 x.com 页面元素
python3 skills/browser-use/run.py click --ref e43

# 调用 4: 查看所有 tab
python3 skills/browser-use/run.py tabs

# 调用 5: 切换到另一个 tab
python3 skills/browser-use/run.py tab_select --index 0
```

## 典型场景

### 社交平台（X / 小红书 / 微博 / 知乎）

```bash
# 打开 → 搜索 → 获取结果
python3 skills/browser-use/run.py navigate --url https://x.com/search
python3 skills/browser-use/run.py snapshot
# 找到搜索框 ref → 填入关键词
python3 skills/browser-use/run.py fill --ref SEARCH_REF --text "AI agent"
python3 skills/browser-use/run.py press_key --key Enter
python3 skills/browser-use/run.py snapshot
```

### 发帖 / 评论

```bash
python3 skills/browser-use/run.py navigate --url https://x.com/compose/post
python3 skills/browser-use/run.py snapshot
python3 skills/browser-use/run.py fill --ref POST_REF --text "Hello world!"
python3 skills/browser-use/run.py click --ref SUBMIT_REF
```

### 表单填写

```bash
python3 skills/browser-use/run.py navigate --url https://example.com/form
python3 skills/browser-use/run.py snapshot
# 逐一填充字段
python3 skills/browser-use/run.py fill --ref NAME_REF --text "张三"
python3 skills/browser-use/run.py fill --ref EMAIL_REF --text "test@example.com"
python3 skills/browser-use/run.py select --ref COUNTRY_REF --value "CN"
python3 skills/browser-use/run.py click --ref SUBMIT_REF
```

### 信息采集（scroll + 多页）

```bash
python3 skills/browser-use/run.py navigate --url https://www.xiaohongshu.com/explore
python3 skills/browser-use/run.py snapshot
# 滚动加载更多
python3 skills/browser-use/run.py evaluate --function '() => window.scrollBy(0, 800)'
python3 skills/browser-use/run.py snapshot
# 翻页
python3 skills/browser-use/run.py click --ref NEXT_PAGE_REF
```

## 注意事项

- 首次调用会自动启动 daemon + 连接 Chrome 扩展，后续调用免启动
- `snapshot` 返回无障碍树，ref 是元素标识符（如 `e44`）
- ref 在 **同一 daemon 会话** 内跨调用持久，但页面导航/刷新后需重新 snapshot
- 不会打开新浏览器窗口，不会抢占前台焦点
- 用户已登录的 cookies/session 自动复用
