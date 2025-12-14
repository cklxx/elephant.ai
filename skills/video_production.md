---
name: video_production
description: 端到端视频制作 playbook（策划→素材规范→剪辑→字幕→导出→发布→验收），并映射到本仓库的 ffmpeg/视频工具链。
---

# 视频制作（从 Brief 到交付）

> 目标：把“想法/素材/约束”变成可验收的成片（含技术参数、字幕/封面、发布与验收清单），并在需要时落到本仓库提供的 `ffmpeg`/`cmd/videoedit` 管线。

## 0. 何时使用

- 需要把碎片素材（口播/B-roll/屏录/图片/字幕）剪成成片并交付。
- 需要适配不同平台（横屏 16:9、竖屏 9:16、方形 1:1）并遵循导出规格。
- 需要“可复现”的交付：素材清单、剪辑决策、导出参数、验收记录可追溯。

## 1. 必问信息（需求输入）

1. **受众与目标**：你希望观众做什么（了解/转化/培训/投放）？成功指标是什么？
2. **平台与比例**：YouTube / B 站 / TikTok / Reels / 内部培训；横屏/竖屏/方形；目标分辨率与帧率。
3. **时长与结构**：开头 hook（前 3–10 秒）、主体段落、结尾 CTA；是否需要章节/时间轴。
4. **品牌规范**：字体（含字体文件）、色板、Logo、水印位置、片头片尾、字幕样式（字号/描边/阴影/高亮规则）。
5. **音频要求**：是否需要降噪/配乐/旁白；目标响度与 true peak（线上常用参考：-14 LUFS / -1 dBTP）。
6. **交付物**：MP4 + 字幕（SRT/VTT）+ 封面图 + 变更说明；是否需要“快启播/fast start”。

## 2. 工作流（端到端）

### 2.1 预制作（Pre-production）

- 产出三件事（尽量结构化）：
  - **脚本**：旁白/台词 + 强调词 + 停顿点。
  - **镜头单**：镜头类型、时长、所需素材（B-roll/截图/屏录）。
  - **资产清单**：素材来源、授权/版权、logo/字体/配乐许可。

### 2.2 素材预检（Ingest & QC）

对每个输入素材做一次 `ffprobe` 抽检，记录：
- 分辨率、帧率、像素宽高比、时长
- 编码器/码率；音频采样率/声道；是否可变帧率（VFR）

处理原则：
- **帧率**：优先“录多少就导多少”，避免无意义变帧率；平台建议上传帧率与录制一致。
- **隔行**：先去隔行再剪。
- **分辨率不一致**：先统一到目标分辨率/比例，再进入剪辑阶段（减少后续返工）。

### 2.3 剪辑与合成（Edit）

推荐顺序：粗剪 → 精剪 → 声音 → 画面 → 字幕/图形。

- **粗剪**：先把结构跑通（开场→主体→结尾），只做必要裁切。
- **精剪**：控制节奏与信息密度（口播尽量“可理解、可跟上、可复述”）。
- **声音优先**：人声清晰优先于画面“高级感”；配乐要给人声留空间。
- **画面统一**：做最小必要调色（统一曝光/白平衡/对比度）；避免“每段素材像不同片子”。
- **字幕**：移动端优先（字号、对比度、安全边距）；关键术语一致。

### 2.4 导出（Export）

通用在线发布导出建议（以 YouTube 为代表的参考配置）：
- 容器：MP4
- 视频：H.264（progressive / high profile）；常见 4:2:0
- 音频：AAC；48kHz
- fast start：`moov atom` 前置

YouTube（SDR）常见上传码率参考：
- 1080p：约 8 Mbps（24/25/30fps）或 12 Mbps（48/50/60fps）
- 4K：约 35–45 Mbps（24/25/30fps）或 53–68 Mbps（48/50/60fps）

音频响度（线上参考值）：
- Integrated：-14 LUFS
- True peak：≤ -1 dBTP

### 2.5 字幕与可访问性（Captions）

- 对公开视频/培训视频：建议交付字幕文件（SRT/VTT），并准备“烧录字幕版”与“独立字幕文件版”两份。
- 字幕应覆盖对理解重要的对话与关键音效（可访问性基本要求）。

### 2.6 发布与验收（Delivery）

交付物建议最小集合：
- 成片：`final.mp4`
- 字幕：`final.srt` / `final.vtt`
- 封面图：`thumbnail.png`
- 变更说明：`CHANGELOG.md`（结构/节奏/字幕/导出参数/已知限制）

验收清单（建议逐条勾选）：
- 画面：无黑帧；无明显跳剪/穿帮；比例正确；移动端 UI 不遮挡关键信息
- 声音：无爆音；人声明晰；响度/true peak 达标
- 字幕：时间轴对齐；关键术语一致；移动端可读
- 导出：帧率与源一致；码率/格式符合平台建议；fast start 生效

## 3. 本仓库工具链映射（让流程“真能跑通”）

当需要脚本化、可复现的剪辑合成，本仓库提供：

- `cmd/videoedit`：封装 `scripts/video_editing_demo.sh` 的 CLI（拼接/水印/混音/字幕）。
- `scripts/video_editing_demo.sh`：可用于生成样片、套模板、扩展滤镜链。
- `ffmpeg` / `ffprobe`：素材预检、转码、拼接、字幕烧录与导出。

示例：一键跑通样片（用于验收“环境可剪辑、可导出”）

```bash
go run ./cmd/videoedit --output deliverables/demo.mp4
```

## 4. 参考资料（调研摘录）

- YouTube 推荐上传编码参数（容器/编码器/帧率/码率/fast start）：http://support.google.com/youtube/answer/1722171
- Adobe：导出参数权衡（H.264/帧率匹配/码率建议/比例建议）：https://www.adobe.com/creativecloud/video/hub/guides/best-export-settings-for-premiere-pro.html
- APU：YouTube 响度目标（-14 LUFS / -1 dBTP）与“通常只衰减不提升”的注意点：https://apu.software/youtube-audio-loudness-target/
- APU：常见平台响度目标表（含 EBU R128 等参考）：https://apu.software/loudness-standards/
- W3C WAI：字幕（Prerecorded）理解文档与资源汇总：https://www.w3.org/WAI/WCAG22/Understanding/captions-prerecorded.html
