# 2026-02-08 Foundation Offline Eval Report

## 1. 目标与方法

本轮评测目标：在**不依赖模型调用**前提下，评估基础能力是否达标，重点看：
- 提示词写得是否清晰可执行（Prompt Quality）
- 工具定义是否可用（Tool Usability）
- 工具是否容易被发现（Tool Discoverability）
- 在不显式点名工具时，系统是否能“推断并命中”正确工具（Implicit Tool-Use Readiness）

评测实现：`alex eval foundation`（离线 deterministic scoring）。
场景集：`evaluation/agent_eval/datasets/foundation_eval_cases.yaml`，共 47 个隐式意图 case。

## 2. 运行矩阵（更大规模）

共跑 7 组配置（47 case/组，合计 329 case）。

| Run | Mode | Preset | Toolset | Top-K | Overall | Prompt | Usability | Discoverability | Implicit Top-1 | Implicit Top-K |
|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|
| web-full | web | full | default | 3 | 95.7 | 98.0 | 99.7 | 94.5 | 59.6% | 89.4% |
| web-architect | web | architect | default | 3 | 95.7 | 98.0 | 99.7 | 94.5 | 59.6% | 89.4% |
| web-lark-local | web | lark-local | lark-local | 3 | 90.6 | 98.0 | 97.8 | 90.6 | 46.8% | 74.5% |
| web-full-top5 | web | full | default | 5 | 97.8 | 98.0 | 99.7 | 94.5 | 59.6% | 97.9% |
| cli-full | cli | full | default | 3 | 85.2 | 98.0 | 97.3 | 88.5 | 38.3% | 55.3% |
| cli-safe | cli | safe | default | 3 | 85.6 | 98.0 | 97.9 | 89.7 | 38.3% | 55.3% |
| cli-read-only | cli | read-only | default | 3 | 85.8 | 98.0 | 98.3 | 89.9 | 40.4% | 55.3% |

结论：
- Web 面可发现性明显更好（Top-K 89.4%），CLI 面显著偏低（Top-K 55.3%）。
- Top-K 从 3 提升到 5 后，`web/full` 从 89.4% -> 97.9%，说明大量失败是“排位差一位/两位”，不是完全不可发现。

## 3. 失败 Case 拆解

### 3.1 `web/full` 失败（5/47）

类别分布：`artifact=2`、`browser=1`、`clarification=1`、`planning=1`。

典型失败：
1. `intent-plan-migration`（expected: `plan`）
- 现象：Top-3 中未出现 `plan`，reason: lexical overlap 不足。
- 根因：意图句只写“phase/milestone/checkpoint/risks”，未覆盖 `plan` 的高权重词。

2. `intent-browse-dom-form`（expected: `browser_dom`）
- 现象：`web_search`、`web_fetch` 排在前面，`browser_dom` 排第 4。
- 根因：描述里“webpage/form/selectors/submit”同时触发 web 工具与 browser 工具，缺少“DOM/selector action”差异化词权重。

3. `intent-list-report-artifacts`（expected: `artifacts_list`）
- 现象：`artifact_manifest` / `artifacts_write` 排更前，`artifacts_list` 第 5。
- 根因：`list` 与 `manifest` 在当前词权重里区分度不够。

4. `intent-clarify-ambiguous-input`（expected: `clarify`）
- 现象：`diagram_render`、`plan` 等噪声项在前，`clarify` 第 4。
- 根因：`clarify` 的语义词池偏窄，且“ambiguous/blocking detail”未被充分映射。

5. `intent-emit-ui-payload`（expected: `a2ui_emit`）
- 现象：`plan`、`clarify`、`request_user`在前，`a2ui_emit` 第 4。
- 根因：`a2ui_emit` 的 discoverability 词（payload/render/protocol）不够突出。

### 3.2 `cli/full` 失败（21/47）

类别分布：`artifact=6`、`workspace=5`、`browser=4`、`execution=2`、`memory=2`、`orchestration=1`、`planning=1`。

关键现象：
- 大量失败集中在 alias 类工具：`read_file/write_file/list_dir/search_file/replace_in_file/shell_exec/execute_code`。
- 这些在 `cli/default` 下并非主要工具名（CLI 更偏 `file_read/file_write/list_files/ripgrep/bash/code_execute`）。

