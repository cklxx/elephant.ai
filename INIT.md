# Kernel Initialization
- generated_at: 2026-02-12T08:38:50Z
- kernel_id: default

## Runtime Config
- schedule: 0,30 * * * *
- state_dir: /Users/bytedance/.alex/kernel
- state_path: /Users/bytedance/.alex/kernel/default/STATE.md
- init_path: /Users/bytedance/.alex/kernel/default/INIT.md
- system_prompt_path: /Users/bytedance/.alex/kernel/default/SYSTEM_PROMPT.md
- timeout_seconds: 900
- lease_seconds: 1800
- max_concurrent: 1
- channel: lark
- user_id: ou_e9b3187320a99a3a73dbe96281f86703
- chat_id: oc_d2c60337c5db1629a78753e5952e41cc

## Seed State
```md
# Kernel State
## identity
elephant.ai autonomous kernel
## recent_actions
(none yet)
```

## Agents
### 1. autonomous-state-loop
- enabled: true
- priority: 5
- metadata:
  - purpose: proactive-loop
- prompt_template:
```text
你是 elephant.ai 的 kernel 自动循环代理。每次循环都必须完成一件真实动作并更新状态文件。

当前状态：
{STATE}

严格按顺序执行：
1) 读取 `README.md` 前 40 行，提炼一句 <=30 字中文摘要。
2) 执行 `git status -sb`，提炼一句 <=40 字的仓库状态摘要。
3) 生成 `## 执行总结` 小节（2-4 条 bullet），必须包含：
   - README摘要
   - 仓库状态
   - 下一步建议（1 条）
4) 最终仅输出两行：
   done: cycle completed
   notify: <一句需要通知 cklxx 的信息，含摘要与下一步>
```

