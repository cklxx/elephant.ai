# 2026-02-10 基础评测重建（现有 tools + skills）

## 1) 目标
- 删除基础评测中所有失效工具调用 case。
- 针对当前实际可用 tools/skills 重新建立基础评测集合并重跑。

## 2) 当前可用工具清单
来自本轮 inventory run（`tmp/foundation-suite-r21-inventory-20260210-115731`）：
- `browser_action`
- `channel`
- `clarify`
- `execute_code`
- `memory_get`
- `memory_search`
- `plan`
- `read_file`
- `replace_in_file`
- `request_user`
- `shell_exec`
- `skills`
- `web_search`
- `write_file`

## 3) 删除失效工具调用评测（基础层）
已重构并清理：
- `evaluation/agent_eval/datasets/foundation_eval_cases_tool_coverage.yaml`
  - 从 `20` case 精简到 `10` case，移除所有依赖失效工具的 case。
- `evaluation/agent_eval/datasets/foundation_eval_cases_prompt_effectiveness.yaml`
  - 移除 `find/ripgrep/grep/search_file/list_dir/browser_dom/browser_info/browser_screenshot/web_fetch` 相关 case。
- `evaluation/agent_eval/datasets/foundation_eval_cases_proactivity.yaml`
  - 全量重写为仅使用现有可用 tools/skills 的主动性 case（含 `skills`、`channel` 覆盖）。

失效工具二次校验：以上 3 个基础数据集 `INVALID=0`。

## 4) 新基础评测套件
- 新增：`evaluation/agent_eval/datasets/foundation_eval_suite_basic_active.yaml`
- collections：
  - `tool-coverage`
  - `prompt-effectiveness`
  - `proactivity`

## 5) 评测结果（x/x）
运行：
- `go run ./cmd/alex eval foundation-suite --suite evaluation/agent_eval/datasets/foundation_eval_suite_basic_active.yaml --output tmp/foundation-suite-r21-basic-active-20260210-115920 --format markdown`

结果：
- Collections: `3/3`
- Cases: `31/31`
- Applicable Cases: `31/31`
- N/A: `0`
- pass@1: `27/31`
- pass@5: `31/31`
- Failed: `0`

## 6) 结论
1. 基础评测已完成“失效工具调用清零”（N/A 从高值降至 `0`）。  
2. 新基础集合已与当前 tools/skills 可用性一致，可用于后续工具优化的稳定回归基线。  