根因：
- 场景集混入了 lark-local alias 预期，和 `cli/default` 工具面不对齐，导致系统性 false negative。
- 这是**评测集与配置面不一致**问题，不是纯 discoverability 差。

### 3.3 `web/lark-local` 失败（12/47）

失败集中于 `artifact/workspace/orchestration`：
- `acp_executor`、`artifacts_*`、`write_attachment` 在该 preset/toolset 下不可用或被限制。
- 同时存在 4~6 名“排位失败”现象（例：`read_file` 第 6，`browser_dom` 第 5）。

根因：
- policy/preset 限制导致 expected tool 不在候选集中。
- 需要做 “scenario gating by allowed tools”。

## 4. 成功 Case 原因

高置信成功的共同特征：
- 意图词与工具名/描述强对齐（动词+对象）
- 工具名具备明确 action-object 结构
- 工具描述语义稠密，包含场景词

示例（`web/full`）：
1. `intent-fetch-url-content` -> `web_fetch(27.4)`
- 原因：`full/page/content/web` 与 `web_fetch` 描述强重叠。

2. `intent-lark-send-update` -> `lark_send_message(30.8)`（cli/full）
- 原因：`send/update/message/lark/chat` 完整匹配。

3. `intent-build-ppt-from-images` -> `pptx_from_images(22.3)`
- 原因：`slide/deck/image` 与工具语义高度专一。

4. `intent-cancel-reminder` -> `cancel_timer(18.5)`
- 原因：`cancel + active + timer` 明确且无歧义。

## 5. 工具可用性/可发现性问题画像

跨运行最常见问题（issue breakdown Top）：
1. `metadata_tags_missing`
2. `semantic_tokens_sparse`
3. `property_description_thin`
4. `name_not_action_object`（在 CLI 更突出）

说明：
- 可用性总体高（所有 run `pass_rate=100%`，critical=0）。
- 短板主要在“可发现性语义层”，尤其 tags 与描述词稀疏。

## 6. 提示词评测结论

Prompt 平均分 98，整体稳定。
弱点集中在：
- `preset/code-expert`
- `preset/devops`

两者共同 gap：`缺少隐式工具发现提示`。

## 7. 如何解决（按优先级）

### P0: 修评测口径一致性（先防误判）
1. 为每个 mode/preset 自动过滤 scenario：
- 若 `expected_tools` 不在当前可用工具集合，标记为 `N/A (policy gated)`，不计失败。
2. 拆分场景集：
- `foundation_eval_cases.web.yaml`
- `foundation_eval_cases.cli.yaml`
- `foundation_eval_cases.lark_local.yaml`

预期收益：CLI Top-K 有望从 ~55% 拉升到 >75%（去掉不适配 expected）。

### P1: 提升可发现性语义
1. 补齐 tags（优先 `artifacts_*`, `clarify`, `a2ui_emit`, `browser_dom`）。
2. 工具描述补“同义词簇”：
- `browser_dom`: dom, selector, field, submit
- `artifacts_list`: list, enumerate, index, generated files
- `clarify`: ambiguity, missing requirement, blocking detail

预期收益：`web/full` Top-3 失败 case 从 5 降到 1~2。

### P2: 提升提示词对“隐式选工具”的先验引导
在 `code-expert` / `devops` 增加一段固定指令：
- 先判定任务意图类别
- 再从候选工具中做最小充分选择
- 对边界模糊时给出备选工具与选择理由

预期收益：减少 Top3/Top5 边界错位。

## 8. 产物路径

核心结果（JSON）：
- `tmp/eval-foundation-20260208/web-full/foundation_result_foundation-20260208-052913.json`
- `tmp/eval-foundation-20260208/web-architect/foundation_result_foundation-20260208-052931.json`
- `tmp/eval-foundation-20260208/web-lark-local/foundation_result_foundation-20260208-052931.json`
- `tmp/eval-foundation-20260208/web-full-top5/foundation_result_foundation-20260208-052950.json`
- `tmp/eval-foundation-20260208/cli-full/foundation_result_foundation-20260208-052943.json`
- `tmp/eval-foundation-20260208/cli-safe/foundation_result_foundation-20260208-052943.json`
- `tmp/eval-foundation-20260208/cli-read-only/foundation_result_foundation-20260208-052950.json`

对应 Markdown 报告已同步输出在各目录下。
