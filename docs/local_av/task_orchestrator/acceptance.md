# 任务编排服务验收方案

## 1. 环境准备
- Go 1.24+，启用 `GODEBUG=asyncpreemptoff=0` 保证调度稳定。  
- 预装 FFmpeg、SoX，可执行路径写入 `PATH`。  
- 准备示例素材：不少于 3 段视频、2 段音频和 2 条 TTS 文本。  
- 新建输出目录，确保拥有写权限，并清理旧产物。

## 2. 配置校验
- 执行 `task-orchestrator --spec examples/local_av/sample_job.yaml --dry-run`。  
- 期望输出：列出阶段执行顺序、FFmpeg/TTS 命令，不创建文件。  
- 校验 YAML 中的 `working_dir` 被归一化在白名单根路径内。

## 3. 功能用例
| 用例编号 | 场景 | 步骤 | 预期结果 |
| --- | --- | --- | --- |
| ORC-001 | 基础流程 | `task-orchestrator --spec sample` | 任务完成，产生最终媒体、日志中包含每个阶段耗时 |
| ORC-002 | Dry-run | 增加 `--dry-run` | 不写磁盘，输出命令行预览 |
| ORC-003 | 失败回滚 | 人为删除某段素材 | 阶段失败后任务标记为失败，输出清晰的错误信息 |
| ORC-004 | 重试机制 | 配置 `retries: 2`，模拟首次失败 | 第二次成功后记录重试次数 |
| ORC-005 | 超时控制 | 将 `stage_timeout` 设为 1s，执行长任务 | 超时被取消，产生日志与指标 |
| ORC-006 | 视频预检 | 使用两段分辨率不同素材，省略 `filters`/`preset` | CLI 在 `video_concat` 前失败并提示参数不兼容 |

## 4. 指标验收
- `local_av_orchestrator_job_stage_duration_seconds`：每个阶段至少上报 1 条样本，并区分 `status=success|retry|failed|timeout`。
- `local_av_orchestrator_job_stage_failures_total`：在失败场景下按 `reason`（`timeout`、`canceled`、`error`）计数 +1。
- `local_av_orchestrator_job_stage_retries_total`：重试场景增加计数。
- `local_av_orchestrator_jobs_active`：dry-run 时为 0，正式执行时上升至 1，任务结束后归零。

## 5. 文档与交付
- CLI 使用手册：参数说明、示例 YAML、常见错误。  
- 运维手册：日志路径、指标采集、重试策略。  
- 验收报告需附：命令输出、日志摘要、指标截图。

## 6. 验收流程
1. 实施方演示 dry-run 与正式执行。  
2. 甲方随机抽检一个失败场景，验证错误处理。  
3. 复核生成文件的元数据（分辨率、码率、时长）。  
4. 双方签署验收单并归档日志。
