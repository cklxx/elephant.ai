# 视频处理与 FFmpeg 管线技术方案

## 1. 职责范围
- 负责视频拼接、转场、调色、字幕/水印叠加以及最终封装。  
- 支持 CPU 与 GPU 编码器自动选择，暴露可配置模板。  
- 处理阶段包括：素材预检查 → 统一参数 → 拼接 → 滤镜 → 导出。

## 2. 组件拆分
| 组件 | 描述 | 核心能力 |
| --- | --- | --- |
| `internal/ffmpeg` | 通用执行器与探测器，封装 `ffmpeg`/`ffprobe` | 构建命令、控制 Dry-run、采集日志、元数据预检 |
| `internal/ffmpeg/preset.go` | 拼接/封装模板加载 | `PresetLibrary`, `Preset.Args` |
| `internal/ffmpeg/probe.go` | ffprobe 调用及解析 | `LocalProber`, `ProbeResult` |
| `configs/ffmpeg/*.yaml` | 转码模板（分辨率、码率、编码器） | `default_1080p`, `social_720p` |

## 3. 流程详解
1. **素材检测**：通过 `ffprobe` 获取分辨率、帧率，写入检查报告（本地 CLI 已对接 `LocalProber` 自动执行）。
2. **参数统一**：若素材不一致，执行预处理（scale、fps、pix_fmt）；当未显式声明滤镜或模板时直接阻断不兼容输入，提示操作人补齐转换。
3. **拼接策略**：
   - 同编码：使用 concat demuxer；
   - 异编码：使用 `filter_complex` + `concat=n`.  
4. **滤镜管线**：
   - 转场：`xfade`, `fade`。  
   - 调色：LUT、`eq`。  
   - 水印：`overlay`。  
5. **输出封装**：
   - H.264/H.265 + AAC；
   - 根据模板控制码率、关键帧间隔、B 帧数量。  
6. **校验**：生成 `mediainfo` 报告，和模板比对。

## 4. 命令模板
```yaml
preset: default_1080p
video_codec: h264
video_bitrate: 6M
audio_codec: aac
audio_bitrate: 192k
pixel_format: yuv420p
keyint: 48
```

执行时合成命令：
```bash
ffmpeg -y -safe 0 -f concat -i segments.txt -vf "scale=1920:1080,format=yuv420p" \
  -c:v h264 -b:v 6M -g 48 -c:a aac -b:a 192k output.mp4
```

## 5. GPU 支持
- 检测 `nvidia-smi` / `vainfo`，在模板中启用 `use_gpu: true`。  
- 映射表：
  - `h264_nvenc`: 直播/快速导出；
  - `hevc_nvenc`: 4K/HDR；
  - `h264_qsv`: Intel QuickSync。  
- 自动回退：若 GPU 不可用，记录警告并使用 CPU 模式。

## 6. 异常处理
- 检测 FFmpeg 返回码，若为 1~255 记录 stderr。  
- 常见错误：
  - `Non-monotonous DTS`: 自动插入 `-fflags +genpts`。  
  - `Impossible to open`: 检查路径/权限。  
- 重试策略：区分可重试/不可重试错误，最多重试 2 次。

## 7. 指标
- `ffmpeg_job_duration_seconds{job_type="concat"}`。  
- `ffmpeg_gpu_sessions_total`。  
- `ffmpeg_retry_total`。
- `ffmpeg_video_precheck_failures_total`（预检失败次数，后续接 Prometheus）。

## 8. 迭代计划
1. MVP：拼接 + 模板导出。
2. v1：滤镜模块化、GPU 自动探测。
3. v2：并行帧处理、智能 B-roll 插入。
4. v3：基于 FFmpeg filtergraph 的 DSL 编辑器。

## 9. 任务进度
- [x] 本地 FFmpeg Executor、concat/mux 调用封装。
- [x] 编排阶段完成基础拼接路径。
- [x] 模板化导出（Preset Library）与 ffprobe 预检查。
- [ ] GPU 自动探测与高级滤镜。
