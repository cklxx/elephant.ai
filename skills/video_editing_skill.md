# 视频剪辑与合成技能

> 一句话总结：直接调用本地 `ffmpeg`/`ffprobe` 管线进行素材预检、拼接、滤镜、水印与导出，可通过 `bash`/`code_execute` 工具运行脚本或定制命令来产出成片。

## 技能目标
- 读取/校验用户提供的素材（分辨率、帧率、编码器），必要时利用模板统一参数。
- 拼接/剪辑片段，套用滤镜（`xfade`、`drawtext`、`overlay` 等）并叠加音轨或水印。
- 交付可下载的视频文件，并给出 ffprobe/mediainfo 校验结果。

## 支持的工具链
| 工具 | 用途 | 调用方式 |
| --- | --- | --- |
| `ffmpeg` | 拼接、滤镜、水印、转码 | `bash -lc "ffmpeg ..."` 或 `code_execute` 运行脚本 |
| `ffprobe` | 获取流信息与元数据 | `bash -lc "ffprobe ..."` |
| `scripts/video_editing_demo.sh` | 生成示例片段 → 拼接 → 加水印与配乐；支持模拟缺失文件错误，亦可加载自定义 manifest/音轨，并可通过 `ENABLE_GPU=1`/`PREFERRED_GPU_BACKEND` 试探 `nvidia-smi`/`vainfo` 状态；设置 `RUN_STATUS_FILE=/tmp/status.txt` 可输出时长/输出大小等运行摘要，再结合 `METRICS_LOG_PATH=/tmp/demo.prom` 生成伪指标文本（详见 `docs/local_av/ffmpeg_pipeline/pseudo_metrics.md`，暂未连接 orchestrator 指标）；若需自定义水印，可传入 `WATERMARK_TEXT`/`POSITION`/`FONT_SIZE`/`OPACITY`/`MARGIN` 控制 `drawtext`，或通过 `WATERMARK_IMAGE_PATH`/`WATERMARK_IMAGE_SCALE`/`WATERMARK_IMAGE_OPACITY` 叠加 PNG/logo；同时支持 `SUBTITLE_FILE`/`SUBTITLE_CHARSET`/`SUBTITLE_FORCE_STYLE` 在示例流程中叠加 SRT/ASS 字幕并记录 style（字幕模板库/字体回退仍在缺口列表） | `bash scripts/video_editing_demo.sh OUTPUT_PATH=demo.mp4 [SOURCE_MANIFEST=segments.txt] [PRIMARY_AUDIO_PATH=bgm.wav]` |
| `cmd/videoedit` | 将 demo 脚本封装为 CLI，可通过 `go run ./cmd/videoedit` 或编译后二进制一键触发 concat→水印→混音流程，附带 `--enable-gpu`、`--gpu-backend`、`--watermark-*`、`--watermark-image*` 以及新的 `--subtitle-*` 参数 | `go run ./cmd/videoedit --output deliverables/cli_demo.mp4 [--manifest segments.txt] [--simulate-missing-input]` |
| `scripts/verify_ffmpeg.sh` | 快速确认二进制可用并输出参数 | 可选，验收/排障使用 |

