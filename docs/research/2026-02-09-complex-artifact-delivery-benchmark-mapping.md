# 复杂任务+文件产物评测调研与映射（2026-02-09）

## 目标
- 补齐“只看首工具路由”之外的交付导向评测视角：要求 agent 在复杂任务里输出可核验文件产物。
- 把外部 benchmark 的高难模式映射到本仓库可稳定回归的离线 foundation 集合。

## 结论摘要
- 复杂任务评测不能只看 `pass@1/pass@5` 路由命中，还要看“交付契约覆盖率”（是否覆盖产物写入、可追溯、发布/上传、清理）。
- 新增 `complex_artifact_delivery` 集合，作为与 `challenge_hard_v2` 并行的交付压力层。
- 报告新增 good/bad 交付抽样检查，避免总分掩盖交付质量问题。

## 外部基准与可迁移难题模式

### 1) SWE-bench / SWE-bench Verified
- 价值：真实仓库修复链路，强调可复现变更与验证流程。
- 可迁移模式：
  - 变更证据产物（补丁、测试输出、修复说明）
  - 最终交付必须可审计（manifest/trace）
- 本地映射：`replace_in_file + shell_exec + artifacts_write + artifact_manifest`

### 2) GAIA
- 价值：跨工具检索+执行+验证，强调真实世界任务完成。
- 可迁移模式：
  - 多源证据收集后生成最终交付文件
  - 长任务中的中间产物组织与最终摘要
- 本地映射：`web_search + web_fetch + browser_screenshot + artifacts_write`

### 3) OSWorld
- 价值：复杂长链任务，环境交互和执行稳定性压力大。
- 可迁移模式：
  - 多步骤操作后沉淀“可交接产物”
  - 清理过期产物与状态恢复
- 本地映射：`shell_exec + find + artifacts_delete + artifact_manifest`

### 4) MLE-bench
- 价值：实验类任务闭环，要求实验产物与报告可复核。
- 可迁移模式：
  - 指标结果文件 + 解释文档双产物交付
  - 失败重跑后产物治理
- 本地映射：`execute_code + artifacts_write + artifacts_list + artifacts_delete`

## 本地评测设计原则
- 用 `pass@1/pass@5` 衡量工具路由正确性。
- 额外引入 `deliverable_check`：
  - 信号覆盖（matched/required）
  - 覆盖率（coverage）
  - good/bad 状态
- 不可用工具场景标记 N/A，不计失败；但有替代路径时必须推动可用性修复。

## 新增集合（complex_artifact_delivery）覆盖面
- 长文档产物交付（report/spec/runbook）
- 视觉/多媒体产物（screenshot/diagram/pptx）
- 证据链可追溯（manifest/list）
- 发布交付（write_attachment/lark_upload_file）
- 清理治理（artifacts_delete）
- 多源融合与多轮任务交付

## 报告格式新增项
- 总览增加：`Deliverable Cases x/x`、`Deliverable Good x/x`、`Deliverable Bad x/x`
- 每集合增加：交付相关 x/x 指标
- 新增章节：`Deliverable Sampling Check`
  - Good Case Samples（抽样）
  - Bad Case Samples（抽样）
  - 每条样本展示 expected / top matches / contract coverage / reason

## 参考来源
- SWE-bench (OpenReview): https://openreview.net/forum?id=VTF8yNQM66
- SWE-bench 官网: https://www.swebench.com
- GAIA (OpenReview): https://openreview.net/forum?id=fibxvahvs3
- OSWorld (OpenReview): https://openreview.net/forum?id=s9rOjjY3I4
- MLE-bench (OpenReview): https://openreview.net/forum?id=IFeW4h3Y96
