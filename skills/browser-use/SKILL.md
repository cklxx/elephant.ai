---
name: browser-use
description: When you need to interact with web pages using the user's logged-in browser → control Chrome via Playwright MCP, reusing cookies/session.
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
  context_signals:
    keywords: ["浏览器", "browser", "网页", "截图", "打开", "navigate", "snapshot", "tab", "标签页", "页面", "网站", "click", "点击", "form", "表单", "scrape", "抓取", "小红书", "微博", "知乎", "x.com", "twitter", "发帖", "评论", "点赞"]
  confidence_threshold: 0.6
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

## Tab 复用 & 会话持久性

**关键限制**：每次 run.py 调用会启动一个新的 MCP 进程，拿到一个新的 tab context。无法跨调用 `tabs select` 到之前的 tab。

**复用策略**：

1. **pipeline 模式（推荐）** — 多步操作打包到一个 pipeline 中，所有步骤共享同一个 tab：
   ```bash
   python3 skills/browser-use/run.py pipeline --steps '[
     {"tool":"browser_navigate","args":{"url":"https://x.com"}},
     {"tool":"browser_snapshot","args":{}},
     {"tool":"browser_click","args":{"ref":"e43","element":"Search"}},
     {"tool":"browser_type","args":{"ref":"e295","text":"AI agent","submit":true}},
     {"tool":"browser_snapshot","args":{}}
   ]'
   ```

2. **跨调用复用** — 再次 `navigate` 到同一 URL（Chrome 缓存使重新加载很快，且 cookies/登录状态保持）：
   ```bash
   # 第一次调用：打开 x.com，发帖
   python3 skills/browser-use/run.py navigate --url https://x.com
   # ... 交互完成

   # 第二次调用：再次打开同一页面，继续操作（登录状态保持）
   python3 skills/browser-use/run.py navigate --url https://x.com
   ```

3. **组合策略** — 先 navigate 进入页面，再用多轮单步操作（每轮先 snapshot 获取当前状态）：
   ```bash
   # 先导航
   python3 skills/browser-use/run.py navigate --url https://xiaohongshu.com
   # 看当前页面结构
   python3 skills/browser-use/run.py snapshot
   # 根据 snapshot 中的 ref 操作
   python3 skills/browser-use/run.py click --ref eXX --element '搜索'
   ```

## 典型工作流

### 基本流程
1. `navigate` → 打开目标页面
2. `snapshot` → 获取页面结构和元素 ref
3. `click` / `type` → 与页面交互
4. `snapshot` → 确认结果

### 社交平台操作（X / 小红书 / 微博 / 知乎）

**浏览 feed / 搜索**：
```bash
# 打开 → 搜索 → 获取结果
python3 skills/browser-use/run.py pipeline --steps '[
  {"tool":"browser_navigate","args":{"url":"https://x.com/search"}},
  {"tool":"browser_snapshot","args":{}},
  {"tool":"browser_type","args":{"ref":"SEARCH_REF","text":"AI agent","submit":true}},
  {"tool":"browser_wait_for","args":{"text":"results"}},
  {"tool":"browser_snapshot","args":{}}
]'
```

**发帖 / 评论**：
```bash
# 打开 → 找到输入框 → 输入内容 → 提交
python3 skills/browser-use/run.py pipeline --steps '[
  {"tool":"browser_navigate","args":{"url":"https://x.com/compose/post"}},
  {"tool":"browser_snapshot","args":{}},
  {"tool":"browser_type","args":{"ref":"POST_REF","text":"Hello world!","submit":false}},
  {"tool":"browser_click","args":{"ref":"SUBMIT_REF","element":"Post"}}
]'
```

**批量信息采集**：
```bash
# 打开 → snapshot → 提取数据 → scroll → snapshot → 提取更多
python3 skills/browser-use/run.py navigate --url https://www.xiaohongshu.com/explore
python3 skills/browser-use/run.py snapshot
# 解析 snapshot 中的内容
python3 skills/browser-use/run.py evaluate --function '() => window.scrollBy(0, 800)'
python3 skills/browser-use/run.py snapshot
```

### 表单填写 & 电商操作
```bash
# 自动填表：navigate → snapshot 找字段 → 逐一 type → 提交
python3 skills/browser-use/run.py pipeline --steps '[
  {"tool":"browser_navigate","args":{"url":"https://example.com/form"}},
  {"tool":"browser_snapshot","args":{}},
  {"tool":"browser_type","args":{"ref":"NAME_REF","text":"张三","submit":false}},
  {"tool":"browser_type","args":{"ref":"EMAIL_REF","text":"test@example.com","submit":false}},
  {"tool":"browser_click","args":{"ref":"SUBMIT_REF","element":"Submit"}}
]'
```

## 注意事项

- snapshot 返回无障碍树（YAML），ref 是元素标识符，用于 click/type 定位
- 每次新调用后 **必须先 snapshot** 获取最新的 ref（ref 不跨调用持久化）
- pipeline 内的 ref 在同一会话中有效，但建议在关键操作前重新 snapshot
- 用户已登录的 cookies/session 自动复用，无需重新登录
- 截图保存到当前目录，用于视觉确认
