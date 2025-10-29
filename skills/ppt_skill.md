# PPT 生成技能

## 技能目标
- 基于用户提供的主题、受众和风格要求，规划完整的幻灯片结构。
- 选择合适的内容生成与排版方案，自动化生产 PPTX 或网页版演示文稿。
- 将最终文件或可视化结果返回给用户，并提供复核与二次编辑入口。

## 推荐方案：python-pptx
`python-pptx` 是在离线环境下生成标准 PPTX 幻灯片的首选方案，具备以下优势：

- **控制力**：支持对版式、文本样式、图片、图表进行逐页控制，便于精细排版。
- **可交付性**：输出文件直接兼容 Microsoft PowerPoint、WPS、Google Slides 等常见工具。
- **本地化**：无需外部 SaaS 平台或网络授权，安装依赖即可运行。

> 以下流程默认采用 `python-pptx`，仅当用户明确要求其他平台时再考虑替代方案。

## 交互流程
1. **需求澄清**：确认演示主题、目标受众、页数/结构、品牌色、是否需要图表/图像。
2. **内容规划**：产出目录草案，给用户确认。必要时使用 `todo_update` 跟踪章节。
3. **工具准备**：确认运行环境可安装并使用 `python-pptx`（Python 3.8+，具备文件读写权限）。
4. **素材准备**：
   - 文本内容：直接生成或调用检索工具补充事实。
   - 图像/图表：确认是否需要外部 API（如生成图表数据 -> `matplotlib`/`plotly` -> 导出图片）。
5. **生成演示文稿**：调用对应脚本或 CLI，产出 PPTX/HTML。
6. **验证与交付**：
   - 本地校验文件是否包含指定页数与重点内容。
   - 提供下载路径或可视化链接。
   - 向用户回顾结构并征求修改。

## python-pptx 自动化模板
```bash
pip install python-pptx Pillow
```

```python
from pptx import Presentation
from pptx.util import Inches, Pt

outline = [
    {"title": "封面", "bullets": ["项目名称", "副标题", "演讲者"]},
    {"title": "问题背景", "bullets": ["现状", "挑战", "机会"]},
]

def build_presentation(outline, output_path="output.pptx"):
    prs = Presentation()
    # 主题可选: prs.slide_layouts[0] 封面, [1] 标题+内容
    for index, section in enumerate(outline):
        layout = 0 if index == 0 else 1
        slide = prs.slides.add_slide(prs.slide_layouts[layout])
        slide.shapes.title.text = section["title"]
        if layout == 1:
            body = slide.shapes.placeholders[1].text_frame
            body.clear()
            for bullet in section.get("bullets", []):
                p = body.add_paragraph()
                p.text = bullet
                p.font.size = Pt(24)
    prs.save(output_path)

if __name__ == "__main__":
    build_presentation(outline)
```

- 将规划好的 `outline` 替换为根据用户需求生成的章节列表。
- 如需插入图片，调用 `slide.shapes.add_picture("path", Inches(x), Inches(y), height=...)`。
- 生成后使用 `file_download` 或其他传输手段返回 `output.pptx`。

## 与用户对话时的提示
- 确认文件交付格式（PPTX、PDF、网页链接）。
- 告知所需时间及依赖安装步骤，必要时请求用户确认可以执行安装命令。
- 生成后主动提供修改建议入口，如“需要调整配色或结构吗？”。

## 质量核对清单
- [ ] 页数与章节与确认稿一致。
- [ ] 关键术语、数字来源明确且准确。
- [ ] 所有图片/图表具有合法来源或已获得授权。
- [ ] 演示文件可以正常打开且版式未错位。

## 进一步扩展
- 可结合图像生成模型（如 Stable Diffusion）生成配图，再嵌入到幻灯片。
- 若需自动语音讲解，可生成脚本并调用 TTS 工具产出音频。
- 将幻灯片转为网页展示时，可考虑使用 `reveal.js` 或 `Deck.js`。
