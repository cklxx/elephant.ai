# FFmpeg 验收缺口矩阵

> 目标：明确每个验收用例、指标与交付物当前的完成度与缺口，便于后续排期与对外同步。

## 1. 验收用例覆盖情况
| 用例 | 当前状态 | 缺口说明 | 责任模块 |
| --- | --- | --- | --- |
| FFM-001 同编码拼接 | ✅ Demo 已覆盖 concat demuxer | - | `scripts/video_editing_demo.sh` |
| FFM-002 异编码拼接 | ⚠️ `scripts/video_editing_demo.sh` 可引用 `SOURCE_MANIFEST` 来使用真实素材，但仍需手动编辑 filter_graph/scale | 缺少自动化示例；`JobSpec` 也未提供异编码模板 | `scripts/video_editing_demo.sh`, `configs/ffmpeg` |
| FFM-002A 预检阻断 | ⚠️ 预检逻辑在 `JobSpec.Validate`/`LocalProber` 中，但无回归脚本与日志示例；demo 仅输出“尚未自动 scale”提醒 | 需要 CLI/skill 级别的复现步骤及失败截图 | `internal/task`, `skills/video_editing_skill.md` |
| FFM-003 转场效果 | ✅ demo 启用 `xfade`（通过 `TRANSITION_FILTER`），可复用 | - | `scripts/video_editing_demo.sh` |
| FFM-004 水印叠加 | ⚠️ demo/CLI 现支持 `WATERMARK_*` 文字、`WATERMARK_IMAGE_*` PNG overlay 以及 `SUBTITLE_*`/`--subtitle-*` 以渲染 SRT/ASS 字幕，可复现 logo+文字+字幕组合，但字幕样式库与“无字体环境”回退仍缺 | 需补充字幕/字体回退脚本及真实素材，详见 `unresolved_work.md#6-水印与字幕灵活性状态进行中` | `scripts/video_editing_demo.sh`, `cmd/videoedit` |
| FFM-005 GPU 编码 | ⚠️ Demo/CLI 新增 `ENABLE_GPU=1`/`--enable-gpu`，可检测 `nvidia-smi` 并切换 `h264_nvenc`，但 VAAPI、orchestrator、性能指标仍缺失 | 需脚本级 VAAPI 支持、真实 GPU 样例以及 orchestrator wiring | Demo/Skill、`internal/orchestrator` |
| FFM-006 错误处理 | ⚠️ Demo 可通过 `SIMULATE_MISSING_INPUT=1` 触发缺失文件日志，但 orchestrator/测试仍未覆盖 | 需把错误注入纳入自动化测试并输出截图/metrics | Demo/Skill、`tests` |

## 2. 指标与可观测性
| 指标/功能 | 状态 | 缺口细节 | 责任模块 |
| --- | --- | --- | --- |
| `ffmpeg_job_duration_seconds` | ❌ 未在 orchestrator 中暴露 | 缺少计时逻辑与 exporter wiring；`RUN_STATUS_FILE` + `METRICS_LOG_PATH` 仅提供脚本级摘要/伪指标（见 `pseudo_metrics.md`），无法满足监控要求 | `internal/orchestrator`, metrics exporter |
| `ffmpeg_retry_total` | ❌ | 未收集 per-job retry 次数；伪指标日志也无重试字段 | 同上 |
| `ffmpeg_video_precheck_failures_total` | ❌ | 未上报 `LocalProber`/`JobSpec.Validate` 失败数 | 同上 |
| GPU/CPU 切换日志 | ⚠️ Demo 现会输出 `GPU_STATUS_MESSAGE`、`GPU_STATUS_FILE`，并可在 `RUN_STATUS_FILE`/`METRICS_LOG_PATH` 中带上 GPU 状态，但 orchestrator 与指标仍无记录 | Demo/Skill + orchestrator |

## 3. Demo / Skill / CLI 交互
| 能力 | 状态 | 缺口说明 |
| --- | --- | --- |
| `scripts/video_editing_demo.sh` | ⚠️ 支持 `SOURCE_MANIFEST`、`PRIMARY_AUDIO_PATH`、`SECONDARY_AUDIO_PATH` 并内置 `amix`，但缺少真实样本包与自动 scale/fps | 补充参数、样例素材与失败模拟 |
| `skills/video_editing_skill.md` | ⚠️ 缺少一键触发的提示模板、FAQ 未覆盖 GPU/指标说明 | 增加 prompt snippet、FAQ 扩展 |
| CLI/聊天入口 | ⚠️ `cmd/videoedit` 可通过 `go run ./cmd/videoedit` 调用 demo，但尚未提供发行版、工具注册或 orchestrator 集成 | 需要将 CLI 打包为正式子命令/工具，并串联 GPU/指标逻辑 |

## 4. 资料/文档
| 文档 | 状态 | 缺口说明 |
| --- | --- | --- |
| 技术方案 (`tech_plan.md`) | ⚠️ 描述了 GPU/指标，但无落地追踪 | 需要在每段附 `Status:` |
| 验收方案 (`acceptance.md`) | ⚠️ 未标注完成度 | 需引用本矩阵并标注“未完成”用例 |
| 未完成事项 (`unresolved_work.md`) | ✅ 新增 | 需持续同步状态，避免与其它文档脱节 |
| 状态面板 (`status.md`) | ✅ 已列关键 TODO，但缺少逐用例 mapping | 需与本矩阵双向引用 |

## 5. 建议下一步
1. **GPU 后续**：在现有 `ENABLE_GPU=1` 基础上补充 VAAPI/多 GPU 支持，并将 `use_gpu`/指标 wiring 落到 orchestrator，而非仅在 demo 中记录日志。
2. **指标埋点**：在 `internal/orchestrator` 的 ffmpeg 执行前后打点并暴露到现有 Prometheus exporter。
3. **缺口对齐**：每完成一项用例/指标更新 `status.md` 和本文件，保持“完成 vs 未完成”清单一致。