## 快速开始
### 0. 使用 `videoedit` CLI 一键执行
```bash
go run ./cmd/videoedit \
  --output deliverables/cli_demo.mp4 \
  --segment-duration 1.5 \
  --resolution 1920x1080 \
  --simulate-missing-input
```
- CLI 会自动拼接需要的环境变量并调用 `scripts/video_editing_demo.sh`，终端直接输出 FFmpeg/ffprobe 日志。
- 支持 `--manifest`, `--primary-audio`, `--secondary-audio`, `--audio-volume` 等参数来加载真实素材或控制混音。
- 若需使用本地安装的 ffmpeg，可添加 `--ffmpeg-bin /opt/ffmpeg/bin/ffmpeg --ffprobe-bin /opt/ffmpeg/bin/ffprobe`。
- GPU 验收试跑：追加 `--enable-gpu --gpu-backend cuda`（或 `vaapi`）后，CLI 会向脚本传入 `ENABLE_GPU=1` 并输出是否检测到 `nvidia-smi`/`vainfo` 的日志；当前环境无 GPU 时会自动回退 CPU，同时提示缺口。
- 自定义水印：通过 `--watermark-text "合成完成" --watermark-position top-left --watermark-font-size 60 --watermark-opacity 0.5 --watermark-margin 24` 调整文字样式，若需叠加品牌 logo，可再追加 `--watermark-image assets/logo.png --watermark-image-scale 0.7 --watermark-image-opacity 0.8`；字幕模板/无字体环境仍在缺口列表，详见 `docs/local_av/ffmpeg_pipeline/unresolved_work.md#6-水印与字幕灵活性状态进行中`。
- 字幕输入：通过 `--subtitle-file captions/demo.srt --subtitle-charset UTF-8 --subtitle-style "FontName=SourceHanSans,Fontsize=40"` 可以把 SRT/ASS 直接交给 demo 并写入 filtergraph，方便展示“字幕 + 水印 + PNG”组合；仍缺字幕模板库与字体缺失回退，见 `docs/local_av/ffmpeg_pipeline/unresolved_work.md#6-水印与字幕灵活性状态进行中`。
- 若需保留执行耗时与输出大小，可追加 `--run-status-file artifacts/cli_status.txt`，CLI 会将路径传递给 demo 脚本并生成同名摘要；同时可追加 `--metrics-log artifacts/cli.prom` 让脚本输出伪指标文本（详见 `docs/local_av/ffmpeg_pipeline/pseudo_metrics.md`）；这些记录仍是临时替代品，`ffmpeg_job_duration_seconds` 等指标仍未实现。
- CLI 仍是 demo 封装，尚未与 orchestrator 指令/工具注册联动；若需自定义滤镜仍需改脚本（缺口记录于 `docs/local_av/ffmpeg_pipeline/unresolved_work.md`）。

### 1. 用 demo 脚本模拟剪辑
```
OUTPUT_PATH=deliverables/demo.mp4 VIDEO_RESOLUTION=1920x1080 SEGMENT_DURATION=2 \
  RUN_STATUS_FILE=artifacts/demo_status.txt METRICS_LOG_PATH=artifacts/demo.prom \
  bash scripts/video_editing_demo.sh
```
- 脚本会生成两段彩色片段 → concat → 加水印与合成正弦配乐。
- 末尾会输出 ffprobe 摘要，可作为“Agent 已完成剪辑任务”的截图/日志。
- 若设置 `RUN_STATUS_FILE=...`，脚本结束时会额外写入包含开始/结束时间、耗时、输出大小以及 GPU 状态的摘要文件，用于在缺少 Prometheus 指标时人工补齐验收记录；若再配置 `METRICS_LOG_PATH=...`，脚本会追加伪 Prometheus 指标文本（详见 `pseudo_metrics.md`），方便日后迁移到真实指标；注意 orchestrator 层的 `ffmpeg_job_duration_seconds` 指标仍未实现。
- 如需定制颜色/字幕，可直接编辑脚本顶部变量或在 `bash` 里传入 `FFMPEG_BIN=/usr/bin/ffmpeg` 等环境变量。
- 若要复现缺失文件验收（FFM-006），可追加 `SIMULATE_MISSING_INPUT=1`，脚本会提前执行一次带坏 manifest 的 concat 并捕获错误日志，帮助验收未完成项；失败结果同样会写入 `RUN_STATUS_FILE`/`METRICS_LOG_PATH`，便于将缺口记录到验收附件中。
- 需要加载真实素材时，可传入 `SOURCE_MANIFEST=/path/to/segments.txt`（格式与 FFmpeg concat manifest 相同），脚本会跳过彩色片段生成并提示仍需人工处理分辨率/帧率不一致的问题。
- 自定义水印示例：`WATERMARK_TEXT="交付样片" WATERMARK_POSITION=top-right WATERMARK_FONT_SIZE=56 WATERMARK_OPACITY=0.6 WATERMARK_MARGIN=32 WATERMARK_IMAGE_PATH=assets/logo.png WATERMARK_IMAGE_SCALE=0.6 WATERMARK_IMAGE_OPACITY=0.85 bash scripts/video_editing_demo.sh OUTPUT_PATH=deliverables/wm_custom.mp4`；如需完全关闭文字水印，可移除字体或仅保留 PNG overlay，字幕样式/字体回退仍需手工处理。
- 字幕示例：`SUBTITLE_FILE=captions/zh_demo.srt SUBTITLE_CHARSET=UTF-8 SUBTITLE_FORCE_STYLE="FontName=SourceHanSans,Fontsize=42" bash scripts/video_editing_demo.sh OUTPUT_PATH=deliverables/subbed.mp4`；脚本会记录 charset/style 并把 `subtitles` 滤镜插入管线，但仍缺字幕模板库与“无字体环境”自动回退，请参见 `docs/local_av/ffmpeg_pipeline/unresolved_work.md#6-水印与字幕灵活性状态进行中`。
- 演示多轨音频混合：`PRIMARY_AUDIO_PATH=bgm.wav SECONDARY_AUDIO_PATH=voiceover.wav AUDIO_VOLUME=0.6 bash scripts/video_editing_demo.sh OUTPUT_PATH=deliverables/real_mix.mp4`。
- GPU 能力探测：`ENABLE_GPU=1 PREFERRED_GPU_BACKEND=cuda bash scripts/video_editing_demo.sh OUTPUT_PATH=deliverables/gpu_probe.mp4` 会在日志中给出“检测到 nvidia-smi”或“缺少驱动，已回退 CPU”的说明；若提供 `GPU_STATUS_FILE=/tmp/gpu_status.txt`，脚本会生成记录供验收附件使用；搭配 `RUN_STATUS_FILE=/tmp/demo_summary.txt METRICS_LOG_PATH=/tmp/demo.prom` 可补充运行时长 + 伪指标，但 Prometheus 指标仍是缺口。

