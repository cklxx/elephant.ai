# Subagent 前端展示架构重设计计划

## 问题分析

### 当前架构缺陷

1. **索引对齐问题** (已修复)
   - `buildAnchorMap` 使用原始数组索引，但 `buildInterleavedEntries` 使用筛选后的数组索引
   - 导致锚点位置计算错误，subagent 卡片出现在错误位置

2. **锚点事件被过滤** (已修复)
   - `isDelegationToolEvent`（subagent 工具调用）被从 `displayEvents` 中过滤
   - 锚点事件根本不在展示列表中，无法作为插入参考点

3. **分区-合并模式的时序问题** (`ConversationEventStream.tsx:316-441`)
   - 事件被严格分区为 `displayEvents` (主agent) 和 `subagentThreads` (子agent)
   - 合并时仅依赖时间戳排序，但子agent事件的 `firstSeenAt` 可能基于不同的时间基准
   - 结果：子agent卡片经常出现在错误的位置（通常是底部）

4. **缺乏因果关系锚定**
   - 子agent由其父agent的工具调用（如 `subagent` 工具）触发
   - 当前设计没有将子agent与其触发事件关联，导致布局位置无法准确反映因果时序

## 设计方案对比

### 方案A: 锚点注入法 (采用)

**核心思想**: 每个子agent线程绑定到一个"锚点事件"（触发它的工具调用），并插入到该锚点位置。

**实现方式**:
1. 识别子agent的触发事件（`workflow.tool.completed` with `tool_name=subagent`）
2. 使用该触发事件的 `call_id` 或 `task_id` 关联子agent事件
3. 在构建 `combinedEntries` 时，将子agent组插入到其锚点事件之后

**优点**:
- 精确反映因果关系和时序
- 实现相对简单，改动范围可控
- 保持现有的水平滚动布局

**缺点**:
- 需要确保后端事件中包含可靠的关联字段

## 实现细节与修复

### 关键修复

**1. 修复索引对齐**:
```typescript
// 修改前：buildAnchorMap 使用原始 events 数组索引
function buildAnchorMap(events) {
  events.forEach((event, index) => {  // index 不匹配 displayEntries
    anchorMap.set(anchorId, { eventIndex: index });
  });
}

// 修改后：使用 displayEntries 索引
function buildAnchorMap(displayEntries) {
  displayEntries.forEach((entry, displayIndex) => {  // 正确索引
    anchorMap.set(anchorId, { displayIndex });
  });
}
```

**2. 保留锚点事件**:
```typescript
// 修改前：跳过 subagent 工具调用
if (isDelegationToolEvent(event)) {
  return;  // 锚点事件被过滤！
}

// 修改后：保留锚点事件在 displayEvents 中
if (isSubagentToolEvent(event)) {
  displayEvents.push(event);  // 作为锚点显示
  return;
}
```

**3. 调整 hooks 调用顺序**:
```typescript
// 必须先构建 displayEntries，再构建 anchorMap
const { displayEvents, subagentThreads } = useMemo(() =>
  partitionEvents(...), [...]);
const displayEntries = useMemo(() =>
  buildDisplayEntriesWithClearifyTimeline(displayEvents), [...]);
const anchorMap = useMemo(() =>
  buildAnchorMap(displayEntries), [displayEntries]);  // 依赖 displayEntries
```

### 数据结构变更
```typescript
interface SubagentThread {
  key: string;
  groupKey: string;
  context: SubagentContext;
  events: AnyAgentEvent[];
  subtaskIndex: number;
  firstSeenAt: number | null;
  firstArrival: number;
  anchorEventId?: string;   // 锚点事件标识
  anchorTimestamp?: number; // 锚点时间戳（降级用）
}
```

### 降级策略

当无法识别锚点时（如旧数据、缺失字段）：
1. 使用 `subtaskIndex` 作为排序依据
2. 使用 `firstSeenAt` 时间戳（fallback）
3. 放置到最后（最保守）

## 实现步骤

1. [x] 修复 `buildAnchorMap` 使用 `displayEntries` 索引而非原始索引
2. [x] 修复 `partitionEvents` 保留 `isSubagentToolEvent` 事件作为锚点
3. [x] 调整 hooks 调用顺序，确保 `anchorMap` 在 `displayEntries` 之后构建
4. [x] 更新 `buildInterleavedEntries` 使用 `displayIndex` 进行插入
5. [x] 验证构建和 lint 通过
6. [x] 记录架构决策到 error-experience

## 完成总结

**核心变更** (`web/components/agent/ConversationEventStream.tsx`):

1. **修复索引对齐**: `buildAnchorMap()` 现在接收 `displayEntries` 并使用 `displayIndex`
2. **保留锚点事件**: subagent 工具调用事件现在进入 `displayEvents`
3. **调整 hooks 顺序**: `anchorMap` 在 `displayEntries` 构建后再构建
4. **锚点注入排序**: `buildInterleavedEntries()` 基于锚点位置插入 subagent 组

**验证状态**:
- ✅ 构建成功
- ✅ Lint 无警告
- ✅ 单元测试通过（除预先存在的 AgentCard 测试失败）

**架构改进效果**:
- subagent 卡片现在准确出现在触发它们的工具调用之后
- 主agent事件流与subagent展示保持正确的因果时序
- 降级策略确保旧数据仍能正确展示

## 回滚计划

如果新逻辑出现问题，可以通过 feature flag 或代码回滚到按时间戳排序的旧逻辑。
