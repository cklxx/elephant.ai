---
name: ppt_deck
description: PPT 产出 playbook（目标/受众→故事线→版式设计→可访问性→交付），含 10/20/30 与 Microsoft Accessibility Checker 最佳实践。
---

# PPT 产出（从目标到可交付 Deck）

> 目标：用“可讲述的故事线 + 可阅读的版式 + 可访问的细节 + 可复用的模板”交付一份可演示、可评审、可二次编辑的 PPT/Slides。

## 0. 何时使用

- 需要把一个主题变成可演示的 deck（汇报/方案/培训/融资/复盘）。
- 需要形成可复用模板（品牌规范、字体、配色、版式网格）。
- 需要面向更广泛人群交付（可访问性：标题、阅读顺序、对比度、alt text 等）。

## 1. 必问信息（需求输入）

1. **受众与目标**：听众是谁？希望他们在会后做什么？（决策/批准/理解/行动）
2. **场景与时长**：现场讲/线上讲/发文档自读？总时长与 Q&A 时间？
3. **交付格式**：PPTX / PDF / Google Slides / Web（reveal.js）；是否需要讲稿/备注。
4. **品牌规范**：字体、主/辅色、Logo 使用、图标风格、图表风格、封面样式。
5. **内容资产**：已有数据、图表、截图、案例、引用来源（需要可追溯）。

## 2. 结构设计（先讲清楚，再做漂亮）

### 2.1 建议的故事线框架（可选其一）

- **SCQA**：Situation → Complication → Question → Answer（适合问题解决/方案类）。
- **金字塔结构**：先结论后论据，逐层展开（适合管理汇报/决策）。
- **Before/After/Bridge**：现状 → 目标 → 路线图（适合转型/规划）。

### 2.2 常用 deck 结构模板（可直接套用）

1. 封面（标题 + 一句话结论 + 署名/日期）
2. TL;DR（3–5 条要点）
3. 背景/现状（数据与事实）
4. 问题/机会（为什么现在要做）
5. 目标与原则（成功标准）
6. 方案（可选项对比 + 推荐项）
7. 实施计划（里程碑/资源/风险）
8. 指标与复盘方式（如何衡量）
9. 结尾 CTA（需要对方做什么）
10. 附录（数据口径/术语/细节）

### 2.3 10/20/30 作为“强约束”参考

在需要强节奏、强聚焦的 pitch 场景：可把“10 张以内 / 20 分钟内 / 字号不小于 30pt”作为硬约束，逼迫信息聚焦（不是所有汇报都必须严格遵守，但对抗“信息堆叠”非常有效）。

## 3. 视觉与版式（可读性优先）

可复用的设计规则（建议写入模板）：

- **一页一个观点**：每页只服务一个结论；标题写“结论句”，不要只写名词。
- **大字号**：正文尽量 ≥ 18pt（培训/自读场景更大），关键数字更大。
- **对比度**：文本与背景保持高对比；不要只用颜色表达差异（色弱/投影环境会丢信息）。
- **网格与对齐**：统一边距、对齐线、间距；宁可留白，不要塞满。
- **图表减法**：去掉网格线/3D/装饰；突出关键数据点与趋势。

## 4. 可访问性（默认打开 Accessibility Checker 的心智）

PowerPoint/Office 的可访问性最佳实践（建议作为发布前必检项）：

- **每页唯一标题**（方便导航与屏幕阅读器）
- **阅读顺序正确**（元素添加顺序 ≠ 视觉顺序，需要检查并调整）
- **所有视觉元素加 alt text**（图片/图表/形状/图标）
- **颜色不是唯一信息载体**（用文本/形状/纹理辅助表达）
- **足够对比度**（文字/背景）
- **字体与留白**：较大字号、无衬线字体、避免拥挤（提升可读性）

## 5. 生产与交付（让 deck 可用、可改、可验收）

### 5.1 交付物建议最小集合

- `deck.pptx`：可编辑源文件
- `deck.pdf`：方便预览/发文档
- `notes.md`：讲稿/旁白（可从 speaker notes 导出）
- `SOURCES.md`：数据/图片/引用来源（可审计）

### 5.2 自动化生成（按环境选路线）

- **离线 PPTX**：`python-pptx` 生成结构化页面 + 插图/图表（适合批量生成/模板化）。
- **Web deck**：Markdown → reveal.js（适合工程化版本管理与 CI 导出 PDF）。
- **协作交付**：Google Slides API（适合在线协作与共享链接）。
- **纯图片 PPTX（Alex 工具链）**：先用 `text_to_image` 生成每页 slide 的图片（建议 16:9，如 `1600x900`），再用 `pptx_from_images` 把图片按顺序拼成 `deck.pptx`（适合快速出稿/可演示；后续可在 PowerPoint/Keynote 里补充可编辑元素）。

示例（两步走）：

1) 生成 slide 图片（每页一张，保持风格一致）

```json
{"prompt":"封面：主题 + 副标题，极简风格，16:9，留白，深色背景，标题可读","size":"1600x900"}
```

2) 组装 PPTX（纯图片页）

```json
{"images":["[doubao_seedream-..._0.png]","[doubao_seedream-..._1.png]"],"output_name":"deck.pptx"}
```

> 选择原则：先保证“结构与讲述”，再决定工具；不要为了自动化牺牲信息结构与可读性。

## 6. 参考资料（调研摘录）

- Guy Kawasaki：10/20/30 规则原文：https://guykawasaki.com/the_102030_rule/
- Microsoft：让 PowerPoint 更可访问（alt text/阅读顺序/对比度/标题等）：https://support.microsoft.com/en-us/office/make-your-powerpoint-presentations-accessible-to-people-with-disabilities-6f7772b2-2f33-4bd2-8ca7-dae3b2b3ef25
- Microsoft：Accessibility Checker 使用与修复流程：https://support.microsoft.com/en-us/office/improve-accessibility-with-the-accessibility-checker-a16f6de0-2f39-4a2b-8bd8-5ad801426c7f
- Presentation Zen（演示设计与叙事）：http://www.presentationzen.com/