### 2. 手写 ffmpeg 管线（适合 code_execute）
示例：将多段 MP4 缩放到 1080p 并加转场（适合 `code_execute` 运行 Bash 脚本）。
```
cat <<'LIST' > segments.txt
file 'clips/intro.mp4'
file 'clips/interview.mp4'
file 'clips/outro.mp4'
LIST

bash -lc "ffmpeg -y -safe 0 -f concat -i segments.txt \ \
  -vf 'scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2,format=yuv420p,xfade=transition=fade:duration=1:offset=4' \ \
  -c:v libx264 -preset medium -crf 18 -c:a aac -b:a 192k deliverables/final.mp4"
```
- 若需要水印可追加 `-i watermark.png -filter_complex "[0:v][1:v]overlay=W-w-48:H-h-48"`。
- 添加背景音乐：`-i music.wav -filter_complex "[0:a][1:a]amix=inputs=2:duration=longest"`。

### 3. `code_execute` 交互示例
当需要完整脚本（包含生成素材、转码、校验）时，可直接在 `code_execute` 中粘贴多行 Bash：
```python
code_execute(
    language="bash",
    code=r"""
set -euo pipefail
OUTPUT_PATH=${OUTPUT_PATH:-deliverables/from_code_exec.mp4}
SEGMENT_DURATION=${SEGMENT_DURATION:-1.5}
VIDEO_RESOLUTION=${VIDEO_RESOLUTION:-1280x720}
RUN_STATUS_FILE=${RUN_STATUS_FILE:-artifacts/from_code_exec_status.txt}
METRICS_LOG_PATH=${METRICS_LOG_PATH:-artifacts/from_code_exec.prom}
bash scripts/video_editing_demo.sh
cat "$RUN_STATUS_FILE"
cat "$METRICS_LOG_PATH"
ffprobe -hide_banner -select_streams v:0 -show_entries stream=width,height,r_frame_rate -of default=noprint_wrappers=1 "$OUTPUT_PATH"
"""
)
```
- `code_execute` 会捕捉 stdout/stderr，适合长日志或需要在一次调用中串起多个命令的场景。
- 若要定制滤镜，可在调用脚本前 `cat` 新的 `filter_complex` 到临时文件并传给 `ffmpeg`。
- 需要加载用户素材时，可在脚本前创建 manifest：
  ```python
  code_execute(
      language="bash",
      code=r"""
  set -euo pipefail
  cat <<'LIST' > /tmp/user_segments.txt
  file 'assets/interview.mp4'
  file 'assets/broll.mp4'
  LIST
  PRIMARY_AUDIO_PATH=audio/bgm.wav SECONDARY_AUDIO_PATH=audio/voice.wav \
  SOURCE_MANIFEST=/tmp/user_segments.txt OUTPUT_PATH=deliverables/user_mix.mp4 \
    bash scripts/video_editing_demo.sh
  ffprobe -hide_banner -select_streams v:0 -show_entries stream=codec_name,width,height -of default=noprint_wrappers=1 deliverables/user_mix.mp4
  """,
  )
  ```
  _注意：脚本仅执行 concat + overlay，不会自动对齐分辨率/帧率，若素材不一致需手工添加滤镜；相关缺口记录于 `docs/local_av/ffmpeg_pipeline/unresolved_work.md`。_

