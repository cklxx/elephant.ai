Summary: Subagent parallel delegation intermittently failed with upstream rejections when many subtasks started simultaneously.
Remediation: Enforced `maxWorkers` as the default `subagent` parallelism cap, added a small start stagger, and added regression tests.

