# 前端重构 - Manus 风格终端布局

## 问题分析

### 原始问题
1. **输入框消失**: 提交任务后，输入框被隐藏 (`!sessionId && !currentSessionId` 条件)
2. **布局不连贯**: 有/无 session 时布局差异巨大
3. **非终端体验**: 不像持续性对话界面
4. **复杂状态管理**: sessionId、taskId、currentSessionId 多重状态

### 根本原因
```typescript
// 旧代码 - page.tsx:109-120
{!sessionId && !currentSessionId && (
  <div className="mb-6">
    <TaskInput ... />  // ❌ 条件渲染导致输入框消失
  </div>
)}
```

## Manus 设计原则

### 核心理念
1. **持续性输入** - 输入框永远固定在底部
2. **流式输出** - 事件流在上方滚动显示
3. **极简主义** - 无冗余装饰，专注内容
4. **单一焦点** - 清晰的任务流
5. **空间效率** - 充分利用垂直空间

### 布局结构
```
┌───────────────────────────────────────┐
│ Header (极简)                         │
│  ALEX  session-id  [Clear]           │
├───────────────────────────────────────┤
│                                       │
│ Output Area (flex-1, overflow-auto)   │
│  Terminal-style event stream          │
│  Auto-scroll to bottom                │
│  Monospace font                       │
│                                       │
├───────────────────────────────────────┤
│ Input Area (固定底部, 始终可见)       │
│  [textarea]  [Send]                   │
│  Enter to send · Shift+Enter newline │
└───────────────────────────────────────┘
```

## 重构实现

### 1. 主页面 (`app/page.tsx`)

**核心变化:**
```typescript
// 新布局 - Flexbox 三段式
<div className="flex flex-col h-[calc(100vh-8rem)]">
  {/* Header - 固定高度 */}
  <div className="flex-shrink-0 pb-3 mb-3 border-b">...</div>

  {/* Output - 弹性高度，可滚动 */}
  <div ref={outputRef} className="flex-1 overflow-y-auto mb-4">
    {events.length === 0 ? <EmptyState /> : <TerminalOutput />}
  </div>

  {/* Input - 固定底部，始终可见 */}
  <div className="flex-shrink-0 border-t pt-3">
    <TaskInput
      onSubmit={handleTaskSubmit}
      placeholder={sessionId ? "Continue..." : "Describe task..."}
    />
  </div>
</div>
```

**关键特性:**
- ✅ 输入框永远可见 (无条件渲染)
- ✅ 自动滚动到底部 (useEffect + scrollHeight)
- ✅ 状态简化 (只需 sessionId + taskId)
- ✅ 清除功能 (Clear button)

### 2. 终端输出 (`components/agent/TerminalOutput.tsx`)

**新建组件，终端风格:**
```typescript
export function TerminalOutput({ events, ...props }) {
  return (
    <div className="space-y-2 font-mono text-xs">
      {events.map((event, idx) => (
        <EventLine key={idx} event={event} />
      ))}
    </div>
  );
}

function EventLine({ event }) {
  // 时间戳 + 彩色事件类型 + 内容
  return (
    <div className="flex gap-3 hover:bg-muted/30">
      <span className="text-muted-foreground/50">{timestamp}</span>
      <span className={getEventStyle()}>{formatContent()}</span>
    </div>
  );
}
```

**事件类型样式:**
- `task_started` - 绿色
- `task_completed` - 粗体绿色 + ✓
- `task_failed` - 红色 + ✗
- `plan_created` - 蓝色
- `tool_call` - 青色 + ▸
- `tool_result` - 青色 + ✓/✗
- `thinking` - 紫色 + 💭
- `step_start/complete` - 黄色

**内容格式化:**
```typescript
// tool_call
▸ file_read(path: src/main.go)

// tool_result
✓ file_read → package main\nfunc main() {...

// step_start
→ Step 1: Analyzing codebase structure

// thinking
💭 I need to first understand the project...
```

### 3. 输入组件 (`components/agent/TaskInput.tsx`)

**简化设计:**
```typescript
export function TaskInput({ onSubmit, loading, placeholder }) {
  return (
    <form>
      <div className="flex gap-2 items-end">
        {/* 自动高度 textarea */}
        <textarea
          ref={textareaRef}
          rows={1}
          className="flex-1 min-h-[2.5rem] max-h-32"
          style={{ fieldSizing: 'content' }}
        />

        {/* 发送按钮 */}
        <button type="submit" className="h-10">
          <Send /> Send
        </button>
      </div>

      {/* 提示文本 */}
      <div className="text-xs text-muted-foreground">
        Enter to send · Shift+Enter for new line
      </div>
    </form>
  );
}
```

**特性:**
- ✅ 自动高度调整 (useEffect)
- ✅ 横向布局 (flex gap-2)
- ✅ 最大高度限制 (max-h-32)
- ✅ Enter 发送, Shift+Enter 换行
- ✅ 加载状态动画

## 技术细节

### 自动滚动
```typescript
const outputRef = useRef<HTMLDivElement>(null);

useEffect(() => {
  if (outputRef.current) {
    outputRef.current.scrollTop = outputRef.current.scrollHeight;
  }
}, [events]); // 新事件时自动滚动
```

