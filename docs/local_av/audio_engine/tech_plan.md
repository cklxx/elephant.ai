# 音频混音引擎技术方案

## 1. 模块职责
- 管理多音轨混音、音量包络、降噪与动态处理。  
- 统一生成最终合成音轨，供视频封装或独立导出。  
- 支持 TTS、BGM、环境音等多源素材的时间轴对齐与淡入淡出。

## 2. 组件结构
| 子模块 | 职责 | 核心功能 |
| --- | --- | --- |
| `internal/audio` | 混音逻辑、滤镜链构建 | `Engine.Mixdown`, `applyLoudnorm`, 包络滤镜 |
| `configs/audio/presets.yaml` | 声道布局、效果链模板 | `dialogue_mix`, `podcast_mix` |
| `internal/task` | 任务模型定义 | `AudioTrack.Envelope`, `EnvelopeSpec` |

## 3. 混音流程
1. **轨道解析**：根据 `AudioTrack` 定义（类型、起始时间、音量、是否循环）。  
2. **时间轴对齐**：将 TTS 结果和 BGM 对齐，填充静音段。  
3. **滤镜链**：
   - 降噪：`anlmdn`。
   - 均衡：`equalizer`。
   - 压缩：`acompressor`。
   - 归一化：`loudnorm` 双遍处理（首遍统计缓存，次遍带 `measured_*` 参数输出）。
    - 包络：`afade` + 分段 `volume=...:enable='between(t,...)'`。
4. **混音输出**：使用 `amix=inputs=n`，并在输出阶段设置 `dynaudnorm`（可选）。

## 4. 数据结构
```yaml
audio:
  mixdown_output: build/output/mix.wav
  loudness_target: -16
  tracks:
    - name: narration
      source: tts:narration
      gain: 0
      offset: 0s
      effects: [anlmdn, eq_dialogue]
    - name: bgm
      source: assets/music.mp3
      gain: -8
      offset: 0s
      loop: true
      effects: [ducking]
      envelope:
        fade_in: 2s
        fade_out:
          start: 55s
          duration: 5s
        segments:
          - start: 0s
            end: 12s
            gain: -10
          - start: 12s
            gain: -4
```

## 5. 算法要点
- **Ducking**：
  - 依据主轨包络生成背景音乐衰减曲线。
  - 通过 `sidechaincompress` 或手动调制 `volume` 滤镜实现。  
- **循环处理**：BGM 支持循环填充，使用 `aloop=loop=-1:size=2e9`.
- **双遍响度**：
  - 首遍 `print_format=json` 生成统计，写入内存缓存，命中后跳过再次测量。
  - 次遍将 `measured_I/LRA/TP/thresh` 与 `offset` 注入 `loudnorm`，保留 `linear=true`。
- **淡入淡出**：`afade=t=in:ss=0:d=1.5` 与 `afade=t=out:st=<start>:d=<duration>`。
- **包络分段**：通过 `volume=volume=<multiplier>:enable='between(t,start,end)'` 构造区间增益；无 `end` 时退化为 `gte(t,start)`。

## 6. 性能优化
- 尽量合并滤镜链，避免多次读写。  
- 对长音频采用分段归一化策略，降低内存占用。  
- 支持将临时缓存写入 RAM Disk（可配置）。

## 7. 错误与重试
- 检测音频采样率不一致 → 自动插入 `aresample=48000`.  
- 若 TTS 轨道缺失 → 抛出 `ErrMissingDependency`，由编排层决定是否降级。  
- 对 `loudnorm` 双遍流程，若第一次失败则降级为单遍。

## 8. 迭代路线
1. MVP：amix + loudnorm 单遍。
2. v1：ducking、循环、包络编辑。
3. v2：多声道支持（5.1/7.1）、对话检测自动增益。
4. v3：插件化效果链，GUI 波形编辑。

## 9. 任务进度
- [x] amix 单遍混音管线与基础效果映射。
- [x] Loop/ducking 支持并接入 orchestrator。
- [x] 双遍 loudnorm 与测量缓存。
- [x] 包络编辑。
- [ ] 多声道/自动增益与预设体系。
