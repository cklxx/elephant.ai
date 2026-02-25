# Claude Code Visualizer - 快速开始

## 🚀 启动步骤

### 1. 启动开发服务器

```bash
cd web
PORT=3002 npm run dev
```

等待服务器启动完成，看到：
```
✓ Ready in 1803ms
- Local: http://localhost:3002
```

### 2. 打开可视化界面

在浏览器打开：**http://localhost:3002/visualizer**

你应该看到：
- ✅ 顶部显示"已连接"（绿点）
- ✅ 代码库文件夹自动显示（灰色卡片）
- ✅ 工作目录路径显示在顶部

### 3. 发送测试事件

**方法 A：使用测试脚本（推荐）**

在项目根目录打开新终端：

```bash
./scripts/test-visualizer.sh
```

你会看到螃蟹开始移动，文件夹变色！🦀

**方法 B：手动发送单个事件**

```bash
echo '{
  "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
  "event": "tool-use",
  "tool": "Read",
  "path": "/Users/bytedance/code/elephant.ai/README.md",
  "status": "started",
  "details": {}
}' | curl -s -X POST "http://localhost:3002/api/visualizer/events" \
  -H "Content-Type: application/json" -d @-
```

### 4. 使用真实的 Claude Code

**注意**：Claude Code hooks 只在 Claude Code **运行时**才会触发事件。

在任意项目中启动 Claude Code：

```bash
claude-code
```

然后执行一些操作：
```
> Read the README.md file
> Search for "function" in src/
> List all Go files
```

回到浏览器，你应该看到实时更新！

## 🔍 调试检查清单

### ✅ 检查 1：开发服务器运行正常

```bash
curl http://localhost:3002/api/visualizer/events
# 应该返回 JSON: {"events": [...], "count": N}
```

### ✅ 检查 2：文件夹扫描成功

```bash
curl http://localhost:3002/api/visualizer/folders
# 应该返回 JSON: {"workspace": "...", "folders": [...]}
```

### ✅ 检查 3：SSE 连接工作

在浏览器开发者工具：
1. 打开 Network 标签
2. 筛选 "stream"
3. 应该看到 `/api/visualizer/stream` 持续连接（Status: 200, Type: eventsource）

### ✅ 检查 4：Hook 脚本可执行

```bash
ls -la ~/.claude/hooks/visualizer-hook.sh
# 应该显示 -rwxr-xr-x (可执行权限)

# 手动测试 hook
echo '{"hook_event_name":"tool-use","tool_name":"Read","tool_input":{"file_path":"/test.md"}}' | \
  ~/.claude/hooks/visualizer-hook.sh

# 检查事件是否到达
curl -s http://localhost:3002/api/visualizer/events\?limit\=1 | jq .
```

### ✅ 检查 5：前端控制台无错误

打开浏览器开发者工具 Console 标签，应该看到：
```
[FolderMap] Loaded 23 folders from /Users/...
[VisualizerStream] Connected
```

## 🐛 常见问题

### 问题：页面显示"等待 Claude Code 活动"

**原因**：没有收到事件

**解决**：
1. 运行测试脚本：`./scripts/test-visualizer.sh`
2. 刷新页面（Cmd+R / Ctrl+R）
3. 检查浏览器 Console 是否有错误

### 问题：文件夹不显示或显示"正在扫描代码库"

**原因**：API 扫描失败或超时

**解决**：
1. 检查 `/api/visualizer/folders` 是否返回数据
2. 查看服务器日志：`tail -f /tmp/visualizer-dev.log`
3. 确认工作目录有代码文件

### 问题：SSE 显示"未连接"

**原因**：SSE 连接失败

**解决**：
1. 确认开发服务器运行在 `localhost:3002`
2. 检查浏览器 Network 标签是否有 CORS 错误
3. 尝试刷新页面

### 问题：Claude Code hooks 不触发

**原因**：Hooks 只在 Claude Code **运行时**触发，不是在 IDE 中

**解决**：
1. 确认 `~/.claude/hooks.json` 配置正确
2. 使用 `claude-code` CLI 工具（不是 VSCode 插件）
3. 或使用测试脚本模拟事件

## 📊 验证成功的标志

当一切正常工作时，你应该看到：

1. ✅ **顶部**: "已连接" + 绿色圆点
2. ✅ **文件夹区**: 显示代码库所有文件夹（灰色）
3. ✅ **运行测试后**: 文件夹变色（蓝色/紫色），螃蟹移动
4. ✅ **右侧事件日志**: 显示所有工具调用记录
5. ✅ **底部统计**: 显示事件总数、活跃文件夹数

## 🎬 演示命令

一个完整的演示流程：

```bash
# 终端 1：启动服务器
cd web && PORT=3002 npm run dev

# 终端 2：发送测试事件
./scripts/test-visualizer.sh

# 浏览器：打开 http://localhost:3002/visualizer
# 观察螃蟹动画和文件夹变色 🦀✨
```

## 📖 更多信息

- 完整文档：`VISUALIZER_README.md`
- 实现记录：`docs/records/2026-02-10-claude-code-visualizer-implementation.md`
- Hook 脚本：`~/.claude/hooks/visualizer-hook.sh`
- Hook 配置：`~/.claude/hooks.json`

---

**享受可视化 Claude Code 的工作过程！** 🦀✨
