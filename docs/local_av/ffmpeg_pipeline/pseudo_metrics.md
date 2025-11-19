# Demo Pseudo Metrics 指南

> 目的：在正式 Prometheus 指标 (`ffmpeg_job_duration_seconds`, `ffmpeg_retry_total` 等) 尚未接入 orchestrator 之前，使用脚本级日志快速补齐验收所需的“运行耗时/输出大小”记录。该文件不会被监控系统采集，只是验收过程中的临时产物。

## 1. 如何生成
- `scripts/video_editing_demo.sh` 支持 `METRICS_LOG_PATH=/tmp/ffmpeg_demo.prom`，脚本结束时会在指定路径追加一段伪指标。
- `cmd/videoedit` 暴露 `--metrics-log artifacts/demo.prom`，CLI 会将路径透传给脚本。
- 建议与 `RUN_STATUS_FILE` 一同使用：前者保存人读友好的摘要，后者补充准 Prometheus 格式的文本，方便后续迁移。

## 2. 文件格式
示例内容：
```
# pseudo metrics emitted 2024-06-13T03:25:04+00:00; replace with Prometheus exporter once ready
ffmpeg_demo_run_total{status="succeeded",gpu_backend="cpu"} 1 1718249104
ffmpeg_demo_run_duration_seconds{status="succeeded",gpu_backend="cpu"} 3.247
ffmpeg_demo_output_size_bytes{status="succeeded",gpu_backend="cpu"} 2310456
```
- `status`：`succeeded` 或 `failed`，直接映射 FFmpeg 返回码。
- `gpu_backend`：`cpu`、`cuda`、`vaapi-pending` 等，对应 demo 脚本的检测结果。
- 时间戳：`ffmpeg_demo_run_total` 附带的 Unix 时间，方便后续转换。
- 每次运行都会追加 3 行记录，方便按时间顺序 grep。

## 3. 限制（仍未完成的部分）
- ❌ **未接 Prometheus**：这些文本不会被 exporter 抓取，正式指标 (`ffmpeg_job_duration_seconds`, `ffmpeg_retry_total` 等) 仍需在 `internal/orchestrator` 内实现。
- ❌ **缺少 retry/失败分类**：只能看到“是否成功”以及耗时/输出大小，无法拆分具体错误类型。
- ❌ **缺少多作业关联**：demo 只处理单次渲染，无法区分项目/任务 ID；需要 orchestrator 接入后才能补齐维度。
- ⚠️ **VAAPI/GPU 仍在 TODO**：当 `vainfo` 存在时会记录 `vaapi-pending`，以此提醒该路径尚未完成。

> 一旦 Prometheus 指标落地，请删除或退役该伪指标文件，避免造成“指标已完成”的误解。
