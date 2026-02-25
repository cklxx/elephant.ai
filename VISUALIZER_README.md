# Claude Code Visualizer 🦀

实时可视化 Claude Code 在代码库中的工作过程，将 AI 的思考和操作以直观的动画形式展现。

## ✨ 特性

- **实时文件夹热力图**：颜色深度反映代码活动强度（文件数量和代码行数）
- **螃蟹 AI Agent**：可爱的螃蟹角色在文件夹间移动，展示当前正在操作的位置
- **工具识别**：自动识别 Read、Write、Edit、Grep、Glob、Bash 等工具
- **事件日志**：完整记录所有工具调用历史
- **Server-Sent Events (SSE)**：零轮询，实时推送事件
- **独立部署**：可作为独立项目运行，不依赖 elephant.ai 主项目

## 🎬 快速开始

### 1. 安装依赖

```bash
cd web
npm install
```

### 2. 配置 Claude Code Hooks

visualizer 通过 Claude Code 的 hook 机制捕获工具调用事件。

#### 步骤 2.1: 安装 jq

```bash
# macOS
brew install jq

# Ubuntu/Debian
sudo apt-get install jq
```

#### 步骤 2.2: 复制 hook 脚本

将 `~/.claude/hooks/visualizer-hook.sh` 标记为可执行：

```bash
chmod +x ~/.claude/hooks/visualizer-hook.sh
```

#### 步骤 2.3: 配置 hooks.json

确保 `~/.claude/hooks.json` 包含以下配置：

```json
{
  "hooks": [
    {
      "event": "tool-use",
      "matcher": "**/*",
      "hooks": [
        {
          "type": "command",
          "command": "VISUALIZER_URL=http://localhost:3002/api/visualizer/events ~/.claude/hooks/visualizer-hook.sh",
          "async": true,
          "timeout": 5
        }
      ]
    },
    {
      "event": "tool-result",
      "matcher": "**/*",
      "hooks": [
        {
          "type": "command",
          "command": "VISUALIZER_URL=http://localhost:3002/api/visualizer/events ~/.claude/hooks/visualizer-hook.sh",
          "async": true,
          "timeout": 5
        }
      ]
    }
  ]
}
```

### 3. 启动开发服务器

```bash
cd web
PORT=3002 npm run dev
```

### 4. 打开可视化界面

