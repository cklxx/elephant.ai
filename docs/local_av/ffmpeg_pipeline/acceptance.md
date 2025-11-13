# 视频处理与 FFmpeg 管线验收方案

## 1. 环境准备
- FFmpeg ≥ 6.1，确保启用常用滤镜（`xfade`, `frei0r`, `loudnorm`）。  
- 安装 `ffprobe`, `mediainfo` 用于验收校验。  
- GPU 验收需具备 NVIDIA 驱动 535+ 或 Intel Media SDK。  
- 准备 4 段测试素材（不同帧率/分辨率）与水印 PNG。

## 2. 验收用例
| 编号 | 场景 | 操作 | 判定标准 |
| --- | --- | --- | --- |
| FFM-001 | 同编码拼接 | 使用 concat demuxer | 输出时长 == 各片段时长之和 ±0.1s |
| FFM-002 | 异编码拼接 | 自动触发 filter_complex | 输出帧率、分辨率与模板一致 |
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
- 转码模板样例 (`configs/ffmpeg/default_1080p.yaml`)。  
- 验收脚本：`scripts/verify_ffmpeg.sh`，自动对比输出参数。  
- GPU 验收记录：`gpu_capability.json`。

## 6. 验收流程
1. 实施方按顺序执行用例，保存命令与日志。  
2. 甲方复核输出文件元数据与截图。  
3. 双方确认性能数据，必要时进行二次测试。  
4. 填写验收报告并归档素材样本。
