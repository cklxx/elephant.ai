# Claude Code Visualizer

实时可视化 Claude Code 在 elephant.ai 代码库中的工作过程。

## 快速开始

### 1. 启动可视化界面

```bash
cd web
PORT=3002 npm run dev
```

在浏览器中打开：`http://localhost:3002/visualizer`

### 2. 配置 Claude Code Hooks

Hooks 已经配置在 `~/.claude/hooks.json`，会自动捕获以下事件：
- `tool-use` - 工具调用开始
- `tool-result` - 工具调用完成

Hook 脚本位置：`~/.claude/hooks/visualizer-hook.sh`

### 3. 使用 Claude Code

在项目根目录打开 Claude Code：

```bash
cd /Users/bytedance/code/elephant.ai
claude-code
```

执行任何操作（读取文件、编辑代码、搜索等），可视化界面会实时显示：
- 🦀 螃蟹移动到正在操作的文件/文件夹
- 💬 气泡显示当前操作和文件名
- 📊 事件日志记录所有历史操作

## 界面说明

### 主要区域

```
┌─────────────────────────────────────────────────────┐
│  🦀 Claude Code Visualizer          📊 7 事件  ✓ 已连接 │
├──────────────────────────┬──────────────────────────┤
│                          │  📊 事件日志               │
│   Folder Treemap         │  ┌──────────────────┐    │
│   (文件夹可视化)           │  │ 📖 Read          │    │
│                          │  │ path/to/file.go  │    │
│   🦀 (螃蟹在此移动)        │  │ 12:34:56         │    │
│                          │  └──────────────────┘    │
│                          │  ...                     │
└──────────────────────────┴──────────────────────────┘
```

### 螃蟹动画状态

| 工具 | 螃蟹行为 | 气泡显示 |
|------|---------|---------|
| Read | 移动到文件，眼睛发光 | 📖 正在阅读 |
| Write | 移动到文件，挥舞钳子 | ✍️ 正在写入 |
| Edit | 移动到文件，精准修改 | ✏️ 正在编辑 |
| Grep | 快速扫描多个文件 | 🔍 正在搜索 |
| Glob | 在文件夹间跳跃 | 🗂️ 正在查找 |
| Bash | 移动到终端区域 | 💻 执行命令 |
| Thinking | 飘到顶部，思考气泡 | 💭 正在思考 |

### 事件日志

右侧面板显示所有工具调用历史：
- **颜色标记**：不同工具用不同颜色的左边框
- **状态标签**：started (蓝色) / completed (绿色) / error (红色)
- **文件路径**：显示最后两级目录
- **时间戳**：操作发生的时间

## 技术架构

```
Claude Code (hooks)
      ↓
~/.claude/hooks/visualizer-hook.sh
      ↓ (HTTP POST)
/api/visualizer/events
      ↓ (SSE broadcast)
/api/visualizer/stream
      ↓
Frontend (React + SSE)
      ↓
🦀 Crab Animation + Event Log
```

### 数据流

1. **捕获事件**：Hook 脚本监听 `tool-use` 和 `tool-result` 事件
2. **解析数据**：使用 `jq` 从 stdin 解析 JSON，提取工具名和文件路径
3. **发送到 API**：异步 POST 到 `/api/visualizer/events`
4. **验证和去重**：Zod 验证 + 内容哈希去重
5. **广播事件**：通过 SSE 推送给所有连接的客户端
6. **前端更新**：React 组件实时更新螃蟹位置和事件日志

## 配置选项

### 环境变量

```bash
# API 端点 (默认: http://localhost:3002/api/visualizer/events)
export VISUALIZER_URL="http://custom-host:port/api/visualizer/events"

# 启用调试日志 (写入 ~/.claude/visualizer-hook.log)
export DEBUG=1
```

### URL 参数

访问可视化界面时可使用以下参数：

```
http://localhost:3002/visualizer?workspace=/path/to/project&depth=4
```

- `workspace` - 项目根目录（自动检测 go.mod 或 .git）
- `depth` - 文件夹扫描深度（默认：3）

## 故障排查

### Hook 不触发

1. 检查 hooks 配置：
   ```bash
   cat ~/.claude/hooks.json
   ```

2. 检查 hook 脚本权限：
   ```bash
   ls -la ~/.claude/hooks/visualizer-hook.sh
   chmod +x ~/.claude/hooks/visualizer-hook.sh
   ```

3. 启用调试模式查看日志：
   ```bash
   DEBUG=1 cat test-event.json | ~/.claude/hooks/visualizer-hook.sh
   cat ~/.claude/visualizer-hook.log
   ```

### API 不响应

1. 检查 Next.js 开发服务器是否运行：
   ```bash
   curl http://localhost:3002/api/visualizer/events
   ```

2. 查看服务器日志：
   ```bash
   tail -f /tmp/visualizer-test.log
   ```

3. 确认 API 路由有 `export const dynamic = 'force-dynamic'` 配置

### 前端不更新

1. 检查 SSE 连接状态（界面右上角应显示 "✓ 已连接"）
2. 打开浏览器开发者工具查看控制台错误
3. 检查网络标签中的 `/api/visualizer/stream` 请求

## 性能优化

当前配置：
- **事件历史限制**：200 条（可在 `events/route.ts` 中调整 `MAX_EVENTS`）
- **去重缓存**：500 条哈希（避免重复事件）
- **心跳间隔**：30 秒（保持 SSE 连接）
- **文件夹扫描深度**：3 层（平衡性能和可见性）

如果遇到性能问题：
- 减少扫描深度：`?depth=2`
- 增加事件去重窗口（修改哈希函数的时间戳精度）
- 清空事件历史：重启 dev server

## 未来增强

- [ ] 历史回放功能（时间轴拖动）
- [ ] 3D 可视化（Three.js 球面文件树）
- [ ] 导出为视频/GIF
- [ ] 多螃蟹模式（多个 Claude Code 实例）
- [ ] 实时性能指标（操作耗时、tokens 消耗）

## 相关文档

- 计划文档：`docs/plans/claude-code-visualizer.md`
- API 实现：`web/app/api/visualizer/`
- 前端组件：`web/components/visualizer/`
- Hook 脚本：`~/.claude/hooks/visualizer-hook.sh`
- Hook 配置：`~/.claude/hooks.json`