访问 [http://localhost:3002/visualizer](http://localhost:3002/visualizer)

### 5. 开始使用 Claude Code

在任意项目中打开 Claude Code CLI 或 IDE 插件，执行一些操作：

```bash
claude-code
> Read the README.md file
> Search for "function" in the codebase
> List all TypeScript files
```

你应该能在可视化界面中看到：
- 文件夹热力图实时更新
- 螃蟹移动到相应文件夹
- 事件日志记录所有操作

## 🏗️ 架构

```
┌─────────────────┐
│ Claude Code CLI │
└────────┬────────┘
         │ (stdin JSON)
         ↓
┌────────────────────┐
│ visualizer-hook.sh │  ← ~/.claude/hooks/
└────────┬───────────┘
         │ (HTTP POST)
         ↓
┌────────────────────┐
│  /api/visualizer   │
│    /events  (POST) │  ← Next.js API Routes
│    /stream  (SSE)  │
└────────┬───────────┘
         │ (Server-Sent Events)
         ↓
┌────────────────────┐
│ Visualizer Page    │  ← React + Tailwind CSS
│  - FolderMap       │
│  - CrabAgent       │
│  - EventLog        │
└────────────────────┘
```

## 📁 文件结构

```
web/
├── app/
│   ├── api/visualizer/
│   │   ├── events/route.ts    # POST 接收事件, GET 查询历史
│   │   └── stream/route.ts    # SSE 实时流
│   └── visualizer/
│       └── page.tsx            # 可视化页面入口
├── components/visualizer/
│   ├── CodeVisualizer.tsx      # 主组件
│   ├── FolderMap.tsx           # 文件夹热力图
│   ├── CrabAgent.tsx           # 螃蟹动画
│   └── EventLog.tsx            # 事件日志
└── hooks/
    └── useVisualizerStream.ts  # SSE 连接 hook

~/.claude/hooks/
├── visualizer-hook.sh          # Hook 脚本
└── hooks.json                  # Hook 配置
```

## 🔧 配置选项

### 环境变量

- `PORT`: 开发服务器端口 (默认: 3000，推荐: 3002)
- `VISUALIZER_URL`: Hook 发送事件的 URL (默认: http://localhost:3002/api/visualizer/events)

### Hook 脚本参数

在 `~/.claude/hooks.json` 中可以自定义：

```json
{
  "command": "VISUALIZER_URL=http://custom-host:port/api/visualizer/events ~/.claude/hooks/visualizer-hook.sh",
  "async": true,
  "timeout": 5  // 超时时间（秒）
}
```

## 🎨 自定义样式

### 修改颜色方案

编辑 `web/components/visualizer/FolderMap.tsx`:

```tsx
const getFolderStyle = (folder: FolderStats) => {
  const intensity = getIntensity(folder);

  // 自定义颜色映射
  if (intensity > 0.7) {
    bgColor = 'bg-purple-600';  // 高强度 -> 紫色
  } else if (intensity > 0.4) {
    bgColor = 'bg-blue-600';    // 中强度 -> 蓝色
  } else {
    bgColor = 'bg-blue-200';    // 低强度 -> 浅蓝
  }
  // ...
};
```

### 修改螃蟹样式

编辑 `web/components/visualizer/CrabAgent.tsx` 中的 `CrabSVG` 组件。

### 添加动画效果

编辑 `web/app/globals.css`:

```css
@keyframes custom-wave {
  /* 自定义动画 */
}
```

## 🚀 作为独立项目部署

### Docker 部署

创建 `Dockerfile`:

```dockerfile
FROM node:20-alpine AS base

WORKDIR /app
COPY web/package*.json ./
RUN npm ci --only=production

COPY web/ ./
RUN npm run build

EXPOSE 3002
CMD ["npm", "start"]
```

构建并运行：

```bash
docker build -t claude-visualizer .
docker run -p 3002:3002 claude-visualizer
```

### Vercel/Netlify 部署

**注意**：由于需要 API Routes (非静态导出)，必须部署到支持 Next.js 服务端功能的平台。

1. 移除 `next.config.mjs` 中的 `output: 'export'`
2. 部署到 Vercel:

```bash
vercel --prod
```

3. 更新 hook URL:

```bash
export VISUALIZER_URL=https://your-domain.vercel.app/api/visualizer/events
```

## 📊 性能优化

- **事件去重**：自动过滤重复事件（基于内容哈希）
- **内存限制**：最多保存 200 个事件
- **SSE 心跳**：30 秒心跳保持连接
- **异步 Hook**：Hook 脚本异步执行，不阻塞 Claude Code

## 🐛 故障排查

### Hook 不触发

```bash
# 测试 hook 脚本
echo '{"hook_event_name": "tool-use", "tool_name": "Read", "tool_input": {"file_path": "/test.ts"}}' | \
  ~/.claude/hooks/visualizer-hook.sh

# 检查日志
tail -f ~/.claude/hook.log  # (如果配置了日志)
```

### API 500 错误

检查 Next.js 开发服务器日志：

```bash
tail -f /tmp/visualizer-dev.log
```

### SSE 连接失败

1. 检查浏览器控制台是否有 CORS 错误
2. 确认开发服务器运行在 http://localhost:3002
3. 尝试刷新页面重新建立连接

### 文件夹不显示

确认事件中包含有效的 `path` 字段：

```bash
curl "http://localhost:3002/api/visualizer/events?limit=10" | jq '.events[] | .path'
```

## 🤝 贡献

欢迎提交 Issue 和 Pull Request!

### 开发流程

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交改动 (`git commit -m 'Add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 开启 Pull Request

## 📄 许可证

MIT License - 详见 LICENSE 文件

## 🙏 致谢

- **Claude Code**: Anthropic 官方 CLI 工具
- **Next.js**: React 框架
- **Tailwind CSS**: 实用优先的 CSS 框架
- **Zod**: TypeScript 运行时验证

---

**Made with ❤️ for the Claude Code community**
