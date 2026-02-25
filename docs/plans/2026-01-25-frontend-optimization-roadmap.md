# 前端优化实施路线图

**基于**: [frontend-best-practices-research.md](../frontend-best-practices-research.md)
**创建日期**: 2026-01-25
**负责人**: cklxx

---

## 执行摘要

本项目前端架构已达到行业领先水平，主要差距在可观测性和生产稳定性。优化分为 4 个阶段，总计 8-12 周完成核心优化。

---

## Phase 1: 可观测性基础 (Week 1-2)

### 目标
建立前端性能监控和错误追踪体系

### 任务清单

#### Task 1.1: Sentry 集成
- [ ] 创建 Sentry 项目 + 获取 DSN
- [ ] 安装依赖: `npm install @sentry/nextjs`
- [ ] 配置文件:
  ```bash
  touch sentry.client.config.ts
  touch sentry.server.config.ts
  ```
- [ ] 添加环境变量: `NEXT_PUBLIC_SENTRY_DSN`
- [ ] 配置 Source Maps 上传 (next.config.js)

**预计工时**: 1 天

---

#### Task 1.2: 自定义性能指标
- [ ] 实现 TTFT 追踪
  ```typescript
  // lib/analytics/performance.ts
  export function trackTTFT(sessionId: string, duration: number) {
    Sentry.setMeasurement('ttft', duration, 'millisecond');
  }
  ```
- [ ] SSE 连接监控
- [ ] 渲染帧率追踪
- [ ] 内存占用监控

**预计工时**: 2 天

---

#### Task 1.3: 错误分级处理
- [ ] 定义错误类型体系
  ```typescript
  // lib/errors/AppError.ts
  export class AppError extends Error {
    type: 'network' | 'auth' | 'validation' | 'fatal';
    recoverable: boolean;
    retryable: boolean;
  }
  ```
- [ ] 重构现有 ErrorBoundary
- [ ] 实现自动重试逻辑
- [ ] 配置 Sentry 错误分组

**预计工时**: 3 天

---

#### Task 1.4: 性能基准测试
- [ ] 使用 Lighthouse CI
- [ ] 设置性能预算
  - FCP < 1.5s
  - LCP < 2.5s
  - TTFT < 500ms
  - Bundle size < 500KB
- [ ] 配置 CI/CD 自动检测

**预计工时**: 2 天

---

### 验收标准
- ✅ Sentry 错误自动上报率 > 95%
- ✅ P99 TTFT < 1s
- ✅ SSE 连接成功率 > 99%
- ✅ 错误恢复率 > 60%

---

## Phase 2: 用户体验优化 (Week 3-5)

### 目标
提升感知性能和交互体验

### 任务清单

#### Task 2.1: 智能 Loading 状态
- [ ] 实现任务时长估算
  ```typescript
  // lib/estimator/taskDuration.ts
  export function estimateTaskDuration(
    taskType: string
  ): { p50: number; p90: number } {
    const history = getTaskHistory(taskType);
    return calculatePercentiles(history);
  }
  ```
- [ ] 添加 ETA 显示组件
- [ ] 优化骨架屏样式
- [ ] 添加进度条

**预计工时**: 3 天

---

#### Task 2.2: 流式渲染优化
- [ ] 实现 Token 批处理
  ```typescript
  // hooks/useTokenBatcher.ts
  export function useTokenBatcher(
    onFlush: (tokens: string[]) => void,
    batchSize = 20,
    flushInterval = 50
  ) { ... }
  ```
- [ ] 减少重排次数
- [ ] 优化虚拟滚动参数
- [ ] 添加渲染性能监控

**预计工时**: 4 天

---

#### Task 2.3: Bundle 优化
- [ ] 运行 Bundle Analyzer
  ```bash
  npm run build
  npm run analyze
  ```
- [ ] 代码分割优化
- [ ] 移除未使用依赖
- [ ] 压缩图片资源
- [ ] Tree shaking 检查

**预计工时**: 3 天

---

#### Task 2.4: 虚拟滚动调优
- [ ] 动态高度测量
- [ ] 智能 overscan 策略
- [ ] Intersection Observer 优化
- [ ] 滚动性能监控

**预计工时**: 2 天

---

### 验收标准
- ✅ ETA 估算误差 < 20%
- ✅ 渲染帧率 > 50 FPS
- ✅ Bundle size ↓ 15%
- ✅ 用户感知等待时间 ↓ 30%

---

## Phase 3: 功能完善 (Week 6-10)

### 目标
补齐离线支持和 A/B 测试能力

### 任务清单

#### Task 3.1: Service Worker 实现
- [ ] 注册 Service Worker
  ```typescript
  // public/sw.js
  self.addEventListener('install', event => { ... });
  self.addEventListener('fetch', event => { ... });
  ```
- [ ] 实现 Cache-First 策略
- [ ] 配置缓存白名单
- [ ] 添加更新提示 UI

**预计工时**: 5 天

