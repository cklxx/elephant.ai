# 积分与订阅系统设计概览
> Last updated: 2025-11-18


本节基于现有的用户认证服务，为“积分体系 + 两档月订阅”提供抽象化的领域模型与服务编排。目标是让后续的计费、配额或营销能力能够直接复用统一的数据结构与 API。

## 数据模型

| 实体 | 新增字段 | 说明 |
| ---- | -------- | ---- |
| `auth_users` | `points_balance BIGINT` | 用户积分余额，所有变更都通过服务层校验，保证不会出现负值或溢出。 |
| | `subscription_tier TEXT` | 订阅档位，允许值：`free` / `supporter` / `professional`。|
| | `subscription_expires_at TIMESTAMPTZ` | 订阅到期时间，仅在付费档位上使用，免费档位保持 `NULL`。|

### 订阅档位

| 档位 | 月费 (美元) | 用途 |
| ---- | ------------ | ---- |
| `free` | 0 | 默认体验，不附带订阅权益。|
| `supporter` | 20 | 入门付费档，可附加配额或特性。|
| `professional` | 100 | 高阶付费档，用于大规模或团队场景。|

`internal/auth/domain` 新增 `SubscriptionTier` 与 `SubscriptionPlan`，统一管理档位定义以及定价。任何需要展示或计算价格的模块都可以通过 `SubscriptionTier.Plan()` 获取统一的元数据。

## 服务能力

`auth.Service` 新增：

- `AdjustPoints(ctx, userID, delta)`：调整积分余额并防止负值或溢出，可供任务结算、营销赠送等场景调用。
- `UpdateSubscription(ctx, userID, tier, expiresAt)`：切换订阅档位，自动校验档位合法性，并确保付费档必须携带未来的到期时间。免费档调用时会清空到期时间。
- HTTP 层新增 `/api/auth/points` 与 `/api/auth/subscription`，要求登录态并复用以上服务方法，从而支持原型阶段的自助调试；`/api/auth/plans` 公开订阅目录供前端渲染选择器或说明文案。

所有写入操作都会刷新 `updated_at` 并走统一的仓储接口，确保内存版与 Postgres 版仓储行为一致。

## API 展现

后端 `/api/auth/*` 返回的用户对象新增：

```json
{
  "points_balance": 1200,
  "subscription": {
    "tier": "supporter",
    "monthly_price_cents": 2000,
    "expires_at": "2024-08-01T00:00:00Z"
  }
}
```

前端 `authClient` 会在登录、刷新、存储读取时统一归一化字段，保证历史 Session 也能升级到最新结构，同时暴露 `isPaid` 便于 UI 控制订阅权益。
账户下拉菜单会同步展示积分余额与当前订阅计划，并在付费档位时显示续订日期。

## 后续扩展点

1. **积分结算策略**：可在业务层实现积分获取／扣减的事件流（例如任务完成奖励、订阅赠送）。
2. **订阅续费任务**：利用现有的计划任务框架，以 `subscription_expires_at` 为触发条件自动降级或续费。
3. **前端展示与支付集成**：在 Header / 设置页展示当前积分与订阅状态，并接入支付网关完成扣款和续费。

## 剩余工作清单

- [ ] **支付与订阅扣款**：在 `internal/payments`（待建目录）中引入与支付网关的结算流程，实现支持者/专业版的订阅下单、回调校验以及失败重试，并在 `internal/auth/app.Service.UpdateSubscription` 中串联自动续费逻辑。
- [ ] **订阅到期自动降级**：基于 `cmd/alex-server` 现有的后台任务框架，新增计划任务扫描 `subscription_expires_at`，在超时后调用服务层降级为 `free` 档，并记录审计日志。
- [ ] **积分流水与配额联动**：在 `internal/auth` 之上扩展积分流水表（建议位于 `migrations/auth/002_points_ledger.sql`），保证每次调用 `AdjustPoints` 都会写入来源、用途与操作者信息，并与业务配额系统联动扣除积分。
- [ ] **前端订阅管理页面**：在 `web/app/settings/subscription`（待建）实现用户自助查看/升级/取消订阅的 UI，复用 `web/lib/auth/client` 的缓存刷新能力，并补充对应的 `npm test` 覆盖。
- [ ] **运营与风控报表**：补充 `docs/operations/` 下的运维手册，列出积分异常、订阅续费失败等告警与排查流程，同时在日志中补充结构化字段便于监控平台消费。
