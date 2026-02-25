"""audio-tts skill — macOS say -> m4a"""
from __future__ import annotations

import json
import os
import subprocess
import sys
import time
from pathlib import Path


def _run(cmd: list[str]) -> tuple[int, str, str]:
    result = subprocess.run(cmd, capture_output=True, text=True)
    return result.returncode, result.stdout, result.stderr


def speak(args: dict) -> dict:
    text = str(args.get("text", "")).strip()
    if not text:
        return {"success": False, "error": "text is required"}

    voice = str(args.get("voice", "")).strip()
    rate = args.get("rate", None)
    output = str(args.get("output", "")) or f"/tmp/tts_{int(time.time())}.m4a"

    tmp_aiff = f"/tmp/tts_{int(time.time())}.aiff"
    say_cmd = ["say", "-o", tmp_aiff]
    if voice:
        say_cmd += ["-v", voice]
    if rate is not None:
        try:
            say_cmd += ["-r", str(int(rate))]
        except Exception:
            return {"success": False, "error": "rate must be int"}
    say_cmd += [text]

    code, _, err = _run(say_cmd)
    if code != 0:
        return {"success": False, "error": f"say failed: {err.strip()}"}

    # convert to m4a
    Path(output).parent.mkdir(parents=True, exist_ok=True)
    code, _, err = _run(["afconvert", "-f", "m4af", "-d", "aac", tmp_aiff, output])
    try:
        os.remove(tmp_aiff)
    except Exception:
        pass
    if code != 0:
        return {"success": False, "error": f"afconvert failed: {err.strip()}"}
    if not Path(output).exists() or Path(output).stat().st_size == 0:
        return {"success": False, "error": "output file missing or empty"}

    return {
        "success": True,
        "audio_path": output,
        "text": text,
        "voice": voice or "default",
        "message": f"语音已保存到 {output}",
    }


def run(args: dict) -> dict:
    action = args.pop("action", "speak")
    if action == "speak":
        return speak(args)
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
