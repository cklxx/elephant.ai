#!/usr/bin/env python3
"""image-creation skill — AI 图片生成。

通过 Seedream API 生成图片。需要 ARK_API_KEY 环境变量。
"""

from __future__ import annotations

from pathlib import Path
import sys

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv

load_repo_dotenv(__file__)

import base64
import json
import math
import os
import sys
import time
import urllib.error
import urllib.request


_ARK_BASE = "https://ark.cn-beijing.volces.com/api/v3"
_DEFAULT_SEEDREAM_TEXT_ENDPOINT_ID = "doubao-seedream-4-5-251128"
_MIN_IMAGE_PIXELS = 1920 * 1920


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
    except urllib.error.HTTPError as exc:
        body = ""
        try:
            body = exc.read().decode().strip()
        except Exception:
            body = ""
        detail = body or str(exc)
        return {"error": f"HTTP Error {exc.code}: {detail}"}
    except urllib.error.URLError as exc:
        return {"error": str(exc)}


def _resolve_seedream_text_endpoint() -> str:
    endpoint = os.environ.get("SEEDREAM_TEXT_ENDPOINT_ID", "").strip()
    if endpoint:
        return endpoint
    model = os.environ.get("SEEDREAM_TEXT_MODEL", "").strip()
    if model:
        return model
    return _DEFAULT_SEEDREAM_TEXT_ENDPOINT_ID


def _normalize_size(size: str) -> str:
    parts = size.lower().split("x")
    if len(parts) != 2:
        raise ValueError("size must be WIDTHxHEIGHT")
    width = int(parts[0].strip())
    height = int(parts[1].strip())
    if width <= 0 or height <= 0:
        raise ValueError("size must be WIDTHxHEIGHT with positive integers")
    pixels = width * height
    if pixels >= _MIN_IMAGE_PIXELS:
        return f"{width}x{height}"

    scale = math.sqrt(_MIN_IMAGE_PIXELS / pixels)
    scaled_width = math.ceil(width * scale)
    scaled_height = math.ceil(height * scale)
    return f"{scaled_width}x{scaled_height}"


def generate(args: dict) -> dict:
    prompt = args.get("prompt", "")
    if not prompt:
        return {"success": False, "error": "prompt is required"}

    endpoint = _resolve_seedream_text_endpoint()

    style = str(args.get("style", "realistic")).strip()
    requested_size = str(args.get("size", "1920x1920")).strip()
    try:
        effective_size = _normalize_size(requested_size)
    except ValueError as exc:
        return {"success": False, "error": str(exc)}

    prompt_with_style = prompt
    if style:
        prompt_with_style = f"{prompt}, {style} style"

    result = _ark_request(endpoint, {
        "prompt": prompt_with_style,
        "size": effective_size,
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
        "style": style,
        "size": effective_size,
        "requested_size": requested_size,
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
