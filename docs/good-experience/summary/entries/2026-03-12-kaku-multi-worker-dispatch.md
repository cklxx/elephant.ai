Summary: kaku-cli 多 worker 并发调度采用 leader+worker 分离架构，leader 通过 get-text 巡查 pane 状态、send-text(no-paste) 派发任务，关键经验：send-text 必须 no-paste 模式确保回车生效，需监控 worker context 容量避免任务堆积。

## Metadata
- id: goodsum-2026-03-12-kaku-multi-worker-dispatch
- tags: [summary, kaku, cli, multi-worker, dispatch, concurrency]
- derived_from:
  - docs/good-experience/entries/2026-03-12-kaku-multi-worker-dispatch.md
