# Feishu CLI Canonical Surface (Draft)

日期：2026-03-06  
状态：Draft / implementation-ready

## 目标

把面向 LLM 的飞书对象操作统一收敛到一个 canonical entrypoint：`feishu-cli`。

原则：
- 保留 Lark/Feishu Go delivery 层作为消息入口、会话绑定、事件回调基础设施。
- 把 LLM 可调用的对象操作统一抽象为 CLI contract，而不是继续暴露碎片化 Go tool action matrix。
- product-facing 能力域只有一个入口：`feishu-cli`。

## 分层边界

### 保留在 Go delivery / channel 层
- webhook / callback
- chat routing
- session binding
- streaming delivery
- progress listener
- reaction / message ingress

### 收敛到 Feishu CLI 层
- communicate: send message / upload file / read history
- schedule: query/create/update event, list rooms/meetings
- task: list/create/update/delete task, subtask
- document: create/read/update doc blocks, write markdown
- knowledge: wiki space/node read/create
- data: bitable tables/records/fields
- drive: list/copy/delete files, folders
- org: contacts / departments / mailgroups

## 推荐命令模型

对用户/LLM 只讲 job-to-be-done，不讲 OpenAPI 分类。

### L1: 能力域
- `alex feishu communicate ...`
- `alex feishu schedule ...`
- `alex feishu task ...`
- `alex feishu document ...`
- `alex feishu knowledge ...`
- `alex feishu data ...`
- `alex feishu drive ...`
- `alex feishu org ...`

### L2: 具体动作
示例：
- `alex feishu communicate send --chat current --text '...'`
- `alex feishu schedule query --range today`
- `alex feishu document write --doc <url-or-token> --markdown-file /tmp/x.md`
- `alex feishu task create --title '...' --due tomorrow`

## LLM 披露策略

skill 名称：`feishu-cli`

skill 里只暴露：
- 什么时候该用它
- 哪些高层能力可用
- 常见命令模式
- 审批/确认边界

不暴露：
- 零散 channel.action matrix 细节
- 不必要 token/id 细节

## 审批与风险

- read-only：默认可直接执行
- reversible write：允许执行，但保留 approval hook
- irreversible / external blast-radius 大的操作：显式确认

## 迁移顺序

### Phase 1
先做 CLI façade + skill 文档：
- 命令内部先复用现有 Go infra client
- 不先改 delivery 层

### Phase 2
让 LLM prompt / docs / examples 优先走 `feishu-cli`

### Phase 3
逐步把旧 channel action matrix 从“主叙事”降级为“兼容实现”

## 验收标准

- 新增 Feishu 能力时，优先新增 CLI 子命令和 skill 文案，而不是新增 prompt 里的散乱 action。
- 产品文档把 `feishu-cli` 作为唯一 canonical entrypoint。
- LLM 对飞书操作的首选路径稳定收敛到 CLI contract。

