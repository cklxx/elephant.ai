# Claude Code 可视化动画系统

**创建时间**: 2026-02-10 22:40
**状态**: 进行中

## 目标

创建一个动画界面，实时可视化 Claude Code 的工作过程：
- CC 以小螃蟹形象展示
- 显示文件树结构（文件夹和文件）
- 螃蟹在文件间移动表示正在操作
- 头上气泡显示当前动作和文件名
- 思考时螃蟹站得高高的，离开文件区域
- 可视化所有工具调用（Read、Write、Edit、Grep、Glob、Bash 等）

## 架构设计

### 1. Hook 数据采集层
**文件**: `~/.claude/hooks/visualizer-hook.sh`
- 捕获所有工具调用事件
- 解析事件数据（工具类型、文件路径、操作内容）
- 通过 WebSocket/HTTP 发送到可视化服务

### 2. 可视化服务层
**路径**: `web/app/visualizer/`
- WebSocket 服务接收 hook 数据
- 维护项目文件树状态
- 广播事件给前端

### 3. 前端展示层
**技术栈**: Next.js + Canvas/SVG + Framer Motion
- 文件树可视化（D3.js 或 React Flow）
- 螃蟹角色动画
- 气泡提示
- 实时事件流

## 组件设计

### 工具到动画的映射

| 工具 | 动画效果 | 图标 |
|------|---------|-----|
| Read | 螃蟹爬到文件上，眼睛发光读取 | 📖 |
| Write | 螃蟹在文件上挥舞钳子写入 | ✍️ |
| Edit | 螃蟹用钳子精准修改 | ✏️ |
| Grep | 螃蟹快速扫描多个文件 | 🔍 |
| Glob | 螃蟹在文件夹间跳跃搜索 | 🗂️ |
| Bash | 螃蟹进入终端区域敲命令 | 💻 |
| WebFetch | 螃蟹伸出长钳子到云端抓取 | 🌐 |
| AskUserQuestion | 螃蟹停下，头上问号气泡 | ❓ |
| Thinking | 螃蟹飘到高处，思考气泡 | 💭 |

## 实现步骤

### Phase 1: Hook 和服务搭建
- [ ] 创建 Claude Code hook 脚本
- [ ] 创建 WebSocket 服务端点
- [ ] 实现事件数据格式

### Phase 2: 基础可视化
- [ ] 文件树组件
- [ ] 螃蟹角色 SVG
- [ ] 基础移动动画

### Phase 3: 完整动画系统
- [ ] 所有工具的动画映射
- [ ] 气泡提示系统
- [ ] 思考状态动画
- [ ] 平滑过渡效果

### Phase 4: 增强和优化
- [ ] 历史回放功能
- [ ] 性能优化
- [ ] 美化和细节

## 技术细节

### Hook 事件格式
```json
{
  "timestamp": "2026-02-10T22:40:00Z",
  "event": "tool_use",
  "tool": "Read",
  "path": "/Users/bytedance/code/elephant.ai/internal/agent/react.go",
  "status": "started|completed|error",
  "details": {}
}
```

### WebSocket 协议
- 端点: `ws://localhost:3002/api/visualizer`
- 心跳: 每 30s
- 自动重连

## 参考资源

- [Claude Code Hooks Guide](https://code.claude.com/docs/en/hooks-guide)
- [Hooks Reference](https://docs.claude.com/en/docs/claude-code/hooks)
- [Claude Code Hooks Mastery](https://github.com/disler/claude-code-hooks-mastery)
