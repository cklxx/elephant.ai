#!/usr/bin/env python3
"""video-production skill — Seedance 视频生成。

通过 ARK API (Seedance) 生成短视频。
"""

from __future__ import annotations

from pathlib import Path
import sys

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv

load_repo_dotenv(__file__)

import json
import os
import sys
import time
import urllib.error
import urllib.request


_ARK_BASE = "https://ark.cn-beijing.volces.com/api/v3"


def generate(args: dict) -> dict:
    prompt = args.get("prompt", "")
    if not prompt:
        return {"success": False, "error": "prompt is required"}

    api_key = os.environ.get("ARK_API_KEY", "")
    endpoint = os.environ.get("SEEDANCE_ENDPOINT_ID", "")
    if not api_key:
        return {"success": False, "error": "ARK_API_KEY not set"}
    if not endpoint:
        return {"success": False, "error": "SEEDANCE_ENDPOINT_ID not set"}

    body = json.dumps({
        "model": endpoint,
        "prompt": prompt,
        "duration": args.get("duration", 5),
    }).encode()

    req = urllib.request.Request(
        f"{_ARK_BASE}/videos/generations",
        data=body,
        headers={"Authorization": f"Bearer {api_key}", "Content-Type": "application/json"},
        method="POST",
    )

    try:
        with urllib.request.urlopen(req, timeout=300) as resp:
            data = json.loads(resp.read().decode())
    except urllib.error.HTTPError as exc:
        body = ""
        try:
            body = exc.read().decode().strip()
        except Exception:
            body = ""
        detail = body or str(exc)
        return {"success": False, "error": f"HTTP Error {exc.code}: {detail}"}
    except urllib.error.URLError as exc:
        return {"success": False, "error": str(exc)}

    videos = data.get("data", [])
    if not videos:
        return {"success": False, "error": "no video returned"}

    output = args.get("output", f"/tmp/seedance_{int(time.time())}.mp4")
    video_url = videos[0].get("url", "")
    if not video_url:
        return {"success": False, "error": "response missing video url"}

    out_path = Path(output)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    try:
        with urllib.request.urlopen(video_url, timeout=300) as resp:
            out_path.write_bytes(resp.read())
    except urllib.error.URLError as exc:
        return {"success": False, "error": f"download failed: {exc}"}

    if not out_path.exists():
        return {"success": False, "error": f"video file not found after write: {output}"}
    if out_path.stat().st_size <= 0:
        return {"success": False, "error": f"video file is empty after write: {output}"}

    return {"success": True, "path": output, "prompt": prompt, "message": f"视频已保存到 {output}"}


def run(args: dict) -> dict:
    action = args.pop("action", "generate")
    if action == "generate":
        return generate(args)
    return {"success": False, "error": f"unknown action: {action}"}


def main() -> None:
    if len(sys.argv) > 1:
        args = json.loads(sys.argv[1])
    elif not sys.stdin.isatty():
        args = json.load(sys.stdin)
    else:
        args = {}
    result = run(args)
    json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")
    sys.exit(0 if result.get("success") else 1)


if __name__ == "__main__":
    main()
