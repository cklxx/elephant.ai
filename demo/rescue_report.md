# interactive-demo-deployer rescue report

## 时间
- 执行时间：$(date '+%Y-%m-%d %H:%M:%S')

## 进程状态检查
- 执行命令：`ps aux | grep -i interactive-demo-deployer | grep -v grep || true`
- 结果：未发现明确命名为 `interactive-demo-deployer` 的唯一进程。
- 观察到相关长期运行 agent/launcher 进程（happy/claude remote launcher）存在，疑似 orchestrator 卡住而非完全退出。

## 端口状态检查
- 执行命令：`lsof -iTCP -sTCP:LISTEN -n -P | egrep ':3000|:4173|:8000|:8080|:5000'`
- 结果：
  - `:8000` -> python/uvicorn 在监听
  - `:3000` -> node 在监听
  - `:4173` -> node 在监听
  - `:8080` -> 进程在监听
  - `:5000` -> 进程在监听

## 日志检查
- 路径：`~/.alex/kernel/default/demo/logs/`
- 文件：`diagnose_20260222.log` 存在（非空）

## 当前决策
- 决策：暂不盲目 kill 全部相关进程，避免误伤当前在线 demo。
- 理由：核心端口仍在监听，说明 demo 运行面未完全失活；更可能是 deployer orchestration 卡住。

## 下一步（自动执行）
1. 从 `diagnose_20260222.log` 与 shell 历史回溯 deployer 启动命令。
2. 精确定位并重启卡住的 deployer 进程（仅定向，不全量）。
3. 对 3000/4173/8000 进行 HTTP 探活与浏览器验证。
4. 在本报告补充最终可访问 URL（或失败原因与替代方案）。

## 当前可访问候选
- 待验证：`http://127.0.0.1:3000`
- 待验证：`http://127.0.0.1:4173`
- 待验证：`http://127.0.0.1:8000`

