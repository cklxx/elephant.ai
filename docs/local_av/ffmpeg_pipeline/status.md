# FFmpeg 视频管线状态面板

> 最近更新：`scripts/video_editing_demo.sh` 与 `cmd/videoedit` CLI 新增 `SUBTITLE_FILE`/`SUBTITLE_CHARSET`/`SUBTITLE_FORCE_STYLE` 以及对应的 `--subtitle-*` 标志，可直接在 demo 流程中套用 SRT/ASS 字幕并将 `subtitles` 滤镜插入 filtergraph；日志会记录使用的 charset/style，帮助对齐 FFM-004“字幕叠加”验收。但字幕预设库、字体缺失回退、多字幕层及 orchestrator 级封装仍未交付（缺口详见本页与 `unresolved_work.md`）。
>
> 关联缺口矩阵：见 `docs/local_av/ffmpeg_pipeline/gap_matrix.md`；若需具体“未完成”任务，请查阅 `docs/local_av/ffmpeg_pipeline/unresolved_work.md`。

## 完成项概览
- [x] **基础执行链路**：`internal/ffmpeg` 提供 `LocalExecutor`/`LocalProber`，支持 concat/mux、ffprobe 元数据采集。
- [x] **模板与校验**：`configs/ffmpeg/presets.yaml`、`JobSpec.Validate`（`internal/task/spec.go`）负责参数统一及 TTS 字段清洗。
- [x] **技能/脚本**：`skills/video_editing_skill.md`、`scripts/video_editing_demo.sh`、`scripts/verify_ffmpeg.sh` 可通过 `bash`或 `code_execute` 直接演示拼接 → 水印 → 配乐 → 校验的完整流程。
- [x] **错误注入示例**：`scripts/video_editing_demo.sh` 暴露 `SIMULATE_MISSING_INPUT=1`，可在生成最终结果前模拟一次失败的 concat 以重现 FFM-006 缺失文件日志。
- [x] **实战素材挂载**：Demo 脚本支持 `SOURCE_MANIFEST`、`PRIMARY_AUDIO_PATH`、`SECONDARY_AUDIO_PATH`，可使用真实片段与多轨音频，但仍缺自动滤镜/预处理逻辑（详见“未完成事项”）。
- [x] **CLI 封装（Beta）**：`cmd/videoedit` 将 demo 脚本封装为 `go run ./cmd/videoedit ...`，并在技能文档提供复制模板，便于对话中一键触发（仍需发行版/工具注册支持）。
- [x] **GPU 探测日志（Beta）**：`scripts/video_editing_demo.sh` 与 `cmd/videoedit` 暴露 `ENABLE_GPU=1`/`--enable-gpu`、`PREFERRED_GPU_BACKEND` 等入口，可在检测到 `nvidia-smi` 时切换 `h264_nvenc` 并输出 `GPU_STATUS_MESSAGE`/`GPU_STATUS_FILE`，方便验收附录记录（仍缺 VAAPI/orchestrator 集成）。
- [x] **运行摘要（临时方案）**：`scripts/video_editing_demo.sh` 现在支持 `RUN_STATUS_FILE=...` 将开始/结束时间、耗时、输出大小与 GPU 状态写入文本，`cmd/videoedit` 也可通过 `--run-status-file` 传参；该摘要仅替代缺失指标，仍需 orchestrator 暴露 `ffmpeg_job_duration_seconds` 等正式监控。
- [x] **伪指标日志（临时方案）**：新增 `METRICS_LOG_PATH`/`--metrics-log` 可将伪 Prometheus 文本写入文件，方便验收附上耗时/输出大小记录（格式详见 `pseudo_metrics.md`）；但这只是缺口提示，仍需正式指标。
- [x] **可配置文字/PNG 水印（Beta）**：Demo/CLI 暴露 `WATERMARK_*`/`--watermark-*` 与 `WATERMARK_IMAGE_*`/`--watermark-image*`，允许直接组合文字与 PNG/logo 水印；如字体缺失仍会跳过 `drawtext` 并提醒字幕样式/字体回退待补齐。

## 未完成事项
| 类别 | 描述 | 影响 |
| --- | --- | --- |
| GPU 自动化 | Demo/CLI 现可通过 `ENABLE_GPU=1` 检测 `nvidia-smi` 并切换 `h264_nvenc`，但 VAAPI 管线、orchestrator 的 `use_gpu` 模板、失败重试与指标上报仍未实现。 | GPU 用例 FFM-005 仍无法验收，日志仅涵盖“检测到/未检测到 nvidia-smi”。 |
| 指标与重试 | 技术方案要求的 `ffmpeg_job_duration_seconds`, `ffmpeg_retry_total`, `ffmpeg_video_precheck_failures_total` 等 Prometheus 指标尚未在 `internal/orchestrator` 中暴露。 | 无法度量拼接性能或失败率，也无法向平台上报重试次数；`RUN_STATUS_FILE` + `METRICS_LOG_PATH` 只能输出脚本级摘要/伪指标，详见 `pseudo_metrics.md`，仍无法替代监控闭环。 |
| 实战素材覆盖 | Demo 现可加载 `SOURCE_MANIFEST`、自定义音轨并混音，但仍缺真实样本包与自动 scale/fps 对齐逻辑；FFM-002A 仍无脚本化预检示例。 | 测试样例不足以证明真实素材兼容性，需要补充脚本或样本文件。 |
| 交互集成 | `cmd/videoedit` 提供 CLI 封装，但尚未发布预编译二进制/`video_edit` 工具注册，无法直接在 orchestrator 或 UI 中调用。 | CLI 仍需要通过 `go run` 触发，未与任务系统、GPU/指标联动；参见 `gap_matrix.md` 中“CLI/聊天入口”缺口。 |
| 水印/字幕灵活性 | Demo/CLI 已支持 `WATERMARK_*` + `WATERMARK_IMAGE_*` 以及最新的 `SUBTITLE_*`/`--subtitle-*` 开关，可叠加文字、PNG/logo 与基础字幕文件；但字幕样式预设、字体缺失回退、双语/多字幕层封装仍未实现。 | 仍难以覆盖字幕定制或缺字体环境；详见 `unresolved_work.md#6-水印与字幕灵活性状态进行中`。 |

## 后续建议
1. **GPU 探测脚本**：在 demo/技能中加入 `nvidia-smi --query-gpu`/`vainfo` 检查，并根据结果拼接 `-hwaccel`、`-c:v h264_nvenc` 参数，失败时记录 warning。
2. **指标落地**：在 `internal/orchestrator` 的 ffmpeg执行路径周围增加计时器和 retry 计数，并通过现有的 metrics exporter 暴露。
3. **实战样本库**：准备包含不同编码、水印、字幕、坏片段的素材，完善 `scripts/video_editing_demo.sh` 的参数以覆盖验收用例。
4. **技能入口优化**：在技能文档中附带可复制的提示模板或封装 `bash` helper，使对话中可以“一键”触发常见流程。

> 当完成上述 TODO 后，请同步更新本文件和 `skills/video_editing_skill.md`，确保文档与实际能力保持一致。
