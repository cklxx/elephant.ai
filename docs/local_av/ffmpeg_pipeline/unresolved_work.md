# FFmpeg 管线未完成项追踪

> 本文件补充 `status.md` 与 `gap_matrix.md`，将“尚未交付”的能力拆解成可执行的任务并记录当前阻塞。更新脚本/技能后，如仍未满足验收，请在此同步说明。

## 1. GPU 自动化（状态：进行中）
- 需求：按照技术方案的 `use_gpu` 模板自动探测 `nvidia-smi`/`vainfo`，在可用时拼接 `-hwaccel`、`h264_nvenc`/`hevc_nvenc` 参数，并在失败时回退 CPU。
- 现状：`scripts/video_editing_demo.sh` 与 `cmd/videoedit` 现已支持 `ENABLE_GPU=1`/`--enable-gpu`，能检测 `nvidia-smi` 并切换 `h264_nvenc`，同时输出 `GPU_STATUS_MESSAGE` 或写入 `GPU_STATUS_FILE`；但 orchestrator 仍无 `use_gpu` 参数，VAAPI/多 GPU 也未落地。
- 待办：补充 VAAPI/多 GPU 支持、在 orchestrator 层串联 `use_gpu` 模板与指标，并添加真实 GPU 样例及回归测试。

## 2. 指标与重试闭环（状态：未开始）
- 需求：技术方案列出的 `ffmpeg_job_duration_seconds`, `ffmpeg_retry_total`, `ffmpeg_video_precheck_failures_total` 需通过 Prometheus exporter 暴露。
- 现状：代码仅在文档中描述指标，未有实现或测试；`scripts/video_editing_demo.sh` 虽可通过 `RUN_STATUS_FILE` + `METRICS_LOG_PATH` 写入人读摘要与伪指标（详见 `pseudo_metrics.md`），但这些文本无法被监控系统采集，也不包含 retry/错误类型。
- 待办：在 `internal/orchestrator` 的执行路径新增计时/重试计数，并补充单测与 dashboard；届时退役伪指标文件以免混淆。

## 3. 实战素材/多轨管线（状态：进行中）
- 需求：验收需覆盖真实素材、字幕、`amix` 多轨与错误注入，证明脚本能消费用户提供的 manifest 与音轨。
- 现状：`scripts/video_editing_demo.sh` 现已支持 `SOURCE_MANIFEST`、`PRIMARY_AUDIO_PATH`、`SECONDARY_AUDIO_PATH`，可复用真实片段与多轨音频；但仍缺少示例素材及自动滤镜模板。
- 待办：整理示例素材包、在 skill 文档中列出下载路径，并补充自动 scale/fps 对齐逻辑。

## 4. 错误处理与任务重试（状态：未开始）
- 需求：FFM-006 要求 orchestrator 层能够捕获 FFmpeg 失败、打标错误类型并触发重试/告警。
- 现状：Demo 脚本可通过 `SIMULATE_MISSING_INPUT=1` 输出日志，但 orchestrator、tests 仍缺少验证。
- 待办：在 `tests/` 下增加集成用例，接入 metrics，并将失败截图纳入验收报告。

## 5. 交互入口/技能封装（状态：进行中）
- 需求：提供 `video_edit` CLI 或工具入口，允许对话中一键调用，而非手写 `bash`。
- 现状：`cmd/videoedit` 已将 demo 脚本封装为 `go run ./cmd/videoedit --output ...` 形式，技能文档也提供复制模板；但尚未发布二进制或接入 orchestrator/工具注册，无法在生产对话中直接引用。
- 待办：打包 CLI（或新增 `alex video-edit` 子命令）、接入 `toolregistry`、输出成功/失败指标，并补充 GPU/异常处理开关。

## 6. 水印与字幕灵活性（状态：进行中）
- 需求：验收要求脚本支持自定义文字/图片水印、透明 PNG 覆盖及字幕样式切换，以满足不同品牌/频道需求。
- 现状：`scripts/video_editing_demo.sh` 与 `cmd/videoedit` 已提供 `WATERMARK_*` + `WATERMARK_IMAGE_*` 环境变量（以及 `--watermark-*`、`--watermark-image*` 标志），可叠加 PNG/logo 并与文字水印共存，如今又新增 `SUBTITLE_FILE`/`SUBTITLE_CHARSET`/`SUBTITLE_FORCE_STYLE` 与 `--subtitle-*` 标志，可在 demo 中套用单条 SRT/ASS 字幕并记录 charset/style；但仍依赖字体文件，缺少 ASS/SRT 样式预设、字体缺失回退、双语/多字幕层封装。
- 待办：补充字幕/字体回退策略、在技能/验收文档中添加可复制的字幕样式模板，支持多字幕层/语言，必要时提供示例素材与截图，同时为 orchestrator/CLI 引入“字幕预设库”配置以满足 FFM-004 的最终要求。

> 当上述事项有进展时，请同时更新 `status.md`、`gap_matrix.md` 及本文件，使“未完成”标签与实际开发状态一致。
