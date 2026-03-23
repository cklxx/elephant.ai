---
name: video-production
description: When the user wants to create a short video → generate with Seedance, validate and save output files.
triggers:
  intent_patterns:
    - "视频|video|短视频|short.*video|生成.*视频|create.*video"
    - "动画|animation|动态.*效果|motion|转场|transition"
    - "视频.*素材|video.*clip|片段|footage"
    - "图片.*变.*视频|image.*to.*video|动起来|animate.*this"
    - "seedance|生成.*动画|make.*video"
  context_signals:
    keywords: ["视频", "video", "动画", "animation", "短视频", "seedance", "clip", "motion", "片段"]
  confidence_threshold: 0.6
priority: 7
requires_tools: [bash]
max_tokens: 200
cooldown: 30
---

# video-production

Generate short videos via ARK Seedance backend.

## Required Env
- `ARK_API_KEY`
- `SEEDANCE_ENDPOINT_ID`

## Constraints
- `action=generate` only.
- Backend must return a video `url`; missing URL fails fast.
- Output file must be written and non-empty, otherwise `success=false`.
- Default output path: `/tmp/seedance_<ts>.mp4`.

## Parameters
| name | type | required | notes |
|---|---|---|---|
| prompt | string | yes | video description |
| duration | number | no | seconds, default `5` |
| output | string | no | output path |

## Usage

```bash
python3 skills/video-production/run.py generate --prompt 'cute cat animation' --duration 5 --output /tmp/cat.mp4
```
