# Subagent 展示架构重构：锚点注入法

## 问题背景

### 症状
- 当有 subagent 执行结果时，subagent 卡片被放在最底部
- 主 agent 的事件出现在 subagent 卡片上方，而不是按触发顺序排列
- 时序混乱导致用户难以理解事件因果关系

### 根本原因分析

**旧架构的分区-合并模式问题** (`ConversationEventStream.tsx:316-441`):

```typescript
// 旧逻辑：将事件严格分区
function partitionEvents(events) {
  const displayEvents = [];      // 主 agent 事件
  const subagentThreads = [];    // subagent 事件线程

  events.forEach(event => {
    if (isSubagentLike(event)) {
      // subagent 事件进入独立线程
      subagentThreads.push(event);
    } else {
      displayEvents.push(event);
    }
  });

  return { displayEvents, subagentThreads };
}

// 合并时仅依赖时间戳
const combined = [...displayEvents, ...subagentThreads]
  .sort((a, b) => a.timestamp - b.timestamp);
```

**问题点**:
1. **时间戳不可靠**: subagent 和主 agent 的时间戳可能来自不同的时钟基准
2. **缺乏因果关联**: subagent 由其父 agent 的工具调用触发，但这种关系在分区后丢失
3. **排序粒度问题**: 全局时间戳排序无法反映"在此事件后插入子agent结果"的语义

## 解决方案：锚点注入法

### 核心思想
每个 subagent 线程绑定到一个"锚点事件"（触发它的工具调用），并插入到该锚点位置。

### 架构变更

```
旧数据流:
Events → partitionEvents → displayEvents + subagentThreads
                       ↓
              combinedEntries (按ts排序)
                       ↓
                 渲染为列表

新数据流:
Events → identifyAnchors → Map<anchorId, position>
                       ↓
              buildInterleavedEntries
                       ↓
    统一时间线: [event, event, subagentGroup@anchor2, event, ...]
                       ↓
                 渲染为列表
```

### 关键实现

1. **锚点识别** (`getSubagentAnchorId`):
```typescript
function getSubagentAnchorId(event): string | undefined {
  // Primary: call_id (if starts with "subagent")
  if (event.call_id?.startsWith("subagent")) {
    return `call:${event.call_id}`;
  }

  // Secondary: parent_task_id + task_id
  if (event.parent_task_id && event.task_id) {
    return `task:${event.parent_task_id}:${event.task_id}`;
  }

  // Tertiary: subtask_index
  if (event.parent_task_id && event.subtask_index !== undefined) {
    return `subtask:${event.parent_task_id}:${event.subtask_index}`;
  }
}
```

2. **锚点映射构建** (`buildAnchorMap`):
```typescript
function buildAnchorMap(events) {
  const map = new Map();
  events.forEach((event, index) => {
    if (isDelegationToolEvent(event)) {
      const anchorId = `call:${event.call_id}`;
      map.set(anchorId, { timestamp: event.timestamp, eventIndex: index });
    }
  });
  return map;
}
```

3. **交错时间线构建** (`buildInterleavedEntries`):
```typescript
function buildInterleavedEntries(displayEntries, subagentThreads, anchorMap) {
  const result = [];
  const inserted = new Set();

  displayEntries.forEach((entry, currentIndex) => {
    // 查找应在此位置后插入的 subagent 组
    const toInsert = subagentThreads.filter(thread => {
      if (thread.anchorEventId) {
        const anchor = anchorMap.get(thread.anchorEventId);
        return anchor && anchor.eventIndex <= currentIndex;
      }
      // Fallback to timestamp
      return thread.firstSeenAt <= entry.timestamp;
    });

    // 插入 subagent 组
    toInsert.forEach(group => {
      if (!inserted.has(group.key)) {
        inserted.add(group.key);
        result.push({ kind: "subagentGroup", ...group });
      }
    });

    // 添加当前主事件
    result.push(entry);
  });

  return result;
}
```

## 经验总结

### 设计原则
1. **因果关系优先于时间顺序**: 在分布式系统中，逻辑因果关系比物理时间戳更可靠
2. **显式关联优于隐式推断**: 通过锚点显式关联 subagent 与其触发事件
3. **降级策略**: 当锚点信息缺失时，优雅回退到时间戳排序

### 工程实践
- 保持 `partitionEvents` 接口不变，仅扩展返回值
- 单元测试覆盖锚点提取和排序逻辑
- 保留旧行为作为 fallback，确保向后兼容

### 文件变更
- `web/components/agent/ConversationEventStream.tsx`: 核心逻辑重构
- `web/components/agent/ConversationEventStream.test.ts`: 新增单元测试

## 相关链接
- 设计计划: `docs/plans/subagent-display-redesign.md`
