#!/usr/bin/env python3
"""image-creation skill — AI 图片生成。

通过 Seedream API 生成图片。需要 ARK_API_KEY 环境变量。
"""

from __future__ import annotations

import base64
import json
import os
import sys
import time
import urllib.error
import urllib.request


_ARK_BASE = "https://ark.cn-beijing.volces.com/api/v3"


def _ark_request(endpoint_id: str, payload: dict) -> dict:
    """Call ARK (Volcengine) API."""
    api_key = os.environ.get("ARK_API_KEY", "")
    if not api_key:
        return {"error": "ARK_API_KEY not set"}

    url = f"{_ARK_BASE}/images/generations"
    body = {
        "model": endpoint_id,
        **payload,
    }
    data = json.dumps(body).encode()
    req = urllib.request.Request(
        url,
        data=data,
        headers={
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=120) as resp:
            return json.loads(resp.read().decode())
    except urllib.error.URLError as exc:
        return {"error": str(exc)}


def generate(args: dict) -> dict:
    prompt = args.get("prompt", "")
    if not prompt:
        return {"success": False, "error": "prompt is required"}

    endpoint = os.environ.get("SEEDREAM_TEXT_ENDPOINT_ID", "")
    if not endpoint:
        return {"success": False, "error": "SEEDREAM_TEXT_ENDPOINT_ID not set"}

    size = args.get("size", "1024x1024")
    w, h = size.split("x")

    result = _ark_request(endpoint, {
        "prompt": prompt,
        "size": f"{w}x{h}",
        "n": 1,
    })

    if "error" in result:
        return {"success": False, **result}

    images = result.get("data", [])
    if not images:
        return {"success": False, "error": "no image returned"}

    # Save image if output path specified
    output = args.get("output", f"/tmp/seedream_{int(time.time())}.png")
    img_data = images[0].get("b64_json", "")
    if img_data:
        with open(output, "wb") as f:
            f.write(base64.b64decode(img_data))

    return {
        "success": True,
        "image_path": output,
        "prompt": prompt,
        "size": size,
        "message": f"图片已保存到 {output}",
    }


def refine(args: dict) -> dict:
    image_path = args.get("image_path", "")
    prompt = args.get("prompt", "")
    if not image_path or not prompt:
        return {"success": False, "error": "image_path and prompt are required"}

    endpoint = os.environ.get("SEEDREAM_I2I_ENDPOINT_ID", "")
    if not endpoint:
        return {"success": False, "error": "SEEDREAM_I2I_ENDPOINT_ID not set"}

    with open(image_path, "rb") as f:
        img_b64 = base64.b64encode(f.read()).decode()

    result = _ark_request(endpoint, {
        "prompt": prompt,
        "image": img_b64,
        "n": 1,
    })

    if "error" in result:
        return {"success": False, **result}

    images = result.get("data", [])
    if not images:
        return {"success": False, "error": "no image returned"}

    output = args.get("output", f"/tmp/seedream_refined_{int(time.time())}.png")
    img_data = images[0].get("b64_json", "")
    if img_data:
        with open(output, "wb") as f:
            f.write(base64.b64decode(img_data))

    return {
        "success": True,
        "image_path": output,
        "prompt": prompt,
        "message": f"优化后图片已保存到 {output}",
    }


def run(args: dict) -> dict:
    action = args.pop("action", "generate")
    if action == "generate":
        return generate(args)
    if action == "refine":
        return refine(args)
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
    sys.exit(0 if result.get("success", False) else 1)


if __name__ == "__main__":
    main()
