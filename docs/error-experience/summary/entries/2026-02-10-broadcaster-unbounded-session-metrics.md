Summary: `EventBroadcaster` 的按 session 聚合指标使用无界 `sync.Map`，在长跑场景持续累积 orphan session，导致内存持续增长并触发 OOM 风险。
Remediation: 将 `dropsPerSession`/`noClientBySession` 改为带上限与 TTL 的有界计数存储（2048 cap + 30m TTL + 周期 prune），并增加大规模会话回归测试。
