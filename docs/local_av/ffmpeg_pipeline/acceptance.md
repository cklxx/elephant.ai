# 视频处理与 FFmpeg 管线验收方案

## 1. 环境准备
- FFmpeg ≥ 6.1，确保启用常用滤镜（`xfade`, `frei0r`, `loudnorm`）。  
- 安装 `ffprobe`, `mediainfo` 用于验收校验。  
- GPU 验收需具备 NVIDIA 驱动 535+ 或 Intel Media SDK。  
- 准备 4 段测试素材（不同帧率/分辨率）与水印 PNG。

## 2. 验收用例
> **完成度提示**：详见 `docs/local_av/ffmpeg_pipeline/gap_matrix.md#1-验收用例覆盖情况` 以及 `docs/local_av/ffmpeg_pipeline/unresolved_work.md`。目前 FFM-002、FFM-002A、FFM-005、FFM-006 仍未交付脚本级复现；FFM-005 仅新增“nvidia-smi 探测 + h264_nvenc 切换”的 demo，并未接入 orchestrator/指标。
| 编号 | 场景 | 操作 | 判定标准 |
| --- | --- | --- | --- |
| FFM-001 | 同编码拼接 | 使用 concat demuxer | 输出时长 == 各片段时长之和 ±0.1s |
| FFM-002 | 异编码拼接 | 自动触发 filter_complex | 输出帧率、分辨率与模板一致 |
| FFM-002A | 预检阻断 | 输入两段分辨率不同素材且未配置滤镜/模板 | CLI 在 concat 前失败并提示参数不一致 |
| FFM-003 | 转场效果 | 配置 `xfade=fade` | 视觉检查转场平滑，无闪烁 |
| FFM-004 | 水印叠加 | 指定位置/透明度 | 水印坐标准确，透明度 70%±5% |
| FFM-005 | GPU 编码 | 开启 `use_gpu` | 处理速度提升 ≥ 2 倍，且日志标记 GPU |
| FFM-006 | 错误处理 | 输入缺失文件 | 任务失败，stderr 指出缺失路径 |

## 3. 性能指标
- 1080p@30fps、10 分钟素材，CPU 模式处理时长 ≤ 15 分钟。  
- GPU 模式处理时长 ≤ 8 分钟。  
- 重试后成功率 ≥ 99%。

## 4. 输出校验
- 使用 `mediainfo output.mp4`：
  - `Format/Info`: AVC (H.264)。  
  - `Overall bit rate`: ±10% 模板值。  
  - `Writing library` 包含 `task-orchestrator`.  
- 提供 QC 报告：截图、音频波形（Audacity 导出）。

## 5. 交付物
- 转码模板样例 (`configs/ffmpeg/presets.yaml`)。
- 验收脚本：`scripts/verify_ffmpeg.sh`，自动对比输出参数（可通过 `FFMPEG_BIN`、`FFPROBE_BIN`、`VIDEO_RESOLUTION` 等环境变量指定不同工具或输出要求）。
- 视频剪辑模拟：`scripts/video_editing_demo.sh`，批量生成彩色片段、拼接并叠加水印/配乐，可直接用作 Bash/CodeExec 调用示例来证明剪辑能力；支持 `SOURCE_MANIFEST` 来加载真实素材、`PRIMARY_AUDIO_PATH`/`SECONDARY_AUDIO_PATH` 来演示多轨混音，并提供 `SIMULATE_MISSING_INPUT=1` 参数预演 FFM-006；同时新增 `ENABLE_GPU=1`/`PREFERRED_GPU_BACKEND`/`GPU_STATUS_FILE`，可尝试检测 `nvidia-smi` 并在可用时切换 `h264_nvenc`，且现在支持 `RUN_STATUS_FILE=...` 将时长、输出大小、GPU 状态写入摘要文件；如需额外输出伪 Prometheus 指标，可设置 `METRICS_LOG_PATH=artifacts/demo.prom`（格式见 `pseudo_metrics.md`），并可通过 `WATERMARK_TEXT`/`WATERMARK_POSITION`/`WATERMARK_FONT_SIZE`/`WATERMARK_OPACITY`/`WATERMARK_MARGIN` 自定义 `drawtext` 水印内容与位置，或在需要 logo/品牌图层时传入 `WATERMARK_IMAGE_PATH`/`WATERMARK_IMAGE_SCALE`/`WATERMARK_IMAGE_OPACITY` 来叠加 PNG（脚本会自动转成 overlay）；若需叠加字幕，现在可传入 `SUBTITLE_FILE`（支持 SRT/ASS）、`SUBTITLE_CHARSET` 与 `SUBTITLE_FORCE_STYLE`，脚本会自动把 `subtitles` 滤镜插入 filtergraph 并记录所用样式，但字幕模板库与“无字体环境”自动回退仍未交付，缺口记录见 `unresolved_work.md`。
- CLI/技能入口：`cmd/videoedit` 将上述脚本封装为 `go run ./cmd/videoedit --output deliverables/demo.mp4 --manifest segments.txt` 形式的一键命令，便于在 `bash`/`code_execute` 中复用，可追加 `--run-status-file artifacts/cli_status.txt` 获得脚本级运行摘要，并通过 `--metrics-log artifacts/cli.prom` 让脚本写入伪指标文本；CLI 现同步暴露 `--watermark-text`/`--watermark-position`/`--watermark-font-size`/`--watermark-opacity`/`--watermark-margin` 以及 `--watermark-image`/`--watermark-image-scale`/`--watermark-image-opacity` 来直接控制文字或 PNG 水印，并新增 `--subtitle-file`/`--subtitle-charset`/`--subtitle-style` 以将 SRT/ASS 直接传入 demo；但整体依旧是 demo 包装，尚未与 orchestrator 指令或 GPU/指标等验收项联动，字幕模板、字体缺失回退与多样式预设仍在缺口列表中。
- Agent 技能：`skills/video_editing_skill.md` 描述了如何通过 `bash`/`code_execute` 执行剪辑脚本与自定义 ffmpeg 管线，指导 Agent 直接产出视频而非仅做环境验证。
- 状态看板：`docs/local_av/ffmpeg_pipeline/status.md` 列出了已交付能力与剩余 TODO（如 GPU 自动化、指标落地、实战样本），便于验收双方了解尚未完成的项目内容。
- GPU 验收记录：`gpu_capability.json`。

## 6. 验收流程
1. 实施方按顺序执行用例，保存命令与日志。  
2. 甲方复核输出文件元数据与截图。  
3. 双方确认性能数据，必要时进行二次测试。  
4. 填写验收报告并归档素材样本。
