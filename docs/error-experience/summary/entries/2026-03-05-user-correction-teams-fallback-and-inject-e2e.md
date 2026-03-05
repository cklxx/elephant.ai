# 2026-03-05 Summary · teams 验收必须覆盖 fallback + inject 端到端

- 用户纠正了“单点冒烟即可”的错误方向：teams 验证必须覆盖自动 fallback 与注入产品链路。
- 新规则：teams 相关变更的验证最少包含两类用例：
  - `fallback_clis` 自动切换成功；
  - `run_tasks(wait=false)` 期间 `reply_agent(message)` 注入生效。
- 参考详情：
  - docs/error-experience/entries/2026-03-05-user-correction-teams-fallback-and-inject-e2e.md
