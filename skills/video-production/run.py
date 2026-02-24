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
_DEFAULT_SEEDANCE_ENDPOINT_ID = "doubao-seedance-1-0-pro-fast-251015"
_DEFAULT_SEEDANCE_FALLBACK_ENDPOINTS = [
    _DEFAULT_SEEDANCE_ENDPOINT_ID,
]


def _candidate_endpoints(primary: str) -> list[str]:
    candidates: list[str] = [primary.strip()]
    raw_fallbacks = os.environ.get("SEEDANCE_ENDPOINT_FALLBACKS", "")
    if raw_fallbacks:
        candidates.extend(part.strip() for part in raw_fallbacks.split(","))
    candidates.extend(_DEFAULT_SEEDANCE_FALLBACK_ENDPOINTS)
    unique: list[str] = []
    for value in candidates:
        if value and value not in unique:
            unique.append(value)
    return unique


def _discover_seedance_endpoints(api_key: str) -> list[str]:
    req = urllib.request.Request(
        f"{_ARK_BASE}/models",
        headers={"Authorization": f"Bearer {api_key}"},
        method="GET",
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            payload = json.loads(resp.read().decode())
    except Exception:
        return []

    models = payload.get("data", [])
    discovered: list[str] = []
    for model in models:
        model_id = str(model.get("id", "")).strip()
        if model_id and "seedance" in model_id.lower() and model_id not in discovered:
            discovered.append(model_id)
    return discovered


def generate(args: dict) -> dict:
    prompt = args.get("prompt", "")
    if not prompt:
        return {"success": False, "error": "prompt is required"}

    api_key = os.environ.get("ARK_API_KEY", "")
    endpoint = os.environ.get("SEEDANCE_ENDPOINT_ID", "").strip() or _DEFAULT_SEEDANCE_ENDPOINT_ID
    if not api_key:
        return {"success": False, "error": "ARK_API_KEY not set"}

    attempts: list[str] = []
    endpoint_errors: list[str] = []
    candidates = _candidate_endpoints(endpoint)
    discovered_model_ids = False
    data: dict = {}

    while candidates:
        current_endpoint = candidates.pop(0)
        attempts.append(current_endpoint)
        request_body = json.dumps({
            "model": current_endpoint,
            "prompt": prompt,
            "duration": args.get("duration", 5),
        }).encode()

        req = urllib.request.Request(
            f"{_ARK_BASE}/videos/generations",
            data=request_body,
            headers={"Authorization": f"Bearer {api_key}", "Content-Type": "application/json"},
            method="POST",
        )

        try:
            with urllib.request.urlopen(req, timeout=300) as resp:
                data = json.loads(resp.read().decode())
            endpoint = current_endpoint
            break
        except urllib.error.HTTPError as exc:
            detail = ""
            try:
                detail = exc.read().decode().strip()
            except Exception:
                detail = ""
            endpoint_errors.append(f"{current_endpoint}: HTTP {exc.code} {detail or str(exc)}")
            if exc.code != 404:
                return {
                    "success": False,
                    "error": f"endpoint={current_endpoint} HTTP Error {exc.code}: {detail or str(exc)}",
                }
            if not discovered_model_ids:
                discovered_model_ids = True
                for candidate in _discover_seedance_endpoints(api_key):
                    if candidate not in attempts and candidate not in candidates:
                        candidates.append(candidate)
        except urllib.error.URLError as exc:
            return {"success": False, "error": str(exc)}
    else:
        attempted = ", ".join(attempts)
        details = "; ".join(endpoint_errors)
        return {
            "success": False,
            "error": f"all Seedance endpoints failed with 404. attempted: {attempted}",
            "details": details,
            "attempted_endpoints": attempts,
        }

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

    return {
        "success": True,
        "path": output,
        "prompt": prompt,
        "endpoint": endpoint,
        "message": f"视频已保存到 {output}",
    }


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
