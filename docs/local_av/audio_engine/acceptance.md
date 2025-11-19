# 音频混音引擎验收方案
> Last updated: 2025-11-18


## 1. 环境准备
- FFmpeg 支持 `loudnorm`, `anlmdn`, `sidechaincompress`。  
- SoX ≥ 14.4（可用于波形对比）。  
- 准备素材：旁白（干声）、BGM、环境音、音效。  
- 校准监听环境：使用耳机或监听音箱，音量统一。

## 2. 功能用例
| 编号 | 场景 | 操作 | 预期结果 |
| --- | --- | --- | --- |
| AUD-001 | 基础混音 | 执行 Mixdown | 输出单一 WAV，电平峰值 < -1dBFS |
| AUD-002 | Ducking | 主轨 + BGM，启用 ducking | 主轨说话时 BGM 至少降低 8dB |
| AUD-003 | 循环 BGM | 配置 `loop: true` | 输出时长覆盖全片，无突兀接缝 |
| AUD-004 | 效果链 | 应用 `anlmdn` + `eq_dialogue` | 信噪比提升 ≥ 6dB（与原始对比） |
| AUD-005 | TTS 缺失 | 删除 TTS 输出 | 系统返回可读错误并标记依赖缺失 |
| AUD-006 | 双遍响度缓存 | 同一素材连续执行两次 Mixdown | 首次日志输出 `cache=false`，第二次为 `cache=true`，响度仍命中目标 |
| AUD-007 | 包络编辑 | 配置淡入淡出 + 分段增益 | 输出波形在指定时间窗口内按增益变化，淡入淡出平滑 |

## 3. 性能指标
- 单次混音时长 ≤ 音频长度的 1.2 倍。  
- `loudnorm` 双遍缓存命中率 ≥ 80%（通过 Prometheus 或日志中 `cache=true/false` 统计）。
- 混音后的 LUFS ≈ 目标值 ±1。

## 4. 验收工具
- `ffmpeg -i mix.wav -filter_complex ebur128 -f null -`：验证响度。  
- `sox mix.wav -n stat`：获取动态范围。  
- `audacity`：人工抽查波形。

## 5. 交付物
- 混音模板样例与说明。  
- 自动化验收脚本：`scripts/verify_audio_mix.sh`。  
- 测试结果记录表：包含 LUFS、峰值、噪声指标与缓存命中率。
  - 包络验证：记录关键时间戳的电平测量值（可通过 `sox ... stat -freq` 或 `ffmpeg volumedetect`）。

## 6. 流程
1. 实施方执行全部用例并记录指标。  
2. 甲方抽样试听并校验指标脚本输出。  
3. 双方签字确认，归档脚本与日志。