### 自动调整高度
```typescript
const textareaRef = useRef<HTMLTextAreaElement>(null);

useEffect(() => {
  if (textareaRef.current) {
    textareaRef.current.style.height = 'auto';
    textareaRef.current.style.height = textareaRef.current.scrollHeight + 'px';
  }
}, [task]);
```

### Plan 审批集成
```typescript
// 从事件流中提取 plan 状态
const { planState, currentPlan } = useMemo(() => {
  const lastPlanEvent = [...events]
    .reverse()
    .find(e => e.event === 'plan_created' || e.event === 'plan_approved');

  return {
    planState: lastPlanEvent?.event === 'plan_created' ? 'awaiting_approval' : 'approved',
    currentPlan: lastPlanEvent?.data?.plan || null,
  };
}, [events]);

// 条件渲染 Plan 卡片
{planState === 'awaiting_approval' && currentPlan && (
  <ResearchPlanCard
    plan={currentPlan}
    onApprove={handleApprove}
    onReject={handleReject}
  />
)}
```

## 样式规范

### Tailwind 类名约定
```css
/* Layout */
flex flex-col h-[calc(100vh-8rem)]  /* 全高布局 */
flex-shrink-0                        /* 固定高度区域 */
flex-1 overflow-y-auto               /* 弹性滚动区域 */

/* Typography */
font-mono text-xs                    /* 等宽小字体 */
text-muted-foreground/50             /* 半透明次要文字 */
tracking-tight                       /* 紧凑字间距 */

/* Spacing */
space-y-2                            /* 垂直间距 */
gap-3                                /* Flex 间距 */
pb-3 mb-3                            /* Padding + Margin */

/* Borders */
border-b border-border/50            /* 50%透明边框 */

/* Interactive */
hover:bg-muted/30                    /* 悬浮背景 */
transition-colors                    /* 平滑过渡 */
```

### 颜色系统
```typescript
// 连接状态
isConnected ? 'bg-green-500' : 'bg-gray-400'

// 事件类型
'text-green-600'   // 成功/开始
'text-red-500'     // 错误/失败
'text-blue-600'    // Plan 相关
'text-cyan-600'    // 工具调用
'text-purple-600'  // 思考
'text-yellow-600'  // 步骤
```

## 对比总结

### 旧版本
- ❌ 输入框条件渲染，提交后消失
- ❌ 布局分散，状态切换明显
- ❌ AgentOutput + ManusAgentOutput 两套实现
- ❌ 复杂的状态管理逻辑
- ❌ 卡片式布局，非终端风格

### 新版本
- ✅ 输入框固定底部，始终可见
- ✅ 统一三段式布局，平滑体验
- ✅ TerminalOutput 单一实现
- ✅ 简化状态管理
- ✅ 终端风格，紧凑高效

## 使用方式

### 本地开发
```bash
./deploy.sh
# 访问 http://localhost:3000
```

### 输入任务
1. 在底部输入框输入任务描述
2. 按 Enter 发送 (或点击 Send 按钮)
3. 事件流在上方实时显示
4. 输入框保持可见，可继续对话

### 清除会话
点击右上角 "Clear" 按钮重置会话

## 文件变更

### 新增文件
- `web/components/agent/TerminalOutput.tsx` - 终端风格输出组件

### 修改文件
- `web/app/page.tsx` - 主页面布局重构
- `web/components/agent/TaskInput.tsx` - 输入组件简化

### 保留文件 (待清理)
- `web/components/agent/AgentOutput.tsx` - 旧版输出
- `web/components/agent/ManusAgentOutput.tsx` - 旧版 Manus 输出

## 后续优化

### 性能优化
- [ ] 虚拟滚动 (react-window) 处理大量事件
- [ ] 事件去重和合并
- [ ] Debounce 自动滚动

### 功能增强
- [ ] 事件搜索/过滤
- [ ] 导出事件日志
- [ ] 快捷键支持 (Ctrl+K 清除等)
- [ ] 多会话切换

### 视觉优化
- [ ] 语法高亮 (代码块)
- [ ] 事件展开/折叠
- [ ] Dark mode 优化
- [ ] 自定义配色方案

## 测试清单

- [x] 输入框始终可见
- [x] 提交任务后输入框不消失
- [x] 事件流正确显示
- [x] 自动滚动到底部
- [x] Enter 发送, Shift+Enter 换行
- [x] 清除按钮工作
- [x] Plan 审批卡片显示
- [ ] 长时间运行任务测试
- [ ] 大量事件性能测试
- [ ] 移动端响应式测试

## 结论

通过采用 Manus 的终端风格布局，我们实现了:
1. **更好的用户体验** - 输入框始终可见，像真正的终端
2. **更简洁的代码** - 减少条件渲染，统一状态管理
3. **更高效的空间利用** - Flexbox 布局充分利用屏幕高度
4. **更清晰的信息流** - 时间戳 + 彩色事件类型，易于扫描

这是一个重大的前端架构改进，为后续功能扩展打下了坚实基础。
