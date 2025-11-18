# 本地优先音视频平台索引
> Last updated: 2025-11-18


> 📌 本文档已拆分为按模块维护的技术方案与验收手册，便于针对性评审与实施。

| 模块 | 技术方案 | 验收手册 | 当前进度 |
| --- | --- | --- | --- |
| 任务编排服务 | [docs/local_av/task_orchestrator/tech_plan.md](local_av/task_orchestrator/tech_plan.md) | [docs/local_av/task_orchestrator/acceptance.md](local_av/task_orchestrator/acceptance.md) | ✅ MVP 编排 + 超时/重试 + Prometheus 指标已上线 |
| 视频处理与 FFmpeg 管线 | [docs/local_av/ffmpeg_pipeline/tech_plan.md](local_av/ffmpeg_pipeline/tech_plan.md) | [docs/local_av/ffmpeg_pipeline/acceptance.md](local_av/ffmpeg_pipeline/acceptance.md) | 🟡 模板库与预检上线，高级滤镜/GPU 待补充 |
| 音频混音引擎 | [docs/local_av/audio_engine/tech_plan.md](local_av/audio_engine/tech_plan.md) | [docs/local_av/audio_engine/acceptance.md](local_av/audio_engine/acceptance.md) | 🟡 混音、Loop、双遍 loudnorm、包络完成，预设体系待补齐 |
| 文字转语音网关 | [docs/local_av/tts_gateway/tech_plan.md](local_av/tts_gateway/tech_plan.md) | [docs/local_av/tts_gateway/acceptance.md](local_av/tts_gateway/acceptance.md) | 🟡 文件缓存 + Mock Provider 可用，云端对接未启用 |
| 存储与元数据治理 | [docs/local_av/storage_layer/tech_plan.md](local_av/storage_layer/tech_plan.md) | [docs/local_av/storage_layer/acceptance.md](local_av/storage_layer/acceptance.md) | 🟡 根目录守护与覆写策略完成，版本化待补齐 |

## 快速总览

- **目标**：以本地处理为核心，辅以第三方 API，实现音视频拼接、混音、字幕与 TTS 能力。  
- **架构要点**：Go 编排服务 + 本地 FFmpeg/SoX + 可切换的 TTS API + 统一存储治理。  
- **迭代策略**：先交付可运行的 MVP（编排 + FFmpeg + TTS API），再扩展模板、缓存与监控。

> 如需了解历史版本，请查阅仓库 tag `local-av-plan-v1`。