---

#### Task 3.2: IndexedDB 缓存
- [ ] 实现 SessionCache 类
  ```typescript
  // lib/offline/SessionCache.ts
  export class SessionCache {
    async saveSession(session: SessionDetails);
    async getSession(sessionId: string);
    async listSessions();
  }
  ```
- [ ] 缓存会话数据
- [ ] 缓存事件历史
- [ ] 实现同步队列

**预计工时**: 4 天

---

#### Task 3.3: A/B 测试框架
- [ ] 实现特性开关
  ```typescript
  // lib/experiments/FeatureFlag.tsx
  export function FeatureFlag({
    feature,
    children,
    fallback
  }) { ... }
  ```
- [ ] 用户分组算法（一致性哈希）
- [ ] 配置管理界面
- [ ] Analytics 事件追踪

**预计工时**: 4 天

---

#### Task 3.4: 离线 UI 优化
- [ ] 离线状态指示器
- [ ] 同步进度显示
- [ ] 冲突解决 UI
- [ ] 离线模式切换开关

**预计工时**: 3 天

---

### 验收标准
- ✅ 离线可访问历史会话
- ✅ 网络恢复后自动同步
- ✅ A/B 测试支持百分比灰度
- ✅ 可动态调整特性开关

---

## Phase 4: 国际化 (Week 11-14)

### 目标
支持中英文切换

### 任务清单

#### Task 4.1: i18n 框架集成
- [ ] 安装 react-i18next
  ```bash
  npm install react-i18next i18next
  ```
- [ ] 配置 i18n
  ```typescript
  // lib/i18n/config.ts
  i18n.use(initReactI18next).init({ ... });
  ```
- [ ] 创建语言文件
  ```
  locales/
    en.json
    zh.json
  ```

**预计工时**: 3 天

---

#### Task 4.2: 文案提取翻译
- [ ] 提取所有硬编码文案
- [ ] 生成 en.json
- [ ] 翻译 zh.json
- [ ] 替换组件中的文案

**预计工时**: 5-7 天

---

#### Task 4.3: 本地化组件
- [ ] 语言切换器
- [ ] 日期时间本地化
- [ ] 数字货币格式化
- [ ] 相对时间显示

**预计工时**: 2 天

---

### 验收标准
- ✅ 支持中英文切换
- ✅ 无遗漏硬编码文案
- ✅ 日期时间正确本地化
- ✅ 加载性能无影响

---

## 关键风险与缓解

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|---------|
| Sentry 配额超限 | 高 | 中 | 设置采样率 (10%) |
| i18n 遗漏文案 | 中 | 高 | ESLint 规则检查 |
| 离线同步冲突 | 高 | 中 | CRDT 数据结构 |
| Bundle size 增加 | 中 | 中 | 严格 code review |

---

## 技术债务优先级

### 高优先级 (本季度解决)
1. ❌ 缺少前端性能监控
2. ❌ 错误处理不够细化
3. ⚠️ SSE 重连策略可优化

### 中优先级 (下季度解决)
4. ⚠️ 虚拟滚动可进一步优化
5. ❌ 缺少 A/B 测试框架
6. ⚠️ Markdown 渲染安全性待加强

### 低优先级 (长期规划)
7. ❌ 无国际化支持
8. ❌ 无离线支持

---

## 成功指标看板

### 性能指标
| 指标 | 当前值 | 目标值 | 进度 |
|------|--------|--------|------|
| TTFT (P99) | ~400ms | < 500ms | ✅ |
| 渲染帧率 | ~60 FPS | > 50 FPS | ✅ |
| Bundle Size | 未测量 | < 500KB | ⏳ |
| 错误率 | 未监控 | < 0.1% | ⏳ |

### 业务指标
| 指标 | 当前值 | 目标值 | 进度 |
|------|--------|--------|------|
| 错误恢复率 | ~40% | > 60% | ⏳ |
| 离线可用率 | 0% | 100% | ⏳ |
| 多语言覆盖 | 0% | 100% | ⏳ |

---

## 附录: 快速启动命令

### 开发环境
```bash
# 启动开发服务器
npm run dev

# 运行 Bundle Analyzer
npm run build && npm run analyze

# 运行性能测试
npx lighthouse http://localhost:3000 --view

# 运行 E2E 测试
npm run test:e2e
```

### 生产环境
```bash
# 构建
npm run build

# 启动生产服务器
npm start

# 验证 Service Worker
open http://localhost:3000
# DevTools > Application > Service Workers
```

---

## 参考文档

- [完整调研报告](../frontend-best-practices-research.md)
- [Sentry Next.js 集成](https://docs.sentry.io/platforms/javascript/guides/nextjs/)
- [react-i18next 文档](https://react.i18next.com/)
- [Service Worker API](https://developer.mozilla.org/en-US/docs/Web/API/Service_Worker_API)

---

**状态**: 📝 规划中
**下次更新**: Phase 1 完成后
