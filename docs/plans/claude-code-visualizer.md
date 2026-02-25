# Claude Code 可视化动画系统

**创建时间**: 2026-02-10 22:40
**更新时间**: 2026-02-11 00:30
**状态**: ✅ 已完成（核心功能）

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

### Phase 1: Hook 和服务搭建 ✅
- [x] 创建 Claude Code hook 脚本 (`~/.claude/hooks/visualizer-hook.sh`)
- [x] 创建 SSE 服务端点 (使用 SSE 替代 WebSocket，更简单)
- [x] 实现事件数据格式 (Zod 验证 + 去重)

### Phase 2: 基础可视化 ✅
- [x] 文件树组件 (使用 FolderTreemap 替代简单树，更美观)
- [x] 螃蟹角色 SVG (完整的螃蟹设计，带钳子和腿)
- [x] 基础移动动画 (CSS transition + 固定 viewport)

### Phase 3: 完整动画系统 ✅
- [x] 所有工具的动画映射 (Read/Write/Edit/Grep/Glob/Bash 等)
- [x] 气泡提示系统 (实时显示操作和文件名)
- [x] 思考状态动画 (螃蟹飘到顶部)
- [x] 平滑过渡效果 (duration-700 + ease-out)

### Phase 4: 增强和优化 🔄
- [ ] 历史回放功能 (未来迭代)
- [x] 性能优化 (事件去重、限制 200 条历史)
- [x] 美化和细节 (颜色、边框、阴影、动画关键帧)

## 测试结果

### ✅ 功能验证
- Hook 脚本正确解析 Claude Code 事件 (使用 jq 从 stdin 解析 JSON)
- API 端点接收并存储事件 (Zod 验证 + 去重)
- SSE 流实时推送事件到前端 (30 秒心跳)
- FolderTreemap 动态更新活跃文件夹
- 螃蟹移动到目标文件夹位置
- 气泡显示正确的工具和文件名
- 思考状态时螃蟹飘到顶部
- 事件日志正确显示历史记录

### 🧪 端到端测试
```bash
# 1. 启动开发服务器
cd web && PORT=3002 npm run dev

# 2. 发送测试事件
cat test-event.json | VISUALIZER_URL=http://localhost:3002/api/visualizer/events \
  ~/.claude/hooks/visualizer-hook.sh

# 3. 访问可视化界面
open http://localhost:3002/visualizer
```

测试通过：事件从 hook → API → SSE → 前端流畅传递，螃蟹动画正常显示。

## 已知问题

1. **FolderTreemap 初始加载较慢** (扫描整个项目需 1-2 秒)
   - 缓解：已限制扫描深度 (默认 3 层)
   - 未来：添加 loading 状态和增量加载

2. **大量事件时可能卡顿** (未测试超过 1000 个事件)
   - 缓解：限制历史 200 条 + 事件去重
   - 未来：虚拟化列表渲染

3. **螃蟹定位依赖 DOM 查询** (可能有小的偏移)
   - 当前：使用固定 viewport 减少滚动影响
   - 未来：使用 Canvas 坐标系统

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
