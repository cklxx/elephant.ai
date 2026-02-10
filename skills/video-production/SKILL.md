---
name: video-production
description: Generate short videos with Seedance and save validated output files.
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
python3 skills/video-production/run.py '{"action":"generate","prompt":"cute cat animation","duration":5,"output":"/tmp/cat.mp4"}'
```
