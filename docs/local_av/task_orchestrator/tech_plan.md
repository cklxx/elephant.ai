# 任务编排服务技术方案

## 1. 模块定位
- 负责接收音视频加工任务（CLI/REST），统一解析 YAML 规格。  
- 将任务拆分为 TTS 生成、音频混音、视频处理、成品封装等阶段，串联内部执行引擎。  
- 管控资源：并发度、优先级、GPU/CPU 标签、临时目录生命周期。  
- 提供可观测性：结构化日志、Prometheus 指标、审计事件。

## 2. 组件结构
| 子组件 | 职责 | 关键接口 |
| --- | --- | --- |
| `cmd/task-orchestrator` | CLI/REST 入口，支持 `--spec`、`--dry-run` | `main.go`、`ServeHTTP` |
| `internal/task/spec` | YAML 解析、schema 校验、路径归一化 | `LoadSpec`、`(*JobSpec).Validate` |
| `internal/orchestrator` | 调用各执行模块，维护阶段状态机 | `Run(ctx, *JobSpec)` |
| `internal/ffmpeg/probe` | ffprobe 预检接口 | `Prober.Probe` |
| `internal/ffmpeg/preset` | 输出模板仓库 | `PresetLibrary.Get` |
| `internal/orchestrator/pipeline` | 阶段定义、回滚策略、指标采集 | `Stage`、`Result` |

## 3. 工作流
1. **载入任务**：读取 YAML → 解析 → 校验输出路径是否在白名单根目录内。  
2. **预处理**：
   - 拉取/校验素材，生成校验和。
   - 根据配置生成执行计划（阶段 DAG）。
3. **执行阶段**：
   - `tts`: 调用 TTS 客户端，产出本地缓存音频。
   - `audio_mix`: 调用音频引擎生成合成音轨。
   - `video_concat`: 调用 FFmpeg 管线拼接视频；若未显式配置滤镜则先运行 `ffprobe` 预检，阻断不兼容分辨率/帧率。
   - `mux`: 将视频与音频合并，生成最终输出。
4. **收尾**：归档元数据、生成报告、触发 webhook。

## 4. 数据模型
```go
// internal/task/spec.go

// JobSpec -> AudioSpec -> AudioTrack/TTSRequest 形成树形结构
// 校验内容：路径合法、输出不覆盖已有文件（除非 force）、引用别名存在。
```

- 状态记录结构：
```json
{
  "job_id": "20250327-001",
  "stage": "audio_mix",
  "status": "running",
  "started_at": "2025-03-27T05:08:00Z",
  "metadata": {
    "inputs": 3,
    "uses_gpu": false
  }
}
```

## 5. 依赖与集成
- 调用音频、视频、TTS 模块，通过接口约束避免耦合。
- 预加载 FFmpeg Preset Library，允许视频阶段按名称应用码率/像素格式参数。
- 使用 `context.Context` 统一超时与取消策略。  
- `goroutine` + `errgroup` 并发运行独立阶段，确保日志包含阶段名称。

## 6. 安全与审计
- 限制任务 YAML 可访问的素材根目录。  
- CLI/REST 层面增加 `--read-only`、`--allow-overwrite` 标志。  
- 记录 TTS 文本摘要（hash）与供应商响应 ID，满足追踪需求。  
- 所有外部调用的 HTTP Header 中附带 request-id，方便排查。

## 7. 可观测性
- Prometheus 指标（namespace `local_av_orchestrator`）：
  - `local_av_orchestrator_job_stage_duration_seconds{stage,status}`：阶段耗时按成功/失败/重试分类。
  - `local_av_orchestrator_job_stage_failures_total{stage,reason}`：记录超时、取消、错误等失败原因。
  - `local_av_orchestrator_job_stage_retries_total{stage}`：统计重试次数。
  - `local_av_orchestrator_jobs_active`：当前正在执行的任务数量（便于观测 `job_queue_depth`）。
- 将 YAML 中的 `job.tags` 注入日志字段。
- 在 dry-run 模式下输出拟执行的 FFmpeg/TTS 请求，辅助变更评审。

## 8. 迭代路线
1. MVP：CLI + 串行阶段执行 + dry-run。
2. v1：REST 接口、并行阶段调度、Prometheus 指标。
3. v2：优先级队列、阶段回滚、任务断点续跑。
4. v3：多节点协同（Redis 队列）、可视化仪表板。

## 9. 任务进度
- [x] 串行阶段编排、工作目录守护。
- [x] 阶段级超时与全局重试策略。
- [ ] REST 接口与并行阶段调度。
- [x] 指标暴露。
- [x] 视频预检与 Preset 注入。
- [ ] 任务断点续跑。
