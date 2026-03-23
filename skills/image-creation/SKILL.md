---
name: image-creation
description: When the user wants to create or edit images → generate with Seedream (text-to-image + image-to-image).
triggers:
  intent_patterns:
    - "生成图片|画|draw|image|图片|插图|illustration|设计图|海报"
    - "帮我画.*|画一个|画个|generate.*image|create.*image"
    - "logo|图标设计|icon.*design|封面|cover.*image|thumbnail"
    - "壁纸|wallpaper|背景图|banner|头像|avatar|profile.*pic"
    - "效果图|mockup|示意图|概念图|concept.*art"
    - "修改.*图片|edit.*image|换个.*风格|style.*transfer|重新生成"
    - "图生图|image.*to.*image|以图生图|参考.*这张"
  context_signals:
    keywords: ["图片", "image", "draw", "画", "生成", "设计", "logo", "海报", "封面", "banner", "头像", "壁纸", "概念图", "mockup"]
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

Generate images via Seedream.

## Required Env
- `ARK_API_KEY` (required)
- `SEEDREAM_TEXT_ENDPOINT_ID` (optional; fallback: `SEEDREAM_TEXT_MODEL` -> built-in default model)
- `SEEDREAM_I2I_ENDPOINT_ID` (required for `refine`)

## Constraints
- Backend minimum pixels: `1920*1920`. Smaller inputs (for example `1024x1024`) are auto-upscaled.
- `success=true` only when the output file is actually written and non-empty.
- Backend response must contain `b64_json` or `url`; otherwise the call fails.
- Default output path is `/tmp` unless `output` is provided.
- `watermark` defaults to `false` (no "AI generated" watermark). Set to `true` only when you explicitly need watermark.

## Usage

```bash
# Text to image
python3 skills/image-creation/run.py generate --prompt 'white cat in moonlight' --style realistic --watermark false

# Image to image
python3 skills/image-creation/run.py refine --image_path /tmp/cat.png --prompt 'add starry sky background' --watermark false
```

## Parameters

### generate
| name | type | required | notes |
|------|------|------|------|
| prompt | string | yes | image description |
| style | string | no | style tag (default: `realistic`) |
| size | string | no | `WIDTHxHEIGHT`, default `1920x1920` |
| watermark | bool | no | default `false`; whether to enable API watermark |
| output | string | no | output file path (default `/tmp/seedream_<ts>.png`) |

### refine
| name | type | required | notes |
|------|------|------|------|
| image_path | string | yes | input image path |
| prompt | string | yes | refinement instruction |
| watermark | bool | no | default `false`; whether to enable API watermark |
| output | string | no | output path (default `/tmp/seedream_refined_<ts>.png`) |
