---
name: image-creation
description: AI 图片生成与迭代优化（文生图、图生图、风格迁移）。
triggers:
  intent_patterns:
    - "生成图片|画|draw|image|图片|插图|illustration|设计图|海报"
  context_signals:
    keywords: ["图片", "image", "draw", "画", "生成", "设计"]
  confidence_threshold: 0.6
priority: 7
exclusive_group: image
requires_tools: [bash, write]
max_tokens: 200
cooldown: 30
output:
  format: markdown
  artifacts: true
  artifact_type: image
---

# image-creation

AI 图片生成：文生图 + 图生图 + 风格迁移。

## 调用

```bash
# 文生图
python3 skills/image-creation/run.py '{"action":"generate", "prompt":"一只在月光下的白猫", "style":"realistic"}'

# 图生图（风格迁移）
python3 skills/image-creation/run.py '{"action":"refine", "image_path":"/tmp/cat.png", "prompt":"添加星空背景"}'
```

## 参数

### generate
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| prompt | string | 是 | 图片描述 |
| style | string | 否 | 风格（realistic/anime/oil-painting），默认 realistic |
| size | string | 否 | 尺寸（1024x1024/512x512），默认 1024x1024 |
| output | string | 否 | 输出文件路径 |

### refine
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| image_path | string | 是 | 原图路径 |
| prompt | string | 是 | 修改指令 |