### 4. 结果验证与交付
```
bash -lc "ffprobe -hide_banner -select_streams v:0 -show_entries stream=codec_name,width,height,r_frame_rate -of default=noprint_wrappers=1 deliverables/final.mp4"
file_download("deliverables/final.mp4")
```

## 建议流程
1. **需求澄清**：时长、比例、滤镜、字幕、水印、交付格式。
2. **素材检查**：`ffprobe input.mp4`，确认分辨率/帧率一致性；必要时在 `bash` 中调用 `scripts/verify_ffmpeg.sh` 排查环境问题。
3. **脚本化执行**：优先在 `code_execute` 中编写完整的 Python/Bash 脚本（便于复用与追溯），或直接运行 `scripts/video_editing_demo.sh` 作为模板快速复制。
4. **校验与总结**：输出 ffprobe 结果、截图或 GIF 以证明剪辑完成，并记录命令供复测。

## 常见问题
- **字体缺失导致 `drawtext` 报错**：传入 `FONT_FILE=/path/to/font.ttf` 或把文字移到后期工具完成。
- **concat 失败**：确保输入编码一致，或改用 `-filter_complex concat=n=3:v=1:a=1`。
- **音画不同步**：使用 `-vsync 2`、`aresample=async=1` 等参数，必要时在脚本中对齐帧率。

## 验收技巧
- 运行 `scripts/video_editing_demo.sh OUTPUT_PATH=artifacts/demo.mp4`，再 `file_download("artifacts/demo.mp4")`，即可给用户展示 Agent 在当前环境中真实拼接出的样片。
- 对真实素材，保留 `segments.txt`、ffmpeg 日志与 ffprobe 摘要，便于提交验收报告。

## 未完成与后续计划
- **GPU 自动探测**：demo/CLI 现已支持 `ENABLE_GPU=1`/`--enable-gpu`，可检测 `nvidia-smi` 并切换 `h264_nvenc`；但 VAAPI/多 GPU 管理、失败重试与 orchestrator 的 `use_gpu` 模板仍未落地，相关缺口记录于 `docs/local_av/ffmpeg_pipeline/unresolved_work.md#1-gpu-自动化`。
- **指标与重试**：监控指标（`ffmpeg_job_duration_seconds`, `ffmpeg_retry_total` 等）仍停留在技术方案文档中，尚未落地到 `internal/orchestrator` 或可复用的工具；`RUN_STATUS_FILE` + `METRICS_LOG_PATH` 只是临时的脚本级摘要/伪指标，仍需 Prometheus exporter 才能满足验收。
- **真实素材/多轨管线**：脚本现可加载 `SOURCE_MANIFEST` 并混合 `PRIMARY_AUDIO_PATH`/`SECONDARY_AUDIO_PATH`，但仍缺官方示例素材、字幕文件与自动化滤镜/预检，无法完整覆盖 FFM-002A、FFM-006。
- **自动化错误处理**：虽然 `SIMULATE_MISSING_INPUT=1` 可帮助重现缺失文件日志（FFM-006），但 orchestrator/技能仍缺乏在真实任务中捕获、重试、上报的闭环，需要结合指标与任务状态做成体系化能力。
- **交互封装**：`go run ./cmd/videoedit` 已能封装 demo，但尚未嵌入 UI/工具注册或 orchestrator 的任务系统，仍需提供正式的 `video_edit` 指令/CLI 发布包。
- **水印与字幕灵活性**：`WATERMARK_*` + `WATERMARK_IMAGE_*` 环境变量（以及 CLI 中的 `--watermark-*`、`--watermark-image*`）已覆盖文字与 PNG/logo overlay，最新的 `SUBTITLE_*`/`--subtitle-*` 也能直接渲染单条 SRT/ASS 字幕；但字幕样式切换、预设库、缺字体环境的自动回退与多字幕层封装仍未交付，详见 `docs/local_av/ffmpeg_pipeline/unresolved_work.md#6-水印与字幕灵活性状态进行中`。

有关整体完成度可参见 `docs/local_av/ffmpeg_pipeline/status.md`，若需查看逐用例/指标的完成情况与缺口，请查阅 `docs/local_av/ffmpeg_pipeline/gap_matrix.md` 以及 `docs/local_av/ffmpeg_pipeline/unresolved_work.md`。
