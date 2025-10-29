# PPT 生成技能

> 一句话总结：利用 `python-pptx`、`reveal-md` 或 Google Slides API 等现成方案，通过清晰的需求收集 → 结构规划 → 自动化脚本生成 → 复核交付的流程，快速构建符合用户要求的演示文稿。

## 技能目标
- 基于用户提供的主题、受众和风格要求，规划完整的幻灯片结构。
- 选择合适的内容生成与排版方案，自动化生产 PPTX 或网页版演示文稿。
- 将最终文件或可视化结果返回给用户，并提供复核与二次编辑入口。

## 工作流程
1. **需求澄清**：确认演示主题、目标受众、页数/结构、品牌色、是否需要图表/图片/视频以及交付格式（PPTX、PDF、网页）。
2. **资源检查**：确认运行环境可执行 Python、Node.js 或具备外网访问（如需调用 Google Slides API），并检查写文件权限。
3. **内容规划**：生成目录草案并请用户确认；可通过 `todo_update` 记录章节和行动项。
4. **生成实现**：根据场景选择对应工具（见下文），按照步骤运行脚本或 CLI。
5. **验证与交付**：
   - 自检页数、要点、格式是否符合需求。
   - 输出文件提供下载（`file_download`）、或给出在线地址（如本地启动 `reveal-md` 预览链接）。
   - 反馈总结并征求下一步修改意见。

## 方案一：python-pptx（离线 PPTX 生成）
- **适用场景**：需要本地生成标准 PPTX；无外网依赖。
- **关键能力**：完全控制每页版式、文本、图片、图表。

### 步骤
1. 安装依赖：
   ```bash
   pip install python-pptx Pillow matplotlib
   ```
2. 准备结构化大纲（根据需求澄清结果生成）：
   ```python
   outline = [
       {"title": "封面", "bullets": ["项目名称", "副标题", "演讲者"]},
       {"title": "问题背景", "bullets": ["现状", "挑战", "机会"]},
       {"title": "解决方案", "bullets": ["方案概述", "核心亮点", "实施路径"]},
   ]
   ```
3. 调用脚本生成 PPTX：
   ```python
   from pptx import Presentation
   from pptx.util import Inches, Pt

   def build_presentation(outline, output_path="output.pptx"):
       prs = Presentation()
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
4. 如需添加图片或图表：
   ```python
   slide = prs.slides.add_slide(prs.slide_layouts[5])  # 空白布局
   slide.shapes.add_picture("chart.png", Inches(1), Inches(1), height=Inches(4))
   ```
5. 调用 `file_download("output.pptx")` 将结果提供给用户。

> 生成图表的常见方式：使用 `matplotlib`/`seaborn` 绘制保存为 PNG，再插入到幻灯片中。

## 方案二：reveal-md（快速生成网页演示 + 可导出 PDF）
- **适用场景**：希望获得响应式网页演示，或通过浏览器另存为 PDF。
- **关键能力**：基于 Markdown 编写内容，通过命令行转换为 `reveal.js` 页面。

### 步骤
1. 安装 Node.js 环境后执行：
   ```bash
   npm install -g reveal-md
   ```
2. 生成 Markdown：
   ```markdown
   # 封面
   项目名称\n演讲者

   ---

   ## 问题背景
   - 现状
   - 挑战
   - 机会
   ```
3. 启动本地预览并导出：
   ```bash
   reveal-md slides.md --print pdf --static dist
   ```
   - 使用浏览器打开 `http://localhost:1948` 进行预览。
   - `dist` 目录下的 HTML/资源可打包交付，或通过 `--print` 生成 PDF。
4. 给用户返回生成的网页链接（本地可通过隧道或部署静态站点），或提供 PDF 下载。

## 方案三：Google Slides API（云端协作）
- **适用场景**：用户希望在线协作、共享链接或使用 Google Workspace。
- **关键能力**：通过 OAuth2 凭证调用 Google Slides API 创建演示文稿。

### 步骤
1. 在 Google Cloud Console 创建项目、启用 Slides API，并下载 OAuth 客户端凭证。
2. 安装依赖：
   ```bash
   pip install google-api-python-client google-auth-httplib2 google-auth-oauthlib
   ```
3. 使用脚本创建幻灯片：
   ```python
   from google.oauth2 import service_account
   from googleapiclient.discovery import build

   SCOPES = ["https://www.googleapis.com/auth/presentations"]
   creds = service_account.Credentials.from_service_account_file(
       "service-account.json", scopes=SCOPES
   )
   slides = build("slides", "v1", credentials=creds)

   presentation = slides.presentations().create(body={"title": "示例演示"}).execute()
   presentation_id = presentation["presentationId"]

   requests = [
       {
           "createSlide": {
               "slideLayoutReference": {"predefinedLayout": "TITLE_AND_BODY"}
           }
       }
   ]

   slides.presentations().batchUpdate(
       presentationId=presentation_id, body={"requests": requests}
   ).execute()

   print(f"https://docs.google.com/presentation/d/{presentation_id}/edit")
   ```
4. 将生成的链接分享给用户，并说明可在线编辑/导出。

## 与用户沟通要点
- 交付格式、语言、品牌指南、配色和字体偏好需要确认清楚。
- 若需安装依赖或访问外部 API，提前征求同意并解释安全影响。
- 引导用户进行最终审阅，并提供进一步修改或迭代的选项。

## 质量核对清单
- [ ] 页数、章节结构与确认稿一致，标题/内容逻辑连贯。
- [ ] 数据、引用、图像来源合法且注明出处。
- [ ] 视觉风格（颜色、字体、版式）符合用户要求且在全局保持一致。
- [ ] 交付文件能够正常打开（PPTX/PDF/网页），并附带清晰的使用说明。

## 进一步扩展
- 结合图像生成模型（Stable Diffusion、DALL·E 等）自动生成插图，保存后插入对应页面。
- 调用语音合成（TTS）生成讲稿音频，或使用 `pptx.util` 添加演讲备注。
- 使用自动化测试（例如脚本检查页数、标题命名）保障交付质量。
